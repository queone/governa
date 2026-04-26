package governance

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
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
	Mode      Mode
	Target    string
	Type      RepoType
	RepoName  string
	Stack     string
	InitGit   bool
	AssumeYes bool // AC79 Part B: --yes — batch-overwrite all colliding files; skip the review-doc workflow.
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
	target    string
	repoType  string
	repoName  string
	stack     string
	initGit   bool
	assumeYes bool
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
	fset.StringVar(&values.stack, "s", "", "stack or platform for CODE repos")
	fset.StringVar(&values.stack, "stack", "", "stack or platform for CODE repos")
	fset.StringVar(&values.repoType, "k", "", "repo type: CODE|DOC")
	fset.StringVar(&values.repoType, "type", "", "repo type: CODE|DOC")
	fset.BoolVar(&values.initGit, "g", false, "initialize git if target is not already a repo")
	fset.BoolVar(&values.initGit, "init-git", false, "initialize git if target is not already a repo")
	fset.BoolVar(&values.assumeYes, "y", false, "batch-overwrite all colliding files; skip the review-doc workflow")
	fset.BoolVar(&values.assumeYes, "yes", false, "batch-overwrite all colliding files; skip the review-doc workflow")
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
		Mode:      mode,
		Target:    target,
		Type:      RepoType(strings.ToUpper(strings.TrimSpace(values.repoType))),
		RepoName:  strings.TrimSpace(values.repoName),
		Stack:     strings.TrimSpace(values.stack),
		InitGit:   values.initGit,
		AssumeYes: values.assumeYes,
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
			{Flag: "-k, --type", Desc: "repo type: CODE or DOC"},
			{Flag: "-s, --stack", Desc: "stack or platform (CODE repos)"},
			{Flag: "-t, --target", Desc: "target directory (default: current dir)"},
			{Flag: "-g, --init-git", Desc: "initialize git if target is not a repo"},
			{Flag: "-y, --yes", Desc: "batch-overwrite all colliding files; skip the review-doc workflow"},
		}, "Detects whether the target is a new or existing repo and prompts for missing parameters. Colliding files (existing content differs from template) are NOT touched — they're recorded in `.governa/sync-review.md` for DEV + QA + Director review. `--yes` batch-overwrites every collision directly (escape hatch, skips the review-doc loop).")
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
		if cfg.Type != RepoTypeCode && cfg.Type != RepoTypeDoc {
			return errors.New("repo type must be CODE or DOC: use -k or --type")
		}
		if cfg.Type == RepoTypeCode && cfg.Stack == "" {
			return errors.New("stack/platform is required for CODE repos: use -s or --stack")
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
	sc := bufio.NewScanner(os.Stdin)

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

	if cfg.Type == RepoTypeCode && cfg.Stack == "" {
		cfg.Stack = promptParam("Stack (Go, Node, Rust, Python, Java): ", "", sc)
	}

}

// collisionRecord captures one file whose existing content differs from the
// rendered template. Populated during runSync and rendered into
// `.governa/sync-review.md` for DEV/Director/QA to review.
type collisionRecord struct {
	path     string // absolute path on disk
	existing string // content currently on disk
	proposed string // content the template would write
}

