# internal/color — cross-repo sync

**Canonical source:** this directory (`internal/color/` in the governa repo)
**Synced repos:** iq, skout, utils

---

## How it works

Four repos share an identical `internal/color` package. Governa is the
canonical source. The other three repos each carry a copy of the package;
whenever governa's copy changes, each of those repos re-syncs by
overwriting its local copy with governa's.

Each of the other repos is self-contained — nothing in iq, skout, or utils
references governa. Only an agent invoked in one of those repos reaches
out to governa to perform the sync.

### For an agent invoked in another repo (iq, skout, utils)

You are responsible for **your own repo only**. Never modify governa or
any repo other than the one you were invoked in. Repos are assumed to
live as siblings under a common parent directory, so governa is reachable
at `../governa/` from your repo's working directory.

1. **Compare your local `internal/color/` against the canonical copy.**
   ```bash
   diff -q internal/color/color.go      ../governa/internal/color/color.go
   diff -q internal/color/color_test.go ../governa/internal/color/color_test.go
   ```
   If both files are identical, no sync is needed — stop.
2. **Overwrite your local copies with the canonical ones.**
   ```bash
   cp ../governa/internal/color/color.go      internal/color/color.go
   cp ../governa/internal/color/color_test.go internal/color/color_test.go
   ```
3. **Run your repo's canonical build script.** (`./build.sh` or equivalent) Verify the full pipeline passes.
4. **Commit the synced files in your repo** with a message that identifies the sync, e.g. `sync internal/color from governa`.

### Feedback: if your repo is ahead of governa

If, when you run the `diff` step above, you discover that your local
`internal/color/` contains functions, fixes, or test coverage that are
**not present in governa's canonical copy**, do not run the `cp` step —
overwriting would lose work that governa's agent has not seen yet.

Instead:

1. **Create `internal/color/color-sync-feedback.md` in your repo** describing:
   - What your local copy has that the canonical copy does not (new
     functions, tests, bug fixes, behavior changes).
   - Why the local change was made (context, incident, requirement).
   - Verification status (tests passing, in production, etc.).
2. **Ask the user to share `color-sync-feedback.md` with governa's agent** so the canonical copy can absorb the improvement.
3. **Once the improvement lands in governa**, delete the feedback file and run a normal sync on the next invocation.

### Making changes to the package

All enhancements and fixes land in governa first. Once verified there,
the change is picked up by the other three repos the next time an agent
is invoked in them and runs the steps above. Do not make repo-local
color changes — they will diverge and be overwritten on the next sync.

---

## Standing notes

**Package surface:**
`Gra`, `Grn`, `GrnR`, `GrnD`, `Yel`, `Blu`, `Cya`, `Red`, `RedR`, `RedD`,
`Whi`, `Whi2`, `BoldW`, `ShowPalette`, `FormatUsage`, `UsageLine`

All color functions share the same `func(any) string` signature. Existing call
sites compile unchanged across syncs — new functions are additive only.

**Residual risk (low, informational):**
The env-based 256-color detection path (`COLORTERM`/`TERM` parsing in the
`color256` init func) is not exercised by unit tests. The fallback logic itself
*is* tested (`TestColorFunctions256Codes` and `TestColorFunctionsBasicCodes`
force `color256 = true/false`), but the actual environment sniffing runs only
at init time and depends on real terminal state.
