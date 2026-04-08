package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

var semverTagPattern = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)

type relConfig struct {
	tag     string
	message string
}

func main() {
	cfg, help, err := parseRelArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if help {
		printRelUsage(os.Stdout)
		return
	}
	if err := runRelease(cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func parseRelArgs(args []string) (relConfig, bool, error) {
	if len(args) == 0 {
		return relConfig{}, true, nil
	}
	if len(args) == 1 && isHelpArg(args[0]) {
		return relConfig{}, true, nil
	}
	for _, arg := range args {
		if isHelpArg(arg) {
			return relConfig{}, false, errors.New("help flags must be used by themselves")
		}
		if strings.HasPrefix(arg, "-") {
			return relConfig{}, false, fmt.Errorf("unsupported option %q; use positional args or -h, -?, --help", arg)
		}
	}
	if len(args) != 2 {
		return relConfig{}, false, errors.New("usage: rel vX.Y.Z \"release message\"")
	}

	cfg := relConfig{
		tag:     strings.TrimSpace(args[0]),
		message: strings.TrimSpace(args[1]),
	}
	if !semverTagPattern.MatchString(cfg.tag) {
		return relConfig{}, false, fmt.Errorf("release tag must match vMAJOR.MINOR.PATCH: %q", cfg.tag)
	}
	if cfg.message == "" {
		return relConfig{}, false, errors.New("release message must be non-empty")
	}
	if len(cfg.message) > 80 {
		return relConfig{}, false, errors.New("release message must be 80 characters or fewer")
	}
	return cfg, false, nil
}

func runRelease(cfg relConfig) error {
	if err := ensureGitRepo(); err != nil {
		return err
	}

	fmt.Printf("%s %s\n", yel("release tag:"), grn(cfg.tag))
	fmt.Printf("%s %s\n", yel("release message:"), grn(fmt.Sprintf("%q", cfg.message)))
	fmt.Printf("%s %s\n", yel("remote:"), cya("origin"))

	fmt.Println(yel("\nFiles that will be staged (git status):"))
	if err := runGit("git status preview", "status", "--short"); err != nil {
		return err
	}

	fmt.Println(yel("\nplan:"))
	fmt.Printf("- git add .\n")
	fmt.Printf("- git commit -m %q\n", cfg.message)
	fmt.Printf("- git tag %s\n", cfg.tag)
	fmt.Printf("- git push origin %s\n", cfg.tag)
	fmt.Printf("- git push origin\n")

	ok, err := confirm(yel("Review the file list above. Proceed with release? (y/N): "))
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("release aborted")
	}

	steps := []struct {
		name string
		args []string
	}{
		{name: "git add", args: []string{"add", "."}},
		{name: "git commit", args: []string{"commit", "-m", cfg.message}},
		{name: "git tag", args: []string{"tag", cfg.tag}},
		{name: "git push tag", args: []string{"push", "origin", cfg.tag}},
		{name: "git push branch", args: []string{"push", "origin"}},
	}
	for _, step := range steps {
		if err := runGit(step.name, step.args...); err != nil {
			return err
		}
	}
	return nil
}

func isHelpArg(arg string) bool {
	return arg == "-h" || arg == "-?" || arg == "--help"
}

func printRelUsage(w *os.File) {
	fmt.Fprintln(w, "usage: rel vX.Y.Z \"release message\"")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Options:")
	fmt.Fprintln(w, "  -h, -?, --help   Show this help")
}

func ensureGitRepo() error {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("verify git repo: %w: %s", err, strings.TrimSpace(string(output)))
	}
	if strings.TrimSpace(string(output)) != "true" {
		return errors.New("current directory is not inside a git work tree")
	}
	return nil
}

func confirm(prompt string) (bool, error) {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("read confirmation: %w", err)
	}
	value := strings.TrimSpace(line)
	return value == "y" || value == "Y", nil
}

func runGit(name string, args ...string) error {
	fmt.Printf("%s %s\n", yel("running:"), grn("git "+strings.Join(args, " ")))
	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s failed: %w", name, err)
	}
	return nil
}
