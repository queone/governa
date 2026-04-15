// build is based on an original build.sh Bash script from the source project
// that inspired this template.
package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"github.com/queone/governa/examples/code/internal/color"
)

type buildConfig struct {
	verbose bool
	targets []string
}

type semver struct {
	major int
	minor int
	patch int
}

// build, bootstrap, and rel are intentionally treated as go-run entrypoints
// rather than installed binaries for now. That may change in the future.
var scriptOnlyCommands = map[string]struct{}{
	"build":     {},
	"bootstrap": {},
	"rel":       {},
}

var programVersionInlineRe = regexp.MustCompile(`(?m)^\s*const\s+programVersion\s*(?:string\s*)?=\s*"([^"]*)"`)
var programVersionBlockRe = regexp.MustCompile(`(?s)const\s*\(.*?programVersion\s*(?:string\s*)?=\s*"([^"]*)".*?\)`)

func main() {
	cfg, help, err := parseBuildArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if help {
		printBuildUsage()
		return
	}
	if err := runBuild(cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func parseBuildArgs(args []string) (buildConfig, bool, error) {
	if len(args) == 1 && isBuildHelpArg(args[0]) {
		return buildConfig{}, true, nil
	}
	cfg := buildConfig{}
	for _, arg := range args {
		switch arg {
		case "-v", "--verbose":
			cfg.verbose = true
		case "-h", "-?", "--help":
			return buildConfig{}, false, errors.New("help flags must be used by themselves")
		default:
			if strings.HasPrefix(arg, "-") {
				return buildConfig{}, false, fmt.Errorf("unsupported option %q; use target names plus optional -v, --verbose", arg)
			}
			cfg.targets = append(cfg.targets, arg)
		}
	}
	return cfg, false, nil
}

func printBuildUsage() {
	fmt.Println("usage: build [target ...] [-v|--verbose]")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -h, -?, --help      Show this help")
	fmt.Println("  -v, --verbose       Run go test in verbose mode")
}

func isBuildHelpArg(arg string) bool {
	return arg == "-h" || arg == "-?" || arg == "--help"
}

func runBuild(cfg buildConfig) error {
	modulePath, err := modulePath()
	if err != nil {
		return err
	}
	binDir, err := goBinDir()
	if err != nil {
		return err
	}
	ext := binaryExt()
	scopes := packageScopes(cfg.targets)

	fmt.Println(color.Yel("==> Update go.mod to reflect actual dependencies"))
	if err := runStreaming("go", "mod", "tidy"); err != nil {
		return err
	}

	fmt.Println("\n" + color.Yel("==> Format Go code according to standard rules"))
	if output := runCapturedSoft("go", append([]string{"fmt"}, scopes...)...); strings.TrimSpace(output) == "" {
		fmt.Println("    No formatting changes needed.")
	} else {
		writeIndented(output)
	}

	fmt.Println("\n" + color.Yel("==> Automatically fix Go code for API/language changes"))
	if output := runCapturedSoft("go", append([]string{"fix"}, scopes...)...); strings.TrimSpace(output) == "" {
		fmt.Println("    No fixes applied.")
	} else {
		writeIndented(output)
	}

	fmt.Println("\n" + color.Yel("==> Check code for potential issues"))
	if output, failed := runCapturedCheck("go", append([]string{"vet"}, scopes...)...); failed {
		writeIndented(output)
		return fmt.Errorf("go vet found issues")
	} else if trimmed := strings.TrimSpace(output); trimmed != "" {
		writeIndented(output)
	} else {
		fmt.Println("    No issues found by go vet.")
	}

	coverFile, err := os.CreateTemp("", "build-cover-*.out")
	if err != nil {
		return fmt.Errorf("create coverage file: %w", err)
	}
	coverPath := coverFile.Name()
	coverFile.Close()
	defer os.Remove(coverPath)

	fmt.Println("\n" + color.Yel("==> Run tests for all packages in the repository"))
	testArgs := []string{"test"}
	if cfg.verbose {
		testArgs = append(testArgs, "-v")
	}
	testArgs = append(testArgs, "-coverprofile="+coverPath)
	testArgs = append(testArgs, scopes...)
	if err := runStreaming("go", testArgs...); err != nil {
		return err
	}
	if err := printCoverageSummary(coverPath, modulePath); err != nil {
		return err
	}

	fmt.Println("\n" + color.Yel("==> Ensure staticcheck is available"))
	staticcheckPath, err := ensureStaticcheck()
	if err != nil {
		return err
	}

	fmt.Println("\n" + color.Yel("==> Analyze Go code for potential issues"))
	if output, failed := runCapturedCheck(staticcheckPath, scopes...); failed {
		writeIndented(output)
		return fmt.Errorf("staticcheck found issues")
	} else if trimmed := strings.TrimSpace(output); trimmed != "" {
		writeIndented(output)
	} else {
		fmt.Println("    No issues found by staticcheck.")
	}

	targets, err := buildTargets(cfg.targets)
	if err != nil {
		return err
	}
	if len(cfg.targets) == 0 {
		fmt.Println("\n" + color.Yel("==> Building all utilities"))
	} else {
		fmt.Printf("\n%s %s\n", color.Yel("==> Building specific utilities:"), color.Grn(strings.Join(cfg.targets, " ")))
	}
	if shouldSkipBinaryInstall(cfg.targets) {
		fmt.Printf("    %s %s\n", color.Yel("Skipping binary install for"), color.Cya(joinScriptOnlyTargets(cfg.targets)+"; run them with go run for now."))
	}
	if len(targets) > 0 {
		fmt.Println("\n" + color.Yel("==> Validate programVersion declarations"))
		if err := validateProgramVersions(targets); err != nil {
			return err
		}
	}
	for _, target := range targets {
		outputPath := filepath.Join(binDir, target+ext)
		fmt.Printf("\n%s %s\n", color.Yel("==> Building and installing"), color.Grn(target))
		if err := runStreaming("go", "build", "-o", outputPath, "-ldflags", "-s -w", "./cmd/"+target); err != nil {
			return err
		}
		fmt.Printf("    installed: %s\n", color.Cya(outputPath))
	}

	if nextTag, ok, err := nextPatchTag(); err != nil {
		return err
	} else if ok {
		fmt.Printf("\n%s\n\n    go run ./cmd/rel %s %s\n", color.Yel("==> To release, run:"), color.Grn(nextTag), color.Gra("\"<release message>\""))
	}
	return nil
}

func modulePath() (string, error) {
	output, err := runCaptured("go", "list", "-m", "-f", "{{.Path}}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}

func goBinDir() (string, error) {
	output, err := runCaptured("go", "env", "GOPATH")
	if err != nil {
		return "", err
	}
	gopath := strings.TrimSpace(output)
	if gopath == "" {
		return "", errors.New("go env GOPATH returned an empty value")
	}
	return filepath.Join(gopath, "bin"), nil
}

func binaryExt() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

func packageScopes(targets []string) []string {
	if len(targets) == 0 {
		return []string{"./..."}
	}
	out := make([]string, 0, len(targets))
	for _, target := range targets {
		out = append(out, "./cmd/"+target)
	}
	return out
}

func buildTargets(targets []string) ([]string, error) {
	if len(targets) > 0 {
		return filterInstallTargets(targets), nil
	}
	entries, err := os.ReadDir("cmd")
	if err != nil {
		return nil, fmt.Errorf("read ./cmd: %w", err)
	}
	var out []string
	for _, entry := range entries {
		if entry.IsDir() {
			out = append(out, entry.Name())
		}
	}
	return filterInstallTargets(out), nil
}

func filterInstallTargets(targets []string) []string {
	out := make([]string, 0, len(targets))
	for _, target := range targets {
		if _, skip := scriptOnlyCommands[target]; skip {
			continue
		}
		out = append(out, target)
	}
	slices.Sort(out)
	return out
}

func shouldSkipBinaryInstall(requested []string) bool {
	if len(requested) == 0 {
		return true
	}
	for _, target := range requested {
		if _, skip := scriptOnlyCommands[target]; skip {
			return true
		}
	}
	return false
}

func joinScriptOnlyTargets(requested []string) string {
	var names []string
	if len(requested) == 0 {
		for name := range scriptOnlyCommands {
			names = append(names, "cmd/"+name)
		}
	} else {
		for _, target := range requested {
			if _, skip := scriptOnlyCommands[target]; skip {
				names = append(names, "cmd/"+target)
			}
		}
	}
	slices.Sort(names)
	return strings.Join(names, ", ")
}

func extractProgramVersion(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	if match := programVersionInlineRe.FindSubmatch(content); match != nil {
		return string(match[1]), nil
	}
	if match := programVersionBlockRe.FindSubmatch(content); match != nil {
		return string(match[1]), nil
	}
	return "", nil
}

func validateProgramVersions(targets []string) error {
	for _, target := range targets {
		mainPath := filepath.Join("cmd", target, "main.go")
		ver, err := extractProgramVersion(mainPath)
		if err != nil {
			return err
		}
		if ver == "" {
			return fmt.Errorf("cmd/%s/main.go must declare a non-empty const programVersion string literal", target)
		}
		fmt.Printf("    %s: programVersion = %s\n", color.Cya("cmd/"+target), color.Grn(fmt.Sprintf("%q", ver)))
	}
	return nil
}

func ensureStaticcheck() (string, error) {
	if path, err := exec.LookPath("staticcheck"); err == nil {
		fmt.Printf("    found: %s\n", color.Cya(path))
		return path, nil
	}
	fmt.Printf("    installing: %s\n", color.Grn("honnef.co/go/tools/cmd/staticcheck@latest"))
	if err := runStreaming("go", "install", "honnef.co/go/tools/cmd/staticcheck@latest"); err != nil {
		return "", err
	}
	if path, err := exec.LookPath("staticcheck"); err == nil {
		return path, nil
	}
	binDir, err := goBinDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(binDir, "staticcheck"+binaryExt())
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("staticcheck not found after installation: %w", err)
	}
	return path, nil
}

func printCoverageSummary(coverPath, modulePath string) error {
	output, err := runCaptured("go", "tool", "cover", "-func="+coverPath)
	if err != nil {
		return err
	}
	var total string
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "total:") {
			fields := strings.Fields(line)
			if len(fields) > 0 {
				total = fields[len(fields)-1]
			}
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan coverage output: %w", err)
	}
	if total == "" {
		return nil
	}
	domainPct, err := domainCoverage(coverPath, modulePath+"/internal/")
	if err != nil {
		return err
	}
	coverageText := fmt.Sprintf("domain coverage: %.1f%%", domainPct)
	styledCoverage := color.Red(coverageText)
	switch {
	case domainPct >= 75:
		styledCoverage = color.Grn(coverageText)
	case domainPct >= 50:
		styledCoverage = color.Yel(coverageText)
	}
	fmt.Printf("    %s  %s\n", styledCoverage, color.Gra("(total: "+total+")"))
	return nil
}

func domainCoverage(coverPath, prefix string) (float64, error) {
	content, err := os.ReadFile(coverPath)
	if err != nil {
		return 0, fmt.Errorf("read coverage profile: %w", err)
	}
	var totalStatements int
	var coveredStatements int
	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "mode:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 3 {
			continue
		}
		if !strings.HasPrefix(fields[0], prefix) {
			continue
		}
		statements, err := strconv.Atoi(fields[1])
		if err != nil {
			return 0, fmt.Errorf("parse coverage statements from %q: %w", line, err)
		}
		count, err := strconv.Atoi(fields[2])
		if err != nil {
			return 0, fmt.Errorf("parse coverage count from %q: %w", line, err)
		}
		totalStatements += statements
		if count > 0 {
			coveredStatements += statements
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("scan coverage profile: %w", err)
	}
	if totalStatements == 0 {
		return 0, nil
	}
	return float64(coveredStatements) / float64(totalStatements) * 100, nil
}

func nextPatchTag() (string, bool, error) {
	output, err := runCaptured("git", "tag", "--list")
	if err != nil {
		return "", false, err
	}
	var versions []semver
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "v") {
			continue
		}
		parts := strings.Split(strings.TrimPrefix(line, "v"), ".")
		if len(parts) != 3 {
			continue
		}
		major, err1 := strconv.Atoi(parts[0])
		minor, err2 := strconv.Atoi(parts[1])
		patch, err3 := strconv.Atoi(parts[2])
		if err1 != nil || err2 != nil || err3 != nil {
			continue
		}
		versions = append(versions, semver{major: major, minor: minor, patch: patch})
	}
	if err := scanner.Err(); err != nil {
		return "", false, fmt.Errorf("scan git tags: %w", err)
	}
	if len(versions) == 0 {
		return "", false, nil
	}
	slices.SortFunc(versions, func(a, b semver) int {
		if a.major != b.major {
			return a.major - b.major
		}
		if a.minor != b.minor {
			return a.minor - b.minor
		}
		return a.patch - b.patch
	})
	last := versions[len(versions)-1]
	return fmt.Sprintf("v%d.%d.%d", last.major, last.minor, last.patch+1), true, nil
}

func runCaptured(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s %s failed: %w\n%s", name, strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

func runCapturedSoft(name string, args ...string) string {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil && strings.TrimSpace(string(output)) == "" {
		return err.Error()
	}
	return string(output)
}

func runCapturedCheck(name string, args ...string) (string, bool) {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	return string(output), err != nil
}

func runStreaming(name string, args ...string) error {
	command := strings.TrimSpace(name + " " + strings.Join(args, " "))
	fmt.Printf("    %s\n", color.Grn(command))
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s failed: %w", name, strings.Join(args, " "), err)
	}
	return nil
}

func writeIndented(text string) {
	scanner := bufio.NewScanner(strings.NewReader(strings.TrimSpace(text)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "FAIL") {
			line = color.Red(line)
		}
		fmt.Printf("    %s\n", line)
	}
}
