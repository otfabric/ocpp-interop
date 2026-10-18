#!/usr/bin/env bash
# Container-contract smoke test for one adapter image.
#
# Usage: smoke.sh <adapter>          e.g. smoke.sh mobilityhouse
#
# Checks the parts of docs/CONTAINER_CONTRACT.md that do not need a second
# adapter: the command surface, usage errors and exit codes, capabilities, the
# baked fixtures, fixture validation, readiness, the event stream, the control
# socket, timeouts and a clean stop on SIGTERM. The checks that run a server
# or a station, and the fixture checks, run for every OCPP version the image
# declares, and so do the transport checks: security profiles 2 and 3 (TLS,
# with CAs and certificates generated for the run) and compression. Where a check needs a peer, the image talks to itself;
# scripts/interop.sh pairs different adapters.
#
# Environment: REGISTRY, VERSION (image tag, default dev), VERBOSE=1,
#              REPORT_DIR (keeps the capability document when set).

set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
# shellcheck source=scripts/lib.sh
source "${SCRIPT_DIR}/lib.sh"

ADAPTER="${1:?usage: smoke.sh <adapter>}"
IMAGE="$(image_of "${ADAPTER}")"
FIXTURE="${ROOT}/fixtures/v16/baseline/fixture.json"
STATION=$(jq -r '.stations[0].id' "${FIXTURE}")
# Set per version by use_version, as in scripts/interop.sh.
V="1.6" VDIR="v16"
use_version() {
    V="$1"
    VDIR=$(jq -rn --arg v "$1" '{"1.6": "v16", "2.0.1": "v201", "2.1": "v21"}[$v]')
    FIXTURE="${ROOT}/fixtures/${VDIR}/baseline/fixture.json"
    STATION=$(jq -r '.stations[0].id' "${FIXTURE}")
}
require_tools
WORK=$(mktemp -d)
trap 'rm -rf "${WORK}"; cleanup' EXIT
network_up
log "${IMAGE}"

expect_exit() {  # expect_exit <code> <label> <docker run args...>
    local want="$1" label="$2"; shift 2
    docker run --rm --network "${NETWORK}" --label "ocpp-interop.run=${RUN_ID}" "$@" >/dev/null 2>&1
    local got=$?
    if [[ "${got}" == "${want}" ]]; then ok "${label}: exit ${got}"; else fail "${label}: exit ${got}, want ${want}"; fi
}

# ---- capabilities ----
caps=$(docker run --rm "${IMAGE}" print-capabilities 2>/dev/null)
if [[ -n "${REPORT_DIR:-}" ]]; then
    mkdir -p "${REPORT_DIR}"
    printf '%s\n' "${caps}" > "${REPORT_DIR}/capabilities-${ADAPTER}.json"
fi
if jq -e --arg a "${ADAPTER}" '
        .schemaVersion == "1.0" and .adapter == $a and (.adapterVersion | type == "string")
        and (.contractVersion | test("^[0-9]+\\.[0-9]+$"))
        and (.upstream.revision | test("^[0-9a-f]{40}$")) and (.upstream.version | type == "string")
        and .upstream.independentEngine == true
        and (.versions | length) > 0
        and ([.versions[] | (.client.supported | type == "boolean") and (.server.supported | type == "boolean")
              and ([.features | .securityProfiles, .compression, .send, .callResultError] | all(. != null))] | all)' \
        <<<"${caps}" >/dev/null 2>&1
then ok "print-capabilities"; else fail "print-capabilities is not a valid capability document: ${caps}"; fi
if [[ $(wc -l <<<"${caps}") -eq 1 ]]; then ok "print-capabilities is one line"; else fail "print-capabilities printed more than one line"; fi
# For the version that use_version set:
has_feature() { jq -e --arg v "${V}" --arg f "$1" '.versions[$v].features[$f] == true' <<<"${caps}" >/dev/null 2>&1; }
has_role() { jq -e --arg v "${V}" --arg r "$1" '.versions[$v][$r].supported == true' <<<"${caps}" >/dev/null 2>&1; }
has_profile() { jq -e --arg v "${V}" --argjson p "$1" '.versions[$v].features.securityProfiles | index($p) != null' <<<"${caps}" >/dev/null 2>&1; }
DECLARED=$(jq -r '.versions | keys[]' <<<"${caps}" 2>/dev/null | tr '\n' ' ')
log "declared OCPP versions: ${DECLARED}"

# ---- the baked fixtures are the repository's ----
for v in 1.6 2.0.1 2.1; do
    dir=$(jq -rn --arg v "${v}" '{"1.6": "v16", "2.0.1": "v201", "2.1": "v21"}[$v]')
    if diff -q <(docker run --rm "${IMAGE}" print-fixture --version "${v}" baseline 2>/dev/null) "${ROOT}/fixtures/${dir}/baseline/fixture.json" >/dev/null
    then ok "print-fixture --version ${v} baseline"; else fail "print-fixture --version ${v} baseline differs from fixtures/${dir}/baseline/fixture.json"; fi
done
if diff -q <(docker run --rm "${IMAGE}" print-fixture --version 1.6 2>/dev/null) "${FIXTURE}" >/dev/null
then ok "print-fixture defaults to baseline"; else fail "print-fixture without a name is not the baseline fixture"; fi
expect_exit 1 "print-fixture of an unknown name" "${IMAGE}" print-fixture --version 1.6 no-such-fixture
expect_exit 2 "print-fixture without --version" "${IMAGE}" print-fixture baseline
expect_exit 0 "version" "${IMAGE}" version
if [[ $(docker run --rm "${IMAGE}" version 2>/dev/null | wc -l) -eq 1 ]]; then ok "version is one line"; else fail "version is not one line"; fi

