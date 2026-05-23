// Package governarm implements the `governa rm` subcommand.
package governarm

import (
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/queone/governa-color"
	"github.com/queone/governa/internal/emission"
	"github.com/queone/governa/internal/governance"
	"github.com/queone/governa/internal/templates"
)

const (
	ExitOK       = 0
	ExitEnvError = 1
	ExitUsage    = 2

	rmMarkerPrefix = "<!-- governa-rm: emitted-by governa "
)

var hybridPaths = map[string]bool{
	"AGENTS.md":    true,
	"README.md":    true,
	"CHANGELOG.md": true,
}

var expectedDivergence = map[string]bool{
	"plan.md": true,
	"arch.md": true,
}

type routing struct {
	Path   string
	Kind   string
	Diff   string
	Reason string
}

// RunCLI executes governa rm against the current consumer repo root.
func RunCLI(args []string, tfs fs.FS, out, errOut io.Writer) (int, error) {
	if helpRequested(args) {
		printUsage(errOut)
		return ExitOK, nil
	}
	fset := flag.NewFlagSet("governa rm", flag.ContinueOnError)
	fset.SetOutput(errOut)
	if err := fset.Parse(args); err != nil {
		printUsage(errOut)
		return ExitUsage, nil
	}
	if len(fset.Args()) > 0 {
		return ExitUsage, fmt.Errorf("governa rm: no positional arguments accepted; run from the consumer repo root (got: %v)", fset.Args())
	}
	cwd, err := os.Getwd()
	if err != nil {
		return ExitEnvError, fmt.Errorf("governa rm: get cwd: %w", err)
	}
	target, err := filepath.Abs(cwd)
	if err != nil {
		return ExitEnvError, fmt.Errorf("governa rm: resolve cwd: %w", err)
	}
	return Run(target, tfs, out)
}

// Run emits Governa-removal AC files for target.
func Run(target string, tfs fs.FS, out io.Writer) (int, error) {
	if err := emission.RefuseGovernaSource(target, "governa rm"); err != nil {
		return ExitEnvError, err
	}
	if err := emission.RequireGovernaAdopted(target, "governa rm"); err != nil {
		return ExitEnvError, err
	}
	if _, err := os.Stat(filepath.Join(target, ".git")); err != nil {
		return ExitEnvError, fmt.Errorf("governa rm: target %s is not a git worktree (no .git/) — governa rm needs git history to allocate the cleanup AC number", target)
	}

	flavor := "doc"
	if fileExists(filepath.Join(target, "go.mod")) {
		flavor = "code"
	}
	gcfg := governance.Config{
		Mode:     governance.ModeApply,
		Target:   target,
		RepoName: governance.InferRepoName(target),
	}
	if flavor == "code" {
		gcfg.Type = governance.RepoTypeCode
		gcfg.Stack = governance.InferStack(target)
		if gcfg.Stack == "" {
			gcfg.Stack = "Go"
		}
	} else {
		gcfg.Type = governance.RepoTypeDoc
	}
	canon, err := governance.RenderCanonicalFiles(tfs, gcfg, target)
	if err != nil {
		return ExitEnvError, fmt.Errorf("governa rm: render canon: %w", err)
	}

	canonVersion := "v" + templates.TemplateVersion
	acNum, reused, err := emission.AllocateACNumber(target, "governa-rm", canonVersion)
	if err != nil {
		return ExitEnvError, fmt.Errorf("governa rm: allocate AC number: %w", err)
	}
	stubRel := fmt.Sprintf("governa/ac%d-governa-rm-%s.md", acNum, canonVersion)
	diffsRel := fmt.Sprintf("governa/ac%d-governa-rm-%s-diffs.md", acNum, canonVersion)
	stubPath := filepath.Join(target, stubRel)
	diffsPath := filepath.Join(target, diffsRel)
	if reused {
		for _, path := range []string{stubPath, diffsPath} {
			if _, err := os.Stat(path); err == nil {
				unedited, verifyErr := emission.VerifyUnedited(path, rmMarkerPrefix)
				if verifyErr != nil {
					return ExitEnvError, fmt.Errorf("governa rm: verify %s: %w", path, verifyErr)
				}
				if !unedited {
					rel, _ := filepath.Rel(target, path)
					return ExitEnvError, fmt.Errorf("governa rm: %s has been edited since last governa-rm emission — delete or rename the emitted files before re-running", rel)
				}
			}
		}
	}

	inScope, outOfScope, review := classify(target, canon)
	stubBody := buildStub(acNum, canonVersion, diffsRel, inScope, outOfScope, review)
	diffsBody := buildDiffs(acNum, canonVersion, review)
	if err := emission.EnsureDocsDir(target, "governa rm"); err != nil {
		return ExitEnvError, err
	}
	if err := emission.WriteWithMarker(stubPath, rmMarkerPrefix, canonVersion, stubBody); err != nil {
		return ExitEnvError, fmt.Errorf("governa rm: write %s: %w", stubRel, err)
	}
	if err := emission.WriteWithMarker(diffsPath, rmMarkerPrefix, canonVersion, diffsBody); err != nil {
		return ExitEnvError, fmt.Errorf("governa rm: write %s: %w", diffsRel, err)
	}
	fmt.Fprintf(out, "wrote %s and %s\n", stubRel, diffsRel)
	return ExitOK, nil
}

