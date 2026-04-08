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

var governedSectionNames = []string{
	"Purpose",
	"Governed Sections",
	"Interaction Mode",
	"Approval Boundaries",
	"Review Style",
	"File-Change Discipline",
	"Release Or Publish Triggers",
	"Documentation Update Expectations",
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
	Apply              bool
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
	apply              bool
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
	fs.BoolVar(&values.apply, "a", false, "write .template-proposed files for actionable candidates (enhance only)")
	fs.BoolVar(&values.apply, "apply", false, "write .template-proposed files for actionable candidates (enhance only)")
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
		Apply:              values.apply,
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
		return RunEnhance(root, cfg)
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
	if cfg.Apply && cfg.Mode != ModeEnhance {
		return errors.New("--apply is only valid with --mode enhance")
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

	canonical, err := planCanonical(root, cfg, targetAbs)
	if err != nil {
		return err
	}

	versionContent, _ := os.ReadFile(filepath.Join(root, "TEMPLATE_VERSION"))
	templateVersion := strings.TrimSpace(string(versionContent))
	manifest := buildManifest(canonical, templateVersion, root, targetAbs)
	manifestOp := operation{
		kind:    "write",
		path:    filepath.Join(targetAbs, manifestFileName),
		content: formatManifest(manifest),
		note:    "bootstrap manifest",
	}

	var ops []operation
	if adopt {
		ops = compactOperations(applyAdoptTransforms(canonical))
	} else {
		ops = compactOperations(canonical)
	}
	ops = append(ops, manifestOp)

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

// RunEnhance runs enhance mode against a reference repo. Exported for testing.
func RunEnhance(root string, cfg Config) error {
	refAbs, err := filepath.Abs(cfg.Reference)
	if err != nil {
		return fmt.Errorf("resolve reference path: %w", err)
	}
	report, err := ReviewEnhancement(root, refAbs)
	if err != nil {
		return err
	}
	printEnhancementSummary(report)

	selected, deferred, ok := selectActionableCandidates(report.Candidates)
	if !ok {
		fmt.Println("no actionable improvements found; no AC doc created")
		return nil
	}

	docsDir := filepath.Join(root, "docs")
	acNum, err := nextACNumber(docsDir)
	if err != nil {
		return err
	}
	slug := acSlug(selected)
	acFileName := fmt.Sprintf("ac-%03d-%s.md", acNum, slug)
	acPath := filepath.Join(docsDir, acFileName)
	acContent := renderACDoc(selected, deferred, report, acNum)

	if cfg.DryRun {
		fmt.Printf("dry-run write %s (enhancement AC doc)\n", acPath)
		if cfg.Apply {
			if err := applyProposals(root, selected, deferred, true); err != nil {
				return err
			}
		}
		fmt.Println("dry-run: no template changes applied")
		return nil
	}
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		return fmt.Errorf("create docs directory: %w", err)
	}
	if err := os.WriteFile(acPath, []byte(acContent), 0o644); err != nil {
		return fmt.Errorf("write AC doc: %w", err)
	}
	fmt.Printf("write %s (enhancement AC doc)\n", acPath)
	if cfg.Apply {
		if err := applyProposals(root, selected, deferred, false); err != nil {
			return err
		}
	}
	fmt.Println("enhance mode is review-first: no template changes applied")
	return nil
}

func applyProposals(templateRoot string, selected EnhancementCandidate, deferred []EnhancementCandidate, dryRun bool) error {
	candidates := []EnhancementCandidate{selected}
	for _, d := range deferred {
		if isActionable(d) {
			candidates = append(candidates, d)
		}
	}

	for _, c := range candidates {
		refContent, err := os.ReadFile(c.Path)
		if err != nil {
			return fmt.Errorf("read reference file %s: %w", c.Path, err)
		}

		targetPath := filepath.Join(templateRoot, c.TemplateTarget)
		proposal := proposalPath(targetPath)

		if dryRun {
			fmt.Printf("dry-run propose %s\n", proposal)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(proposal), 0o755); err != nil {
			return fmt.Errorf("create directory for proposal: %w", err)
		}
		if err := os.WriteFile(proposal, refContent, 0o644); err != nil {
			return fmt.Errorf("write proposal %s: %w", proposal, err)
		}
		fmt.Printf("propose %s\n", proposal)
	}
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
	manifest, hasManifest, err := readManifest(referenceRoot)
	if err != nil {
		return EnhancementReport{}, err
	}
	var mmap map[string]ManifestEntry
	if hasManifest {
		mmap = manifestEntryMap(manifest)
	}

	var candidates []EnhancementCandidate
	if governanceCandidates, err := reviewGovernedSections(templateRoot, referenceRoot, mmap); err != nil {
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
		candidate, ok, err := reviewMappedFile(templateRoot, referenceRoot, item, mmap)
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
	canonical, err := planCanonical(root, cfg, targetRoot)
	if err != nil {
		return nil, err
	}
	if !adopt {
		return compactOperations(canonical), nil
	}
	return compactOperations(applyAdoptTransforms(canonical)), nil
}

func planCanonical(root string, cfg Config, targetRoot string) ([]operation, error) {
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
		source:  filepath.Join("base", "AGENTS.md"),
	}}

	versionContent, err := os.ReadFile(filepath.Join(root, "TEMPLATE_VERSION"))
	if err != nil {
		return nil, fmt.Errorf("read template version: %w", err)
	}
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
		source: filepath.Join("base", "AGENTS.md"),
	})

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
		sourceRel, _ := filepath.Rel(root, path)
		ops = append(ops, operation{
			kind:    "write",
			path:    filepath.Join(targetRoot, targetRel),
			content: content,
			note:    "overlay file",
			source:  sourceRel,
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk overlay templates: %w", err)
	}

	return ops, nil
}

