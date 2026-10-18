#!/usr/bin/env bash
# Reference-stack self-test: every client adapter against every server adapter,
# with CALLs from both ends of the connection.
#
# Usage: interop.sh <adapter> [<adapter> ...]      e.g. interop.sh mobilityhouse chargetime
#
# For each OCPP version both adapters declare (1.6 for now), and each pairing
# of a station (`client run`, or a one-shot `client <Action>`) and a CSMS
# (`server`), on a private Docker network:
#
#   from-station   every station sample of the fixture is sent by the station
#                  (`docker exec <station> ocpp-interop call ...`); the result
#                  must be the CSMS answer the fixture defines (docs/FIXTURES.md),
#                  and the CSMS must report the CALL it received;
#   from-csms      every CSMS sample is sent by the CSMS to the connected station
#                  (`docker exec <csms> ocpp-interop call --station ...`), with
#                  the station's fixture answer;
#   connectivity   one-shot client, an unknown identity, the close of a session;
#   profile 1      Basic authentication: accepted, no credentials, wrong password;
#   errors         NotImplemented for an action without an answer, the fixture's
#                  own CALLERROR answers.
#
# Every result is a report result (schemas/report.schema.json) with the
# status PASS, FAIL, SKIP_UNSUPPORTED (a capability one side does not declare),
# KNOWN_ISSUE (a failure that tests/known-bugs.tsv explains) or INFRA_ERROR.
# Known bugs that no longer reproduce fail the run.
#
# This tests the reference adapters against each other. It says nothing about
# any library that consumes the images.
#
# Environment: REGISTRY, VERSION (image tag, default dev), VERBOSE=1,
#              REPORT_DIR (keeps every result document and the report).

set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
# shellcheck source=scripts/lib.sh
source "${SCRIPT_DIR}/lib.sh"

