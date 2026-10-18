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

Both adapters declare OCPP 1.6 in both roles, exactly what
[scripts/smoke.sh](../scripts/smoke.sh) and
[scripts/interop.sh](../scripts/interop.sh) exercise:

| | mobilityhouse | chargetime |
|---|---|---|
| Station sends | Authorize, BootNotification, DataTransfer, DiagnosticsStatusNotification, FirmwareStatusNotification, Heartbeat, MeterValues, SecurityEventNotification, StartTransaction, StatusNotification, StopTransaction | same |
| Station handles | CancelReservation, ChangeAvailability, ChangeConfiguration, ClearCache, ClearChargingProfile, DataTransfer, GetCompositeSchedule, GetConfiguration, GetDiagnostics, GetLocalListVersion, RemoteStartTransaction, RemoteStopTransaction, ReserveNow, Reset, SendLocalList, SetChargingProfile, TriggerMessage, UnlockConnector, UpdateFirmware | same |
| CSMS sends | the station's handled actions, and ExtendedTriggerMessage | same |
| CSMS handles | the station's sent actions except SecurityEventNotification | same |
| `securityProfiles` | 0, 1 | 0, 1 |
| `fixtureCallErrors` | true | false |
| `messageIds` | true | false |
| `compression`, `send`, `callResultError` | false | false |

SecurityEventNotification and ExtendedTriggerMessage are listed as sent
because the self-tests send them and check that the other end answers
`NotImplemented` (the baseline fixture has no answer for them). Each adapter
explains its limitations in `notes`; the mobilityhouse station cannot send a
SecurityEventNotification without `techInfo` (see
[tests/known-bugs.tsv](../tests/known-bugs.tsv)).

## Example

```json
{
  "schemaVersion": "1.0",
  "adapter": "mobilityhouse",
  "adapterVersion": "v0.1.0",
  "contractVersion": "0.2",
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
| `send` | OCPP 2.1 SEND frames sent and received |
| `callResultError` | OCPP 2.1 CALLRESULTERROR frames sent and received |
| `fixtureCallErrors` | The adapter answers with the CALLERROR a fixture's `callError` answer names. Without it, such a fixture exits 5 |
| `messageIds` | Result documents and the `call-sent` and `call-result` events carry the message id of the CALL the upstream sent |

Further boolean keys may be added in a minor version of the contract.
