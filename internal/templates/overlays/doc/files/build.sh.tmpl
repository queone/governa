#!/usr/bin/env bash
# build.sh — self-contained DOC release and release-prep tooling.
#
# Targets Bash 3.2+ (macOS system Bash): no associative arrays, mapfile,
# ${var^^}, or &>>.
#
# Dispatch:
#   ./build.sh                                show usage
#   ./build.sh prep [flags] vX.Y.Z "message" stage a release
#   ./build.sh vX.Y.Z "message"              run the release
set -euo pipefail

_git_err=''
_rel_tag_for_recovery=''
_prep_cl_err=''
_prep_ie_err=''
_prep_ctargets=''

_color_init() {
  _color_on=1
  [ -n "${NO_COLOR:-}" ] && _color_on=0
  [ "${TERM:-}" = dumb ] && _color_on=0
  if [ -n "${GOVERNA_FORCE_TTY:-}" ]; then
    [ "$GOVERNA_FORCE_TTY" = 1 ] || _color_on=0
  elif [ ! -t 1 ]; then
    _color_on=0
  fi
  _color256=0
  case "${COLORTERM:-}" in truecolor | 24bit) _color256=1 ;; esac
  case "${TERM:-}" in *256color*) _color256=1 ;; esac
}

_wrap() {
  if [ "$_color_on" = 1 ] && [ "$_color256" = 1 ]; then
    printf '\033[%sm%s\033[0m' "$1" "$2"
  else
    printf '%s' "$2"
  fi
}

yel7() { _wrap '38;5;227' "$1"; }
grn3() { _wrap '38;5;34' "$1"; }
cya4() { _wrap '38;5;44' "$1"; }
whi5() { _wrap '38;5;231' "$1"; }

