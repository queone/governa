package governance

import (
	"bufio"
	"cmp"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/queone/governa/internal/color"
	"github.com/queone/governa/internal/templates"
)

type Mode string

const (
	ModeSync    Mode = "sync"
	ModeEnhance Mode = "enhance"
)

type RepoType string

const (
	RepoTypeCode RepoType = "CODE"
	RepoTypeDoc  RepoType = "DOC"
)

// ErrConflictsPresent is returned by runSync when sync completes but one or
// more conflicts (e.g., symlink-vs-regular-file collisions) were detected.
// Conflicts have already been printed to stderr and recorded in the review
// doc before this error is returned; the error exists purely so scripted
// callers can distinguish "sync completed cleanly" (exit 0) from "sync
// completed with manual-resolution blockers" (non-zero exit).
var ErrConflictsPresent = errors.New("sync completed with conflicts requiring manual resolution")

var governedSectionNames = []string{
	"Purpose",
	"Governed Sections",
	"Interaction Mode",
	"Approval Boundaries",
	"Review Style",
	"File-Change Discipline",
	"Release Or Publish Triggers",
	"Documentation Update Expectations",
	"Project Rules",
}

type Config struct {
	Mode               Mode
	Target             string
	Reference          string
	Type               RepoType
	RepoName           string
	Purpose            string
	Stack              string
	PublishingPlatform string
	Style              string
	InitGit            bool
	DryRun             bool
	Input              io.Reader // interactive prompt source; nil defaults to os.Stdin
}

type Assessment struct {
	RepoShape          string
	ResolvedType       RepoType // type used to compute expected artifacts; resolved from RepoShape when caller passed ""
	ExistingArtifacts  []string
	CollisionRisk      string
	Recommendation     string
	CodeSignals        int
	DocSignals         int
	CollidingArtifacts []string
}

// deriveTypeFromShape maps RepoShape → RepoType. Returns "" when shape is
// ambiguous (mixed/unclear/empty) so callers can prompt or default.
func deriveTypeFromShape(shape string) RepoType {
	switch shape {
	case "likely CODE":
		return RepoTypeCode
	case "likely DOC":
		return RepoTypeDoc
	}
	return ""
}

type EnhancementCandidate struct {
	Area            string
	Path            string
	Section         string
	Disposition     string
	Reason          string
	Portability     string
	TemplateTarget  string
	Summary         string
	CollisionImpact string
	DeltaSections   []string
	ChangeOrigin    string
}

type EnhancementReport struct {
	ReferenceRoot string
	Candidates    []EnhancementCandidate
}

type operation struct {
	kind    string
	path    string
	note    string
	content string
	linkTo  string
	source  string
}

// conflict represents a pre-apply condition that blocks a planned operation
// from landing safely. Conflicts are surfaced in the review doc under a
// ## Conflicts section and trigger a post-transform `needs manual resolution`
// status line. They are distinct from collision scores (which represent
// existing-vs-proposed content differences).
type conflict struct {
	kind        string // "symlink-vs-regular"
	path        string // absolute path of the blocked op target
	description string // operator-facing explanation including required action
}

type flagValues struct {
	target             string
	reference          string
	repoType           string
	repoName           string
	purpose            string
	stack              string
	publishingPlatform string
	style              string
	initGit            bool
	dryRun             bool
}

func RunWithFS(tfs fs.FS, repoRoot string, cfg Config) error {
	switch cfg.Mode {
	case ModeSync:
		return runSync(tfs, repoRoot, cfg)
	case ModeEnhance:
		return RunEnhance(tfs, repoRoot, cfg)
	default:
		return fmt.Errorf("unsupported mode %q", cfg.Mode)
	}
}

// ParseModeArgs parses flags for a given mode without the -m flag.
// Used by cmd/governa where the mode is determined by the subcommand.
func ParseModeArgs(mode Mode, args []string) (Config, bool, error) {
	return parseFlags(mode, args)
}

func parseFlags(mode Mode, args []string) (Config, bool, error) {
	values := flagValues{}
	fset := flag.NewFlagSet("governa", flag.ContinueOnError)
	fset.SetOutput(os.Stderr)
	fset.StringVar(&values.target, "t", "", "target directory")
	fset.StringVar(&values.target, "target", "", "target directory")
	fset.StringVar(&values.reference, "r", "", "reference repo for enhance")
	fset.StringVar(&values.reference, "reference", "", "reference repo for enhance")
	fset.StringVar(&values.repoType, "y", "", "repo type: CODE|DOC")
	fset.StringVar(&values.repoType, "type", "", "repo type: CODE|DOC")
	fset.StringVar(&values.repoName, "n", "", "repo name")
	fset.StringVar(&values.repoName, "repo-name", "", "repo name")
	fset.StringVar(&values.purpose, "p", "", "project purpose")
	fset.StringVar(&values.purpose, "purpose", "", "project purpose")
	fset.StringVar(&values.stack, "s", "", "stack or platform for CODE repos")
	fset.StringVar(&values.stack, "stack", "", "stack or platform for CODE repos")
	fset.StringVar(&values.publishingPlatform, "u", "", "publishing platform for DOC repos")
	fset.StringVar(&values.publishingPlatform, "publishing-platform", "", "publishing platform for DOC repos")
	fset.StringVar(&values.style, "v", "", "style or voice for DOC repos")
	fset.StringVar(&values.style, "style", "", "style or voice for DOC repos")
	fset.BoolVar(&values.initGit, "g", false, "initialize git if target is not already a repo")
	fset.BoolVar(&values.initGit, "init-git", false, "initialize git if target is not already a repo")
	fset.BoolVar(&values.dryRun, "d", false, "preview changes without writing")
	fset.BoolVar(&values.dryRun, "dry-run", false, "preview changes without writing")
	if slices.Contains(args, "-?") || slices.Contains(args, "-h") || slices.Contains(args, "--help") {
		printModeHelp(mode)
		return Config{}, true, nil
	}
	if err := fset.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printModeHelp(mode)
			return Config{}, true, nil
		}
		return Config{}, false, err
	}

	target := strings.TrimSpace(values.target)
	if target == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return Config{}, false, fmt.Errorf("resolve current working directory: %w", err)
		}
		target = cwd
	}

	cfg := Config{
		Mode:               mode,
		Target:             target,
		Reference:          strings.TrimSpace(values.reference),
		Type:               RepoType(strings.ToUpper(strings.TrimSpace(values.repoType))),
		RepoName:           strings.TrimSpace(values.repoName),
		Purpose:            strings.TrimSpace(values.purpose),
		Stack:              strings.TrimSpace(values.stack),
		PublishingPlatform: strings.TrimSpace(values.publishingPlatform),
		Style:              strings.TrimSpace(values.style),
		InitGit:            values.initGit,
		DryRun:             values.dryRun,
	}
	// Validation is deferred to runSync (after prompts) for ModeSync.
	// For enhance, validate immediately.
	if mode == ModeEnhance {
		return cfg, false, validateConfig(cfg)
	}
	return cfg, false, nil
}

// ModeHelp returns mode-specific flag usage text.
func ModeHelp(mode Mode) string {
	switch mode {
	case ModeSync:
		return color.FormatUsage("governa sync [options]", []color.UsageLine{
			{Flag: "-n, --repo-name", Desc: "repo name"},
			{Flag: "-y, --type", Desc: "repo type: CODE or DOC"},
			{Flag: "-p, --purpose", Desc: "project purpose"},
			{Flag: "-s, --stack", Desc: "stack or platform (CODE repos)"},
			{Flag: "-u, --publishing-platform", Desc: "publishing platform (DOC repos)"},
			{Flag: "-v, --style", Desc: "style or voice (DOC repos)"},
			{Flag: "-t, --target", Desc: "target directory (default: current dir)"},
			{Flag: "-g, --init-git", Desc: "initialize git if target is not a repo"},
			{Flag: "-d, --dry-run", Desc: "preview changes without writing"},
		}, "Detects whether the target is a new or existing repo and prompts for missing parameters.")
	case ModeEnhance:
		return color.FormatUsage("governa enhance [options]", []color.UsageLine{
			{Flag: "-r, --reference", Desc: "reference repo to review for improvements"},
			{Flag: "-d, --dry-run", Desc: "preview changes without writing"},
		}, "Without -r: self-review embedded vs on-disk templates. With -r: review reference repo.")
	}
	return ""
}

func printModeHelp(mode Mode) {
	fmt.Fprint(os.Stderr, ModeHelp(mode))
}

func inferRepoName(targetDir string) string {
	abs, err := filepath.Abs(targetDir)
	if err != nil {
		return filepath.Base(targetDir)
	}
	return filepath.Base(abs)
}

func inferPurpose(targetDir string) string {
	path := filepath.Join(targetDir, "README.md")
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	foundContent := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			if foundContent {
				// Blank line after content means end of first paragraph.
				break
			}
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		// Skip badge lines, HTML, and image links
		if strings.HasPrefix(line, "![") || strings.HasPrefix(line, "<") || strings.HasPrefix(line, "[!") {
			continue
		}
		foundContent = true
		if len(line) > 200 {
			return line[:200]
		}
		return line
	}
	return ""
}

var stackManifests = []struct {
	file  string
	stack string
}{
	{"go.mod", "Go"},
	{"package.json", "Node"},
	{"Cargo.toml", "Rust"},
	{"pyproject.toml", "Python"},
	{"pom.xml", "Java"},
	{"build.gradle", "Java"},
}

func inferStack(targetDir string) string {
	for _, sm := range stackManifests {
		if _, err := os.Stat(filepath.Join(targetDir, sm.file)); err == nil {
			return sm.stack
		}
	}
	return ""
}

// resolveAdoptParams fills in missing adopt config fields using the priority:
// explicit flag > manifest params > inference from target directory.
// It returns the resolved config and a list of source annotations for display.
func resolveAdoptParams(cfg Config, targetDir string) (Config, []paramSource) {
	manifest, hasManifest, _ := readManifest(targetDir)
	var sources []paramSource

	if cfg.RepoName == "" {
		if hasManifest && manifest.Params.RepoName != "" {
			cfg.RepoName = manifest.Params.RepoName
			sources = append(sources, paramSource{"repo-name", cfg.RepoName, "manifest"})
		} else {
			cfg.RepoName = inferRepoName(targetDir)
			sources = append(sources, paramSource{"repo-name", cfg.RepoName, "inferred"})
		}
	} else {
		sources = append(sources, paramSource{"repo-name", cfg.RepoName, "flag"})
	}

	if cfg.Purpose == "" {
		if hasManifest && manifest.Params.Purpose != "" {
			cfg.Purpose = manifest.Params.Purpose
			sources = append(sources, paramSource{"purpose", cfg.Purpose, "manifest"})
		} else {
			cfg.Purpose = inferPurpose(targetDir)
			if cfg.Purpose != "" {
				sources = append(sources, paramSource{"purpose", cfg.Purpose, "inferred"})
			}
		}
	} else {
		sources = append(sources, paramSource{"purpose", cfg.Purpose, "flag"})
	}

	if cfg.Stack == "" {
		if hasManifest && manifest.Params.Stack != "" {
			cfg.Stack = manifest.Params.Stack
			sources = append(sources, paramSource{"stack", cfg.Stack, "manifest"})
		} else {
			cfg.Stack = inferStack(targetDir)
			if cfg.Stack != "" {
				sources = append(sources, paramSource{"stack", cfg.Stack, "inferred"})
			}
		}
	} else {
		sources = append(sources, paramSource{"stack", cfg.Stack, "flag"})
	}

	if cfg.Type == "" && hasManifest && manifest.Params.Type != "" {
		cfg.Type = RepoType(manifest.Params.Type)
		sources = append(sources, paramSource{"type", string(cfg.Type), "manifest"})
	}

	if cfg.PublishingPlatform == "" && hasManifest && manifest.Params.PublishingPlatform != "" {
		cfg.PublishingPlatform = manifest.Params.PublishingPlatform
		sources = append(sources, paramSource{"publishing-platform", cfg.PublishingPlatform, "manifest"})
	}

	if cfg.Style == "" && hasManifest && manifest.Params.Style != "" {
		cfg.Style = manifest.Params.Style
		sources = append(sources, paramSource{"style", cfg.Style, "manifest"})
	}

	return cfg, sources
}

type paramSource struct {
	name   string
	value  string
	source string // "flag", "manifest", "inferred"
}

func printParamSources(sources []paramSource) {
	for _, s := range sources {
		display := s.value
		if len(display) > 80 {
			display = display[:77] + "..."
		}
		fmt.Printf("%s: %s (%s)\n", s.name, display, s.source)
	}
}

func validateConfig(cfg Config) error {
	switch cfg.Mode {
	case ModeSync:
		if cfg.RepoName == "" {
			return errors.New("repo name is required: use -n or --repo-name")
		}
		if cfg.Purpose == "" {
			return errors.New("project purpose is required: use -p or --purpose")
		}
		if cfg.Type != RepoTypeCode && cfg.Type != RepoTypeDoc {
			return errors.New("repo type must be CODE or DOC: use -y or --type")
		}
		if cfg.Type == RepoTypeCode && cfg.Stack == "" {
			return errors.New("stack/platform is required for CODE repos: use -s or --stack")
		}
		if cfg.Type == RepoTypeDoc {
			if cfg.PublishingPlatform == "" {
				return errors.New("publishing platform is required for DOC repos: use -u or --publishing-platform")
			}
			if cfg.Style == "" {
				return errors.New("style is required for DOC repos: use -v or --style")
			}
		}
	case ModeEnhance:
		// -r is optional: empty means self-review mode
	default:
		return errors.New("unsupported mode")
	}
	return nil
}

// detectSyncMode inspects the target directory and returns one of:
//   - "re-sync"  — manifest found (adopt path with manifest defaults)
//   - "adopt"    — governance artifacts found but no manifest
//   - "new"      — fresh directory
func detectSyncMode(targetDir string) string {
	// Check for manifest first (authoritative).
	for _, name := range []string{manifestFileName, legacyManifestFileName} {
		if _, err := os.Stat(filepath.Join(targetDir, name)); err == nil {
			return "re-sync"
		}
	}
	// Check for governance artifacts.
	for _, artifact := range []string{"AGENTS.md", "CLAUDE.md"} {
		if _, err := os.Stat(filepath.Join(targetDir, artifact)); err == nil {
			return "adopt"
		}
	}
	if _, err := os.Stat(filepath.Join(targetDir, "docs", "roles")); err == nil {
		return "adopt"
	}
	return "new"
}

// promptRead reads a single line from the scanner. Returns empty string on EOF.
func promptRead(sc *bufio.Scanner) string {
	if sc.Scan() {
		return strings.TrimSpace(sc.Text())
	}
	return ""
}

// promptParam prints a prompt to stderr and reads a response. If the response
// is empty, returns defaultVal.
func promptParam(prompt string, defaultVal string, sc *bufio.Scanner) string {
	fmt.Fprint(os.Stderr, prompt)
	answer := promptRead(sc)
	if answer == "" {
		return defaultVal
	}
	return answer
}

// promptMissing fills in any missing Config fields by prompting interactively.
// Fields already set (via flags, manifest, or inference) are not prompted.
func promptMissing(cfg *Config, targetDir string) {
	r := cfg.Input
	if r == nil {
		r = os.Stdin
	}
	sc := bufio.NewScanner(r)

	if cfg.RepoName == "" {
		basename := inferRepoName(targetDir)
		answer := promptParam(fmt.Sprintf("Use '%s' as repo name? [Y/n]: ", basename), "", sc)
		if answer == "" || strings.EqualFold(answer, "y") || strings.EqualFold(answer, "yes") {
			cfg.RepoName = basename
		} else {
			cfg.RepoName = promptParam("Repo name: ", "", sc)
		}
	}

	if cfg.Type == "" {
		for cfg.Type != RepoTypeCode && cfg.Type != RepoTypeDoc {
			answer := promptParam("Repo type — CODE or DOC: ", "", sc)
			if answer == "" {
				break // EOF or empty input; let validation catch it
			}
			cfg.Type = RepoType(strings.ToUpper(answer))
		}
	}

	if cfg.Purpose == "" {
		cfg.Purpose = promptParam("Project purpose (one line): ", "", sc)
	}

	if cfg.Type == RepoTypeCode && cfg.Stack == "" {
		cfg.Stack = promptParam("Stack (Go, Node, Rust, Python, Java): ", "", sc)
	}

	if cfg.Type == RepoTypeDoc {
		if cfg.PublishingPlatform == "" {
			cfg.PublishingPlatform = promptParam("Publishing platform: ", "", sc)
		}
		if cfg.Style == "" {
			cfg.Style = promptParam("Style or voice: ", "", sc)
		}
	}
}

