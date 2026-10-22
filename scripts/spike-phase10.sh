#!/bin/bash
#
# spike-phase10.sh: the Phase 10 live spike (ROADMAP Phase 10 "Research: Yes,
# spike first"). A human runs it against one team in their own org. It is
# read-only: GETs, plus POSTs that only read (WIQL queries, workitemsbatch).
# Nothing is created, updated or deleted.
#
# It answers:
#   Q1  Does WIQL accept [System.IterationId] = N, live and with ASOF, and does
#       it return the same set as the [System.IterationPath] equivalent?
#   Q2  Can workitemsbatch take "fields" and "$expand" together? Does it take
#       "asOf"? How does it report an id it cannot return?
#   Q3  Are pull-request links readable from relations in one batch call
#       (no N+1)?
#   Q4  What does $timeframe=current return, how many current iterations
#       exist, and is today inside a gap between iterations?
#   Q5  What shape do the team's area-path field values have?
#
# Output rules: the PAT never appears in argv (it is passed to curl through a
# 0600 header file), and nothing prints work item titles, descriptions or
# other content: only status codes, counts, field reference names and dates.
#
# Usage:
#   SPIKE_TEAM="Project/Team" scripts/spike-phase10.sh
# The PAT and org URL come from ADO_PAT / ADO_ORG_URL, or from
# ~/.config/adoBizInt/config.yaml. Requires curl and jq. bash 3.2 compatible.

set -u
set -o pipefail

team_arg=${SPIKE_TEAM:-}
case "$team_arg" in
    */*) ;;
    *) echo "set SPIKE_TEAM=Project/Team"; exit 1 ;;
esac
project=${team_arg%%/*}
team=${team_arg#*/}

cfg="$HOME/.config/adoBizInt/config.yaml"
yaml_value() {
    [ -f "$cfg" ] || return 0
    awk -v k="$1" '$1 == k":" { $1 = ""; sub(/^ +/, ""); gsub(/^["'\'']|["'\'']$/, ""); print; exit }' "$cfg"
}
pat=${ADO_PAT:-$(yaml_value pat)}
org=${ADO_ORG_URL:-$(yaml_value org_url)}
org=${org%/}
if [ -z "$pat" ] || [ -z "$org" ]; then
    echo "no PAT or org_url (set ADO_PAT and ADO_ORG_URL, or use the config file)"
    exit 1
fi

tmp=$(mktemp -d) || exit 1
trap 'rm -rf "$tmp"' EXIT
umask 077
printf 'Authorization: Basic %s\n' "$(printf ':%s' "$pat" | base64 | tr -d '\n')" >"$tmp/auth"
unset pat

enc() { jq -rn --arg v "$1" '$v|@uri'; }
P=$(enc "$project")
T=$(enc "$team")
V="api-version=7.1"

