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
	"sort"
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

// ReachabilityHeaderReminder is shared verbatim with the "Reachability of
// canon-only branches" section in docs/drift-scan.md; AT1/AT2 reference
// this constant directly to enforce byte-equality between both surfaces.
const ReachabilityHeaderReminder = "Reachability check: verify divergent canon-code branches reach this consumer's structure before treating as drift."

// Config holds drift-scan invocation parameters.
type Config struct {
	Target     string // resolved absolute path to target repo
	Flavor     string // "code" or "doc"
	JSON       bool
	DiffLines  int    // diff truncation limit
	RepoName   string // overrides basename of Target
	Invocation string // exact CLI invocation string for the report header

	// OverrideCanonID bypasses the canon-id derivation (which reads
	// templates.TemplateVersion at runtime). Used in tests to pin a stable,
	// synthetic canon identifier (e.g., "v0.0.0-test"). Production callers
	// leave this empty.
	OverrideCanonID string
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

	// Canon identifier from embedded templates.TemplateVersion (or test
	// override). v-prefix matches the semver tag form. The version-based
	// identifier is invariant across install timing — vcs.revision was prone
	// to drift when binaries got installed during release prep before the
	// release commit was made.
	sha := cfg.OverrideCanonID
	if sha == "" {
		sha = "v" + templates.TemplateVersion
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
		fr := FileResult{
			Relpath:        rel,
			Classification: ClassTargetNoCanon,
			CanonRef:       fmt.Sprintf("(no canon path for flavor %s)", flavor),
		}
		// emit unified diff against empty canon so the file
		// surfaces in drift-report-<sha>-diffs.md with all target lines as `+`.
		if targetBytes, rerr := os.ReadFile(filepath.Join(cfg.Target, rel)); rerr == nil {
			fr.Diff = unifiedDiff("", string(targetBytes), rel, cfg.DiffLines)
		}
		report.Files = append(report.Files, fr)
	}

	// name-reference body scan — the asymmetry note's second
	// branch. Surface target-only files referenced from divergent target
	// files (e.g., rel.sh references ./cmd/rel/color.go which is target-only).
	// Same `target-has-no-canon` classification as the cross-flavor case;
	// shared Director Review Q (keep / delete / migrate-to-canon).
	var divergentForScan []FileResult
	for _, f := range report.Files {
		if isDivergentClass(f.Classification) {
			divergentForScan = append(divergentForScan, f)
		}
	}
	alreadySurfaced := map[string]bool{}
	for _, f := range report.Files {
		if f.Classification == ClassTargetNoCanon {
			alreadySurfaced[f.Relpath] = true
		}
	}
	for _, rel := range nameReferencedTargetOnlyFiles(cfg.Target, divergentForScan, canon, otherCanon, alreadySurfaced) {
		fr := FileResult{
			Relpath:        rel,
			Classification: ClassTargetNoCanon,
			CanonRef:       fmt.Sprintf("(no canon path for flavor %s — name-referenced from a divergent target file)", flavor),
		}
		// emit unified diff against empty canon (all target lines as `+`).
		if targetBytes, rerr := os.ReadFile(filepath.Join(cfg.Target, rel)); rerr == nil {
			fr.Diff = unifiedDiff("", string(targetBytes), rel, cfg.DiffLines)
		}
		report.Files = append(report.Files, fr)
	}

	// emit report-pair files at consumer repo root, no AC
	// staging. Director-set: overwrite silently on existing files (idempotent
	// re-scan). Director-set: minimal one-line stdout summary after emission.
	if cfg.JSON {
		// JSON consumers get the full report on stdout (unchanged behavior
		// for the --json flag; report-pair files still emitted).
		writeReport(out, report, true)
	}
	if err := writeDriftReport(cfg.Target, sha, report); err != nil {
		return ExitEnvError, fmt.Errorf("drift-scan: write drift-report-%s.md: %w", sha, err)
	}
	if err := writeDriftReportDiffs(cfg.Target, sha, report); err != nil {
		return ExitEnvError, fmt.Errorf("drift-scan: write drift-report-%s-diffs.md: %w", sha, err)
	}
	if !cfg.JSON {
		fmt.Fprintf(out, "wrote drift-report-%s.md and drift-report-%s-diffs.md (%s)\n",
			sha, sha, tallyClassifications(report.Files))
	}
	return ExitOK, nil
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
	Header ReportHeader `json:"header"`
	Files  []FileResult `json:"files"`
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
// section INSTANTIATED in the staged AC. Two shapes:
//
//  1. **Tool-emitted form** — canon file content defines the form of a
//     section the staged AC's tool-emitted text instantiates (e.g.,
//     docs/critique-protocol.md defines `## Director Review` round-append
//     structure the tool emits on subsequent rounds).
//
//  2. **Operator-instantiated form** — canon file content defines the form
//     of a section the staged AC's Operator-fill text instantiates (e.g.,
//     AGENTS.md defines the `## Objective Fit` 3-part form the Operator
//     fills, and the AT-label convention every AT line carries).
//
// Both shapes hard-route to sync via the same mechanic. Importance,
// frequency-of-edit, or being-a-template are not sufficient on their own —
// the file must define a form INSTANTIATED in the staged AC body.
//
// When any registry file is divergent (any classification other than
// ClassMatch or ClassExpectedDivergence), the file is auto-routed into
// `## In Scope` as a sync action regardless of its raw classification, and
// the staged AC carries a `### Format-defining file routing` sub-subsection
// under `## Implementation Notes` naming each one with the rationale. The
// Director Review Q for these files is suppressed; the routing is forced.
//
// Initial registry:
//   - docs/ac-template.md (tool-emit + Operator-fill: defines every AC's section shape)
//   - docs/critique-protocol.md (tool-emit: round-append structure + four-field terminator)
//   - AGENTS.md (Operator-fill: Objective Fit 3-part form, AT-label convention)
//
// Addition criterion: a future canon file is added to this registry when
// (and only when) it passes the inclusion test above (either shape).
var formatDefiningCanonPaths = map[string]bool{
	"docs/ac-template.md":       true,
	"docs/critique-protocol.md": true,
	"AGENTS.md":                 true,
}

// isFormatDefining reports whether relpath is in the format-defining registry.
func isFormatDefining(relpath string) bool {
	return formatDefiningCanonPaths[relpath]
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
		// emit unified diff against empty target so the file
		// surfaces in drift-report-<sha>-diffs.md with all canon lines as `-`.
		fr.Diff = unifiedDiff(canon, "", relpath, cfg.DiffLines)
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

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
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
// buildACStub constructs the partially-filled AC stub.

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
// writeReport emits the drift-report file 1 content (header + per-file
// classification list). Diffs live in the sister file emitted by
// writeDriftReportDiffs. JSON mode emits the full Report struct.
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
	fmt.Fprintf(out, "- Counts: %s\n", tallyClassifications(r.Files))
	fmt.Fprintln(out)

	if r.Header.Flavor == "code" {
		fmt.Fprintln(out, ReachabilityHeaderReminder)
		fmt.Fprintln(out)
	}

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
		if isFormatDefining(f.Relpath) {
			fmt.Fprintln(out, "Format-defining: yes")
			fmt.Fprintln(out)
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
	}
}

// writeDriftReport writes the drift-report file 1 to <target>/drift-report-<sha>.md.
// Overwrites any existing file at that path (Director-set Q2: idempotent re-scan).
func writeDriftReport(target, sha string, r Report) error {
	path := filepath.Join(target, "drift-report-"+sha+".md")
	var b strings.Builder
	writeReport(&b, r, false)
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// writeDriftReportDiffs writes the drift-report file 2 to <target>/drift-report-<sha>-diffs.md.
// Per-file H2 section + diff hunk for each divergent file, prefaced with the
// convention stamp. Overwrites any existing file (Director-set Q2).
func writeDriftReportDiffs(target, sha string, r Report) error {
	path := filepath.Join(target, "drift-report-"+sha+"-diffs.md")
	var b strings.Builder
	fmt.Fprintf(&b, "# Drift-Scan Diffs (governa @ %s)\n\n", sha)
	fmt.Fprintln(&b, "Diff convention: `+` lines exist in TARGET; `-` lines exist in CANON. `+` is \"target has this; canon does not\"; `-` is \"canon has this; target does not\".")
	fmt.Fprintln(&b)
	var divergent []FileResult
	for _, f := range r.Files {
		if f.Diff != "" {
			divergent = append(divergent, f)
		}
	}
	sort.Slice(divergent, func(i, j int) bool {
		return divergent[i].Relpath < divergent[j].Relpath
	})
	if len(divergent) == 0 {
		fmt.Fprintln(&b, "No divergent files.")
		return os.WriteFile(path, []byte(b.String()), 0o644)
	}
	for _, f := range divergent {
		fmt.Fprintf(&b, "## `%s`\n\n", f.Relpath)
		canonOnly, targetOnly := computeDirection(f.Diff)
		fmt.Fprintln(&b, formatDirection(canonOnly, targetOnly))
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, "```diff")
		fmt.Fprintln(&b, f.Diff)
		fmt.Fprintln(&b, "```")
		fmt.Fprintln(&b)
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// computeDirection counts target-only (`+`-prefixed) and canon-only
// (`-`-prefixed) lines in a unified-diff string, excluding the `+++ ` and
// `--- ` header lines and `@@ ` hunk headers. input to
// formatDirection for the per-file Direction line in the diffs file.
func computeDirection(diff string) (canonOnly, targetOnly int) {
	for line := range strings.SplitSeq(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "+++"), strings.HasPrefix(line, "---"):
			continue
		case strings.HasPrefix(line, "@@"):
			continue
		case strings.HasPrefix(line, "+"):
			targetOnly++
		case strings.HasPrefix(line, "-"):
			canonOnly++
		}
	}
	return canonOnly, targetOnly
}

// formatDirection returns a one-line natural-language summary of which side
// carries which content. Emitted above each per-file diff hunk in the diffs
// file.
func formatDirection(canonOnly, targetOnly int) string {
	switch {
	case targetOnly > 0 && canonOnly == 0:
		return fmt.Sprintf("Direction: target leads — target carries %d lines absent in canon.", targetOnly)
	case canonOnly > 0 && targetOnly == 0:
		return fmt.Sprintf("Direction: canon leads — canon carries %d lines absent in target.", canonOnly)
	case targetOnly > 0 && canonOnly > 0:
		return fmt.Sprintf("Direction: target carries %d lines absent in canon; canon carries %d lines absent in target.", targetOnly, canonOnly)
	default:
		return "Direction: no line-level divergence detected."
	}
}

// isDivergentClass reports whether c is one of the classifications that
// indicates a per-file divergence requiring routing analysis (preserve,
// ambiguity, clear-sync). Match, expected-divergence, missing-in-target,
// and target-has-no-canon are routed by other paths or skipped entirely.
func isDivergentClass(c Classification) bool {
	return c == ClassPreserve || c == ClassAmbiguity || c == ClassClearSync
}

// EmbeddedFS exposes the templates FS to test callers without exporting the
// templates package elsewhere.
var EmbeddedFS = templates.EmbeddedFS

// buildSisterDiffs constructs the sister-file content carrying full per-file
// diffs. The AC stays a clean decision document; this sister file holds the
// verification material the target-repo Operator needs without re-running
// the scan. Title points back at the parent AC; one `## <relpath>` section
// per divergent file with the verbatim `diff -u` hunk.
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

// nameReferencedPathRe forms the name-reference extraction set: backticked,
// double-quoted, and bare paths after `go run` or `exec` keywords. The
// captured path must start with `.` or `/` (relative or absolute) so we
// don't false-positive on prose tokens like backticked words.
var (
	backtickedPathRe      = regexp.MustCompile("`([./][^`]+)`")
	quotedPathRe          = regexp.MustCompile(`"([./][^"]+)"`)
	goRunOrExecLineTailRe = regexp.MustCompile(`(?m)(?:go run|exec)\s+(.+)$`)
)

// extractPathReferences returns path-like substrings found in content via
// three forms: backticked paths starting with . or /, double-quoted paths,
// and bare path tokens following `go run` or `exec` keywords. Used by the
// name-reference body scan to find target-only files
// referenced from divergent target files.
func extractPathReferences(content string) []string {
	var refs []string
	for _, m := range backtickedPathRe.FindAllStringSubmatch(content, -1) {
		refs = append(refs, m[1])
	}
	for _, m := range quotedPathRe.FindAllStringSubmatch(content, -1) {
		refs = append(refs, m[1])
	}
	for _, m := range goRunOrExecLineTailRe.FindAllStringSubmatch(content, -1) {
		for tok := range strings.FieldsSeq(m[1]) {
			if strings.HasPrefix(tok, "./") || strings.HasPrefix(tok, "/") {
				refs = append(refs, tok)
			}
		}
	}
	return refs
}

// normalizeRefPath resolves a path reference to a target-relative path.
// Absolute paths (leading `/`) are treated as target-rooted (drop the slash);
// `./X` is treated as relative to the referrer's directory; bare relative
// paths are also resolved against the referrer's directory.
func normalizeRefPath(ref, refererRel string) string {
	switch {
	case strings.HasPrefix(ref, "/"):
		return strings.TrimPrefix(ref, "/")
	case strings.HasPrefix(ref, "./"):
		stripped := strings.TrimPrefix(ref, "./")
		refererDir := filepath.Dir(refererRel)
		if refererDir == "." || refererDir == "" {
			return stripped
		}
		return filepath.ToSlash(filepath.Join(refererDir, stripped))
	default:
		refererDir := filepath.Dir(refererRel)
		return filepath.ToSlash(filepath.Join(refererDir, ref))
	}
}

// nameReferencedTargetOnlyFiles scans divergent target files for path
// references to other target files that have no canon counterpart in
// either flavor. Returns the deduplicated, sorted list. Implements the
// asymmetry note's second branch: name-reference body scan.
func nameReferencedTargetOnlyFiles(target string, divergent []FileResult, ourCanon map[string]string, otherCanon map[string]bool, alreadySurfaced map[string]bool) []string {
	found := map[string]bool{}
	for _, f := range divergent {
		content, err := os.ReadFile(filepath.Join(target, f.Relpath))
		if err != nil {
			continue
		}
		for _, ref := range extractPathReferences(string(content)) {
			resolved := normalizeRefPath(ref, f.Relpath)
			if resolved == "" || resolved == f.Relpath {
				continue
			}
			absPath := filepath.Join(target, resolved)
			info, err := os.Stat(absPath)
			if err != nil || info.IsDir() {
				continue
			}
			if _, inOurs := ourCanon[resolved]; inOurs {
				continue
			}
			if otherCanon[resolved] {
				continue
			}
			if alreadySurfaced[resolved] {
				continue
			}
			found[resolved] = true
		}
	}
	var out []string
	for k := range found {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