func runSync(tfs fs.FS, repoRoot string, cfg Config) error {
	targetAbs, err := filepath.Abs(cfg.Target)
	if err != nil {
		return fmt.Errorf("resolve target path: %w", err)
	}
	if err := os.MkdirAll(targetAbs, 0o755); err != nil && !cfg.DryRun {
		return fmt.Errorf("create target directory: %w", err)
	}

	syncMode := detectSyncMode(targetAbs)
	adopt := syncMode != "new"

	// For adopt/re-sync, resolve params from manifest and inference.
	if adopt {
		resolved, sources := resolveAdoptParams(cfg, targetAbs)
		cfg = resolved
		printParamSources(sources)
	}

	// Infer type from AssessTarget before prompting (flag > manifest > infer > prompt).
	// AssessTarget now auto-resolves an empty type from RepoShape before computing
	// expected artifacts, so the printed assessment is consistent whether cfg.Type
	// came from a manifest (re-sync) or was inferred from disk signals (first sync).
	assessment, err := AssessTarget(targetAbs, cfg.Type)
	if err != nil {
		return err
	}
	// Track whether the type was inferred from repo shape (vs supplied by
	// flag or resolved from a manifest). Only the inferred case should
	// print a `type: <TYPE> (inferred)` line below — flag/manifest
	// provenance is already covered by printParamSources on the adopt path.
	typeInferred := cfg.Type == "" && assessment.ResolvedType != ""
	if typeInferred {
		cfg.Type = assessment.ResolvedType
	}

	// Prompt for any still-missing parameters.
	promptMissing(&cfg, targetAbs)

	// Validate after prompts have filled gaps.
	if err := validateConfig(cfg); err != nil {
		return err
	}

	printAssessment(cfg.Mode, targetAbs, assessment)
	if typeInferred {
		fmt.Printf("type: %s (inferred)\n", cfg.Type)
	}

	canonical, err := planCanonical(tfs, repoRoot, cfg, targetAbs)
	if err != nil {
		return err
	}

	templateVersion := readTemplateVersion(repoRoot)
	manifest := buildManifest(canonical, templateVersion, tfs, repoRoot, targetAbs)
	manifest.Params = ManifestParams{
		RepoName:           cfg.RepoName,
		Purpose:            cfg.Purpose,
		Type:               string(cfg.Type),
		Stack:              cfg.Stack,
		PublishingPlatform: cfg.PublishingPlatform,
		Style:              cfg.Style,
	}
	manifestOp := operation{
		kind:    "write",
		path:    filepath.Join(targetAbs, manifestFileName),
		content: formatManifest(manifest),
		note:    "bootstrap manifest",
	}

	var ops []operation
	var syncConflicts []conflict
	if adopt {
		oldManifest, _, _ := readManifest(targetAbs)
		oldEntryMap := manifestEntryMap(oldManifest)
		newEntryMap := manifestEntryMap(manifest)
		transformed, scores, conflicts := applyAdoptTransforms(canonical, oldEntryMap, newEntryMap, targetAbs)
		syncConflicts = conflicts

		// If any symlink ops were blocked by conflicts, rebuild the manifest
		// without those entries. Other skip ops (overlay collisions) retain
		// their baseline source checksums — those are load-bearing for
		// future standing drift detection.
		if len(conflicts) > 0 {
			blockedPaths := make(map[string]bool, len(conflicts))
			for _, c := range conflicts {
				blockedPaths[c.path] = true
			}
			filteredCanonical := make([]operation, 0, len(canonical))
			for _, op := range canonical {
				if op.kind == "symlink" && blockedPaths[op.path] {
					continue
				}
				filteredCanonical = append(filteredCanonical, op)
			}
			manifest = buildManifest(filteredCanonical, templateVersion, tfs, repoRoot, targetAbs)
			manifest.Params = ManifestParams{
				RepoName:           cfg.RepoName,
				Purpose:            cfg.Purpose,
				Type:               string(cfg.Type),
				Stack:              cfg.Stack,
				PublishingPlatform: cfg.PublishingPlatform,
				Style:              cfg.Style,
			}
			manifestOp = operation{
				kind:    "write",
				path:    filepath.Join(targetAbs, manifestFileName),
				content: formatManifest(manifest),
				note:    "bootstrap manifest",
			}
		}

		ops = compactOperations(transformed)
		emitAdoptAdvisories(targetAbs)
		ops = append(ops, manifestOp)
		if err := applyOperations(ops, cfg.DryRun); err != nil {
			return err
		}
		// Filter to only collision scores (keep/adopt) for the review doc
		var collisions []collisionScore
		for _, s := range scores {
			if s.recommendation != "accept" {
				collisions = append(collisions, s)
			}
		}
		if len(collisions) > 0 || len(conflicts) > 0 {
			if err := writeSyncReview(targetAbs, collisions, conflicts, oldManifest.TemplateVersion, templateVersion, cfg.DryRun); err != nil {
				return err
			}
			if err := writeProposedFiles(targetAbs, collisions, cfg.DryRun); err != nil {
				return err
			}
		}
		printAdoptDriftFromScores(collisions)
		if len(conflicts) > 0 {
			printConflictsSummary(targetAbs, conflicts)
		}
	} else {
		ops = compactOperations(canonical)
		ops = append(ops, manifestOp)
		if err := applyOperations(ops, cfg.DryRun); err != nil {
			return err
		}
	}
	if cfg.InitGit {
		if err := maybeInitGit(targetAbs, cfg.DryRun); err != nil {
			return err
		}
	}
	if len(syncConflicts) > 0 {
		return ErrConflictsPresent
	}
	return nil
}

// enhanceHeadingRe matches the heading format produced by renderACDoc: "# AC<N> Enhance: ..."
var enhanceHeadingRe = regexp.MustCompile(`^# AC\d+ Enhance:`)

// existingEnhanceAC holds the path and AC number of a prior enhance-generated AC.
type existingEnhanceAC struct {
	path    string
	acNum   int
	heading string
}

// findExistingEnhanceAC scans docsDir for AC files whose first line matches
// the enhance-generated heading format (# ACN Enhance: ...). Results are
// sorted by AC number ascending.
func findExistingEnhanceAC(docsDir string) []existingEnhanceAC {
	entries, err := os.ReadDir(docsDir)
	if err != nil {
		return nil
	}
	var results []existingEnhanceAC
	for _, entry := range entries {
		name := entry.Name()
		match := workingACFileRe.FindStringSubmatch(name)
		if match == nil {
			continue
		}
		path := filepath.Join(docsDir, name)
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		var firstLine string
		if scanner.Scan() {
			firstLine = scanner.Text()
		}
		f.Close()
		if enhanceHeadingRe.MatchString(firstLine) {
			num, _ := strconv.Atoi(match[1])
			results = append(results, existingEnhanceAC{
				path:    path,
				acNum:   num,
				heading: firstLine,
			})
		}
	}
	slices.SortFunc(results, func(a, b existingEnhanceAC) int {
		return cmp.Compare(a.acNum, b.acNum)
	})
	return results
}

// collisionAction describes what to do with an existing enhance AC.
type collisionAction struct {
	mode    string // "replace", "update", or "new"
	oldPath string // path of existing AC (empty for "new")
	acNum   int    // AC number to use
}

// promptEnhanceCollision handles collision with existing enhance ACs.
func promptEnhanceCollision(existing []existingEnhanceAC, nextNum int, sc *bufio.Scanner) collisionAction {
	if len(existing) == 1 {
		e := existing[0]
		fmt.Fprintf(os.Stderr, "Existing enhance AC: %s\n  %s\n", filepath.Base(e.path), e.heading)
		answer := promptParam("Replace, update, or new? [r/u/n]: ", "", sc)
		switch strings.ToLower(answer) {
		case "r", "replace":
			return collisionAction{"replace", e.path, e.acNum}
		case "u", "update":
			return collisionAction{"update", e.path, e.acNum}
		default:
			return collisionAction{"new", "", nextNum}
		}
	}
	// Multiple matches — list and prompt for selection.
	fmt.Fprintln(os.Stderr, "Existing enhance ACs:")
	for i, e := range existing {
		fmt.Fprintf(os.Stderr, "  %d. %s — %s\n", i+1, filepath.Base(e.path), e.heading)
	}
	answer := promptParam(fmt.Sprintf("Select AC to replace/update (1–%d), or n for new: ", len(existing)), "", sc)
	if strings.EqualFold(answer, "n") || answer == "" {
		return collisionAction{"new", "", nextNum}
	}
	idx, err := strconv.Atoi(answer)
	if err != nil || idx < 1 || idx > len(existing) {
		return collisionAction{"new", "", nextNum}
	}
	e := existing[idx-1]
	action := promptParam("Replace or update? [r/u]: ", "", sc)
	switch strings.ToLower(action) {
	case "r", "replace":
		return collisionAction{"replace", e.path, e.acNum}
	case "u", "update":
		return collisionAction{"update", e.path, e.acNum}
	default:
		return collisionAction{"new", "", nextNum}
	}
}

// RunEnhance runs enhance mode against a reference repo.
func RunEnhance(tfs fs.FS, repoRoot string, cfg Config) error {
	refAbs, err := filepath.Abs(cfg.Reference)
	if err != nil {
		return fmt.Errorf("resolve reference path: %w", err)
	}
	report, err := ReviewEnhancement(tfs, repoRoot, refAbs)
	if err != nil {
		return err
	}
	printEnhancementSummary(report)

	selected, deferred, ok := selectActionableCandidates(report.Candidates)
	if !ok {
		fmt.Println("no actionable improvements found; no AC doc created")
		return nil
	}

	docsDir := filepath.Join(repoRoot, "docs")
	nextNum, err := nextACNumber(docsDir)
	if err != nil {
		return err
	}

	// Check for existing enhance-generated ACs.
	acNum := nextNum
	var action collisionAction
	existing := findExistingEnhanceAC(docsDir)
	if len(existing) > 0 {
		r := cfg.Input
		if r == nil {
			r = os.Stdin
		}
		sc := bufio.NewScanner(r)
		action = promptEnhanceCollision(existing, nextNum, sc)
		acNum = action.acNum
	} else {
		action = collisionAction{mode: "new", acNum: nextNum}
	}

	slug := acSlug(selected)
	var acPath string
	switch action.mode {
	case "update":
		// Keep the old file path, overwrite in place.
		acPath = action.oldPath
	case "replace":
		// Same AC number, new slug-based filename. Old file will be removed.
		acFileName := fmt.Sprintf("ac%d-%s.md", acNum, slug)
		acPath = filepath.Join(docsDir, acFileName)
	default:
		// New AC with next sequential number.
		acFileName := fmt.Sprintf("ac%d-%s.md", acNum, slug)
		acPath = filepath.Join(docsDir, acFileName)
	}
	acContent := renderACDoc(selected, deferred, report, acNum)

	if cfg.DryRun {
		switch action.mode {
		case "update":
			fmt.Printf("dry-run update %s (enhancement AC doc)\n", acPath)
		case "replace":
			fmt.Printf("dry-run replace %s -> %s (enhancement AC doc)\n", filepath.Base(action.oldPath), filepath.Base(acPath))
		default:
			fmt.Printf("dry-run write %s (enhancement AC doc)\n", acPath)
		}
		fmt.Println("dry-run: no changes applied")
		return nil
	}
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		return fmt.Errorf("create docs directory: %w", err)
	}
	// For replace: remove the old file if its path differs from the new one.
	if action.mode == "replace" && action.oldPath != acPath {
		os.Remove(action.oldPath)
	}
	if err := os.WriteFile(acPath, []byte(acContent), 0o644); err != nil {
		return fmt.Errorf("write AC doc: %w", err)
	}
	switch action.mode {
	case "update":
		fmt.Printf("updated %s (enhancement AC doc)\n", acPath)
	case "replace":
		fmt.Printf("replaced %s (enhancement AC doc)\n", acPath)
	default:
		fmt.Printf("write %s (enhancement AC doc)\n", acPath)
	}
	return nil
}

// SelfReviewDelta represents a single file difference found during self-review.
type SelfReviewDelta struct {
	Path     string
	Kind     string   // "changed", "added", "removed"
	Sections []string // non-empty for structured markdown with per-section diffs
}

// RunSelfReview compares two template FS instances (typically embedded vs on-disk)
// and reports files that differ. This is an informational pre-release audit tool;
// it does not create AC docs or proposal files.
// selfReviewRoots are the subdirectories within the template FS that
// self-review compares. Go source files at the FS root are excluded.
var selfReviewRoots = []string{"base", "overlays"}