func classify(target string, canon map[string]string) (inScope, outOfScope, review []routing) {
	for _, rel := range sortedKeys(canon) {
		targetPath := filepath.Join(target, rel)
		content, err := os.ReadFile(targetPath)
		if err != nil {
			continue
		}
		if expectedDivergence[rel] {
			outOfScope = append(outOfScope, routing{Path: rel, Kind: "keep", Reason: "repo-owned Governa-adjacent content"})
			continue
		}
		if markers := emission.PreserveMarkers(target, rel); len(markers) > 0 {
			outOfScope = append(outOfScope, routing{Path: rel, Kind: "keep", Reason: "preserve marker: " + strings.Join(markers, " | ")})
			continue
		}
		if hybridPaths[rel] {
			review = append(review, routing{Path: rel, Kind: "hybrid", Reason: "mixed canon-shape and consumer content", Diff: unifiedDiff(canon[rel], string(content), rel)})
			continue
		}
		if string(content) == canon[rel] {
			inScope = append(inScope, routing{Path: rel, Kind: "delete file", Reason: "byte-equal Governa canon"})
			continue
		}
		review = append(review, routing{Path: rel, Kind: "ambiguity", Reason: "consumer-edited canon file", Diff: unifiedDiff(canon[rel], string(content), rel)})
	}
	if info, err := os.Lstat(filepath.Join(target, "CLAUDE.md")); err == nil && info.Mode()&os.ModeSymlink != 0 {
		inScope = append(inScope, routing{Path: "CLAUDE.md", Kind: "delete symlink", Reason: "Governa compatibility link"})
	}
	outOfScope = append(outOfScope, targetOnlyRoutes(target, canon)...)
	return inScope, outOfScope, review
}

func targetOnlyRoutes(target string, canon map[string]string) []routing {
	var routes []routing
	_ = filepath.WalkDir(target, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		rel, err := filepath.Rel(target, path)
		if err != nil || rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			if rel == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if rel == "CLAUDE.md" {
			return nil
		}
		if _, ok := canon[rel]; ok {
			return nil
		}
		routes = append(routes, routing{Path: rel, Kind: "keep", Reason: "target-only repo-owned file"})
		return nil
	})
	sort.Slice(routes, func(i, j int) bool { return routes[i].Path < routes[j].Path })
	return routes
}

