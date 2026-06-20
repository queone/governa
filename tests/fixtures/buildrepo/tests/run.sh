#!/usr/bin/env bash
# Minimal test runner for the buildrepo fixture.
# Supports the AT14 force-failure hook only; not a real test suite.
if [ -n "${GOVERNA_SELFTEST_FORCE_FAIL:-}" ]; then
  printf 'tests/run.sh: forced failure (GOVERNA_SELFTEST_FORCE_FAIL set)\n' >&2
  exit 1
fi
printf 'tests/run.sh: pass=0 fail=0\n'