func RunSelfReview(baselineFS, currentFS fs.FS, version string) ([]SelfReviewDelta, error) {
	baselineFiles := make(map[string]string)
	for _, root := range selfReviewRoots {
		err := fs.WalkDir(baselineFS, root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			content, readErr := fs.ReadFile(baselineFS, path)
			if readErr != nil {
				return readErr
			}
			baselineFiles[path] = string(content)
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walk baseline templates (%s): %w", root, err)
		}
	}

	var deltas []SelfReviewDelta
	currentFiles := make(map[string]bool)

	for _, root := range selfReviewRoots {
		err := fs.WalkDir(currentFS, root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			currentFiles[path] = true
			content, readErr := fs.ReadFile(currentFS, path)
			if readErr != nil {
				return readErr
			}
			currentContent := string(content)

			baselineContent, exists := baselineFiles[path]
			if !exists {
				deltas = append(deltas, SelfReviewDelta{Path: path, Kind: "added"})
				return nil
			}
			if currentContent != baselineContent {
				delta := SelfReviewDelta{Path: path, Kind: "changed"}
				if strings.HasSuffix(path, ".md") || strings.HasSuffix(path, ".md.tmpl") {
					delta.Sections = diffMarkdownSections(baselineContent, currentContent)
				}
				deltas = append(deltas, delta)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walk current templates (%s): %w", root, err)
		}
	}

	for path := range baselineFiles {
		if !currentFiles[path] {
			deltas = append(deltas, SelfReviewDelta{Path: path, Kind: "removed"})
		}
	}

	slices.SortFunc(deltas, func(a, b SelfReviewDelta) int {
		return cmp.Compare(a.Path, b.Path)
	})
	return deltas, nil
}

func PrintSelfReview(deltas []SelfReviewDelta, version string) {
	fmt.Printf("mode: self-review (comparing local templates against embedded v%s)\n", version)
	if len(deltas) == 0 {
		fmt.Println("no changes since embedded version")
		return
	}
	changed, added, removed := 0, 0, 0
	for _, d := range deltas {
		switch d.Kind {
		case "changed":
			changed++
			if len(d.Sections) > 0 {
				fmt.Printf("  %s %s (sections: %s)\n", color.Yel("changed:"), d.Path, strings.Join(d.Sections, ", "))
			} else {
				fmt.Printf("  %s %s\n", color.Yel("changed:"), d.Path)
			}
		case "added":
			added++
			fmt.Printf("  %s   %s\n", color.Grn("added:"), d.Path)
		case "removed":
			removed++
			fmt.Printf("  %s %s\n", color.Red("removed:"), d.Path)
		}
	}
	fmt.Printf("summary: %d changed, %d added, %d removed\n", changed, added, removed)
}

func AssessTarget(root string, repoType RepoType) (Assessment, error) {
	var files []string
	hasSourceFile := false
	hasCodeManifest := false
	hasCodeLayout := false
	hasDocPlanningMarker := false
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == ".git" || strings.HasPrefix(rel, ".git"+string(os.PathSeparator)) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		// .governa-proposed/ is a sync working artifact, not repo content.
		// Skip it entirely so its markdown files don't inflate docSignals on
		// re-sync vs first-sync (utils round-5 finding).
		if rel == ".governa-proposed" || strings.HasPrefix(rel, ".governa-proposed"+string(os.PathSeparator)) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		files = append(files, rel)
		return nil
	})
	if err != nil {
		return Assessment{}, fmt.Errorf("scan target repo: %w", err)
	}

	codeSignals := 0
	docSignals := 0
	if len(files) == 0 {
		return Assessment{
			RepoShape:      "empty",
			ResolvedType:   repoType, // no files to infer from; preserve caller input (possibly "")
			CollisionRisk:  "low",
			Recommendation: "safe to apply",
		}, nil
	}
	for _, rel := range files {
		base := filepath.Base(rel)
		ext := strings.ToLower(filepath.Ext(rel))
		topLevel := strings.Split(rel, string(os.PathSeparator))[0]
		// Governa-owned paths (generated by sync itself) must not contribute
		// to signal counts. Otherwise re-sync inflates docSignals vs
		// first-sync for the same underlying repo content. Scope is
		// signal-counting only — files remain in `files` for other uses
		// (collision detection, structural analysis, etc.) and
		// expectedArtifactPaths/ExistingArtifacts are unchanged.
		if isGovernaOwnedPath(rel) {
			switch topLevel {
			case "cmd", "internal", "pkg", "src":
				hasCodeLayout = true
			}
			continue
		}
		switch ext {
		case ".go", ".py", ".js", ".ts", ".tsx", ".jsx", ".rs", ".java", ".kt", ".swift", ".c", ".cc", ".cpp", ".cs":
			codeSignals++
			hasSourceFile = true
		case ".md", ".mdx":
			docSignals++
		}
		switch base {
		case "go.mod", "package.json", "pyproject.toml", "Cargo.toml", "pom.xml", "build.gradle", "Makefile", "Dockerfile":
			codeSignals += 3
			hasCodeManifest = true
		case "style.md", "voice.md", "content-plan.md", "calendar.md", "mkdocs.yml", "mkdocs.yaml":
			docSignals += 3
			hasDocPlanningMarker = true
		case "README.md", "AGENTS.md", "CLAUDE.md", "arch.md", "plan.md":
			docSignals++
		}
		switch topLevel {
		case "cmd", "internal", "pkg", "src":
			hasCodeLayout = true
		}
	}

	repoShape := "unclear"
	switch {
	case hasCodeManifest && hasSourceFile:
		repoShape = "likely CODE"
	case hasCodeLayout && hasSourceFile:
		repoShape = "likely CODE"
	case hasDocPlanningMarker && !hasSourceFile && !hasCodeManifest:
		repoShape = "likely DOC"
	case codeSignals > docSignals && codeSignals > 0:
		repoShape = "likely CODE"
	case docSignals > codeSignals && docSignals > 0:
		repoShape = "likely DOC"
	case codeSignals > 0 && docSignals > 0:
		repoShape = "mixed"
	}

	// Resolve an empty caller-provided type from the detected shape so the
	// expected-artifacts check uses the same repo type regardless of whether
	// cfg.Type was pre-populated from a manifest. Without this, first-sync
	// and re-sync on the same on-disk state produce different assessments.
	resolvedType := repoType
	if resolvedType == "" {
		resolvedType = deriveTypeFromShape(repoShape)
	}

	expected := expectedArtifactPaths(resolvedType)
	var existing []string
	var collisions []string
	for _, rel := range expected {
		full := filepath.Join(root, rel)
		info, err := os.Stat(full)
		if err == nil {
			existing = append(existing, rel)
			if !info.IsDir() && info.Size() > 0 {
				collisions = append(collisions, rel)
			}
		}
	}

	collisionRisk := "low"
	switch {
	case len(collisions) >= 3:
		collisionRisk = "high"
	case len(collisions) > 0:
		collisionRisk = "medium"
	}

	recommendation := "safe to apply"
	if collisionRisk == "high" || repoShape == "unclear" || repoShape == "mixed" {
		recommendation = "safe with proposals only"
	}
	if repoShape == "unclear" && len(existing) == 0 {
		recommendation = "needs manual mapping first"
	}

	return Assessment{
		RepoShape:          repoShape,
		ResolvedType:       resolvedType,
		ExistingArtifacts:  existing,
		CollisionRisk:      collisionRisk,
		Recommendation:     recommendation,
		CodeSignals:        codeSignals,
		DocSignals:         docSignals,
		CollidingArtifacts: collisions,
	}, nil
}

func ReviewEnhancement(tfs fs.FS, repoRoot string, referenceRoot string) (EnhancementReport, error) {
	manifest, hasManifest, err := readManifest(referenceRoot)
	if err != nil {
		return EnhancementReport{}, err
	}
	var mmap map[string]ManifestEntry
	if hasManifest {
		mmap = manifestEntryMap(manifest)
	}

	var candidates []EnhancementCandidate
	if governanceCandidates, err := reviewGovernedSections(tfs, referenceRoot, mmap); err != nil {
		return EnhancementReport{}, err
	} else {
		candidates = append(candidates, governanceCandidates...)
	}

	mappings := []enhancementMapping{
		{Area: "CODE overlay", ReferencePaths: []string{"README.md"}, TemplateTarget: "overlays/code/files/README.md.tmpl"},
		{Area: "CODE overlay", ReferencePaths: []string{"arch.md"}, TemplateTarget: "overlays/code/files/arch.md.tmpl"},
		{Area: "CODE overlay", ReferencePaths: []string{"plan.md"}, TemplateTarget: "overlays/code/files/plan.md.tmpl"},
		{Area: "CODE overlay", ReferencePaths: []string{"docs/development-cycle.md"}, TemplateTarget: "overlays/code/files/docs/development-cycle.md.tmpl"},
		{Area: "CODE overlay", ReferencePaths: []string{"docs/ac-template.md"}, TemplateTarget: "overlays/code/files/docs/ac-template.md.tmpl"},
		{Area: "CODE overlay", ReferencePaths: []string{"docs/build-release.md"}, TemplateTarget: "overlays/code/files/docs/build-release.md.tmpl"},
		{Area: "CODE overlay", ReferencePaths: []string{"build.sh"}, TemplateTarget: "overlays/code/files/build.sh.tmpl"},
		{Area: "CODE overlay", ReferencePaths: []string{"cmd/build/main.go"}, TemplateTarget: "overlays/code/files/cmd/build/main.go.tmpl"},
		{Area: "CODE overlay", ReferencePaths: []string{"cmd/rel/main.go"}, TemplateTarget: "overlays/code/files/cmd/rel/main.go.tmpl"},
		{Area: "DOC overlay", ReferencePaths: []string{"README.md"}, TemplateTarget: "overlays/doc/files/README.md.tmpl"},
		{Area: "DOC overlay", ReferencePaths: []string{"style.md", "voice.md"}, TemplateTarget: "overlays/doc/files/style.md.tmpl"},
		{Area: "DOC overlay", ReferencePaths: []string{"content-plan.md", "calendar.md"}, TemplateTarget: "overlays/doc/files/content-plan.md.tmpl"},
		{Area: "DOC overlay", ReferencePaths: []string{"publishing-workflow.md"}, TemplateTarget: "overlays/doc/files/publishing-workflow.md.tmpl"},
		{Area: "DOC overlay", ReferencePaths: []string{"release.md"}, TemplateTarget: "overlays/doc/files/release.md.tmpl"},
		{Area: "DOC overlay", ReferencePaths: []string{"rel.sh"}, TemplateTarget: "overlays/doc/files/rel.sh.tmpl"},
		{Area: "DOC overlay", ReferencePaths: []string{"cmd/rel/main.go"}, TemplateTarget: "overlays/doc/files/cmd/rel/main.go.tmpl"},
		{Area: "examples or upgrade path", ReferencePaths: []string{"TEMPLATE_VERSION"}, TemplateTarget: "TEMPLATE_VERSION"},
	}
	for _, item := range mappings {
		candidate, ok, err := reviewMappedFile(tfs, repoRoot, referenceRoot, item, mmap)
		if err != nil {
			return EnhancementReport{}, err
		}
		if ok {
			candidates = append(candidates, candidate)
		}
	}

	slices.SortFunc(candidates, func(a, b EnhancementCandidate) int {
		if byArea := cmp.Compare(a.Area, b.Area); byArea != 0 {
			return byArea
		}
		if bySection := cmp.Compare(a.Section, b.Section); bySection != 0 {
			return bySection
		}
		return cmp.Compare(a.Path, b.Path)
	})
	return EnhancementReport{ReferenceRoot: referenceRoot, Candidates: candidates}, nil
}

type enhancementMapping struct {
	Area           string
	ReferencePaths []string
	TemplateTarget string
}

type markdownSection struct {
	Name string
	Body string
}

// governaOwnedPaths are exact repo-relative paths that governa writes or
// maintains. Files at these paths must not contribute to codeSignals or
// docSignals — they are bookkeeping/generated content, not repo-authored
// signals. This set is used by isGovernaOwnedPath; its scope is intentionally
// limited to signal counting and does NOT affect expectedArtifactPaths,
// ExistingArtifacts computation, collision scoring, review rendering, or
// .governa-proposed/ materialization.
var governaOwnedPaths = map[string]bool{
	// Bookkeeping
	".governa-manifest":      true,
	".repokit-manifest":      true, // legacy, kept for backward-compat repos
	"TEMPLATE_VERSION":       true,
	"governa-sync-review.md": true,

	// Agent entrypoints (any future entrypoint name that symlinks to AGENTS.md
	// would be added here via the same pattern).
	"AGENTS.md": true,
	"CLAUDE.md": true,

	// Root overlay markdown governa writes into generated repos
	"arch.md": true,
	"plan.md": true,

	// docs/ overlay markdown governa writes
	"docs/README.md":                 true,
	"docs/development-cycle.md":      true,
	"docs/development-guidelines.md": true,
	"docs/build-release.md":          true,
	"docs/ac-template.md":            true,
}

// isGovernaOwnedPath reports whether a repo-relative path is governa-owned
// (written or maintained by sync). Used by AssessTarget to skip signal
// increments for such paths so first-sync and re-sync produce the same
// `signals:` counts for the same underlying repo content.
//
// Scope — this helper ONLY affects codeSignals/docSignals. It does NOT:
//   - filter files out of ExistingArtifacts or collision reporting
//   - change which scored files appear in the review doc's Recommendations table
//   - alter which files get materialized under .governa-proposed/
//   - affect RepoShape, CollisionRisk, Recommendation, or ResolvedType beyond
//     what the corrected signal counts naturally produce
func isGovernaOwnedPath(rel string) bool {
	// Normalize separators for cross-platform map lookup
	norm := filepath.ToSlash(rel)
	if governaOwnedPaths[norm] {
		return true
	}
	// docs/roles/* is owned by governa (DEV, QA, director reference, maintainer, README)
	if strings.HasPrefix(norm, "docs/roles/") {
		return true
	}
	return false
}

func expectedArtifactPaths(repoType RepoType) []string {
	base := []string{"AGENTS.md", "CLAUDE.md", "TEMPLATE_VERSION", "README.md"}
	switch repoType {
	case RepoTypeCode:
		return append(
			base,
			"arch.md",
			"plan.md",
			"CHANGELOG.md",
			filepath.Join("docs", "README.md"),
			filepath.Join("docs", "development-cycle.md"),
			filepath.Join("docs", "ac-template.md"),
			filepath.Join("docs", "build-release.md"),
		)
	case RepoTypeDoc:
		return append(base, "style.md", "content-plan.md", "publishing-workflow.md")
	default:
		return base
	}
}

var goStackPattern = regexp.MustCompile(`(?i)\b(go|golang)\b`)

func stackSuggestsGo(stack string) bool {
	return goStackPattern.MatchString(stack)
}

func planRender(tfs fs.FS, repoRoot string, cfg Config, targetRoot string, adopt bool) ([]operation, error) {
	canonical, err := planCanonical(tfs, repoRoot, cfg, targetRoot)
	if err != nil {
		return nil, err
	}
	if !adopt {
		return compactOperations(canonical), nil
	}
	transformed, _, _ := applyAdoptTransforms(canonical, nil, nil, targetRoot)
	return compactOperations(transformed), nil
}

func readModulePath(targetRoot string) string {
	goModPath := filepath.Join(targetRoot, "go.mod")
	content, err := os.ReadFile(goModPath)
	if err != nil {
		return ""
	}
	for line := range strings.SplitSeq(string(content), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module"))
		}
	}
	return ""
}

