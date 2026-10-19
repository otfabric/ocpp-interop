# Capabilities

`ocpp-interop print-capabilities` prints one JSON line, which validates
against [schemas/capabilities.schema.json](../schemas/capabilities.schema.json).
A consumer reads it to decide which scenarios to run against an image, and
reports the rest as `SKIP_UNSUPPORTED`.

## Rules

1. A capability is `true`, or an action is listed, only when the self-tests
   of this repository exercise it (REQUIREMENTS.md R4.2). An upstream's
   documentation alone is not enough.
2. Capabilities are declared per OCPP version and per role. Support for a
   feature in 1.6 says nothing about 2.0.1.
3. `sends` lists the actions the adapter can send in that role, and
   `handles` the actions it answers. An action missing from these lists is
   unsupported.
4. `notes` explains limitations in plain words, with a link to evidence
   where there is one.

## Declared now

`mobilityhouse`, `chargetime` and `lorenzodonini` declare OCPP 1.6 and
2.0.1 in both roles, `mobilityhouse` also OCPP 2.1, `everest` OCPP 1.6,
2.0.1 and 2.1 as a station only, and `citrineos` the same three as a CSMS only, exactly what
[scripts/smoke.sh](../scripts/smoke.sh) and
[scripts/interop.sh](../scripts/interop.sh) exercise. Each role sends every
action of the version that the specification gives to it (the direction
tables of [scripts/validate-fixtures.py](../scripts/validate-fixtures.py):
OCPP 1.6 with the Security Whitepaper messages, 2.0.1 Part 2, 2.1 Edition 2
Part 2), and handles every action the other role sends. The baseline
fixtures have a sample and an answer for each, so the self-tests send every
declared action and check its answer.

| Declared actions | OCPP 1.6 (39) | OCPP 2.0.1 (64) | OCPP 2.1 (91) |
|---|---|---|---|
| Station sends | 14, all four stations | 25, all four stations | 35, mobilityhouse and everest |
| Station handles | 26, all four stations | 40, all four stations | 55, mobilityhouse; 54, everest |
| CSMS sends | 26, all three adapters | 40, all three adapters | 55, mobilityhouse |
| CSMS handles | 14, all three adapters | 25, all three adapters | 36, mobilityhouse |

The 2.1 station lists differ: mobilityhouse does not send BatterySwap
(below), everest does not send GetCertificateChainStatus nor answer
NotifyWebPaymentStarted (libocpp's message queue does not know them,
[adapters/everest/README.md](../adapters/everest/README.md)); neither
sends NotifyPeriodicEventStream (a SEND).

DataTransfer goes both ways and is counted in each direction. The pinned
releases of Java-OCA-OCPP (v2.0) and ocpp-go (v0.19.0) have a request and a
response type for every 1.6 and 2.0.1 action; ocpp 2.1.0 has one for every
action of the three versions except NotifyPeriodicEventStream.

Not declared by mobilityhouse, in OCPP 2.1 (the station's 37 actions less
these two):

- BatterySwap, by the station: ocpp 2.1.0 cannot send it (it writes the
  battery fields `soC` and `soH` as `SoC` and `soh`). The CSMS handles it
  (since v0.4.1): the everest station sends it.