# api METHOD URL [JSON_BODY]: response body in $tmp/resp, status in $code.
api() {
    if [ $# -ge 3 ]; then
        code=$(curl -sS -o "$tmp/resp" -w '%{http_code}' -X "$1" -H @"$tmp/auth" \
            -H 'Content-Type: application/json' --data-binary "$3" "$2")
    else
        code=$(curl -sS -o "$tmp/resp" -w '%{http_code}' -X "$1" -H @"$tmp/auth" "$2")
    fi
}
# errmsg prints the server's error type and message, trimmed, for a non-2xx.
errmsg() {
    jq -r '"\(.typeKey // "?"): \(.message // "?")"' <"$tmp/resp" 2>/dev/null | cut -c1-220
}
say() { printf '%s\n' "$*"; }

say "spike: team=$team_arg org=$org"
today=$(date -u +%Y-%m-%d)

# ---- Q4: iterations -------------------------------------------------------
api GET "$org/$P/$T/_apis/work/teamsettings/iterations?$V"
if [ "$code" != 200 ]; then
    say "Q4 all-iterations status=$code $(errmsg)"
    exit 1
fi
cp "$tmp/resp" "$tmp/its"
say "Q4 all-iterations status=200 count=$(jq '.count' <"$tmp/its")"
say "Q4 timeFrame counts: $(jq -c '[.value[].attributes.timeFrame] | group_by(.) | map({(.[0]|tostring): length}) | add' <"$tmp/its")"
say "Q4 iterations with null dates: $(jq '[.value[] | select(.attributes.startDate == null or .attributes.finishDate == null)] | length' <"$tmp/its")"
say "Q4 sample date strings: $(jq -c '[.value[] | select(.attributes.startDate != null)][0].attributes | {startDate, finishDate}' <"$tmp/its")"
in_range=$(jq --arg d "$today" '[.value[] | select(.attributes.startDate != null and .attributes.finishDate != null)
    | select((.attributes.startDate[:10]) <= $d and $d <= (.attributes.finishDate[:10]))] | length' <"$tmp/its")
say "Q4 iterations whose [start, finish] contains today ($today, inclusive): $in_range"

api GET "$org/$P/$T/_apis/work/teamsettings/iterations?\$timeframe=current&$V"
say "Q4 \$timeframe=current status=$code count=$(jq '.count // (.value|length)' <"$tmp/resp" 2>/dev/null)"
[ "$code" = 200 ] || say "Q4 error: $(errmsg)"

# Pick the current iteration, else the latest one that has started.
jq --arg d "$today" '[.value[] | select(.attributes.startDate != null and .attributes.startDate[:10] <= $d)]
    | sort_by(.attributes.startDate) | last' <"$tmp/its" >"$tmp/it"
if [ "$(jq -r 'type' <"$tmp/it")" != object ]; then
    say "no started iteration with dates; Q1-Q3 skipped"
    exit 0
fi
it_path=$(jq -r '.path' <"$tmp/it")
it_start=$(jq -r '.attributes.startDate[:10]' <"$tmp/it")
it_finish=$(jq -r '.attributes.finishDate[:10]' <"$tmp/it")
say "picked iteration: timeFrame=$(jq -r '.attributes.timeFrame' <"$tmp/it") start=$it_start finish=$it_finish"
# An instant one day into the sprint, at midday UTC, for ASOF.
asof=$(jq -rn --arg s "$it_start" '($s + "T12:00:00Z" | fromdateiso8601 + 86400) | todate')
say "asof instant: $asof"

# ---- integer iteration id (classification node) --------------------------
rel=${it_path#*\\}
# The trailing newline matters: read skips a final unterminated line, which
# would silently look up the parent node instead.
rel_enc=$(printf '%s\n' "$rel" | tr '\\' '\n' | while IFS= read -r seg; do enc "$seg"; done | paste -sd/ -)
api GET "$org/$P/_apis/wit/classificationnodes/Iterations/$rel_enc?$V"
if [ "$code" = 200 ]; then
    it_id=$(jq -r '.id' <"$tmp/resp")
    say "iteration node status=200 integer id found=$([ -n "$it_id" ] && echo yes || echo no) node_matches_team_path=$(jq -r --arg p "$it_path" '(.path | sub("^\\\\"; "") | sub("\\\\Iteration\\\\"; "\\")) == $p' <"$tmp/resp")"
else
    it_id=""
    say "iteration node status=$code $(errmsg)"
fi

# ---- Q1: WIQL by id vs path, live and ASOF --------------------------------
path_lit=$(printf '%s' "$it_path" | sed "s/'/''/g")
wiql() { # wiql LABEL WHERE [ASOF]
    local q="SELECT [System.Id] FROM WorkItems WHERE [System.TeamProject] = @project AND $2"
    [ -n "${3:-}" ] && q="$q ASOF '$3'"
    api POST "$org/$P/$T/_apis/wit/wiql?$V" "$(jq -n --arg q "$q" '{query: $q}')"
    if [ "$code" = 200 ]; then
        jq '[.workItems[].id] | sort' <"$tmp/resp" >"$tmp/$1"
        say "Q1 $1 status=200 count=$(jq length <"$tmp/$1") asOf=$(jq -r '.asOf // "none"' <"$tmp/resp")"
    else
        echo '[]' >"$tmp/$1"
        say "Q1 $1 status=$code $(errmsg)"
    fi
}
wiql path_live "[System.IterationPath] = '$path_lit'"
wiql path_asof "[System.IterationPath] = '$path_lit'" "$asof"
if [ -n "$it_id" ]; then
    wiql id_live "[System.IterationId] = $it_id"
    wiql id_asof "[System.IterationId] = $it_id" "$asof"
    say "Q1 id_live == path_live: $(jq -n --slurpfile a "$tmp/id_live" --slurpfile b "$tmp/path_live" '$a == $b')"
    say "Q1 id_asof == path_asof: $(jq -n --slurpfile a "$tmp/id_asof" --slurpfile b "$tmp/path_asof" '$a == $b')"
fi
wiql path_under_live "[System.IterationPath] UNDER '$path_lit'"
say "Q1 states present live (no state filter): $(
    ids=$(jq -c '.[:200]' <"$tmp/path_live")
    if [ "$ids" != "[]" ]; then
        api POST "$org/$P/_apis/wit/workitemsbatch?$V" "$(jq -n --argjson ids "$ids" '{ids: $ids, fields: ["System.State"]}')"
        jq -c '[.value[].fields["System.State"]] | group_by(.) | map({(.[0]): length}) | add' <"$tmp/resp" 2>/dev/null
    else echo none; fi)"

# ---- Q2/Q3: workitemsbatch ------------------------------------------------
ids=$(jq -c '.[:20]' <"$tmp/path_live")
if [ "$ids" = "[]" ]; then
    say "no items in the iteration; Q2-Q3 skipped"
else
    n=$(jq length <<<"$ids")
    f='["System.Id","System.State","System.WorkItemType","Microsoft.VSTS.Scheduling.StoryPoints","Microsoft.VSTS.Scheduling.RemainingWork"]'

    api POST "$org/$P/_apis/wit/workitemsbatch?$V" "$(jq -n --argjson ids "$ids" --argjson f "$f" '{ids: $ids, fields: $f, "$expand": "Relations"}')"
    say "Q2 fields+\$expand=Relations status=$code $([ "$code" = 200 ] && echo "returned=$(jq '.count' <"$tmp/resp") relations_present=$(jq '[.value[] | select(.relations != null)] | length' <"$tmp/resp")" || errmsg)"

    api POST "$org/$P/_apis/wit/workitemsbatch?$V" "$(jq -n --argjson ids "$ids" '{ids: $ids, "$expand": "Relations"}')"
    say "Q2 \$expand=Relations only status=$code requested=$n $([ "$code" = 200 ] && echo "returned=$(jq '.count' <"$tmp/resp")" || errmsg)"
    if [ "$code" = 200 ]; then
        say "Q3 relation types: $(jq -c '[.value[].relations[]?.attributes.name] | group_by(.) | map({(.[0] // "null"): length}) | add' <"$tmp/resp")"
        say "Q3 items with a PR ArtifactLink: $(jq '[.value[] | select([.relations[]? | select(.rel == "ArtifactLink" and (.url | test("PullRequestId"; "i")))] | length > 0)] | length' <"$tmp/resp")"
        say "Q3 PR link url shape: $(jq -r '[.value[].relations[]? | select(.rel == "ArtifactLink" and (.url | test("PullRequestId"; "i"))) | .url | sub("[0-9a-fA-F-]{36}"; "<guid>"; "g") | sub("%2F[0-9]+$"; "%2F<prid>")][0] // "none"' <"$tmp/resp")"
        say "Q3 parent links (System.LinkTypes.Hierarchy-Reverse): $(jq '[.value[].relations[]? | select(.rel == "System.LinkTypes.Hierarchy-Reverse")] | length' <"$tmp/resp")"
    fi

    api POST "$org/$P/_apis/wit/workitemsbatch?$V" "$(jq -n --argjson ids "$ids" --argjson f "$f" --arg a "$asof" '{ids: $ids, fields: $f, asOf: $a}')"
    say "Q2 fields+asOf status=$code $([ "$code" = 200 ] && echo "returned=$(jq '.count' <"$tmp/resp")" || errmsg)"
    if [ "$code" = 200 ]; then
        say "Q2 numeric field types (StoryPoints/RemainingWork): $(jq -c '[.value[].fields | (.["Microsoft.VSTS.Scheduling.StoryPoints"], .["Microsoft.VSTS.Scheduling.RemainingWork"]) | type] | group_by(.) | map({(.[0]): length}) | add' <"$tmp/resp")"
    fi

    bogus=$(jq -c '. + [2147483000]' <<<"$ids")
    api POST "$org/$P/_apis/wit/workitemsbatch?$V" "$(jq -n --argjson ids "$bogus" '{ids: $ids, fields: ["System.Id"], errorPolicy: "Omit"}')"
    say "Q2 missing id, errorPolicy=Omit status=$code $([ "$code" = 200 ] && echo "requested=$((n + 1)) count=$(jq '.count' <"$tmp/resp") nulls=$(jq '[.value[] | select(. == null)] | length' <"$tmp/resp")" || errmsg)"
    api POST "$org/$P/_apis/wit/workitemsbatch?$V" "$(jq -n --argjson ids "$bogus" '{ids: $ids, fields: ["System.Id"]}')"
    say "Q2 missing id, default policy status=$code $([ "$code" = 200 ] && echo "count=$(jq '.count' <"$tmp/resp")" || errmsg)"
fi

# ---- Q5: team area paths --------------------------------------------------
api GET "$org/$P/$T/_apis/work/teamsettings/teamfieldvalues?$V"
if [ "$code" = 200 ]; then
    say "Q5 teamfieldvalues status=200 field=$(jq -r '.field.referenceName' <"$tmp/resp") values=$(jq '.values | length' <"$tmp/resp") includeChildren=$(jq -c '[.values[].includeChildren] | group_by(.) | map({(.[0]|tostring): length}) | add' <"$tmp/resp") default_in_values=$(jq '.defaultValue as $d | [.values[].value] | index($d) != null' <"$tmp/resp")"
else
    say "Q5 teamfieldvalues status=$code $(errmsg)"
fi

say "spike done"