func planCanonical(tfs fs.FS, repoRoot string, cfg Config, targetRoot string) ([]operation, error) {
	modulePath := readModulePath(targetRoot)
	if modulePath == "" {
		// New repos don't have go.mod yet; use repo name as placeholder
		modulePath = cfg.RepoName
	}
	placeholders := map[string]string{
		"{{REPO_NAME}}":           cfg.RepoName,
		"{{PROJECT_PURPOSE}}":     cfg.Purpose,
		"{{STACK_OR_PLATFORM}}":   valueOrDefault(cfg.Stack, "TBD"),
		"{{PUBLISHING_PLATFORM}}": valueOrDefault(cfg.PublishingPlatform, "TBD"),
		"{{DOC_STYLE}}":           valueOrDefault(cfg.Style, "TBD"),
		"{{MODULE_PATH}}":         modulePath,
	}

	agentsContent, err := readAndRender(tfs, "base/AGENTS.md", placeholders)
	if err != nil {
		return nil, err
	}
	ops := []operation{{
		kind:    "write",
		path:    filepath.Join(targetRoot, "AGENTS.md"),
		content: agentsContent,
		note:    "base governance contract",
		source:  "base/AGENTS.md",
	}}

	versionContent := []byte(readTemplateVersion(repoRoot))
	ops = append(ops, operation{
		kind:    "write",
		path:    filepath.Join(targetRoot, "TEMPLATE_VERSION"),
		content: string(versionContent),
		note:    "template version marker",
		source:  "TEMPLATE_VERSION",
	})

	ops = append(ops, operation{
		kind:   "symlink",
		path:   filepath.Join(targetRoot, "CLAUDE.md"),
		linkTo: "AGENTS.md",
		note:   "agent alias link",
		source: "base/AGENTS.md",
	})

	overlayPrefix := "overlays/" + strings.ToLower(string(cfg.Type)) + "/files"
	err = fs.WalkDir(tfs, overlayPrefix, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel := strings.TrimPrefix(path, overlayPrefix+"/")
		if cfg.Type == RepoTypeCode && !stackSuggestsGo(cfg.Stack) &&
			(rel == "cmd/rel/main.go.tmpl" ||
				rel == "cmd/build/main.go.tmpl" ||
				strings.HasPrefix(rel, "internal/color/")) {
			return nil
		}
		// Skip internal/color when module path is unknown (adopt without go.mod)
		if strings.HasPrefix(rel, "internal/color/") && modulePath == "" {
			return nil
		}
		// Skip docs/knowledge/README.md if the target doesn't use docs/knowledge/
		if rel == "docs/knowledge/README.md.tmpl" || rel == "docs/knowledge/README.md" {
			if shouldSkipKnowledgeDir(targetRoot) {
				return nil
			}
		}
		targetRel := strings.TrimSuffix(rel, ".tmpl")
		content, err := readAndRender(tfs, path, placeholders)
		if err != nil {
			return err
		}
		ops = append(ops, operation{
			kind:    "write",
			path:    filepath.Join(targetRoot, targetRel),
			content: content,
			note:    "overlay file",
			source:  path,
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk overlay templates: %w", err)
	}

	return ops, nil
}

func applyAdoptTransforms(ops []operation, oldManifest map[string]ManifestEntry, newManifest map[string]ManifestEntry, targetDir string) ([]operation, []collisionScore, []conflict) {
	out := make([]operation, len(ops))
	var scores []collisionScore
	var conflicts []conflict
	modulePath := readModulePath(targetDir)
	firstSync := len(oldManifest) == 0
	for i, op := range ops {
		// Derive repo-relative path for manifest lookup.
		var repoRel string
		if rel, err := filepath.Rel(targetDir, op.path); err == nil {
			repoRel = filepath.ToSlash(rel)
		}
		oldEntry := oldManifest[repoRel]
		newEntry := newManifest[repoRel]

		switch {
		case op.kind == "write" && op.note == "base governance contract":
			score := scoreGovernanceCollision(op, oldEntry.SourceChecksum, newEntry.SourceChecksum)
			scores = append(scores, score)
			if score.recommendation == "accept" {
				out[i] = op // file doesn't exist, write directly
			} else {
				out[i] = operation{kind: "skip"} // collision handled via review doc
			}
		case op.kind == "write" && op.note == "template version marker":
			out[i] = op // always write — must match manifest version
		case op.kind == "symlink":
			// Detect symlink-vs-regular-file collision. Use Lstat so we don't
			// follow symlinks — an existing symlink is fine, a regular file is not.
			if info, err := os.Lstat(op.path); err == nil && info.Mode()&os.ModeSymlink == 0 {
				conflicts = append(conflicts, symlinkConflict(op, repoRel))
				out[i] = operation{kind: "skip"}
			} else {
				out[i] = skipIfExists(op)
			}
		case op.kind == "write" && op.note == "overlay file":
			score := scoreOverlayCollision(op.path, op.content, oldEntry.SourceChecksum, newEntry.SourceChecksum)
			promoteStandingDrift(&score)
			if firstSync && score.standingDrift {
				// Rewrite reason for first-sync: no prior history to reference
				score.reason = firstSyncDriftReason(score)
			}
			promoteStructuralNotes(&score)
			demoteScaffold(&score)
			demoteExtractedPackage(&score, modulePath)
			if score.recommendation == "accept" {
				out[i] = op // file doesn't exist, write directly
			} else {
				scores = append(scores, score)
				out[i] = operation{kind: "skip"} // collision handled via review doc
			}
		default:
			out[i] = op
		}
	}
	return out, scores, conflicts
}

// symlinkConflict builds the operator-facing conflict description for a
// symlink op that was blocked by an existing regular file. The message
// enforces governa's agent-agnostic invariant: AGENTS.md is the canonical
// governance contract, and any agent-specific entrypoint (CLAUDE.md, future
// names) must be a symlink to it so every agent loads the same rules.
func symlinkConflict(op operation, repoRel string) conflict {
	if repoRel == "" {
		repoRel = filepath.Base(op.path)
	}
	linkTarget := op.linkTo
	if linkTarget == "" {
		linkTarget = "AGENTS.md"
	}
	// Description is a multi-line block starting with a `### <file>` heading.
	// It is rendered as-is under the review doc's ## Conflicts section, not
	// wrapped in a bullet. The entrypoint name (repoRel) and link target are
	// parameterized so this structure applies to any blocked symlink-to-AGENTS
	// entrypoint — CLAUDE.md is the current concrete instance; future
	// agent-specific entrypoints inherit the same formatting.
	var b strings.Builder
	fmt.Fprintf(&b, "### `%s`\n\n", repoRel)
	fmt.Fprintf(&b, "`%s` exists as a regular file. Governa is agent-agnostic: `%s` is the canonical governance contract, and agent-specific entrypoints (`CLAUDE.md` for Claude Code, others as they emerge) must be symlinks to it so all agents load the same rules.\n\n", repoRel, linkTarget)
	fmt.Fprintln(&b, "**Resolution:**")
	fmt.Fprintln(&b, "")
	fmt.Fprintf(&b, "1. Diff the existing file against the newly written `%s`:\n\n", linkTarget)
	fmt.Fprintf(&b, "        diff %s %s\n\n", repoRel, linkTarget)
	fmt.Fprintf(&b, "2. Migrate any unique repo-specific rules from `%s` into `%s` using the governance section structure. If existing content is already covered by `%s`, skip this step.\n\n", repoRel, linkTarget, linkTarget)
	fmt.Fprintf(&b, "3. Delete the existing `%s` and re-run `governa sync` to create the symlink.\n\n", repoRel)
	fmt.Fprintf(&b, "Note: `%s` was written to the repo root during this sync so you can diff against it. This is intentional — the temporary inconsistency (`%s` claims `%s` is a symlink while it is not) resolves as soon as you complete the steps above.\n",
		linkTarget, linkTarget, repoRel)

	return conflict{
		kind:        "symlink-vs-regular",
		path:        op.path,
		description: b.String(),
	}
}

// firstSyncDriftReason produces a standing-drift reason string that does
// not imply a prior sync history. Used when oldManifest is empty.
func firstSyncDriftReason(score collisionScore) string {
	if len(score.driftSections) > 0 {
		return fmt.Sprintf("differs from template baseline in: %s (no prior sync)", strings.Join(score.driftSections, ", "))
	}
	return "differs from template baseline (no prior sync)"
}

func compactOperations(ops []operation) []operation {
	out := make([]operation, 0, len(ops))
	for _, op := range ops {
		if op.kind == "skip" {
			continue
		}
		out = append(out, op)
	}
	return out
}

func applyOperations(ops []operation, dryRun bool) error {
	for _, op := range ops {
		switch op.kind {
		case "mkdir":
			fmt.Printf("%s %s (%s)\n", formatAction(dryRun, "mkdir"), op.path, op.note)
			if dryRun {
				continue
			}
			if err := os.MkdirAll(op.path, 0o755); err != nil {
				return fmt.Errorf("create directory %s: %w", op.path, err)
			}
		case "write":
			fmt.Printf("%s %s (%s)\n", formatAction(dryRun, "write"), op.path, op.note)
			if dryRun {
				continue
			}
			if err := os.MkdirAll(filepath.Dir(op.path), 0o755); err != nil {
				return fmt.Errorf("create parent directory for %s: %w", op.path, err)
			}
			mode := os.FileMode(0o644)
			if strings.HasSuffix(op.path, ".sh") {
				mode = 0o755
			}
			if err := os.WriteFile(op.path, []byte(op.content), mode); err != nil {
				return fmt.Errorf("write %s: %w", op.path, err)
			}
		case "symlink":
			fmt.Printf("%s %s -> %s (%s)\n", formatAction(dryRun, "symlink"), op.path, op.linkTo, op.note)
			if dryRun {
				continue
			}
			if err := os.MkdirAll(filepath.Dir(op.path), 0o755); err != nil {
				return fmt.Errorf("create parent directory for %s: %w", op.path, err)
			}
			if err := os.RemoveAll(op.path); err != nil && !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("remove existing path before symlink %s: %w", op.path, err)
			}
			if err := os.Symlink(op.linkTo, op.path); err != nil {
				return fmt.Errorf("create symlink %s -> %s: %w", op.path, op.linkTo, err)
			}
		default:
			return fmt.Errorf("unsupported operation kind %q", op.kind)
		}
	}
	return nil
}

func maybeInitGit(targetRoot string, dryRun bool) error {
	gitDir := filepath.Join(targetRoot, ".git")
	if _, err := os.Stat(gitDir); err == nil {
		fmt.Printf("%s %s (git repo already present)\n", formatAction(dryRun, "skip"), gitDir)
		return nil
	}
	fmt.Printf("%s git init %s\n", formatAction(dryRun, "exec"), targetRoot)
	if dryRun {
		return nil
	}
	cmd := exec.Command("git", "init", targetRoot)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git init %s: %w: %s", targetRoot, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func formatAction(dryRun bool, action string) string {
	if dryRun {
		return "dry-run " + action
	}
	return action
}

func printAssessment(mode Mode, target string, a Assessment) {
	fmt.Printf("mode: %s\n", mode)
	fmt.Printf("target: %s\n", target)
	fmt.Printf("repo-shape: %s\n", a.RepoShape)
	fmt.Printf("signals: code=%d doc=%d\n", a.CodeSignals, a.DocSignals)
	fmt.Printf("existing-artifacts: %s\n", joinOrNone(a.ExistingArtifacts))
	fmt.Printf("collision-risk: %s\n", a.CollisionRisk)
	// The `recommendation:` line was dropped in AC46. It was derived from
	// repo-shape + collision-risk (both still printed) and created perceived
	// contradiction with the final `disposition:` line when conflicts existed.
	// The Assessment.Recommendation struct field stays for any programmatic use.
	//
	// The `collisions:` line is suppressed when it's redundant with
	// `existing-artifacts:` (the common case). Only print when the two
	// differ — e.g., when a file exists at an expected path but is empty.
	if len(a.CollidingArtifacts) > 0 && !slices.Equal(a.CollidingArtifacts, a.ExistingArtifacts) {
		fmt.Printf("collisions: %s\n", strings.Join(a.CollidingArtifacts, ", "))
	}
}

func printEnhancementSummary(report EnhancementReport) {
	fmt.Printf("mode: enhance\n")
	fmt.Printf("reference: %s\n", displayReferenceRoot())
	if len(report.Candidates) == 0 {
		fmt.Println("candidates: none")
		fmt.Printf("%s none detected\n", color.Yel("drift:"))
		return
	}
	counts := countEnhancementCandidates(report.Candidates)
	fmt.Printf("candidates: %d (accept=%d adapt=%d defer=%d reject=%d)\n",
		len(report.Candidates), counts["accept"], counts["adapt"], counts["defer"], counts["reject"])
	for _, c := range report.Candidates {
		fmt.Println(formatCandidateLine(c, report.ReferenceRoot))
	}
	printEnhanceDrift(report.Candidates)
}

func printEnhanceDrift(candidates []EnhancementCandidate) {
	govChanged := 0
	overlayDiff := 0
	for _, c := range candidates {
		if c.Disposition != "accept" && c.Disposition != "adapt" {
			continue
		}
		if c.Area == "base governance" {
			govChanged++
		} else {
			overlayDiff++
		}
	}
	if govChanged == 0 && overlayDiff == 0 {
		fmt.Printf("%s none detected\n", color.Yel("drift:"))
		return
	}
	fmt.Printf("%s %d of %d governance sections changed, %d overlay files differ\n",
		color.Yel("drift:"), govChanged, len(governedSectionNames), overlayDiff)
}

func formatCandidateLine(c EnhancementCandidate, referenceRoot string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "- area=%s path=%s disposition=%s portability=%s", c.Area, displayReferencePath(referenceRoot, c.Path), c.Disposition, c.Portability)
	if c.Section != "" {
		fmt.Fprintf(&b, " section=%s", c.Section)
	}
	if c.TemplateTarget != "" {
		fmt.Fprintf(&b, " template-target=%s", c.TemplateTarget)
	}
	if len(c.DeltaSections) > 0 {
		fmt.Fprintf(&b, " delta-sections=%s", strings.Join(c.DeltaSections, ","))
	}
	if c.ChangeOrigin != "" {
		fmt.Fprintf(&b, " change-origin=%s", c.ChangeOrigin)
	}
	fmt.Fprintf(&b, " collision-impact=%s", c.CollisionImpact)
	fmt.Fprintf(&b, " reason=%s", c.Reason)
	if c.Summary != "" {
		fmt.Fprintf(&b, " summary=%s", c.Summary)
	}
	return b.String()
}

func reviewGovernedSections(tfs fs.FS, referenceRoot string, mmap map[string]ManifestEntry) ([]EnhancementCandidate, error) {
	refPath := filepath.Join(referenceRoot, "AGENTS.md")
	refInfo, err := os.Stat(refPath)
	if err != nil || refInfo.IsDir() {
		return nil, nil
	}

	templateContent, err := fs.ReadFile(tfs, "base/AGENTS.md")
	if err != nil {
		return nil, fmt.Errorf("read template governance file base/AGENTS.md: %w", err)
	}
	refContent, err := os.ReadFile(refPath)
	if err != nil {
		return nil, fmt.Errorf("read reference governance file %s: %w", refPath, err)
	}

	// Three-way pre-filter using manifest
	var sectionOrigin string
	if mmap != nil {
		if entry, ok := mmap["AGENTS.md"]; ok && entry.Kind == "file" {
			userChanged := computeChecksum(string(refContent)) != entry.Checksum
			templateChanged := false
			if entry.SourcePath != "" && entry.SourceChecksum != "" {
				sourceContent, readErr := fs.ReadFile(tfs, entry.SourcePath)
				if readErr == nil {
					templateChanged = computeChecksum(string(sourceContent)) != entry.SourceChecksum
				}
			}
			switch {
			case !userChanged && !templateChanged:
				return nil, nil
			case !userChanged && templateChanged:
				return nil, nil
			case userChanged && !templateChanged:
				sectionOrigin = "user"
			case userChanged && templateChanged:
				sectionOrigin = "both"
			}
		}
	}

	templateSections := sectionMap(parseLevel2Sections(string(templateContent)))
	refSections := sectionMap(parseLevel2Sections(string(refContent)))
	var candidates []EnhancementCandidate
	for _, section := range governedSectionNames {
		refBody, ok := refSections[section]
		if !ok {
			continue
		}
		templateBody := templateSections[section]
		if governanceSectionCovered(section, templateBody, refBody) {
			continue
		}
		portability, disposition, reason := classifyEnhancement(refBody, referenceRoot, "base/AGENTS.md", true)
		candidates = append(candidates, EnhancementCandidate{
			Area:            "base governance",
			Path:            refPath,
			Section:         section,
			Disposition:     disposition,
			Reason:          reason,
			Portability:     portability,
			TemplateTarget:  "base/AGENTS.md",
			Summary:         summarizeSectionDelta(section, refBody),
			CollisionImpact: "medium",
			ChangeOrigin:    sectionOrigin,
		})
		// Subsection drill-down: when a ## section is deferred, check ### subsections individually.
		if disposition == "defer" {
			subsections := parseLevel3Sections(refBody)
			for _, sub := range subsections {
				subPort, subDisp, subReason := classifyEnhancement(sub.Body, referenceRoot, "base/AGENTS.md", true)
				if subDisp == "accept" {
					candidates = append(candidates, EnhancementCandidate{
						Area:            "base governance",
						Path:            refPath,
						Section:         section + " > " + sub.Name,
						Disposition:     subDisp,
						Reason:          subReason,
						Portability:     subPort,
						TemplateTarget:  "base/AGENTS.md",
						Summary:         summarizeSectionDelta(sub.Name, sub.Body),
						CollisionImpact: "medium",
						ChangeOrigin:    sectionOrigin,
					})
				}
			}
		}
	}
	return candidates, nil
}

func reviewMappedFile(tfs fs.FS, repoRoot string, referenceRoot string, item enhancementMapping, mmap map[string]ManifestEntry) (EnhancementCandidate, bool, error) {
	refPath, ok := firstExistingPath(referenceRoot, item.ReferencePaths)
	if !ok {
		return EnhancementCandidate{}, false, nil
	}

	refContent, err := os.ReadFile(refPath)
	if err != nil {
		return EnhancementCandidate{}, false, fmt.Errorf("read reference file %s: %w", refPath, err)
	}

	targetContent, err := readTemplateOrRoot(tfs, repoRoot, item.TemplateTarget)
	targetExists := err == nil
	if err != nil && !errors.Is(err, fs.ErrNotExist) && !errors.Is(err, os.ErrNotExist) {
		return EnhancementCandidate{}, false, fmt.Errorf("read template file %s: %w", item.TemplateTarget, err)
	}
	if targetExists && normalizedEqual(string(refContent), string(targetContent)) {
		return EnhancementCandidate{}, false, nil
	}

	// Three-way comparison using manifest
	var changeOrigin string
	if mmap != nil {
		refRel, _ := filepath.Rel(referenceRoot, refPath)
		refRelSlash := filepath.ToSlash(refRel)
		if entry, ok := mmap[refRelSlash]; ok && entry.Kind == "file" {
			userChanged := computeChecksum(string(refContent)) != entry.Checksum
			templateChanged := false
			if entry.SourcePath != "" && entry.SourceChecksum != "" {
				sourceContent, readErr := readTemplateOrRoot(tfs, repoRoot, entry.SourcePath)
				if readErr == nil {
					templateChanged = computeChecksum(string(sourceContent)) != entry.SourceChecksum
				}
			}
			switch {
			case !userChanged && !templateChanged:
				return EnhancementCandidate{}, false, nil
			case !userChanged && templateChanged:
				changeOrigin = "template"
			case userChanged && !templateChanged:
				changeOrigin = "user"
			case userChanged && templateChanged:
				changeOrigin = "both"
			}
		}
	}

	portability, disposition, reason := classifyEnhancement(string(refContent), referenceRoot, item.TemplateTarget, false)
	collisionImpact := "low"
	if targetExists {
		collisionImpact = "medium"
	}

	if changeOrigin == "template" {
		disposition = "defer"
		reason = "reference file is stale; template has already evolved past the bootstrap baseline"
		portability = "needs-review"
	}

	var deltaSections []string
	if targetExists {
		deltaSections = diffMarkdownSections(string(targetContent), string(refContent))
	}

	return EnhancementCandidate{
		Area:            item.Area,
		Path:            refPath,
		Disposition:     disposition,
		Reason:          reason,
		Portability:     portability,
		TemplateTarget:  item.TemplateTarget,
		Summary:         summarizeFileContent(refPath, string(refContent)),
		CollisionImpact: collisionImpact,
		DeltaSections:   deltaSections,
		ChangeOrigin:    changeOrigin,
	}, true, nil
}

