#!/usr/bin/env bash
# Container-contract smoke test for one adapter image.
#
# Usage: smoke.sh <adapter>          e.g. smoke.sh mobilityhouse
#
# Checks the parts of docs/CONTAINER_CONTRACT.md that do not need a second
# adapter: the command surface, usage errors and exit codes, capabilities, the
# baked fixtures, fixture validation, readiness, the event stream, the control
# socket, timeouts and a clean stop on SIGTERM. Where a check needs a peer, the
# image talks to itself; scripts/interop.sh pairs different adapters.
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
        and (.versions["1.6"].client.supported | type == "boolean")
        and (.versions["1.6"].server.supported | type == "boolean")
        and ([.versions["1.6"].features | .securityProfiles, .compression, .send, .callResultError] | all(. != null))' \
        <<<"${caps}" >/dev/null 2>&1
then ok "print-capabilities"; else fail "print-capabilities is not a valid capability document: ${caps}"; fi
if [[ $(wc -l <<<"${caps}") -eq 1 ]]; then ok "print-capabilities is one line"; else fail "print-capabilities printed more than one line"; fi
has_feature() { jq -e --arg f "$1" '.versions["1.6"].features[$f] == true' <<<"${caps}" >/dev/null 2>&1; }
has_role() { jq -e --arg r "$1" '.versions["1.6"][$r].supported == true' <<<"${caps}" >/dev/null 2>&1; }
has_profile() { jq -e --argjson p "$1" '.versions["1.6"].features.securityProfiles | index($p) != null' <<<"${caps}" >/dev/null 2>&1; }

# ---- the baked fixtures are the repository's ----
for v in 1.6 2.0.1; do
    dir=$([[ "${v}" == 1.6 ]] && echo v16 || echo v201)
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
for v in 2.0.1 2.1; do
    jq -e --arg v "${v}" '.versions[$v].server.supported == true' <<<"${caps}" >/dev/null 2>&1 && continue
    expect_exit 5 "server --version ${v} (not declared)" "${IMAGE}" server --version "${v}"
    expect_exit 5 "client run --version ${v} (not declared)" "${IMAGE}" client run --version "${v}" --url ws://nowhere:9000/ocpp
done
for p in 2 3; do
    has_profile "${p}" && continue
    expect_exit 5 "server with security profile ${p} (not declared)" "${IMAGE}" server --version 1.6 --security-profile "${p}"