bold() {
  if [ "$_color_on" = 1 ] && [ "$_color256" = 1 ]; then
    local reset bold1 s
    reset=$(printf '\033[0m')
    bold1=$(printf '\033[1m')
    s=${1//"$reset"/"$reset$bold1"}
    printf '\033[1m%s\033[0m' "$s"
  else
    printf '%s' "$1"
  fi
}

_emit_usage_line() {
  local flag="$1" desc="$2" col pad
  col=$((2 + ${#flag}))
  if [ "$col" -lt 38 ]; then
    pad=$(printf '%*s' $((38 - col)) '')
  else
    pad='  '
  fi
  printf '  %s%s%s\n' "$flag" "$pad" "$desc"
}

_trim() {
  local s="$1"
  s="${s#"${s%%[![:space:]]*}"}"
  s="${s%"${s##*[![:space:]]}"}"
  printf '%s' "$s"
}

_byte_len() { LC_ALL=C printf '%s' "$1" | LC_ALL=C wc -c | tr -d ' '; }

_go_quote() {
  printf '%s' "$1" | LC_ALL=C awk '
    BEGIN { for (i = 1; i < 256; i++) ord[sprintf("%c", i)] = i; printf "\"" }
    { if (NR > 1) printf "\\n"
      n = length($0)
      for (i = 1; i <= n; i++) {
        c = substr($0, i, 1); b = ord[c]
        if (c == "\\") printf "\\\\"
        else if (c == "\"") printf "\\\""
        else if (b == 7) printf "\\a"
        else if (b == 8) printf "\\b"
        else if (b == 9) printf "\\t"
        else if (b == 10) printf "\\n"
        else if (b == 11) printf "\\v"
        else if (b == 12) printf "\\f"
        else if (b == 13) printf "\\r"
        else if (b < 32 || b == 127) printf "\\x%02x", b
        else printf "%s", c
      }
    }
    END { printf "\"" }
  '
}

build_usage() {
  printf '%s\n' "$(bold "$(whi5 'Usage:')")"
  printf '  %s\n' 'build prep [flags] vX.Y.Z "release message"'
  printf '  %s\n' 'build vX.Y.Z "release message"'
  _emit_usage_line '-h, -?, --help' 'show this help'
}

# ── release ────────────────────────────────────────────────────────────────

rel_usage() {
  printf '%s %s\n' "$(bold "$(whi5 'Usage:')")" 'rel vX.Y.Z "release message"'
  _emit_usage_line '-h, -?, --help' 'show this help'
  printf '\n%s\n' 'Release message must be 80 characters or fewer.'
}

_ensure_git_repo() {
  local out rc=0
  out=$(git rev-parse --is-inside-work-tree 2>&1) || rc=$?
  if [ "$rc" -ne 0 ]; then
    printf 'verify git repo: exit status %d: %s\n' "$rc" "$(_trim "$out")" >&2
    return 1
  fi
  [ "$(_trim "$out")" = true ] || {
    printf 'current directory is not inside a git work tree\n' >&2
    return 1
  }
}

_run_git() {
  local name="$1" rc=0
  shift
  printf '%s %s\n' "$(yel7 'running:')" "$(grn3 "git $*")"
  git "$@" || rc=$?
  if [ "$rc" -ne 0 ]; then
    _git_err="$name failed: exit status $rc"
    return 1
  fi
}

_recovery_error() {
  local step="$1" tag="$2" completed="$3" giterr="$4"
  printf '%s failed: %s\n' "$step" "$giterr"
  [ -z "$completed" ] && return 0
  printf '\ncompleted before failure: %s\n' "$completed"
  case ",$completed," in
  *", git push tag,"*)
    printf '\nrecovery: tag %s was pushed but the branch push failed\n' "$tag"
    printf '  to retry: git push origin\n'
    ;;
  *", git tag,"*)
    printf '\nrecovery: tag %s exists locally but was not pushed\n' "$tag"
    printf '  to retry push: git push origin %s && git push origin\n' "$tag"
    printf '  to remove tag: git tag -d %s\n' "$tag"
    ;;
  esac
}

_rel_step() {
  local name="$1" completed="$2"
  shift 2
  if _run_git "$name" "$@"; then return 0; fi
  _recovery_error "$name" "$_rel_tag_for_recovery" "$completed" "$_git_err" >&2
  return 1
}

rel_run() {
  local tag="$1" message="$2" completed=''
  _rel_tag_for_recovery="$tag"
  _ensure_git_repo || return 1

  printf '%s %s\n' "$(yel7 'release tag:')" "$(grn3 "$tag")"
  printf '%s %s\n' "$(yel7 'release message:')" "$(grn3 "$(_go_quote "$message")")"
  printf '%s %s\n' "$(yel7 'remote:')" "$(cya4 'origin')"
  printf '%s\n' "$(yel7 "$(printf '\nFiles that will be staged (git status):')")"
  _run_git 'git status preview' status --short || {
    printf '%s\n' "$_git_err" >&2
    return 1
  }

  printf '%s\n' "$(yel7 "$(printf '\nplan:')")"
  printf '%s\n' '- git add .'
  printf -- '- git commit -m %s\n' "$(_go_quote "$message")"
  printf -- '- git tag -a %s -m %s\n' "$tag" "$(_go_quote "$message")"
  printf '%s\n' "- git push origin $tag"
  printf '%s\n' '- git push origin'
  printf '%s' "$(yel7 'Review the file list above. Proceed with release? (y/N): ')"

  local answer=''
  IFS= read -r answer || true
  case "$answer" in y | Y) ;; *) printf 'release aborted\n' >&2; return 1 ;; esac

  _rel_step 'git add' "$completed" add . || return 1
  completed='git add'
  _rel_step 'git commit' "$completed" commit -m "$message" || return 1
  completed='git add, git commit'
  _rel_step 'git tag' "$completed" tag -a "$tag" -m "$message" || return 1
  completed='git add, git commit, git tag'
  _rel_step 'git push tag' "$completed" push origin "$tag" || return 1
  completed='git add, git commit, git tag, git push tag'
  _rel_step 'git push branch' "$completed" push origin || return 1
}

# ── release prep ────────────────────────────────────────────────────────────

prep_usage() {
  cat <<'EOF'
prep vX.Y.Z "release message" [--dry-run|-n]

Stages a release by inserting a CHANGELOG row, deleting completed AC files,
sweeping matching plan.md IE entries, and printing the release command.

Flags:
  -h, -?, --help   show this help
  --dry-run, -n    print intended writes without modifying the working tree

Prints the canonical release command on success. Does not run the release.
EOF
}

_prep_validate_git_state() {
  local root="$1" version="$2" out rc=0 latest head ref dirty
  out=$(cd "$root" && git rev-parse --is-inside-work-tree 2>&1) || rc=$?
  if [ "$rc" -ne 0 ]; then
    printf 'prep: verify git repo: exit status %d: %s\n' "$rc" "$(_trim "$out")" >&2
    return 1
  fi
  [ "$(_trim "$out")" = true ] || { printf 'prep: not inside a git work tree\n' >&2; return 1; }
  if (cd "$root" && git rev-parse -q --verify "refs/tags/$version" >/dev/null 2>&1); then
    printf 'prep: tag %s already exists\n' "$version" >&2
    return 1
  fi
  latest=$(cd "$root" && git describe --tags --abbrev=0 2>/dev/null) || return 0
  rc=0; out=$(cd "$root" && git rev-parse HEAD 2>&1) || rc=$?
  if [ "$rc" -ne 0 ]; then printf 'prep: compare HEAD to %s: exit status %d\n' "$latest" "$rc" >&2; return 1; fi
  head="$out"
  rc=0; out=$(cd "$root" && git rev-parse "$latest" 2>&1) || rc=$?
  if [ "$rc" -ne 0 ]; then printf 'prep: compare HEAD to %s: exit status %d\n' "$latest" "$rc" >&2; return 1; fi
  ref="$out"
  [ "$head" != "$ref" ] && return 0
  rc=0; out=$(cd "$root" && git status --porcelain 2>&1) || rc=$?
  if [ "$rc" -ne 0 ]; then printf 'prep: check working tree: exit status %d\n' "$rc" >&2; return 1; fi
  dirty="$out"
  if [ -z "$dirty" ]; then
    printf 'prep: nothing to release: HEAD is at %s and working tree is clean\n' "$latest" >&2
    return 1
  fi
}

_prep_detect_changelog_targets() {
  local root="$1" version="$2" vstripped="${2#v}" marker p out=''
  marker="| $vstripped |"
  _prep_cl_err=''
  _prep_ctargets=''
  for p in "$root/CHANGELOG.md"; do
    [ -f "$p" ] || continue
    if grep -Fq "$marker" "$p"; then
      _prep_cl_err="$p already has a row for $vstripped (prep is not idempotent on CHANGELOG)"
      return 1
    fi
    out="$p"
  done
  _prep_ctargets="$out"
}

_prep_parse_ac_refs() {
  printf '%s' "$1" | grep -oE 'AC[0-9]+' | sed 's/^AC//' | LC_ALL=C sort -n -u || true
}

_prep_find_ac_files() {
  local root="$1" acnums="$2" f name num
  [ -n "$acnums" ] || return 0
  for f in "$root"/governa/ac*.md; do
    [ -f "$f" ] || continue
    name=$(basename "$f")
    [ "$name" = ac-template.md ] && continue
    case "$name" in ac[0-9]*-*.md) num=$(printf '%s' "$name" | sed -E 's/^ac([0-9]+)-.*/\1/') ;; *) continue ;; esac
    if printf '%s\n' "$acnums" | grep -qx "$num"; then printf '%s\n' "$f"; fi
  done | LC_ALL=C sort
}

