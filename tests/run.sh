#!/usr/bin/env bash
# tests/run.sh — governa build.sh test harness.
# Bash 3.2+ (no mapfile, no assoc arrays, no ${var^^}). Sources build.sh to
# unit-test functions; also invokes it as a subprocess for golden-output cases.
# Invoked by build.sh self-test (full no-target builds only); GOVERNA_BUILD_TEST=1
# is set by that invocation so subprocesses do not recurse into the self-test.
set -u

# Self-test force-failure hook: exit non-zero immediately so a full build fails.
if [ -n "${GOVERNA_SELFTEST_FORCE_FAIL:-}" ]; then
  printf 'tests/run.sh: GOVERNA_SELFTEST_FORCE_FAIL set — forced failure\n' >&2
  exit 1
fi

GOVERNA_BUILD_TEST=1; export GOVERNA_BUILD_TEST  # recursion guard for subprocesses

_TESTS_DIR="$(cd "$(dirname "$0")" && pwd)"
_ROOT="$(cd "$_TESTS_DIR/.." && pwd)"
_BS="$_ROOT/build.sh"
_FIXT="$_TESTS_DIR/fixtures"
_GOLD="$_TESTS_DIR/golden"
_SHIMS="$_TESTS_DIR/shims"
_SHIMS_REAL="$(cd "$_SHIMS" && pwd -P)"
export GOVERNA_SHIM_ROOT="$_SHIMS"

# Private temp dir for all test I/O — avoids /tmp/rsh_* collisions under parallel runs.
_TMPD="$(mktemp -d "${TMPDIR:-/tmp}/governa-tests.XXXXXX")"
_TMPD="$(cd "$_TMPD" && pwd)"

# Source build.sh for function-level tests; disable errexit so failure paths
# can be probed.
# shellcheck disable=SC1090
source "$_BS"
set +e
export NO_COLOR=1
_color_init

_pass=0 _fail=0
_ok()   { printf 'PASS  %s\n' "$1"; _pass=$((_pass+1)); }
_fail() { printf 'FAIL  %s\n' "$1"; _fail=$((_fail+1)); }

# ── cleanup ──────────────────────────────────────────────────────────────────
_PREP_TMP='' _MULTI_TMP='' _PDRY_REPO='' _REL_REPO='' _BUILD_REPO='' _RAWMSG_REPO=''
_cleanup() {
  rm -rf "$_TMPD" "$_PREP_TMP" "$_MULTI_TMP" "$_PDRY_REPO" "$_REL_REPO" "$_BUILD_REPO" "$_RAWMSG_REPO"
}
trap '_cleanup' EXIT

# ── golden comparison helper ─────────────────────────────────────────────────
# _cmp_golden LABEL GOLDEN_DIR OUT_FILE ERR_FILE STATUS_FILE [SED_SCRIPT]
# Compares byte-for-byte using diff on files; sed script is applied to a temp
# file so the originals are not modified. Trailing-newline differences are
# preserved (no variable-comparison stripping).
_cmp_golden() {
  local label="$1" gdir="$2" out="$3" err="$4" stat="$5" san="${6:-}"
  local ok=1 stream actual expected tmp
  tmp="$_TMPD/cmp.$$"
  for stream in stdout stderr status; do
    actual="$out"
    [ "$stream" = stderr  ] && actual="$err"
    [ "$stream" = status  ] && actual="$stat"
    expected="$gdir/$stream"
    [ -f "$expected" ] || { _fail "$label: missing golden $stream"; ok=0; continue; }
    if [ -n "$san" ] && [ "$stream" != status ]; then
      sed "$san" "$actual" >"$tmp"
      actual="$tmp"
    fi
    if diff -q "$expected" "$actual" >/dev/null 2>&1; then
      :
    else
      _fail "$label: $stream mismatch"
      diff "$expected" "$actual" | head -20 | sed 's/^/       /'
      ok=0
    fi
  done
  rm -f "$tmp"
  [ "$ok" = 1 ] && _ok "$label"
}

# _cmp_file LABEL EXPECTED_FILE ACTUAL_FILE [SED_SCRIPT]
_cmp_file() {
  local label="$1" expected="$2" actual="$3" san="${4:-}" tmp
  tmp="$_TMPD/cmpf.$$"
  if [ -n "$san" ]; then
    sed "$san" "$actual" >"$tmp"; actual="$tmp"
  fi
  if diff -q "$expected" "$actual" >/dev/null 2>&1; then
    _ok "$label"
  else
    _fail "$label"
    diff "$expected" "$actual" | head -10 | sed 's/^/       /'
  fi
  rm -f "$tmp"
}

# ── path sanitizer for build goldens ─────────────────────────────────────────
# Normalizes: coverage temp path → COVERFILE; GOPATH install path → GOPATH.
_BUILD_SAN="s|-coverprofile=[^ ]*|-coverprofile=COVERFILE|g;s|${_SHIMS_REAL}/gopath|GOPATH|g;s|${_SHIMS}/gopath|GOPATH|g"

# ── _go_quote parity ─────────────────────────────────────────────────────────
_tgq() {
  local in="$1" want="$2" label="$3"
  local got; got=$(_go_quote "$in")
  [ "$got" = "$want" ] && _ok "_go_quote: $label" || { _fail "_go_quote: $label"; printf '       want=%s got=%s\n' "$want" "$got"; }
}
_tgq 'hello'         '"hello"'        'plain ASCII'
_tgq ''              '""'             'empty string'
_tgq 'a"b'           '"a\"b"'         'embedded double-quote'
_tgq 'a\b'           '"a\\b"'         'backslash'
_tgq "$(printf '\t')" '"\t"'          'tab'
# Bare leading \n is consumed as awk RS; test realistic mid-string newline instead.
_nl_mid="hello
world"
_tgq "$_nl_mid" '"hello\nworld"' 'newline in middle'
_tgq "$(printf '\r')" '"\r"'          'carriage-return'
_tgq "$(printf '\001')" '"\x01"'      'control byte 0x01'
_tgq "$(printf '\177')" '"\x7f"'      'DEL 0x7f'
_tgq "$(printf '\007')" '"\a"'        'BEL'
_tgq "$(printf '\b')"   '"\b"'        'backspace'
_tgq "$(printf '\f')"   '"\f"'        'form feed'
_tgq "$(printf '\v')"   '"\v"'        'vertical tab'
_tgq 'héllo'         '"héllo"'        'UTF-8 multi-byte (pass-through)'
# Documented exception: 0xff passes through raw (display divergence; no Unicode classification).
got_ff=$(_go_quote "$(printf '\377')")
[ "$got_ff" = "\"$(printf '\377')\"" ] && _ok "_go_quote: 0xff pass-through (exception)" \
  || _fail "_go_quote: 0xff unexpected: $got_ff"