# ---- usage errors and unsupported requests never touch the network ----
expect_exit 2 "no command" "${IMAGE}" ""
expect_exit 2 "unknown command" "${IMAGE}" frobnicate
expect_exit 2 "server without --version" "${IMAGE}" server
expect_exit 2 "server with an unknown flag" "${IMAGE}" server --version 1.6 --frobnicate 1
expect_exit 2 "server with --version twice" "${IMAGE}" server --version 1.6 --version 1.6
expect_exit 2 "server with an unknown version" "${IMAGE}" server --version 9.9
expect_exit 2 "server with a port that is not a number" "${IMAGE}" server --version 1.6 --port nine
expect_exit 2 "server with security profile 4" "${IMAGE}" server --version 1.6 --security-profile 4
expect_exit 2 "client without --url" "${IMAGE}" client Heartbeat --version 1.6
expect_exit 2 "client without --version" "${IMAGE}" client Heartbeat --url ws://nowhere:9000/ocpp
expect_exit 2 "client with a URL that is not ws://" "${IMAGE}" client Heartbeat --version 1.6 --url http://nowhere/ocpp
expect_exit 2 "client with a payload that is not JSON" "${IMAGE}" client Heartbeat --version 1.6 --url ws://nowhere:9000/ocpp --payload '{nope'
expect_exit 2 "client with a payload that is not an object" "${IMAGE}" client Heartbeat --version 1.6 --url ws://nowhere:9000/ocpp --payload '[1]'
expect_exit 2 "client with an action that is not an action name" "${IMAGE}" client heartbeat --version 1.6 --url ws://nowhere:9000/ocpp
expect_exit 2 "client run with --payload" "${IMAGE}" client run --version 1.6 --url ws://nowhere:9000/ocpp --payload '{}'
expect_exit 2 "call without an action" "${IMAGE}" call --payload '{}'
expect_exit 2 "--tls-dir with security profile 0" "${IMAGE}" server --version 1.6 --tls-dir /tmp
expect_exit 2 "server with security profile 2 without --tls-dir" "${IMAGE}" server --version 1.6 --security-profile 2
expect_exit 2 "server with a --tls-dir without the keys" "${IMAGE}" server --version 1.6 --security-profile 2 --tls-dir /no/such/dir
expect_exit 2 "server with --compression (a client flag)" "${IMAGE}" server --version 1.6 --compression
expect_exit 2 "client with wss:// and security profile 0" "${IMAGE}" client Heartbeat --version 1.6 --url wss://nowhere:9000/ocpp
expect_exit 2 "client with ws:// and security profile 3" "${IMAGE}" client Heartbeat --version 1.6 --url ws://nowhere:9000/ocpp --security-profile 3 --tls-dir /tmp
expect_exit 2 "client with --compression twice" "${IMAGE}" client Heartbeat --version 1.6 --url ws://nowhere:9000/ocpp --compression --compression
expect_exit 2 "client with a value after --compression" "${IMAGE}" client Heartbeat --version 1.6 --url ws://nowhere:9000/ocpp --compression yes
for v in 2.0.1 2.1; do
    jq -e --arg v "${v}" '.versions[$v].server.supported == true' <<<"${caps}" >/dev/null 2>&1 && continue
    expect_exit 5 "server --version ${v} (not declared)" "${IMAGE}" server --version "${v}"
    expect_exit 5 "client run --version ${v} (not declared)" "${IMAGE}" client run --version "${v}" --url ws://nowhere:9000/ocpp
done
for p in 2 3; do
    has_profile "${p}" && continue
    expect_exit 5 "server with security profile ${p} (not declared)" "${IMAGE}" server --version 1.6 --security-profile "${p}" --tls-dir /tmp
