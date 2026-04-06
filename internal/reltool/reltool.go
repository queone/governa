package reltool

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"

	"repo-governance-template/internal/color"
)

var semverTagPattern = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)

type Config struct {
	Tag     string
	Message string
}

func ParseArgs(args []string) (Config, bool, error) {
	if len(args) == 0 {
		return Config{}, true, nil
	}
	if len(args) == 1 && IsHelpArg(args[0]) {
		return Config{}, true, nil
	}
	for _, arg := range args {
		if IsHelpArg(arg) {
			return Config{}, false, errors.New("help flags must be used by themselves")
		}
		if strings.HasPrefix(arg, "-") {
			return Config{}, false, fmt.Errorf("unsupported option %q; use positional args or -h, -?, --help", arg)
		}
	}
	if len(args) != 2 {
		return Config{}, false, errors.New("usage: rel vX.Y.Z \"release message\"")
	}

	cfg := Config{
		Tag:     strings.TrimSpace(args[0]),
		Message: strings.TrimSpace(args[1]),
	}
	if !semverTagPattern.MatchString(cfg.Tag) {
		return Config{}, false, fmt.Errorf("release tag must match vMAJOR.MINOR.PATCH: %q", cfg.Tag)
	}
	if cfg.Message == "" {
		return Config{}, false, errors.New("release message must be non-empty")
	}
	return cfg, false, nil
}

func IsHelpArg(arg string) bool {
	return arg == "-h" || arg == "-?" || arg == "--help"
}

func Usage() string {
	return "usage: rel vX.Y.Z \"release message\"\n\nOptions:\n  -h, -?, --help   Show this help\n"
}

func Run(cfg Config, in io.Reader, out io.Writer, errOut io.Writer) error {
	if err := ensureGitRepo(); err != nil {
		return err
	}

	fmt.Fprintf(out, "%s %s\n", color.Yel("release tag:"), color.Grn(cfg.Tag))
	fmt.Fprintf(out, "%s %s\n", color.Yel("release message:"), color.Grn(fmt.Sprintf("%q", cfg.Message)))
	fmt.Fprintf(out, "%s %s\n", color.Yel("remote:"), color.Cya("origin"))

	fmt.Fprintln(out, color.Yel("\nFiles that will be staged (git status):"))
	if err := runGit(out, errOut, "git status preview", "status", "--short"); err != nil {
		return err
	}

	fmt.Fprintln(out, color.Yel("\nplan:"))
	fmt.Fprintln(out, "- git add .")
	fmt.Fprintf(out, "- git commit -m %q\n", cfg.Message)
	fmt.Fprintf(out, "- git tag %s\n", cfg.Tag)
	fmt.Fprintf(out, "- git push origin %s\n", cfg.Tag)
	fmt.Fprintln(out, "- git push origin")

	ok, err := confirm(in, out, color.Yel("Review the file list above. Proceed with release? (y/N): "))
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
		{name: "git commit", args: []string{"commit", "-m", cfg.Message}},
		{name: "git tag", args: []string{"tag", cfg.Tag}},
		{name: "git push tag", args: []string{"push", "origin", cfg.Tag}},
		{name: "git push branch", args: []string{"push", "origin"}},
	}
	for _, step := range steps {
		if err := runGit(out, errOut, step.name, step.args...); err != nil {
			return err
		}
	}
	return nil
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

func confirm(in io.Reader, out io.Writer, prompt string) (bool, error) {
	fmt.Fprint(out, prompt)
	reader := bufio.NewReader(in)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("read confirmation: %w", err)
	}
	value := strings.TrimSpace(line)
	return value == "y" || value == "Y", nil
}

func runGit(out io.Writer, errOut io.Writer, name string, args ...string) error {
	fmt.Fprintf(out, "%s %s\n", color.Yel("running:"), color.Grn("git "+strings.Join(args, " ")))
	cmd := exec.Command("git", args...)
	cmd.Stdout = out
	cmd.Stderr = errOut
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s failed: %w", name, err)
	}
	return nil
}