# Documented exception: U+200B (ZERO WIDTH SPACE) passes through raw; Go would emit ​.
_u200b="$(printf '\342\200\213')"  # UTF-8 encoding of U+200B
got_200b=$(_go_quote "$_u200b")
[ "$got_200b" = "\"$_u200b\"" ] && _ok "_go_quote: U+200B pass-through (exception)" \
  || _fail "_go_quote: U+200B unexpected: $got_200b"
# Commit-path raw-bytes: run the release path with an embedded 0x01 byte using
# real git, then compare the committed message from git log with the raw input.
_RAWMSG_REPO="$(mktemp -d "${TMPDIR:-/tmp}/governa-rawmsg.XXXXXX")"
_RAWMSG_REPO="$(cd "$_RAWMSG_REPO" && pwd)"
mkdir -p "$_RAWMSG_REPO/cmd/widget"
printf 'module example.com/widget\n\ngo 1.23\n' >"$_RAWMSG_REPO/go.mod"
printf 'package main\n\nconst programVersion = "0.1.0"\n\nfunc main() {}\n' \
  >"$_RAWMSG_REPO/cmd/widget/main.go"
( cd "$_RAWMSG_REPO" && git init -q && git config user.email t@t && git config user.name t \
  && git add -A && git commit -q -m init ) 2>/dev/null
printf '\n// release change\n' >>"$_RAWMSG_REPO/cmd/widget/main.go"
_raw_msg=$(printf 'a\001b')
( cd "$_RAWMSG_REPO" && NO_COLOR=1 \
    bash "$_BS" v0.2.0 "$_raw_msg" ) >"$_TMPD/rawmsg-rel.out" 2>"$_TMPD/rawmsg-rel.err" <<< "y" || true
( cd "$_RAWMSG_REPO" && git log -1 --format=%B ) >"$_TMPD/rawmsg-actual"
printf '%s\n\n' "$_raw_msg" >"$_TMPD/rawmsg-expected"
_cmp_file "release commit preserves raw 0x01 message bytes" \
  "$_TMPD/rawmsg-expected" "$_TMPD/rawmsg-actual"
rm -rf "$_RAWMSG_REPO"; _RAWMSG_REPO=''
_displayed=$(_go_quote "$_raw_msg")
[ "$_displayed" = '"a\x01b"' ] && _ok "_go_quote display form differs from raw" \
  || _fail "_go_quote display form wrong: $_displayed"

# Pinned staticcheck must ignore an earlier PATH executable.
_path_stub_dir="$_TMPD/path-stub"
mkdir -p "$_path_stub_dir"
printf '%s\n' '#!/usr/bin/env bash' 'printf used >"$GOVERNA_PATH_STUB_MARKER"' 'exit 99' \
  >"$_path_stub_dir/staticcheck"
chmod +x "$_path_stub_dir/staticcheck"
_path_stub_marker="$_TMPD/path-stub-used"
PATH="$_path_stub_dir:$_SHIMS:$PATH" GOVERNA_PATH_STUB_MARKER="$_path_stub_marker" \
  _ensure_staticcheck "$_SHIMS/gopath/bin" '' >"$_TMPD/staticcheck-ensure.out"
PATH="$_path_stub_dir:$_SHIMS:$PATH" GOVERNA_PATH_STUB_MARKER="$_path_stub_marker" \
  "$_staticcheck_path" ./... >"$_TMPD/staticcheck.out" 2>"$_TMPD/staticcheck.err"
[ "$_staticcheck_path" = "$_SHIMS/gopath/bin/staticcheck" ] \
  && _ok "staticcheck: pinned GOPATH executable selected" \
  || _fail "staticcheck: wrong executable selected: $_staticcheck_path"
[ ! -e "$_path_stub_marker" ] \
  && _ok "staticcheck: earlier PATH executable ignored" \
  || _fail "staticcheck: earlier PATH executable was invoked"

# ── color / TTY gating ───────────────────────────────────────────────────────
_with_color() { _color_on=1; _color256=1; }
_no_color()   { _color_on=0; _color256=0; }

# Each helper: SGR when enabled+256color, plain when off.
_with_color
got=$(yel7 "x"); want=$(printf '\033[38;5;227mx\033[0m')
[ "$got" = "$want" ] && _ok "color: yel7 SGR when on"    || _fail "color: yel7 SGR mismatch"
got=$(yel5 "x"); want=$(printf '\033[38;5;220mx\033[0m')
[ "$got" = "$want" ] && _ok "color: yel5 SGR when on"    || _fail "color: yel5 SGR mismatch"
got=$(grn3 "x"); want=$(printf '\033[38;5;34mx\033[0m')
[ "$got" = "$want" ] && _ok "color: grn3 SGR when on"    || _fail "color: grn3 SGR mismatch"
got=$(grn5 "x"); want=$(printf '\033[38;5;46mx\033[0m')
[ "$got" = "$want" ] && _ok "color: grn5 SGR when on"    || _fail "color: grn5 SGR mismatch"
got=$(gra5 "x"); want=$(printf '\033[38;5;245mx\033[0m')
[ "$got" = "$want" ] && _ok "color: gra5 SGR when on"    || _fail "color: gra5 SGR mismatch"
got=$(cya4 "x"); want=$(printf '\033[38;5;44mx\033[0m')
[ "$got" = "$want" ] && _ok "color: cya4 SGR when on"    || _fail "color: cya4 SGR mismatch"
got=$(red3 "x"); want=$(printf '\033[38;5;124mx\033[0m')
[ "$got" = "$want" ] && _ok "color: red3 SGR when on"    || _fail "color: red3 SGR mismatch"
got=$(whi5 "x"); want=$(printf '\033[38;5;231mx\033[0m')
[ "$got" = "$want" ] && _ok "color: whi5 SGR when on"    || _fail "color: whi5 SGR mismatch"

