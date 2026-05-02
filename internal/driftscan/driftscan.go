// Package driftscan implements the `governa drift-scan` subcommand.
//
// It walks the canon overlay, byte-compares each governed file against the
// target adopted repo, classifies divergences, collects evidence (preserve
// markers, git log), computes next-AC and next-IE numbers, and emits a
// structured report. When the target has prerequisites (plan.md + docs/),
// it also stages a partially-filled AC stub and inserts plan.md IE entries.
//
// drift-scan is governa-internal protocol that consumes overlays. It does not
// have a consumer-overlay counterpart by design — the project rule that
// requires source-level changes under internal/ to propagate to overlays does
// not apply here. See docs/ac104-drift-scan-cmd.md.
package driftscan

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"

	"github.com/queone/governa/internal/governance"
	"github.com/queone/governa/internal/templates"
)

// Exit codes.
const (
	ExitOK       = 0
	ExitEnvError = 1
	ExitUsage    = 2
)

// asymmetryNote is the one-line scan-asymmetry text. AC105 Part B requires
// the same verbatim string in both the staged AC's `## Implementation Notes`
// opening and the console report header.
const asymmetryNote = "Scan walks canon→target only. Files in target with no canon counterpart are not enumerated here except via per-file `Coupled local-only files` sub-bullets."

// Classification labels per docs/drift-scan.md.
type Classification string

const (
	ClassMatch              Classification = "match"
	ClassPreserve           Classification = "preserve"
	ClassAmbiguity          Classification = "ambiguity"
	ClassClearSync          Classification = "clear-sync"
	ClassMissingTarget      Classification = "missing-in-target"
	ClassTargetNoCanon      Classification = "target-has-no-canon"
	ClassExpectedDivergence Classification = "expected-divergence" // per-repo content files (e.g., plan.md)
)

// Config holds drift-scan invocation parameters.
type Config struct {
	Target     string // resolved absolute path to target repo
	Flavor     string // "code" or "doc"
	JSON       bool
	DiffLines  int    // diff truncation limit
	RepoName   string // overrides basename of Target
	Invocation string // exact CLI invocation string for the report header

	// OverrideSHA bypasses canonSHA() lookup. Used in tests where
	// runtime/debug.ReadBuildInfo()'s vcs.revision is unavailable.
	// Production callers leave this empty.
	OverrideSHA string
}

// ParseArgs parses CLI arguments. Returns config, help bool, error.
func ParseArgs(args []string) (Config, bool, error) {
	cfg := Config{DiffLines: 200}
	fset := flag.NewFlagSet("governa drift-scan", flag.ContinueOnError)
	fset.SetOutput(os.Stderr)

	fset.StringVar(&cfg.Flavor, "f", "", "overlay flavor: code|doc")
	fset.StringVar(&cfg.Flavor, "flavor", "", "overlay flavor: code|doc")
	fset.BoolVar(&cfg.JSON, "j", false, "emit JSON report instead of markdown")
	fset.BoolVar(&cfg.JSON, "json", false, "emit JSON report instead of markdown")
	fset.IntVar(&cfg.DiffLines, "l", 200, "diff truncation limit")
	fset.IntVar(&cfg.DiffLines, "diff-lines", 200, "diff truncation limit")
	fset.StringVar(&cfg.RepoName, "n", "", "override repo name (default: basename of target)")
	fset.StringVar(&cfg.RepoName, "repo-name", "", "override repo name (default: basename of target)")

	for _, a := range args {
		if a == "-h" || a == "--help" || a == "-?" {
			printUsage()
			return cfg, true, nil
		}
	}

	if err := fset.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printUsage()
			return cfg, true, nil
		}
		return cfg, false, err
	}

	rest := fset.Args()
	if len(rest) == 0 {
		return cfg, false, fmt.Errorf("drift-scan: missing <repo-path> argument")
	}
	if len(rest) > 1 {
		return cfg, false, fmt.Errorf("drift-scan: unexpected extra arguments: %v", rest[1:])
	}

	abs, err := filepath.Abs(rest[0])
	if err != nil {
		return cfg, false, fmt.Errorf("drift-scan: resolve target path: %w", err)
	}
	cfg.Target = abs

	cfg.Invocation = "governa drift-scan " + strings.Join(args, " ")

	if cfg.Flavor != "" && cfg.Flavor != "code" && cfg.Flavor != "doc" {
		return cfg, false, fmt.Errorf("drift-scan: --flavor must be code or doc, got %q", cfg.Flavor)
	}

	return cfg, false, nil
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `Usage: governa drift-scan <repo-path> [flags]

Scan an adopted-governa repo against canon. Stages a partially-filled AC stub
and IE entries when plan.md and docs/ exist.

Flags:
  -f, --flavor code|doc      overlay flavor (default: auto-detect)
  -j, --json                 emit JSON report instead of markdown
  -l, --diff-lines <N>       diff truncation limit (default: 200)
  -n, --repo-name <name>     override repo name (default: basename of target)
  -h, --help                 show this help`)
}

// RunCLI is the cmd-layer entry point. Parses args, runs the scan, returns exit code.
func RunCLI(args []string, tfs fs.FS) (int, error) {
	cfg, help, err := ParseArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		printUsage()
		return ExitUsage, nil
	}
	if help {
		return ExitOK, nil
	}
	exit, err := Run(cfg, tfs, os.Stdout)
	return exit, err
}

