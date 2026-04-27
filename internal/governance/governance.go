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
	"strings"

	"github.com/queone/governa/internal/color"
	"github.com/queone/governa/internal/templates"
)

type Mode string

const (
	ModeApply Mode = "apply"
)

type RepoType string

const (
	RepoTypeCode RepoType = "CODE"
	RepoTypeDoc  RepoType = "DOC"
)

type Config struct {
	Mode     Mode
	Target   string
	Type     RepoType
	RepoName string
	Stack    string
	InitGit  bool
}

type Assessment struct {
	RepoShape         string
	ResolvedType      RepoType // type used to compute expected artifacts; resolved from RepoShape when caller passed ""
	ExistingArtifacts []string
	OverwriteRisk     string
	CodeSignals       int
	DocSignals        int
	OverwrittenFiles  []string
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

type flagValues struct {
	target   string
	repoType string
	repoName string
	stack    string
	initGit  bool
}

// RunWithFS dispatches the one supported mode (apply) against the template FS.
func RunWithFS(tfs fs.FS, cfg Config) error {
	switch cfg.Mode {
	case ModeApply:
		return runApply(tfs, cfg)
	default:
		return fmt.Errorf("unsupported mode %q", cfg.Mode)
	}
}

// ParseModeArgs parses flags for the given mode.
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
		Mode:     mode,
		Target:   target,
		Type:     RepoType(strings.ToUpper(strings.TrimSpace(values.repoType))),
		RepoName: strings.TrimSpace(values.repoName),
		Stack:    strings.TrimSpace(values.stack),
		InitGit:  values.initGit,
	}
	// Validation is deferred to runApply (after prompts).
	return cfg, false, nil
}