# All helpers emit plain text when color is off.
_no_color
for _fn in yel7 yel5 grn3 grn5 gra5 cya4 red3 whi5; do
  got=$("$_fn" "x")
  [ "$got" = "x" ] && _ok "color: $_fn plain when _color_on=0" \
    || _fail "color: $_fn not plain: '$got'"
done

# _color_init with various env combinations.
_tci() {  # label want_on env_setup_cmds...
  local label="$1" want_on="$2"; shift 2
  eval "$*"
  _color_init
  [ "$_color_on" = "$want_on" ] && _ok "_color_init: $label" \
    || _fail "_color_init: $label (want _color_on=$want_on got $_color_on)"
}
_tci "NO_COLOR=1 → off"           0 "NO_COLOR=1 GOVERNA_FORCE_TTY='' TERM='' COLORTERM=''"
_tci "TERM=dumb → off"            0 "TERM=dumb NO_COLOR='' GOVERNA_FORCE_TTY=''"
_tci "FORCE_TTY=1 → on"           1 "GOVERNA_FORCE_TTY=1 NO_COLOR='' TERM='' COLORTERM=truecolor"
_tci "FORCE_TTY=0 → off"          0 "GOVERNA_FORCE_TTY=0 NO_COLOR='' TERM='' COLORTERM=truecolor"
_tci "FORCE_TTY=1 + NO_COLOR → off" 0 "GOVERNA_FORCE_TTY=1 NO_COLOR=1 TERM='' COLORTERM=''"
# 256-color gate: _color_on=1 but _color256=0 → _wrap still emits plain text.
_color_on=1; _color256=0
got=$(yel7 "x")
[ "$got" = "x" ] && _ok "color: 256color gate off (_color256=0): yel7 plain" \
  || _fail "color: 256color gate off: yel7 not plain: '$got'"
# Verify _color_init sets _color256 correctly from env.
_tci "256color gate: COLORTERM=truecolor → on" 1 \
     "GOVERNA_FORCE_TTY=1 NO_COLOR='' TERM='' COLORTERM=truecolor"
[ "$_color256" = 1 ] && _ok "_color_init: COLORTERM=truecolor → _color256=1" \
  || _fail "_color_init: COLORTERM=truecolor → _color256=$_color256"
_tci "256color gate: TERM=*256color* → on"   1 \
     "GOVERNA_FORCE_TTY=1 NO_COLOR='' TERM=xterm-256color COLORTERM=''"
[ "$_color256" = 1 ] && _ok "_color_init: TERM=*256color → _color256=1" \
  || _fail "_color_init: TERM=*256color → _color256=$_color256"
_tci "256color gate: no COLORTERM or 256 TERM → on (TTY-only)" 1 \
     "GOVERNA_FORCE_TTY=1 NO_COLOR='' TERM=xterm COLORTERM=''"
[ "$_color256" = 0 ] && _ok "_color_init: plain TERM → _color256=0" \
  || _fail "_color_init: plain TERM → _color256=$_color256"

# Restore for remaining tests.
_color_on=0; _color256=0
NO_COLOR=1; export NO_COLOR
unset GOVERNA_FORCE_TTY TERM COLORTERM 2>/dev/null; true

# ── mdcheck / _scan_nested_fences ────────────────────────────────────────────
# Check clean files produce no output.
_scan_clean() {
  local label="$1" file="$2"
  local got; got=$(_scan_nested_fences "$(basename "$file")" "$file")
  [ -z "$got" ] && _ok "mdcheck: $label → clean" \
    || _fail "mdcheck: $label → unexpected: $got"
}
# Check flagged file produces exact expected line.
_scan_flagged() {
  local label="$1" file="$2" expected_line="$3"
  local got; got=$(_scan_nested_fences "$(basename "$file")" "$file")
  [ "$got" = "$expected_line" ] && _ok "mdcheck: $label → exact match" \
    || { _fail "mdcheck: $label"; printf '       want: %s\n       got:  %s\n' "$expected_line" "$got"; }
}
_scan_clean   "clean"         "$_FIXT/mdcheck/clean.md"
_scan_clean   "tilde-outer"   "$_FIXT/mdcheck/tilde-outer.md"
_scan_clean   "quad-backtick" "$_FIXT/mdcheck/quad-backtick.md"
# nested-tagged.md: 3-backtick fence at line 3 contains tagged inner at line 5.
_scan_flagged "nested-tagged" "$_FIXT/mdcheck/nested-tagged.md" \
  "nested-tagged.md:5: 3-backtick fence opened at line 3 contains nested tagged fence; use 4+ backticks or ~~~ for the outer fence"

# Repository-wide discovery excludes intentionally invalid Markdown fixtures.
_md_list=$(_md_files "$_ROOT")
if printf '%s\n' "$_md_list" | grep -Fq '/tests/fixtures/'; then
  _fail "mdcheck: tracked fixture paths leaked into repository discovery"
else
  _ok "mdcheck: tracked fixture paths excluded from repository discovery"
fi

# The non-git filesystem fallback applies the same exclusion.
_md_fallback_root="$_TMPD/mdcheck-fallback"
mkdir -p "$_md_fallback_root/tests/fixtures"
printf '# kept\n' >"$_md_fallback_root/kept.md"
printf '# ignored\n' >"$_md_fallback_root/tests/fixtures/ignored.md"
_md_fallback_list=$(_md_files "$_md_fallback_root")
if printf '%s\n' "$_md_fallback_list" | grep -Fq 'kept.md' \
  && ! printf '%s\n' "$_md_fallback_list" | grep -Fq 'ignored.md'; then
  _ok "mdcheck: filesystem fallback excludes fixture paths"
else
  _fail "mdcheck: filesystem fallback fixture exclusion failed"
fi

# ── prep function unit tests ──────────────────────────────────────────────────
_PREP_TMP="$(mktemp -d "${TMPDIR:-/tmp}/governa-prep-test.XXXXXX")"
_PREP_TMP="$(cd "$_PREP_TMP" && pwd)"
cp -r "$_FIXT/preprepo/." "$_PREP_TMP/"
( cd "$_PREP_TMP" && git init -q && git config user.email t@t && git config user.name t \
  && git add -A && git commit -q -m init && git tag v0.1.0 ) 2>/dev/null
