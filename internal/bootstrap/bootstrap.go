package bootstrap

import (
	"cmp"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
)

type Mode string

const (
	ModeNew     Mode = "new"
	ModeAdopt   Mode = "adopt"
	ModeEnhance Mode = "enhance"
)

type RepoType string

const (
	RepoTypeCode RepoType = "CODE"
	RepoTypeDoc  RepoType = "DOC"
)

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
}

type flagValues struct {
	mode               string
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

func ParseArgs(args []string) (Config, error) {
	values := flagValues{}
	fs := flag.NewFlagSet("bootstrap", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&values.mode, "m", "", "mode: new|adopt|enhance")
	fs.StringVar(&values.mode, "mode", "", "mode: new|adopt|enhance")
	fs.StringVar(&values.target, "t", "", "target directory")
	fs.StringVar(&values.target, "target", "", "target directory")
	fs.StringVar(&values.reference, "r", "", "reference repo for enhance")
	fs.StringVar(&values.reference, "reference", "", "reference repo for enhance")
	fs.StringVar(&values.repoType, "y", "", "repo type: CODE|DOC")
	fs.StringVar(&values.repoType, "type", "", "repo type: CODE|DOC")
	fs.StringVar(&values.repoName, "n", "", "repo name")
	fs.StringVar(&values.repoName, "repo-name", "", "repo name")
	fs.StringVar(&values.purpose, "p", "", "project purpose")
	fs.StringVar(&values.purpose, "purpose", "", "project purpose")
	fs.StringVar(&values.stack, "s", "", "stack or platform for CODE repos")
	fs.StringVar(&values.stack, "stack", "", "stack or platform for CODE repos")
	fs.StringVar(&values.publishingPlatform, "u", "", "publishing platform for DOC repos")
	fs.StringVar(&values.publishingPlatform, "publishing-platform", "", "publishing platform for DOC repos")
	fs.StringVar(&values.style, "v", "", "style or voice for DOC repos")
	fs.StringVar(&values.style, "style", "", "style or voice for DOC repos")
	fs.BoolVar(&values.initGit, "g", false, "initialize git if target is not already a repo")
	fs.BoolVar(&values.initGit, "init-git", false, "initialize git if target is not already a repo")
	fs.BoolVar(&values.dryRun, "d", false, "preview changes without writing")
	fs.BoolVar(&values.dryRun, "dry-run", false, "preview changes without writing")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: bootstrap -m, --mode new|adopt|enhance [options]\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	target := strings.TrimSpace(values.target)
	if target == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return Config{}, fmt.Errorf("resolve current working directory: %w", err)
		}
		target = cwd
	}

	cfg := Config{
		Mode:               Mode(strings.ToLower(strings.TrimSpace(values.mode))),
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
	return cfg, validateConfig(cfg)
}

func Run(cfg Config) error {
	root, err := templateRoot()
	if err != nil {
		return err
	}

	switch cfg.Mode {
	case ModeNew:
		return runNewOrAdopt(root, cfg, false)
	case ModeAdopt:
		return runNewOrAdopt(root, cfg, true)
	case ModeEnhance:
		return runEnhance(root, cfg)
	default:
		return fmt.Errorf("unsupported mode %q", cfg.Mode)
	}
}