// ModeHelp returns mode-specific flag usage text.
func ModeHelp(mode Mode) string {
	switch mode {
	case ModeApply:
		return color.FormatUsage("governa apply [options]", []color.UsageLine{
			{Flag: "-n, --repo-name", Desc: "repo name"},
			{Flag: "-k, --type", Desc: "repo type: CODE or DOC"},
			{Flag: "-s, --stack", Desc: "stack or platform (CODE repos; currently: Go)"},
			{Flag: "-t, --target", Desc: "target directory (default: current dir)"},
			{Flag: "-g, --init-git", Desc: "initialize git if target is not a repo"},
		}, "Apply governance template to a new or existing repo. Detects repo state and prompts for missing parameters. After apply, all files are consumer-owned.")
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
// explicit flag > inference from target directory.
// It returns the resolved config and a list of source annotations for display.
func resolveAdoptParams(cfg Config, targetDir string) (Config, []paramSource) {
	var sources []paramSource

	if cfg.RepoName == "" {
		cfg.RepoName = inferRepoName(targetDir)
		sources = append(sources, paramSource{"repo-name", cfg.RepoName, "inferred"})
	} else {
		sources = append(sources, paramSource{"repo-name", cfg.RepoName, "flag"})
	}

	if cfg.Stack == "" {
		cfg.Stack = inferStack(targetDir)
		if cfg.Stack != "" {
			sources = append(sources, paramSource{"stack", cfg.Stack, "inferred"})
		}
	} else {
		sources = append(sources, paramSource{"stack", cfg.Stack, "flag"})
	}

	return cfg, sources
}

type paramSource struct {
	name   string
	value  string
	source string // "flag", "inferred"
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
	case ModeApply:
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

// detectApplyMode inspects the target directory and returns one of:
//   - "existing" — governance artifacts found
//   - "new"      — fresh directory
func detectApplyMode(targetDir string) string {
	for _, artifact := range []string{"AGENTS.md", "CLAUDE.md"} {
		if _, err := os.Stat(filepath.Join(targetDir, artifact)); err == nil {
			return "existing"
		}
	}
	if matches, _ := filepath.Glob(filepath.Join(targetDir, "docs", "role-*.md")); len(matches) > 0 {
		return "existing"
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
// Fields already set (via flags or inference) are not prompted.
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

// runApply writes the base template + overlay files to the target directory.
// After apply, all files are consumer-owned.
func runApply(tfs fs.FS, cfg Config) error {
	targetAbs, err := filepath.Abs(cfg.Target)
	if err != nil {
		return fmt.Errorf("resolve target path: %w", err)
	}
	if err := os.MkdirAll(targetAbs, 0o755); err != nil {
		return fmt.Errorf("create target directory: %w", err)
	}

	applyMode := detectApplyMode(targetAbs)
	existing := applyMode != "new"

	if existing {
		fmt.Fprintln(os.Stderr, "existing governance files detected; apply will overwrite them")
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

	canonical, err := planCanonical(tfs, cfg, targetAbs)
	if err != nil {
		return err
	}

	resolved := make([]operation, 0, len(canonical))
	for _, op := range canonical {
		if op.kind == "symlink" {
			info, err := os.Lstat(op.path)
			if err == nil && info.Mode()&os.ModeSymlink == 0 {
				rel, _ := filepath.Rel(targetAbs, op.path)
				fmt.Fprintf(os.Stderr, "warning: %s exists as a regular file; expected symlink to %s — delete the file and re-run to create the symlink\n", rel, op.linkTo)
				resolved = append(resolved, operation{kind: "skip"})
				continue
			}
			resolved = append(resolved, skipIfExists(op))
			continue
		}
		resolved = append(resolved, op)
	}

	ops := compactOperations(resolved)
	if err := applyOperations(ops); err != nil {
		return err
	}

	// Write adoption AC
	applyACPath := filepath.Join(targetAbs, "docs", "ac1-governa-apply.md")
	applyACContent := renderApplyAC(templates.TemplateVersion, cfg, canonical)
	if err := os.MkdirAll(filepath.Dir(applyACPath), 0o755); err != nil {
		return fmt.Errorf("create docs/: %w", err)
	}
	if err := os.WriteFile(applyACPath, []byte(applyACContent), 0o644); err != nil {
		return fmt.Errorf("write adoption AC: %w", err)
	}
	fmt.Printf("write %s (adoption record)\n", displayPath(targetAbs, applyACPath))

	if cfg.InitGit {
		if err := maybeInitGit(targetAbs); err != nil {
			return err
		}
	}

	return nil
}

// displayPath renders an absolute path as repo-relative when possible.
func displayPath(targetAbs, absPath string) string {
	if rel, err := filepath.Rel(targetAbs, absPath); err == nil {
		return filepath.ToSlash(rel)
	}
	return absPath
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
			RepoShape:     "empty",
			ResolvedType:  repoType, // no files to infer from; preserve caller input (possibly "")
			OverwriteRisk: "low",
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
	// cfg.Type was pre-populated from flags or inference.
	resolvedType := repoType
	if resolvedType == "" {
		resolvedType = deriveTypeFromShape(repoShape)
	}

	expected := expectedArtifactPaths(resolvedType)
	var existing []string
	var overwrites []string
	for _, rel := range expected {
		full := filepath.Join(root, rel)
		info, err := os.Stat(full)
		if err == nil {
			existing = append(existing, rel)
			if !info.IsDir() && info.Size() > 0 {
				overwrites = append(overwrites, rel)
			}
		}
	}

	overwriteRisk := "low"
	switch {
	case len(overwrites) >= 3:
		overwriteRisk = "high"
	case len(overwrites) > 0:
		overwriteRisk = "medium"
	}

	return Assessment{
		RepoShape:         repoShape,
		ResolvedType:      resolvedType,
		ExistingArtifacts: existing,
		OverwriteRisk:     overwriteRisk,
		CodeSignals:       codeSignals,
		DocSignals:        docSignals,
		OverwrittenFiles:  overwrites,
	}, nil
}

func expectedArtifactPaths(repoType RepoType) []string {
	base := []string{"AGENTS.md", "CLAUDE.md"}
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

func planCanonical(tfs fs.FS, cfg Config, targetRoot string) ([]operation, error) {
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
	fmt.Printf("overwrite-risk: %s\n", a.OverwriteRisk)
	if len(a.OverwrittenFiles) > 0 && !slices.Equal(a.OverwrittenFiles, a.ExistingArtifacts) {
		fmt.Printf("overwrites: %s\n", strings.Join(a.OverwrittenFiles, ", "))
	}
}

func skipIfExists(op operation) operation {
	if _, err := os.Stat(op.path); err == nil {
		return operation{kind: "skip"}
	}
	return op
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

// renderApplyAC produces the docs/ac1-governa-apply.md adoption record.
func renderApplyAC(templateVersion string, cfg Config, ops []operation) string {
	var b strings.Builder
	fmt.Fprintln(&b, "# AC1 Governa Apply")
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "Applied governa v%s governance template (%s overlay) to %s.\n", templateVersion, cfg.Type, cfg.RepoName)
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Summary")
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "Applied governa v%s governance template (%s overlay). All files below are now consumer-owned — modify freely to fit the repo's needs.\n", templateVersion, cfg.Type)
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## In Scope")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Files written by governa apply:")
	fmt.Fprintln(&b)
	for _, op := range ops {
		if op.kind == "skip" {
			continue
		}
		fmt.Fprintf(&b, "- `%s`", filepath.Base(op.path))
		if op.note != "" {
			fmt.Fprintf(&b, " (%s)", op.note)
		}
		fmt.Fprintln(&b)
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Out Of Scope")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "- All applied files are consumer-owned and can be freely modified")
	fmt.Fprintln(&b, "- Governa is not a runtime dependency — this repo does not import or inherit from the template repo")
	fmt.Fprintln(&b, "- Future governa improvements can be adopted by having a coding agent read the governa repo and cherry-pick useful changes")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Acceptance Tests")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "**AT1** [Manual] — Verify AGENTS.md exists and sections match repo needs.")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "**AT2** [Manual] — Verify role files in docs/role-*.md reflect the repo's delivery model.")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "**AT3** [Manual] — Verify CLAUDE.md is a symlink to AGENTS.md.")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Status")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "`PENDING` — review applied governance and adapt to repo needs.")
	return b.String()
}