# Add a wip commit so HEAD != tag
printf 'package main\n\nconst programVersion = "0.1.0"\n\nfunc main() { _ = programVersion }\n' \
  >"$_PREP_TMP/cmd/widget/main.go"
( cd "$_PREP_TMP" && git add -A && git commit -q -m wip ) 2>/dev/null

# _prep_parse_ac_refs
got=$(_prep_parse_ac_refs "fix: no refs here")
[ -z "$got" ] && _ok "prep: _prep_parse_ac_refs → empty on no refs" \
  || _fail "prep: _prep_parse_ac_refs unexpectedly non-empty: $got"
got=$(_prep_parse_ac_refs "AC42: do stuff, also AC10")
[ "$got" = "$(printf '10\n42')" ] && _ok "prep: _prep_parse_ac_refs → sorted unique AC nums" \
  || _fail "prep: _prep_parse_ac_refs wrong: '$got'"

# _prep_detect_version_targets: single-utility
_prep_detect_version_targets "$_PREP_TMP"
[ -z "$_prep_warning" ] && _ok "prep: single-utility: no warning" \
  || _fail "prep: single-utility: unexpected warning: $_prep_warning"
printf '%s\n' "$_prep_vtargets" | grep -q "cmd/widget/main.go" \
  && _ok "prep: single-utility: vtargets has widget" \
  || _fail "prep: single-utility: vtargets missing widget: '$_prep_vtargets'"

# _prep_detect_version_targets: multi-utility warning
_MULTI_TMP="$(mktemp -d "${TMPDIR:-/tmp}/governa-multi-test.XXXXXX")"
_MULTI_TMP="$(cd "$_MULTI_TMP" && pwd)"
cp -r "$_FIXT/preprepo-multi/." "$_MULTI_TMP/"
printf 'module example.com/multi\n\ngo 1.23\n' >"$_MULTI_TMP/go.mod"
_prep_detect_version_targets "$_MULTI_TMP"
[ -n "$_prep_warning" ] && _ok "prep: multi-utility: warning emitted" \
  || _fail "prep: multi-utility: expected warning"
[ -z "$_prep_vtargets" ] && _ok "prep: multi-utility: no bump targets (ambiguous)" \
  || _fail "prep: multi-utility: unexpected bump targets: $_prep_vtargets"

# _prep_detect_changelog_targets
_prep_detect_changelog_targets "$_PREP_TMP" v0.2.0
[ -z "$_prep_cl_err" ] && _ok "prep: cl-detect: no error" \
  || _fail "prep: cl-detect: unexpected error: $_prep_cl_err"
printf '%s\n' "$_prep_ctargets" | grep -q "CHANGELOG.md" \
  && _ok "prep: cl-detect: root CHANGELOG" \
  || _fail "prep: cl-detect: root CHANGELOG missing"
printf '%s\n' "$_prep_ctargets" | grep -q "internal/templates/CHANGELOG.md" \
  && _ok "prep: cl-detect: templates CHANGELOG" \
  || _fail "prep: cl-detect: templates CHANGELOG missing"

# _prep_apply_version_bump
_VER_TMP="$_PREP_TMP/cmd/widget/main.go"
_prep_apply_version_bump "$_VER_TMP" programVersion 0.2.0
grep -q '"0.2.0"' "$_VER_TMP" && _ok "prep: version bump applied" \
  || _fail "prep: version bump failed"
m=$(stat -f '%Lp' "$_VER_TMP" 2>/dev/null || stat -c '%a' "$_VER_TMP" 2>/dev/null)
[ "$m" = "644" ] && _ok "prep: version bump preserves 0644 mode" \
  || _fail "prep: version bump mode: $m (want 644)"

# _prep_apply_changelog_insert + idempotency guard
_CL_TMP="$_PREP_TMP/CHANGELOG.md"
_prep_apply_changelog_insert "$_CL_TMP" 0.2.0 "AC42: test"
grep -Fq '| 0.2.0 | AC42: test |' "$_CL_TMP" && _ok "prep: changelog row inserted" \
  || _fail "prep: changelog row not inserted"
_prep_detect_changelog_targets "$_PREP_TMP" v0.2.0
[ -n "$_prep_cl_err" ] && _ok "prep: changelog idempotency guard fires" \
  || _fail "prep: changelog idempotency guard not fired"

# _prep_remove_ie_lines: removes matching IE with backslash in content
_PLAN_TMP="$_PREP_TMP/plan.md"
cat >"$_PLAN_TMP" <<'PLAN'
# Plan

## Ideas To Explore

- IE1: keep (no pointer)
- IE2: Z:\path\name → governa/ac42-sample.md
PLAN
_prep_remove_ie_lines "$_PREP_TMP" "- IE2: Z:\\path\\name → governa/ac42-sample.md"
grep -Fq 'IE1: keep' "$_PLAN_TMP" && _ok "prep: IE removal keeps non-matching line" \
  || _fail "prep: IE removal removed wrong line"
grep -Fq 'Z:\path\name' "$_PLAN_TMP" && _fail "prep: IE with backslash NOT removed" \
  || _ok "prep: IE with backslash removed"

# The -B and --no-build forms must reach prep_run with identical arguments.
_saved_prep_run=$(declare -f prep_run)
_prep_alias_args=''
prep_run() { _prep_alias_args="$*"; return 0; }
prep_main -B v0.2.0 'alias test' >"$_TMPD/prep-short.out" 2>"$_TMPD/prep-short.err"
_prep_short_args="$_prep_alias_args"
_prep_alias_args=''
prep_main --no-build v0.2.0 'alias test' >"$_TMPD/prep-long.out" 2>"$_TMPD/prep-long.err"
_prep_long_args="$_prep_alias_args"
eval "$_saved_prep_run"
[ "$_prep_short_args" = '0 1 v0.2.0 alias test' ] \
  && [ "$_prep_short_args" = "$_prep_long_args" ] \
  && _ok "prep: -B matches --no-build dispatch" \
  || _fail "prep: -B/--no-build mismatch: short='$_prep_short_args' long='$_prep_long_args'"

