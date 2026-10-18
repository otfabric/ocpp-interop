#!/usr/bin/env bash
# Shared helpers for the self-test scripts. Source, do not execute.
#
# Servers and stations run as containers on a private Docker network and reach
# each other by container name, so no host port is published and parallel runs
# do not collide.

REGISTRY="${REGISTRY:-ghcr.io/otfabric}"
VERSION="${VERSION:-dev}"
RUN_ID="${RUN_ID:-$$}"
NETWORK="ocpp-interop-${RUN_ID}"
READY_FILE="/run/ocpp-interop/ready"
WAIT_TIMEOUT="${WAIT_TIMEOUT:-60}"

PASS=0
FAIL=0
KNOWN=0
SKIPPED=0

log()  { echo "[$(basename "$0" .sh)] $*" >&2; }
ok()   { PASS=$((PASS + 1)); [[ -n "${VERBOSE:-}" ]] && log "ok: ${CHECK_PREFIX:-}$*"; return 0; }
fail() { FAIL=$((FAIL + 1)); log "FAIL: ${CHECK_PREFIX:-}$*"; return 0; }

image_of() { echo "${REGISTRY}/ocpp-interop-$1:${VERSION}"; }

require_tools() {
    local t
    for t in docker jq; do
        command -v "$t" >/dev/null || { log "missing required tool: $t"; exit 2; }
    done
}

network_up() { docker network create "${NETWORK}" >/dev/null; }

cleanup() {
    docker ps -aq --filter "label=ocpp-interop.run=${RUN_ID}" | xargs -r docker rm -f >/dev/null 2>&1
    docker network rm "${NETWORK}" >/dev/null 2>&1
    return 0
}

# start <name> <adapter> [docker run options --] <command and args...>
# Starts a detached container on the test network.
start() {
    local name="$1" adapter="$2"; shift 2
    local opts=()
    if [[ " $* " == *" -- "* ]]; then
        while [[ "$1" != "--" ]]; do opts+=("$1"); shift; done
        shift
    fi
    docker rm -f "${name}" >/dev/null 2>&1
    docker run -d --name "${name}" --network "${NETWORK}" --label "ocpp-interop.run=${RUN_ID}" \
        ${opts[@]+"${opts[@]}"} "$(image_of "${adapter}")" "$@" >/dev/null
}

# wait_ready <container>: the ready file exists only once the server listens,
# or the station's connection is open. No fixed delays.
wait_ready() {
    local name="$1" waited=0 state
    while true; do
        state=$(docker inspect --format '{{.State.Status}}' "${name}" 2>/dev/null || echo absent)
        if [[ "${state}" != "running" ]]; then
            log "${name} is ${state}"; docker logs --tail 20 "${name}" >&2 2>/dev/null; return 1
        fi
        docker exec "${name}" test -f "${READY_FILE}" 2>/dev/null && return 0
        if (( waited >= WAIT_TIMEOUT * 5 )); then
            log "${name} not ready after ${WAIT_TIMEOUT}s"; docker logs --tail 20 "${name}" >&2; return 1
        fi
        sleep 0.2; waited=$((waited + 1))
    done
}

stop_container() { docker rm -f "$1" >/dev/null 2>&1; return 0; }

# The scripts run with pipefail. A reader that stops early (head -1) makes
# the writer die of SIGPIPE when it is still writing, and pipefail then
# fails the whole pipeline although the reader got its line: take the first
# line with sed -n 1p, which reads to the end.

# events <container>: the JSON Lines a container wrote to stdout.
events() { docker logs "$1" 2>/dev/null; }

# wait_event <container> <jq condition> [args for jq...]: prints the first
# event that satisfies the condition, waiting up to 5 s for it.
wait_event() {
    local name="$1" condition="$2"; shift 2
    local i found
    for i in $(seq 1 50); do
        found=$(events "${name}" | jq -c "$@" "select(${condition})" 2>/dev/null | sed -n 1p)
        [[ -n "${found}" ]] && { echo "${found}"; return 0; }
        (( i < 50 )) && sleep 0.1
    done
    return 1
}

# exited <container>: waits (up to 5 s) until the stopped event, the last
# line an adapter writes, has reached the container's log. Docker can report
# a container as exited before its log copier has written the last lines, so
# reading the log right after the exit may miss them.
exited() { wait_event "$1" '.event == "stopped"' >/dev/null || true; }

# run_client <adapter> [docker run options --] <client args...>: prints the
# result document of a one-shot client and returns the adapter's exit code.
run_client() {
    local adapter="$1"; shift
    local opts=()
    if [[ " $* " == *" -- "* ]]; then
        while [[ "$1" != "--" ]]; do opts+=("$1"); shift; done
        shift
    fi
    docker run --rm --network "${NETWORK}" --label "ocpp-interop.run=${RUN_ID}" \
        ${opts[@]+"${opts[@]}"} "$(image_of "${adapter}")" client "$@" 2>/dev/null
}

# JSON helpers for jq, shared by the tests:
#   norm   date-times without a zero fraction of a second, so that
#          2026-01-01T10:00:00.000Z and 2026-01-01T10:00:00Z compare equal
#   within(want; got)  every field of want is in got with an equal value
#          (numbers compare by value); "${now}" in want stands for any RFC 3339
#          UTC date-time. Arrays must have the same length.
# shellcheck disable=SC2016,SC2034
JQ_LIB='
def isdate: type == "string" and test("^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\\.[0-9]+)?(Z|[+-][0-9]{2}:[0-9]{2})$");
def norm: if isdate then sub("\\.0+(?=Z$)"; "") | sub("\\+00:00$"; "Z") else . end;
def within($want; $got):
  if ($want | type) == "object" then
    ($got | type) == "object" and ([$want | to_entries[] | . as $e | ($got | has($e.key)) and within($e.value; $got[$e.key])] | all)
  elif ($want | type) == "array" then
    ($got | type) == "array" and ($want | length) == ($got | length)
    and ([range(0; $want | length) as $i | within($want[$i]; $got[$i])] | all)
  elif $want == "${now}" then ($got | isdate)
  elif ($want | type) == "string" then ($got | type) == "string" and ($want | norm) == ($got | norm)
  else $want == $got end;
'

finish() {
    echo "" >&2
    local extra=""
    [[ "${KNOWN}" -gt 0 ]] && extra="${extra}, ${KNOWN} known upstream bugs"
    [[ "${SKIPPED}" -gt 0 ]] && extra="${extra}, ${SKIPPED} skipped (capability not declared)"
    log "${PASS} passed, ${FAIL} failed${extra}"
    [[ "${FAIL}" -eq 0 ]]
}
