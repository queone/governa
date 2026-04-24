package governance

import (
	"bufio"
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
	"strings"

	"github.com/queone/governa/internal/color"
	"github.com/queone/governa/internal/templates"
)

type Mode string

const (
	ModeSync Mode = "sync"
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

type Config struct {
	Mode               Mode
	Target             string
	Type               RepoType
	RepoName           string
	Purpose            string
	Stack              string
	PublishingPlatform string
	Style              string
	InitGit            bool
	DryRun             bool
	AssumeYes          bool      // AC78 Part C: --yes — apply overwrite to every collision.
	AssumeNo           bool      // AC78 Part C: --no  — apply keep to every collision.
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
	repoType           string
	repoName           string
	purpose            string
	stack              string
	publishingPlatform string
	style              string
	initGit            bool
	dryRun             bool
	assumeYes          bool
	assumeNo           bool
}

// RunWithFS dispatches the one supported mode (sync) against the template FS.
func RunWithFS(tfs fs.FS, repoRoot string, cfg Config) error {
	switch cfg.Mode {
	case ModeSync:
		return runSync(tfs, repoRoot, cfg)
	default:
		return fmt.Errorf("unsupported mode %q", cfg.Mode)
	}
}

// ParseModeArgs parses flags for the given mode. Kept for cmd/governa
// compatibility even though only ModeSync is supported today.
func ParseModeArgs(mode Mode, args []string) (Config, bool, error) {
	return parseFlags(mode, args)
}

func parseFlags(mode Mode, args []string) (Config, bool, error) {
	values := flagValues{}
	fset := flag.NewFlagSet("governa", flag.ContinueOnError)
	fset.SetOutput(os.Stderr)
	fset.StringVar(&values.target, "t", "", "target directory")
	fset.StringVar(&values.target, "target", "", "target directory")
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
	fset.StringVar(&values.repoType, "y", "", "repo type: CODE|DOC")
	fset.StringVar(&values.repoType, "type", "", "repo type: CODE|DOC")
	fset.BoolVar(&values.initGit, "g", false, "initialize git if target is not already a repo")
	fset.BoolVar(&values.initGit, "init-git", false, "initialize git if target is not already a repo")
	fset.BoolVar(&values.dryRun, "d", false, "preview changes without writing")
	fset.BoolVar(&values.dryRun, "dry-run", false, "preview changes without writing")
	fset.BoolVar(&values.assumeYes, "yes", false, "overwrite all collisions without prompting")
	fset.BoolVar(&values.assumeNo, "no", false, "keep all collisions (no overwrites) without prompting")
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

	if values.assumeYes && values.assumeNo {
		return Config{}, false, fmt.Errorf("flags --yes and --no are mutually exclusive")
	}

	cfg := Config{
		Mode:               mode,
		Target:             target,
		Type:               RepoType(strings.ToUpper(strings.TrimSpace(values.repoType))),
		RepoName:           strings.TrimSpace(values.repoName),
		Purpose:            strings.TrimSpace(values.purpose),
		Stack:              strings.TrimSpace(values.stack),
		PublishingPlatform: strings.TrimSpace(values.publishingPlatform),
		Style:              strings.TrimSpace(values.style),
		InitGit:            values.initGit,
		DryRun:             values.dryRun,
		AssumeYes:          values.assumeYes,
		AssumeNo:           values.assumeNo,
	}
	// Validation is deferred to runSync (after prompts) for ModeSync.
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
		}, "Detects whether the target is a new or existing repo and prompts for missing parameters. On collisions, prompts interactively with 'k' keep / 'o' overwrite / 's' skip. --yes / --no batch-apply those choices for non-interactive runs.")
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
	// Check for manifest first (authoritative). Current path (`.governa/manifest`)
	// takes priority; fall back to pre-AC55 (`.governa-manifest`) and pre-governa
	// (`.repokit-manifest`) layouts so legacy repos still detect as re-sync before
	// migrateGovernaLegacyPaths runs.
	for _, name := range []string{manifestFileName, legacyPreAC55ManifestFile, legacyManifestFileName} {
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

// runSync is the sync-mode entrypoint. Path constants used here and throughout
// the file come from manifest.go: ".governa/manifest", ".governa/proposed",
// ".governa/sync-review.md", ".governa/feedback" (AC55).
// AC78 Part C: collision resolution verbs recognized by promptCollisionChoice.
type collisionChoice int

const (
	choiceKeep collisionChoice = iota
	choiceOverwrite
	choiceSkip
)

// runSync writes the base template + overlay files to the target directory.
// For every write-op whose target exists and differs from the template, it
// consults cfg (AssumeYes / AssumeNo / DryRun / interactive stdin) to decide
// whether to keep, overwrite, or skip. No sync-review artifact, no proposed/
// sidecar, no manifest checksum tracking.
func runSync(tfs fs.FS, repoRoot string, cfg Config) error {
	targetAbs, err := filepath.Abs(cfg.Target)
	if err != nil {
		return fmt.Errorf("resolve target path: %w", err)
	}
	if err := os.MkdirAll(targetAbs, 0o755); err != nil && !cfg.DryRun {
		return fmt.Errorf("create target directory: %w", err)
	}

	if !cfg.DryRun {
		if err := migrateGovernaLegacyPaths(targetAbs); err != nil {
			return fmt.Errorf("migrate legacy governa paths: %w", err)
		}
	}

	syncMode := detectSyncMode(targetAbs)
	adopt := syncMode != "new"

	if adopt {
		resolved, sources := resolveAdoptParams(cfg, targetAbs)
		cfg = resolved
		printParamSources(sources)
	}

	assessment, err := AssessTarget(targetAbs, cfg.Type)
	if err != nil {
		return err
	}
	typeInferred := cfg.Type == "" && assessment.ResolvedType != ""
	if typeInferred {
		cfg.Type = assessment.ResolvedType
	}

	promptMissing(&cfg, targetAbs)

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
	params := ManifestParams{
		RepoName:           cfg.RepoName,
		Purpose:            cfg.Purpose,
		Type:               string(cfg.Type),
		Stack:              cfg.Stack,
		PublishingPlatform: cfg.PublishingPlatform,
		Style:              cfg.Style,
	}
	manifest := buildManifest(templateVersion, params)
	manifestOp := operation{
		kind:    "write",
		path:    filepath.Join(targetAbs, manifestFileName),
		content: formatManifest(manifest),
		note:    "bootstrap manifest",
	}

	// Resolve collisions for every write op that targets an existing file.
	// symlink ops fall through to applyOperations (symlinkConflict handles
	// the regular-file-blocking-symlink case there).
	resolved := make([]operation, 0, len(canonical))
	var syncConflicts []conflict
	for _, op := range canonical {
		if op.kind == "write" {
			existing, differs, exists, readErr := readExistingForCollision(op.path, op.content)
			_ = existing
			if readErr != nil {
				return fmt.Errorf("read %s: %w", op.path, readErr)
			}
			if exists && differs {
				choice, err := resolveCollision(op.path, targetAbs, cfg)
				if err != nil {
					return err
				}
				switch choice {
				case choiceKeep, choiceSkip:
					resolved = append(resolved, operation{kind: "skip"})
					continue
				case choiceOverwrite:
					// fall through to append the write op
				}
			}
		}
		if op.kind == "symlink" {
			// Preserve the pre-AC78 symlink-vs-regular-file conflict detection:
			// if an existing regular file blocks the symlink, emit a conflict
			// and skip the op so applyOperations doesn't surface a lower-level
			// error.
			info, err := os.Lstat(op.path)
			if err == nil && info.Mode()&os.ModeSymlink == 0 {
				rel, _ := filepath.Rel(targetAbs, op.path)
				syncConflicts = append(syncConflicts, symlinkConflict(op, rel))
				resolved = append(resolved, operation{kind: "skip"})
				continue
			}
			resolved = append(resolved, skipIfExists(op))
			continue
		}
		resolved = append(resolved, op)
	}

	ops := compactOperations(resolved)
	ops = append(ops, manifestOp)
	if err := applyOperations(ops, cfg.DryRun); err != nil {
		return err
	}

	if cfg.InitGit {
		if err := maybeInitGit(targetAbs, cfg.DryRun); err != nil {
			return err
		}
	}

	if len(syncConflicts) > 0 {
		for _, c := range syncConflicts {
			fmt.Fprintln(os.Stderr, c.description)
		}
		return ErrConflictsPresent
	}
	return nil
}

// readExistingForCollision returns the on-disk content, whether it differs
// from the proposed template content, and whether the file exists at all.
func readExistingForCollision(path, proposed string) (existing string, differs bool, exists bool, err error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, false, nil
	}
	if err != nil {
		return "", false, false, err
	}
	s := string(b)
	return s, s != proposed, true, nil
}

