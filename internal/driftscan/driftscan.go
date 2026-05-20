// Package driftscan implements the `governa drift-scan` subcommand.
//
// It runs against the current working directory (no positional arguments)
// after a positive governa-adoption check, walks the canon overlay embedded
// in the binary, byte-compares each governed file against the cwd,
// classifies divergences, collects evidence (preserve markers, git log),
// allocates a monotonic AC number, and emits one file under `<cwd>/docs/`:
// the AC stub (`ac<N>-drift-scan-v<X.Y.Z>.md`, conforming to
// `docs/ac-template.md`). The file carries a line-1 HTML emission marker
// (`drift-scan: emitted-by governa v<X.Y.Z>; emission-sha=...`) so re-runs
// against the same canon version can refuse overwrite on a stub the
// Operator has already edited. Per-file diffs are not snapshotted — adopters
// use `governa render-canon` + standard `diff -ru` to inspect changes (see
// AGENTS.md `### Drift-Scan Adoption`). The tool makes no commits and does
// not modify `plan.md`.
//
// The Go package itself has no consumer-overlay counterpart — its source
// lives only here. (The user-facing `docs/drift-scan.md` *does* have
// consumer-overlay counterparts; the source-to-overlay propagation rule
// applies to docs, not to internal/ Go code.) See AC136 for the
// consumer-run reframing.
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

	"github.com/queone/governa/internal/emission"
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