_prep_find_ie_lines() {
  local root="$1" acnums="$2" line num
  [ -n "$acnums" ] || return 0
  [ -f "$root/plan.md" ] || return 0
  while IFS= read -r line || [ -n "$line" ]; do
    case "$line" in
    *"→ governa/ac"[0-9]*-*) num=$(printf '%s' "$line" | sed -E 's/.*→[[:space:]]+governa\/ac([0-9]+)-.*/\1/') ;;
    *) continue ;;
    esac
    if printf '%s\n' "$acnums" | grep -qx "$num"; then printf '%s\n' "$line"; fi
  done <"$root/plan.md"
}

_prep_apply_changelog_insert() {
  local path="$1" version="$2" message="$3" tmp row
  if ! grep -Eq '^\| Unreleased \|' "$path"; then
    _prep_cl_err="$path has no | Unreleased | row"
    return 1
  fi
  tmp=$(mktemp "${TMPDIR:-/tmp}/prep-cl.XXXXXX")
  row="| $version | $message |"
  if ! row="$row" awk '
    BEGIN { row = ENVIRON["row"] }
    { print }
    !done && /^\| Unreleased \|/ { print row; done = 1 }
  ' "$path" >"$tmp"; then
    rm -f "$tmp"; _prep_cl_err="awk failed on $path"; return 1
  fi
  if ! cat "$tmp" >"$path"; then rm -f "$tmp"; _prep_cl_err="write failed: $path"; return 1; fi
  rm -f "$tmp"
}