[[ $# -ge 1 ]] || { echo "usage: interop.sh <adapter> [<adapter> ...]" >&2; exit 2; }
ADAPTERS=("$@")
OCPP="1.6"
FIXTURE="${ROOT}/fixtures/v16/baseline/fixture.json"
KNOWN_BUGS="${ROOT}/tests/known-bugs.tsv"
STATION=$(jq -r '.stations[0].id' "${FIXTURE}")
require_tools
WORK=$(mktemp -d)
trap 'rm -rf "${WORK}"; cleanup' EXIT
network_up
STARTED=$(date -u +%Y-%m-%dT%H:%M:%SZ)
: > "${WORK}/results.jsonl"

now_ms() {
    if [[ -n "${EPOCHREALTIME:-}" ]]; then local t="${EPOCHREALTIME/[.,]/}"; echo $((t / 1000)); else echo $(($(date +%s) * 1000)); fi
}

# ---- capabilities, upstream versions and the known bugs that apply ----
# ${WORK}/active: "<adapter>\t<role>\t<test id regex>\t<evidence>\t<summary>"
: > "${WORK}/active"
: > "${WORK}/reproduced"
for adapter in "${ADAPTERS[@]}"; do
    docker run --rm "$(image_of "${adapter}")" print-capabilities > "${WORK}/caps-${adapter}.json" 2>/dev/null \
        || { log "${adapter}: print-capabilities failed"; exit 1; }
    upstream=$(jq -r '.upstream.version' "${WORK}/caps-${adapter}.json")
    log "${adapter}: upstream ${upstream}"
    while IFS=$'\t' read -r bug_adapter bug_version bug_regex bug_evidence bug_text; do
        [[ -z "${bug_adapter}" || "${bug_adapter}" == \#* ]] && continue
        bug_role="any"
        [[ "${bug_adapter}" == *:* ]] && { bug_role="${bug_adapter#*:}"; bug_adapter="${bug_adapter%%:*}"; }
        if [[ "${bug_adapter}" == "${adapter}" && "${bug_version}" == "${upstream}" ]]; then
            printf '%s\t%s\t%s\t%s\t%s\n' "${bug_adapter}" "${bug_role}" "${bug_regex}" "${bug_evidence}" "${bug_text}" >> "${WORK}/active"
            log "known bug in ${adapter} ${upstream} (${bug_role}): ${bug_text}"
        fi
    done < "${KNOWN_BUGS}"
done

caps() { cat "${WORK}/caps-$1.json"; }
upstream_of() { jq -r '.upstream.version' "${WORK}/caps-$1.json"; }
# declares <adapter> <jq condition on .versions["1.6"]>
declares() { jq -e --arg v "${OCPP}" ".versions[\$v] // {} | $2" "${WORK}/caps-$1.json" >/dev/null 2>&1; }
sends() { declares "$1" ".$2.sends // [] | index(\"$3\") != null"; }
handles() { declares "$1" ".$2.handles // [] | index(\"$3\") != null"; }
feature() { declares "$1" ".features.$2 == true"; }
profile() { declares "$1" ".features.securityProfiles // [] | index($2) != null"; }

# known_bug <client> <server> <test id>: "<evidence>\t<summary>" of a known
# bug that explains a failure of this test in this pairing, if there is one.
known_bug() {
    local bug_adapter bug_role bug_regex bug_evidence bug_text
    while IFS=$'\t' read -r bug_adapter bug_role bug_regex bug_evidence bug_text; do
        if { [[ "${bug_role}" != server && "${bug_adapter}" == "$1" ]] || [[ "${bug_role}" != client && "${bug_adapter}" == "$2" ]]; } \
            && [[ "$3" =~ ${bug_regex} ]]; then
            printf '%s\t%s\n' "${bug_evidence}" "${bug_text}"
            return 0
        fi
    done < "${WORK}/active"
    return 1
}

FEATURE_OF='{
  "BootNotification":"provisioning","Heartbeat":"provisioning","StatusNotification":"provisioning",
  "Authorize":"authorization","StartTransaction":"transactions","StopTransaction":"transactions",
  "MeterValues":"meter-values","DataTransfer":"data-transfer",
  "DiagnosticsStatusNotification":"firmware-diagnostics","FirmwareStatusNotification":"firmware-diagnostics",
  "UpdateFirmware":"firmware-diagnostics","GetDiagnostics":"firmware-diagnostics",
  "RemoteStartTransaction":"remote-commands","RemoteStopTransaction":"remote-commands","Reset":"remote-commands",
  "UnlockConnector":"remote-commands","ChangeAvailability":"remote-commands","TriggerMessage":"remote-commands",
  "ClearCache":"remote-commands","GetConfiguration":"configuration","ChangeConfiguration":"configuration",
  "SetChargingProfile":"smart-charging","ClearChargingProfile":"smart-charging","GetCompositeSchedule":"smart-charging",
  "ReserveNow":"reservation","CancelReservation":"reservation",
  "GetLocalListVersion":"local-auth-list","SendLocalList":"local-auth-list",
  "SecurityEventNotification":"security-extensions","ExtendedTriggerMessage":"security-extensions"
}'

impl() {  # impl <adapter>: the implementation object of a report result
    jq -cn --arg n "$1" --arg v "$(upstream_of "$1")" --arg i "$(image_of "$1")" \
        --arg id "$(docker image inspect --format '{{.Id}}' "$(image_of "$1")" 2>/dev/null)" \
        '{name: $n, version: $v, image: $i, imageId: $id}'
}

keep() {  # keep <label> <document>
    [[ -n "${REPORT_DIR:-}" && -n "$2" ]] || return 0
    mkdir -p "${REPORT_DIR}"
    jq -e 'type == "object"' <<<"$2" >/dev/null 2>&1 || return 0
    printf '%s\n' "$2" > "${REPORT_DIR}/result-$(echo "$1" | tr -c 'A-Za-z0-9._\n-' '_').json"
}

# record <test id> <client> <server> <initiator> <feature> <action> <status> <duration ms> [<error or evidence>]
# A FAIL that a known bug explains becomes KNOWN_ISSUE.
record() {
    local id="$1" client="$2" server="$3" initiator="$4" feat="$5" action="$6" status="$7" ms="$8" note="${9:-}"
    local bug evidence=""
    if [[ "${status}" == FAIL ]] && bug=$(known_bug "${client}" "${server}" "${id}"); then
        status=KNOWN_ISSUE; evidence="${bug%%$'\t'*}"
        echo "${bug#*$'\t'}" >> "${WORK}/reproduced"
        KNOWN=$((KNOWN + 1))
        [[ -n "${VERBOSE:-}" ]] && log "known bug: ${client}->${server}: ${id}"
    fi
    case "${status}" in
        PASS) ok "${client}->${server}: ${id}" ;;
        FAIL|INFRA_ERROR) fail "${client}->${server}: ${id}: ${note}" ;;
        SKIP_UNSUPPORTED) SKIPPED=$((SKIPPED + 1)); evidence="${note}"; note="" ;;
    esac
    jq -cn --arg id "${id}" --arg v "${OCPP}" --argjson c "$(impl "${client}")" --argjson s "$(impl "${server}")" \
        --arg i "${initiator}" --arg f "${feat}" --arg a "${action}" --arg st "${status}" --argjson ms "${ms}" \
        --arg err "${note}" --arg ev "${evidence}" '
        {testId: $id, version: $v, client: $c, server: $s, initiator: $i, feature: $f, status: $st, durationMs: $ms}
        + (if $a != "" then {action: $a} else {} end)
        + (if ($st == "FAIL" or $st == "INFRA_ERROR" or $st == "KNOWN_ISSUE") and $err != "" then {error: $err} else {} end)
        + (if $ev != "" then {evidence: $ev} else {} end)' >> "${WORK}/results.jsonl"
}

