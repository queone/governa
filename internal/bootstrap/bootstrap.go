package bootstrap

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

	"github.com/kquo/governa/internal/color"
	"github.com/kquo/governa/internal/templates"
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
	ExistingArtifacts  []string
	CollisionRisk      string
	Recommendation     string
	CodeSignals        int
	DocSignals         int
	CollidingArtifacts []string
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
	if _, err := os.Stat(filepath.Join(targetDir, "docs", "agent-roles")); err == nil {
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
	assessment, err := AssessTarget(targetAbs, cfg.Type)
	if err != nil {
		return err
	}
	if cfg.Type == "" {
		switch assessment.RepoShape {
		case "likely CODE":
			cfg.Type = RepoTypeCode
		case "likely DOC":
			cfg.Type = RepoTypeDoc
		}
	}

	// Prompt for any still-missing parameters.
	promptMissing(&cfg, targetAbs)

	// Validate after prompts have filled gaps.
	if err := validateConfig(cfg); err != nil {
		return err
	}

	printAssessment(cfg.Mode, targetAbs, assessment)

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
	if adopt {
		oldManifest, _, _ := readManifest(targetAbs)
		oldEntryMap := manifestEntryMap(oldManifest)
		newEntryMap := manifestEntryMap(manifest)
		transformed, scores := applyAdoptTransforms(canonical, oldEntryMap, newEntryMap, targetAbs)
		ops = compactOperations(transformed)
		emitAdoptAdvisories(targetAbs)
		ops = append(ops, manifestOp)
		if err := applyOperations(ops, cfg.DryRun); err != nil {
			return err
		}
		// Filter to only collision scores (keep/review) for the review doc
		var collisions []collisionScore
		for _, s := range scores {
			if s.recommendation != "accept" {
				collisions = append(collisions, s)
			}
		}
		if len(collisions) > 0 {
			if err := writeAdoptReview(targetAbs, collisions, cfg.DryRun); err != nil {
				return err
			}
		}
		printAdoptDriftFromScores(collisions)
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
			CollisionRisk:  "low",
			Recommendation: "safe to apply",
		}, nil
	}
	for _, rel := range files {
		base := filepath.Base(rel)
		ext := strings.ToLower(filepath.Ext(rel))
		topLevel := strings.Split(rel, string(os.PathSeparator))[0]
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

	expected := expectedArtifactPaths(repoType)
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

func expectedArtifactPaths(repoType RepoType) []string {
	base := []string{"AGENTS.md", "CLAUDE.md", "TEMPLATE_VERSION", "README.md"}
	switch repoType {
	case RepoTypeCode:
		return append(
			base,
			"arch.md",
			"plan.md",
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
	transformed, _ := applyAdoptTransforms(canonical, nil, nil, targetRoot)
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

func applyAdoptTransforms(ops []operation, oldManifest map[string]ManifestEntry, newManifest map[string]ManifestEntry, targetDir string) ([]operation, []collisionScore) {
	out := make([]operation, len(ops))
	var scores []collisionScore
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
			out[i] = skipIfExists(op)
		case op.kind == "symlink":
			out[i] = skipIfExists(op)
		case op.kind == "write" && op.note == "overlay file":
			score := scoreOverlayCollision(op.path, op.content, oldEntry.SourceChecksum, newEntry.SourceChecksum)
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
	return out, scores
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
	fmt.Printf("recommendation: %s\n", a.Recommendation)
	if len(a.CollidingArtifacts) > 0 {
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
	current := markdownSection{}
	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			if current.Name != "" {
				current.Body = strings.TrimSpace(current.Body)
				sections = append(sections, current)
			}
			current = markdownSection{Name: strings.TrimSpace(strings.TrimPrefix(line, "## "))}
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
	if current.Name != "" {
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
	path             string // target file path
	recommendation   string // "keep", "review: cherry-pick", "review: content changed", "review: no action likely", "accept"
	reason           string
	existingLines    int
	proposedLines    int
	existingSections int
	proposedSections int
	missingSections  []string         // sections in proposed but not in existing
	changedSections  []string         // shared sections with different content (markdown only)
	contentChanged   bool             // true when template source changed and existing differs from new template
	proposedContent  string           // the template content for the review doc
	governancePatch  string           // non-empty if this is an AGENTS.md patch with missing sections
	structuralNotes  []structuralNote // section-level structural observations
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
			score.recommendation = "review: content changed"
			score.reason = fmt.Sprintf("template changed since last sync (non-markdown, existing %d lines, proposed %d lines)", score.existingLines, score.proposedLines)
			score.contentChanged = true
			return score
		}
		score.recommendation = "review: no action likely"
		score.reason = fmt.Sprintf("non-markdown file (existing %d lines, proposed %d lines)", score.existingLines, score.proposedLines)
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

	// Structural comparison for matching sections
	score.structuralNotes = compareStructure(existingContent, proposedContent)

	// Detect section-level content changes when template source changed.
	if templateChanged {
		score.changedSections = detectChangedSections(existingContent, proposedContent)
		if len(score.changedSections) > 0 {
			score.contentChanged = true
		}
	}

	// Decision rules
	if score.existingLines >= 2*score.proposedLines {
		if score.contentChanged {
			score.recommendation = "review: content changed"
			score.reason = fmt.Sprintf("existing is more developed (%d lines vs %d proposed) but template sections changed: %s", score.existingLines, score.proposedLines, strings.Join(score.changedSections, ", "))
			return score
		}
		score.recommendation = "keep"
		score.reason = fmt.Sprintf("existing is more developed (%d lines vs %d proposed)", score.existingLines, score.proposedLines)
		return score
	}
	if score.existingSections > score.proposedSections {
		if score.contentChanged {
			score.recommendation = "review: content changed"
			score.reason = fmt.Sprintf("existing has richer structure (%d sections vs %d proposed) but template sections changed: %s", score.existingSections, score.proposedSections, strings.Join(score.changedSections, ", "))
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
			score.recommendation = "review: content changed"
			score.reason = fmt.Sprintf("template sections changed: %s", strings.Join(score.changedSections, ", "))
			return score
		}
		score.recommendation = "keep"
		score.reason = fmt.Sprintf("existing covers same content under different headings (%d sections vs %d proposed)", score.existingSections, score.proposedSections)
		return score
	}
	if len(score.missingSections) > 0 {
		score.recommendation = "review: cherry-pick"
		score.reason = fmt.Sprintf("proposed adds sections: %s", strings.Join(score.missingSections, ", "))
		return score
	}

	if score.contentChanged {
		score.recommendation = "review: content changed"
		score.reason = fmt.Sprintf("template sections changed: %s", strings.Join(score.changedSections, ", "))
		return score
	}
	score.recommendation = "review: no action likely"
	score.reason = fmt.Sprintf("similar content (%d lines vs %d proposed, %d sections vs %d)", score.existingLines, score.proposedLines, score.existingSections, score.proposedSections)
	return score
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
				return collisionScore{
					path:            op.path,
					recommendation:  "review: content changed",
					reason:          fmt.Sprintf("governed sections changed: %s", strings.Join(changedGoverned, ", ")),
					existingLines:   countLines(string(existingContent)),
					proposedLines:   countLines(op.content),
					changedSections: changedGoverned,
					contentChanged:  true,
					proposedContent: op.content,
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
		recommendation:  "review: cherry-pick",
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

func renderAdoptReview(scores []collisionScore) string {
	var b strings.Builder
	fmt.Fprintln(&b, "# governa sync review")
	fmt.Fprintln(&b, "")
	fmt.Fprintf(&b, "Generated by `governa sync`. Review each recommendation and take action.\n\n")

	fmt.Fprintln(&b, "## Recommendations")
	fmt.Fprintln(&b, "")
	fmt.Fprintln(&b, "| File | Recommendation | Reason | Existing Lines | Proposed Lines |")
	fmt.Fprintln(&b, "|------|----------------|--------|---------------|----------------|")
	for _, s := range scores {
		fmt.Fprintf(&b, "| `%s` | %s | %s | %d | %d |\n", scoreRelPath(s.path), s.recommendation, s.reason, s.existingLines, s.proposedLines)
	}

	// Action summary
	keeps, cherryPicks, contentChanged, noAction := 0, 0, 0, 0
	for _, s := range scores {
		switch s.recommendation {
		case "keep":
			keeps++
		case "review: cherry-pick":
			cherryPicks++
		case "review: content changed":
			contentChanged++
		case "review: no action likely":
			noAction++
		}
	}
	fmt.Fprintf(&b, "\n## Summary\n\n")
	fmt.Fprintf(&b, "- **keep**: %d files (existing is more developed or identical, no action needed)\n", keeps)
	fmt.Fprintf(&b, "- **review: cherry-pick**: %d files (proposed adds sections worth considering)\n", cherryPicks)
	fmt.Fprintf(&b, "- **review: content changed**: %d files (template sections changed since last sync)\n", contentChanged)
	fmt.Fprintf(&b, "- **review: no action likely**: %d files (structurally different but not clearly better)\n", noAction)

	if cherryPicks > 0 {
		fmt.Fprintf(&b, "\n## Cherry-Pick Candidates\n\n")
		fmt.Fprintln(&b, "These files have sections or content worth considering. Compare against your existing file and cherry-pick useful additions.")
		fmt.Fprintln(&b, "")
		for _, s := range scores {
			if s.recommendation != "review: cherry-pick" {
				continue
			}
			rel := scoreRelPath(s.path)
			if s.governancePatch != "" {
				fmt.Fprintf(&b, "### `%s` (governance patch)\n\n", rel)
				fmt.Fprintf(&b, "Missing governed sections: %s\n\n", strings.Join(s.missingSections, ", "))
				fmt.Fprintf(&b, "Patched content that includes the missing sections:\n\n")
				fmt.Fprintf(&b, "```markdown\n%s\n```\n\n", s.governancePatch)
			} else {
				fmt.Fprintf(&b, "### `%s`\n\n", rel)
				if len(s.missingSections) > 0 {
					fmt.Fprintf(&b, "Proposed adds sections: %s\n\n", strings.Join(s.missingSections, ", "))
				}
				fmt.Fprintf(&b, "Proposed content:\n\n")
				fmt.Fprintf(&b, "```\n%s\n```\n\n", s.proposedContent)
			}
		}
	}

	// Content changes
	if contentChanged > 0 {
		fmt.Fprintf(&b, "\n## Content Changes\n\n")
		fmt.Fprintln(&b, "The template content for these files changed since the last sync. Review the changed sections and incorporate relevant updates.")
		fmt.Fprintln(&b, "")
		for _, s := range scores {
			if s.recommendation != "review: content changed" {
				continue
			}
			rel := scoreRelPath(s.path)
			if len(s.changedSections) > 0 {
				fmt.Fprintf(&b, "### `%s`\n\n", rel)
				fmt.Fprintf(&b, "Changed sections: %s\n\n", strings.Join(s.changedSections, ", "))
				existingBytes, _ := os.ReadFile(s.path)
				existingMap := sectionMap(parseLevel2Sections(string(existingBytes)))
				proposedMap := sectionMap(parseLevel2Sections(s.proposedContent))
				for _, sec := range s.changedSections {
					fmt.Fprintf(&b, "#### %s\n\n", sec)
					fmt.Fprintln(&b, "**Your version:**")
					fmt.Fprintln(&b, "")
					fmt.Fprintf(&b, "```markdown\n%s\n```\n\n", strings.TrimSpace(existingMap[sec]))
					fmt.Fprintln(&b, "**Template version:**")
					fmt.Fprintln(&b, "")
					fmt.Fprintf(&b, "```markdown\n%s\n```\n\n", strings.TrimSpace(proposedMap[sec]))
				}
			} else {
				// Non-markdown file — no section detail available
				fmt.Fprintf(&b, "### `%s`\n\n", rel)
				fmt.Fprintln(&b, "Template content changed since last sync. Compare your version against the template and incorporate relevant updates.")
				fmt.Fprintln(&b, "")
			}
		}
	}

	// Structural notes
	hasStructuralNotes := false
	for _, s := range scores {
		if len(s.structuralNotes) > 0 {
			hasStructuralNotes = true
			break
		}
	}
	if hasStructuralNotes {
		fmt.Fprintf(&b, "\n## Structural Observations\n\n")
		fmt.Fprintln(&b, "The following sections use a more complex structure than the template version. Consider adopting the simpler format while preserving project-specific rules.")
		fmt.Fprintln(&b, "")
		for _, s := range scores {
			for _, note := range s.structuralNotes {
				fmt.Fprintf(&b, "### %s in `%s`\n\n", note.section, scoreRelPath(s.path))
				fmt.Fprintf(&b, "%s\n\n", note.observation)
				fmt.Fprintln(&b, "**Your version:**")
				fmt.Fprintln(&b, "")
				fmt.Fprintf(&b, "```markdown\n## %s\n\n%s\n```\n\n", note.section, note.existingBody)
				fmt.Fprintln(&b, "**Template version:**")
				fmt.Fprintln(&b, "")
				fmt.Fprintf(&b, "```markdown\n## %s\n\n%s\n```\n\n", note.section, note.templateBody)
			}
		}
	}

	fmt.Fprintf(&b, "\n## Status\n\n`PENDING`\n")
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

func scoreRelPath(path string) string {
	rel := filepath.Base(path)
	if idx := strings.Index(path, "/docs/"); idx >= 0 {
		rel = path[idx+1:]
	} else if idx := strings.Index(path, "/cmd/"); idx >= 0 {
		rel = path[idx+1:]
	}
	return rel
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

func writeAdoptReview(targetDir string, scores []collisionScore, dryRun bool) error {
	reviewPath := filepath.Join(targetDir, "governa-adopt-review.md")
	content := renderAdoptReview(scores)
	fmt.Printf("%s %s (adopt review document)\n", formatAction(dryRun, "write"), reviewPath)
	if dryRun {
		return nil
	}
	return os.WriteFile(reviewPath, []byte(content), 0o644)
}

func printAdoptDriftFromScores(scores []collisionScore) {
	if len(scores) == 0 {
		fmt.Printf("%s none detected\n", color.Yel("drift:"))
		return
	}
	keeps, reviews := 0, 0
	for _, s := range scores {
		if s.recommendation == "keep" {
			keeps++
		} else if strings.HasPrefix(s.recommendation, "review") {
			reviews++
		}
	}
	parts := []string{}
	if keeps > 0 {
		parts = append(parts, fmt.Sprintf("%d files unchanged (existing more developed)", keeps))
	}
	if reviews > 0 {
		parts = append(parts, fmt.Sprintf("%d files to review", reviews))
	}
	fmt.Printf("%s %s\n", color.Yel("drift:"), strings.Join(parts, ", "))
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
