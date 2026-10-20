#!/bin/bash
#
# smoke.sh: the PLAT-05 live smoke (08-CONTEXT D-14).
#
# A human runs this from a terminal against the org in their own ADO config
# (env vars or ~/.config/adoBizInt/config.yaml). It is never run in CI: CI has
# no ADO credentials and must not have any (v1.0 D-17). It performs no writes:
# every write verb runs as a dry run, without the confirm flag, so only the
# pre-read GETs reach ADO (D-13).
#
# What it does:
#   1. Builds a fresh binary into a temporary directory with exactly the go.mod
#      toolchain, and refuses to go on unless `go version -m` shows the go.mod
#      go directive and azuredevops/v7 v7.1.0. A binary found on PATH is never
#      run: older builds there predate the v7 swap (08-RESEARCH Pitfall 12).
#   2. Runs the eight CLI reads with --json, one or two per sub-client with a
#      call site (D-16). A read passes only on exit 0 with .complete == true.
#   3. Checks identity (the location sub-client): the log lines this run
#      appended must not say the current user ID could not be cached.
#   4. Dry-runs the six write verbs with --json. PASS needs exit 0 and
#      .write.outcome == dry_run; exit 4 is NODIFF (the pre-read ran and found
#      nothing to change); a verb whose target variable is unset is SKIP, with
#      the missing variable named, never PASS.
#   5. Prints one row per check, a totals line, and exits 1 when any check
#      failed.
#
# Dry-run targets. Set these in the shell before running; each must exist in
# the org, and SMOKE_PROJECT must be one of the configured projects:
#   SMOKE_PROJECT        the project every dry run is scoped to (--project)
#   SMOKE_WORKITEM_ID    the scratch work item the user designates (D-13)
#   SMOKE_STATE          a state different from the scratch item's current one
#   SMOKE_REPO           the repository of SMOKE_PR_ID (--repo)
#   SMOKE_PR_ID          an open pull request (prs approve, prs complete)
#   SMOKE_APPROVAL_ID    a release approval id (releases approve, reject). Pick
#                        a recent pending one: the pre-read pages through
#                        approvals until it finds the id, so an old one is slow
#                        and can be throttled (08-RESEARCH Pitfall 14)
#   SMOKE_DEFINITION_ID  a pipeline definition id (pipelines queue)
#   SMOKE_BRANCH         the branch the queue dry run names (--branch)
# D-13 requires a dry run of every write verb. A SKIP row therefore closes the
# live smoke only with a waiver the user writes in 08-SMOKE.md, section
# "Dry-run waivers (D-13)"; an unwaived SKIP blocks Phase 9.
#
# Output rules (08-RESEARCH Pitfall 11): no shell xtrace, and nothing echoes the
# environment, the config file or any request header. Rows carry only the check
# name, the exit code and complete or outcome; on a failed check the tool's own
# stderr is shown, which never carries the PAT (TestPATNeverLogged).
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
pass_n=0
nodiff_n=0
skip_n=0

row() {
    printf '%-32s %-6s %s\n' "$1" "$2" "$3" >>"$results"
    case "$2" in
        PASS) pass_n=$((pass_n + 1)) ;;
        NODIFF) nodiff_n=$((nodiff_n + 1)) ;;
        SKIP) skip_n=$((skip_n + 1)) ;;
        FAIL) failures=$((failures + 1)) ;;
    esac
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
    last_rc=$rc
}

# run_dry NAME REQUIRED_VARS -- ARGS... dry-runs one write verb and records a
# row under NAME, always "dry-run <noun> <verb>" (08-08 counts SKIP rows by that
# prefix). An unset or empty variable in REQUIRED_VARS records SKIP naming the
# first one missing, and nothing runs. The confirm flag is never passed.
run_dry() {
    local name="$1" vars="$2" v rc outcome detail note
    shift 2
    if [ "${1:-}" = "--" ]; then
        shift
    fi
    for v in $vars; do
        if [ -z "${!v:-}" ]; then
            row "$name" SKIP "set $v"
            return
        fi
    done
    "$tmp/adoBizInt" "$@" --json >"$tmp/out" 2>"$tmp/err" </dev/null
    rc=$?
    outcome=$(jq -r '.write.outcome' <"$tmp/out" 2>/dev/null)
    [ -n "$outcome" ] || outcome=none
    detail="exit=$rc outcome=$outcome"
    note=$(throttle_note)
    [ -z "$note" ] || detail="$detail; $note"
    if [ "$rc" -eq 0 ] && [ "$outcome" = dry_run ]; then
        row "$name" PASS "$detail"
    elif [ "$rc" -eq 4 ]; then
        row "$name" NODIFF "$detail (pre-read ran, no diff)"
    else
        row "$name" FAIL "$detail"
        show_stderr "$name" "$rc"
    fi
}