// resolveCollision decides keep / overwrite / skip for one collision based on
// cfg (AssumeYes / AssumeNo / DryRun) and interactive stdin. AC78 Part C.
func resolveCollision(absPath, targetAbs string, cfg Config) (collisionChoice, error) {
	rel, err := filepath.Rel(targetAbs, absPath)
	if err != nil {
		rel = absPath
	}
	rel = filepath.ToSlash(rel)

	if cfg.DryRun {
		fmt.Printf("dry-run collision: %s (would prompt — auto-skipping)\n", rel)
		return choiceSkip, nil
	}
	if cfg.AssumeYes {
		fmt.Printf("collision: %s — overwrite (--yes)\n", rel)
		return choiceOverwrite, nil
	}
	if cfg.AssumeNo {
		fmt.Printf("collision: %s — keep (--no)\n", rel)
		return choiceKeep, nil
	}

	// Interactive prompt — require a TTY.
	in := cfg.Input
	if in == nil {
		in = os.Stdin
	}
	if !isTTY(in) {
		return choiceSkip, fmt.Errorf(
			"collision detected for %s but stdin is not a TTY; pass --yes to overwrite all collisions or --no to keep all",
			rel,
		)
	}

	reader := bufio.NewReader(in)
	for {
		fmt.Printf("collision: %s — [k]eep / [o]verwrite / [s]kip: ", rel)
		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) && strings.TrimSpace(line) == "" {
				return choiceSkip, fmt.Errorf("collision prompt closed with no choice for %s", rel)
			}
			return choiceSkip, fmt.Errorf("read collision prompt response: %w", err)
		}
		if choice, ok := parseCollisionReply(line); ok {
			return choice, nil
		}
		fmt.Printf("  expected k/o/s\n")
	}
}