// Run executes the drift-scan against cfg.Target, writing the report to out.
// Returns an exit code suitable for os.Exit.
func Run(cfg Config, tfs fs.FS, out io.Writer) (int, error) {
	// H1: validate target exists.
	info, err := os.Stat(cfg.Target)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return ExitEnvError, fmt.Errorf("drift-scan: target %s does not exist; pass an existing repo path", cfg.Target)
		}
		return ExitEnvError, fmt.Errorf("drift-scan: stat target %s: %w", cfg.Target, err)
	}
	if !info.IsDir() {
		return ExitEnvError, fmt.Errorf("drift-scan: target %s is not a directory", cfg.Target)
	}

	// Fail-safe: refuse governa-self.
	if err := governance.DetectGovernaCheckoutAt(cfg.Target); err == nil {
		return ExitEnvError, fmt.Errorf("drift-scan: target %s looks like a governa checkout — drift-scan is for adopted repos, not the governa source", cfg.Target)
	}

	// C1: require target to be a git repo. gitLogN silently returning empty
	// would otherwise downgrade ambiguous files to clear-sync without warning.
	gitDir := filepath.Join(cfg.Target, ".git")
	if _, err := os.Stat(gitDir); err != nil {
		return ExitEnvError, fmt.Errorf("drift-scan: target %s is not a git worktree (no .git/) — drift-scan needs git history to classify divergent files; run `git init` and commit, or pass an apply'd target", cfg.Target)
	}
	if _, err := exec.LookPath("git"); err != nil {
		return ExitEnvError, fmt.Errorf("drift-scan: git binary not found on PATH; install git before running drift-scan")
	}

	// Canon SHA via build-time vcs.revision (or test override).
	sha := cfg.OverrideSHA
	if sha == "" {
		var err error
		sha, err = canonSHA()
		if err != nil {
			return ExitEnvError, fmt.Errorf("drift-scan: %w", err)
		}
	}

	// Flavor.
	flavor := cfg.Flavor
	if flavor == "" {
		var err error
		flavor, err = detectFlavor(cfg.Target)
		if err != nil {
			return ExitEnvError, fmt.Errorf("drift-scan: %w", err)
		}
	}

	// Repo name.
	repoName := cfg.RepoName
	if repoName == "" {
		repoName = governance.InferRepoName(cfg.Target)
	}

	// Build governance.Config to drive the renderer.
	gcfg := governance.Config{
		Mode:     governance.ModeApply,
		Target:   cfg.Target,
		RepoName: repoName,
	}
	switch flavor {
	case "code":
		gcfg.Type = governance.RepoTypeCode
		stack := governance.InferStack(cfg.Target)
		if stack == "" {
			return ExitEnvError, fmt.Errorf("drift-scan: cannot resolve template variable {{STACK_OR_PLATFORM}} — target %s lacks a recognized stack manifest (go.mod, package.json, etc.); pass -f to disambiguate or run drift-scan against an apply'd target", cfg.Target)
		}
		gcfg.Stack = stack
	case "doc":
		gcfg.Type = governance.RepoTypeDoc
	}

	// Render canon to memory.
	canon, err := governance.RenderCanonicalFiles(tfs, gcfg, cfg.Target)
	if err != nil {
		return ExitEnvError, fmt.Errorf("drift-scan: render canon: %w", err)
	}

	// M4: ensure Invocation has a sensible default for library callers.
	invocation := cfg.Invocation
	if invocation == "" {
		invocation = fmt.Sprintf("governa drift-scan %s (programmatic)", cfg.Target)
	}

	// Walk canon, classify each.
	report := Report{
		Header: ReportHeader{
			Invocation: invocation,
			CanonSHA:   sha,
			Target:     cfg.Target,
			Flavor:     flavor,
			RepoName:   repoName,
		},
	}
	for _, relpath := range sortedKeys(canon) {
		canonContent := canon[relpath]
		fr := classifyFile(cfg, relpath, canonContent, sha)
		report.Files = append(report.Files, fr)
	}

	// C4: surface target-has-no-canon files for the chosen flavor.
	otherFlavor := "doc"
	if flavor == "doc" {
		otherFlavor = "code"
	}
	otherCanon, _ := otherFlavorCanonPaths(tfs, otherFlavor, repoName, cfg.Target)
	for _, rel := range targetGovernanceFilesNotInCanon(cfg.Target, canon, otherCanon) {
		report.Files = append(report.Files, FileResult{
			Relpath:        rel,
			Classification: ClassTargetNoCanon,
			CanonRef:       fmt.Sprintf("(no canon path for flavor %s)", flavor),
		})
	}

	// Enrich divergent files with same-directory local-only siblings and
	// compute routing groups (Part A: coupling-aware analysis).
	canonPaths := make(map[string]bool, len(canon))
	for k := range canon {
		canonPaths[k] = true
	}
	for i := range report.Files {
		if !isDivergentClass(report.Files[i].Classification) {
			continue
		}
		report.Files[i].CoupledLocalOnly = enumerateLocalOnlySiblings(cfg.Target, report.Files[i].Relpath, canonPaths)
	}
	report.RoutingGroups = computeRoutingGroups(report.Files, cfg.Target)

	// H3: compute numbering once and pass forward.
	report.NextAC, _ = nextACNumber(cfg.Target)
	report.NextIE, _ = nextIENumber(cfg.Target)

	// Pre-staging gates: orphaned-IE, prior-staging-for-this-SHA.
	if err := detectOrphanedIEs(cfg.Target); err != nil {
		// Still emit report; staging is skipped.
		report.StagingError = err.Error()
		writeReport(out, report, cfg.JSON)
		return ExitEnvError, nil
	}
	if err := checkPriorStaging(cfg.Target, sha); err != nil {
		report.StagingError = err.Error()
		writeReport(out, report, cfg.JSON)
		return ExitEnvError, nil
	}

	// Auto-stage if prereqs exist.
	planPath := filepath.Join(cfg.Target, "plan.md")
	docsDir := filepath.Join(cfg.Target, "docs")
	planExists := fileExists(planPath)
	docsExists := dirExists(docsDir)

	if planExists && docsExists {
		stageErr := stageAll(cfg.Target, sha, &report, report.NextAC, report.NextIE)
		if stageErr != nil {
			report.StagingError = stageErr.Error()
			writeReport(out, report, cfg.JSON)
			return ExitEnvError, nil
		}
	} else {
		var missing []string
		if !planExists {
			missing = append(missing, "plan.md")
		}
		if !docsExists {
			missing = append(missing, "docs/")
		}
		report.StagingError = fmt.Sprintf("staging skipped: target missing prerequisite(s): %s", strings.Join(missing, ", "))
		writeReport(out, report, cfg.JSON)
		return ExitEnvError, nil
	}

	writeReport(out, report, cfg.JSON)
	return ExitOK, nil
}

// canonSHA returns the 7-char canon SHA. Tries (in order) the build-time
// vcs.revision setting (works for `go build` / `go install`), then a
// runtime.Caller-based git rev-parse fallback (works for `go run` from a
// source checkout). The fallback exists because `go run` defaults to
// -buildvcs=auto which silently omits VCS info — without the fallback,
// `go run ./cmd/governa drift-scan ...` would always fail.
func canonSHA() (string, error) {
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" && s.Value != "" {
				if len(s.Value) >= 7 {
					return s.Value[:7], nil
				}
				return s.Value, nil
			}
		}
	}
	if sha, err := canonSHAFromSourceCheckout(); err == nil {
		return sha, nil
	}
	return "", fmt.Errorf("vcs.revision unavailable: BuildInfo lacks vcs.revision and source-checkout fallback failed — `go build` / `go install` from a git checkout, or pass `-buildvcs=true` to `go run`")
}

// canonSHAFromSourceCheckout uses runtime.Caller to locate this source file
// on disk and runs `git rev-parse HEAD` from its directory. Works when the
// binary is invoked from a source checkout (`go run` or any source-tree
// build); fails when the source is in a Go module cache or the source dir
// is not a git worktree.
func canonSHAFromSourceCheckout() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok || file == "" {
		return "", fmt.Errorf("runtime.Caller unavailable")
	}
	dir := filepath.Dir(file)
	cmd := exec.Command("git", "-C", dir, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	sha := strings.TrimSpace(string(out))
	if len(sha) >= 7 {
		return sha[:7], nil
	}
	return sha, nil
}

func detectFlavor(target string) (string, error) {
	hasGoMod := fileExists(filepath.Join(target, "go.mod"))
	hasJekyll := fileExists(filepath.Join(target, "_config.yml"))
	switch {
	case hasGoMod && !hasJekyll:
		return "code", nil
	case !hasGoMod && hasJekyll:
		return "doc", nil
	case !hasGoMod && !hasJekyll:
		return "doc", nil
	default:
		return "", fmt.Errorf("ambiguous flavor: target has both go.mod and _config.yml — pass -f|--flavor code|doc to disambiguate")
	}
}

// FileResult captures per-file scan outcome.
type FileResult struct {
	Relpath        string         `json:"relpath"`
	Classification Classification `json:"classification"`
	Diff           string         `json:"diff,omitempty"`
	Commits        []string       `json:"commits,omitempty"`
	Markers        []string       `json:"preserve_markers,omitempty"`
	CanonRef       string         `json:"canon_ref,omitempty"`
	CompareCommand string         `json:"compare_command,omitempty"`
	// CanonContent is the rendered canon file body. Populated for files that
	// have a canon (every class except ClassTargetNoCanon). Used by the AC
	// stub to render content previews for missing-in-target files and to
	// detect empty-canon edge cases. Not serialized to JSON to keep reports
	// small; the canon SHA + path identify the source.
	CanonContent string `json:"-"`
	// CoupledLocalOnly lists same-directory files that exist in target but
	// not in canon for this flavor. Surfaced in the AC's per-file block so
	// the Director sees the local-only files that ride along with this
	// file's routing decision. Populated only for divergent classifications
	// (preserve, ambiguity, clear-sync).
	CoupledLocalOnly []string `json:"coupled_local_only,omitempty"`
}

