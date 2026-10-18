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

All three adapters declare OCPP 1.6 and 2.0.1 in both roles, and
`mobilityhouse` also OCPP 2.1, exactly what
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
| Station sends | 14, all three adapters | 25, all three adapters | 35, mobilityhouse |
| Station handles | 26, all three adapters | 40, all three adapters | 55, mobilityhouse |
| CSMS sends | 26, all three adapters | 40, all three adapters | 55, mobilityhouse |
| CSMS handles | 14, all three adapters | 25, all three adapters | 35, mobilityhouse |

DataTransfer goes both ways and is counted in each direction. The pinned
releases of Java-OCA-OCPP (v2.0) and ocpp-go (v0.19.0) have a request and a
response type for every 1.6 and 2.0.1 action; ocpp 2.1.0 has one for every
action of the three versions except NotifyPeriodicEventStream.

Not declared, in OCPP 2.1 (the station's 37 actions less these two):

- BatterySwap: ocpp 2.1.0 cannot send it (it writes the battery fields
  `soC` and `soH` as `SoC` and `soh`), so no self-test reaches a CSMS with
  it; the CSMS does not declare it either.
- NotifyPeriodicEventStream: a SEND
  ([CONTAINER_CONTRACT.md](CONTAINER_CONTRACT.md#ocpp-21-send-and-callresulterror)).

A declared action can still fail with a particular peer, or with any peer,
because of an upstream defect; those are in
[tests/known-bugs.tsv](../tests/known-bugs.tsv) and
[COMPATIBILITY.md](COMPATIBILITY.md) (for example GetCompositeSchedule with
ocpp-go 2.0.1, whose response type does not have the OCA structure).

Before the baseline fixtures covered every action (contract 0.4, v0.2.0),
the adapters declared 11, 19, 20 and 10 actions in 1.6 (station sends,
station handles, CSMS sends, CSMS handles), 13, 14, 15 and 12 in 2.0.1, and
18, 23, 24 and 17 in 2.1.

The features differ per adapter; they are the same in every version it
declares:

| | mobilityhouse | chargetime | lorenzodonini |
|---|---|---|---|
| `securityProfiles` | 0, 1 | 0, 1 | 0, 1 |
| `fixtureCallErrors` | true | false | true |
| `messageIds` | true | false | true |
| `compression`, `send`, `callResultError` | false | false | false |

Each adapter explains its limitations in `notes`, per version. Known
upstream defects that the self-tests show are in
[tests/known-bugs.tsv](../tests/known-bugs.tsv) and
[COMPATIBILITY.md](COMPATIBILITY.md); a declared action can still fail
against a particular peer because of one (for example, the chargetime
2.0.1 CSMS with a station that offers compression).

## Example

```json
{
  "schemaVersion": "1.0",
  "adapter": "mobilityhouse",
  "adapterVersion": "v0.1.0",
  "contractVersion": "0.4",
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
      "features": {"securityProfiles": [0, 1], "compression": false, "send": false, "callResultError": false,
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
| `securityProfiles` | Profiles exercised: 0 (none), 1 (Basic over `ws://`), 2 (TLS + Basic), 3 (mutual TLS) |
| `compression` | `permessage-deflate` negotiated and used |
| `send` | OCPP 2.1 SEND frames (message type 6) sent and received. `false` everywhere: no pinned upstream release implements SEND |
| `callResultError` | OCPP 2.1 CALLRESULTERROR frames (message type 5) sent and received. `false` everywhere, for the same reason |
| `fixtureCallErrors` | The adapter answers with the CALLERROR a fixture's `callError` answer names. Without it, such a fixture exits 5 |
| `messageIds` | Result documents and the `call-sent` and `call-result` events carry the message id of the CALL the upstream sent |

Further boolean keys may be added in a minor version of the contract.