_prep_remove_ie_lines() {
  local root="$1" lines="$2" tmp
  [ -n "$lines" ] || return 0
  tmp=$(mktemp "${TMPDIR:-/tmp}/prep-plan.XXXXXX")
  _prep_ie_err=''
  if ! drop="$lines" awk '
    BEGIN { n=split(ENVIRON["drop"], a, "\n"); for (i=1; i<=n; i++) if (a[i] != "") d[a[i]]=1 }
    { if (!($0 in d)) print }
  ' "$root/plan.md" >"$tmp"; then
    rm -f "$tmp"; _prep_ie_err='awk failed on plan.md'; return 1
  fi
  if ! cat "$tmp" >"$root/plan.md"; then rm -f "$tmp"; _prep_ie_err="write failed: $root/plan.md"; return 1; fi
  rm -f "$tmp"
}

_prep_emit_release_command() {
  printf '\nrelease command:\n  ./build.sh %s %s\n' "$1" "$(_go_quote "$2")"
}

_prep_print_dry_run() {
  local ctargets="$1" version="$2" message="$3" acfiles="$4" ielines="$5" p line
  printf '\n--- dry run (no writes) ---\n'
  printf 'CHANGELOG inserts:\n'
  while IFS= read -r p; do [ -n "$p" ] && printf '  %s: | %s | %s |\n' "$p" "$version" "$message"; done <<EOF
$ctargets
EOF
  printf 'AC deletions:\n'
  while IFS= read -r p; do [ -n "$p" ] && printf '  delete %s\n' "$p"; done <<EOF
$acfiles
EOF
  printf 'plan.md AC-pointer IE removals:\n'
  while IFS= read -r line; do [ -n "$line" ] && printf '  remove: %s\n' "$(_trim "$line")"; done <<EOF
$ielines
EOF
  printf -- '--- end dry run ---\n'
}