// ReportHeader is the report's self-identifying header.
type ReportHeader struct {
	Invocation string `json:"invocation"`
	CanonSHA   string `json:"canon_sha"`
	Target     string `json:"target"`
	Flavor     string `json:"flavor"`
	RepoName   string `json:"repo_name"`
}

// Report is the full scan output.
type Report struct {
	Header       ReportHeader `json:"header"`
	Files        []FileResult `json:"files"`
	NextAC       int          `json:"next_ac"`
	NextIE       int          `json:"next_ie"`
	Staging      *StagingInfo `json:"staging,omitempty"`
	StagingError string       `json:"staging_error,omitempty"`
	// RoutingGroups holds divergent files clustered by routing-coupling: two
	// files are in the same group iff they share a coupled local-only sibling
	// or are linked by a shell→binary `go run` reference. Each inner slice is
	// a list of relpaths. Populated in Run after classification; consumed by
	// the AC stub to emit one Director Review entry per group.
	RoutingGroups [][]string `json:"routing_groups,omitempty"`
}

// StagingInfo records what got staged.
type StagingInfo struct {
	ACPath      string   `json:"ac_path"`
	PlanInserts []string `json:"plan_md_inserts"`
}

// expectedDivergencePaths are governed files whose content is per-repo by
// design (template ships a stub; adopted repos fill it). Byte-compare always
// diverges; we skip them and surface as ClassMatch with a note. (H2)
var expectedDivergencePaths = map[string]bool{
	"plan.md": true,
}

func classifyFile(cfg Config, relpath, canon, sha string) FileResult {
	targetPath := filepath.Join(cfg.Target, relpath)
	fr := FileResult{
		Relpath:      relpath,
		CanonRef:     fmt.Sprintf("governa @ %s: internal/templates/overlays/<flavor>/files/%s", sha, relpath),
		CanonContent: canon,
	}
	targetBytes, err := os.ReadFile(targetPath)
	if err != nil {
		fr.Classification = ClassMissingTarget
		return fr
	}
	target := string(targetBytes)
	if target == canon {
		fr.Classification = ClassMatch
		fr.CompareCommand = fmt.Sprintf("byte-equal (canon @ %s vs %s)", sha, relpath)
		return fr
	}
	// H2: per-repo content files always diverge from the canon stub. Use
	// ClassExpectedDivergence (not ClassMatch) so the AC stub renders them
	// in their own subsection — listing them under "Match evidence" misled
	// readers into thinking content equality was verified.
	if expectedDivergencePaths[relpath] {
		fr.Classification = ClassExpectedDivergence
		fr.CompareCommand = fmt.Sprintf("expected per-repo divergence (canon @ %s is a content stub; %s carries repo-specific content)", sha, relpath)
		return fr
	}

	// Divergent — collect evidence.
	fr.Diff = unifiedDiff(canon, target, relpath, cfg.DiffLines)
	fr.Commits = gitLogN(cfg.Target, relpath, 5)
	fr.Markers = grepPreserveMarkers(cfg.Target, relpath)

	switch {
	case len(fr.Markers) > 0:
		fr.Classification = ClassPreserve
	case len(fr.Commits) > 0:
		fr.Classification = ClassAmbiguity
	default:
		fr.Classification = ClassClearSync
	}
	return fr
}

// unifiedDiff produces a `diff -u`-style output truncated to maxLines.
// Uses the system `diff` binary for fidelity with what users expect. Falls
// back to a placeholder marker if `diff` is unavailable so the staged AC
// surfaces the failure instead of an empty diff hunk. (H5)
func unifiedDiff(canon, target, relpath string, maxLines int) string {
	if _, err := exec.LookPath("diff"); err != nil {
		return fmt.Sprintf("[diff unavailable: %s — install GNU/BSD diff and re-run]", err)
	}
	canonF, err := os.CreateTemp("", "drift-canon-")
	if err != nil {
		return fmt.Sprintf("[diff failed: create canon tmp: %s]", err)
	}
	defer os.Remove(canonF.Name())
	canonF.WriteString(canon)
	canonF.Close()

	targetF, err := os.CreateTemp("", "drift-target-")
	if err != nil {
		return fmt.Sprintf("[diff failed: create target tmp: %s]", err)
	}
	defer os.Remove(targetF.Name())
	targetF.WriteString(target)
	targetF.Close()

	// M6: -L is the portable form (BSD + GNU diff both accept it; --label is
	// GNU-only on older systems).
	cmd := exec.Command("diff", "-u",
		"-L", "canon/"+relpath,
		"-L", "target/"+relpath,
		canonF.Name(), targetF.Name())
	out, runErr := cmd.CombinedOutput()
	// `diff` exits 1 when files differ — that's the success path here. Only
	// exit codes ≥ 2 indicate trouble.
	if exitErr, ok := runErr.(*exec.ExitError); ok && exitErr.ExitCode() >= 2 {
		return fmt.Sprintf("[diff failed: exit %d: %s]", exitErr.ExitCode(), strings.TrimSpace(string(out)))
	}
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	if maxLines > 0 && len(lines) > maxLines {
		extra := len(lines) - maxLines
		lines = lines[:maxLines]
		lines = append(lines, fmt.Sprintf("[... %d more lines truncated ...]", extra))
	}
	return strings.Join(lines, "\n")
}

func gitLogN(targetRoot, relpath string, n int) []string {
	cmd := exec.Command("git", "-C", targetRoot, "log",
		fmt.Sprintf("-n%d", n),
		"--follow", "--pretty=oneline", "--", relpath)
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var result []string
	for line := range strings.SplitSeq(strings.TrimRight(string(out), "\n"), "\n") {
		if line == "" {
			continue
		}
		result = append(result, line)
	}
	return result
}

// preserveMarkerPatterns is the fixed phrase set per docs/drift-scan.md.
// Format strings %s = relpath, %%s = literal %s placeholder for the qualifier.
var preserveMarkerPatterns = []string{
	`preserve %s`,
	`do not sync %s`,
	`intentional divergence: %s`,
	`%s: keep local`,
}

// grepPreserveMarkers scans CHANGELOG and docs/ac*.md for verbatim preserve
// markers naming relpath. C2: phrases must appear at a "boundary" — start of
// line, after a `|` (CHANGELOG table cell), or after a `;` (CHANGELOG cell
// separator) — optionally preceded by a list/bold marker. This avoids
// matching prose like "we should preserve docs/foo.md eventually" where the
// phrase appears mid-sentence.
func grepPreserveMarkers(targetRoot, relpath string) []string {
	var hits []string

	// Build per-pattern anchored regexes once.
	type compiled struct {
		re *regexp.Regexp
	}
	var compiledPats []compiled
	anchor := `(?:^|[|;])\s*(?:[-*]\s+|\*\*[^*]+\*\*\s+)?`
	for _, pat := range preserveMarkerPatterns {
		phrase := fmt.Sprintf(pat, relpath)
		compiledPats = append(compiledPats, compiled{
			re: regexp.MustCompile(anchor + regexp.QuoteMeta(phrase)),
		})
	}

	scan := func(content string) {
		for line := range strings.SplitSeq(content, "\n") {
			for _, c := range compiledPats {
				if c.re.MatchString(line) {
					hits = append(hits, strings.TrimSpace(line))
					break
				}
			}
		}
	}

	if changelog, err := os.ReadFile(filepath.Join(targetRoot, "CHANGELOG.md")); err == nil {
		scan(string(changelog))
	}

	docsDir := filepath.Join(targetRoot, "docs")
	if entries, err := os.ReadDir(docsDir); err == nil {
		for _, e := range entries {
			name := e.Name()
			if !strings.HasPrefix(name, "ac") || !strings.HasSuffix(name, ".md") {
				continue
			}
			content, err := os.ReadFile(filepath.Join(docsDir, name))
			if err != nil {
				continue
			}
			scan(string(content))
		}
	}

	return uniq(hits)
}