done
doc=$(run_client "${ADAPTER}" FrobnicateThing --version 1.6 --url ws://nowhere:9000/ocpp); code=$?
if [[ "${code}" == 5 ]] && jq -e '.ok == false and .error.code == "unsupported" and .operation == "FrobnicateThing"' <<<"${doc}" >/dev/null 2>&1
then ok "client with an action the upstream does not have: exit 5 and a result document"; else fail "unknown action: exit ${code}, ${doc}"; fi
expect_exit 3 "call without a running server or client run" "${IMAGE}" call Heartbeat

# A Reset the fixture's station answers: Soft in 1.6, Immediate in 2.0.1.
reset_payload() { if [[ "${V}" == 1.6 ]]; then echo '{"type":"Soft"}'; else echo '{"type":"Immediate"}'; fi; }

# served: ready within the wait timeout, or the exit code it stopped with
served() {  # served <container>
    local i
    for i in $(seq 1 150); do
        events "$1" | grep -q '"event":"ready"' && { echo ready; return; }
        [[ "$(docker inspect -f '{{.State.Running}}' "$1" 2>/dev/null)" == "true" ]] || break
        sleep 0.2
    done
    echo "exit $(docker inspect -f '{{.State.ExitCode}}' "$1" 2>/dev/null)"
}
# version_checks: everything that depends on the OCPP version, for the
# version use_version set.
version_checks() {
    # ---- fixtures: an invalid one stops the process before it listens (exit 4) ----
    # Each invalid fixture sits at <dir>/${VDIR}/baseline/fixture.json, so that it
    # breaks exactly one rule and not also "name is the directory name".
    bad="${WORK}/fixtures"
    for case in good json name role token version derived callerror badcode untyped send; do mkdir -p "${bad}/${case}/${VDIR}/baseline"; done
    mkdir -p "${bad}/name/${VDIR}/other"
    cp "${FIXTURE}" "${bad}/good/${VDIR}/baseline/fixture.json"
    echo '{ not json' > "${bad}/json/${VDIR}/baseline/fixture.json"
    cp "${FIXTURE}" "${bad}/name/${VDIR}/other/fixture.json"
    jq '.answers.csms.Reset = {"payload": {"status": "Accepted"}}' "${FIXTURE}" > "${bad}/role/${VDIR}/baseline/fixture.json"
    # One character more than the version allows: 20 (1.6), 36 (2.0.1), 255 (2.1).
    case "${V}" in
        1.6) long="0123456789ABCDEF01234" other="2.0.1" ;;
        2.0.1) long="0123456789ABCDEF0123456789ABCDEF01234" other="1.6" ;;
        *) long=$(printf 'A%.0s' $(seq 1 256)) other="2.0.1" ;;
    esac
    jq --arg t "${long}" '.idTokens[0].idToken = $t' "${FIXTURE}" > "${bad}/token/${VDIR}/baseline/fixture.json"
    jq --arg v "${other}" '.ocppVersion = $v' "${FIXTURE}" > "${bad}/version/${VDIR}/baseline/fixture.json"
    jq 'del(.idTokens[0].type)' "${FIXTURE}" > "${bad}/untyped/${VDIR}/baseline/fixture.json"
    jq '.answers.station.Reset = {"derived": "idTokens"}' "${FIXTURE}" > "${bad}/derived/${VDIR}/baseline/fixture.json"
    # A SEND is never answered (OCPP 2.1: NotifyPeriodicEventStream).
    jq '.answers.csms.NotifyPeriodicEventStream = {"payload": {}}' "${FIXTURE}" > "${bad}/send/${VDIR}/baseline/fixture.json"
    jq '.answers.csms.Heartbeat = {"callError": {"code": "GenericError", "description": "fixture says no"}}' "${FIXTURE}" \
        > "${bad}/callerror/${VDIR}/baseline/fixture.json"
    jq '.answers.csms.Heartbeat = {"callError": {"code": "NoSuchErrorCode", "description": "not an OCPP-J error code"}}' "${FIXTURE}" \
        > "${bad}/badcode/${VDIR}/baseline/fixture.json"
    chmod -R a+rX "${WORK}"
    mount=(-v "${bad}:/bad:ro")
    expect_exit 4 "server with a fixture that is not JSON" "${mount[@]}" "${IMAGE}" server --version "${V}" --fixture /bad/json/${VDIR}/baseline/fixture.json
    expect_exit 4 "server with a fixture whose name is not its directory" "${mount[@]}" "${IMAGE}" server --version "${V}" --fixture /bad/name/${VDIR}/other/fixture.json
    expect_exit 4 "server with a CSMS answer to an action the CSMS does not receive" "${mount[@]}" "${IMAGE}" server --version "${V}" --fixture /bad/role/${VDIR}/baseline/fixture.json
    expect_exit 4 "server with an id token longer than $((${#long} - 1)) characters" "${mount[@]}" "${IMAGE}" server --version "${V}" --fixture /bad/token/${VDIR}/baseline/fixture.json
    [[ "${V}" != 1.6 ]] && expect_exit 4 "server with an id token without a type" "${mount[@]}" "${IMAGE}" server --version "${V}" --fixture /bad/untyped/${VDIR}/baseline/fixture.json
    expect_exit 4 "server with a fixture of another OCPP version" "${mount[@]}" "${IMAGE}" server --version "${V}" --fixture /bad/version/${VDIR}/baseline/fixture.json
    expect_exit 4 "server with a derived answer that does not fit the action" "${mount[@]}" "${IMAGE}" server --version "${V}" --fixture /bad/derived/${VDIR}/baseline/fixture.json
    [[ "${V}" == 2.1 ]] && expect_exit 4 "server with an answer to a SEND" "${mount[@]}" "${IMAGE}" server --version "${V}" --fixture /bad/send/${VDIR}/baseline/fixture.json
    expect_exit 5 "server with a callError answer that is not an OCPP-J error code" "${mount[@]}" "${IMAGE}" server --version "${V}" --fixture /bad/badcode/${VDIR}/baseline/fixture.json
    expect_exit 4 "server with a missing fixture" "${IMAGE}" server --version "${V}" --fixture /no/such/fixture.json
    expect_exit 4 "client with a missing fixture" "${IMAGE}" client Heartbeat --version "${V}" --url ws://nowhere:9000/ocpp --fixture /no/such/fixture.json

    start "ctl-${RUN_ID}" "${ADAPTER}" "${mount[@]}" -- server --version "${V}" --fixture /bad/good/${VDIR}/baseline/fixture.json
    got=$(served "ctl-${RUN_ID}"); stop_container "ctl-${RUN_ID}"
    if [[ "${got}" == ready ]]; then ok "server with a mounted valid fixture: ready"; else fail "server with a mounted valid fixture: ${got}"; fi
    start "cerr-${RUN_ID}" "${ADAPTER}" "${mount[@]}" -- server --version "${V}" --fixture /bad/callerror/${VDIR}/baseline/fixture.json
    got=$(served "cerr-${RUN_ID}"); stop_container "cerr-${RUN_ID}"
    if has_feature fixtureCallErrors; then
        if [[ "${got}" == ready ]]; then ok "server with a callError answer: ready (fixtureCallErrors)"; else fail "server with a callError answer: ${got}"; fi
    else
        if [[ "${got}" == "exit 5" ]]; then ok "server with a callError answer: exit 5 (no fixtureCallErrors)"; else fail "server with a callError answer: ${got}, want exit 5"; fi
    fi

    # ---- SEND (OCPP 2.1): without the send feature it is never sent as a CALL ----
    if [[ "${V}" == 2.1 ]] && ! has_feature send && has_role client; then
        doc=$(run_client "${ADAPTER}" NotifyPeriodicEventStream --version "${V}" --url ws://nowhere:9000/ocpp \
            --payload "$(jq -c '.samples.notifyPeriodicEventStream.payload' "${FIXTURE}")"); code=$?
        if [[ "${code}" == 5 ]] && jq -e '.ok == false and .error.code == "unsupported"' <<<"${doc}" >/dev/null 2>&1
        then ok "a SEND action without the send feature: unsupported, exit 5"; else fail "SEND action without the send feature: exit ${code}, ${doc}"; fi
    fi

    # ---- a connection that cannot be made ----
    if has_role client; then
        doc=$(run_client "${ADAPTER}" Heartbeat --version "${V}" --url "ws://no-such-host-${RUN_ID}:9000/ocpp" --connect-timeout-ms 1500); code=$?
        if [[ "${code}" == 3 ]] && jq -e '.ok == false and .error.code == "connect-failed" and .response == null and .callError == null' <<<"${doc}" >/dev/null 2>&1
        then ok "connect failure: exit 3 and a result document"; else fail "connect failure: exit ${code}, ${doc}"; fi
    fi

    has_role server || return 0

    # ---- server: readiness, events, identity, clean stop ----
    srv="srv-${RUN_ID}"
    start "${srv}" "${ADAPTER}" -- server --version "${V}"
    if wait_ready "${srv}"; then ok "server becomes ready"; else fail "server did not become ready"; return 0; fi
    first=$(events "${srv}" | sed -n 1p)
    if jq -e --arg a "${ADAPTER}" --arg v "${V}" '.event == "ready" and .adapter == $a and .version == $v and .port == 9000 and .fixture == "baseline" and (.time | type == "string")' <<<"${first}" >/dev/null 2>&1
    then ok "ready event"; else fail "first stdout line is not the ready event: ${first}"; fi
    health=$(docker inspect --format '{{if .State.Health}}yes{{else}}no{{end}}' "${srv}")
    if [[ "${health}" == yes ]]; then ok "image has a health check"; else fail "image has no health check"; fi
    user=$(docker exec "${srv}" id -u 2>/dev/null)
    if [[ "${user}" == 10001 ]]; then ok "runs as uid 10001"; else fail "runs as uid ${user}, want 10001"; fi
    if docker exec "${srv}" sh -c 'test -s /licenses/ocpp-interop/LICENSE && test -s /licenses/THIRD_PARTY_NOTICES.md' 2>/dev/null
    then ok "/licenses has this repository's licence and the notices"; else fail "/licenses is incomplete"; fi
    labels=$(docker image inspect --format '{{json .Config.Labels}}' "${IMAGE}")
    if jq -e '.["org.opencontainers.image.source"] and .["org.opencontainers.image.licenses"] and .["org.opencontainers.image.title"]' <<<"${labels}" >/dev/null 2>&1
    then ok "OCI labels"; else fail "OCI labels missing: ${labels}"; fi

    doc=$(docker exec "${srv}" ocpp-interop call --station "${STATION}" Reset --payload "$(reset_payload)" 2>/dev/null); code=$?
    if [[ "${code}" == 1 ]] && jq -e '.ok == false and .error.code == "not-connected"' <<<"${doc}" >/dev/null 2>&1
    then ok "call to a station that is not connected: not-connected, exit 1"; else fail "call to a station that is not connected: exit ${code}, ${doc}"; fi

    if has_role client; then
        doc=$(run_client "${ADAPTER}" Heartbeat --version "${V}" --url "ws://${srv}:9000/ocpp"); code=$?
        if [[ "${code}" == 0 ]] && jq -e --arg a "${ADAPTER}" --arg v "${V}" "${JQ_LIB}"'
                .schemaVersion == "1.0" and .adapter == $a and .version == $v and .operation == "Heartbeat"
                and .ok == true and .error == null and .callError == null and .request == {}
                and (.response.currentTime | isdate) and (.elapsedMs | type == "number")' <<<"${doc}" >/dev/null 2>&1
        then ok "client result document, \${now} is a date-time"; else fail "Heartbeat: exit ${code}, ${doc}"; fi
        if [[ $(wc -l <<<"${doc}") -eq 1 ]]; then ok "client result is one line"; else fail "client printed more than one line on stdout"; fi
        if has_feature messageIds; then
            id=$(jq -r '.messageId // empty' <<<"${doc}")
            if [[ -n "${id}" ]] && wait_event "${srv}" '.event == "call-received" and .messageId == $id' --arg id "${id}" >/dev/null
            then ok "messageIds: the result's messageId is the one the server received"; else fail "messageIds: ${doc}"; fi
        elif jq -e 'has("messageId")' <<<"${doc}" >/dev/null 2>&1; then
            fail "messageIds is not declared, but the result has a messageId"
        else
            ok "no messageId, as declared"
        fi
        for want in \
            '.event == "connection-opened" and .station == $s' \
            '.event == "call-received" and .station == $s and .action == "Heartbeat" and .payload == {}' \
            '.event == "call-answered" and .station == $s and .action == "Heartbeat" and (.payload.currentTime | type == "string")' \
            '.event == "connection-closed" and .station == $s'
        do
            if wait_event "${srv}" "${want}" --arg s "${STATION}" >/dev/null; then ok "server event: ${want}"; else fail "no server event with ${want}"; fi
        done
        doc=$(run_client "${ADAPTER}" Heartbeat --version "${V}" --url "ws://${srv}:9000/ocpp" --station NOT-IN-THE-FIXTURE); code=$?
        if [[ "${code}" == 3 ]] && jq -e '.error.code == "connect-failed"' <<<"${doc}" >/dev/null 2>&1
        then ok "unknown station identity: connect-failed, exit 3"; else fail "unknown station identity: exit ${code}, ${doc}"; fi
        if wait_event "${srv}" '.event == "connection-rejected" and .station == "NOT-IN-THE-FIXTURE" and .reason == "unknown-station"' >/dev/null
        then ok "server event: connection-rejected unknown-station"; else fail "no connection-rejected event for an unknown station"; fi
    fi
    if events "${srv}" | grep -v '^{' | grep -q .; then fail "server stdout has lines that are not JSON"; else ok "server stdout is JSON Lines only"; fi

    # A second server in the same container: the port is taken.
    docker exec "${srv}" ocpp-interop server --version "${V}" --ready-file /tmp/second --control-socket /tmp/second.sock >/dev/null 2>&1
    code=$?
    if [[ "${code}" != 0 ]]; then ok "second server on a busy port fails (exit ${code})"; else fail "second server on a busy port exited 0"; fi

    docker stop -t 15 "${srv}" >/dev/null
    code=$(docker inspect --format '{{.State.ExitCode}}' "${srv}")
    exited "${srv}"
    last=$(events "${srv}" | tail -1)
    if [[ "${code}" == 0 ]]; then ok "SIGTERM: exit 0"; else fail "SIGTERM: exit ${code}"; fi
    if jq -e '.event == "stopped"' <<<"${last}" >/dev/null 2>&1; then ok "stopped event"; else fail "last stdout line is not the stopped event: ${last}"; fi
    stop_container "${srv}"

    has_role client || return 0

    # ---- client run: hold, control socket, lost connection, timeouts ----
    start "${srv}" "${ADAPTER}" -- server --version "${V}"
    wait_ready "${srv}" || { fail "server did not become ready"; return 0; }
    sta="sta-${RUN_ID}"
    start "${sta}" "${ADAPTER}" -- client run --version "${V}" --url "ws://${srv}:9000/ocpp"
    if wait_ready "${sta}"; then ok "client run becomes ready"; else fail "client run did not become ready"; fi
    if wait_event "${sta}" '.event == "ready" and .station == $s and (.url | endswith("/ocpp/" + $s))' --arg s "${STATION}" >/dev/null
    then ok "client run: ready event"; else fail "client run: no ready event"; fi
    doc=$(docker exec "${sta}" ocpp-interop call BootNotification --payload "$(jq -c '.samples.boot.payload' "${FIXTURE}")" 2>/dev/null); code=$?
    if [[ "${code}" == 0 ]] && jq -e '.ok and .response.status == "Accepted" and .response.interval == 300' <<<"${doc}" >/dev/null 2>&1
    then ok "call from client run"; else fail "call from client run: exit ${code}, ${doc}"; fi
    doc=$(docker exec "${sta}" ocpp-interop call --station SOMEONE-ELSE Heartbeat 2>/dev/null); code=$?
    if [[ "${code}" == 1 ]] && jq -e '.error.code == "not-connected"' <<<"${doc}" >/dev/null 2>&1
    then ok "call from client run for another station: not-connected"; else fail "call for another station: exit ${code}, ${doc}"; fi
    doc=$(docker exec "${srv}" ocpp-interop call --station "${STATION}" Reset --payload "$(reset_payload)" 2>/dev/null); code=$?
    # The fixture's answer, as scripts/interop.sh compares it (every field of
    # the answer, with an equal value).
    if [[ "${code}" == 0 ]] && jq -e "${JQ_LIB}"'.ok and within({"status":"Accepted"}; .response)' <<<"${doc}" >/dev/null 2>&1
    then ok "call from the server to the connected station"; else fail "call from the server: exit ${code}, ${doc}"; fi
    doc=$(docker exec "${srv}" ocpp-interop call --station "${STATION}" Reset --payload '{"type":"Sideways"}' 2>/dev/null); code=$?
    if [[ "${code}" == 1 ]] && jq -e '.ok == false and .error.code == "invalid-request"' <<<"${doc}" >/dev/null 2>&1
    then ok "a request the upstream refuses: invalid-request, exit 1"; else fail "invalid request: exit ${code}, ${doc}"; fi

    # A station that does not answer: the CSMS's call times out.
    docker pause "${sta}" >/dev/null
    doc=$(docker exec "${srv}" ocpp-interop call --station "${STATION}" Reset --payload "$(reset_payload)" --timeout-ms 1000 2>/dev/null); code=$?
    docker unpause "${sta}" >/dev/null
    if [[ "${code}" == 1 ]] && jq -e '.ok == false and .error.code == "timeout"' <<<"${doc}" >/dev/null 2>&1
    then ok "silent station: timeout, exit 1"; else fail "silent station: exit ${code}, ${doc}"; fi

    # A CSMS that does not answer: the station's call times out.
    docker pause "${srv}" >/dev/null
    doc=$(docker exec "${sta}" ocpp-interop call Heartbeat --timeout-ms 1000 2>/dev/null); code=$?
    docker unpause "${srv}" >/dev/null
    if [[ "${code}" == 1 ]] && jq -e '.ok == false and .error.code == "timeout"' <<<"${doc}" >/dev/null 2>&1
    then ok "silent CSMS: timeout, exit 1"; else fail "silent CSMS: exit ${code}, ${doc}"; fi

    # The CSMS goes away: client run reports it and exits 1.
    docker stop -t 15 "${srv}" >/dev/null
    for _ in $(seq 1 100); do [[ "$(docker inspect -f '{{.State.Running}}' "${sta}")" == "true" ]] || break; sleep 0.2; done
    code=$(docker inspect --format '{{.State.ExitCode}}' "${sta}")
    exited "${sta}"
    if [[ "${code}" == 1 ]]; then ok "client run: lost connection, exit 1"; else fail "client run after the server stopped: exit ${code} (running: $(docker inspect -f '{{.State.Running}}' "${sta}"))"; fi
    if events "${sta}" | tail -2 | jq -s -e '.[0].event == "connection-closed" and .[1].event == "stopped"' >/dev/null 2>&1
    then ok "client run: connection-closed and stopped events"; else fail "client run: last events are $(events "${sta}" | tail -2 | tr '\n' ' ')"; fi
    stop_container "${sta}"; stop_container "${srv}"

    # --hold-ms: disconnect and exit 0.
    start "${srv}" "${ADAPTER}" -- server --version "${V}"
    wait_ready "${srv}" || { fail "server did not become ready"; return 0; }
    start "${sta}" "${ADAPTER}" -- client run --version "${V}" --url "ws://${srv}:9000/ocpp" --hold-ms 1500
    for _ in $(seq 1 100); do [[ "$(docker inspect -f '{{.State.Running}}' "${sta}")" == "true" ]] || break; sleep 0.2; done
    code=$(docker inspect --format '{{.State.ExitCode}}' "${sta}")
    exited "${sta}"
    if [[ "${code}" == 0 ]] && events "${sta}" | tail -1 | jq -e '.event == "stopped"' >/dev/null 2>&1
    then ok "client run --hold-ms: stopped, exit 0"; else fail "client run --hold-ms: exit ${code}"; fi
    if wait_event "${srv}" '.event == "connection-closed" and .station == $s' --arg s "${STATION}" >/dev/null
    then ok "server sees the held connection close"; else fail "server did not report the close of the held connection"; fi
    stop_container "${sta}"

    # SIGTERM to client run: exit 0.
    start "${sta}" "${ADAPTER}" -- client run --version "${V}" --url "ws://${srv}:9000/ocpp"
    wait_ready "${sta}" || fail "client run did not become ready"
    docker stop -t 15 "${sta}" >/dev/null
    code=$(docker inspect --format '{{.State.ExitCode}}' "${sta}")
    exited "${sta}"
    if [[ "${code}" == 0 ]] && events "${sta}" | tail -1 | jq -e '.event == "stopped"' >/dev/null 2>&1
    then ok "client run SIGTERM: stopped, exit 0"; else fail "client run SIGTERM: exit ${code}"; fi
    if events "${sta}" | grep -v '^{' | grep -q .; then fail "client run stdout has lines that are not JSON"; else ok "client run stdout is JSON Lines only"; fi
    stop_container "${sta}"; stop_container "${srv}"

    # client run that cannot connect: exit 3.
    start "${sta}" "${ADAPTER}" -- client run --version "${V}" --url "ws://no-such-host-${RUN_ID}:9000/ocpp" --connect-timeout-ms 1500
    for _ in $(seq 1 100); do [[ "$(docker inspect -f '{{.State.Running}}' "${sta}")" == "true" ]] || break; sleep 0.2; done
    code=$(docker inspect --format '{{.State.ExitCode}}' "${sta}")
    exited "${sta}"
    if [[ "${code}" == 3 ]] && events "${sta}" | sed -n 1p | jq -e '.event == "connection-failed"' >/dev/null 2>&1
    then ok "client run that cannot connect: connection-failed, exit 3"; else fail "client run that cannot connect: exit ${code}, events: $(events "${sta}" | tr '\n' ' ')"; fi
    stop_container "${sta}"
}