func firstExistingPath(root string, rels []string) (string, bool) {
	for _, rel := range rels {
		path := filepath.Join(root, rel)
		info, err := os.Stat(path)
		if err == nil && !info.IsDir() {
			return path, true
		}
	}
	return "", false
}

func parseLevel2Sections(content string) []markdownSection {
	lines := strings.Split(content, "\n")
	var sections []markdownSection
	var preamble strings.Builder
	inPreamble := true
	current := markdownSection{}
	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			if inPreamble {
				preambleText := strings.TrimSpace(preamble.String())
				if preambleText != "" {
					sections = append(sections, markdownSection{Name: "(preamble)", Body: preambleText})
				}
				inPreamble = false
			}
			if current.Name != "" {
				current.Body = strings.TrimSpace(current.Body)
				sections = append(sections, current)
			}
			current = markdownSection{Name: strings.TrimSpace(strings.TrimPrefix(line, "## "))}
			continue
		}
		if inPreamble {
			preamble.WriteString(line)
			preamble.WriteString("\n")
			continue
		}
		if current.Name == "" {
			continue
		}
		if current.Body == "" {
			current.Body = line
		} else {
			current.Body += "\n" + line
		}
	}
	if inPreamble {
		// File has no ## sections at all — no preamble section emitted
		// (whole-file scoring handles this case)
	} else if current.Name != "" {
		current.Body = strings.TrimSpace(current.Body)
		sections = append(sections, current)
	}
	return sections
}

// patchGovernedSections merges missing governed sections from templateContent into existingContent.
// Returns the patched content and true if any sections were added, or the original content and false
// if all governed sections are already present.
func patchGovernedSections(existingContent, templateContent string) (string, bool) {
	existingSections := sectionMap(parseLevel2Sections(existingContent))
	templateSections := sectionMap(parseLevel2Sections(templateContent))

	var missing []markdownSection
	for _, name := range governedSectionNames {
		if _, exists := existingSections[name]; exists {
			continue
		}
		if body, inTemplate := templateSections[name]; inTemplate {
			missing = append(missing, markdownSection{Name: name, Body: body})
		}
	}

	if len(missing) == 0 {
		return existingContent, false
	}

	var b strings.Builder
	b.WriteString(strings.TrimRight(existingContent, "\n"))
	for _, section := range missing {
		b.WriteString("\n\n## ")
		b.WriteString(section.Name)
		b.WriteString("\n\n")
		b.WriteString(section.Body)
	}
	b.WriteString("\n")
	return b.String(), true
}

func sectionMap(sections []markdownSection) map[string]string {
	out := make(map[string]string, len(sections))
	for _, section := range sections {
		out[section.Name] = section.Body
	}
	return out
}

func normalizedEqual(a, b string) bool {
	return normalizeText(a) == normalizeText(b)
}

func governanceSectionCovered(section, templateBody, referenceBody string) bool {
	if normalizedEqual(templateBody, referenceBody) {
		return true
	}
	templateSignals := sectionSignals(section, templateBody)
	referenceSignals := sectionSignals(section, referenceBody)
	if len(referenceSignals) == 0 || len(templateSignals) == 0 {
		return false
	}
	for signal := range referenceSignals {
		if !templateSignals[signal] {
			return false
		}
	}
	return constraintsCovered(templateBody, referenceBody)
}

func normalizeText(value string) string {
	lines := strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

type signalDef struct {
	Name  string
	Match func(text string) bool
}

var defaultSignalDefs = map[string][]signalDef{
	"Interaction Mode": {
		{"exploratory-discussion-default", func(t string) bool { return containsAll(t, "exploratory", "discussion") }},
		{"changes-need-authorization", func(t string) bool {
			return (containsAll(t, "create", "artifacts") || containsAll(t, "make", "changes")) && containsAny(t, "authoriz", "authoris")
		}},
		{"minimal-change-on-authorization", func(t string) bool {
			return containsAll(t, "smallest", "change") || containsAll(t, "minimal", "change")
		}},
		{"surface-assumptions", func(t string) bool { return containsAny(t, "assumptions", "ambiguities", "missing context") }},
	},
	"Approval Boundaries": {
		{"destructive-needs-approval", func(t string) bool { return containsAny(t, "destructive") }},
		{"release-needs-approval", func(t string) bool { return containsAny(t, "release", "publish", "deploy") }},
		{"governance-needs-approval", func(t string) bool {
			return containsAny(t, "governance files", "ci", "secrets", "external integrations")
		}},
	},
	"Review Style": {
		{"review-findings-first", func(t string) bool { return containsAll(t, "findings", "before") }},
		{"review-bugs-regressions", func(t string) bool { return containsAny(t, "bugs", "regressions", "missing tests") }},
		{"review-evidence", func(t string) bool { return containsAny(t, "concrete evidence", "file paths", "coverage") }},
	},
	"File-Change Discipline": {
		{"targeted-edits", func(t string) bool { return containsAny(t, "targeted edits", "broad rewrites") }},
		{"preserve-user-changes", func(t string) bool { return containsAny(t, "preserve user changes", "unrelated local modifications") }},
		{"docs-in-same-pass", func(t string) bool { return containsAny(t, "directly affected docs", "self-contained") }},
	},
	"Release Or Publish Triggers": {
		{"release-only-on-request", func(t string) bool {
			return containsAny(t, "explicitly asks", "explicitly asks for it", "release-scoped")
		}},
		{"release-artifacts-same-pass", func(t string) bool { return containsAny(t, "required release artifacts", "same pass") }},
	},
	"Documentation Update Expectations": {
		{"docs-align-with-behavior", func(t string) bool { return containsAny(t, "aligned with behavior", "drift") }},
		{"update-user-facing-docs", func(t string) bool {
			return containsAny(t, "user-facing docs", "operating instructions", "setup", "workflows")
		}},
		{"update-arch-plan-style-when-material", func(t string) bool { return containsAny(t, "architecture", "planning", "style docs") }},
	},
}

func sectionSignals(section, body string) map[string]bool {
	text := normalizedSignalText(body)
	signals := map[string]bool{}
	for _, def := range defaultSignalDefs[section] {
		if def.Match(text) {
			signals[def.Name] = true
		}
	}
	return signals
}

func normalizedSignalText(value string) string {
	lower := strings.ToLower(normalizeText(value))
	replacer := strings.NewReplacer(
		"-", " ",
		"_", " ",
		"`", " ",
		".", " ",
		",", " ",
		":", " ",
		";", " ",
		"(", " ",
		")", " ",
		"/", " ",
	)
	return replacer.Replace(lower)
}

func containsAll(text string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(text, strings.ToLower(part)) {
			return false
		}
	}
	return true
}

func containsAny(text string, parts ...string) bool {
	for _, part := range parts {
		if strings.Contains(text, strings.ToLower(part)) {
			return true
		}
	}
	return false
}

func diffMarkdownSections(templateContent, referenceContent string) []string {
	templateSections := parseLevel2Sections(templateContent)
	referenceSections := parseLevel2Sections(referenceContent)

	if len(templateSections) == 0 || len(referenceSections) == 0 {
		return nil
	}

	templateMap := sectionMap(templateSections)
	referenceMap := sectionMap(referenceSections)
	seen := map[string]bool{}
	var deltas []string

	for _, rs := range referenceSections {
		seen[rs.Name] = true
		tb, ok := templateMap[rs.Name]
		if !ok {
			deltas = append(deltas, rs.Name)
			continue
		}
		if !normalizedEqual(tb, rs.Body) {
			deltas = append(deltas, rs.Name)
		}
	}

	for _, ts := range templateSections {
		if seen[ts.Name] {
			continue
		}
		if _, ok := referenceMap[ts.Name]; !ok {
			deltas = append(deltas, ts.Name)
		}
	}

	return deltas
}

func extractConstraints(body string) []string {
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	var constraints []string
	var current strings.Builder

	flush := func() {
		text := strings.TrimSpace(current.String())
		if text != "" {
			constraints = append(constraints, normalizedSignalText(text))
		}
		current.Reset()
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if text, ok := stripListPrefix(trimmed); ok {
			flush()
			current.WriteString(text)
		} else {
			if current.Len() > 0 {
				current.WriteString(" ")
			}
			current.WriteString(trimmed)
		}
	}
	flush()
	return constraints
}

func stripListPrefix(line string) (string, bool) {
	if strings.HasPrefix(line, "- ") {
		return line[2:], true
	}
	if strings.HasPrefix(line, "* ") {
		return line[2:], true
	}
	for i, ch := range line {
		if ch >= '0' && ch <= '9' {
			continue
		}
		if ch == '.' && i > 0 && i+1 < len(line) && line[i+1] == ' ' {
			return line[i+2:], true
		}
		break
	}
	return "", false
}

func constraintsCovered(templateBody, referenceBody string) bool {
	templateConstraints := extractConstraints(templateBody)
	referenceConstraints := extractConstraints(referenceBody)

	if len(referenceConstraints) == 0 {
		return true
	}

	for _, rc := range referenceConstraints {
		if !slices.Contains(templateConstraints, rc) {
			return false
		}
	}
	return true
}

type classificationContext struct {
	Content        string
	ReferenceRoot  string
	TemplateTarget string
	Governance     bool
}

type classificationRule struct {
	Name        string
	Priority    int // lower wins; first matching rule by priority applies
	Match       func(ctx classificationContext) bool
	Portability string
	Disposition string
	Reason      string
}

type markerRule struct {
	Name  string
	Match func(content, referenceRoot string) bool
}

var defaultMarkerRules = []markerRule{
	{"mentions reference repo name", func(content, refRoot string) bool {
		lower := strings.ToLower(content)
		refName := strings.ToLower(filepath.Base(refRoot))
		return refName != "" && strings.Contains(lower, refName)
	}},
	{"contains absolute user path", func(content, _ string) bool {
		return strings.Contains(content, "/Users/") ||
			strings.Contains(content, "/home/") ||
			strings.Contains(content, "\\Users\\")
	}},
}

var defaultClassificationRules = []classificationRule{
	{
		Name:     "project-specific",
		Priority: 100,
		Match: func(ctx classificationContext) bool {
			return len(projectSpecificMarkers(ctx.Content, ctx.ReferenceRoot)) > 0
		},
		Portability: "project-specific", Disposition: "defer",
		Reason: "content appears tied to the reference repo and should not be imported directly",
	},
	{
		Name:        "governance",
		Priority:    200,
		Match:       func(ctx classificationContext) bool { return ctx.Governance },
		Portability: "portable", Disposition: "accept",
		Reason: "section-level governance delta is reusable enough to review directly against the base contract",
	},
	{
		Name:     "workflow-helper",
		Priority: 300,
		Match: func(ctx classificationContext) bool {
			return strings.HasSuffix(ctx.TemplateTarget, ".go.tmpl") || strings.HasSuffix(ctx.TemplateTarget, ".sh.tmpl") || ctx.TemplateTarget == "TEMPLATE_VERSION"
		},
		Portability: "portable", Disposition: "accept",
		Reason: "workflow helper or release artifact is concrete and portable enough for direct template review",
	},
	{
		Name:        "default",
		Priority:    9999,
		Match:       func(_ classificationContext) bool { return true },
		Portability: "needs-review", Disposition: "adapt",
		Reason: "artifact may contain reusable structure, but the content should be adapted before it becomes template baseline",
	},
}

func classifyEnhancement(content, referenceRoot, templateTarget string, governance bool) (string, string, string) {
	ctx := classificationContext{
		Content:        content,
		ReferenceRoot:  referenceRoot,
		TemplateTarget: templateTarget,
		Governance:     governance,
	}
	sorted := make([]classificationRule, len(defaultClassificationRules))
	copy(sorted, defaultClassificationRules)
	slices.SortStableFunc(sorted, func(a, b classificationRule) int {
		return cmp.Compare(a.Priority, b.Priority)
	})
	for _, rule := range sorted {
		if rule.Match(ctx) {
			return rule.Portability, rule.Disposition, rule.Reason
		}
	}
	return "needs-review", "adapt", "no classification rule matched"
}

func projectSpecificMarkers(content, referenceRoot string) []string {
	var markers []string
	for _, rule := range defaultMarkerRules {
		if rule.Match(content, referenceRoot) {
			markers = append(markers, rule.Name)
		}
	}
	return markers
}

func summarizeSectionDelta(section, body string) string {
	first := firstNonEmptyLine(body)
	if first == "" {
		return fmt.Sprintf("section %q differs from the template baseline", section)
	}
	return fmt.Sprintf("section %q begins with %q", section, truncateForSummary(first))
}

func summarizeFileContent(path, content string) string {
	headings := extractHeadings(content)
	switch {
	case len(headings) > 0:
		return fmt.Sprintf("headings: %s", strings.Join(headings, ", "))
	default:
		first := firstNonEmptyLine(content)
		if first == "" {
			return fmt.Sprintf("%s is present but mostly empty", filepath.Base(path))
		}
		return fmt.Sprintf("starts with %q", truncateForSummary(first))
	}
}

func extractHeadings(content string) []string {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	var headings []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if headingText, ok := markdownHeading(trimmed); ok {
			headings = append(headings, headingText)
		}
		if len(headings) == 3 {
			break
		}
	}
	return headings
}

func markdownHeading(line string) (string, bool) {
	if !strings.HasPrefix(line, "#") {
		return "", false
	}
	hashes := 0
	for hashes < len(line) && line[hashes] == '#' {
		hashes++
	}
	if hashes >= len(line) || line[hashes] != ' ' {
		return "", false
	}
	return strings.TrimSpace(line[hashes:]), true
}