func uniq(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	var out []string
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

var (
	// L1: require the .md suffix to avoid matching backup files.
	acFilenameRe = regexp.MustCompile(`^ac(\d+)-.*\.md$`)
	acRefRe      = regexp.MustCompile(`AC(\d+)`)
	ieRe         = regexp.MustCompile(`IE(\d+)`)
)

func nextACNumber(targetRoot string) (int, error) {
	max := 0
	docsDir := filepath.Join(targetRoot, "docs")
	if entries, err := os.ReadDir(docsDir); err == nil {
		for _, e := range entries {
			m := acFilenameRe.FindStringSubmatch(e.Name())
			if m == nil {
				continue
			}
			n, _ := strconv.Atoi(m[1])
			if n > max {
				max = n
			}
		}
	}
	cmd := exec.Command("git", "-C", targetRoot, "log", "--all", "--pretty=%B")
	if out, err := cmd.Output(); err == nil {
		for _, m := range acRefRe.FindAllStringSubmatch(string(out), -1) {
			n, _ := strconv.Atoi(m[1])
			if n > max {
				max = n
			}
		}
	}
	return max + 1, nil
}

func nextIENumber(targetRoot string) (int, error) {
	planBytes, err := os.ReadFile(filepath.Join(targetRoot, "plan.md"))
	if err != nil {
		return 1, nil
	}
	max := 0
	for _, m := range ieRe.FindAllStringSubmatch(string(planBytes), -1) {
		n, _ := strconv.Atoi(m[1])
		if n > max {
			max = n
		}
	}
	return max + 1, nil
}

// detectOrphanedIEs scans plan.md for AC-pointer IE entries pointing at
// docs/ac*-drift-scan-from-*.md and verifies the referenced AC file exists.
// L2: accepts both `→` (U+2192) and ASCII `->` to be lenient on Operator typing.
func detectOrphanedIEs(targetRoot string) error {
	planBytes, err := os.ReadFile(filepath.Join(targetRoot, "plan.md"))
	if err != nil {
		return nil // plan.md missing is handled separately
	}
	re := regexp.MustCompile(`(?m)^- (IE\d+):.*(?:→|->)\s*(docs/ac\d+-drift-scan-from-[^\s]+\.md)`)
	for _, m := range re.FindAllStringSubmatch(string(planBytes), -1) {
		ieID := m[1]
		acPath := m[2]
		full := filepath.Join(targetRoot, acPath)
		if _, err := os.Stat(full); errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("plan.md %s points to deleted %s; remove the orphaned IE entries before re-running so staging does not produce duplicates", ieID, acPath)
		}
	}
	return nil
}

// checkPriorStaging looks for any pre-existing ac*-drift-scan-from-<sha>.md
// matching the current canon SHA in target's docs/.
func checkPriorStaging(targetRoot, sha string) error {
	docsDir := filepath.Join(targetRoot, "docs")
	entries, err := os.ReadDir(docsDir)
	if err != nil {
		return nil
	}
	suffix := fmt.Sprintf("-drift-scan-from-%s.md", sha)
	for _, e := range entries {
		name := e.Name()
		if strings.HasSuffix(name, suffix) {
			return fmt.Errorf("a prior drift-scan AC for canon SHA %s already exists at docs/%s; resolve (commit, delete, or amend) before re-running", sha, name)
		}
	}
	return nil
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

func dirExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// stageAll writes the AC stub and inserts plan.md IE entries atomically.
// H3: AC and IE numbers are passed in (computed once in Run) rather than
// recomputed here, eliminating the race window between the two reads.
func stageAll(targetRoot, sha string, report *Report, acN, ieStart int) error {
	acFilename := fmt.Sprintf("ac%d-drift-scan-from-%s.md", acN, sha)
	sisterFilename := fmt.Sprintf("ac%d-drift-scan-from-%s-diffs.md", acN, sha)
	acPath := filepath.Join(targetRoot, "docs", acFilename)
	sisterPath := filepath.Join(targetRoot, "docs", sisterFilename)

	planPath := filepath.Join(targetRoot, "plan.md")
	planBytes, err := os.ReadFile(planPath)
	if err != nil {
		return fmt.Errorf("read plan.md: %w", err)
	}

	ieEntries := buildIEEntries(ieStart, sha, acFilename)
	newPlanContent, err := insertIEsIntoPlan(string(planBytes), ieEntries)
	if err != nil {
		return err
	}

	acContent := buildACStub(acN, sha, report)
	sisterContent := buildSisterDiffs(acN, sha, report)

	// Atomic: write all three via temp files, then rename. Sister file
	// (Part B) is staged alongside the AC so the target-repo Operator has
	// the full diffs without re-running the scan.
	if err := atomicWrite(acPath, []byte(acContent)); err != nil {
		return fmt.Errorf("write AC: %w", err)
	}
	if err := atomicWrite(sisterPath, []byte(sisterContent)); err != nil {
		os.Remove(acPath)
		return fmt.Errorf("write sister diffs file: %w", err)
	}
	if err := atomicWrite(planPath, []byte(newPlanContent)); err != nil {
		// Roll back AC + sister writes.
		os.Remove(acPath)
		os.Remove(sisterPath)
		return fmt.Errorf("write plan.md: %w", err)
	}

	report.Staging = &StagingInfo{
		ACPath:      filepath.Join("docs", acFilename),
		PlanInserts: ieEntries,
	}
	return nil
}

func atomicWrite(dst string, content []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(dst), filepath.Base(dst)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, dst); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}

// buildIEEntries produces a single AC-pointer IE pointing at the staged AC.
// The AC carries the per-file divergence detail under ## Implementation Notes;
// separate pre-rubric IEs per ambiguity file are not emitted (they duplicate
// what the AC already carries).
func buildIEEntries(ieStart int, sha, acFilename string) []string {
	return []string{fmt.Sprintf(
		"- IE%d: drift-scan against governa @ %s → docs/%s",
		ieStart, sha, acFilename,
	)}
}

// insertIEsIntoPlan inserts new IE entries into plan.md after the highest
// existing IE<M> line. Replaces "(none active)" if that's the only content
// under ## Ideas To Explore. Errors if the section heading is missing.
func insertIEsIntoPlan(plan string, entries []string) (string, error) {
	lines := strings.Split(plan, "\n")
	var ideasIdx = -1
	for i, l := range lines {
		if strings.TrimSpace(l) == "## Ideas To Explore" {
			ideasIdx = i
			break
		}
	}
	if ideasIdx < 0 {
		return "", fmt.Errorf("plan.md missing `## Ideas To Explore` section or IE entries; fix the section heading before re-running")
	}

	// Find last IE<N> line within the Ideas To Explore section, or (none active) marker.
	lastIEIdx := -1
	noneActiveIdx := -1
	endSection := len(lines)
	for i := ideasIdx + 1; i < len(lines); i++ {
		l := lines[i]
		if strings.HasPrefix(strings.TrimSpace(l), "## ") {
			endSection = i
			break
		}
		if ieRe.MatchString(l) && strings.HasPrefix(strings.TrimSpace(l), "- IE") {
			lastIEIdx = i
		}
		if strings.TrimSpace(l) == "(none active)" {
			noneActiveIdx = i
		}
	}

	if noneActiveIdx >= 0 && lastIEIdx < 0 {
		// Replace (none active) with the new entries.
		out := append([]string(nil), lines[:noneActiveIdx]...)
		out = append(out, entries...)
		out = append(out, lines[noneActiveIdx+1:]...)
		return strings.Join(out, "\n"), nil
	}

	if lastIEIdx < 0 {
		return "", fmt.Errorf("plan.md missing `## Ideas To Explore` section or IE entries; fix the section heading before re-running")
	}

	// Insert after lastIEIdx.
	out := append([]string(nil), lines[:lastIEIdx+1]...)
	out = append(out, entries...)
	out = append(out, lines[lastIEIdx+1:endSection]...)
	out = append(out, lines[endSection:]...)
	return strings.Join(out, "\n"), nil
}