# ── self-test force-failure + target isolation ────────────────────────────────
# GOVERNA_SELFTEST_FORCE_FAIL=1 makes tests/run.sh exit non-zero.
_selftest_rc=0
( GOVERNA_SELFTEST_FORCE_FAIL=1 bash "$_TESTS_DIR/run.sh" ) 2>/dev/null || _selftest_rc=$?
[ "$_selftest_rc" -ne 0 ] && _ok "self-test: SELFTEST_FORCE_FAIL causes non-zero exit" \
  || _fail "self-test: SELFTEST_FORCE_FAIL did not exit non-zero"

# Create a temp copy of buildrepo for target-isolation tests.
_BUILD_REPO="$(mktemp -d "${TMPDIR:-/tmp}/governa-bld.XXXXXX")"
_BUILD_REPO="$(cd "$_BUILD_REPO" && pwd)"
cp -r "$_FIXT/buildrepo/." "$_BUILD_REPO/"

# Targeted build (driftscan) with GOVERNA_SELFTEST_FORCE_FAIL=1 → succeeds.
# build_run self-test condition requires ${#targets[@]} -eq 0; a named
# target skips the self-test entirely, so SELFTEST_FORCE_FAIL has no effect.
_selftest_rc=0
( cd "$_BUILD_REPO" && NO_COLOR=1 PATH="$_SHIMS:$PATH" \
    GOVERNA_SELFTEST_FORCE_FAIL=1 \
    bash "$_BS" driftscan ) >"$_TMPD/selftest-tgt.out" 2>"$_TMPD/selftest-tgt.err" || _selftest_rc=$?
[ "$_selftest_rc" -eq 0 ] \
  && _ok "self-test: targeted build with SELFTEST_FORCE_FAIL=1 → exit 0" \
  || { _fail "self-test: targeted build unexpectedly failed (rc=$_selftest_rc)"; \
       tail -5 "$_TMPD/selftest-tgt.err" | sed 's/^/       /'; }

# Full no-target build with GOVERNA_SELFTEST_FORCE_FAIL=1 → fails.
# Unset GOVERNA_BUILD_TEST so the self-test invocation is not suppressed.
_selftest_rc=0
( cd "$_BUILD_REPO" && NO_COLOR=1 PATH="$_SHIMS:$PATH" \
    GOVERNA_BUILD_TEST= GOVERNA_SELFTEST_FORCE_FAIL=1 \
    bash "$_BS" ) >"$_TMPD/selftest-full.out" 2>"$_TMPD/selftest-full.err" || _selftest_rc=$?
[ "$_selftest_rc" -ne 0 ] \
  && _ok "self-test: full build with SELFTEST_FORCE_FAIL=1 → exit non-zero" \
  || _fail "self-test: full build did not fail (rc=$_selftest_rc)"
grep -q 'self-test' "$_TMPD/selftest-full.err" \
  && _ok "self-test: full build failure mentions self-test" \
  || _fail "self-test: full build stderr missing 'self-test': $(cat "$_TMPD/selftest-full.err")"

# ── static-golden function calls, both NO_COLOR modes ────────────────────────
# Mode 1 (NO_COLOR=1): _color_on=0 via env gate.
{ build_usage; } >"$_TMPD/out" 2>"$_TMPD/err"; printf '%d\n' $? >"$_TMPD/rc"
_cmp_golden "build-help (NO_COLOR=1)" "$_GOLD/build-help" "$_TMPD/out" "$_TMPD/err" "$_TMPD/rc"

{ rel_main; } >"$_TMPD/out" 2>"$_TMPD/err"; printf '%d\n' $? >"$_TMPD/rc"
_cmp_golden "rel-usage (NO_COLOR=1)" "$_GOLD/rel-usage" "$_TMPD/out" "$_TMPD/err" "$_TMPD/rc"

{ rel_main v1.2.3; } >"$_TMPD/out" 2>"$_TMPD/err"; printf '%d\n' $? >"$_TMPD/rc"
_cmp_golden "rel-bad-args (NO_COLOR=1)" "$_GOLD/rel-bad-args" "$_TMPD/out" "$_TMPD/err" "$_TMPD/rc"

{ prep_main; } >"$_TMPD/out" 2>"$_TMPD/err"; printf '%d\n' $? >"$_TMPD/rc"
_cmp_golden "prep-help (NO_COLOR=1)" "$_GOLD/prep-help" "$_TMPD/out" "$_TMPD/err" "$_TMPD/rc"

# Mode 2 (NO_COLOR unset, non-TTY): _color_init with GOVERNA_FORCE_TTY=0 → _color_on=0.
unset NO_COLOR
GOVERNA_FORCE_TTY=0; _color_init

{ build_usage; } >"$_TMPD/out" 2>"$_TMPD/err"; printf '%d\n' $? >"$_TMPD/rc"
_cmp_golden "build-help (NO_COLOR unset)" "$_GOLD/build-help" "$_TMPD/out" "$_TMPD/err" "$_TMPD/rc"

{ rel_main; } >"$_TMPD/out" 2>"$_TMPD/err"; printf '%d\n' $? >"$_TMPD/rc"
_cmp_golden "rel-usage (NO_COLOR unset)" "$_GOLD/rel-usage" "$_TMPD/out" "$_TMPD/err" "$_TMPD/rc"

{ rel_main v1.2.3; } >"$_TMPD/out" 2>"$_TMPD/err"; printf '%d\n' $? >"$_TMPD/rc"
_cmp_golden "rel-bad-args (NO_COLOR unset)" "$_GOLD/rel-bad-args" "$_TMPD/out" "$_TMPD/err" "$_TMPD/rc"

{ prep_main; } >"$_TMPD/out" 2>"$_TMPD/err"; printf '%d\n' $? >"$_TMPD/rc"
_cmp_golden "prep-help (NO_COLOR unset)" "$_GOLD/prep-help" "$_TMPD/out" "$_TMPD/err" "$_TMPD/rc"

NO_COLOR=1; GOVERNA_FORCE_TTY=''; export NO_COLOR; _color_init

# ── build golden cases (subprocess + shims) ───────────────────────────────────
# Rebuild _BUILD_REPO as a fresh copy for the golden-output tests.
rm -rf "$_BUILD_REPO"
_BUILD_REPO="$(mktemp -d "${TMPDIR:-/tmp}/governa-bld.XXXXXX")"
_BUILD_REPO="$(cd "$_BUILD_REPO" && pwd)"
cp -r "$_FIXT/buildrepo/." "$_BUILD_REPO/"