func firstNonEmptyLine(content string) string {
	for line := range strings.SplitSeq(strings.ReplaceAll(content, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

func truncateForSummary(value string) string {
	if len(value) <= 72 {
		return value
	}
	return strings.TrimSpace(value[:69]) + "..."
}

func candidateRank(c EnhancementCandidate) int {
	switch {
	case c.Disposition == "accept" && c.Portability == "portable":
		return 1
	case c.Disposition == "accept" && c.Portability == "needs-review":
		return 2
	case c.Disposition == "adapt" && c.Portability == "portable":
		return 3
	case c.Disposition == "adapt" && c.Portability == "needs-review":
		return 4
	default:
		return 99
	}
}

func isActionable(c EnhancementCandidate) bool {
	return c.Disposition == "accept" || c.Disposition == "adapt"
}

func selectActionableCandidates(candidates []EnhancementCandidate) (selected EnhancementCandidate, deferred []EnhancementCandidate, ok bool) {
	var actionable []EnhancementCandidate
	for _, c := range candidates {
		if isActionable(c) {
			actionable = append(actionable, c)
		}
	}
	if len(actionable) == 0 {
		return EnhancementCandidate{}, nil, false
	}
	best := 0
	for i := 1; i < len(actionable); i++ {
		if candidateRank(actionable[i]) < candidateRank(actionable[best]) {
			best = i
		}
	}
	selected = actionable[best]
	for i, c := range actionable {
		if i != best {
			deferred = append(deferred, c)
		}
	}
	return selected, deferred, true
}

var workingACFileRe = regexp.MustCompile(`^ac(\d+)-.*\.md$`)

func isWorkingACFile(name string) bool {
	return workingACFileRe.MatchString(name)
}

func isACKeeperFile(name string) bool {
	return name == "ac-template.md"
}

func nextACNumber(docsDir string) (int, error) {
	entries, err := os.ReadDir(docsDir)
	if err != nil {
		return 1, nil
	}
	max := 0
	for _, entry := range entries {
		name := entry.Name()
		match := workingACFileRe.FindStringSubmatch(name)
		if match == nil {
			continue
		}
		num, err := strconv.Atoi(match[1])
		if err != nil {
			continue
		}
		if num > max {
			max = num
		}
	}
	return max + 1, nil
}

func acSlug(c EnhancementCandidate) string {
	base := c.Area
	if c.Section != "" {
		base = c.Section
	}
	slug := strings.ToLower(base)
	slug = strings.ReplaceAll(slug, " ", "-")
	var clean []byte
	for _, ch := range []byte(slug) {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-' {
			clean = append(clean, ch)
		}
	}
	result := string(clean)
	if len(result) > 40 {
		result = result[:40]
	}
	return strings.TrimRight(result, "-")
}

func renderACDoc(selected EnhancementCandidate, deferred []EnhancementCandidate, report EnhancementReport, acNumber int) string {
	var b strings.Builder

	title := fmt.Sprintf("Enhance: %s", selected.Area)
	if selected.Section != "" {
		title = fmt.Sprintf("Enhance: %s — %s", selected.Area, selected.Section)
	}
	fmt.Fprintf(&b, "# AC%d %s\n\n", acNumber, title)

	b.WriteString("## Objective Fit\n\n")
	b.WriteString("1. Improve the template based on a governed reference repo review.\n")
	b.WriteString("2. This is the highest-priority actionable enhancement from the latest enhance run.\n")
	fmt.Fprintf(&b, "3. The enhance review classified this candidate as `%s` with portability `%s`.\n", selected.Disposition, selected.Portability)
	b.WriteString("4. Direct roadmap work — aligns with R3 (safe refresh path).\n\n")

	b.WriteString("## Summary\n\n")
	fmt.Fprintf(&b, "%s\n\n", selected.Reason)
	if selected.Summary != "" {
		fmt.Fprintf(&b, "Evidence: %s\n\n", selected.Summary)
	}
	if len(selected.DeltaSections) > 0 {
		fmt.Fprintf(&b, "Changed sections: %s\n\n", strings.Join(selected.DeltaSections, ", "))
	}

	b.WriteString("## In Scope\n\n")
	if selected.TemplateTarget != "" {
		fmt.Fprintf(&b, "- Review and update `%s`\n", selected.TemplateTarget)
	}
	if len(selected.DeltaSections) > 0 {
		for _, ds := range selected.DeltaSections {
			fmt.Fprintf(&b, "- Section: `%s`\n", ds)
		}
	} else if selected.Section != "" {
		fmt.Fprintf(&b, "- Section: `%s`\n", selected.Section)
	}
	fmt.Fprintf(&b, "- Area: %s\n", selected.Area)
	fmt.Fprintf(&b, "- Disposition: `%s`, Portability: `%s`\n\n", selected.Disposition, selected.Portability)

	b.WriteString("## Out Of Scope\n\n")
	b.WriteString("- Auto-applying template changes\n")
	b.WriteString("- Changes unrelated to this specific enhancement candidate\n\n")

	b.WriteString("## Implementation Notes\n\n")
	fmt.Fprintf(&b, "- Source: `%s`\n", displayReferencePath(report.ReferenceRoot, selected.Path))
	fmt.Fprintf(&b, "- Collision impact: `%s`\n", selected.CollisionImpact)
	if selected.ChangeOrigin != "" {
		fmt.Fprintf(&b, "- Change origin: `%s`\n", selected.ChangeOrigin)
	}
	b.WriteString("\n")

	b.WriteString("## Acceptance Tests\n\n")
	b.WriteString("- [Manual] Verify the template target reflects the enhancement\n")
	b.WriteString("- [Manual] Verify generated repos benefit from the change\n\n")

	b.WriteString("## Documentation Updates\n\n")
	b.WriteString("- Update any docs affected by the template target change\n\n")

	if len(deferred) > 0 {
		b.WriteString("## Deferred Candidates\n\n")
		b.WriteString("The following actionable candidates were identified but not selected for this AC:\n\n")
		for _, d := range deferred {
			fmt.Fprintf(&b, "- **%s**", d.Area)
			if d.Section != "" {
				fmt.Fprintf(&b, " — %s", d.Section)
			}
			fmt.Fprintf(&b, ": `%s` (%s, %s)", d.Disposition, d.Portability, d.CollisionImpact)
			if d.TemplateTarget != "" {
				fmt.Fprintf(&b, " target=`%s`", d.TemplateTarget)
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("## Status\n\nPENDING\n")
	return b.String()
}

func displayReferenceRoot() string {
	return "<reference-root>"
}

func displayReferencePath(referenceRoot, path string) string {
	if referenceRoot == "" || path == "" {
		return filepath.ToSlash(path)
	}
	rel, err := filepath.Rel(referenceRoot, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(path)
	}
	if rel == "." {
		return displayReferenceRoot()
	}
	return displayReferenceRoot() + "/" + filepath.ToSlash(rel)
}

func countEnhancementCandidates(candidates []EnhancementCandidate) map[string]int {
	counts := map[string]int{
		"accept": 0,
		"adapt":  0,
		"defer":  0,
		"reject": 0,
	}
	for _, candidate := range candidates {
		counts[candidate.Disposition]++
	}
	return counts
}

type structuralNote struct {
	section      string
	observation  string
	existingBody string
	templateBody string
}

type collisionScore struct {
	path                   string // target file path
	recommendation         string // "keep", "adopt", "accept"
	reason                 string
	existingLines          int
	proposedLines          int
	existingSections       int
	proposedSections       int
	missingSections        []string          // sections in proposed but not in existing
	changedSections        []string          // shared sections with different content (markdown only)
	changedClassifications map[string]string // section name → "structural" or "cosmetic"
	contentChanged         bool              // true when template source changed and existing differs from new template
	proposedContent        string            // the template content for the review doc
	governancePatch        string            // non-empty if this is an AGENTS.md patch with missing sections
	structuralNotes        []structuralNote  // section-level structural observations
	sectionRenames         map[string]string // old name → new name (detected renames)
	standingDrift          bool              // true when file differs from template but template hasn't changed since last sync
	driftSections          []string          // sections that differ from template (standing drift only)
}

func countLines(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

func markdownSectionNames(content string) []string {
	var names []string
	for line := range strings.SplitSeq(content, "\n") {
		if after, ok := strings.CutPrefix(line, "## "); ok {
			names = append(names, strings.TrimSpace(after))
		}
	}
	return names
}

func scoreOverlayCollision(existingPath string, proposedContent string, oldSourceChecksum string, newSourceChecksum string) collisionScore {
	score := collisionScore{
		path:            existingPath,
		proposedLines:   countLines(proposedContent),
		proposedContent: proposedContent,
	}

	existingBytes, err := os.ReadFile(existingPath)
	if err != nil {
		// File doesn't exist — accept the proposed content
		score.recommendation = "accept"
		score.reason = "file does not exist in target"
		return score
	}
	existingContent := string(existingBytes)
	score.existingLines = countLines(existingContent)

	// Identical file detection
	if existingContent == proposedContent {
		score.recommendation = "keep"
		score.reason = "identical to template"
		return score
	}

	// Detect whether the template source changed since last sync.
	templateChanged := oldSourceChecksum != "" && newSourceChecksum != "" && oldSourceChecksum != newSourceChecksum
	// Even if the template changed, the repo may have already absorbed the
	// changes manually. Check whether existing content still differs from the
	// new template. If it matches, no content-change flag is needed.
	alreadyAbsorbed := templateChanged && existingContent == proposedContent // (caught by identical check above, but defensive)

	isMarkdown := strings.HasSuffix(existingPath, ".md") || strings.HasSuffix(existingPath, ".md.tmpl")
	if !isMarkdown {
		if templateChanged && !alreadyAbsorbed {
			score.recommendation = "adopt"
			score.reason = fmt.Sprintf("template changed since last sync (non-markdown, existing %d lines, proposed %d lines)", score.existingLines, score.proposedLines)
			score.contentChanged = true
			return score
		}
		score.recommendation = "keep"
		score.reason = fmt.Sprintf("non-markdown file (existing %d lines, proposed %d lines)", score.existingLines, score.proposedLines)
		if !templateChanged && existingContent != proposedContent {
			score.standingDrift = true
		}
		return score
	}

	existingNames := markdownSectionNames(existingContent)
	proposedNames := markdownSectionNames(proposedContent)
	score.existingSections = len(existingNames)
	score.proposedSections = len(proposedNames)

	// Check for sections in proposed that are missing from existing
	existingSet := make(map[string]bool, len(existingNames))
	for _, name := range existingNames {
		existingSet[name] = true
	}
	for _, name := range proposedNames {
		if !existingSet[name] {
			score.missingSections = append(score.missingSections, name)
		}
	}

	// Section rename detection
	existingMap := sectionMap(parseLevel2Sections(existingContent))
	proposedMap := sectionMap(parseLevel2Sections(proposedContent))
	score.sectionRenames = detectSectionRenames(existingNames, proposedNames, existingMap, proposedMap)

	// Structural comparison for matching sections
	score.structuralNotes = compareStructure(existingContent, proposedContent)

	// Detect section-level content changes when template source changed.
	if templateChanged {
		score.changedSections = detectChangedSections(existingContent, proposedContent)
		if len(score.changedSections) > 0 {
			score.contentChanged = true
			score.changedClassifications = classifySections(existingContent, proposedContent, score.changedSections)
		}
	} else if existingContent != proposedContent {
		// Template unchanged since last sync but file still differs — standing drift.
		driftSections := detectChangedSections(existingContent, proposedContent)
		if len(driftSections) > 0 {
			score.standingDrift = true
			score.driftSections = driftSections
		}
	}

	// Decision rules
	if score.existingLines >= 2*score.proposedLines {
		if score.contentChanged {
			score.recommendation = "adopt"
			score.reason = fmt.Sprintf("existing is more developed (%d lines vs %d proposed) but template sections changed: %s", score.existingLines, score.proposedLines, taggedSectionList(score.changedSections, score.changedClassifications))
			return score
		}
		score.recommendation = "keep"
		score.reason = fmt.Sprintf("existing is more developed (%d lines vs %d proposed)", score.existingLines, score.proposedLines)
		return score
	}
	if score.existingSections > score.proposedSections {
		if score.contentChanged {
			score.recommendation = "adopt"
			score.reason = fmt.Sprintf("existing has richer structure (%d sections vs %d proposed) but template sections changed: %s", score.existingSections, score.proposedSections, taggedSectionList(score.changedSections, score.changedClassifications))
			return score
		}
		score.recommendation = "keep"
		score.reason = fmt.Sprintf("existing has richer structure (%d sections vs %d proposed)", score.existingSections, score.proposedSections)
		return score
	}
	// When existing has at least as many sections as proposed, different section
	// names likely mean the existing file covers the same content under more
	// specific headings — not a real cherry-pick opportunity.
	if score.existingSections >= score.proposedSections && len(score.missingSections) > 0 {
		if score.contentChanged {
			score.recommendation = "adopt"
			score.reason = fmt.Sprintf("template sections changed: %s", taggedSectionList(score.changedSections, score.changedClassifications))
			return score
		}
		score.recommendation = "keep"
		score.reason = fmt.Sprintf("existing covers same content under different headings (%d sections vs %d proposed)", score.existingSections, score.proposedSections)
		return score
	}
	if len(score.missingSections) > 0 {
		score.recommendation = "adopt"
		score.reason = fmt.Sprintf("proposed adds sections: %s", strings.Join(score.missingSections, ", "))
		return score
	}

	if score.contentChanged {
		score.recommendation = "adopt"
		score.reason = fmt.Sprintf("template sections changed: %s", taggedSectionList(score.changedSections, score.changedClassifications))
		return score
	}
	score.recommendation = "keep"
	score.reason = fmt.Sprintf("similar content (%d lines vs %d proposed, %d sections vs %d)", score.existingLines, score.proposedLines, score.existingSections, score.proposedSections)
	return score
}

// promoteStandingDrift overrides keep to "adopt" when the file has un-adopted
// template differences from previous sync rounds. Called after initial scoring.
func promoteStandingDrift(score *collisionScore) {
	if !score.standingDrift {
		return
	}
	if score.recommendation == "keep" {
		if len(score.driftSections) > 0 {
			score.recommendation = "adopt"
			score.reason = fmt.Sprintf("un-adopted template differences in: %s", strings.Join(score.driftSections, ", "))
		} else {
			score.recommendation = "adopt"
			score.reason = "file differs from template baseline (unchanged since last sync)"
		}
	}
}

// promoteStructuralNotes overrides keep to "adopt" when the file has structural
// observations (e.g., subsection nesting deeper than template). Called after
// promoteStandingDrift.
func promoteStructuralNotes(score *collisionScore) {
	if len(score.structuralNotes) == 0 {
		return
	}
	if score.recommendation == "keep" {
		var sections []string
		for _, n := range score.structuralNotes {
			sections = append(sections, n.section)
		}
		score.recommendation = "adopt"
		score.reason = fmt.Sprintf("structural alignment needed in: %s", strings.Join(sections, ", "))
	}
}

// scaffoldFiles are the known template files that contain placeholder content
// intended to be replaced by the repo. When the repo has replaced the
// placeholders, these files should not score as adopt.
var scaffoldFiles = map[string]bool{
	"README.md": true,
	"arch.md":   true,
	"plan.md":   true,
}

// knownMergeTargets are files where the correct operator action is to union
// template patterns into the existing file rather than replacing wholesale.
// When such a file scores as `adopt`, the Adoption Items entry appends a
// merge hint so the operator doesn't blindly copy the template version over
// their own patterns. A real `merge` recommendation category is deferred to
// a future IE; this hint is the minimal workable improvement.
var knownMergeTargets = map[string]bool{
	".gitignore": true,
}

// scaffoldMarkers are placeholder strings from template scaffold files.
// If the proposed content contains any of these and the existing content
// does not, the repo has replaced the scaffold with real content.
var scaffoldMarkers = []string{
	"State why this repo exists",
	"Document the system's major components",
	"## Replace Me",
	"project-specific anti-patterns and guardrails here",
	"active work items only; remove when shipped",
}

// demoteScaffold overrides adopt to keep for known scaffold files (README.md,
// arch.md, plan.md) when the proposed content contains placeholder markers
// and the existing content does not. Only applies when the adopt reason is
// scaffold-driven (standing drift or cherry-pick), not content-changed or structural.
func demoteScaffold(score *collisionScore) {
	if score.recommendation != "adopt" {
		return
	}
	// Only demote for scaffold-driven reasons, not template evolution or structural
	if score.contentChanged || len(score.structuralNotes) > 0 {
		return
	}
	base := filepath.Base(score.path)
	if !scaffoldFiles[base] {
		return
	}
	existing, err := os.ReadFile(score.path)
	if err != nil {
		return
	}
	existingStr := string(existing)
	proposedHasMarker := false
	for _, marker := range scaffoldMarkers {
		if strings.Contains(score.proposedContent, marker) {
			proposedHasMarker = true
			break
		}
	}
	if !proposedHasMarker {
		return
	}
	existingHasMarker := false
	for _, marker := range scaffoldMarkers {
		if strings.Contains(existingStr, marker) {
			existingHasMarker = true
			break
		}
	}
	if existingHasMarker {
		return // repo still has placeholder content — adopt is appropriate
	}
	score.recommendation = "keep"
	score.reason = "repo has replaced template scaffolding with project content"
}

// demoteExtractedPackage overrides adopt to keep for non-markdown files where
// the existing file is ≤ ¼ the lines of the proposed content and imports a
// local package (module-path-prefixed). This indicates the repo has extracted
// the template's monolithic logic into a reusable package.
func demoteExtractedPackage(score *collisionScore, modulePath string) {
	if score.recommendation != "adopt" {
		return
	}
	if modulePath == "" {
		return
	}
	isMarkdown := strings.HasSuffix(score.path, ".md") || strings.HasSuffix(score.path, ".md.tmpl")
	if isMarkdown {
		return
	}
	if score.existingLines == 0 || score.proposedLines == 0 {
		return
	}
	if score.existingLines > score.proposedLines/4 {
		return
	}
	existing, err := os.ReadFile(score.path)
	if err != nil {
		return
	}
	// Check for an import of a local package under the repo's module path
	importPrefix := fmt.Sprintf(`"%s/`, modulePath)
	for line := range strings.SplitSeq(string(existing), "\n") {
		if strings.Contains(strings.TrimSpace(line), importPrefix) {
			score.recommendation = "keep"
			score.reason = "repo has extracted template logic into a local package"
			return
		}
	}
}

func scoreGovernanceCollision(op operation, oldSourceChecksum string, newSourceChecksum string) collisionScore {
	existingContent, err := os.ReadFile(op.path)
	if err != nil {
		// File doesn't exist — accept (write directly)
		return collisionScore{
			path:           op.path,
			recommendation: "accept",
			reason:         "file does not exist in target",
			proposedLines:  countLines(op.content),
		}
	}

	patched, changed := patchGovernedSections(string(existingContent), op.content)
	if !changed {
		// All governed sections present. Check if template content changed
		// within those sections since last sync.
		templateChanged := oldSourceChecksum != "" && newSourceChecksum != "" && oldSourceChecksum != newSourceChecksum
		if templateChanged {
			changedGoverned := detectChangedGovernedSections(string(existingContent), op.content)
			if len(changedGoverned) > 0 {
				cls := classifySections(string(existingContent), op.content, changedGoverned)
				return collisionScore{
					path:                   op.path,
					recommendation:         "adopt",
					reason:                 fmt.Sprintf("governed sections changed: %s", taggedSectionList(changedGoverned, cls)),
					existingLines:          countLines(string(existingContent)),
					proposedLines:          countLines(op.content),
					changedSections:        changedGoverned,
					changedClassifications: cls,
					contentChanged:         true,
					proposedContent:        op.content,
				}
			}
		}
		return collisionScore{
			path:           op.path,
			recommendation: "keep",
			reason:         "all governed sections already present",
			existingLines:  countLines(string(existingContent)),
			proposedLines:  countLines(op.content),
		}
	}

	// Find which sections are missing
	existingSections := sectionMap(parseLevel2Sections(string(existingContent)))
	var missing []string
	for _, name := range governedSectionNames {
		if _, exists := existingSections[name]; !exists {
			missing = append(missing, name)
		}
	}

	return collisionScore{
		path:            op.path,
		recommendation:  "adopt",
		reason:          fmt.Sprintf("missing governed sections: %s", strings.Join(missing, ", ")),
		existingLines:   countLines(string(existingContent)),
		proposedLines:   countLines(op.content),
		governancePatch: patched,
		missingSections: missing,
	}
}

// detectChangedGovernedSections compares governed section bodies between
// existing and template content. Returns names of sections where body differs.
func detectChangedGovernedSections(existingContent, templateContent string) []string {
	existingMap := sectionMap(parseLevel2Sections(existingContent))
	templateMap := sectionMap(parseLevel2Sections(templateContent))

	var changed []string
	for _, name := range governedSectionNames {
		existingBody, eOk := existingMap[name]
		templateBody, tOk := templateMap[name]
		if !eOk || !tOk {
			continue
		}
		if strings.TrimSpace(existingBody) != strings.TrimSpace(templateBody) {
			changed = append(changed, name)
		}
	}
	return changed
}

func renderSyncReview(targetDir string, scores []collisionScore, conflicts []conflict, oldVersion, newVersion string) string {
	relPath := func(absPath string) string {
		if targetDir != "" {
			if r, err := filepath.Rel(targetDir, absPath); err == nil {
				return r
			}
		}
		return filepath.Base(absPath)
	}
	var b strings.Builder
	fmt.Fprintln(&b, "# Governa Sync Review")
	fmt.Fprintln(&b, "")
	if oldVersion != "" && newVersion != "" && oldVersion != newVersion {
		fmt.Fprintf(&b, "Template version: %s → %s\n\n", oldVersion, newVersion)
	} else if newVersion != "" {
		fmt.Fprintf(&b, "Template version: %s\n\n", newVersion)
	}
	fmt.Fprintln(&b, "Generated by `governa sync`. Sync automatically updates `TEMPLATE_VERSION` and `.governa-manifest` to record the current template baseline — these are bookkeeping, not review items. This file (`governa-sync-review.md`) and `.governa-proposed/` are working artifacts, not intended to be committed.")
	fmt.Fprintln(&b, "")
	fmt.Fprintln(&b, "## Evaluation Methodology")
	fmt.Fprintln(&b, "")
	fmt.Fprintln(&b, "**Default to adopting template content.** Keep existing content only when it is repo-specific and the reason is documented. Adoptions are non-trivial changes to governance docs — draft an AC before applying them so the work gets scoped and reviewed through the normal development cycle.")
	fmt.Fprintln(&b, "")
	fmt.Fprintln(&b, "The repo agent must follow these steps for every `adopt` item:")
	fmt.Fprintln(&b, "")
	fmt.Fprintln(&b, "1. **Structure pass — match the template shape.**")
	fmt.Fprintln(&b, "   - The agent must adopt template section names and ordering unless the repo has a documented reason to diverge.")
	fmt.Fprintln(&b, "   - The agent must collapse repo subsections that add formatting but not semantic distinction to match the template's flatter structure.")
	fmt.Fprintln(&b, "   - If collapsing would lose genuinely repo-specific detail, the agent must keep it inline under the template's section rather than adding new headings.")
	fmt.Fprintln(&b, "")
	fmt.Fprintln(&b, "2. **Content pass — adopt template wording as the base.**")
	fmt.Fprintln(&b, "   - For each section, the agent must start from the template text in `.governa-proposed/<file>`.")
	fmt.Fprintln(&b, "   - The agent must layer repo-specific additions (project names, file paths, domain rules) on top.")
	fmt.Fprintln(&b, "   - If the template wording covers the same intent with better or more general phrasing, the agent must adopt it and drop the repo's version.")
	fmt.Fprintln(&b, "   - The agent must not sacrifice detail that is definitively specific to the repo.")
	fmt.Fprintln(&b, "")
	fmt.Fprintln(&b, "3. **Residual check — minimize future drift.**")
	fmt.Fprintln(&b, "   - After edits, each remaining difference from the template must be explainable as repo-specific with a clear reason.")
	fmt.Fprintln(&b, "   - If a difference has no repo-specific justification, the agent must adopt the template version.")
	fmt.Fprintln(&b, "")
	fmt.Fprintln(&b, "4. **Role files pass — adopt directory and file renames.**")
	fmt.Fprintln(&b, "   - When the template renames or restructures a directory, the agent must migrate rather than maintain a divergent path.")
	fmt.Fprintln(&b, "")
	fmt.Fprintln(&b, "5. **Manifest pass — confirm baseline after adoptions.**")
	fmt.Fprintln(&b, "   - Sync has already written the updated manifest and TEMPLATE_VERSION. After applying adoptions, the agent must confirm these baseline artifacts remain correct so the next sync diffs against the right starting point.")
	fmt.Fprintln(&b, "")
	fmt.Fprintln(&b, "6. **Report — explain each decision to the director.**")
	fmt.Fprintln(&b, "   - For each `adopt` item, the agent must state one of: **adopted** (with summary of changes), **kept** (with documented repo-specific reason), or **needs director judgment** (with explanation).")
	fmt.Fprintln(&b, "   - The agent must not silently skip any `adopt` item. Every item must have a stated disposition.")
	fmt.Fprintln(&b, "")
	fmt.Fprintln(&b, "7. **Feedback — surface improvements for the governance template.**")
	fmt.Fprintln(&b, "   - The agent must note any recommendations that were confusing, lacked sufficient context to evaluate, or didn't account for a common repo pattern.")
	fmt.Fprintln(&b, "   - The director routes this feedback to governa DEV and QA to improve future sync output and methodology.")
	fmt.Fprintln(&b, "")

	if len(conflicts) > 0 {
		fmt.Fprintln(&b, "## Conflicts")
		fmt.Fprintln(&b, "")
		fmt.Fprintln(&b, "**These conflicts block sync from completing correctly. Resolve them before acting on the recommendations below.**")
		fmt.Fprintln(&b, "")
		for _, c := range conflicts {
			// Descriptions are multi-line blocks starting with a ### heading;
			// render as-is so the heading and numbered steps land cleanly under
			// ## Conflicts rather than being wrapped in a single bullet.
			fmt.Fprintln(&b, c.description)
		}
	}

	fmt.Fprintln(&b, "## Recommendations")
	fmt.Fprintln(&b, "")
	fmt.Fprintln(&b, "| File | Recommendation | Reason | Existing Lines | Proposed Lines |")
	fmt.Fprintln(&b, "|------|----------------|--------|---------------|----------------|")
	for _, s := range scores {
		fmt.Fprintf(&b, "| `%s` | %s | %s | %d | %d |\n", relPath(s.path), s.recommendation, s.reason, s.existingLines, s.proposedLines)
	}

	// Action summary
	keeps, adopts := 0, 0
	for _, s := range scores {
		switch s.recommendation {
		case "keep":
			keeps++
		case "adopt":
			adopts++
		}
	}
	fmt.Fprintf(&b, "\n## Summary\n\n")
	fmt.Fprintf(&b, "- **keep**: %d files (no adoption work needed)\n", keeps)
	fmt.Fprintf(&b, "- **adopt**: %d files (must compare `.governa-proposed/<file>` and adopt unless repo-specific)\n", adopts)

	// Adoption Items — single detail section for all adopt files
	if adopts > 0 {
		fmt.Fprintf(&b, "\n## Adoption Items\n\n")
		fmt.Fprintln(&b, "For each file below, read `.governa-proposed/<file>` and adopt the template content. Keep only content that is definitively repo-specific with a documented reason.")
		fmt.Fprintln(&b, "")
		for _, s := range scores {
			if s.recommendation != "adopt" {
				continue
			}
			rel := relPath(s.path)
			fmt.Fprintf(&b, "- `%s`", rel)
			// Show relevant detail based on what triggered the adopt
			if len(s.missingSections) > 0 && !s.contentChanged && !s.standingDrift {
				fmt.Fprintf(&b, " — missing sections: %s", strings.Join(s.missingSections, ", "))
			} else if len(s.changedSections) > 0 {
				fmt.Fprintf(&b, " — changed: %s", taggedSectionList(s.changedSections, s.changedClassifications))
			} else if len(s.driftSections) > 0 {
				fmt.Fprintf(&b, " — drifting sections: %s", strings.Join(s.driftSections, ", "))
			}
			// Structural observation text (no inline code blocks)
			for _, note := range s.structuralNotes {
				fmt.Fprintf(&b, " — %s: %s", note.section, note.observation)
			}
			// Merge hint for known merge-target files (e.g., .gitignore)
			// where wholesale replacement would lose repo-specific patterns.
			if knownMergeTargets[filepath.Base(rel)] {
				fmt.Fprintf(&b, " — merge template patterns into your existing file (don't replace wholesale)")
			}
			fmt.Fprintf(&b, " → `diff %s .governa-proposed/%s`\n", rel, rel)
		}
		fmt.Fprintln(&b, "")
	}

	// Advisory notes: keep files with missing sections, section renames
	hasAdvisory := false
	for _, s := range scores {
		if s.recommendation == "keep" && len(s.missingSections) > 0 {
			hasAdvisory = true
			break
		}
		if len(s.sectionRenames) > 0 {
			hasAdvisory = true
			break
		}
	}
	if hasAdvisory {
		fmt.Fprintf(&b, "\n## Advisory Notes\n\n")
		fmt.Fprintln(&b, "These notes are informational — they do not change the recommendation for any file.")
		fmt.Fprintln(&b, "")
		for _, s := range scores {
			rel := relPath(s.path)
			// Append a diff command suffix only when the proposed counterpart
			// was actually materialized by writeProposedFiles. Both surfaces
			// delegate to shouldMaterializeProposal so the review doc can
			// never point at a missing file.
			diffSuffix := ""
			if shouldMaterializeProposal(s) {
				diffSuffix = fmt.Sprintf(" — `diff %s .governa-proposed/%s`", rel, rel)
			}
			if s.recommendation == "keep" && len(s.missingSections) > 0 {
				fmt.Fprintf(&b, "- `%s`: template also has sections not in this file: %s — review if relevant to this repo%s\n", rel, strings.Join(s.missingSections, ", "), diffSuffix)
			}
			if len(s.sectionRenames) > 0 {
				renameKeys := make([]string, 0, len(s.sectionRenames))
				for oldName := range s.sectionRenames {
					renameKeys = append(renameKeys, oldName)
				}
				slices.Sort(renameKeys)
				for _, oldName := range renameKeys {
					fmt.Fprintf(&b, "- `%s`: Section renamed: %s → %s%s\n", rel, oldName, s.sectionRenames[oldName], diffSuffix)
				}
			}
		}
		fmt.Fprintln(&b, "")
	}

	// Next Steps — closing action block. Content adapts to the sync outcome.
	fmt.Fprintln(&b, "")
	fmt.Fprintln(&b, "## Next Steps")
	fmt.Fprintln(&b, "")
	switch {
	case len(conflicts) > 0:
		fmt.Fprintln(&b, "1. Resolve the conflicts above (each entry under `## Conflicts` has numbered steps).")
		fmt.Fprintln(&b, "2. Re-run `governa sync` to complete the sync. Adoption items below stay until conflicts are cleared.")
	case adopts > 0:
		fmt.Fprintln(&b, "1. Work through `## Adoption Items` following the Evaluation Methodology above.")
		fmt.Fprintln(&b, "2. After adoption decisions are made, commit the bookkeeping files (`TEMPLATE_VERSION`, `.governa-manifest`) to record the new baseline.")
		fmt.Fprintln(&b, "3. The review artifact (`governa-sync-review.md`) and `.governa-proposed/` are working artifacts — not intended to be committed.")
	default:
		fmt.Fprintln(&b, "No adoption work needed.")
		fmt.Fprintln(&b, "")
		fmt.Fprintln(&b, "1. Commit the bookkeeping files (`TEMPLATE_VERSION`, `.governa-manifest`) to record the new baseline.")
		fmt.Fprintln(&b, "2. The review artifact (`governa-sync-review.md`) is not intended to be committed.")
	}
	fmt.Fprintln(&b, "")

	// Status reflects whether any blocking/mandatory operator action is
	// required. `keep`-with-advisory items do not block — they are
	// reviewable but not required — so `CLEAN` does not imply "nothing to
	// look at," only "no required adoption/conflict action."
	if adopts > 0 || len(conflicts) > 0 {
		fmt.Fprintf(&b, "## Status\n\n`PENDING` — operator review required\n")
	} else {
		fmt.Fprintf(&b, "## Status\n\n`CLEAN` — no required adoption/conflict action\n")
	}
	return b.String()
}

// compareStructure checks matching ## sections for structural differences.
// Returns notes where the template uses a simpler structure than the existing.
func compareStructure(existingContent, proposedContent string) []structuralNote {
	existingSections := parseLevel2Sections(existingContent)
	proposedSections := parseLevel2Sections(proposedContent)
	proposedMap := sectionMap(proposedSections)

	var notes []structuralNote
	for _, es := range existingSections {
		proposedBody, exists := proposedMap[es.Name]
		if !exists {
			continue
		}
		existingHasSubsections := strings.Contains(es.Body, "\n### ") || strings.HasPrefix(es.Body, "### ")
		proposedHasSubsections := strings.Contains(proposedBody, "\n### ") || strings.HasPrefix(proposedBody, "### ")
		if existingHasSubsections && !proposedHasSubsections {
			notes = append(notes, structuralNote{
				section:      es.Name,
				existingBody: es.Body,
				templateBody: proposedBody,
				observation:  "template uses simpler structure (flat bullets) — consider adopting the format while preserving project-specific rules",
			})
		}
	}
	return notes
}

// detectChangedSections compares shared ## sections between existing and
// proposed content and returns section names where the body differs.
func detectChangedSections(existingContent, proposedContent string) []string {
	existingSections := parseLevel2Sections(existingContent)
	proposedMap := sectionMap(parseLevel2Sections(proposedContent))

	var changed []string
	for _, es := range existingSections {
		proposedBody, exists := proposedMap[es.Name]
		if !exists {
			continue
		}
		if strings.TrimSpace(es.Body) != strings.TrimSpace(proposedBody) {
			changed = append(changed, es.Name)
		}
	}
	return changed
}

// classifyChange determines whether the difference between two section bodies
// is "structural" (layout/shape changed) or "cosmetic" (wording-only).
//
// Structural signals: heading count delta, numbered-list item count delta,
// numbered-list reorder, bullet count delta >1, paragraph count delta.
// If none fire, the change is cosmetic.
func classifyChange(existingBody, proposedBody string) string {
	existing := strings.TrimSpace(existingBody)
	proposed := strings.TrimSpace(proposedBody)
	if existing == proposed {
		return "cosmetic" // identical — shouldn't happen, but safe default
	}

	eLines := strings.Split(existing, "\n")
	pLines := strings.Split(proposed, "\n")

	// Heading count (### or deeper).
	eHeadings := countPrefix(eLines, "### ")
	pHeadings := countPrefix(pLines, "### ")
	if eHeadings != pHeadings {
		return "structural"
	}

	// Numbered list items (lines starting with digit+period).
	eNumbered := countNumbered(eLines)
	pNumbered := countNumbered(pLines)
	if eNumbered != pNumbered {
		return "structural"
	}

	// Numbered list reorder: same count but items moved to different positions.
	if eNumbered > 0 && numberedItemsReordered(eLines, pLines) {
		return "structural"
	}

	// Bullet list items (lines starting with "- ").
	eBullets := countPrefix(eLines, "- ")
	pBullets := countPrefix(pLines, "- ")
	if abs(eBullets-pBullets) > 1 {
		return "structural"
	}

	// Paragraph count (non-empty blocks separated by blank lines).
	eParas := countParagraphs(eLines)
	pParas := countParagraphs(pLines)
	if eParas != pParas {
		return "structural"
	}

	return "cosmetic"
}

func countPrefix(lines []string, prefix string) int {
	n := 0
	for _, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), prefix) {
			n++
		}
	}
	return n
}

var numberedLineRe = regexp.MustCompile(`^\s*\d+\.\s`)

func countNumbered(lines []string) int {
	n := 0
	for _, l := range lines {
		if numberedLineRe.MatchString(l) {
			n++
		}
	}
	return n
}

// numberedItemsReordered returns true if the numbered list items appear in a
// different order. It compares the text after stripping the number prefix.
func numberedItemsReordered(eLines, pLines []string) bool {
	eItems := numberedItemTexts(eLines)
	pItems := numberedItemTexts(pLines)
	if len(eItems) != len(pItems) {
		return true
	}
	for i := range eItems {
		if eItems[i] != pItems[i] {
			return true
		}
	}
	return false
}

var numberedItemRe = regexp.MustCompile(`^\s*\d+\.\s+(.*)`)

func numberedItemTexts(lines []string) []string {
	var items []string
	for _, l := range lines {
		if m := numberedItemRe.FindStringSubmatch(l); m != nil {
			items = append(items, strings.TrimSpace(m[1]))
		}
	}
	return items
}

func countParagraphs(lines []string) int {
	if len(lines) == 0 {
		return 0
	}
	paras := 0
	inPara := false
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			inPara = false
		} else if !inPara {
			paras++
			inPara = true
		}
	}
	return paras
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// taggedSectionList formats changed section names with their classification,
// e.g. "Pre-Release Checklist (structural), Build (cosmetic)".
func taggedSectionList(sections []string, classifications map[string]string) string {
	parts := make([]string, len(sections))
	for i, name := range sections {
		if cls, ok := classifications[name]; ok {
			parts[i] = fmt.Sprintf("%s (%s)", name, cls)
		} else {
			parts[i] = name
		}
	}
	return strings.Join(parts, ", ")
}

// detectSectionRenames finds one-to-one best-match renames between sections
// that exist in one version but not the other. Returns old→new name map.
// Uses line overlap (shared lines / max lines) with a 50% threshold.
// Document order breaks ties; consumed pairs can't match again.
func detectSectionRenames(existingNames, proposedNames []string, existingMap, proposedMap map[string]string) map[string]string {
	// Find unmatched sections on each side.
	proposedSet := make(map[string]bool, len(proposedNames))
	for _, n := range proposedNames {
		proposedSet[n] = true
	}
	existingSet := make(map[string]bool, len(existingNames))
	for _, n := range existingNames {
		existingSet[n] = true
	}
	var unmatchedExisting []string
	for _, n := range existingNames {
		if !proposedSet[n] {
			unmatchedExisting = append(unmatchedExisting, n)
		}
	}
	var unmatchedProposed []string
	for _, n := range proposedNames {
		if !existingSet[n] {
			unmatchedProposed = append(unmatchedProposed, n)
		}
	}

	if len(unmatchedExisting) == 0 || len(unmatchedProposed) == 0 {
		return nil
	}

	consumed := make(map[string]bool)
	renames := make(map[string]string)

	for _, pName := range unmatchedProposed {
		bestOld := ""
		bestOverlap := 0.0
		for _, eName := range unmatchedExisting {
			if consumed[eName] {
				continue
			}
			overlap := lineOverlap(existingMap[eName], proposedMap[pName])
			if overlap >= 0.5 && overlap > bestOverlap {
				bestOverlap = overlap
				bestOld = eName
			}
		}
		if bestOld != "" {
			renames[bestOld] = pName
			consumed[bestOld] = true
		}
	}
	if len(renames) == 0 {
		return nil
	}
	return renames
}

// lineOverlap computes the fraction of shared lines between two bodies.
// Returns shared / max(len(a), len(b)).
func lineOverlap(bodyA, bodyB string) float64 {
	aLines := strings.Split(strings.TrimSpace(bodyA), "\n")
	bLines := strings.Split(strings.TrimSpace(bodyB), "\n")
	if len(aLines) == 0 && len(bLines) == 0 {
		return 1.0
	}
	bSet := make(map[string]bool, len(bLines))
	for _, l := range bLines {
		bSet[strings.TrimSpace(l)] = true
	}
	shared := 0
	for _, l := range aLines {
		if bSet[strings.TrimSpace(l)] {
			shared++
		}
	}
	maxLen := max(len(bLines), len(aLines))
	return float64(shared) / float64(maxLen)
}

// classifySections builds a section-name → "structural"/"cosmetic" map
// for each changed section by comparing the section bodies.
func classifySections(existingContent, proposedContent string, changedSections []string) map[string]string {
	existingMap := sectionMap(parseLevel2Sections(existingContent))
	proposedMap := sectionMap(parseLevel2Sections(proposedContent))
	result := make(map[string]string, len(changedSections))
	for _, name := range changedSections {
		result[name] = classifyChange(existingMap[name], proposedMap[name])
	}
	return result
}

// parseLevel3Sections parses ### subsections within a ## section body.
// Returns a slice of section structs with Name and Body.
func parseLevel3Sections(body string) []markdownSection {
	var sections []markdownSection
	lines := strings.Split(body, "\n")
	var current *markdownSection
	for _, line := range lines {
		if after, ok := strings.CutPrefix(line, "### "); ok {
			if current != nil {
				sections = append(sections, *current)
			}
			current = &markdownSection{Name: strings.TrimSpace(after)}
			continue
		}
		if current != nil {
			current.Body += line + "\n"
		}
	}
	if current != nil {
		sections = append(sections, *current)
	}
	return sections
}

// readmeMissingWhySection returns true if the target directory contains a
// README.md that does not have a ## Why section. Returns false if README.md
// is absent (template will generate one with the section).
// shouldSkipKnowledgeDir returns true if the target repo does not use
// docs/knowledge/ or has only README.md there with no sibling files.
func shouldSkipKnowledgeDir(targetDir string) bool {
	knowledgeDir := filepath.Join(targetDir, "docs", "knowledge")
	entries, err := os.ReadDir(knowledgeDir)
	if err != nil {
		// Directory doesn't exist — skip
		return true
	}
	for _, entry := range entries {
		if entry.Name() != "README.md" {
			// Has real content beyond README.md (file or subdirectory) — keep
			return false
		}
	}
	// Only README.md or empty — skip
	return true
}

func readmeMissingWhySection(targetDir string) bool {
	content, err := os.ReadFile(filepath.Join(targetDir, "README.md"))
	if err != nil {
		return false
	}
	return !strings.Contains(string(content), "## Why")
}

func writeSyncReview(targetDir string, scores []collisionScore, conflicts []conflict, oldVersion, newVersion string, dryRun bool) error {
	reviewPath := filepath.Join(targetDir, "governa-sync-review.md")
	content := renderSyncReview(targetDir, scores, conflicts, oldVersion, newVersion)
	fmt.Printf("%s %s (sync review document)\n", formatAction(dryRun, "write"), reviewPath)
	if dryRun {
		return nil
	}
	return os.WriteFile(reviewPath, []byte(content), 0o644)
}

// printConflictsSummary emits the final sync disposition line when conflicts
// are detected. The pre-sync assessment is a separate, distinctly-labeled
// operator surface (printed earlier) that describes what sync detected before
// writing. The disposition is the final state after transforms and conflict
// detection. Paths are rendered repo-relative to match the review doc.
func printConflictsSummary(targetDir string, conflicts []conflict) {
	plural := "conflict"
	if len(conflicts) != 1 {
		plural = "conflicts"
	}
	fmt.Fprintf(os.Stderr, "%s needs manual resolution — %d %s detected — see governa-sync-review.md\n",
		color.Yel("disposition:"), len(conflicts), plural)
	for _, c := range conflicts {
		rel := c.path
		if targetDir != "" {
			if r, err := filepath.Rel(targetDir, c.path); err == nil {
				rel = r
			} else {
				rel = filepath.Base(c.path)
			}
		}
		fmt.Fprintf(os.Stderr, "  %s %s\n", color.Yel("conflict:"), rel)
	}
}

// writeProposedFiles writes the template version of each reviewable file
// to .governa-proposed/<path> so agents can diff directly against their files.
// shouldMaterializeProposal reports whether a score warrants writing its
// template counterpart into .governa-proposed/. Used by writeProposedFiles
// (the producer) and by renderSyncReview's Advisory Notes section (the
// consumer that points operators at the diff command). Having a single
// predicate guarantees the two surfaces stay aligned — the review doc
// never points to a file that wasn't materialized.
func shouldMaterializeProposal(s collisionScore) bool {
	if s.recommendation == "adopt" {
		return true
	}
	if s.recommendation == "keep" && (len(s.missingSections) > 0 || len(s.sectionRenames) > 0) {
		return true
	}
	return false
}

func writeProposedFiles(targetDir string, scores []collisionScore, dryRun bool) error {
	proposedDir := filepath.Join(targetDir, ".governa-proposed")
	wrote := false
	for _, s := range scores {
		if s.proposedContent == "" {
			continue
		}
		if !shouldMaterializeProposal(s) {
			continue
		}
		rel, err := filepath.Rel(targetDir, s.path)
		if err != nil {
			rel = filepath.Base(s.path)
		}
		outPath := filepath.Join(proposedDir, rel)
		fmt.Printf("%s %s (proposed template)\n", formatAction(dryRun, "write"), outPath)
		if dryRun {
			wrote = true
			continue
		}
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return fmt.Errorf("create proposed dir for %s: %w", rel, err)
		}
		mode := os.FileMode(0o644)
		if strings.HasSuffix(rel, ".sh") {
			mode = 0o755
		}
		if err := os.WriteFile(outPath, []byte(s.proposedContent), mode); err != nil {
			return fmt.Errorf("write proposed %s: %w", rel, err)
		}
		wrote = true
	}
	if !wrote {
		return nil
	}
	// Write an explanation file for the directory (ABOUT.md, not README.md,
	// to avoid colliding with a proposed repo README.md).
	aboutPath := filepath.Join(proposedDir, "ABOUT.md")
	if !dryRun {
		about := "# Proposed Template Files\n\nEach file here is the template version of a file flagged in `governa-sync-review.md`:\n\n- files marked `adopt` — compare and adopt the template content\n- files marked `keep` with advisory notes (missing sections or section renames) — compare to see what the advisory references\n\nUse them for direct comparison:\n\n    diff <your-file> .governa-proposed/<file>\n\nFiles marked `keep` with no advisory notes are not materialized here — there is no review work expected for them.\n\nThis directory is not intended to be committed. Repo governance decides cleanup.\n"
		os.WriteFile(aboutPath, []byte(about), 0o644)
	}
	return nil
}

func printAdoptDriftFromScores(scores []collisionScore) {
	if len(scores) == 0 {
		fmt.Printf("%s none detected\n", color.Yel("drift:"))
		return
	}
	keeps, adopts := 0, 0
	for _, s := range scores {
		switch s.recommendation {
		case "keep":
			keeps++
		case "adopt":
			adopts++
		}
	}
	parts := []string{}
	if keeps > 0 {
		parts = append(parts, fmt.Sprintf("%d files unchanged", keeps))
	}
	if adopts > 0 {
		parts = append(parts, fmt.Sprintf("%d files to adopt", adopts))
	}
	suffix := ""
	if adopts > 0 {
		suffix = " — see governa-sync-review.md"
	}
	fmt.Printf("%s %s%s\n", color.Yel("drift:"), strings.Join(parts, ", "), suffix)
}

func emitAdoptAdvisories(targetDir string) {
	if readmeMissingWhySection(targetDir) {
		fmt.Printf("  %s existing README.md has no ## Why section (see template for guidance)\n", color.Yel("advisory:"))
	}
}

func skipIfExists(op operation) operation {
	if _, err := os.Stat(op.path); err == nil {
		return operation{kind: "skip"}
	}
	return op
}

// readTemplateOrRoot reads a file from the template FS first; if not found,
// falls back to the repo root. This handles files like TEMPLATE_VERSION that
// live at the repo root rather than inside internal/templates/.
// readTemplateVersion returns the template version string. When repoRoot is
// set (enhance mode, dev), it reads from the TEMPLATE_VERSION file on disk.
// When repoRoot is empty (installed binary, consumer modes), it falls back
// to the compiled-in templates.TemplateVersion constant.
func readTemplateVersion(repoRoot string) string {
	if repoRoot != "" {
		content, err := os.ReadFile(filepath.Join(repoRoot, "TEMPLATE_VERSION"))
		if err == nil {
			return strings.TrimSpace(string(content))
		}
	}
	return templates.TemplateVersion
}

func readTemplateOrRoot(tfs fs.FS, repoRoot, path string) ([]byte, error) {
	content, err := fs.ReadFile(tfs, path)
	if err == nil {
		return content, nil
	}
	return os.ReadFile(filepath.Join(repoRoot, path))
}

func readAndRender(tfs fs.FS, path string, placeholders map[string]string) (string, error) {
	content, err := fs.ReadFile(tfs, path)
	if err != nil {
		return "", fmt.Errorf("read template file %s: %w", path, err)
	}
	out := string(content)
	keys := make([]string, 0, len(placeholders))
	for k := range placeholders {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, key := range keys {
		out = strings.ReplaceAll(out, key, placeholders[key])
	}
	return out, nil
}

func valueOrDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func joinOrNone(items []string) string {
	if len(items) == 0 {
		return "none"
	}
	return strings.Join(items, ", ")
}