// buildACStub constructs the partially-filled AC stub.
func buildACStub(acN int, sha string, report *Report) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# AC%d Drift-Scan from governa @ %s\n\n", acN, sha)

	clearSync := filterByClass(report.Files, ClassClearSync)
	preserves := filterByClass(report.Files, ClassPreserve)
	ambiguities := filterByClass(report.Files, ClassAmbiguity)
	matches := filterByClass(report.Files, ClassMatch)
	expectedDiv := filterByClass(report.Files, ClassExpectedDivergence)
	missing := filterByClass(report.Files, ClassMissingTarget)
	noCanon := filterByClass(report.Files, ClassTargetNoCanon)

	// Split missing-in-target by canon-emptiness: non-empty canon files are
	// create candidates that route into ## In Scope; empty-canon files stay
	// in Warnings as informational.
	var missingCreate, missingEmpty []FileResult
	for _, f := range missing {
		if strings.TrimSpace(f.CanonContent) != "" {
			missingCreate = append(missingCreate, f)
		} else {
			missingEmpty = append(missingEmpty, f)
		}
	}

	inScopeEmpty := len(clearSync) == 0 && len(missingCreate) == 0

	// Summary placeholder
	fmt.Fprintln(&b, "## Summary")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "<!-- TBD by Operator -->")
	if inScopeEmpty {
		fmt.Fprintln(&b, "<!-- HINT: `## In Scope` is empty (every divergent file is preserved, pending Director routing, or absent from canon). Per protocol, state explicitly that this AC ships only itself plus the staged plan.md IE entry — no file edits land. -->")
	}
	fmt.Fprintln(&b)

	// Objective Fit placeholder
	fmt.Fprintln(&b, "## Objective Fit")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "<!-- TBD by Operator -->")
	fmt.Fprintln(&b)

	// In Scope: clear-sync files + missing-in-target with non-empty canon.
	fmt.Fprintln(&b, "## In Scope")
	fmt.Fprintln(&b)
	if inScopeEmpty {
		fmt.Fprintln(&b, "None.")
	} else {
		for _, f := range clearSync {
			fmt.Fprintf(&b, "- `%s` — sync to canon\n", f.Relpath)
		}
		for _, f := range missingCreate {
			fmt.Fprintf(&b, "- `%s` — create from canon\n", f.Relpath)
		}
	}
	fmt.Fprintln(&b)

	// Out Of Scope
	fmt.Fprintln(&b, "## Out Of Scope")
	fmt.Fprintln(&b)
	if len(preserves) == 0 {
		fmt.Fprintln(&b, "None.")
	} else {
		for _, f := range preserves {
			fmt.Fprintf(&b, "- `%s` — preserve marker present:\n", f.Relpath)
			for _, m := range f.Markers {
				fmt.Fprintf(&b, "  - `%s`\n", m)
			}
		}
	}
	fmt.Fprintln(&b)

	// Implementation Notes
	fmt.Fprintln(&b, "## Implementation Notes")
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "Canon: governa @ %s, flavor `%s`. Comparison: `governa drift-scan` against the embedded canon.\n", sha, report.Header.Flavor)
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "Counts: %s.\n", tallyClassifications(report.Files))
	fmt.Fprintln(&b)

	// Scan asymmetry note (Part B). Same verbatim text as the console
	// report header — see asymmetryNote constant.
	fmt.Fprintln(&b, asymmetryNote)
	fmt.Fprintln(&b)

	// Sister-file cross-ref (Part B). Per-file diffs live in the sister
	// file alongside this AC; per-file blocks below carry the commit list
	// but no diff hunks. Both files share the `docs/ac<N>-*.md` prefix so
	// release prep deletes them together.
	sisterFilename := fmt.Sprintf("ac%d-drift-scan-from-%s-diffs.md", acN, sha)
	fmt.Fprintf(&b, "Per-file diffs: `docs/%s`.\n", sisterFilename)
	fmt.Fprintln(&b)

	// Routing summary table (Part B). First sub-subsection of Implementation
	// Notes — gives the reader the routing decision surface up front. Tool
	// fills File and Local edit source from the most-recent commit subject;
	// Operator fills the one-line characterization and recommendation.
	divergent := []FileResult{}
	divergent = append(divergent, preserves...)
	divergent = append(divergent, ambiguities...)
	divergent = append(divergent, clearSync...)
	if len(divergent) > 0 {
		fmt.Fprintln(&b, "### Routing summary")
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, "| File | Local edit source | What diverged | Recommendation |")
		fmt.Fprintln(&b, "|---|---|---|---|")
		for _, f := range divergent {
			editSrc := "—"
			if len(f.Commits) > 0 {
				_, subject, _ := strings.Cut(f.Commits[0], " ")
				// Escape | so it doesn't break the table cell.
				editSrc = strings.ReplaceAll(subject, "|", `\|`)
			}
			fmt.Fprintf(&b, "| `%s` | %s | <!-- TBD by Operator --> | <!-- TBD by Operator --> |\n", f.Relpath, editSrc)
		}
		fmt.Fprintln(&b)
	}

	// Match evidence (true byte-equal only).
	if len(matches) > 0 {
		fmt.Fprintln(&b, "### Match evidence")
		fmt.Fprintln(&b)
		for _, f := range matches {
			fmt.Fprintf(&b, "- `%s` — %s\n", f.Relpath, f.CompareCommand)
		}
		fmt.Fprintln(&b)
	}

	// Expected per-repo divergence (plan.md and similar). Surfaced separately
	// from byte-equal matches so the Operator does not misread "match" as
	// "verified canonical".
	if len(expectedDiv) > 0 {
		fmt.Fprintln(&b, "### Expected per-repo divergence")
		fmt.Fprintln(&b)
		for _, f := range expectedDiv {
			fmt.Fprintf(&b, "- `%s` — %s\n", f.Relpath, f.CompareCommand)
		}
		fmt.Fprintln(&b)
	}

	// Divergent file detail (preserve, ambiguity, clear-sync). `divergent`
	// was declared earlier for the routing summary table.
	if len(divergent) > 0 {
		fmt.Fprintln(&b, "### Divergent files")
		fmt.Fprintln(&b)
		for _, f := range divergent {
			fmt.Fprintf(&b, "#### `%s` — %s\n\n", f.Relpath, f.Classification)
			fmt.Fprintf(&b, "Canon: %s\n\n", f.CanonRef)
			// Coupled local-only files line. Always rendered (None when
			// empty) so silence does not read as "checked, none found".
			if len(f.CoupledLocalOnly) > 0 {
				fmt.Fprintf(&b, "Coupled local-only files: %s\n\n", strings.Join(f.CoupledLocalOnly, ", "))
			} else {
				fmt.Fprintln(&b, "Coupled local-only files: None")
				fmt.Fprintln(&b)
			}
			if len(f.Commits) > 0 {
				fmt.Fprintln(&b, "Local commits (`git log -n 5 --follow`):")
				fmt.Fprintln(&b)
				for _, c := range f.Commits {
					fmt.Fprintf(&b, "- `%s`\n", annotateCommit(c))
				}
				fmt.Fprintln(&b)
			}
		}
	}

	// Missing in target — create candidates (non-empty canon). Routed to
	// ## In Scope above; this subsection carries the actionable detail
	// (canon ref + content preview) so the Operator does not need to leave
	// the AC to see what would be created.
	if len(missingCreate) > 0 {
		fmt.Fprintln(&b, "### Missing in target (create candidates)")
		fmt.Fprintln(&b)
		for _, f := range missingCreate {
			fmt.Fprintf(&b, "#### `%s` — missing-in-target\n\n", f.Relpath)
			fmt.Fprintf(&b, "Canon: %s\n\n", f.CanonRef)
			fmt.Fprintln(&b, "Canon content (preview):")
			fmt.Fprintln(&b)
			fmt.Fprintln(&b, "```")
			fmt.Fprintln(&b, previewCanonContent(f.CanonContent, 30))
			fmt.Fprintln(&b, "```")
			fmt.Fprintln(&b)
		}
	}

	// Files in target without canon. Previously buried in the per-file flat
	// list with no signal to the Operator. The walker only surfaces these
	// when the file IS in the OTHER flavor's canon, so each entry is a
	// genuine flavor-mismatch hint worth the Director's eye.
	if len(noCanon) > 0 {
		fmt.Fprintln(&b, "### Files in target without canon")
		fmt.Fprintln(&b)
		fmt.Fprintf(&b, "These files exist in the target but NOT in canon for flavor `%s`. They DO appear in the other flavor's canon — the Director should confirm flavor selection or accept these as legitimate per-repo additions.\n", report.Header.Flavor)
		fmt.Fprintln(&b)
		for _, f := range noCanon {
			fmt.Fprintf(&b, "- `%s` — %s\n", f.Relpath, f.Classification)
		}
		fmt.Fprintln(&b)
	}

	// Warnings: only missing-in-target with empty canon (rare but possible).
	if len(missingEmpty) > 0 {
		fmt.Fprintln(&b, "### Warnings")
		fmt.Fprintln(&b)
		for _, f := range missingEmpty {
			fmt.Fprintf(&b, "- `%s` — %s (canon is empty; no action)\n", f.Relpath, f.Classification)
		}
		fmt.Fprintln(&b)
	}

	// Post-merge coherence audit placeholder.
	fmt.Fprintln(&b, "### Post-merge coherence audit")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "<!-- TBD by Operator -->")
	if inScopeEmpty {
		fmt.Fprintln(&b, "<!-- HINT: `## In Scope` is empty, so no canonical text lands in this AC — the audit has nothing to apply. Stating `Not applicable — In Scope is empty.` is sufficient. -->")
	}
	fmt.Fprintln(&b)

	// Acceptance Tests (Part B: tool emits ATs only for In Scope deliverables).
	// Preserve-marker ATs and the IE-pointer AT were dropped — they verified
	// scaffolding placed by earlier ACs / by this scan's staging step, not
	// this AC's deliverable.
	fmt.Fprintln(&b, "## Acceptance Tests")
	fmt.Fprintln(&b)
	if len(clearSync) == 0 && len(missingCreate) == 0 {
		fmt.Fprintln(&b, "None — this AC ships only the staged plan.md IE entry; nothing to verify in target.")
		fmt.Fprintln(&b)
	} else {
		atN := 1
		for _, f := range clearSync {
			fmt.Fprintf(&b, "**AT%d** [Automated] — <!-- TBD by Operator: verify `%s` synced to canon (e.g., `rg -qF -- '<canonical line>' %s`). -->\n\n", atN, f.Relpath, f.Relpath)
			atN++
		}
		for _, f := range missingCreate {
			fmt.Fprintf(&b, "**AT%d** [Automated] — `test -f %s` <!-- TBD by Operator: extend with a content check (e.g., `rg -qF -- '<canonical line>' %s`). -->\n\n", atN, f.Relpath, f.Relpath)
			atN++
		}
	}

	// Documentation Updates
	fmt.Fprintln(&b, "## Documentation Updates")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "- `CHANGELOG.md` — release row added at release prep time per template guidance.")
	fmt.Fprintln(&b)

	// Director Review: auto-populate one routing question per coupled set
	// containing at least one ambiguity. Coupled-set grouping (Part A)
	// collapses files that share a local-only sibling or are linked by
	// shell→binary `go run` so the Director routes them together. Groups
	// containing only preserve/clear-sync files are skipped (already routed
	// to Out Of Scope / In Scope respectively). When there are no ambiguity
	// files, the section stays as `None.` per protocol.
	fmt.Fprintln(&b, "## Director Review")
	fmt.Fprintln(&b)
	relpathToFile := map[string]FileResult{}
	for _, f := range report.Files {
		relpathToFile[f.Relpath] = f
	}
	var routingGroups [][]string
	for _, g := range report.RoutingGroups {
		for _, rel := range g {
			if relpathToFile[rel].Classification == ClassAmbiguity {
				routingGroups = append(routingGroups, g)
				break
			}
		}
	}
	if len(routingGroups) == 0 {
		fmt.Fprintln(&b, "None.")
	} else {
		// collectCoupled returns the de-duplicated union of CoupledLocalOnly
		// entries across all files in the group. Local-only siblings are
		// surfaced in the routing question so the Director sees the full
		// blast radius of the routing decision in one place (QA-2).
		collectCoupled := func(group []string) []string {
			seen := map[string]bool{}
			var coupled []string
			for _, rel := range group {
				for _, c := range relpathToFile[rel].CoupledLocalOnly {
					if seen[c] {
						continue
					}
					seen[c] = true
					coupled = append(coupled, c)
				}
			}
			sort.Strings(coupled)
			return coupled
		}
		formatPaths := func(rels []string) string {
			out := make([]string, len(rels))
			for i, r := range rels {
				out[i] = "`" + r + "`"
			}
			return strings.Join(out, ", ")
		}
		for i, g := range routingGroups {
			coupled := collectCoupled(g)
			if len(g) == 1 {
				rel := g[0]
				if len(coupled) > 0 {
					fmt.Fprintf(&b,
						"%d. Should `%s` (with local-only siblings: %s) be synced to canon, preserved with a marker (backfill `preserve %s <qualifier>` in CHANGELOG.md), or deferred to a later AC? Operator lean: <!-- TBD by Operator -->. Why: <!-- TBD by Operator -->.\n",
						i+1, rel, formatPaths(coupled), rel,
					)
				} else {
					fmt.Fprintf(&b,
						"%d. Should `%s` be synced to canon, preserved with a marker (backfill `preserve %s <qualifier>` in CHANGELOG.md), or deferred to a later AC? Operator lean: <!-- TBD by Operator -->. Why: <!-- TBD by Operator -->.\n",
						i+1, rel, rel,
					)
				}
			} else {
				if len(coupled) > 0 {
					fmt.Fprintf(&b,
						"%d. Should %s (coupled — must route together; local-only siblings: %s) be synced to canon, preserved with markers, or deferred to a later AC? Operator lean: <!-- TBD by Operator -->. Why: <!-- TBD by Operator -->.\n",
						i+1, formatPaths(g), formatPaths(coupled),
					)
				} else {
					fmt.Fprintf(&b,
						"%d. Should %s (coupled — must route together) be synced to canon, preserved with markers, or deferred to a later AC? Operator lean: <!-- TBD by Operator -->. Why: <!-- TBD by Operator -->.\n",
						i+1, formatPaths(g),
					)
				}
			}
		}
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, "<!-- HINT: coupled files are pre-grouped above — the routing decision applies to the whole group. Verify the grouping looks right before answering. -->")
	}
	fmt.Fprintln(&b)

	// Status
	fmt.Fprintln(&b, "## Status")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "`PENDING` — awaiting Director critique.")

	return b.String()
}