# ---- transport: security profiles 2 and 3, compression ----

# known_bug <test id>: the evidence of a known upstream bug of this adapter
# (tests/known-bugs.tsv) that explains a failure of that test when the
# adapter is its own peer, as scripts/interop.sh reports it (KNOWN_ISSUE).
known_bug() {
    local who version regex evidence _ adapter peer upstream
    upstream=$(jq -r '.upstream.version' <<<"${caps}")
    while IFS=$'\t' read -r who version regex evidence _; do
        [[ -z "${who}" || "${who}" == \#* ]] && continue
        adapter="${who%%[:@]*}"; peer=""
        [[ "${who}" == *@* ]] && peer="${who#*@}"
        if [[ "${adapter}" == "${ADAPTER}" && "${version}" == "${upstream}" && ( -z "${peer}" || "${peer}" == "${ADAPTER}" ) \
              && "$1" =~ ${regex} ]]; then
            echo "${evidence}"; return 0
        fi
    done < "${ROOT}/tests/known-bugs.tsv"
    return 1
}

# verdict <test id> <label> <problem>: ok, fail, or a known upstream bug. A
# known bug that does not show any more fails, so that the list cannot go stale.
verdict() {
    local evidence
    if evidence=$(known_bug "$1"); then
        if [[ -n "$3" ]]; then KNOWN=$((KNOWN + 1)); log "known upstream bug: ${CHECK_PREFIX}$2: $3 (${evidence})"
        else fail "$2: passes, but tests/known-bugs.tsv lists a bug for $1 (${evidence}): remove the entry"; fi
    elif [[ -n "$3" ]]; then fail "$2: $3"
    else ok "$2"; fi
}

# tls_client <want: ok|connect-failed> <label> [client flags...]: a one-shot
# Heartbeat with the run's TLS material mounted at /tls.
tls_client() {
    local want="$1" label="$2"; shift 2
    local doc code
    doc=$(run_client "${ADAPTER}" -v "${WORK}/tls:/tls:ro" -v "${WORK}/wrong:/wrong:ro" -- Heartbeat --version "${V}" \
        --connect-timeout-ms 5000 "$@"); code=$?
    if [[ "${want}" == ok ]]; then
        if [[ "${code}" == 0 ]] && jq -e '.ok == true' <<<"${doc}" >/dev/null 2>&1; then ok "${label}"; else fail "${label}: exit ${code}, ${doc}"; fi
    elif [[ "${code}" == 3 ]] && jq -e '.ok == false and .error.code == "connect-failed"' <<<"${doc}" >/dev/null 2>&1; then
        ok "${label}: connect-failed"
    else
        fail "${label}: exit ${code}, want 3 connect-failed: ${doc}"
    fi
}

# rejected <server> <n>: the server reported at least n connection-rejected
# events with reason authentication for the station.
rejected() {
    local i
    for i in $(seq 1 50); do
        [[ $(events "$1" | jq -c --arg s "${STATION}" 'select(.event == "connection-rejected" and .station == $s and .reason == "authentication")' 2>/dev/null | wc -l) -ge "$2" ]] && return 0
        sleep 0.1
    done
    return 1
}

# A station that stays connected over the transport the flags set up: two
# CALLs from the station, two from the CSMS (the second message in each
# direction is the one a broken compression context fails on). Prints the
# problems, nothing when every CALL is answered.
session_problems() {  # session_problems <server container> <station container>
    local srv="$1" sta="$2" doc
    doc=$(docker exec "${sta}" ocpp-interop call BootNotification --payload "$(jq -c '.samples.boot.payload' "${FIXTURE}")" --timeout-ms 5000 2>/dev/null)
    jq -e '.ok' <<<"${doc}" >/dev/null 2>&1 || echo "BootNotification: ${doc:0:200}"
    doc=$(docker exec "${sta}" ocpp-interop call Heartbeat --timeout-ms 5000 2>/dev/null)
    jq -e '.ok' <<<"${doc}" >/dev/null 2>&1 || echo "Heartbeat: ${doc:0:200}"
    doc=$(docker exec "${srv}" ocpp-interop call --station "${STATION}" ChangeAvailability \
        --payload "$(jq -c '.samples.changeAvailability.payload' "${FIXTURE}")" --timeout-ms 5000 2>/dev/null)
    jq -e '.ok' <<<"${doc}" >/dev/null 2>&1 || echo "ChangeAvailability from the CSMS: ${doc:0:200}"
    doc=$(docker exec "${srv}" ocpp-interop call --station "${STATION}" ClearCache --payload '{}' --timeout-ms 5000 2>/dev/null)
    jq -e '.ok' <<<"${doc}" >/dev/null 2>&1 || echo "ClearCache from the CSMS: ${doc:0:200}"
}

tls_checks() {
    has_role server && has_role client || return 0
    local srv="tls-${RUN_ID}" sta="tlssta-${RUN_ID}" alias="unlisted-${RUN_ID}" p type dir problems
    local url="wss://tls-${RUN_ID}:9000/ocpp"
    dir="${WORK}/tls/${VDIR}"
    if [[ ! -d "${dir}" ]]; then
        { make_pki "${dir}/rsa" rsa "${STATION}" "${srv}" localhost 127.0.0.1 \
            && make_pki "${dir}/ec" ec "${STATION}" "${srv}" localhost 127.0.0.1; } || { fail "cannot generate the TLS material"; return 0; }
        mkdir -p "${WORK}/wrong/${VDIR}/baseline"
        jq '.stations[0].password = "NotThePassword000001"' "${FIXTURE}" > "${WORK}/wrong/${VDIR}/baseline/fixture.json"
        chmod -R a+rX "${WORK}/wrong"
    fi
    for p in 2 3; do
        has_profile "${p}" || continue
        for type in rsa ec; do
            local tls="/tls/${VDIR}/${type}"
            start "${srv}" "${ADAPTER}" -v "${WORK}/tls:/tls:ro" --network-alias "${alias}" -- \
                server --version "${V}" --security-profile "${p}" --tls-dir "${tls}/server"
            if ! wait_ready "${srv}"; then fail "profile ${p} (${type}): the server did not become ready"; stop_container "${srv}"; continue; fi
            if wait_event "${srv}" '.event == "ready" and .securityProfile == $p' --argjson p "${p}" >/dev/null
            then ok "profile ${p} (${type}): ready event"; else fail "profile ${p} (${type}): no ready event with securityProfile ${p}"; fi
            tls_client ok "profile ${p} (${type}): accepted" --url "${url}" --security-profile "${p}" --tls-dir "${tls}/station"
            if wait_event "${srv}" '.event == "connection-opened" and .station == $s' --arg s "${STATION}" >/dev/null
            then ok "profile ${p} (${type}): connection-opened"; else fail "profile ${p} (${type}): no connection-opened event"; fi
            if [[ "${type}" == rsa ]]; then
                # What must not connect.
                tls_client connect-failed "profile ${p}: plain ws:// to the TLS server" --url "ws://${srv}:9000/ocpp"
                tls_client connect-failed "profile ${p}: a server certificate from another CA" \
                    --url "${url}" --security-profile "${p}" --tls-dir "${tls}/untrusted"
                tls_client connect-failed "profile ${p}: a host name the server certificate does not name" \
                    --url "wss://${alias}:9000/ocpp" --security-profile "${p}" --tls-dir "${tls}/station"
                if [[ "${p}" == 2 ]]; then
                    tls_client connect-failed "profile 2: wrong password" --url "${url}" --security-profile 2 \
                        --tls-dir "${tls}/station" --fixture "/wrong/${VDIR}/baseline/fixture.json"
                    if rejected "${srv}" 1; then ok "profile 2: connection-rejected authentication"; else fail "profile 2: no connection-rejected event for a wrong password"; fi
                else
                    tls_client connect-failed "profile 3: a client certificate of another identity" \
                        --url "${url}" --security-profile 3 --tls-dir "${tls}/other-cn"
                    if rejected "${srv}" 1; then ok "profile 3: connection-rejected authentication"; else fail "profile 3: no connection-rejected event for a certificate of another identity"; fi
                    tls_client connect-failed "profile 3: a client certificate from another CA" \
                        --url "${url}" --security-profile 3 --tls-dir "${tls}/other-ca"
                    tls_client connect-failed "profile 3: no client certificate" \
                        --url "${url}" --security-profile 2 --tls-dir "${tls}/station"
                fi
                # A station that stays connected, CALLs from both ends.
                start "${sta}" "${ADAPTER}" -v "${WORK}/tls:/tls:ro" -- client run --version "${V}" --url "${url}" \
                    --security-profile "${p}" --tls-dir "${tls}/station"
                if wait_ready "${sta}"; then
                    problems=$(session_problems "${srv}" "${sta}")
                    if [[ -z "${problems}" ]]; then ok "profile ${p}: client run, CALLs from both ends"; else fail "profile ${p}: client run: ${problems}"; fi
                else
                    fail "profile ${p}: client run did not connect"
                fi
                stop_container "${sta}"
            fi
            stop_container "${srv}"
        done
    done
}

# The handshake response of the server to a request that offers
# permessage-deflate, read with bash inside the server's own container.
offer_deflate() {  # offer_deflate <server container>
    docker exec "$1" bash -c 'exec 3<>/dev/tcp/127.0.0.1/9000 || exit 1
        printf "GET /ocpp/%s HTTP/1.1\r\nHost: localhost:9000\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\nSec-WebSocket-Version: 13\r\nSec-WebSocket-Protocol: %s\r\nSec-WebSocket-Extensions: permessage-deflate; client_max_window_bits\r\n\r\n" "$1" "$2" >&3
        timeout 2 cat <&3; exit 0' _ "${STATION}" "$(jq -rn --arg v "${V}" '{"1.6": "ocpp1.6", "2.0.1": "ocpp2.0.1", "2.1": "ocpp2.1"}[$v]')" 2>/dev/null | tr -d '\r'
}

compression_checks() {
    has_role server && has_role client || return 0
    local srv="cmp-${RUN_ID}" sta="cmpsta-${RUN_ID}" url problems doc code
    url="ws://cmp-${RUN_ID}:9000/ocpp"
    start "${srv}" "${ADAPTER}" -- server --version "${V}"
    if ! wait_ready "${srv}"; then fail "compression: the server did not become ready"; stop_container "${srv}"; return 0; fi
    if ! has_feature compression; then
        doc=$(run_client "${ADAPTER}" Heartbeat --version "${V}" --url "${url}" --compression); code=$?
        if [[ "${code}" == 5 ]]; then
            ok "compression not declared: client --compression exits 5"
        elif [[ "${code}" == 0 ]] && ! events "${srv}" | jq -e 'select(.event == "connection-opened" and .compression == true)' >/dev/null 2>&1; then
            ok "compression not declared: client --compression connects, not negotiated"
        else
            fail "compression not declared: client --compression: exit ${code}, ${doc}"
        fi
        stop_container "${srv}"; return 0
    fi
    # The CSMS accepts permessage-deflate whenever it is offered.
    if offer_deflate "${srv}" | grep -qi '^sec-websocket-extensions:.*permessage-deflate'
    then ok "compression: the CSMS accepts permessage-deflate"; else fail "compression: the CSMS does not accept permessage-deflate: $(offer_deflate "${srv}" | head -12 | tr '\n' '|')"; fi
    stop_container "${srv}"

    # Without --compression the station does not offer it.
    start "${srv}" "${ADAPTER}" -- server --version "${V}"
    wait_ready "${srv}" || { fail "compression: the server did not become ready"; stop_container "${srv}"; return 0; }
    start "${sta}" "${ADAPTER}" -- client run --version "${V}" --url "${url}"
    if wait_ready "${sta}" && wait_event "${sta}" '.event == "connection-opened" and .compression == false' >/dev/null \
        && wait_event "${srv}" '.event == "connection-opened" and .compression != true' >/dev/null
    then ok "compression: not offered without --compression"; else fail "compression: without --compression: $(events "${sta}" | sed -n 1p) $(events "${srv}" | jq -c 'select(.event == "connection-opened")')"; fi
    stop_container "${sta}"; stop_container "${srv}"

    # With --compression: offered, negotiated, and every message is read.
    start "${srv}" "${ADAPTER}" -- server --version "${V}"
    wait_ready "${srv}" || { fail "compression: the server did not become ready"; stop_container "${srv}"; return 0; }
    start "${sta}" "${ADAPTER}" -- client run --version "${V}" --url "${url}" --compression
    if wait_ready "${sta}"; then
        problems=$(session_problems "${srv}" "${sta}" | tr '\n' ' ')
        events "${sta}" | jq -e 'select(.event == "connection-opened") | .compression == false' >/dev/null 2>&1 \
            && problems+="the station reports compression false; "
        events "${srv}" | jq -e 'select(.event == "connection-opened") | .compression == false' >/dev/null 2>&1 \
            && problems+="the CSMS reports compression false; "
    else
        problems="client run --compression did not connect"
    fi
    verdict "${VDIR}.compression" "compression: client run --compression, CALLs from both ends" "${problems}"
    stop_container "${sta}"; stop_container "${srv}"
}

for v in ${DECLARED}; do
    use_version "${v}"
    log "OCPP ${V}"; CHECK_PREFIX="[${V}] "
    version_checks
    tls_checks
    compression_checks
done

finish
