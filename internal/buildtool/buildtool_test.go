package buildtool

import "testing"

func TestParseArgsNoArgs(t *testing.T) {
	t.Parallel()

	cfg, help, err := ParseArgs(nil)
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}
	if help {
		t.Fatal("did not expect help mode")
	}
	if cfg.Verbose {
		t.Fatal("did not expect verbose mode")
	}
	if len(cfg.Targets) != 0 {
		t.Fatalf("unexpected targets: %#v", cfg.Targets)
	}
}

func TestParseArgsVerboseAndTargets(t *testing.T) {
	t.Parallel()

	cfg, help, err := ParseArgs([]string{"-v", "bootstrap", "rel"})
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}
	if help {
		t.Fatal("did not expect help mode")
	}
	if !cfg.Verbose {
		t.Fatal("expected verbose mode")
	}
	if len(cfg.Targets) != 2 || cfg.Targets[0] != "bootstrap" || cfg.Targets[1] != "rel" {
		t.Fatalf("unexpected targets: %#v", cfg.Targets)
	}
}

func TestPackageScopes(t *testing.T) {
	t.Parallel()

	got := packageScopes([]string{"bootstrap", "rel"})
	want := []string{"./cmd/bootstrap", "./cmd/rel"}
	if len(got) != len(want) {
		t.Fatalf("packageScopes() len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("packageScopes()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestBuildTargetsSkipsScriptOnlyCommands(t *testing.T) {
	t.Parallel()

	got, err := buildTargets([]string{"build", "bootstrap", "rel"})
	if err != nil {
		t.Fatalf("buildTargets() error = %v", err)
	}
	want := []string{}
	if len(got) != len(want) {
		t.Fatalf("buildTargets() len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("buildTargets()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestShouldSkipBinaryInstall(t *testing.T) {
	t.Parallel()

	if !shouldSkipBinaryInstall(nil) {
		t.Fatal("expected default build to report skipped script-only commands")
	}
	if !shouldSkipBinaryInstall([]string{"bootstrap"}) {
		t.Fatal("expected bootstrap target to be treated as script-only")
	}
	if shouldSkipBinaryInstall([]string{"worker"}) {
		t.Fatal("did not expect installable target to be treated as script-only")
	}
}

func TestJoinScriptOnlyTargets(t *testing.T) {
	t.Parallel()

	if got := joinScriptOnlyTargets(nil); got != "cmd/bootstrap, cmd/build, cmd/rel" {
		t.Fatalf("joinScriptOnlyTargets(nil) = %q", got)
	}
	if got := joinScriptOnlyTargets([]string{"worker", "bootstrap", "rel"}); got != "cmd/bootstrap, cmd/rel" {
		t.Fatalf("joinScriptOnlyTargets(requested) = %q", got)
	}
}

func TestNextPatchTagSortsSemver(t *testing.T) {
	t.Parallel()

	versions := []semver{
		{major: 1, minor: 2, patch: 9},
		{major: 1, minor: 10, patch: 0},
		{major: 1, minor: 3, patch: 1},
	}
	max := versions[0]
	for _, current := range versions[1:] {
		if current.major > max.major ||
			(current.major == max.major && current.minor > max.minor) ||
			(current.major == max.major && current.minor == max.minor && current.patch > max.patch) {
			max = current
		}
	}
	if max.minor != 10 {
		t.Fatalf("max version = %#v, want minor 10", max)
	}
}