kebab() { sed -E 's/([a-z0-9])([A-Z])/\1-\2/g' <<<"$1" | tr '[:upper:]' '[:lower:]'; }

# expected <answering role> <action> <request JSON>: what the fixture says the
# answer is, as {"response": {...}} or {"callError": "<code>"}.
expected() {
    jq -cn --slurpfile fx "${FIXTURE}" --arg role "$1" --arg action "$2" --argjson req "$3" '
        $fx[0] as $f | $f.answers[$role][$action] as $a
        | if $a == null then {callError: "NotImplemented"}
          elif $a.callError then {callError: $a.callError.code}
          elif $a.derived == "idTokens" then
            {response: ({idTagInfo: {status: (first($f.idTokens[] | select(.idToken == $req.idTag) | .status) // "Invalid")}}
                        + ($a.payload // {}))}
          elif $a.derived == "configuration" then
            (if ($req.key // []) == [] then [$f.configuration[].key] else $req.key end) as $keys
            | [$keys[] as $k | first($f.configuration[] | select((.key | ascii_downcase) == ($k | ascii_downcase)))
                | {key, readonly, value}] as $known
            | [$keys[] as $k | select([$f.configuration[] | select((.key | ascii_downcase) == ($k | ascii_downcase))] | length == 0) | $k] as $unknown
            | {response: ({configurationKey: $known} + (if $unknown == [] then {} else {unknownKey: $unknown} end))}
          else {response: $a.payload} end'
}

# exchange <id> <client> <server> <initiator> <sender container> <receiver container> <action> <payload> [call flags...]
# One CALL through the control socket of the sender, checked against the
# fixture and against the events of both ends.
exchange() {
    local id="$1" client="$2" server="$3" initiator="$4" sender="$5" receiver="$6" action="$7" payload="$8"; shift 8
    local answering sender_adapter receiver_adapter feat want doc code t0 ms problems=()
    if [[ "${initiator}" == client ]]; then
        answering=csms; sender_adapter="${client}"; receiver_adapter="${server}"
    else
        answering=station; sender_adapter="${server}"; receiver_adapter="${client}"
    fi
    feat=$(jq -r --arg a "${action}" '.[$a] // "other"' <<<"${FEATURE_OF}")
    want=$(expected "${answering}" "${action}" "${payload}")
    local sender_role receiver_role
    sender_role=$([[ "${initiator}" == client ]] && echo client || echo server)
    receiver_role=$([[ "${initiator}" == client ]] && echo server || echo client)
    if ! sends "${sender_adapter}" "${sender_role}" "${action}"; then
        record "${id}" "${client}" "${server}" "${initiator}" "${feat}" "${action}" SKIP_UNSUPPORTED 0 \
            "${sender_adapter} does not declare that its ${sender_role} sends ${action} in OCPP ${OCPP}"
        return
    fi
    if jq -e '.response' <<<"${want}" >/dev/null && ! handles "${receiver_adapter}" "${receiver_role}" "${action}"; then
        record "${id}" "${client}" "${server}" "${initiator}" "${feat}" "${action}" SKIP_UNSUPPORTED 0 \
            "${receiver_adapter} does not declare that its ${receiver_role} handles ${action} in OCPP ${OCPP}"
        return
    fi
    t0=$(now_ms)
    doc=$(docker exec "${sender}" ocpp-interop call "$@" "${action}" --payload "${payload}" 2>/dev/null); code=$?
    ms=$(( $(now_ms) - t0 ))
    keep "${id}-${client}-${server}" "${doc}"

    if ! jq -e --arg a "${action}" '.operation == $a and .version == "1.6"' <<<"${doc}" >/dev/null 2>&1; then
        problems+=("exit ${code}, no result document: ${doc:0:300}")
    elif jq -e '.response' <<<"${want}" >/dev/null; then
        [[ "${code}" == 0 ]] || problems+=("exit ${code}, want 0")
        jq -e '.ok == true and .error == null' <<<"${doc}" >/dev/null || problems+=("not ok: $(jq -c '{error, callError}' <<<"${doc}")")
        jq -e --argjson w "${want}" "${JQ_LIB}"'within($w.response; .response)' <<<"${doc}" >/dev/null \
            || problems+=("response $(jq -c .response <<<"${doc}") is not the fixture answer $(jq -c .response <<<"${want}")")
    else
        [[ "${code}" == 1 ]] || problems+=("exit ${code}, want 1")
        jq -e --argjson w "${want}" '.ok == false and .error.code == "call-error" and .callError.code == $w.callError' <<<"${doc}" >/dev/null \
            || problems+=("want CALLERROR $(jq -r .callError <<<"${want}"), got $(jq -c '{error, callError}' <<<"${doc}" 2>/dev/null)")
    fi
    if [[ ${#problems[@]} -eq 0 ]]; then
        jq -e --argjson p "${payload}" "${JQ_LIB}"'within($p; .request)' <<<"${doc}" >/dev/null \
            || problems+=("the request on the wire $(jq -c .request <<<"${doc}") lost fields of $(jq -c . <<<"${payload}")")
    fi
    if [[ ${#problems[@]} -eq 0 ]]; then
        local request message_id cond
        request=$(jq -c '.request' <<<"${doc}")
        message_id=$(jq -c '.messageId // null' <<<"${doc}")
        cond="${JQ_LIB}"'.event == "call-received" and .station == $s and .action == $a and within($r; .payload)
              and ($m == null or .messageId == null or .messageId == $m)'
        # A CALL the upstream answers with NotImplemented on its own may never
        # reach the adapter (docs/CONTAINER_CONTRACT.md, Events).
        if [[ "$(jq -r '.callError // ""' <<<"${want}")" != NotImplemented ]]; then
            wait_event "${receiver}" "${cond}" --arg s "${STATION}" --arg a "${action}" --argjson r "${request}" --argjson m "${message_id}" >/dev/null \
                || problems+=("${receiver_adapter} reported no call-received event for this CALL")
        fi
        if jq -e '.response' <<<"${want}" >/dev/null; then
            cond="${JQ_LIB}"'.event == "call-answered" and .station == $s and .action == $a and within($w; .payload)
                  and ($m == null or .messageId == null or .messageId == $m)'
            wait_event "${receiver}" "${cond}" --arg s "${STATION}" --arg a "${action}" \
                --argjson w "$(jq -c .response <<<"${want}")" --argjson m "${message_id}" >/dev/null \
                || problems+=("${receiver_adapter} reported no call-answered event with the fixture answer")
        fi
        cond='.event == "call-sent" and .station == $s and .action == $a and .payload == $r'
        wait_event "${sender}" "${cond}" --arg s "${STATION}" --arg a "${action}" --argjson r "${request}" >/dev/null \
            || problems+=("${sender_adapter} reported no call-sent event")
        cond='.event == "call-result" and .station == $s and .action == $a'
        wait_event "${sender}" "${cond}" --arg s "${STATION}" --arg a "${action}" >/dev/null \
            || problems+=("${sender_adapter} reported no call-result event")
    fi
    if [[ ${#problems[@]} -eq 0 ]]; then
        record "${id}" "${client}" "${server}" "${initiator}" "${feat}" "${action}" PASS "${ms}"
    else
        record "${id}" "${client}" "${server}" "${initiator}" "${feat}" "${action}" FAIL "${ms}" "$(IFS='; '; echo "${problems[*]}")"
    fi
}

# one_shot <id> <client> <server> <feature> <action> <want: ok|connect-failed|call-error:<code>> <server container> [client args...]
# [docker run options --] [client flags...]
one_shot() {
    local id="$1" client="$2" server="$3" feat="$4" action="$5" want="$6" srv="$7"; shift 7
    local doc code t0 ms problem="" opts=()
    if [[ " $* " == *" -- "* ]]; then
        while [[ "$1" != "--" ]]; do opts+=("$1"); shift; done
        shift
    fi
    t0=$(now_ms)
    doc=$(run_client "${client}" ${opts[@]+"${opts[@]}"} -- "${action}" --version "${OCPP}" --url "ws://${srv}:9000/ocpp" "$@"); code=$?
    ms=$(( $(now_ms) - t0 ))
    keep "${id}-${client}-${server}" "${doc}"
    case "${want}" in
        ok) [[ "${code}" == 0 ]] && jq -e '.ok == true' <<<"${doc}" >/dev/null 2>&1 || problem="exit ${code}: ${doc:0:300}" ;;
        connect-failed) [[ "${code}" == 3 ]] && jq -e '.ok == false and .error.code == "connect-failed"' <<<"${doc}" >/dev/null 2>&1 \
            || problem="exit ${code}, want 3 connect-failed: ${doc:0:300}" ;;
        call-error:*) [[ "${code}" == 1 ]] && jq -e --arg c "${want#call-error:}" '.error.code == "call-error" and .callError.code == $c' <<<"${doc}" >/dev/null 2>&1 \
            || problem="exit ${code}, want CALLERROR ${want#call-error:}: ${doc:0:300}" ;;
    esac
    ONE_SHOT_PROBLEM="${problem}"; ONE_SHOT_MS="${ms}"; ONE_SHOT_DOC="${doc}"
}

pairing() {  # pairing <client adapter> <server adapter>
    local client="$1" server="$2" srv sta name action payload id doc problem
    srv="csms-${server}-${RUN_ID}"; sta="station-${client}-${RUN_ID}"
    if ! declares "${client}" '.client.supported == true' || ! declares "${server}" '.server.supported == true'; then
        record "v16.pairing" "${client}" "${server}" client other "" SKIP_UNSUPPORTED 0 \
            "OCPP ${OCPP} is not declared by ${client} (client) or ${server} (server)"
        return
    fi
    log "${client} -> ${server}"

    start "${srv}" "${server}" -- server --version "${OCPP}"
    if ! wait_ready "${srv}"; then
        record "v16.pairing" "${client}" "${server}" client connectivity "" INFRA_ERROR 0 "the server did not become ready"
        stop_container "${srv}"; return
    fi
    start "${sta}" "${client}" -- client run --version "${OCPP}" --url "ws://${srv}:9000/ocpp"
    if ! wait_ready "${sta}"; then
        record "v16.pairing" "${client}" "${server}" client connectivity "" INFRA_ERROR 0 "client run did not connect"
        stop_container "${sta}"; stop_container "${srv}"; return
    fi
    if wait_event "${srv}" '.event == "connection-opened" and .station == $s' --arg s "${STATION}" >/dev/null; then
        record "v16.connect.client-run" "${client}" "${server}" client connectivity "" PASS 0
    else
        record "v16.connect.client-run" "${client}" "${server}" client connectivity "" FAIL 0 "the server reported no connection-opened event"
    fi

    # Station -> CSMS, every station sample, in the fixture's order.
    while IFS=$'\t' read -r name action payload; do
        exchange "v16.from-station.$(kebab "${name}")" "${client}" "${server}" client "${sta}" "${srv}" "${action}" "${payload}"
    done < <(jq -r '.samples | to_entries[] | select(.value.from == "station") | [.key, .value.action, (.value.payload | tojson)] | @tsv' "${FIXTURE}")
    # A token the fixture does not know: derived answer Invalid.
    exchange "v16.from-station.authorize-unknown" "${client}" "${server}" client "${sta}" "${srv}" Authorize '{"idTag":"04UNKNOWN00001"}'

    # CSMS -> station, every CSMS sample.
    while IFS=$'\t' read -r name action payload; do
        exchange "v16.from-csms.$(kebab "${name}")" "${client}" "${server}" server "${srv}" "${sta}" "${action}" "${payload}" --station "${STATION}"
    done < <(jq -r '.samples | to_entries[] | select(.value.from == "csms") | [.key, .value.action, (.value.payload | tojson)] | @tsv' "${FIXTURE}")
    # A configuration key the fixture does not know: unknownKey.
    exchange "v16.from-csms.get-configuration-unknown" "${client}" "${server}" server "${srv}" "${sta}" GetConfiguration \
        '{"key":["NoSuchKey","heartbeatinterval"]}' --station "${STATION}"

    # The station closes its connection; the CSMS reports it.
    docker stop -t 15 "${sta}" >/dev/null 2>&1
    if [[ "$(docker inspect -f '{{.State.ExitCode}}' "${sta}")" == 0 ]] \
        && wait_event "${srv}" '.event == "connection-closed" and .station == $s' --arg s "${STATION}" >/dev/null; then
        record "v16.connect.close" "${client}" "${server}" client connectivity "" PASS 0
    else
        record "v16.connect.close" "${client}" "${server}" client connectivity "" FAIL 0 \
            "client run exit $(docker inspect -f '{{.State.ExitCode}}' "${sta}"), or no connection-closed event on the server"
    fi
    stop_container "${sta}"

    # One-shot client: connect, one CALL, disconnect.
    one_shot v16.one-shot.boot "${client}" "${server}" provisioning BootNotification ok "${srv}" \
        --payload "$(jq -c '.samples.boot.payload' "${FIXTURE}")"
    problem="${ONE_SHOT_PROBLEM}"
    [[ -z "${problem}" ]] && ! jq -e "${JQ_LIB}"'(.response.currentTime | isdate) and .response.status == "Accepted"' <<<"${ONE_SHOT_DOC}" >/dev/null 2>&1 \
        && problem="not the fixture answer: ${ONE_SHOT_DOC:0:300}"
    record v16.one-shot.boot "${client}" "${server}" client provisioning BootNotification \
        "$([[ -z "${problem}" ]] && echo PASS || echo FAIL)" "${ONE_SHOT_MS}" "${problem}"

    # An identity the fixture does not list is refused in the handshake.
    one_shot v16.connect.unknown-station "${client}" "${server}" connectivity Heartbeat connect-failed "${srv}" --station OT-FABRIC-UNKNOWN
    problem="${ONE_SHOT_PROBLEM}"
    if [[ -z "${problem}" ]] && ! wait_event "${srv}" '.event == "connection-rejected" and .station == "OT-FABRIC-UNKNOWN" and .reason == "unknown-station"' >/dev/null; then
        problem="the server reported no connection-rejected event with reason unknown-station"
    fi
    record v16.connect.unknown-station "${client}" "${server}" client connectivity "" \
        "$([[ -z "${problem}" ]] && echo PASS || echo FAIL)" "${ONE_SHOT_MS}" "${problem}"

    events "${srv}" | grep -v '^{' | grep -q . && record v16.events.json-lines "${client}" "${server}" server other "" FAIL 0 "server stdout has lines that are not JSON"
    stop_container "${srv}"

    # ---- security profile 1: HTTP Basic authentication over ws:// ----
    if profile "${client}" 1 && profile "${server}" 1; then
        start "${srv}" "${server}" -- server --version "${OCPP}" --security-profile 1
        if wait_ready "${srv}"; then
            one_shot v16.profile1.accepted "${client}" "${server}" security-profile-1 Heartbeat ok "${srv}" --security-profile 1
            record v16.profile1.accepted "${client}" "${server}" client security-profile-1 Heartbeat \
                "$([[ -z "${ONE_SHOT_PROBLEM}" ]] && echo PASS || echo FAIL)" "${ONE_SHOT_MS}" "${ONE_SHOT_PROBLEM}"

            one_shot v16.profile1.no-credentials "${client}" "${server}" security-profile-1 Heartbeat connect-failed "${srv}"
            problem="${ONE_SHOT_PROBLEM}"
            if [[ -z "${problem}" ]] && ! wait_event "${srv}" '.event == "connection-rejected" and .station == $s and .reason == "authentication"' --arg s "${STATION}" >/dev/null; then
                problem="the server reported no connection-rejected event with reason authentication"
            fi
            record v16.profile1.no-credentials "${client}" "${server}" client security-profile-1 "" \
                "$([[ -z "${problem}" ]] && echo PASS || echo FAIL)" "${ONE_SHOT_MS}" "${problem}"

            one_shot v16.profile1.wrong-password "${client}" "${server}" security-profile-1 Heartbeat connect-failed "${srv}" \
                -v "${WORK}/fixtures:/wrong:ro" -- --security-profile 1 --fixture /wrong/password/v16/baseline/fixture.json
            problem="${ONE_SHOT_PROBLEM}"
            if [[ -z "${problem}" ]] && [[ $(events "${srv}" | jq -c 'select(.event == "connection-rejected" and .reason == "authentication")' | wc -l) -lt 2 ]]; then
                problem="the server reported no second connection-rejected event with reason authentication"
            fi
            record v16.profile1.wrong-password "${client}" "${server}" client security-profile-1 "" \
                "$([[ -z "${problem}" ]] && echo PASS || echo FAIL)" "${ONE_SHOT_MS}" "${problem}"
        else
            record v16.profile1.accepted "${client}" "${server}" client security-profile-1 "" INFRA_ERROR 0 "the server did not become ready"
        fi
        stop_container "${srv}"
    else
        for id in v16.profile1.accepted v16.profile1.no-credentials v16.profile1.wrong-password; do
            record "${id}" "${client}" "${server}" client security-profile-1 "" SKIP_UNSUPPORTED 0 \
                "security profile 1 is not declared by both ${client} and ${server}"
        done
    fi

    # ---- the fixture's own CALLERROR answers ----
    if feature "${server}" fixtureCallErrors; then
        start "${srv}" "${server}" -v "${WORK}/fixtures:/errors:ro" -- server --version "${OCPP}" --fixture /errors/callerror/v16/baseline/fixture.json
        if wait_ready "${srv}"; then
            one_shot v16.fixture-callerror.from-station "${client}" "${server}" errors Heartbeat call-error:GenericError "${srv}"
            problem="${ONE_SHOT_PROBLEM}"
            [[ -z "${problem}" ]] && ! jq -e '.callError.description == "fixture says no"' <<<"${ONE_SHOT_DOC}" >/dev/null 2>&1 \
                && problem="CALLERROR description is not the fixture's: ${ONE_SHOT_DOC:0:300}"
            record v16.fixture-callerror.from-station "${client}" "${server}" client errors Heartbeat \
                "$([[ -z "${problem}" ]] && echo PASS || echo FAIL)" "${ONE_SHOT_MS}" "${problem}"
        else
            record v16.fixture-callerror.from-station "${client}" "${server}" client errors Heartbeat INFRA_ERROR 0 "the server did not become ready"
        fi
        stop_container "${srv}"
    else
        record v16.fixture-callerror.from-station "${client}" "${server}" client errors Heartbeat SKIP_UNSUPPORTED 0 \
            "${server} does not declare fixtureCallErrors: its upstream cannot answer with a chosen CALLERROR"
    fi
    if feature "${client}" fixtureCallErrors; then
        start "${srv}" "${server}" -- server --version "${OCPP}"
        start "${sta}" "${client}" -v "${WORK}/fixtures:/errors:ro" -- client run --version "${OCPP}" --url "ws://${srv}:9000/ocpp" \
            --fixture /errors/callerror/v16/baseline/fixture.json
        if wait_ready "${srv}" && wait_ready "${sta}"; then
            t0=$(now_ms)
            doc=$(docker exec "${srv}" ocpp-interop call --station "${STATION}" Reset --payload '{"type":"Hard"}' 2>/dev/null); code=$?
            ms=$(( $(now_ms) - t0 ))
            keep "v16.fixture-callerror.from-csms-${client}-${server}" "${doc}"
            if [[ "${code}" == 1 ]] && jq -e '.error.code == "call-error" and .callError.code == "GenericError" and .callError.description == "fixture says no"' <<<"${doc}" >/dev/null 2>&1; then
                record v16.fixture-callerror.from-csms "${client}" "${server}" server errors Reset PASS "${ms}"
            else
                record v16.fixture-callerror.from-csms "${client}" "${server}" server errors Reset FAIL "${ms}" "exit ${code}: ${doc:0:300}"
            fi
        else
            record v16.fixture-callerror.from-csms "${client}" "${server}" server errors Reset INFRA_ERROR 0 "server or client run did not become ready"
        fi
        stop_container "${sta}"; stop_container "${srv}"
    else
        record v16.fixture-callerror.from-csms "${client}" "${server}" server errors Reset SKIP_UNSUPPORTED 0 \
            "${client} does not declare fixtureCallErrors: its upstream cannot answer with a chosen CALLERROR"
    fi
}

# Fixtures for the error cases, mounted read-only into the containers. The
# path ends in v16/baseline/fixture.json, as the fixture rules require.
mkdir -p "${WORK}/fixtures/password/v16/baseline" "${WORK}/fixtures/callerror/v16/baseline"
jq '.stations[0].password = "NotThePassword000001"' "${FIXTURE}" > "${WORK}/fixtures/password/v16/baseline/fixture.json"
jq '.answers.csms.Heartbeat = {"callError": {"code": "GenericError", "description": "fixture says no"}}
    | .answers.station.Reset = {"callError": {"code": "GenericError", "description": "fixture says no"}}' "${FIXTURE}" \
    > "${WORK}/fixtures/callerror/v16/baseline/fixture.json"
chmod -R a+rX "${WORK}"

log "adapters: ${ADAPTERS[*]} (${VERSION})"
for server in "${ADAPTERS[@]}"; do
    for client in "${ADAPTERS[@]}"; do
        pairing "${client}" "${server}"
    done
done

# ---- a known bug that no longer shows is a stale entry ----
while IFS=$'\t' read -r bug_adapter bug_role _ _ bug_text; do
    if ! grep -qxF "${bug_text}" "${WORK}/reproduced"; then
        fail "known bug of ${bug_adapter} (${bug_role}) did not reproduce: ${bug_text}"
        log "    remove it from tests/known-bugs.tsv; the pin can move to this version"
    fi
done < "${WORK}/active"

# ---- the report ----
REPORT=$(jq -s --arg id "interop-${RUN_ID}" --arg started "${STARTED}" --arg finished "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    --arg producer "ocpp-interop scripts/interop.sh (images ${VERSION})" \
    '{schemaVersion: "1.0", run: {id: $id, started: $started, finished: $finished, producer: $producer}, results: .}' \
    "${WORK}/results.jsonl")
if [[ -n "${REPORT_DIR:-}" ]]; then
    mkdir -p "${REPORT_DIR}"
    printf '%s\n' "${REPORT}" > "${REPORT_DIR}/report-interop.json"
fi
log "results by pairing and initiator:"
jq -r '.results | group_by([.client.name, .server.name, .initiator])[]
       | "  \(.[0].client.name) -> \(.[0].server.name), initiator \(.[0].initiator): "
         + (group_by(.status) | map("\(.[0].status) \(length)") | join(", "))' <<<"${REPORT}" >&2
finish