// runSync writes the base template + overlay files to the target directory.
// Default behavior: new + identical files are written; colliding files are
// NOT touched — they're recorded in `.governa/sync-review.md` for review.
// `--yes` (AssumeYes) is the escape hatch: apply every colliding write
// directly, skip the review-doc workflow. Bookkeeping writes (TEMPLATE_VERSION,
// `.governa/manifest`) always apply regardless of collision state.
func runSync(tfs fs.FS, repoRoot string, cfg Config) error {
	targetAbs, err := filepath.Abs(cfg.Target)
	if err != nil {
		return fmt.Errorf("resolve target path: %w", err)
	}
	if err := os.MkdirAll(targetAbs, 0o755); err != nil {
		return fmt.Errorf("create target directory: %w", err)
	}

	if err := migrateGovernaLegacyPaths(targetAbs); err != nil {
		return fmt.Errorf("migrate legacy governa paths: %w", err)
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

	oldManifest, _, _ := readManifest(targetAbs)
	oldTemplateVersion := oldManifest.TemplateVersion

	canonical, err := planCanonical(tfs, repoRoot, cfg, targetAbs)
	if err != nil {
		return err
	}

	templateVersion := readTemplateVersion(repoRoot)
	params := ManifestParams{
		RepoName: cfg.RepoName,
		Type:     string(cfg.Type),
		Stack:    cfg.Stack,
	}
	manifest := buildManifest(templateVersion, params)
	manifestOp := operation{
		kind:    "write",
		path:    filepath.Join(targetAbs, manifestFileName),
		content: formatManifest(manifest),
		note:    "bootstrap manifest",
	}

	// Classify each canonical op:
	//   - bookkeeping write (template version marker): always apply
	//   - symlink with regular-file collision: conflict, skip
	//   - symlink otherwise: skipIfExists
	//   - write-op where target doesn't exist: apply
	//   - write-op where target exists + identical: apply (touchless)
	//   - write-op where target exists + differs:
	//       AssumeYes: apply (overwrite)
	//       otherwise: record collision, skip the write
	resolved := make([]operation, 0, len(canonical))
	var syncConflicts []conflict
	var collisions []collisionRecord
	for _, op := range canonical {
		if op.kind == "write" {
			isBookkeeping := op.note == "template version marker"
			if !isBookkeeping {
				existingBytes, readErr := os.ReadFile(op.path)
				switch {
				case errors.Is(readErr, os.ErrNotExist):
					// file doesn't exist — fall through to append (new write)
				case readErr != nil:
					return fmt.Errorf("read %s: %w", op.path, readErr)
				default:
					existing := string(existingBytes)
					if existing != op.content {
						if op.note == "base governance contract" {
							merged, sectionCollisions := mergeAgentsSections(op.content, existing, cfg.AssumeYes)
							rel := displayPath(targetAbs, op.path)
							for i := range sectionCollisions {
								sectionCollisions[i].path = rel + " § " + sectionCollisions[i].path
							}
							collisions = append(collisions, sectionCollisions...)
							op.content = merged
							if len(sectionCollisions) > 0 && !cfg.AssumeYes {
								resolved = append(resolved, op)
								continue
							}
							if cfg.AssumeYes {
								fmt.Printf("overwrite (--yes): %s (section-aware)\n", displayPath(targetAbs, op.path))
							}
						} else if cfg.AssumeYes {
							fmt.Printf("overwrite (--yes): %s\n", displayPath(targetAbs, op.path))
						} else {
							collisions = append(collisions, collisionRecord{
								path:     op.path,
								existing: existing,
								proposed: op.content,
							})
							resolved = append(resolved, operation{kind: "skip"})
							continue
						}
					}
					// existing == op.content: fall through — touchless write
				}
			}
		}
		if op.kind == "symlink" {
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
	if err := applyOperations(ops); err != nil {
		return err
	}

	// Write `.governa/sync-review.md` unless --yes was used (escape hatch
	// applies everything directly; no review loop needed).
	if !cfg.AssumeYes {
		reviewPath := filepath.Join(targetAbs, syncReviewFile)
		reviewContent := renderSyncReview(targetAbs, oldTemplateVersion, templateVersion, collisions)
		if err := os.MkdirAll(filepath.Dir(reviewPath), 0o755); err != nil {
			return fmt.Errorf("create %s: %w", governaDir, err)
		}
		if err := os.WriteFile(reviewPath, []byte(reviewContent), 0o644); err != nil {
			return fmt.Errorf("write sync review: %w", err)
		}
		printReviewSummary(targetAbs, reviewPath, len(collisions))
	}

	if cfg.InitGit {
		if err := maybeInitGit(targetAbs); err != nil {
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

// displayPath renders an absolute path as repo-relative when possible, for
// human-readable sync output.
func displayPath(targetAbs, absPath string) string {
	if rel, err := filepath.Rel(targetAbs, absPath); err == nil {
		return filepath.ToSlash(rel)
	}
	return absPath
}

// renderSyncReview produces the `.governa/sync-review.md` content. Scope is
// pending-decisions-only: colliding files each get a header + diff preview.
// Zero-collision case writes a summary-only review. Non-colliding writes are
// NOT listed here — `git diff` shows the rest.
func renderSyncReview(targetAbs, oldVersion, newVersion string, collisions []collisionRecord) string {
	var b strings.Builder
	fmt.Fprintln(&b, "# Governa Sync Review")
	fmt.Fprintln(&b)
	if oldVersion != "" && newVersion != "" && oldVersion != newVersion {
		fmt.Fprintf(&b, "Template version: %s → %s\n\n", oldVersion, newVersion)
	} else if newVersion != "" {
		fmt.Fprintf(&b, "Template version: %s\n\n", newVersion)
	}

	if len(collisions) == 0 {
		fmt.Fprintln(&b, "0 files need review. Sync is clean — non-colliding writes were applied automatically; see `git diff` for the full set of changes.")
		return b.String()
	}

	fmt.Fprintf(&b, "%d file(s) need review. Each entry below lists a file whose existing content differs from the template. Sync did NOT touch these files on disk; they are pending DEV + QA + Director decisions before adoption.\n\n", len(collisions))
	fmt.Fprintln(&b, "For each entry: review the diff, decide keep-as-is or adopt-template, and capture the decision in the next AC. Re-run `governa sync --yes` after the AC ships to apply all adopt decisions in a batch, or edit the files manually.")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Collisions")
	fmt.Fprintln(&b)

	sorted := append([]collisionRecord(nil), collisions...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].path < sorted[j].path })
	for _, c := range sorted {
		rel := displayPath(targetAbs, c.path)
		existingLines := strings.Count(c.existing, "\n")
		proposedLines := strings.Count(c.proposed, "\n")
		fmt.Fprintf(&b, "### `%s`\n\n", rel)
		fmt.Fprintf(&b, "Existing: %d lines · Template: %d lines\n\n", existingLines, proposedLines)
		fmt.Fprintln(&b, "```diff")
		fmt.Fprint(&b, unifiedDiffPreview(c.existing, c.proposed, 50))
		fmt.Fprintln(&b, "```")
		fmt.Fprintln(&b)
	}
	return b.String()
}

// unifiedDiffPreview produces a minimal unified-diff-style rendering of two
// strings, truncated to maxLines. No hunk headers or context calculation —
// each line is labelled `-` (existing), `+` (proposed), or ` ` (unchanged).
// Truncation is indicated with a trailing `... (N more lines)` marker.
func unifiedDiffPreview(existing, proposed string, maxLines int) string {
	eLines := strings.Split(existing, "\n")
	pLines := strings.Split(proposed, "\n")

	var out strings.Builder
	i, j, emitted := 0, 0, 0
	for i < len(eLines) || j < len(pLines) {
		if emitted >= maxLines {
			remaining := (len(eLines) - i) + (len(pLines) - j)
			fmt.Fprintf(&out, "... (%d more lines; see full file on disk)\n", remaining)
			return out.String()
		}
		switch {
		case i < len(eLines) && j < len(pLines) && eLines[i] == pLines[j]:
			fmt.Fprintf(&out, " %s\n", eLines[i])
			i++
			j++
		case j < len(pLines) && (i >= len(eLines) || eLines[i] != pLines[j]):
			fmt.Fprintf(&out, "+%s\n", pLines[j])
			j++
		case i < len(eLines):
			fmt.Fprintf(&out, "-%s\n", eLines[i])
			i++
		}
		emitted++
	}
	return out.String()
}

// printReviewSummary emits a one-line stdout summary pointing the operator at
// the review doc. Distinct counts for clean vs review-needed runs so the
// summary is actionable.
func printReviewSummary(targetAbs, reviewPath string, collisionCount int) {
	rel := displayPath(targetAbs, reviewPath)
	if collisionCount == 0 {
		fmt.Printf("%s %s (0 collisions)\n", color.Yel("review:"), rel)
		return
	}
	plural := "collision"
	if collisionCount != 1 {
		plural = "collisions"
	}
	fmt.Printf("%s %s (%d %s — review + decide)\n", color.Yel("review:"), rel, collisionCount, plural)
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
		case "mkdocs.yml", "mkdocs.yaml":
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
	syncReviewFile:              true, // .governa/sync-review.md (AC79 Part B review artifact)
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
	// docs/roles/* is owned by governa (role docs per overlay)
	if strings.HasPrefix(norm, "docs/roles/") {
		return true
	}
	return false
}

func expectedArtifactPaths(repoType RepoType) []string {
	base := []string{"AGENTS.md", "CLAUDE.md", "TEMPLATE_VERSION"}
	switch repoType {
	case RepoTypeCode:
		return append(
			base,
			"README.md",
			"arch.md",
			"plan.md",
			"CHANGELOG.md",
			filepath.Join("docs", "README.md"),
			filepath.Join("docs", "development-cycle.md"),
			filepath.Join("docs", "ac-template.md"),
			filepath.Join("docs", "build-release.md"),
		)
	case RepoTypeDoc:
		return append(base, "plan.md")
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
		"{{REPO_NAME}}":         cfg.RepoName,
		"{{STACK_OR_PLATFORM}}": valueOrDefault(cfg.Stack, "TBD"),
		"{{MODULE_PATH}}":       modulePath,
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

func applyOperations(ops []operation) error {
	for _, op := range ops {
		switch op.kind {
		case "mkdir":
			fmt.Printf("mkdir %s (%s)\n", op.path, op.note)
			if err := os.MkdirAll(op.path, 0o755); err != nil {
				return fmt.Errorf("create directory %s: %w", op.path, err)
			}
		case "write":
			fmt.Printf("write %s (%s)\n", op.path, op.note)
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
			fmt.Printf("symlink %s -> %s (%s)\n", op.path, op.linkTo, op.note)
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

func maybeInitGit(targetRoot string) error {
	gitDir := filepath.Join(targetRoot, ".git")
	if _, err := os.Stat(gitDir); err == nil {
		fmt.Printf("skip %s (git repo already present)\n", gitDir)
		return nil
	}
	fmt.Printf("exec git init %s\n", targetRoot)
	cmd := exec.Command("git", "init", targetRoot)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git init %s: %w: %s", targetRoot, err, strings.TrimSpace(string(output)))
	}
	return nil
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
// set (test harnesses passing an explicit repo root), it reads from the
// TEMPLATE_VERSION file on disk. When repoRoot is empty (installed binary,
// consumer modes), it falls back to the compiled-in templates.TemplateVersion
// constant.
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

// agentsSection is one `## Heading` block parsed from an AGENTS.md file.
type agentsSection struct {
	heading string // e.g. "Governed Sections"
	body    string // full text including the `## ` line and trailing content
}

// parseAgentsSections splits AGENTS.md content into a preamble (everything
// before the first `## `) and an ordered slice of sections keyed by heading.
func parseAgentsSections(content string) (preamble string, sections []agentsSection) {
	lines := strings.SplitAfter(content, "\n")
	var cur *agentsSection
	for _, line := range lines {
		trimmed := strings.TrimRight(line, "\n")
		if strings.HasPrefix(trimmed, "## ") {
			if cur != nil {
				sections = append(sections, *cur)
			}
			heading := strings.TrimSpace(trimmed[3:])
			cur = &agentsSection{heading: heading, body: line}
			continue
		}
		if cur != nil {
			cur.body += line
		} else {
			preamble += line
		}
	}
	if cur != nil {
		sections = append(sections, *cur)
	}
	return preamble, sections
}

// mergeAgentsSections performs section-aware merge for AGENTS.md. Project Rules
// is consumer-owned and always preserved. All other sections are template-managed.
func mergeAgentsSections(templateContent, existingContent string, assumeYes bool) (merged string, collisions []collisionRecord) {
	tPreamble, tSections := parseAgentsSections(templateContent)
	ePreamble, eSections := parseAgentsSections(existingContent)

	eByHeading := make(map[string]agentsSection, len(eSections))
	for _, s := range eSections {
		eByHeading[s.heading] = s
	}

	tHeadings := make(map[string]bool, len(tSections))
	for _, s := range tSections {
		tHeadings[s.heading] = true
	}

	var out strings.Builder

	// Preamble: template-managed.
	if ePreamble != tPreamble {
		if assumeYes {
			out.WriteString(tPreamble)
		} else {
			out.WriteString(ePreamble)
			collisions = append(collisions, collisionRecord{
				path:     "preamble",
				existing: ePreamble,
				proposed: tPreamble,
			})
		}
	} else {
		out.WriteString(tPreamble)
	}

	// Template-managed sections in template order.
	for _, ts := range tSections {
		if ts.heading == "Project Rules" {
			// Consumer-owned: preserve existing content.
			if es, ok := eByHeading[ts.heading]; ok {
				out.WriteString(es.body)
			} else {
				out.WriteString(ts.body)
			}
			continue
		}
		es, exists := eByHeading[ts.heading]
		if !exists || es.body == ts.body {
			out.WriteString(ts.body)
			continue
		}
		// Consumer edited a template-managed section.
		if assumeYes {
			out.WriteString(ts.body)
		} else {
			out.WriteString(es.body)
			collisions = append(collisions, collisionRecord{
				path:     ts.heading,
				existing: es.body,
				proposed: ts.body,
			})
		}
	}

	// Preserve unknown consumer sections not in the template.
	for _, es := range eSections {
		if !tHeadings[es.heading] {
			out.WriteString(es.body)
		}
	}

	merged = out.String()
	return merged, collisions
}
