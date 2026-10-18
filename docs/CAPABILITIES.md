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
2.0.1 in both roles, `mobilityhouse` also OCPP 2.1, and `everest` OCPP 1.6,
2.0.1 and 2.1 as a station only, exactly what
[scripts/smoke.sh](../scripts/smoke.sh) and
[scripts/interop.sh](../scripts/interop.sh) exercise. For 1.6 and 2.0.1 the
action lists are the same for every adapter:

| | OCPP 1.6 | OCPP 2.0.1 |
|---|---|---|
| Station sends | Authorize, BootNotification, DataTransfer, DiagnosticsStatusNotification, FirmwareStatusNotification, Heartbeat, MeterValues, SecurityEventNotification, StartTransaction, StatusNotification, StopTransaction | Authorize, BootNotification, DataTransfer, FirmwareStatusNotification, Heartbeat, LogStatusNotification, MeterValues, NotifyEvent, NotifyReport, ReservationStatusUpdate, SecurityEventNotification, StatusNotification, TransactionEvent |
| Station handles | CancelReservation, ChangeAvailability, ChangeConfiguration, ClearCache, ClearChargingProfile, DataTransfer, GetCompositeSchedule, GetConfiguration, GetDiagnostics, GetLocalListVersion, RemoteStartTransaction, RemoteStopTransaction, ReserveNow, Reset, SendLocalList, SetChargingProfile, TriggerMessage, UnlockConnector, UpdateFirmware | ChangeAvailability, ClearCache, ClearChargingProfile, DataTransfer, GetBaseReport, GetLocalListVersion, GetVariables, RequestStartTransaction, RequestStopTransaction, Reset, SetChargingProfile, SetVariables, TriggerMessage, UnlockConnector |
| CSMS sends | the station's handled actions, and ExtendedTriggerMessage | the station's handled actions, and CancelReservation |
| CSMS handles | the station's sent actions except SecurityEventNotification | the station's sent actions except ReservationStatusUpdate |

The actions the other end does not handle are listed as sent because the
self-tests send them and check that the other end answers `NotImplemented`
(the baseline fixtures have no answer for them).

OCPP 2.1 (`mobilityhouse` in both roles, `everest` as a station: the pinned
releases of the other upstreams have no 2.1):

| | OCPP 2.1 |
|---|---|
| Station sends | the 2.0.1 list, and ClosePeriodicEventStream, NotifyDERAlarm, NotifyDERStartStop, OpenPeriodicEventStream, ReportDERControl |
| Station handles | the 2.0.1 list, and AdjustPeriodicEventStream, ClearDERControl, ClearTariffs, GetDERControl, GetPeriodicEventStream, GetTariffs, RequestBatterySwap, SetDERControl, SetDefaultTariff |
| CSMS sends | the station's handled actions, and CancelReservation |
| CSMS handles | the station's sent actions except ReservationStatusUpdate |

`everest` (EVerest libocpp) is a station only: it declares `client` for
1.6, 2.0.1 and 2.1 with the station lists above; its `server` is
`supported: false` with empty lists. It can send BatterySwap (libocpp
writes the battery fields `soC` and `soH` as the schema names them, and the
mobilityhouse CSMS accepts the request), but does not declare it: no
CSMS declares that it handles BatterySwap, so no self-test runs it.

BatterySwap is not declared by `mobilityhouse`: ocpp 2.1.0 cannot send it (it writes the
battery fields `soC` and `soH` as `SoC` and `soh`), so no self-test reaches
a CSMS with it. NotifyPeriodicEventStream is a SEND and is not declared
either ([CONTAINER_CONTRACT.md](CONTAINER_CONTRACT.md#ocpp-21-send-and-callresulterror)).

The features differ per adapter; they are the same in every version it
declares:

| | mobilityhouse | chargetime | lorenzodonini | everest |
|---|---|---|---|---|
| `securityProfiles` | 0, 1 | 0, 1 | 0, 1 | 0, 1 |
| `fixtureCallErrors` | true | false | true | true |
| `messageIds` | true | false | true | true |
| `compression`, `send`, `callResultError` | false | false | false | false |

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