done
doc=$(run_client "${ADAPTER}" FrobnicateThing --version 1.6 --url ws://nowhere:9000/ocpp); code=$?
if [[ "${code}" == 5 ]] && jq -e '.ok == false and .error.code == "unsupported" and .operation == "FrobnicateThing"' <<<"${doc}" >/dev/null 2>&1
then ok "client with an action the upstream does not have: exit 5 and a result document"; else fail "unknown action: exit ${code}, ${doc}"; fi
expect_exit 3 "call without a running server or client run" "${IMAGE}" call Heartbeat

# ---- fixtures: an invalid one stops the process before it listens (exit 4) ----
# Each invalid fixture sits at <dir>/v16/baseline/fixture.json, so that it
# breaks exactly one rule and not also "name is the directory name".
bad="${WORK}/fixtures"
for case in good json name role token version derived callerror; do mkdir -p "${bad}/${case}/v16/baseline"; done
mkdir -p "${bad}/name/v16/other"
cp "${FIXTURE}" "${bad}/good/v16/baseline/fixture.json"
echo '{ not json' > "${bad}/json/v16/baseline/fixture.json"
cp "${FIXTURE}" "${bad}/name/v16/other/fixture.json"
jq '.answers.csms.Reset = {"payload": {"status": "Accepted"}}' "${FIXTURE}" > "${bad}/role/v16/baseline/fixture.json"
jq '.idTokens[0].idToken = "0123456789ABCDEF01234"' "${FIXTURE}" > "${bad}/token/v16/baseline/fixture.json"
jq '.ocppVersion = "2.0.1"' "${FIXTURE}" > "${bad}/version/v16/baseline/fixture.json"
jq '.answers.station.Reset = {"derived": "idTokens"}' "${FIXTURE}" > "${bad}/derived/v16/baseline/fixture.json"
jq '.answers.csms.Heartbeat = {"callError": {"code": "GenericError", "description": "fixture says no"}}' "${FIXTURE}" \
    > "${bad}/callerror/v16/baseline/fixture.json"
chmod -R a+rX "${WORK}"
mount=(-v "${bad}:/bad:ro")
expect_exit 4 "server with a fixture that is not JSON" "${mount[@]}" "${IMAGE}" server --version 1.6 --fixture /bad/json/v16/baseline/fixture.json
expect_exit 4 "server with a fixture whose name is not its directory" "${mount[@]}" "${IMAGE}" server --version 1.6 --fixture /bad/name/v16/other/fixture.json
expect_exit 4 "server with a CSMS answer to an action the CSMS does not receive" "${mount[@]}" "${IMAGE}" server --version 1.6 --fixture /bad/role/v16/baseline/fixture.json
expect_exit 4 "server with an id token longer than 20 characters" "${mount[@]}" "${IMAGE}" server --version 1.6 --fixture /bad/token/v16/baseline/fixture.json
expect_exit 4 "server with a fixture of another OCPP version" "${mount[@]}" "${IMAGE}" server --version 1.6 --fixture /bad/version/v16/baseline/fixture.json
expect_exit 4 "server with a derived answer that does not fit the action" "${mount[@]}" "${IMAGE}" server --version 1.6 --fixture /bad/derived/v16/baseline/fixture.json
expect_exit 4 "server with a missing fixture" "${IMAGE}" server --version 1.6 --fixture /no/such/fixture.json
expect_exit 4 "client with a missing fixture" "${IMAGE}" client Heartbeat --version 1.6 --url ws://nowhere:9000/ocpp --fixture /no/such/fixture.json

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
start "ctl-${RUN_ID}" "${ADAPTER}" "${mount[@]}" -- server --version 1.6 --fixture /bad/good/v16/baseline/fixture.json
got=$(served "ctl-${RUN_ID}"); stop_container "ctl-${RUN_ID}"
if [[ "${got}" == ready ]]; then ok "server with a mounted valid fixture: ready"; else fail "server with a mounted valid fixture: ${got}"; fi
start "cerr-${RUN_ID}" "${ADAPTER}" "${mount[@]}" -- server --version 1.6 --fixture /bad/callerror/v16/baseline/fixture.json
got=$(served "cerr-${RUN_ID}"); stop_container "cerr-${RUN_ID}"
if has_feature fixtureCallErrors; then
    if [[ "${got}" == ready ]]; then ok "server with a callError answer: ready (fixtureCallErrors)"; else fail "server with a callError answer: ${got}"; fi
else
    if [[ "${got}" == "exit 5" ]]; then ok "server with a callError answer: exit 5 (no fixtureCallErrors)"; else fail "server with a callError answer: ${got}, want exit 5"; fi
fi

# ---- a connection that cannot be made ----
if has_role client; then
    doc=$(run_client "${ADAPTER}" Heartbeat --version 1.6 --url "ws://no-such-host-${RUN_ID}:9000/ocpp" --connect-timeout-ms 1500); code=$?
    if [[ "${code}" == 3 ]] && jq -e '.ok == false and .error.code == "connect-failed" and .response == null and .callError == null' <<<"${doc}" >/dev/null 2>&1
    then ok "connect failure: exit 3 and a result document"; else fail "connect failure: exit ${code}, ${doc}"; fi
fi

has_role server || { finish; exit $?; }

# ---- server: readiness, events, identity, clean stop ----
srv="srv-${RUN_ID}"
start "${srv}" "${ADAPTER}" -- server --version 1.6
if wait_ready "${srv}"; then ok "server becomes ready"; else fail "server did not become ready"; finish; exit 1; fi
first=$(events "${srv}" | head -1)
if jq -e --arg a "${ADAPTER}" '.event == "ready" and .adapter == $a and .version == "1.6" and .port == 9000 and .fixture == "baseline" and (.time | type == "string")' <<<"${first}" >/dev/null 2>&1
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

doc=$(docker exec "${srv}" ocpp-interop call --station "${STATION}" Reset --payload '{"type":"Soft"}' 2>/dev/null); code=$?
if [[ "${code}" == 1 ]] && jq -e '.ok == false and .error.code == "not-connected"' <<<"${doc}" >/dev/null 2>&1
then ok "call to a station that is not connected: not-connected, exit 1"; else fail "call to a station that is not connected: exit ${code}, ${doc}"; fi

if has_role client; then
    doc=$(run_client "${ADAPTER}" Heartbeat --version 1.6 --url "ws://${srv}:9000/ocpp"); code=$?
    if [[ "${code}" == 0 ]] && jq -e --arg a "${ADAPTER}" "${JQ_LIB}"'
            .schemaVersion == "1.0" and .adapter == $a and .version == "1.6" and .operation == "Heartbeat"
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
    doc=$(run_client "${ADAPTER}" Heartbeat --version 1.6 --url "ws://${srv}:9000/ocpp" --station NOT-IN-THE-FIXTURE); code=$?
    if [[ "${code}" == 3 ]] && jq -e '.error.code == "connect-failed"' <<<"${doc}" >/dev/null 2>&1
    then ok "unknown station identity: connect-failed, exit 3"; else fail "unknown station identity: exit ${code}, ${doc}"; fi
    if wait_event "${srv}" '.event == "connection-rejected" and .station == "NOT-IN-THE-FIXTURE" and .reason == "unknown-station"' >/dev/null
    then ok "server event: connection-rejected unknown-station"; else fail "no connection-rejected event for an unknown station"; fi
fi
if events "${srv}" | grep -v '^{' | grep -q .; then fail "server stdout has lines that are not JSON"; else ok "server stdout is JSON Lines only"; fi

# A second server in the same container: the port is taken.
docker exec "${srv}" ocpp-interop server --version 1.6 --ready-file /tmp/second --control-socket /tmp/second.sock >/dev/null 2>&1
code=$?
if [[ "${code}" != 0 ]]; then ok "second server on a busy port fails (exit ${code})"; else fail "second server on a busy port exited 0"; fi

docker stop -t 15 "${srv}" >/dev/null
code=$(docker inspect --format '{{.State.ExitCode}}' "${srv}")
exited "${srv}"
last=$(events "${srv}" | tail -1)
if [[ "${code}" == 0 ]]; then ok "SIGTERM: exit 0"; else fail "SIGTERM: exit ${code}"; fi
if jq -e '.event == "stopped"' <<<"${last}" >/dev/null 2>&1; then ok "stopped event"; else fail "last stdout line is not the stopped event: ${last}"; fi
stop_container "${srv}"

has_role client || { finish; exit $?; }

# ---- client run: hold, control socket, lost connection, timeouts ----
start "${srv}" "${ADAPTER}" -- server --version 1.6
wait_ready "${srv}" || { fail "server did not become ready"; finish; exit 1; }
sta="sta-${RUN_ID}"
start "${sta}" "${ADAPTER}" -- client run --version 1.6 --url "ws://${srv}:9000/ocpp"
if wait_ready "${sta}"; then ok "client run becomes ready"; else fail "client run did not become ready"; fi
if wait_event "${sta}" '.event == "ready" and .station == $s and (.url | endswith("/ocpp/" + $s))' --arg s "${STATION}" >/dev/null
then ok "client run: ready event"; else fail "client run: no ready event"; fi
doc=$(docker exec "${sta}" ocpp-interop call BootNotification --payload "$(jq -c '.samples.boot.payload' "${FIXTURE}")" 2>/dev/null); code=$?
if [[ "${code}" == 0 ]] && jq -e '.ok and .response.status == "Accepted" and .response.interval == 300' <<<"${doc}" >/dev/null 2>&1
then ok "call from client run"; else fail "call from client run: exit ${code}, ${doc}"; fi
doc=$(docker exec "${sta}" ocpp-interop call --station SOMEONE-ELSE Heartbeat 2>/dev/null); code=$?
if [[ "${code}" == 1 ]] && jq -e '.error.code == "not-connected"' <<<"${doc}" >/dev/null 2>&1
then ok "call from client run for another station: not-connected"; else fail "call for another station: exit ${code}, ${doc}"; fi
doc=$(docker exec "${srv}" ocpp-interop call --station "${STATION}" Reset --payload '{"type":"Soft"}' 2>/dev/null); code=$?
if [[ "${code}" == 0 ]] && jq -e '.ok and .response == {"status":"Accepted"}' <<<"${doc}" >/dev/null 2>&1
then ok "call from the server to the connected station"; else fail "call from the server: exit ${code}, ${doc}"; fi
doc=$(docker exec "${srv}" ocpp-interop call --station "${STATION}" Reset --payload '{"type":"Sideways"}' 2>/dev/null); code=$?
if [[ "${code}" == 1 ]] && jq -e '.ok == false and .error.code == "invalid-request"' <<<"${doc}" >/dev/null 2>&1
then ok "a request the upstream refuses: invalid-request, exit 1"; else fail "invalid request: exit ${code}, ${doc}"; fi

# A station that does not answer: the CSMS's call times out.
docker pause "${sta}" >/dev/null
doc=$(docker exec "${srv}" ocpp-interop call --station "${STATION}" Reset --payload '{"type":"Soft"}' --timeout-ms 1000 2>/dev/null); code=$?
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
start "${srv}" "${ADAPTER}" -- server --version 1.6
wait_ready "${srv}" || { fail "server did not become ready"; finish; exit 1; }
start "${sta}" "${ADAPTER}" -- client run --version 1.6 --url "ws://${srv}:9000/ocpp" --hold-ms 1500
for _ in $(seq 1 100); do [[ "$(docker inspect -f '{{.State.Running}}' "${sta}")" == "true" ]] || break; sleep 0.2; done
code=$(docker inspect --format '{{.State.ExitCode}}' "${sta}")
exited "${sta}"
if [[ "${code}" == 0 ]] && events "${sta}" | tail -1 | jq -e '.event == "stopped"' >/dev/null 2>&1
then ok "client run --hold-ms: stopped, exit 0"; else fail "client run --hold-ms: exit ${code}"; fi
if wait_event "${srv}" '.event == "connection-closed" and .station == $s' --arg s "${STATION}" >/dev/null
then ok "server sees the held connection close"; else fail "server did not report the close of the held connection"; fi
stop_container "${sta}"

# SIGTERM to client run: exit 0.
start "${sta}" "${ADAPTER}" -- client run --version 1.6 --url "ws://${srv}:9000/ocpp"
wait_ready "${sta}" || fail "client run did not become ready"
docker stop -t 15 "${sta}" >/dev/null
code=$(docker inspect --format '{{.State.ExitCode}}' "${sta}")
exited "${sta}"
if [[ "${code}" == 0 ]] && events "${sta}" | tail -1 | jq -e '.event == "stopped"' >/dev/null 2>&1
then ok "client run SIGTERM: stopped, exit 0"; else fail "client run SIGTERM: exit ${code}"; fi
if events "${sta}" | grep -v '^{' | grep -q .; then fail "client run stdout has lines that are not JSON"; else ok "client run stdout is JSON Lines only"; fi
stop_container "${sta}"; stop_container "${srv}"

# client run that cannot connect: exit 3.
start "${sta}" "${ADAPTER}" -- client run --version 1.6 --url "ws://no-such-host-${RUN_ID}:9000/ocpp" --connect-timeout-ms 1500
for _ in $(seq 1 100); do [[ "$(docker inspect -f '{{.State.Running}}' "${sta}")" == "true" ]] || break; sleep 0.2; done
code=$(docker inspect --format '{{.State.ExitCode}}' "${sta}")
exited "${sta}"
if [[ "${code}" == 3 ]] && events "${sta}" | head -1 | jq -e '.event == "connection-failed"' >/dev/null 2>&1
then ok "client run that cannot connect: connection-failed, exit 3"; else fail "client run that cannot connect: exit ${code}, events: $(events "${sta}" | tr '\n' ' ')"; fi
stop_container "${sta}"

finish