func validateConfig(cfg Config) error {
	switch cfg.Mode {
	case ModeNew:
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
	case ModeAdopt:
		if cfg.RepoName == "" {
			return errors.New("repo name is required: use -n or --repo-name")
		}
		if cfg.Purpose == "" {
			return errors.New("project purpose is required: use -p or --purpose")
		}
		if cfg.Type != "" && cfg.Type != RepoTypeCode && cfg.Type != RepoTypeDoc {
			return errors.New("repo type must be CODE or DOC when provided: use -y or --type")
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
		if cfg.Reference == "" {
			return errors.New("reference repo is required for enhance mode: use -r or --reference")
		}
	default:
		return errors.New("mode is required: use -m or --mode")
	}
	return nil
}

func templateRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("resolve template root: runtime caller unavailable")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	info, err := os.Stat(root)
	if err != nil {
		return "", fmt.Errorf("stat template root: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("template root is not a directory: %s", root)
	}
	return root, nil
}

func runNewOrAdopt(root string, cfg Config, adopt bool) error {
	targetAbs, err := filepath.Abs(cfg.Target)
	if err != nil {
		return fmt.Errorf("resolve target path: %w", err)
	}
	if err := os.MkdirAll(targetAbs, 0o755); err != nil && !cfg.DryRun {
		return fmt.Errorf("create target directory: %w", err)
	}

	assessment, err := AssessTarget(targetAbs, cfg.Type)
	if err != nil {
		return err
	}
	if adopt && cfg.Type == "" {
		switch assessment.RepoShape {
		case "likely CODE":
			cfg.Type = RepoTypeCode
		case "likely DOC":
			cfg.Type = RepoTypeDoc
		default:
			return errors.New("repo type could not be inferred confidently for adopt mode; use -y or --type")
		}
	}
	printAssessment(cfg.Mode, targetAbs, assessment)

	ops, err := planRender(root, cfg, targetAbs, adopt)
	if err != nil {
		return err
	}
	if err := applyOperations(ops, cfg.DryRun); err != nil {
		return err
	}
	if cfg.InitGit {
		if err := maybeInitGit(targetAbs, cfg.DryRun); err != nil {
			return err
		}
	}
	return nil
}

func runEnhance(root string, cfg Config) error {
	refAbs, err := filepath.Abs(cfg.Reference)
	if err != nil {
		return fmt.Errorf("resolve reference path: %w", err)
	}
	report, err := ReviewEnhancement(root, refAbs)
	if err != nil {
		return err
	}
	printEnhancementReport(report)
	reportPath := filepath.Join(root, "docs", "enhance-report.md")
	reportContent := renderEnhancementReport(report)
	if cfg.DryRun {
		fmt.Printf("dry-run write %s (enhancement review report)\n", reportPath)
		fmt.Println("dry-run: no template changes applied")
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(reportPath), 0o755); err != nil {
		return fmt.Errorf("create report directory: %w", err)
	}
	if err := os.WriteFile(reportPath, []byte(reportContent), 0o644); err != nil {
		return fmt.Errorf("write enhancement report: %w", err)
	}
	fmt.Printf("write %s (enhancement review report)\n", reportPath)
	fmt.Println("enhance mode is report-first: no template changes applied")
	return nil
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

func ReviewEnhancement(templateRoot, referenceRoot string) (EnhancementReport, error) {
	var candidates []EnhancementCandidate
	if governanceCandidates, err := reviewGovernedSections(templateRoot, referenceRoot); err != nil {
		return EnhancementReport{}, err
	} else {
		candidates = append(candidates, governanceCandidates...)
	}

	mappings := []enhancementMapping{
		{Area: "bootstrap behavior", ReferencePaths: []string{filepath.Join("cmd", "bootstrap", "main.go"), filepath.Join("scripts", "bootstrap")}, TemplateTarget: filepath.Join("cmd", "bootstrap", "main.go")},
		{Area: "bootstrap behavior", ReferencePaths: []string{filepath.Join("scripts", "bootstrap")}, TemplateTarget: filepath.Join("scripts", "README.md")},
		{Area: "CODE overlay", ReferencePaths: []string{"README.md"}, TemplateTarget: filepath.Join("overlays", "code", "files", "README.md.tmpl")},
		{Area: "CODE overlay", ReferencePaths: []string{"arch.md"}, TemplateTarget: filepath.Join("overlays", "code", "files", "arch.md.tmpl")},
		{Area: "CODE overlay", ReferencePaths: []string{"plan.md"}, TemplateTarget: filepath.Join("overlays", "code", "files", "plan.md.tmpl")},
		{Area: "CODE overlay", ReferencePaths: []string{filepath.Join("docs", "development-cycle.md")}, TemplateTarget: filepath.Join("overlays", "code", "files", "docs", "development-cycle.md.tmpl")},
		{Area: "CODE overlay", ReferencePaths: []string{filepath.Join("docs", "ac-template.md")}, TemplateTarget: filepath.Join("overlays", "code", "files", "docs", "ac-template.md.tmpl")},
		{Area: "CODE overlay", ReferencePaths: []string{filepath.Join("docs", "build-release.md")}, TemplateTarget: filepath.Join("overlays", "code", "files", "docs", "build-release.md.tmpl")},
		{Area: "CODE overlay", ReferencePaths: []string{"build.sh"}, TemplateTarget: filepath.Join("overlays", "code", "files", "build.sh.tmpl")},
		{Area: "CODE overlay", ReferencePaths: []string{filepath.Join("cmd", "build", "main.go")}, TemplateTarget: filepath.Join("overlays", "code", "files", "cmd", "build", "main.go.tmpl")},
		{Area: "CODE overlay", ReferencePaths: []string{filepath.Join("cmd", "rel", "main.go")}, TemplateTarget: filepath.Join("overlays", "code", "files", "cmd", "rel", "main.go.tmpl")},
		{Area: "DOC overlay", ReferencePaths: []string{"README.md"}, TemplateTarget: filepath.Join("overlays", "doc", "files", "README.md.tmpl")},
		{Area: "DOC overlay", ReferencePaths: []string{"style.md", "voice.md"}, TemplateTarget: filepath.Join("overlays", "doc", "files", "style.md.tmpl")},
		{Area: "DOC overlay", ReferencePaths: []string{"content-plan.md", "calendar.md"}, TemplateTarget: filepath.Join("overlays", "doc", "files", "content-plan.md.tmpl")},
		{Area: "DOC overlay", ReferencePaths: []string{"publishing-workflow.md"}, TemplateTarget: filepath.Join("overlays", "doc", "files", "publishing-workflow.md.tmpl")},
		{Area: "DOC overlay", ReferencePaths: []string{"release.md"}, TemplateTarget: filepath.Join("overlays", "doc", "files", "release.md.tmpl")},
		{Area: "DOC overlay", ReferencePaths: []string{"rel.sh"}, TemplateTarget: filepath.Join("overlays", "doc", "files", "rel.sh.tmpl")},
		{Area: "DOC overlay", ReferencePaths: []string{filepath.Join("cmd", "rel", "main.go")}, TemplateTarget: filepath.Join("overlays", "doc", "files", "cmd", "rel", "main.go.tmpl")},
		{Area: "examples or upgrade path", ReferencePaths: []string{"TEMPLATE_VERSION"}, TemplateTarget: "TEMPLATE_VERSION"},
	}
	for _, item := range mappings {
		candidate, ok, err := reviewMappedFile(templateRoot, referenceRoot, item)
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

func planRender(root string, cfg Config, targetRoot string, adopt bool) ([]operation, error) {
	placeholders := map[string]string{
		"{{REPO_NAME}}":           cfg.RepoName,
		"{{PROJECT_PURPOSE}}":     cfg.Purpose,
		"{{STACK_OR_PLATFORM}}":   valueOrDefault(cfg.Stack, "TBD"),
		"{{PUBLISHING_PLATFORM}}": valueOrDefault(cfg.PublishingPlatform, "TBD"),
		"{{DOC_STYLE}}":           valueOrDefault(cfg.Style, "TBD"),
	}

	agentsContent, err := readAndRender(filepath.Join(root, "base", "AGENTS.md"), placeholders)
	if err != nil {
		return nil, err
	}
	ops := []operation{{
		kind:    "write",
		path:    filepath.Join(targetRoot, "AGENTS.md"),
		content: agentsContent,
		note:    "base governance contract",
	}}

	if adopt {
		ops[0] = proposeIfExists(ops[0])
	}

	versionContent, err := os.ReadFile(filepath.Join(root, "TEMPLATE_VERSION"))
	if err != nil {
		return nil, fmt.Errorf("read template version: %w", err)
	}
	versionOp := operation{
		kind:    "write",
		path:    filepath.Join(targetRoot, "TEMPLATE_VERSION"),
		content: string(versionContent),
		note:    "template version marker",
	}
	if adopt {
		versionOp = skipIfExists(versionOp)
	}
	ops = append(ops, versionOp)

	linkOp := operation{
		kind:   "symlink",
		path:   filepath.Join(targetRoot, "CLAUDE.md"),
		linkTo: "AGENTS.md",
		note:   "agent alias link",
	}
	if adopt {
		linkOp = skipIfExists(linkOp)
	}
	ops = append(ops, linkOp)

	overlayRoot := filepath.Join(root, "overlays", strings.ToLower(string(cfg.Type)), "files")
	err = filepath.WalkDir(overlayRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(overlayRoot, path)
		if err != nil {
			return err
		}
		if cfg.Type == RepoTypeCode && !stackSuggestsGo(cfg.Stack) &&
			(rel == filepath.Join("cmd", "rel", "main.go.tmpl") ||
				rel == filepath.Join("cmd", "rel", "color.go.tmpl") ||
				rel == filepath.Join("cmd", "build", "main.go.tmpl") ||
				rel == filepath.Join("cmd", "build", "color.go.tmpl")) {
			return nil
		}
		targetRel := strings.TrimSuffix(rel, ".tmpl")
		content, err := readAndRender(path, placeholders)
		if err != nil {
			return err
		}
		op := operation{
			kind:    "write",
			path:    filepath.Join(targetRoot, targetRel),
			content: content,
			note:    "overlay file",
		}
		if adopt {
			op = proposeIfExists(op)
		}
		ops = append(ops, op)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk overlay templates: %w", err)
	}

	return compactOperations(ops), nil
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

func printEnhancementReport(report EnhancementReport) {
	fmt.Printf("mode: enhance\n")
	fmt.Printf("reference: %s\n", displayReferenceRoot())
	if len(report.Candidates) == 0 {
		fmt.Println("candidates: none")
		return
	}
	fmt.Printf("candidates: %d\n", len(report.Candidates))
	for _, c := range report.Candidates {
		fmt.Printf("- area=%s path=%s disposition=%s portability=%s", c.Area, displayReferencePath(report.ReferenceRoot, c.Path), c.Disposition, c.Portability)
		if c.Section != "" {
			fmt.Printf(" section=%s", c.Section)
		}
		if c.TemplateTarget != "" {
			fmt.Printf(" template-target=%s", c.TemplateTarget)
		}
		fmt.Printf(" collision-impact=%s", c.CollisionImpact)
		fmt.Printf(" reason=%s", c.Reason)
		if c.Summary != "" {
			fmt.Printf(" summary=%s", c.Summary)
		}
		fmt.Println()
	}
}

func reviewGovernedSections(templateRoot, referenceRoot string) ([]EnhancementCandidate, error) {
	refPath := filepath.Join(referenceRoot, "AGENTS.md")
	refInfo, err := os.Stat(refPath)
	if err != nil || refInfo.IsDir() {
		return nil, nil
	}

	templatePath := filepath.Join(templateRoot, "base", "AGENTS.md")
	templateContent, err := os.ReadFile(templatePath)
	if err != nil {
		return nil, fmt.Errorf("read template governance file %s: %w", templatePath, err)
	}
	refContent, err := os.ReadFile(refPath)
	if err != nil {
		return nil, fmt.Errorf("read reference governance file %s: %w", refPath, err)
	}

	templateSections := sectionMap(parseLevel2Sections(string(templateContent)))
	refSections := sectionMap(parseLevel2Sections(string(refContent)))
	governed := []string{
		"Purpose",
		"Governed Sections",
		"Interaction Mode",
		"Approval Boundaries",
		"Review Style",
		"File-Change Discipline",
		"Release Or Publish Triggers",
		"Documentation Update Expectations",
	}

	var candidates []EnhancementCandidate
	for _, section := range governed {
		refBody, ok := refSections[section]
		if !ok {
			continue
		}
		templateBody := templateSections[section]
		if governanceSectionCovered(section, templateBody, refBody) {
			continue
		}
		portability, disposition, reason := classifyEnhancement(refBody, referenceRoot, filepath.Join("base", "AGENTS.md"), true)
		candidates = append(candidates, EnhancementCandidate{
			Area:            "base governance",
			Path:            refPath,
			Section:         section,
			Disposition:     disposition,
			Reason:          reason,
			Portability:     portability,
			TemplateTarget:  filepath.Join("base", "AGENTS.md"),
			Summary:         summarizeSectionDelta(section, refBody),
			CollisionImpact: "medium",
		})
	}
	return candidates, nil
}

func reviewMappedFile(templateRoot, referenceRoot string, item enhancementMapping) (EnhancementCandidate, bool, error) {
	refPath, ok := firstExistingPath(referenceRoot, item.ReferencePaths)
	if !ok {
		return EnhancementCandidate{}, false, nil
	}

	refContent, err := os.ReadFile(refPath)
	if err != nil {
		return EnhancementCandidate{}, false, fmt.Errorf("read reference file %s: %w", refPath, err)
	}

	targetPath := filepath.Join(templateRoot, item.TemplateTarget)
	targetContent, err := os.ReadFile(targetPath)
	targetExists := err == nil
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return EnhancementCandidate{}, false, fmt.Errorf("read template file %s: %w", targetPath, err)
	}
	if targetExists && normalizedEqual(string(refContent), string(targetContent)) {
		return EnhancementCandidate{}, false, nil
	}

	portability, disposition, reason := classifyEnhancement(string(refContent), referenceRoot, item.TemplateTarget, false)
	collisionImpact := "low"
	if targetExists {
		collisionImpact = "medium"
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
	return true
}

func normalizeText(value string) string {
	lines := strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func sectionSignals(section, body string) map[string]bool {
	text := normalizedSignalText(body)
	signals := map[string]bool{}
	addSignal := func(name string, ok bool) {
		if ok {
			signals[name] = true
		}
	}
	switch section {
	case "Interaction Mode":
		addSignal("exploratory-discussion-default", containsAll(text, "exploratory", "discussion"))
		addSignal("changes-need-authorization", (containsAll(text, "create", "artifacts") || containsAll(text, "make", "changes")) && containsAny(text, "authoriz", "authoris"))
		addSignal("minimal-change-on-authorization", containsAll(text, "smallest", "change") || containsAll(text, "minimal", "change"))
		addSignal("surface-assumptions", containsAny(text, "assumptions", "ambiguities", "missing context"))
	case "Approval Boundaries":
		addSignal("destructive-needs-approval", containsAny(text, "destructive"))
		addSignal("release-needs-approval", containsAny(text, "release", "publish", "deploy"))
		addSignal("governance-needs-approval", containsAny(text, "governance files", "ci", "secrets", "external integrations"))
	case "Review Style":
		addSignal("review-findings-first", containsAll(text, "findings", "before"))
		addSignal("review-bugs-regressions", containsAny(text, "bugs", "regressions", "missing tests"))
		addSignal("review-evidence", containsAny(text, "concrete evidence", "file paths", "coverage"))
	case "File-Change Discipline":
		addSignal("targeted-edits", containsAny(text, "targeted edits", "broad rewrites"))
		addSignal("preserve-user-changes", containsAny(text, "preserve user changes", "unrelated local modifications"))
		addSignal("docs-in-same-pass", containsAny(text, "directly affected docs", "self-contained"))
	case "Release Or Publish Triggers":
		addSignal("release-only-on-request", containsAny(text, "explicitly asks", "explicitly asks for it", "release-scoped"))
		addSignal("release-artifacts-same-pass", containsAny(text, "required release artifacts", "same pass"))
	case "Documentation Update Expectations":
		addSignal("docs-align-with-behavior", containsAny(text, "aligned with behavior", "drift"))
		addSignal("update-user-facing-docs", containsAny(text, "user-facing docs", "operating instructions", "setup", "workflows"))
		addSignal("update-arch-plan-style-when-material", containsAny(text, "architecture", "planning", "style docs"))
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

func classifyEnhancement(content, referenceRoot, templateTarget string, governance bool) (string, string, string) {
	if markers := projectSpecificMarkers(content, referenceRoot); len(markers) > 0 {
		return "project-specific", "defer", "content appears tied to the reference repo and should not be imported directly"
	}
	if governance {
		return "portable", "accept", "section-level governance delta is reusable enough to review directly against the base contract"
	}
	if strings.HasSuffix(templateTarget, ".go.tmpl") || strings.HasSuffix(templateTarget, ".sh.tmpl") || templateTarget == "TEMPLATE_VERSION" {
		return "portable", "accept", "workflow helper or release artifact is concrete and portable enough for direct template review"
	}
	return "needs-review", "adapt", "artifact may contain reusable structure, but the content should be adapted before it becomes template baseline"
}

func projectSpecificMarkers(content, referenceRoot string) []string {
	lower := strings.ToLower(content)
	refName := strings.ToLower(filepath.Base(referenceRoot))
	var markers []string
	if refName != "" && strings.Contains(lower, refName) {
		markers = append(markers, "mentions reference repo name")
	}
	if strings.Contains(content, "/Users/") || strings.Contains(content, "\\Users\\") {
		markers = append(markers, "contains absolute user path")
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

func renderEnhancementReport(report EnhancementReport) string {
	var b strings.Builder
	b.WriteString("# Enhance Report\n\n")
	b.WriteString(fmt.Sprintf("Reference repo: `%s`\n\n", displayReferenceRoot()))
	b.WriteString(fmt.Sprintf("Candidate count: %d\n\n", len(report.Candidates)))
	counts := countEnhancementCandidates(report.Candidates)
	b.WriteString("## Summary\n\n")
	b.WriteString(fmt.Sprintf("- `accept`: %d\n", counts["accept"]))
	b.WriteString(fmt.Sprintf("- `adapt`: %d\n", counts["adapt"]))
	b.WriteString(fmt.Sprintf("- `defer`: %d\n", counts["defer"]))
	b.WriteString(fmt.Sprintf("- `reject`: %d\n\n", counts["reject"]))

	if len(report.Candidates) == 0 {
		b.WriteString("## Candidates\n\nNo enhancement candidates were found.\n")
		return b.String()
	}

	b.WriteString("## Candidates\n\n")
	for i, c := range report.Candidates {
		b.WriteString(fmt.Sprintf("### %d. %s\n\n", i+1, c.Area))
		b.WriteString(fmt.Sprintf("- Source: `%s`\n", displayReferencePath(report.ReferenceRoot, c.Path)))
		if c.Section != "" {
			b.WriteString(fmt.Sprintf("- Section: `%s`\n", c.Section))
		}
		if c.TemplateTarget != "" {
			b.WriteString(fmt.Sprintf("- Template target: `%s`\n", c.TemplateTarget))
		}
		b.WriteString(fmt.Sprintf("- Portability: `%s`\n", c.Portability))
		b.WriteString(fmt.Sprintf("- Recommendation: `%s`\n", c.Disposition))
		b.WriteString(fmt.Sprintf("- Collision impact: `%s`\n", c.CollisionImpact))
		b.WriteString(fmt.Sprintf("- Reason: %s\n", c.Reason))
		if c.Summary != "" {
			b.WriteString(fmt.Sprintf("- Evidence: %s\n", c.Summary))
		}
		b.WriteString("\n")
	}
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

func proposeIfExists(op operation) operation {
	if _, err := os.Stat(op.path); err == nil {
		op.path = proposalPath(op.path)
		op.note = op.note + "; existing target preserved"
	}
	return op
}

func skipIfExists(op operation) operation {
	if _, err := os.Stat(op.path); err == nil {
		return operation{kind: "skip"}
	}
	return op
}

func proposalPath(path string) string {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	if ext == "" {
		return filepath.Join(dir, name+".template-proposed")
	}
	return filepath.Join(dir, name+".template-proposed"+ext)
}

func readAndRender(path string, placeholders map[string]string) (string, error) {
	content, err := os.ReadFile(path)
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