prep_run() {
  local dry="$1" version="$2" message="$3" root="$PWD" vstripped="${2#v}"
  if ! printf '%s' "$version" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$'; then
    printf 'prep: version must match vMAJOR.MINOR.PATCH: %s\n' "$(_go_quote "$version")" >&2
    return 1
  fi
  if [ -z "$message" ]; then printf 'prep: message must be non-empty\n' >&2; return 1; fi
  if [ "$(_byte_len "$message")" -gt 80 ]; then printf 'prep: message must be 80 characters or fewer\n' >&2; return 1; fi

  _prep_validate_git_state "$root" "$version" || return 1
  _prep_detect_changelog_targets "$root" "$version" || {
    printf 'prep: detect CHANGELOG targets: %s\n' "$_prep_cl_err" >&2
    return 1
  }
  local ctargets="$_prep_ctargets" acnums acfiles ielines
  acnums=$(_prep_parse_ac_refs "$message")
  acfiles=$(_prep_find_ac_files "$root" "$acnums")
  ielines=$(_prep_find_ie_lines "$root" "$acnums")

  if [ "$dry" -eq 1 ]; then
    _prep_print_dry_run "$ctargets" "$vstripped" "$message" "$acfiles" "$ielines"
    _prep_emit_release_command "$version" "$message"
    return 0
  fi

  local path line
  while IFS= read -r path; do
    [ -n "$path" ] || continue
    _prep_apply_changelog_insert "$path" "$vstripped" "$message" || {
      printf 'prep: insert CHANGELOG row in %s: %s\n' "$path" "$_prep_cl_err" >&2
      return 1
    }
  done <<EOF
$ctargets
EOF
  while IFS= read -r path; do
    [ -n "$path" ] || continue
    rm -- "$path" || { printf 'prep: delete %s: failed\n' "$path" >&2; return 1; }
    printf 'prep: deleted %s\n' "$path"
  done <<EOF
$acfiles
EOF
  _prep_remove_ie_lines "$root" "$ielines" || {
    printf 'prep: sweep plan.md AC-pointer IEs: %s\n' "$_prep_ie_err" >&2
    return 1
  }
  while IFS= read -r line; do
    [ -n "$line" ] && printf 'prep: removed plan.md IE line: %s\n' "$(_trim "$line")"
  done <<EOF
$ielines
EOF

  _prep_emit_release_command "$version" "$message"
}

# ── dispatch ────────────────────────────────────────────────────────────────

build_main() {
  if [ "$#" -eq 0 ]; then build_usage; return 0; fi
  if [ "$#" -eq 1 ]; then
    case "$1" in -h | -\? | --help) build_usage; return 0 ;; esac
  fi
  printf 'usage: build\n' >&2
  return 2
}

rel_main() {
  if [ "$#" -ne 2 ]; then printf 'usage: rel vX.Y.Z "release message"\n' >&2; return 2; fi
  local tag message
  tag=$(_trim "$1")
  message=$(_trim "$2")
  if ! printf '%s' "$tag" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$'; then
    printf 'release tag must match vMAJOR.MINOR.PATCH: %s\n' "$(_go_quote "$tag")" >&2
    return 2
  fi
  if [ -z "$message" ]; then printf 'release message must be non-empty\n' >&2; return 2; fi
  if [ "$(_byte_len "$message")" -gt 80 ]; then printf 'release message must be 80 characters or fewer\n' >&2; return 2; fi
  rel_run "$tag" "$message"
}

prep_main() {
  if [ "$#" -eq 0 ]; then prep_usage; return 0; fi
  if [ "$#" -eq 1 ]; then case "$1" in -h | -\? | --help) prep_usage; return 0 ;; esac; fi
  local dry=0 arg version message
  local positional=()
  for arg in "$@"; do
    case "$arg" in
    -h | -\? | --help) printf 'help flags must be used by themselves\n' >&2; return 2 ;;
    --dry-run | -n) dry=1 ;;
    -*) printf 'unsupported option %s; use -h, -?, --help, --dry-run, or -n\n' "$(_go_quote "$arg")" >&2; return 2 ;;
    *) positional+=("$arg") ;;
    esac
  done
  if [ "${#positional[@]}" -ne 2 ]; then
    printf 'usage: prep vX.Y.Z "release message" [--dry-run|-n]\n' >&2
    return 2
  fi
  version=$(_trim "${positional[0]}")
  message=$(_trim "${positional[1]}")
  prep_run "$dry" "$version" "$message"
}

main() {
  _color_init
  if [ "${1:-}" = prep ]; then shift; prep_main "$@"; return $?; fi
  if [ "$#" -ge 1 ] && printf '%s' "$1" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$'; then rel_main "$@"; return $?; fi
  build_main "$@"
}

if [ "${BASH_SOURCE[0]}" = "$0" ]; then
  main "$@"
fi