func buildStub(acNum int, canonVersion, diffsRel string, inScope, outOfScope, review []routing) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# AC%d Governa Removal from %s\n\n", acNum, canonVersion)
	fmt.Fprintln(&b, "Remove Governa canon from this repo through a Director-reviewed cleanup pass.")
	fmt.Fprint(&b, "\n## Summary\n\n")
	fmt.Fprintf(&b, "Extricate Governa canon from this consumer repo without deleting consumer-owned content. Emitted by `governa rm` against canon %s. Implement only after the Director resolves the routing decisions below.\n", canonVersion)
	fmt.Fprintf(&b, "\nUse `%s` for hunk-level guidance on hybrid files. Do not auto-delete routing-pending files until the Director chooses their routing.\n", diffsRel)
	fmt.Fprint(&b, "\n### Routing Decisions\n\n")
	if len(review) == 0 {
		fmt.Fprintln(&b, "`None` — no review items.")
	} else {
		for i, route := range review {
			fmt.Fprintf(&b, "%d. `%s` is %s. Choose: delete canon-shape only, keep entirely, or delete entirely? See [%s](%s#%s).\n", i+1, route.Path, route.Reason, route.Path, diffsRel, anchor(route.Path))
		}
	}
	fmt.Fprint(&b, "\n## In Scope\n\n")
	writeRoutingList(&b, inScope)
	fmt.Fprint(&b, "\n## Out Of Scope\n\n")
	writeRoutingList(&b, outOfScope)
	fmt.Fprint(&b, "\n## Acceptance Tests\n\n")
	fmt.Fprintln(&b, "**AT1** [Automated] [Pre-release gate] — Removed files listed under `## In Scope` no longer exist.")
	fmt.Fprintln(&b, "**AT2** [Manual] [Pre-release gate] — Director confirms every routing-pending file under `### Routing Decisions` was routed exactly as decided.")
	fmt.Fprint(&b, "\n## Status\n\n")
	fmt.Fprintln(&b, "`PENDING` — Emitted by `governa rm`; awaiting Director review.")
	return b.String()
}

func buildDiffs(acNum int, canonVersion string, review []routing) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# AC%d Governa Removal Diffs from %s\n\n", acNum, canonVersion)
	if len(review) == 0 {
		fmt.Fprintln(&b, "No hybrid or ambiguity-classified files require hunk-level guidance.")
		return b.String()
	}
	for _, route := range review {
		fmt.Fprintf(&b, "## `%s`\n\n", route.Path)
		fmt.Fprintf(&b, "Reason: %s.\n\n", route.Reason)
		fmt.Fprintln(&b, "```diff")
		fmt.Fprintln(&b, route.Diff)
		fmt.Fprintln(&b, "```")
		fmt.Fprintln(&b)
	}
	return b.String()
}

func writeRoutingList(b *strings.Builder, routes []routing) {
	if len(routes) == 0 {
		fmt.Fprintln(b, "- None.")
		return
	}
	for _, route := range routes {
		fmt.Fprintf(b, "- `%s` — %s; %s.\n", route.Path, route.Kind, route.Reason)
	}
}

func unifiedDiff(canon, target, rel string) string {
	if _, err := exec.LookPath("diff"); err != nil {
		return fmt.Sprintf("[diff unavailable: %s]", err)
	}
	left, err := os.CreateTemp("", "governa-rm-canon-")
	if err != nil {
		return fmt.Sprintf("[diff failed: %s]", err)
	}
	defer os.Remove(left.Name())
	right, err := os.CreateTemp("", "governa-rm-target-")
	if err != nil {
		return fmt.Sprintf("[diff failed: %s]", err)
	}
	defer os.Remove(right.Name())
	_, _ = left.WriteString(canon)
	_, _ = right.WriteString(target)
	_ = left.Close()
	_ = right.Close()
	cmd := exec.Command("diff", "-u", "-L", "canon/"+rel, "-L", "target/"+rel, left.Name(), right.Name())
	out, runErr := cmd.CombinedOutput()
	if exitErr, ok := runErr.(*exec.ExitError); ok && exitErr.ExitCode() >= 2 {
		return fmt.Sprintf("[diff failed: %s]", strings.TrimSpace(string(out)))
	}
	return strings.TrimRight(string(out), "\n")
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func anchor(path string) string {
	replacer := strings.NewReplacer("`", "", "/", "", ".", "", " ", "-")
	return strings.ToLower(replacer.Replace(path))
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func helpRequested(args []string) bool {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" || arg == "-?" {
			return true
		}
	}
	return false
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, color.FormatUsage("governa rm", nil, "Emit a Director-reviewed cleanup AC for removing Governa canon from an adopted repo."))
}