func filterByClass(files []FileResult, c Classification) []FileResult {
	var out []FileResult
	for _, f := range files {
		if f.Classification == c {
			out = append(out, f)
		}
	}
	return out
}

// tallyClassifications returns a comma-separated count of non-zero
// classifications for the report header, e.g. "5 match, 1 preserve, 4 ambiguity".
// Stable ordering matches the order classifications are introduced in the AC stub.
func tallyClassifications(files []FileResult) string {
	order := []Classification{
		ClassMatch, ClassExpectedDivergence, ClassPreserve, ClassAmbiguity,
		ClassClearSync, ClassMissingTarget, ClassTargetNoCanon,
	}
	counts := map[Classification]int{}
	for _, f := range files {
		counts[f.Classification]++
	}
	var parts []string
	for _, c := range order {
		if n := counts[c]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, c))
		}
	}
	if len(parts) == 0 {
		return "0 files"
	}
	return strings.Join(parts, ", ")
}

// adoptionCommitRe matches commits whose subject signals the file was first
// brought under governance — initial governa adoption commits, or the catch-all
// "govern <repo>" commit some adopters use. False-positive risk is low because
// the regex requires either the literal "governa" token or a sentence-initial
// "govern" verb.
var adoptionCommitRe = regexp.MustCompile(`(?i)\bgoverna\b|^govern[a-z]*\b`)