// ReachabilityHeaderReminder is shared verbatim with the "Reachability of canon-only branches" section in docs/drift-scan.md; tests reference this constant directly to enforce byte-equality between both surfaces.
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
	if len(rest) > 0 {
		return cfg, false, fmt.Errorf("drift-scan: no positional arguments accepted; run from the consumer repo root (got: %v)", rest)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return cfg, false, fmt.Errorf("drift-scan: get cwd: %w", err)
	}
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return cfg, false, fmt.Errorf("drift-scan: resolve cwd: %w", err)
	}
	cfg.Target = abs

	cfg.Invocation = "governa drift-scan " + strings.Join(args, " ")

	if cfg.Flavor != "" && cfg.Flavor != "code" && cfg.Flavor != "doc" {
		return cfg, false, fmt.Errorf("drift-scan: --flavor must be code or doc, got %q", cfg.Flavor)
	}

	return cfg, false, nil
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `Usage: governa drift-scan [flags]

Scan an adopted-governa repo against canon. Run from the consumer repo root
(no positional arguments). Emits an AC stub under docs/.

Flags:
  -f, --flavor code|doc      overlay flavor (default: auto-detect)
  -j, --json                 emit JSON report alongside markdown emission
  -l, --diff-lines <N>       diff truncation limit (default: 200)
  -n, --repo-name <name>     override repo name (default: basename of cwd)
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
	if err := emission.RefuseGovernaSource(cfg.Target, "drift-scan"); err != nil {
		return ExitEnvError, err
	}

	// Positive adoption check: cwd must be a governa-adopted repo. AGENTS.md
	// plus one secondary signal (docs/ac-template.md, docs/release.md,
	// docs/build-release.md, or a CHANGELOG row referencing governa apply).
	if err := emission.RequireGovernaAdopted(cfg.Target, "drift-scan"); err != nil {
		return ExitEnvError, err
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
		// emit unified diff against empty canon so the FileResult carries
		// all target lines as `+` for JSON consumers and downstream tooling.
		if targetBytes, rerr := os.ReadFile(filepath.Join(cfg.Target, rel)); rerr == nil {
			fr.Diff = unifiedDiff("", string(targetBytes), rel, cfg.DiffLines)
		}
		report.Files = append(report.Files, fr)
	}

	// name-reference body scan — the second branch of `target-has-no-canon`.
	// Surface target-only files referenced from divergent target files
	// (e.g., rel.sh references ./cmd/rel/color.go which is target-only).
	// Same `target-has-no-canon` classification as the cross-flavor case;
	// shared routing question (keep / delete / migrate-to-canon).
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

	// Determine AC number: reuse existing same-canon-version stub's N, else
	// allocate next monotonic N from <target>/docs/ac*.md + git log AC refs.
	acNum, reused, err := emission.AllocateACNumber(cfg.Target, "drift-scan", sha)
	if err != nil {
		return ExitEnvError, fmt.Errorf("drift-scan: allocate AC number: %w", err)
	}

	stubRel := fmt.Sprintf("docs/ac%d-drift-scan-%s.md", acNum, sha)
	stubPath := filepath.Join(cfg.Target, stubRel)

	// Edit-detection guard: if reused N (same-canon-version stub exists),
	// refuse overwrite when the AC stub has been edited since emission.
	if reused {
		if _, statErr := os.Stat(stubPath); statErr == nil {
			unedited, vErr := emission.VerifyUnedited(stubPath, driftScanMarkerPrefix)
			if vErr != nil {
				return ExitEnvError, fmt.Errorf("drift-scan: verify %s: %w", stubPath, vErr)
			}
			if !unedited {
				return ExitEnvError, fmt.Errorf("drift-scan: %s has been edited since last drift-scan emission — to re-run, commit edits and delete the stub to regenerate, or rename the stub off the drift-scan-%s slug", stubRel, sha)
			}
		}
	}

	// Build AC stub body (without marker).
	stubBody := buildACStub(report, acNum, sha)

	// Ensure <target>/docs/ exists. Adoption check may pass on AGENTS.md +
	// CHANGELOG row alone; docs/ is required for emission.
	if err := emission.EnsureDocsDir(cfg.Target, "drift-scan"); err != nil {
		return ExitEnvError, err
	}

	// Write with emission marker on line 1 (sha covers body only).
	if err := emission.WriteWithMarker(stubPath, driftScanMarkerPrefix, sha, stubBody); err != nil {
		return ExitEnvError, fmt.Errorf("drift-scan: write %s: %w", stubRel, err)
	}

	// Attach emitted paths so JSON consumers can locate the AC stub.
	report.Emitted = &EmittedPaths{ACStub: stubRel}

	if cfg.JSON {
		// JSON mode emits structured scan data alongside the markdown
		// emission. The AC stub itself stays as the markdown source of truth.
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		_ = enc.Encode(report)
	} else {
		fmt.Fprintf(out, "wrote %s (%s)\n", stubRel, tallyClassifications(report.Files))
	}
	return ExitOK, nil
}

// DetectFlavor reports the consumer flavor inferred from target's repo
// shape: "code" if go.mod present, "doc" if _config.yml present (or
// neither), and an error when both are present (ambiguous).
// Exported so cmd/governa's render-canon subcommand reuses the same
// inference logic drift-scan uses.
func DetectFlavor(target string) (string, error) {
	return detectFlavor(target)
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
	// Emitted carries the repo-root-relative path of the AC stub written
	// by this run. Populated only in JSON mode.
	Emitted *EmittedPaths `json:"emitted,omitempty"`
}

// EmittedPaths names the AC stub written under <target>/docs/ on a
// successful run. The path is repo-root-relative.
type EmittedPaths struct {
	ACStub string `json:"ac_stub"`
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
	"arch.md": true,
}

// formatDefiningCanonPaths is the format-defining-files registry (see
// docs/drift-scan.md `## Format-defining files`).
//
// A file belongs in this registry iff its content defines the form of a
// section instantiated in the emitted AC stub — either the form a tool-
// emitted section conforms to, or the form an Operator-fill section
// instantiates (e.g., AGENTS.md defines the AC-template section shape and
// the AT-label convention every AT line carries).
//
// Importance, frequency-of-edit, or being-a-template are not sufficient on
// their own — the file must define a form instantiated in the emitted AC
// stub body.
//
// When any registry file is divergent (any classification other than
// ClassMatch or ClassExpectedDivergence), the file is auto-routed into
// `## In Scope` as a sync action regardless of its raw classification, and
// the emitted AC stub carries a `### Format-defining file routing` sub-
// subsection under `## Summary` naming each one with the rationale. The
// routing question for these files is suppressed; the routing is forced.
//
// Initial registry:
//   - docs/ac-template.md (defines every AC's section shape)
//   - AGENTS.md (AC-template section shape, AT-label convention)
//
// Addition criterion: a future canon file is added to this registry when
// (and only when) it passes the inclusion test above.
var formatDefiningCanonPaths = map[string]bool{
	"docs/ac-template.md": true,
	"AGENTS.md":           true,
}