_run_build_golden() {  # label golden_dir args...
  local label="$1" gdir="$2"; shift 2
  local rc=0
  ( cd "$_BUILD_REPO" && NO_COLOR=1 PATH="$_SHIMS:$PATH" \
      GOVERNA_SHIM_ROOT="$_SHIMS" \
      bash "$_BS" "$@" ) >"$_TMPD/out" 2>"$_TMPD/err" || rc=$?
  printf '%d\n' "$rc" >"$_TMPD/rc"
  _cmp_golden "$label" "$gdir" "$_TMPD/out" "$_TMPD/err" "$_TMPD/rc" "$_BUILD_SAN"
}

# NO_COLOR=1 is already set; run each case then re-run without NO_COLOR (still
# non-TTY so color stays off) to prove the golden matches in both modes.
_run_build_golden "build-no-args (NO_COLOR=1)"    "$_GOLD/build-no-args"
# Second pass: NO_COLOR unset; stdout is a file so _color_init gates off.
( cd "$_BUILD_REPO" && unset NO_COLOR && PATH="$_SHIMS:$PATH" GOVERNA_SHIM_ROOT="$_SHIMS" \
    bash "$_BS" ) >"$_TMPD/out2" 2>"$_TMPD/err2"
rc_nc=$?; printf '%d\n' "$rc_nc" >"$_TMPD/rc2"
_cmp_golden "build-no-args (NO_COLOR unset)" "$_GOLD/build-no-args" \
  "$_TMPD/out2" "$_TMPD/err2" "$_TMPD/rc2" "$_BUILD_SAN"

_run_build_golden "build-verbose (NO_COLOR=1)"    "$_GOLD/build-verbose"  -v
rc=0
( cd "$_BUILD_REPO" && unset NO_COLOR && PATH="$_SHIMS:$PATH" GOVERNA_SHIM_ROOT="$_SHIMS" \
    bash "$_BS" -v ) >"$_TMPD/out" 2>"$_TMPD/err" || rc=$?
printf '%d\n' "$rc" >"$_TMPD/rc"
_cmp_golden "build-verbose (NO_COLOR unset)" "$_GOLD/build-verbose" \
  "$_TMPD/out" "$_TMPD/err" "$_TMPD/rc" "$_BUILD_SAN"

_run_build_golden "build-driftscan (NO_COLOR=1)"  "$_GOLD/build-driftscan" driftscan
rc=0
( cd "$_BUILD_REPO" && unset NO_COLOR && PATH="$_SHIMS:$PATH" GOVERNA_SHIM_ROOT="$_SHIMS" \
    bash "$_BS" driftscan ) >"$_TMPD/out" 2>"$_TMPD/err" || rc=$?
printf '%d\n' "$rc" >"$_TMPD/rc"
_cmp_golden "build-driftscan (NO_COLOR unset)" "$_GOLD/build-driftscan" \
  "$_TMPD/out" "$_TMPD/err" "$_TMPD/rc" "$_BUILD_SAN"

# ── prep-dry-run golden (subprocess + git shim, both NO_COLOR modes) ──────────
_PDRY_REPO="$(mktemp -d "${TMPDIR:-/tmp}/governa-pdry.XXXXXX")"
_PDRY_REPO="$(cd "$_PDRY_REPO" && pwd)"
cp -r "$_FIXT/preprepo/." "$_PDRY_REPO/"
( cd "$_PDRY_REPO" && git init -q && git config user.email t@t && git config user.name t \
  && git add -A && git commit -q -m init ) 2>/dev/null

rc=0
( cd "$_PDRY_REPO" && PATH="$_SHIMS:$PATH" NO_COLOR=1 \
    bash "$_BS" prep --dry-run --no-build v0.2.0 "AC42: sample improvement" \
    ) >"$_TMPD/out" 2>"$_TMPD/err" || rc=$?
printf '%d\n' "$rc" >"$_TMPD/rc"
# Sanitize fixture repo path (macOS /var is symlink to /private/var; sanitize both).
_pdry_real=$(cd "$_PDRY_REPO" && pwd -P 2>/dev/null || printf '%s' "$_PDRY_REPO")
_pdry_san="s|$_pdry_real/||g;s|$_PDRY_REPO/||g"
_cmp_golden "prep-dry-run (NO_COLOR=1)" "$_GOLD/prep-dry-run" "$_TMPD/out" "$_TMPD/err" "$_TMPD/rc" "$_pdry_san"
# Second pass: NO_COLOR unset → same output (non-TTY pipe).
rc=0
( cd "$_PDRY_REPO" && unset NO_COLOR && PATH="$_SHIMS:$PATH" \
    bash "$_BS" prep --dry-run --no-build v0.2.0 "AC42: sample improvement" \
    ) >"$_TMPD/out" 2>"$_TMPD/err" || rc=$?
printf '%d\n' "$rc" >"$_TMPD/rc"
_cmp_golden "prep-dry-run (NO_COLOR unset)" "$_GOLD/prep-dry-run" \
  "$_TMPD/out" "$_TMPD/err" "$_TMPD/rc" "$_pdry_san"

# ── rel golden cases via recording git shim ───────────────────────────────────
_REL_REPO="$(mktemp -d "${TMPDIR:-/tmp}/governa-rel.XXXXXX")"
_REL_REPO="$(cd "$_REL_REPO" && pwd)"
mkdir -p "$_REL_REPO/cmd/widget"
printf 'module example.com/widget\n\ngo 1.23\n' >"$_REL_REPO/go.mod"
printf 'package main\n\nconst programVersion = "0.1.0"\n\nfunc main() {}\n' \
  >"$_REL_REPO/cmd/widget/main.go"
( cd "$_REL_REPO" && git init -q && git config user.email t@t && git config user.name t \
  && git add -A && git commit -q -m init ) 2>/dev/null

# rel-cancel
rc=0
( cd "$_REL_REPO" && NO_COLOR=1 PATH="$_SHIMS:$PATH" GOVERNA_SHIM_ROOT="$_SHIMS" \
    bash "$_BS" v0.2.0 "sample release" ) >"$_TMPD/out" 2>"$_TMPD/err" <<< "N" || rc=$?
printf '%d\n' "$rc" >"$_TMPD/rc"
_cmp_golden "rel-golden: rel-cancel" "$_GOLD/rel-cancel" "$_TMPD/out" "$_TMPD/err" "$_TMPD/rc"