# The identity check scans only the log bytes this run appends, so a warning
# left by an earlier run cannot fail this one.
log="$HOME/.config/adoBizInt/adoBizInt.log"
log_before=0
if [ -f "$log" ]; then
    log_before=$(wc -c <"$log" | tr -d ' ')
fi

# Reads, grouped by the sub-client each one exercises (D-16).
run_read workitems list        # workitemtracking
first_rc=$last_rc
run_read dashboard counts      # workitemtracking
run_read prs list              # git
run_read branches list         # git
run_read pipelines runs        # build
run_read pipelines definitions # build
run_read releases list         # release (vsrm host)
run_read releases approvals    # release (vsrm host)

# Identity (location sub-client, GetConnectionData at client construction).
# Exit 0, 1 or 2 means the client was built; exit 3 means startup stopped first.
case "$first_rc" in
    0 | 1 | 2)
        if [ ! -f "$log" ]; then
            row identity FAIL "no log file to check at ~/.config/adoBizInt"
        else
            tail -c +$((log_before + 1)) "$log" >"$tmp/log.new"
            if grep -q 'Could not cache current user ID' "$tmp/log.new"; then
                row identity FAIL "log: Could not cache current user ID"
            else
                row identity PASS "no identity warning logged by this run"
            fi
        fi
        ;;
    *)
        row identity SKIP "client not built"
        ;;
esac

# Dry runs, one per write verb (D-13). Each variable is expanded with a default
# so an unset target reaches run_dry as an empty value instead of tripping set -u.
run_dry "dry-run workitems set-state" "SMOKE_WORKITEM_ID SMOKE_PROJECT SMOKE_STATE" -- \
    workitems set-state "${SMOKE_WORKITEM_ID:-}" --project "${SMOKE_PROJECT:-}" --state "${SMOKE_STATE:-}"
run_dry "dry-run prs approve" "SMOKE_PR_ID SMOKE_PROJECT SMOKE_REPO" -- \
    prs approve "${SMOKE_PR_ID:-}" --project "${SMOKE_PROJECT:-}" --repo "${SMOKE_REPO:-}"
run_dry "dry-run prs complete" "SMOKE_PR_ID SMOKE_PROJECT SMOKE_REPO" -- \
    prs complete "${SMOKE_PR_ID:-}" --project "${SMOKE_PROJECT:-}" --repo "${SMOKE_REPO:-}"
run_dry "dry-run releases approve" "SMOKE_APPROVAL_ID SMOKE_PROJECT" -- \
    releases approve "${SMOKE_APPROVAL_ID:-}" --project "${SMOKE_PROJECT:-}"
run_dry "dry-run releases reject" "SMOKE_APPROVAL_ID SMOKE_PROJECT" -- \
    releases reject "${SMOKE_APPROVAL_ID:-}" --project "${SMOKE_PROJECT:-}"
run_dry "dry-run pipelines queue" "SMOKE_DEFINITION_ID SMOKE_PROJECT SMOKE_BRANCH" -- \
    pipelines queue "${SMOKE_DEFINITION_ID:-}" --project "${SMOKE_PROJECT:-}" --branch "${SMOKE_BRANCH:-}"

echo
printf '%-32s %-6s %s\n' CHECK RESULT DETAIL
cat "$results"
echo
echo "totals: PASS=$pass_n NODIFF=$nodiff_n SKIP=$skip_n FAIL=$failures"

if [ "$failures" -ne 0 ]; then
    exit 1
fi
exit 0