// annotateCommit tags `git log` oneline output with `(adoption)` when the
// subject matches adoptionCommitRe. The Operator can then ignore those lines
// at a glance; they are signal-poor (every governed file has them) and
// crowded out the recent, actionable history.
func annotateCommit(line string) string {
	// Oneline format: "<hash> <subject>". Subject starts after the first space.
	_, subject, ok := strings.Cut(line, " ")
	if !ok {
		subject = line
	}
	if adoptionCommitRe.MatchString(subject) {
		return line + " (adoption)"
	}
	return line
}

// previewCanonContent truncates s to maxLines, appending a truncation marker
// when content is dropped. Used in the AC stub's missing-in-target detail
// section so the Operator can see what the canon would create without
// leaving the AC.
func previewCanonContent(s string, maxLines int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if maxLines <= 0 || len(lines) <= maxLines {
		return strings.Join(lines, "\n")
	}
	extra := len(lines) - maxLines
	return strings.Join(lines[:maxLines], "\n") + fmt.Sprintf("\n[... %d more lines truncated ...]", extra)
}

// writeReport emits to out in markdown or JSON.
func writeReport(out io.Writer, r Report, asJSON bool) {
	if asJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		_ = enc.Encode(r)
		return
	}
	fmt.Fprintln(out, "# Drift-Scan Report")
	fmt.Fprintln(out)
	fmt.Fprintf(out, "- Invocation: `%s`\n", r.Header.Invocation)
	fmt.Fprintf(out, "- Canon: governa @ %s\n", r.Header.CanonSHA)
	fmt.Fprintf(out, "- Target: %s\n", r.Header.Target)
	fmt.Fprintf(out, "- Flavor: %s\n", r.Header.Flavor)
	fmt.Fprintf(out, "- Repo name: %s\n", r.Header.RepoName)
	fmt.Fprintf(out, "- Next AC: AC%d\n", r.NextAC)
	fmt.Fprintf(out, "- Next IE: IE%d\n", r.NextIE)
	fmt.Fprintf(out, "- Counts: %s\n", tallyClassifications(r.Files))
	// Asymmetry note — same verbatim text used in the staged AC's
	// `## Implementation Notes` opening (see asymmetryNote constant).
	fmt.Fprintf(out, "- %s\n", asymmetryNote)
	if r.Staging != nil {
		fmt.Fprintf(out, "- Staged: `%s`\n", r.Staging.ACPath)
	}
	if r.StagingError != "" {
		fmt.Fprintf(out, "- Staging: skipped — %s\n", r.StagingError)
	}
	fmt.Fprintln(out)

	fmt.Fprintln(out, "## Files")
	fmt.Fprintln(out)
	for _, f := range r.Files {
		fmt.Fprintf(out, "### `%s` — %s\n\n", f.Relpath, f.Classification)
		if f.CompareCommand != "" {
			fmt.Fprintf(out, "Compare: %s\n\n", f.CompareCommand)
		}
		if f.CanonRef != "" {
			fmt.Fprintf(out, "Canon ref: `%s`\n\n", f.CanonRef)
		}
		if len(f.Markers) > 0 {
			fmt.Fprintln(out, "Preserve markers:")
			for _, m := range f.Markers {
				fmt.Fprintf(out, "- `%s`\n", m)
			}
			fmt.Fprintln(out)
		}
		if len(f.Commits) > 0 {
			fmt.Fprintln(out, "Local commits:")
			for _, c := range f.Commits {
				fmt.Fprintf(out, "- `%s`\n", c)
			}
			fmt.Fprintln(out)
		}
		if f.Diff != "" {
			fmt.Fprintln(out, "```diff")
			fmt.Fprintln(out, f.Diff)
			fmt.Fprintln(out, "```")
			fmt.Fprintln(out)
		}
	}
}

// isDivergentClass reports whether c is one of the classifications that
// indicates a per-file divergence requiring routing analysis (preserve,
// ambiguity, clear-sync). Match, expected-divergence, missing-in-target,
// and target-has-no-canon are routed by other paths or skipped entirely.
func isDivergentClass(c Classification) bool {
	return c == ClassPreserve || c == ClassAmbiguity || c == ClassClearSync
}

// perACFileRe matches per-AC file basenames (ac<N>-<slug>.md). Used to
// filter per-AC files out of CoupledLocalOnly enumeration — they are not
// "coupled" to the file alongside them, just per-repo work products.
var perACFileRe = regexp.MustCompile(`^ac\d+-.*\.md$`)

// noiseFileNames is the explicit set of OS / editor artifacts to exclude
// from CoupledLocalOnly enumeration. Conservative list — adding random
// dotfiles to the filter would also drop genuinely meaningful local files
// like .envrc, .tool-versions, etc.
var noiseFileNames = map[string]bool{
	".DS_Store": true,
	"Thumbs.db": true,
	".gitkeep":  true,
}

// noiseFileSuffixes are basename suffixes that indicate transient editor
// artifacts (Vim swap files, Emacs backups, etc.). Filtered for the same
// reason as noiseFileNames.
var noiseFileSuffixes = []string{".swp", ".swo", "~"}

// isNoiseFile reports whether a basename is OS/editor noise that should
// not appear in the Coupled local-only files list.
func isNoiseFile(name string) bool {
	if noiseFileNames[name] {
		return true
	}
	for _, suf := range noiseFileSuffixes {
		if strings.HasSuffix(name, suf) {
			return true
		}
	}
	return false
}