# rel-success + gitlog verification
_gitlog="$_TMPD/gitlog"
: >"$_gitlog"
rc=0
( cd "$_REL_REPO" && NO_COLOR=1 PATH="$_SHIMS:$PATH" GOVERNA_SHIM_ROOT="$_SHIMS" \
    GOVERNA_GIT_LOG="$_gitlog" \
    bash "$_BS" v0.2.0 "sample release" ) >"$_TMPD/out" 2>"$_TMPD/err" <<< "y" || rc=$?
printf '%d\n' "$rc" >"$_TMPD/rc"
_cmp_golden "rel-golden: rel-success" "$_GOLD/rel-success" "$_TMPD/out" "$_TMPD/err" "$_TMPD/rc"
_cmp_file   "rel-golden: rel-success gitlog" "$_GOLD/rel-success/gitlog" "$_gitlog"

# rel-tag-created-failed
rc=0
( cd "$_REL_REPO" && NO_COLOR=1 PATH="$_SHIMS:$PATH" GOVERNA_SHIM_ROOT="$_SHIMS" \
    GOVERNA_GIT_SHIM_FAIL=push-tag \
    bash "$_BS" v0.2.0 "sample release" ) >"$_TMPD/out" 2>"$_TMPD/err" <<< "y" || rc=$?
printf '%d\n' "$rc" >"$_TMPD/rc"
_cmp_golden "rel-golden: rel-tag-created-failed" "$_GOLD/rel-tag-created-failed" \
  "$_TMPD/out" "$_TMPD/err" "$_TMPD/rc"

# rel-tag-pushed-branch-failed
rc=0
( cd "$_REL_REPO" && NO_COLOR=1 PATH="$_SHIMS:$PATH" GOVERNA_SHIM_ROOT="$_SHIMS" \
    GOVERNA_GIT_SHIM_FAIL=push-branch \
    bash "$_BS" v0.2.0 "sample release" ) >"$_TMPD/out" 2>"$_TMPD/err" <<< "y" || rc=$?
printf '%d\n' "$rc" >"$_TMPD/rc"
_cmp_golden "rel-golden: rel-tag-pushed-branch-failed" "$_GOLD/rel-tag-pushed-branch-failed" \
  "$_TMPD/out" "$_TMPD/err" "$_TMPD/rc"

# ── test-naming lint ─────────────────────────────────────────────────────────
# Forbidden fixture tokens assembled from fragments so this file does not
# trigger _lint_test_naming when build.sh scans tests/run.sh.

# lint-catches-at-numbered-identifier
_lf="$_TMPD/lint-ident-at.sh"
_lint_token="_at""1_foo=1"
printf '%s\n' "$_lint_token" >"$_lf"
_lint_rc=0; _lint_out=$(_lint_test_naming "$_lf" 2>&1) || _lint_rc=$?
if [ "$_lint_rc" -ne 0 ] && printf '%s\n' "$_lint_out" | grep -Fq "$_lf:" \
  && printf '%s\n' "$_lint_out" | grep -Fq "$_lint_token"; then
  _ok "lint: at-numbered identifier diagnostic names path and line"
else
  _fail "lint: at-numbered identifier diagnostic incomplete: $_lint_out"
fi

# lint-catches-ac-numbered-identifier
_lf="$_TMPD/lint-ident-ac.sh"
printf '%s\n' "_ac""1_bar=1" >"$_lf"
_lint_rc=0; _lint_test_naming "$_lf" >/dev/null 2>&1 || _lint_rc=$?
[ "$_lint_rc" -ne 0 ] && _ok "lint: ac-numbered identifier caught" \
  || _fail "lint: ac-numbered identifier not caught"

# lint-catches-shell-label-callsite (double-quoted AT)
_lf="$_TMPD/lint-lbl-sh-at.sh"
printf '%s\n' '_ok "'"AT""1: behavior"'"' >"$_lf"
_lint_rc=0; _lint_test_naming "$_lf" >/dev/null 2>&1 || _lint_rc=$?
[ "$_lint_rc" -ne 0 ] && _ok "lint: shell at-label callsite caught" \
  || _fail "lint: shell at-label callsite not caught"

# lint-catches-single-quoted-shell-label (single-quoted AC)
_lf="$_TMPD/lint-lbl-sh-sq.sh"
_q="'"
printf '%s\n' "_fail ${_q}""AC""1: behavior${_q}" >"$_lf"
_lint_rc=0; _lint_test_naming "$_lf" >/dev/null 2>&1 || _lint_rc=$?
[ "$_lint_rc" -ne 0 ] && _ok "lint: single-quoted shell ac-label caught" \
  || _fail "lint: single-quoted shell ac-label not caught"

# lint-catches-ac-shell-label-callsite (double-quoted AC)
_lf="$_TMPD/lint-lbl-sh-ac.sh"
printf '%s\n' '_ok "'"AC""42: something"'"' >"$_lf"
_lint_rc=0; _lint_test_naming "$_lf" >/dev/null 2>&1 || _lint_rc=$?
[ "$_lint_rc" -ne 0 ] && _ok "lint: shell ac-label callsite caught" \
  || _fail "lint: shell ac-label callsite not caught"

# lint-catches-go-label-callsite (double-quoted AT)
_lf="$_TMPD/lint-lbl-go-at.go"
printf '%s\n' 't.Run("'"AT""1: something"'",' >"$_lf"
_lint_rc=0; _lint_test_naming "$_lf" >/dev/null 2>&1 || _lint_rc=$?
[ "$_lint_rc" -ne 0 ] && _ok "lint: go at-label callsite caught" \
  || _fail "lint: go at-label callsite not caught"

# lint-catches-raw-go-label-callsite (backtick AC)
_lf="$_TMPD/lint-lbl-go-raw.go"
_bq='`'
printf '%s\n' "t.Run(${_bq}""AC""1: something${_bq}," >"$_lf"
_lint_rc=0; _lint_test_naming "$_lf" >/dev/null 2>&1 || _lint_rc=$?
[ "$_lint_rc" -ne 0 ] && _ok "lint: go raw-string ac-label caught" \
  || _fail "lint: go raw-string ac-label not caught"