// parseCollisionReply maps a user's interactive reply to a collisionChoice.
// Returns (choice, true) on a recognized verb; (0, false) otherwise (so the
// caller can reprompt). Exposed so tests can exercise every verb without
// needing a TTY fixture.
func parseCollisionReply(line string) (collisionChoice, bool) {
	switch strings.TrimSpace(strings.ToLower(line)) {
	case "k", "keep":
		return choiceKeep, true
	case "o", "overwrite":
		return choiceOverwrite, true
	case "s", "skip":
		return choiceSkip, true
	}
	return 0, false
}

// isTTY reports whether the reader points at a terminal device. We treat any
// non-*os.File reader (test fixtures, pipes) as non-TTY unless the file's
// Stat reports a character-device mode.
func isTTY(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
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
		// .governa/ is governa-managed metadata (manifest, proposed/, sync-review.md,
		// feedback/); not repo content. Skip it entirely so its markdown files don't
		// inflate docSignals on re-sync vs first-sync (utils round-5 finding).
		if rel == governaDir || strings.HasPrefix(rel, governaDir+string(os.PathSeparator)) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		// Legacy pre-AC55 layout (.governa-proposed/). Kept for repos that haven't
		// re-synced since migration lands — migrateGovernaLegacyPaths removes it,
		// but AssessTarget may run from test harnesses that bypass runSync.
		if rel == legacyPreAC55ProposedDir || strings.HasPrefix(rel, legacyPreAC55ProposedDir+string(os.PathSeparator)) {
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

// governaOwnedPaths are exact repo-relative paths that governa writes or
// maintains. Files at these paths must not contribute to codeSignals or
// docSignals — they are bookkeeping/generated content, not repo-authored
// signals. This set is used by isGovernaOwnedPath; its scope is intentionally
// limited to signal counting and does NOT affect expectedArtifactPaths,
// ExistingArtifacts computation, collision scoring, review rendering, or
// .governa/proposed/ materialization.
var governaOwnedPaths = map[string]bool{
	// Bookkeeping
	manifestFileName:            true, // .governa/manifest
	legacyPreAC78SyncReviewFile: true, // pre-AC78 legacy (retired)
	legacyPreAC55ManifestFile:   true, // pre-AC55 legacy
	legacyPreAC55SyncReviewFile: true, // pre-AC55 legacy
	legacyManifestFileName:      true, // pre-governa legacy
	"TEMPLATE_VERSION":          true,

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
//   - alter which files get materialized under .governa/proposed/
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

// stackIgnoreBlock returns the stack-specific .gitignore block to append
// after the cross-language base, or ("", false) when the stack is unknown
// or the block isn't available. Called by planCanonical when rendering
// .gitignore.tmpl. (AC51 Fix 2)
func stackIgnoreBlock(tfs fs.FS, stack string) (string, bool) {
	var file string
	switch {
	case stackSuggestsGo(stack):
		file = "stack-ignores/go.txt"
	default:
		return "", false
	}
	content, err := fs.ReadFile(tfs, file)
	if err != nil {
		return "", false
	}
	return string(content), true
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
				strings.HasPrefix(rel, "internal/color/") ||
				strings.HasPrefix(rel, "internal/buildtool/") ||
				strings.HasPrefix(rel, "internal/reltool/")) {
			return nil
		}
		// Skip Go internal packages when module path is unknown (adopt without go.mod)
		if modulePath == "" &&
			(strings.HasPrefix(rel, "internal/color/") ||
				strings.HasPrefix(rel, "internal/buildtool/") ||
				strings.HasPrefix(rel, "internal/reltool/")) {
			return nil
		}
		// Skip docs/knowledge/README.md if the target doesn't use docs/knowledge/
		if rel == "docs/knowledge/README.md.tmpl" || rel == "docs/knowledge/README.md" {
			if _, err := os.Stat(filepath.Join(targetRoot, "docs", "knowledge")); errors.Is(err, os.ErrNotExist) {
				return nil
			}
		}
		targetRel := strings.TrimSuffix(rel, ".tmpl")
		content, err := readAndRender(tfs, path, placeholders)
		if err != nil {
			return err
		}
		// Stack-aware .gitignore: append stack-specific additions for known
		// stacks so Go repos (and future stacks) get language-appropriate
		// ignore patterns without requiring a Standing Divergence. (AC51 Fix 2)
		if targetRel == ".gitignore" {
			if block, ok := stackIgnoreBlock(tfs, cfg.Stack); ok {
				content = content + "\n" + block
			}
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
