# internal/color — cross-repo propagation

**Canonical source:** this directory (`internal/color/` in the governa repo)
**Consumer repos:** iq, skout, utils

---

## How it works

Four repos share an identical `internal/color` package. Governa is the
canonical source. The other three repos each carry a copy of the package;
when governa's copy changes, each consumer repo updates by copying the
canonical files over its local copy.

Each consumer repo is self-contained — nothing in iq, skout, or utils
references governa at runtime. An agent invoked in a consumer repo
compares against governa's copy and updates when needed.

### For an agent invoked in a consumer repo (iq, skout, utils)

You are responsible for **your own repo only**. Never modify governa or
any repo other than the one you were invoked in. Repos are assumed to
live as siblings under a common parent directory, so governa is reachable
at `../governa/` from your repo's working directory.

1. **Compare your local `internal/color/` against the canonical copy.**
   ```bash
   diff -q internal/color/color.go      ../governa/internal/color/color.go
   diff -q internal/color/color_test.go ../governa/internal/color/color_test.go
   ```
   If both files are identical, no update is needed — stop.
2. **Overwrite your local copies with the canonical ones.**
   ```bash
   cp ../governa/internal/color/color.go      internal/color/color.go
   cp ../governa/internal/color/color_test.go internal/color/color_test.go
   ```
3. **Run your repo's canonical build script.** (`./build.sh` or equivalent) Verify the full pipeline passes.
4. **Commit the updated files in your repo** with a message that identifies the update, e.g. `update internal/color from governa`.

### If your repo is ahead of governa

If the `diff` step reveals that your local `internal/color/` contains
functions, fixes, or test coverage **not present in governa's canonical
copy**, do not run the `cp` step — overwriting would lose work that
governa's maintainer has not seen yet.

Instead, ask the user to share the differences with governa's maintainer
so the canonical copy can absorb the improvement. Once the improvement
lands in governa, run a normal update.

### Making changes to the package

All enhancements and fixes land in governa first. Once verified there,
consumer repos pick up the change the next time an agent runs the steps
above. Do not make repo-local color changes — they will diverge and be
overwritten on the next update.

---

## Standing notes

**Package surface:**
`Gra`, `Grn`, `GrnR`, `GrnD`, `Yel`, `Blu`, `Cya`, `Red`, `RedR`, `RedD`,
`Whi`, `Whi2`, `BoldW`, `ShowPalette`, `FormatUsage`, `UsageLine`

All color functions share the same `func(any) string` signature. Existing call
sites compile unchanged — new functions are additive only.

**Residual risk (low, informational):**
The env-based 256-color detection path (`COLORTERM`/`TERM` parsing in the
`color256` init func) is not exercised by unit tests. The fallback logic itself
*is* tested (`TestColorFunctions256Codes` and `TestColorFunctionsBasicCodes`
force `color256 = true/false`), but the actual environment sniffing runs only
at init time and depends on real terminal state.