// isFormatDefining reports whether relpath is in the format-defining registry.
func isFormatDefining(relpath string) bool {
	return formatDefiningCanonPaths[relpath]
}

// CoherenceFailure records a single canon-coherence violation.
type CoherenceFailure struct {
	Rule     string // human-readable rule name (e.g. "AT-label convention")
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
var coherenceRules = []coherenceRule{}

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
// the normal emission stdout summary. H1 is a stable string consumer
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
		// emit unified diff against empty target so the FileResult carries
		// all canon lines as `-` for JSON consumers and downstream tooling.
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
	fr.Markers = emission.PreserveMarkers(cfg.Target, relpath)

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
// back to a placeholder marker if `diff` is unavailable so the emitted AC stub
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

// (Removed under AC136: writeReport had a markdown branch for the
// disposable drift-report-pair format and a JSON branch. The new emission
// path writes the AC stub directly under <target>/docs/, and JSON mode
// encodes the Report struct inline in Run. CleanupReminder and
// AdoptionReminder constants were tied to the markdown branch and removed
// alongside it. AC139 retired the sister-diffs file emission entirely.)

const driftScanMarkerPrefix = "<!-- drift-scan: emitted-by governa "

// classCounts tallies per-classification file counts for the report.
func classCounts(files []FileResult) map[Classification]int {
	counts := map[Classification]int{}
	for _, f := range files {
		counts[f.Classification]++
	}
	return counts
}

// buildACStub renders the emitted AC stub body (without the line-1 marker).
// Conforms to docs/ac-template.md shape minus the copy-instruction preamble.
// Per-file diffs are not snapshotted — adopters re-render canon with
// `governa render-canon` and use standard `diff -ru` to inspect changes
// (see AGENTS.md `### Drift-Scan Adoption`).
func buildACStub(r Report, acNum int, canonVersion string) string {
	var b strings.Builder
	counts := classCounts(r.Files)

	// Route files to sections. Format-defining files override the raw
	// classification: any divergence (anything except match / expected-
	// divergence) routes to In Scope as a sync item, regardless of whether
	// the raw classification would have been preserve, ambiguity, etc.
	// See docs/drift-scan.md `## Format-defining files`.
	var syncEntries, oosEntries, reviewEntries, formatDefiningForced []FileResult
	for _, f := range r.Files {
		if isFormatDefining(f.Relpath) && f.Classification != ClassMatch && f.Classification != ClassExpectedDivergence {
			syncEntries = append(syncEntries, f)
			if f.Classification != ClassClearSync && f.Classification != ClassMissingTarget {
				formatDefiningForced = append(formatDefiningForced, f)
			}
			continue
		}
		switch f.Classification {
		case ClassClearSync, ClassMissingTarget:
			syncEntries = append(syncEntries, f)
		case ClassPreserve, ClassExpectedDivergence:
			oosEntries = append(oosEntries, f)
		case ClassAmbiguity, ClassTargetNoCanon:
			reviewEntries = append(reviewEntries, f)
		}
	}

	fmt.Fprintf(&b, "# AC%d Drift-Scan Adoption from governa %s\n\n", acNum, canonVersion)
	fmt.Fprintf(&b, "Adopt %d canon-owned changes from governa %s; %d entries require routing decisions.\n\n",
		counts[ClassClearSync]+counts[ClassMissingTarget],
		canonVersion,
		counts[ClassAmbiguity]+counts[ClassTargetNoCanon])

	fmt.Fprintln(&b, "## Summary")
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "Sync this repo to governa @ %s canon as part of the recurring drift-scan cycle. Drift-scan surfaced %s. Use `governa render-canon` to render canon and standard `diff -ru` to inspect per-file changes (see AGENTS.md `### Drift-Scan Adoption`).\n\n",
		canonVersion, tallyClassifications(r.Files))

	if r.Header.Flavor == "code" {
		fmt.Fprintln(&b, ReachabilityHeaderReminder)
		fmt.Fprintln(&b)
	}

	if len(formatDefiningForced) > 0 {
		fmt.Fprintln(&b, "### Format-defining file routing")
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, "The following files were routed to `## In Scope` as sync items because they are in the format-defining registry; the raw classification (preserve / ambiguity / etc.) is overridden because the file's content defines the form the AC instantiates:")
		fmt.Fprintln(&b)
		for _, f := range formatDefiningForced {
			fmt.Fprintf(&b, "- `%s` — raw classification: %s; forced to sync.\n", f.Relpath, f.Classification)
		}
		fmt.Fprintln(&b)
	}

	fmt.Fprintln(&b, "### Routing Decisions")
	fmt.Fprintln(&b)
	if len(reviewEntries) == 0 {
		fmt.Fprintln(&b, "`None` — no ambiguities or target-only files surfaced.")
	} else {
		for i, f := range reviewEntries {
			switch f.Classification {
			case ClassAmbiguity:
				fmt.Fprintf(&b, "%d. **`%s`**: local commits diverge from canon — sync to canon, preserve as repo-owned, or pin via preserve marker?\n", i+1, f.Relpath)
			case ClassTargetNoCanon:
				fmt.Fprintf(&b, "%d. **`%s`**: file exists in target but not in canon for this flavor — keep, delete, or migrate to canon?\n", i+1, f.Relpath)
			}
		}
	}
	fmt.Fprintln(&b)

	fmt.Fprintln(&b, "## In Scope")
	fmt.Fprintln(&b)
	if len(syncEntries) == 0 {
		fmt.Fprintln(&b, "No sync items.")
	} else {
		fmt.Fprintln(&b, "Sync to canon:")
		fmt.Fprintln(&b)
		for _, f := range syncEntries {
			suffix := ""
			if isFormatDefining(f.Relpath) {
				suffix = " (format-defining)"
			}
			fmt.Fprintf(&b, "- `%s` — %s%s\n", f.Relpath, f.Classification, suffix)
		}
	}
	fmt.Fprintln(&b)

	fmt.Fprintln(&b, "## Out Of Scope")
	fmt.Fprintln(&b)
	if len(oosEntries) == 0 {
		fmt.Fprintln(&b, "No preserved or expected-divergence entries.")
	} else {
		for _, f := range oosEntries {
			fmt.Fprintf(&b, "- `%s` — %s", f.Relpath, f.Classification)
			if len(f.Markers) > 0 {
				fmt.Fprintf(&b, " (markers: %s)", strings.Join(f.Markers, "; "))
			}
			fmt.Fprintln(&b)
		}
	}
	fmt.Fprintln(&b)

	fmt.Fprintln(&b, "## Acceptance Tests")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "**AT1** [Automated] — Canon-coherence precondition passed (emission was not blocked).")
	fmt.Fprintln(&b)
	for i, f := range syncEntries {
		fmt.Fprintf(&b, "**AT%d** [Automated] — `%s` matches canon byte-for-byte after sync.\n\n", i+2, f.Relpath)
	}
	fmt.Fprintf(&b, "**AT%d** [Automated] — Re-running `governa drift-scan` after this AC's sync produces a new AC stub whose `## In Scope` list does not name any file synced under this AC.\n\n", len(syncEntries)+2)

	fmt.Fprintln(&b, "## Status")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "`PENDING` — drift-scan emission; awaiting Director review and implementation authorization.")
	return b.String()
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

// otherFlavorCanonPaths renders the OTHER-flavor canon (relative to the
// scan's current flavor) and returns a set of repo-relative paths it covers.
// Used by the `target-has-no-canon` cross-flavor branch to detect whether a
// file present in the target only matches the canon of a different flavor
// (e.g., a CODE file showing up in a DOC-flavor target — possible flavor
// misclassification or genuine straddling repo).
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
// second branch of `target-has-no-canon`: name-reference body scan.
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
