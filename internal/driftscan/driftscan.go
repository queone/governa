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
	"crypto/sha256"
	"encoding/hex"
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
// opening and the console report header. AC106 Part C updated the wording:
// directory-sibling enumeration is dropped; files in target with no canon
// counterpart surface via the `target-has-no-canon` classification (when in
// the other flavor's canon) or via name-reference body scan.
const asymmetryNote = "Scan walks canon→target only. Files in target with no canon counterpart surface under `### Files in target without canon` (when present in the other flavor's canon) or via name-reference body scan."

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

	// Canon-coherence precondition (see docs/drift-scan.md
	// `## Canon-coherence precondition`). Runs canon-only, before any
	// target file system access. Hard-fails on any registered rule
	// violation: writes a structured stdout report, exits non-zero, no
	// target writes occur. Enumerate-not-bail — all failures in one
	// report. The check is registry-driven via coherenceRules.
	if failures := checkCanonCoherence(canon); len(failures) > 0 {
		writeCoherenceFailureReport(out, failures)
		return ExitEnvError, nil
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

	// Compute routing groups via the unified coupling rule (Part C, AC106).
	// Directory-sibling enumeration is no longer used as a coupling proxy.
	// The unified rule applies at all depths: Go same-package + shell→binary
	// + name-reference body scan. CoupledLocalOnly is left empty;
	// `Coupled-with:` annotations in the staged AC are derived from
	// RoutingGroups instead.
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

// expectedDivergencePaths is the per-repo stub registry (see docs/drift-scan.md
// `## Expected-divergence registry`). Files in this registry are per-repo by
// design — canon ships a stub, adopted repos fill it; byte-compare always
// diverges. The tool skips the byte-compare and classifies these as
// ClassExpectedDivergence so the AC stub renders them in their own subsection
// instead of misleading the Operator with a "match" reading. Future per-repo
// stubs are added here in the same code change that introduces them.
var expectedDivergencePaths = map[string]bool{
	"plan.md": true,
}

// formatDefiningCanonPaths is the format-defining-files registry (see
// docs/drift-scan.md `## Format-defining files`).
//
// A file belongs in this registry iff its content defines the form of a
// section the staged AC itself emits — i.e., divergence in the file would
// make the staged AC's own text contradict canon's specification of that
// form. Importance, frequency-of-edit, or being-a-template are not
// sufficient on their own.
//
// When any registry file is divergent (any classification other than
// ClassMatch or ClassExpectedDivergence), the file is auto-routed into
// `## In Scope` as a sync action regardless of its raw classification, and
// the staged AC carries a `### Format-defining file routing` sub-subsection
// under `## Implementation Notes` naming each one with the rationale. The
// Director Review Q for these files is suppressed; the routing is forced.
//
// Initial registry: docs/ac-template.md (defines the shape of every AC's
// sections), docs/critique-protocol.md (defines round-append structure and
// the four-field terminator the AC uses on later rounds).
//
// Addition criterion: a future canon file is added to this registry when
// (and only when) it passes the inclusion test above.
var formatDefiningCanonPaths = map[string]bool{
	"docs/ac-template.md":       true,
	"docs/critique-protocol.md": true,
}

// isFormatDefining reports whether relpath is in the format-defining registry.
func isFormatDefining(relpath string) bool {
	return formatDefiningCanonPaths[relpath]
}

// governaOnlyPathPrefixes is the registry of path prefixes that exist ONLY
// in governa, not in any consumer overlay. Tool-emitted text into the staged
// consumer AC must qualify any reference to one of these paths via
// qualifyGovernaPath; bare references resolve in governa but break in the
// consumer (see docs/drift-scan.md `## Reference qualification`).
//
// Paths shared with consumer overlays (docs/ac-template.md,
// docs/critique-protocol.md, AGENTS.md, CHANGELOG.md, etc.) stay out of this
// registry — those references are target-relative when emitted into the
// staged AC and must NOT be qualified.
//
// AC107 AT2 walks the staged body and trips on any unqualified governa-only
// path; forgetting qualifyGovernaPath at a future emission site fails the
// test on the first run. Adding a future governa-only prefix to this
// registry extends coverage without rewriting the test.
var governaOnlyPathPrefixes = []string{
	"docs/drift-scan.md",
	"docs/development-cycle.md",
	"docs/development-guidelines.md",
	"docs/build-release.md",
	"internal/",
	"cmd/governa/",
}

// isGovernaOnlyPath reports whether relpath has a prefix in
// governaOnlyPathPrefixes — i.e. references to it must be qualified when
// emitted into a consumer artifact.
func isGovernaOnlyPath(relpath string) bool {
	for _, prefix := range governaOnlyPathPrefixes {
		if strings.HasPrefix(relpath, prefix) {
			return true
		}
	}
	return false
}

// qualifyGovernaPath returns the qualified form `governa @ <sha>: <path>`
// used in tool-emitted text inside the staged consumer AC. Use this at
// every emission site that references a governa-only path so the consumer
// reader can resolve the reference (see docs/drift-scan.md
// `## Reference qualification`).
func qualifyGovernaPath(sha, path string) string {
	return "governa @ " + sha + ": " + path
}

// CoherenceFailure records a single canon-coherence violation.
type CoherenceFailure struct {
	Rule     string // human-readable rule name (e.g. "Objective Fit form")
	Path     string // canon path where the failure was detected
	Expected string // regex source string the path was expected to match
	Preview  string // first lines of the path's content for context
}

// coherenceConformant names a canon path and the regex that path's content
// must match for the rule to hold.
type coherenceConformant struct {
	Path  string
	Regex *regexp.Regexp
}

// coherenceRule defines a cross-file canon-coherence rule. AGENTS.md is the
// authoritative source per AGENTS.md `## Governed Sections`; the rule's
// AuthorityPath should be `AGENTS.md` (or its overlay equivalent rendered
// into canon as `AGENTS.md`). Conformants are other canon paths that must
// instantiate the rule consistently.
type coherenceRule struct {
	Name           string
	AuthorityPath  string
	AuthorityRegex *regexp.Regexp
	Conformants    []coherenceConformant
}

// coherenceRules is the registry of cross-file canon-coherence rules
// checked by the Canon-coherence precondition (see docs/drift-scan.md
// `## Canon-coherence precondition`). Each rule names an authority and
// the conformants that must instantiate it. Future rules are added here.
//
// AGENTS.md is named authoritative per the clause added to `## Governed
// Sections` in AGENTS.md. When canon-internal drift is detected on any
// rule below, drift-scan hard-fails before the canon→target walk and
// emits a structured stdout report — no target writes occur.
var coherenceRules = []coherenceRule{
	{
		Name:           "Objective Fit form",
		AuthorityPath:  "AGENTS.md",
		AuthorityRegex: regexp.MustCompile(`\*\*Outcome\*\* — what this delivers, in one sentence`),
		Conformants: []coherenceConformant{
			{
				Path:  "docs/ac-template.md",
				Regex: regexp.MustCompile(`\*\*Outcome\.\*\* What this delivers, in one sentence\.`),
			},
		},
	},
}

// checkCanonCoherence walks coherenceRules and returns failures. Empty
// return means canon is internally coherent on all registered rules.
// Runs canon-only — does not read the target.
func checkCanonCoherence(canon map[string]string) []CoherenceFailure {
	var failures []CoherenceFailure
	for _, rule := range coherenceRules {
		// Authority site.
		sites := []coherenceConformant{
			{Path: rule.AuthorityPath, Regex: rule.AuthorityRegex},
		}
		sites = append(sites, rule.Conformants...)
		for _, site := range sites {
			content, ok := canon[site.Path]
			if !ok {
				failures = append(failures, CoherenceFailure{
					Rule:     rule.Name,
					Path:     site.Path,
					Expected: site.Regex.String(),
					Preview:  "[file not in canon for this flavor]",
				})
				continue
			}
			if !site.Regex.MatchString(content) {
				failures = append(failures, CoherenceFailure{
					Rule:     rule.Name,
					Path:     site.Path,
					Expected: site.Regex.String(),
					Preview:  previewCanonContent(content, 6),
				})
			}
		}
	}
	return failures
}

// writeCoherenceFailureReport writes the hard-fail report to out. Replaces
// the normal staged-AC summary on stdout. H1 is a stable string consumer
// agents can route on. Per docs/drift-scan.md `## Canon-coherence
// precondition`: governa-side framing, enumerate-not-bail (all failures in
// one report), no target writes occurred before this was called.
func writeCoherenceFailureReport(out io.Writer, failures []CoherenceFailure) {
	fmt.Fprintln(out, "# Canon-Coherence Precondition Failed")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "This is a **governa-side** defect requiring canon reconciliation. Consumer Director's action is \"ping governa maintainer,\" not \"route a divergence.\" Drift-scan refused to emit; no files were staged in the target.")
	fmt.Fprintln(out)

	byRule := map[string][]CoherenceFailure{}
	var ruleOrder []string
	for _, f := range failures {
		if _, ok := byRule[f.Rule]; !ok {
			ruleOrder = append(ruleOrder, f.Rule)
		}
		byRule[f.Rule] = append(byRule[f.Rule], f)
	}

	for _, rule := range ruleOrder {
		fmt.Fprintf(out, "## Rule: %s\n\n", rule)
		fmt.Fprintln(out, "**Authoritative source:** `AGENTS.md` per the `## Governed Sections` clause.")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "**Conflicting sites:**")
		fmt.Fprintln(out)
		for _, f := range byRule[rule] {
			fmt.Fprintf(out, "- `%s` — expected canonical pattern `%s` not found. First lines of canon content:\n\n", f.Path, f.Expected)
			fmt.Fprintln(out, "  ```")
			for line := range strings.SplitSeq(f.Preview, "\n") {
				fmt.Fprintln(out, "  "+line)
			}
			fmt.Fprintln(out, "  ```")
			fmt.Fprintln(out)
		}
	}
	fmt.Fprintln(out, "Reconcile canon-side and re-run drift-scan.")
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

	clearSyncRaw := filterByClass(report.Files, ClassClearSync)
	preservesRaw := filterByClass(report.Files, ClassPreserve)
	ambiguitiesRaw := filterByClass(report.Files, ClassAmbiguity)
	matches := filterByClass(report.Files, ClassMatch)
	expectedDiv := filterByClass(report.Files, ClassExpectedDivergence)
	missing := filterByClass(report.Files, ClassMissingTarget)
	noCanon := filterByClass(report.Files, ClassTargetNoCanon)

	// Format-defining hard-route (Part A, AC106): files in
	// formatDefiningCanonPaths registry that are divergent get auto-routed
	// to ## In Scope as sync, regardless of raw classification. They are
	// suppressed from preserves/ambiguities/clearSync slices and from the
	// Director Review Q list. The `### Format-defining file routing`
	// sub-subsection under ## Implementation Notes names each one with
	// the rationale.
	var formatDefining []FileResult
	var clearSync, preserves, ambiguities []FileResult
	for _, f := range clearSyncRaw {
		if isFormatDefining(f.Relpath) {
			formatDefining = append(formatDefining, f)
		} else {
			clearSync = append(clearSync, f)
		}
	}
	for _, f := range preservesRaw {
		if isFormatDefining(f.Relpath) {
			formatDefining = append(formatDefining, f)
		} else {
			preserves = append(preserves, f)
		}
	}
	for _, f := range ambiguitiesRaw {
		if isFormatDefining(f.Relpath) {
			formatDefining = append(formatDefining, f)
		} else {
			ambiguities = append(ambiguities, f)
		}
	}
	sort.Slice(formatDefining, func(i, j int) bool {
		return formatDefining[i].Relpath < formatDefining[j].Relpath
	})

	// Build per-file coupledWith map from routing groups (Part B/C).
	// Sourced from report.RoutingGroups so when Part C's coupling-rule
	// rewrite lands, this map automatically reflects build-relationship
	// + name-reference signals instead of directory-sibling enumeration.
	coupledWith := map[string][]string{}
	for _, g := range report.RoutingGroups {
		for _, rel := range g {
			for _, other := range g {
				if other == rel {
					continue
				}
				coupledWith[rel] = append(coupledWith[rel], other)
			}
		}
	}
	for k := range coupledWith {
		sort.Strings(coupledWith[k])
	}

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

	inScopeEmpty := len(clearSync) == 0 && len(missingCreate) == 0 && len(formatDefining) == 0

	// Director Review open-Q count: one Q per ambiguity + one per
	// target-has-no-canon. Format-defining files are suppressed (hard-routed
	// to In Scope as sync). Used below to decide whether to emit the In
	// Scope header note (class B: discipline against staleness).
	directorReviewOpenQs := len(ambiguities) + len(noCanon)

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

	// In Scope: clear-sync files + missing-in-target with non-empty canon
	// + format-defining-divergent files (Part A: hard-routed to sync).
	// Header note (class B) is prepended when Director Review has open Qs;
	// when In Scope body is otherwise None and there are open Qs, the
	// header note replaces None as the body.
	fmt.Fprintln(&b, "## In Scope")
	fmt.Fprintln(&b)
	headerNote := "_In Scope expands as Director resolves Q1–Q" + strconv.Itoa(directorReviewOpenQs) + ". Sync resolutions add a sync line here; preserve resolutions add a CHANGELOG marker-backfill line here at the same time. See `" + qualifyGovernaPath(sha, "docs/drift-scan.md") + " ## Resolution protocol`._"
	switch {
	case inScopeEmpty && directorReviewOpenQs > 0:
		// Header note replaces None body.
		fmt.Fprintln(&b, headerNote)
	case inScopeEmpty:
		fmt.Fprintln(&b, "None.")
	default:
		if directorReviewOpenQs > 0 {
			fmt.Fprintln(&b, headerNote)
			fmt.Fprintln(&b)
		}
		for _, f := range formatDefining {
			fmt.Fprintf(&b, "- `%s` — sync to canon (format-defining; hard-routed per `%s ## Format-defining files`)\n", f.Relpath, qualifyGovernaPath(sha, "docs/drift-scan.md"))
		}
		for _, f := range clearSync {
			fmt.Fprintf(&b, "- `%s` — sync to canon\n", f.Relpath)
		}
		for _, f := range missingCreate {
			fmt.Fprintf(&b, "- `%s` — create from canon\n", f.Relpath)
		}
	}
	fmt.Fprintln(&b)

	// Out Of Scope (with class-H follow-on header note per AC107: scaffolds
	// the defer-resolution landing convention when Director Review has open
	// Qs, mirroring AC106's In Scope header note for sync/preserve).
	fmt.Fprintln(&b, "## Out Of Scope")
	fmt.Fprintln(&b)
	ooHeaderNote := "_Defer resolutions add a bullet here naming the file and the follow-on AC pointer added to plan.md as a new IE. See `" + qualifyGovernaPath(sha, "docs/drift-scan.md") + " ## Resolution protocol`._"
	ooEmpty := len(preserves) == 0
	switch {
	case ooEmpty && directorReviewOpenQs > 0:
		fmt.Fprintln(&b, ooHeaderNote)
	case ooEmpty:
		fmt.Fprintln(&b, "None.")
	default:
		if directorReviewOpenQs > 0 {
			fmt.Fprintln(&b, ooHeaderNote)
			fmt.Fprintln(&b)
		}
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

	// Format-defining file routing (Part A). Emitted when any registry
	// file is divergent. Names each one with rationale so the Operator
	// sees why these files are routed differently from raw classification.
	if len(formatDefining) > 0 {
		fmt.Fprintln(&b, "### Format-defining file routing")
		fmt.Fprintln(&b)
		fmt.Fprintf(&b, "The following files are in the format-defining-canon registry (`%s ## Format-defining files`). They are auto-routed to `## In Scope` as sync regardless of raw classification, because the staged AC's auto-emitted form already adopts canon's form for these files; routing them as anything other than sync would leave the AC self-contradictory. Director Review questions for these files are suppressed.\n", qualifyGovernaPath(sha, "docs/drift-scan.md"))
		fmt.Fprintln(&b)
		for _, f := range formatDefining {
			fmt.Fprintf(&b, "- `%s` — raw classification: %s; auto-routed to In Scope as sync.\n", f.Relpath, f.Classification)
		}
		fmt.Fprintln(&b)
	}

	// Routing summary table (Part B). First sub-subsection of Implementation
	// Notes — gives the reader the routing decision surface up front. Tool
	// fills File and Local edit source from the most-recent commit subject;
	// Operator fills the one-line characterization and lean. Column reads
	// `Operator lean (as of staging)` (not `Recommendation`) and is preceded
	// by a staging-time stamp, per class-B discipline: Operator-seeded
	// surfaces that the Director consumes after critique either don't
	// duplicate resolved state or carry an explicit "as of staging" marker.
	// Format-defining files (hard-routed) are excluded — their routing is
	// not a Director decision.
	divergent := []FileResult{}
	divergent = append(divergent, preserves...)
	divergent = append(divergent, ambiguities...)
	divergent = append(divergent, clearSync...)
	if len(divergent) > 0 {
		fmt.Fprintln(&b, "### Routing summary")
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, "_Operator lean below reflects staging-time analysis. Director-resolved routing lives in the Director Review section below; this table does not auto-update on resolution._")
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, "| File | Local edit source | What diverged | Operator lean (as of staging) |")
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
	// was declared earlier for the routing summary table. The Coupled-with
	// line draws from coupledWith (built from report.RoutingGroups) and
	// stays informational — routing decisions are per-file in
	// `## Director Review`, not driven by coupling.
	if len(divergent) > 0 {
		fmt.Fprintln(&b, "### Divergent files")
		fmt.Fprintln(&b)
		for _, f := range divergent {
			fmt.Fprintf(&b, "#### `%s` — %s\n\n", f.Relpath, f.Classification)
			fmt.Fprintf(&b, "Canon: %s\n\n", f.CanonRef)
			if cw := coupledWith[f.Relpath]; len(cw) > 0 {
				fmt.Fprintf(&b, "Coupled-with: %s\n\n", strings.Join(quoteAll(cw), ", "))
			} else {
				fmt.Fprintln(&b, "Coupled-with: None")
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

	// Files in target without canon (Part B: extended with content preview
	// and other-flavor canon path so the Director can decide keep / delete
	// / migrate-to-canon without leaving the AC). Each file also gets a
	// Director Review Q (see ## Director Review below).
	if len(noCanon) > 0 {
		otherFlavor := "doc"
		if report.Header.Flavor == "doc" {
			otherFlavor = "code"
		}
		fmt.Fprintln(&b, "### Files in target without canon")
		fmt.Fprintln(&b)
		fmt.Fprintf(&b, "These files exist in the target but NOT in canon for flavor `%s`. They DO appear in the other flavor's canon (`%s`) — the Director should confirm flavor selection, accept these as legitimate per-repo additions, or migrate them into canon. Each file gets a Director Review Q below.\n", report.Header.Flavor, otherFlavor)
		fmt.Fprintln(&b)
		for _, f := range noCanon {
			fmt.Fprintf(&b, "#### `%s` — target-has-no-canon\n\n", f.Relpath)
			fmt.Fprintf(&b, "Other-flavor canon path: `internal/templates/overlays/%s/files/%s`\n\n", otherFlavor, f.Relpath)
			// Best-effort content preview (head + tail). Skip silently on
			// read errors — the file is informational, not blocking.
			if data, err := os.ReadFile(filepath.Join(report.Header.Target, f.Relpath)); err == nil {
				fmt.Fprintln(&b, "Target content (preview):")
				fmt.Fprintln(&b)
				fmt.Fprintln(&b, "```")
				fmt.Fprintln(&b, previewCanonContent(string(data), 20))
				fmt.Fprintln(&b, "```")
				fmt.Fprintln(&b)
			}
		}
	}

	// Coupled sets reading aid (Part B, Q4 Director-set: descriptive-not-
	// prescriptive). Surfaces the coupling graph informationally — routing
	// decisions are per-file in the Director Review questions above. The
	// heading qualifier and lead-in stamp are part of the spec; class-G
	// negative-regex test enforces no prescriptive language survives.
	if len(report.RoutingGroups) > 0 {
		// Filter to groups with at least 2 files; singletons aren't coupled.
		var multi [][]string
		for _, g := range report.RoutingGroups {
			if len(g) >= 2 {
				multi = append(multi, g)
			}
		}
		if len(multi) > 0 {
			fmt.Fprintln(&b, "### Coupled sets (informational — routing decisions per Q above)")
			fmt.Fprintln(&b)
			fmt.Fprintln(&b, "_The list below names files linked by build-relationship or name-reference signal. It is informational. Routing decisions are made per-file in the Director Review questions above._")
			fmt.Fprintln(&b)
			for _, g := range multi {
				signal := classifyCouplingSignal(g, report.Header.Target)
				fmt.Fprintf(&b, "- %s: %s\n", signal, strings.Join(quoteAll(g), ", "))
			}
			fmt.Fprintln(&b)
		}
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

	// Acceptance Tests (Part B: tool emits ATs only for In Scope deliverables;
	// missing-in-target ATs are byte-equality against canon content per AC106
	// to avoid token-only checks that pass while most canonical lines are
	// missing). Preserve-marker ATs and the IE-pointer AT were dropped —
	// they verified scaffolding placed by earlier ACs / by this scan's
	// staging step, not this AC's deliverable.
	fmt.Fprintln(&b, "## Acceptance Tests")
	fmt.Fprintln(&b)
	atHeaderNote := "_Each sync resolution adds a paired byte-equality AT here. See `" + qualifyGovernaPath(sha, "docs/drift-scan.md") + " ## Resolution protocol`._"
	atEmpty := len(clearSync) == 0 && len(missingCreate) == 0 && len(formatDefining) == 0
	switch {
	case atEmpty && directorReviewOpenQs > 0:
		fmt.Fprintln(&b, atHeaderNote)
		fmt.Fprintln(&b)
	case atEmpty:
		fmt.Fprintln(&b, "None — this AC ships only the staged plan.md IE entry; nothing to verify in target.")
		fmt.Fprintln(&b)
	default:
		if directorReviewOpenQs > 0 {
			fmt.Fprintln(&b, atHeaderNote)
			fmt.Fprintln(&b)
		}
		atN := 1
		for _, f := range formatDefining {
			fmt.Fprintf(&b, "**AT%d** [Automated] — `%s` synced to canon (format-defining hard-route). %s\n\n", atN, f.Relpath, byteEqualityCheck(f.Relpath, f.CanonContent))
			atN++
		}
		for _, f := range clearSync {
			fmt.Fprintf(&b, "**AT%d** [Automated] — `%s` synced to canon. %s\n\n", atN, f.Relpath, byteEqualityCheck(f.Relpath, f.CanonContent))
			atN++
		}
		for _, f := range missingCreate {
			fmt.Fprintf(&b, "**AT%d** [Automated] — `%s` created from canon. %s\n\n", atN, f.Relpath, byteEqualityCheck(f.Relpath, f.CanonContent))
			atN++
		}
	}

	// Documentation Updates
	fmt.Fprintln(&b, "## Documentation Updates")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "- `CHANGELOG.md` — release row added at release prep time per template guidance.")
	fmt.Fprintln(&b)

	// Director Review (Part B / class G+I): Q-per-file emission. One
	// numbered Q per ambiguity file plus one per target-has-no-canon
	// file. Format-defining files are suppressed (hard-routed to In
	// Scope). Coupling is informational via `Coupled-with:` annotation
	// in the Q text — no "must route together" or any other routing
	// constraint claim survives. Class-G negative-regex test enforces
	// no prescriptive language across the full staged-AC body.
	fmt.Fprintln(&b, "## Director Review")
	fmt.Fprintln(&b)
	totalQs := len(ambiguities) + len(noCanon)
	if totalQs == 0 {
		fmt.Fprintln(&b, "None.")
	} else {
		qN := 1
		for _, f := range ambiguities {
			coupled := coupledWith[f.Relpath]
			coupledClause := ""
			if len(coupled) > 0 {
				coupledClause = fmt.Sprintf(" Coupled-with: %s.", strings.Join(quoteAll(coupled), ", "))
			}
			fmt.Fprintf(&b,
				"%d. Should `%s` be synced to canon, preserved with a marker (backfill `preserve %s <qualifier>` in CHANGELOG.md), or deferred to a later AC?%s Operator lean: <!-- TBD by Operator -->. Why: <!-- TBD by Operator -->.\n",
				qN, f.Relpath, f.Relpath, coupledClause,
			)
			qN++
		}
		for _, f := range noCanon {
			coupled := coupledWith[f.Relpath]
			coupledClause := ""
			if len(coupled) > 0 {
				coupledClause = fmt.Sprintf(" Coupled-with: %s.", strings.Join(quoteAll(coupled), ", "))
			}
			fmt.Fprintf(&b,
				"%d. Should `%s` (target-has-no-canon) be kept as a per-repo addition, deleted, or migrated into canon (governa-side AC)?%s Operator lean: <!-- TBD by Operator -->. Why: <!-- TBD by Operator -->.\n",
				qN, f.Relpath, coupledClause,
			)
			qN++
		}
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

// byteEqualityCheck returns a one-line shell command that verifies
// <relpath>'s byte-content matches canon. Uses SHA-256 against the
// canon-side hash computed at staging time — content embedding is
// avoided because canon files like ac-template.md contain `## Director
// Review` and similar headings that bleed into the staged AC's
// structure when embedded literally (and inflate AC size for any
// large canon file). POSIX-safe: `shasum -a 256` is on macOS and
// most Linux distros via coreutils; `awk '{print $1}'` strips the
// trailing filename portion of the shasum output.
func byteEqualityCheck(relpath, canonContent string) string {
	h := sha256.Sum256([]byte(canonContent))
	sum := hex.EncodeToString(h[:])
	return fmt.Sprintf("Verify SHA-256: `[ \"$(shasum -a 256 %s | awk '{print $1}')\" = \"%s\" ]` (canon SHA-256: `%s`).", relpath, sum, sum)
}

// quoteAll wraps each input string in backticks. Used for rendering
// relpath lists in markdown so each path is inline-coded.
func quoteAll(items []string) []string {
	out := make([]string, len(items))
	for i, item := range items {
		out[i] = "`" + item + "`"
	}
	return out
}

// classifyCouplingSignal names which coupling signal produced a routing
// group. Used for the descriptive-not-prescriptive `### Coupled sets`
// reading aid (Q4 Director-set). Returns one of:
//   - "Shell→binary" — group contains a *.sh whose `go run` resolves
//     to another file in the group
//   - "Go same-package" — every file in the group ends in .go
//   - "Name-reference" — fallback when build-relationship signals don't fire
//
// This helper is informational only; routing decisions are per-file in
// the Director Review questions, not driven by signal classification.
func classifyCouplingSignal(group []string, targetRoot string) string {
	for _, rel := range group {
		if !strings.HasSuffix(rel, ".sh") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(targetRoot, rel))
		if err != nil {
			continue
		}
		if shellGoRunRe.MatchString(string(content)) {
			return "Shell→binary"
		}
	}
	allGo := true
	for _, rel := range group {
		if !strings.HasSuffix(rel, ".go") {
			allGo = false
			break
		}
	}
	if allGo {
		return "Go same-package"
	}
	return "Name-reference"
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

// (Removed: enumerateLocalOnlySiblings + helpers per AC106 Part C.
// Directory-sibling enumeration is no longer used as a coupling proxy;
// see computeRoutingGroups for the unified rule.)

// shellGoRunRe matches `go run <arg>` invocations in shell scripts. Used
// for the coarse shell→binary coupling pass: scripts that `go run` a
// package containing another divergent file route together with that
// file. Single regex pass over `*.sh` only — broader scanning of
// `bash -c`, Makefile recipes, and `*.go` build directives is deferred
// until a concrete failure case shows up.
var shellGoRunRe = regexp.MustCompile(`go\s+run\s+(\S+)`)

// goPackageRe extracts the package name from a Go file's package
// declaration. Used by computeRoutingGroups to detect Go same-package
// coupling without a full Go parse.
var goPackageRe = regexp.MustCompile(`(?m)^package\s+(\w+)`)

// goPackageOfFile returns the Go package name declared in <relpath>.
// Returns empty string if the file is not Go, can't be read, or has
// no readable package declaration.
func goPackageOfFile(targetRoot, relpath string) string {
	if !strings.HasSuffix(relpath, ".go") {
		return ""
	}
	content, err := os.ReadFile(filepath.Join(targetRoot, relpath))
	if err != nil {
		return ""
	}
	m := goPackageRe.FindStringSubmatch(string(content))
	if m == nil {
		return ""
	}
	return m[1]
}

// computeRoutingGroups clusters divergent files into routing groups via
// the unified coupling rule (Part C, AC106). Directory-sibling enumeration
// is no longer used. Two files are in the same group iff:
//
//   - **Build-relationship signal:** Go same-package — both files end in
//     .go, sit in the same directory, and declare the same `package X`;
//     OR Shell→binary — a *.sh script's `go run <pkg>` resolves to a
//     directory containing the other file.
//   - **Name-reference signal:** F's content references G's repo-relative
//     path or basename (substring match, no extension stripping).
//
// Applied uniformly at all depths — repo root, subdirectories, anywhere.
// Returns groups as slices of relpaths, sorted by first-relpath for
// stable rendering. Non-divergent files are excluded. Used by the AC
// stub to populate `Coupled-with:` annotations and the `### Coupled
// sets` reading aid (informational; routing decisions are per-file).
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

	// Edge 1: Go same-package. Two .go files in the same directory with
	// the same `package X` declaration are coupled. Go enforces packages
	// are directory-bound, so this is more precise than name-equality
	// alone — it requires both the directory match and the parsed package
	// name match.
	type goKey struct{ dir, pkg string }
	goPackages := map[goKey][]int{}
	for i, f := range divergent {
		pkg := goPackageOfFile(targetRoot, f.Relpath)
		if pkg == "" {
			continue
		}
		k := goKey{dir: filepath.Dir(f.Relpath), pkg: pkg}
		goPackages[k] = append(goPackages[k], i)
	}
	for _, indices := range goPackages {
		for i := 1; i < len(indices); i++ {
			union(indices[0], indices[i])
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

	// Edge 3: name-reference body scan. F's content references G's
	// repo-relative path or basename (substring match). False-positives
	// (e.g., README.md mentions index.md in passing) are accepted as a
	// heuristic limitation — the registry-driven negative-regex test
	// (class G) catches prescriptive language regardless of which signal
	// produced the coupling.
	contents := make(map[int]string, n)
	for i, f := range divergent {
		c, err := os.ReadFile(filepath.Join(targetRoot, f.Relpath))
		if err != nil {
			continue
		}
		contents[i] = string(c)
	}
	for i, content := range contents {
		for j, fj := range divergent {
			if i == j {
				continue
			}
			base := filepath.Base(fj.Relpath)
			if strings.Contains(content, fj.Relpath) || strings.Contains(content, base) {
				union(i, j)
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
