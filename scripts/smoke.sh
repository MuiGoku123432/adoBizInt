#!/bin/bash
#
# smoke.sh: the PLAT-05 live smoke (08-CONTEXT D-14).
#
# A human runs this from a terminal against the org in their own ADO config
# (env vars or ~/.config/adoBizInt/config.yaml). It is never run in CI: CI has
# no ADO credentials and must not have any (v1.0 D-17). It performs no writes.
#
# What it does:
#   1. Builds a fresh binary into a temporary directory with exactly the go.mod
#      toolchain, and refuses to go on unless `go version -m` shows the go.mod
#      go directive and azuredevops/v7 v7.1.0. A binary found on PATH is never
#      run: older builds there predate the v7 swap (08-RESEARCH Pitfall 12).
#   2. Runs CLI reads with --json. A read passes only on exit 0 with
#      .complete == true.
#   3. Prints one row per check and exits 1 when any check failed.
#
# Output rules (08-RESEARCH Pitfall 11): no shell xtrace, and nothing echoes the
# environment, the config file or any request header. Rows carry only the check
# name, the exit code and complete; on a failed check the tool's own stderr is
# shown, which never carries the PAT (TestPATNeverLogged).
#
# Usage, from anywhere inside the repository:
#   scripts/smoke.sh
#
# Requires: go and jq on PATH. bash 3.2 compatible (the macOS /bin/bash).

set -u
set -o pipefail

cd "$(dirname "$0")/.." || exit 1

if ! command -v jq >/dev/null 2>&1; then
    echo "FAIL jq: jq is required to read the JSON envelopes"
    exit 1
fi

tmp=$(mktemp -d) || exit 1
trap 'rm -rf "$tmp"' EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

# Build with exactly the go.mod toolchain, as CI's go-version-file does. Under
# the default GOTOOLCHAIN=auto a newer local Go would build the binary, and the
# provenance gate below would then fail a correct v7 build.
want_go=$(awk '/^go / { print $2; exit }' go.mod)
if [ -z "$want_go" ]; then
    echo "FAIL build: no go directive found in go.mod"
    exit 1
fi
if ! GOTOOLCHAIN=go${want_go} go build -o "$tmp/adoBizInt" ./cmd/adoBizInt 2>"$tmp/build.err"; then
    echo "FAIL build: could not build ./cmd/adoBizInt with toolchain go${want_go}"
    sed 's/^/    /' "$tmp/build.err"
    exit 1
fi

# Provenance gate (Pitfall 12): stop before any ADO call unless the fresh binary
# was built by the go.mod toolchain and links azuredevops/v7 v7.1.0. The two
# lines printed here are what goes into 08-SMOKE.md "Binary provenance".
if ! go version -m "$tmp/adoBizInt" >"$tmp/version" 2>/dev/null; then
    echo "FAIL provenance: go version -m could not read the fresh binary"
    exit 1
fi
first=$(head -n 1 "$tmp/version")
first=${first##*/}
sdk=$(grep -E 'azuredevops/v7[[:space:]]+v7\.1\.0([[:space:]]|$)' "$tmp/version" | head -n 1 | tr '\t' ' ' | tr -s ' ' | sed 's/^ //')
if ! printf '%s\n' "$first" | awk -v w="go$want_go" '{ exit !($2 == w) }' || [ -z "$sdk" ]; then
    echo "FAIL provenance: want go${want_go} and azuredevops/v7 v7.1.0"
    echo "    found: $first"
    grep 'azure-devops-go-api' "$tmp/version" | tr '\t' ' ' | tr -s ' ' | sed 's/^ */    found: /'
    exit 1
fi
echo "provenance: $first"
echo "provenance: $sdk"

# Rows are kept in a file and printed once, as the final table. Every table
# line is formatted by the same printf, so columns stay aligned for CHECK names
# up to 32 characters and are separated by spaces, never tabs.
results="$tmp/results"
: >"$results"
failures=0

row() {
    printf '%-32s %-6s %s\n' "$1" "$2" "$3" >>"$results"
    if [ "$2" = FAIL ]; then
        failures=$((failures + 1))
    fi
}

# show_stderr prints the tool's own stderr for a failed check, indented so it
# can never be read as a table row.
show_stderr() {
    echo "-- $1 stderr (exit=$2):"
    grep -v '^[[:space:]]*$' "$tmp/err" | sed 's/^/    /'
}

# throttle_note returns any "throttled by ADO:" summary line the tool wrote
# (D-05), so a paced run leaves its evidence in the row detail (Pitfall 14).
throttle_note() {
    grep '^throttled by ADO:' "$tmp/err" | tr '\n' ' ' | sed 's/ *$//'
}

# run_read NOUN VERB runs one CLI read with --json and records a row named
# "NOUN VERB". It passes only on exit 0 with .complete == true.
run_read() {
    local name="$1 $2" rc complete detail note
    "$tmp/adoBizInt" "$1" "$2" --json >"$tmp/out" 2>"$tmp/err" </dev/null
    rc=$?
    complete=$(jq -r '.complete' <"$tmp/out" 2>/dev/null)
    [ -n "$complete" ] || complete=none
    detail="exit=$rc complete=$complete"
    note=$(throttle_note)
    [ -z "$note" ] || detail="$detail; $note"
    if [ "$rc" -eq 0 ] && [ "$complete" = true ]; then
        row "$name" PASS "$detail"
    else
        row "$name" FAIL "$detail"
        show_stderr "$name" "$rc"
    fi
}

run_read workitems list

echo
printf '%-32s %-6s %s\n' CHECK RESULT DETAIL
cat "$results"

if [ "$failures" -ne 0 ]; then
    exit 1
fi
exit 0