func applyAdoptTransforms(ops []operation) []operation {
	out := make([]operation, len(ops))
	for i, op := range ops {
		switch {
		case op.kind == "write" && op.note == "base governance contract":
			out[i] = patchOrProposeGovernance(op)
		case op.kind == "write" && op.note == "template version marker":
			out[i] = skipIfExists(op)
		case op.kind == "symlink":
			out[i] = skipIfExists(op)
		case op.kind == "write" && op.note == "overlay file":
			out[i] = proposeIfExists(op)
		default:
			out[i] = op
		}
	}
	return out
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
		return
	}
	counts := countEnhancementCandidates(report.Candidates)
	fmt.Printf("candidates: %d (accept=%d adapt=%d defer=%d reject=%d)\n",
		len(report.Candidates), counts["accept"], counts["adapt"], counts["defer"], counts["reject"])
	for _, c := range report.Candidates {
		fmt.Println(formatCandidateLine(c, report.ReferenceRoot))
	}
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

func reviewGovernedSections(templateRoot, referenceRoot string, mmap map[string]ManifestEntry) ([]EnhancementCandidate, error) {
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

	// Three-way pre-filter using manifest
	var sectionOrigin string
	if mmap != nil {
		if entry, ok := mmap["AGENTS.md"]; ok && entry.Kind == "file" {
			userChanged := computeChecksum(string(refContent)) != entry.Checksum
			templateChanged := false
			if entry.SourcePath != "" && entry.SourceChecksum != "" {
				sourceContent, readErr := os.ReadFile(filepath.Join(templateRoot, entry.SourcePath))
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
			ChangeOrigin:    sectionOrigin,
		})
	}
	return candidates, nil
}

func reviewMappedFile(templateRoot, referenceRoot string, item enhancementMapping, mmap map[string]ManifestEntry) (EnhancementCandidate, bool, error) {
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

	// Three-way comparison using manifest
	var changeOrigin string
	if mmap != nil {
		refRel, _ := filepath.Rel(referenceRoot, refPath)
		refRelSlash := filepath.ToSlash(refRel)
		if entry, ok := mmap[refRelSlash]; ok && entry.Kind == "file" {
			userChanged := computeChecksum(string(refContent)) != entry.Checksum
			templateChanged := false
			if entry.SourcePath != "" && entry.SourceChecksum != "" {
				sourceContent, readErr := os.ReadFile(filepath.Join(templateRoot, entry.SourcePath))
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
		return strings.Contains(content, "/Users/") || strings.Contains(content, "\\Users\\")
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

func nextACNumber(docsDir string) (int, error) {
	entries, err := os.ReadDir(docsDir)
	if err != nil {
		return 1, nil
	}
	max := 0
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "ac-") || !strings.HasSuffix(name, ".md") {
			continue
		}
		if name == "ac-template.md" {
			continue
		}
		trimmed := strings.TrimPrefix(name, "ac-")
		before, _, ok := strings.Cut(trimmed, "-")
		if !ok {
			continue
		}
		numStr := before
		num := 0
		for _, ch := range numStr {
			if ch < '0' || ch > '9' {
				num = -1
				break
			}
			num = num*10 + int(ch-'0')
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
	fmt.Fprintf(&b, "# AC-%03d %s\n\n", acNumber, title)

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

func patchOrProposeGovernance(op operation) operation {
	existingContent, err := os.ReadFile(op.path)
	if err != nil {
		return op // file doesn't exist, write directly
	}
	patched, changed := patchGovernedSections(string(existingContent), op.content)
	if !changed {
		return operation{kind: "skip"}
	}
	op.content = patched
	op.path = proposalPath(op.path)
	op.note = op.note + "; patched with missing governed sections"
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