# lint-ignores-historical-comments
_lf="$_TMPD/lint-historical.sh"
{
  printf '%s\n' '# _at'"1_foo old name"
  printf '%s\n' '// _ac'"1_bar was used"
  printf '%s\n' 'some_code # Historical: _ok "'"AT""1: old label\""
  printf '%s\n' 'some_code // Historical: t.Run("'"AT""1:"'"'
  printf '%s\n' 'some_code /* Historical: _ac'"1_bar"
} >"$_lf"
_lint_rc=0; _lint_test_naming "$_lf" >/dev/null 2>&1 || _lint_rc=$?
[ "$_lint_rc" -eq 0 ] && _ok "lint: historical comments ignored" \
  || _fail "lint: historical comments incorrectly flagged"

# lint-respects-callsite-boundaries
_lf="$_TMPD/lint-boundary.sh"
{
  printf '%s\n' 'report_ok "'"AT""1: behavior"'"'
  printf '%s\n' 'not.Run("'"AC""1: behavior"'",'
} >"$_lf"
_lint_rc=0; _lint_test_naming "$_lf" >/dev/null 2>&1 || _lint_rc=$?
[ "$_lint_rc" -eq 0 ] && _ok "lint: callsite boundaries respected" \
  || _fail "lint: boundary false positive"

# lint-handles-spaced-file-path through _collect_test_files
_space_root="$_TMPD/lint root with spaces"
_lf="$_space_root/sub dir/fixture_test.go"
mkdir -p "$_space_root/sub dir"
printf '%s\n' "_at""1_foo=1" >"$_lf"
_lint_files_sp=()
while IFS= read -r -d '' _lp; do _lint_files_sp+=("$_lp"); done \
  < <(_collect_test_files "$_space_root")
_lint_rc=0; _lint_out=$(_lint_test_naming "${_lint_files_sp[@]}" 2>&1) || _lint_rc=$?
if [ "${#_lint_files_sp[@]}" -eq 1 ] && [ "${_lint_files_sp[0]}" = "$_lf" ] \
  && [ "$_lint_rc" -ne 0 ] && printf '%s\n' "$_lint_out" | grep -Fq "$_lf:"; then
  _ok "lint: spaced path collected and diagnosed intact"
else
  _fail "lint: spaced path collection/diagnostic failed"
fi

# lint-handles-newline-file-path
_nl_dir="$_TMPD/lint-nl"
mkdir -p "$_nl_dir"
_nl_file="$_nl_dir/foo"$'\n'"bar_test.go"
printf '%s\n' "_at""1_foo=1" >"$_nl_file"
_lint_files_nl=()
while IFS= read -r -d '' _lp; do _lint_files_nl+=("$_lp"); done \
  < <(_collect_test_files "$_nl_dir")
_lint_rc=0; _lint_test_naming "${_lint_files_nl[@]}" >/dev/null 2>&1 || _lint_rc=$?
[ "${#_lint_files_nl[@]}" -eq 1 ] && [ "${_lint_files_nl[0]}" = "$_nl_file" ] \
  && [ "$_lint_rc" -ne 0 ] && _ok "lint: newline-in-path collected as one argument" \
  || _fail "lint: newline-in-path collection failed"

# lint-fallback-discovers-go-tests
_fb_dir="$_TMPD/lint-fb"
mkdir -p "$_fb_dir/sub"
printf '%s\n' 'package foo' >"$_fb_dir/sub/example_test.go"
_fb_files=()
_find_bin=$(command -v find)
_fb_path="$_TMPD/find-only"
mkdir -p "$_fb_path"
ln -s "$_find_bin" "$_fb_path/find"
while IFS= read -r -d '' _lp; do _fb_files+=("$_lp"); done \
  < <(PATH="$_fb_path" _collect_test_files "$_fb_dir")
[ "${#_fb_files[@]}" -eq 1 ] && [ "${_fb_files[0]}" = "$_fb_dir/sub/example_test.go" ] \
  && _ok "lint: find fallback discovers go tests" \
  || _fail "lint: find fallback failed"

# The scanner also falls back to grep when rg is unavailable.
for _fb_cmd in grep sed; do
  _fb_bin=$(command -v "$_fb_cmd")
  ln -s "$_fb_bin" "$_fb_path/$_fb_cmd"
done
_lf="$_TMPD/lint-grep-fallback.sh"
printf '%s\n' "_ac""1_fallback=1" >"$_lf"
_lint_rc=0
PATH="$_fb_path" _lint_test_naming "$_lf" >/dev/null 2>&1 || _lint_rc=$?
[ "$_lint_rc" -ne 0 ] && _ok "lint: grep fallback detects violations" \
  || _fail "lint: grep fallback missed violation"

# lint-passes-legitimate-ac-fixture
_lf="$_TMPD/lint-legit.sh"
printf '%s\n' '_prep_parse_ac_refs "'"AC""42: do stuff"'"' >"$_lf"
_lint_rc=0; _lint_test_naming "$_lf" >/dev/null 2>&1 || _lint_rc=$?
[ "$_lint_rc" -eq 0 ] && _ok "lint: legitimate ac fixture passes" \
  || _fail "lint: legitimate ac fixture incorrectly flagged"

# rule-exact-wording-and-mirrors
_rule_behavior='- Name test identifiers, output labels, comments, and errors by behavior.'
_rule_reserve='- Reserve AC, AT, Class, Part, and Round numbers for CHANGELOG rows, commit messages, and `Historical:` comments.'
_rules_ok=1
for _rule_file in \
  "$_ROOT/AGENTS.md" \
  "$_ROOT/internal/templates/base/AGENTS.md" \
  "$_ROOT/internal/templates/overlays/doc/files/AGENTS.md.tmpl"; do
  grep -Fqx -- "$_rule_behavior" "$_rule_file" || _rules_ok=0
  grep -Fqx -- "$_rule_reserve" "$_rule_file" || _rules_ok=0
done
[ "$_rules_ok" -eq 1 ] && _ok "rules: exact wording present in all mirrors" \
  || _fail "rules: exact wording or mirror missing"

# lint-passes-with-no-violations: exercised by the full build (tests/run.sh itself
# is clean; build.sh scans it as part of the build step).

# ── Summary ───────────────────────────────────────────────────────────────────
printf '\ntests/run.sh: pass=%d fail=%d\n' "$_pass" "$_fail"
[ "$_fail" -eq 0 ] || exit 1