- NotifyPeriodicEventStream: a SEND
  ([CONTAINER_CONTRACT.md](CONTAINER_CONTRACT.md#ocpp-21-send-and-callresulterror)),
  neither sent nor handled.

A declared action can still fail with a particular peer, or with any peer,
because of an upstream defect; those are in
[tests/known-bugs.tsv](../tests/known-bugs.tsv) and
[COMPATIBILITY.md](COMPATIBILITY.md) (for example GetCompositeSchedule with
ocpp-go 2.0.1, whose response type does not have the OCA structure).

Before the baseline fixtures covered every action (contract 0.4, v0.2.0),
the adapters declared 11, 19, 20 and 10 actions in 1.6 (station sends,
station handles, CSMS sends, CSMS handles), 13, 14, 15 and 12 in 2.0.1, and
18, 23, 24 and 17 in 2.1.

`citrineos` (CitrineOS) is a CSMS only, for 1.6, 2.0.1 and 2.1: its
`client` is `supported: false` with empty lists. CitrineOS is an
application that answers every CALL itself, so its lists are what the
self-tests found, not the whole of each direction: the CSMS sends 17
actions in 1.6, 37 in 2.0.1 and 38 in 2.1 (those its REST message API has
an endpoint for, whose baseline sample it sends), and handles 8 in 1.6 and
17 in 2.0.1 and 2.1 (those whose answer is the fixture's).
[adapters/citrineos/README.md](../adapters/citrineos/README.md) says why each
other action is left out.

`everest` (EVerest libocpp) is a station only: it declares `client` for
1.6, 2.0.1 and 2.1 with the station lists above (in 2.1 less
GetCertificateChainStatus and NotifyWebPaymentStarted); its `server` is
`supported: false` with empty lists. It sends BatterySwap (libocpp writes
the battery fields `soC` and `soH` as the schema names them), which the
mobilityhouse CSMS answers. Up to v0.4.0 it declared only 11 and 19, 13
and 14, 18 and 23 actions (sends and handles in 1.6, 2.0.1, 2.1).

The features differ per adapter; they are the same in every version it
declares, except `compression` for chargetime:

| | mobilityhouse | chargetime | lorenzodonini | everest | citrineos |
|---|---|---|---|---|---|
| `securityProfiles` | 0, 1, 2, 3 | 0, 1, 2, 3 | 0, 1, 2, 3 | 0, 1, 2, 3 | 0, 1, 2, 3 |
| `compression` | true (1.6, 2.0.1, 2.1) | 2.0.1: true, with known upstream defects in both roles; 1.6: false | false: the CSMS cannot accept it | false: libocpp cannot offer it | true: the CSMS accepts it (no station role) |
| `fixtureCallErrors` | true | false | true | true | false |
| `fixtureAnswers` | (not declared: true) | (true) | (true) | (true) | false |
| `messageIds` | true | false | true | true | true |
| `send`, `callResultError` | false | false | false | false | false |

Each adapter explains its limitations in `notes`, per version. Known
upstream defects that the self-tests show are in
[tests/known-bugs.tsv](../tests/known-bugs.tsv) and
[COMPATIBILITY.md](COMPATIBILITY.md); a declared action or feature can
still fail against a particular peer because of one (for example,
compression with the chargetime 2.0.1 CSMS or station).

## Example

```json
{
  "schemaVersion": "1.0",
  "adapter": "mobilityhouse",
  "adapterVersion": "v0.1.0",
  "contractVersion": "0.5",
  "upstream": {
    "name": "The Mobility House ocpp",
    "repository": "https://github.com/mobilityhouse/ocpp",
    "version": "2.1.0",
    "revision": "91f4f89140a2198dc5a300cfea88d947ac91ae28",
    "license": "MIT",
    "language": "Python",
    "independentEngine": true
  },
  "versions": {
    "1.6": {
      "client": {"supported": true, "sends": ["BootNotification", "Heartbeat"], "handles": ["Reset"]},
      "server": {"supported": true, "sends": ["Reset"], "handles": ["BootNotification", "Heartbeat"]},
      "features": {"securityProfiles": [0, 1, 2, 3], "compression": true, "send": false, "callResultError": false,
                   "fixtureCallErrors": true, "messageIds": true},
      "notes": ["..."]
    }
  }
}
```

(Shortened: `docker run --rm <image> print-capabilities` prints the full
document.) `upstream.revision` is the commit of the upstream release; for a
package installed from a registry (PyPI, Maven Central) it is the commit of
the release's git tag, which the build records but cannot verify from the
artifact.

## Feature keys

| Key | Meaning |
|---|---|
| `securityProfiles` | Profiles exercised, in both roles: 0 (none), 1 (Basic over `ws://`), 2 (TLS + Basic), 3 (mutual TLS); see [Transport security](CONTAINER_CONTRACT.md#transport-security) |
| `compression` | `permessage-deflate`: the station offers it with `--compression` and the CSMS accepts it, and it is used; both roles ([Compression](CONTAINER_CONTRACT.md#compression)) |
| `send` | OCPP 2.1 SEND frames (message type 6) sent and received. `false` everywhere: no pinned upstream release implements SEND |
| `callResultError` | OCPP 2.1 CALLRESULTERROR frames (message type 5) sent and received. `false` everywhere, for the same reason |
| `fixtureCallErrors` | The adapter answers with the CALLERROR a fixture's `callError` answer names. Without it, such a fixture exits 5 |
| `fixtureAnswers` | Every answer is the fixture's, and an action the fixture does not answer is answered with `NotImplemented`. Not declared means `true`. `false` (citrineos): the upstream is an application that answers by itself; `handles` then lists only the actions whose answer the self-tests found equal to the fixture's, and the `NotImplemented` test against it is `SKIP_UNSUPPORTED` |
| `messageIds` | Result documents and the `call-sent` and `call-result` events carry the message id of the CALL the upstream sent |

Further boolean keys may be added in a minor version of the contract.