// enumerateLocalOnlySiblings returns same-directory files in target that
// do not exist in canon for this flavor. Per-AC files, OS/editor noise,
// and symlinks are filtered. Used to populate FileResult.CoupledLocalOnly
// so the Director sees the local-only files that ride along with each
// routing decision.
func enumerateLocalOnlySiblings(targetRoot, relpath string, canonPaths map[string]bool) []string {
	dir := filepath.Dir(relpath)
	if dir == "." {
		dir = ""
	}
	targetDir := filepath.Join(targetRoot, dir)
	entries, err := os.ReadDir(targetDir)
	if err != nil {
		return nil
	}
	var siblings []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		// Skip symlinks — typically aliases (e.g., CLAUDE.md → AGENTS.md)
		// rather than independent local-only files.
		if e.Type()&fs.ModeSymlink != 0 {
			continue
		}
		sibling := e.Name()
		if dir != "" {
			sibling = dir + "/" + e.Name()
		}
		if sibling == relpath {
			continue
		}
		if canonPaths[sibling] {
			continue
		}
		if perACFileRe.MatchString(e.Name()) {
			continue
		}
		if isNoiseFile(e.Name()) {
			continue
		}
		siblings = append(siblings, sibling)
	}
	sort.Strings(siblings)
	return siblings
}

// shellGoRunRe matches `go run <arg>` invocations in shell scripts. Used
// for the coarse shell→binary coupling pass: scripts that `go run` a
// package containing another divergent file route together with that
// file. Single regex pass over `*.sh` only — broader scanning of
// `bash -c`, Makefile recipes, and `*.go` build directives is deferred
// until a concrete failure case shows up.
var shellGoRunRe = regexp.MustCompile(`go\s+run\s+(\S+)`)

// computeRoutingGroups clusters divergent files into routing groups.
// Two files are in the same group iff:
//   - they share at least one coupled local-only sibling, or
//   - one is a shell script whose `go run` arg resolves to a directory
//     containing the other.
//
// Returns groups as slices of relpaths, sorted by first-relpath for
// stable rendering. Non-divergent files are excluded. Used by the AC
// stub to emit one Director Review entry per group instead of per file.
func computeRoutingGroups(files []FileResult, targetRoot string) [][]string {
	var divergent []FileResult
	for _, f := range files {
		if isDivergentClass(f.Classification) {
			divergent = append(divergent, f)
		}
	}
	n := len(divergent)
	if n == 0 {
		return nil
	}

	parent := make([]int, n)
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(i int) int {
		if parent[i] != i {
			parent[i] = find(parent[i])
		}
		return parent[i]
	}
	union := func(a, b int) {
		ra, rb := find(a), find(b)
		if ra != rb {
			parent[ra] = rb
		}
	}

	// Edge 1: shared CoupledLocalOnly entry — two divergent files with
	// overlapping local-only siblings are in the same routing group.
	seenSibling := map[string]int{}
	for i, f := range divergent {
		for _, s := range f.CoupledLocalOnly {
			if j, ok := seenSibling[s]; ok {
				union(i, j)
			} else {
				seenSibling[s] = i
			}
		}
	}

	// Edge 2: shell→binary cross-link. Read each divergent *.sh, find
	// `go run <arg>` references, resolve <arg> to a package directory,
	// and union the script with any divergent file in that directory.
	for i, f := range divergent {
		if !strings.HasSuffix(f.Relpath, ".sh") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(targetRoot, f.Relpath))
		if err != nil {
			continue
		}
		for _, m := range shellGoRunRe.FindAllStringSubmatch(string(content), -1) {
			arg := strings.TrimPrefix(m[1], "./")
			pkgDir := arg
			if filepath.Ext(arg) != "" {
				// Arg is a file path (e.g., ./cmd/rel/main.go); use parent dir.
				pkgDir = filepath.Dir(arg)
			}
			for j, fj := range divergent {
				if i == j {
					continue
				}
				if filepath.Dir(fj.Relpath) == pkgDir {
					union(i, j)
				}
			}
		}
	}

	groupMap := map[int][]string{}
	for i, f := range divergent {
		root := find(i)
		groupMap[root] = append(groupMap[root], f.Relpath)
	}
	var groups [][]string
	for _, g := range groupMap {
		sort.Strings(g)
		groups = append(groups, g)
	}
	sort.Slice(groups, func(a, b int) bool {
		return groups[a][0] < groups[b][0]
	})
	return groups
}

// EmbeddedFS exposes the templates FS to test callers without exporting the
// templates package elsewhere.
var EmbeddedFS = templates.EmbeddedFS

// buildSisterDiffs constructs the sister-file content carrying full per-file
// diffs. The AC stays a clean decision document; this sister file holds the
// verification material the target-repo Operator needs without re-running
// the scan. Title points back at the parent AC; one `## <relpath>` section
// per divergent file with the verbatim `diff -u` hunk.
func buildSisterDiffs(acN int, sha string, report *Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Diffs for AC%d (drift-scan from governa @ %s)\n\n", acN, sha)
	var divergent []FileResult
	for _, f := range report.Files {
		if isDivergentClass(f.Classification) {
			divergent = append(divergent, f)
		}
	}
	sort.Slice(divergent, func(i, j int) bool {
		return divergent[i].Relpath < divergent[j].Relpath
	})
	if len(divergent) == 0 {
		fmt.Fprintln(&b, "No divergent files.")
		return b.String()
	}
	for _, f := range divergent {
		fmt.Fprintf(&b, "## `%s`\n\n", f.Relpath)
		fmt.Fprintln(&b, "```diff")
		fmt.Fprintln(&b, f.Diff)
		fmt.Fprintln(&b, "```")
		fmt.Fprintln(&b)
	}
	return b.String()
}

// otherFlavorCanonPaths renders the OTHER flavor's canon and returns the set
// of relpaths it produces. Used by C4 to detect target files that would be
// governed by the other flavor (suggesting flavor mis-detection or a
// straddling repo).
func otherFlavorCanonPaths(tfs fs.FS, otherFlavor, repoName, target string) (map[string]bool, error) {
	gcfg := governance.Config{
		Mode:     governance.ModeApply,
		Target:   target,
		RepoName: repoName,
	}
	switch otherFlavor {
	case "code":
		gcfg.Type = governance.RepoTypeCode
		gcfg.Stack = "Go" // best-effort; any non-empty stack lets the renderer succeed
	case "doc":
		gcfg.Type = governance.RepoTypeDoc
	default:
		return nil, fmt.Errorf("unknown flavor %q", otherFlavor)
	}
	files, err := governance.RenderCanonicalFiles(tfs, gcfg, target)
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(files))
	for k := range files {
		out[k] = true
	}
	return out, nil
}

// targetGovernanceFilesNotInCanon walks target's docs/ and selected root files,
// returning relpaths that exist in the target but NOT in our canon. If the
// otherCanon map indicates the file IS in the other flavor's canon, the result
// is more useful (suggests flavor mismatch); otherwise it's just a per-repo
// addition the Operator can ignore. We surface only the otherCanon-overlapping
// case to keep the warnings actionable.
func targetGovernanceFilesNotInCanon(target string, ourCanon map[string]string, otherCanon map[string]bool) []string {
	var out []string
	// Walk docs/.
	docsDir := filepath.Join(target, "docs")
	_ = filepath.WalkDir(docsDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(target, p)
		rel = filepath.ToSlash(rel)
		// Skip per-AC files (always per-repo).
		base := filepath.Base(rel)
		if strings.HasPrefix(base, "ac") && strings.HasSuffix(base, ".md") {
			if matched, _ := regexp.MatchString(`^ac\d+-`, base); matched {
				return nil
			}
		}
		if _, inOurs := ourCanon[rel]; inOurs {
			return nil
		}
		if otherCanon[rel] {
			out = append(out, rel)
		}
		return nil
	})
	// Walk selected root files.
	rootEntries, err := os.ReadDir(target)
	if err == nil {
		for _, e := range rootEntries {
			if e.IsDir() {
				continue
			}
			rel := e.Name()
			if _, inOurs := ourCanon[rel]; inOurs {
				continue
			}
			if otherCanon[rel] {
				out = append(out, rel)
			}
		}
	}
	sort.Strings(out)
	return out
}
