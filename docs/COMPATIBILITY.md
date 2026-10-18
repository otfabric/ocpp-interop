# Compatibility

Results of the reference-to-reference self-tests
([scripts/interop.sh](../scripts/interop.sh)): every station (`client`)
against every CSMS (`server`), for OCPP 1.6J, 2.0.1 and 2.1, with CALLs sent by
the station and by the CSMS. The matrix below is the output of `make matrix` (from
`reports/report-interop.json`, written by `make documents`); CI writes the
same matrix for each architecture into its job summary.

Run: 2026-10-18, images built from this repository on linux/arm64
(macOS, Docker 29.8), `make ci`. The linux/amd64 images have not been
self-tested locally; CI does that natively.

`everest` (EVerest libocpp, everest-core 2026.02.1) is a station only. Its
rows were added by a run on 2026-10-18 (linux/arm64, `scripts/smoke.sh
everest` and `scripts/interop.sh mobilityhouse chargetime lorenzodonini
everest`: 1099 PASS, 45 KNOWN_ISSUE, 44 SKIP_UNSUPPORTED, 0 FAIL); the other
cells of that run are the ones below. Every pairing with `everest` as the
CSMS is one `pairing` SKIP_UNSUPPORTED (not in the tables).

Each cell: results of the CALLs and checks initiated by the station / by
the CSMS. A 1.6 pairing has 22 of each: one per fixture sample, two derived
answers (an unknown token, unknown configuration keys), connectivity,
security profile 1 and the fixture's CALLERROR answers. A 2.0.1 pairing has
25 initiated by the station and 18 by the CSMS: one per sample, the derived
answers (an unknown token; unknown component and variable with
case-insensitive names; a read-only variable), connectivity, security
profile 1 and the CALLERROR answers. A 2.1 pairing has 33 initiated by the
station and 28 by the CSMS: the 2.0.1 tests, one per 2.1 sample, the SEND
sample and CALLRESULTERROR (from each end).

## OCPP 1.6J

| Station ↓ / CSMS → | chargetime v2.0 | lorenzodonini v0.19.0 | mobilityhouse 2.1.0 |
|---|---|---|---|
| chargetime v2.0 | 21 pass 1 skip / 21 pass 1 skip | 22 pass / 21 pass 1 skip | 22 pass / 21 pass 1 skip |
| lorenzodonini v0.19.0 | 21 pass 1 skip / 22 pass | 22 pass / 22 pass | 22 pass / 22 pass |
| mobilityhouse 2.1.0 | 20 pass 1 skip 1 known / 22 pass | 21 pass 1 known / 22 pass | 21 pass 1 known / 22 pass |
| everest 2026.02.1 | 21 pass 1 skip / 22 pass | 22 pass / 22 pass | 22 pass / 22 pass |

Without everest: totals 387 PASS, 3 KNOWN_ISSUE, 6 SKIP_UNSUPPORTED, 0 FAIL, 0 INFRA_ERROR.

| Station | CSMS | Test | Status | Evidence |
|---|---|---|---|---|
| mobilityhouse | all three | `v16.from-station.security-event-minimal` | KNOWN_ISSUE | [mobilityhouse/ocpp `v16/call.py`](https://github.com/mobilityhouse/ocpp/blob/91f4f89140a2198dc5a300cfea88d947ac91ae28/ocpp/v16/call.py#L232-L236) |
| chargetime | all three | `v16.fixture-callerror.from-csms` | SKIP_UNSUPPORTED | chargetime does not declare `fixtureCallErrors` |
| all three | chargetime | `v16.fixture-callerror.from-station` | SKIP_UNSUPPORTED | chargetime does not declare `fixtureCallErrors` |

## OCPP 2.0.1

| Station ↓ / CSMS → | chargetime v2.0 | lorenzodonini v0.19.0 | mobilityhouse 2.1.0 |
|---|---|---|---|
| chargetime v2.0 | 24 pass 1 skip / 17 pass 1 skip | 25 pass / 17 pass 1 skip | 25 pass / 17 pass 1 skip |
| lorenzodonini v0.19.0 | 23 pass 1 skip 1 known / 18 pass | 24 pass 1 known / 18 pass | 24 pass 1 known / 17 pass 1 known |
| mobilityhouse 2.1.0 | 7 pass 1 skip 17 known / 1 pass 17 known | 25 pass / 18 pass | 25 pass / 18 pass |
| everest 2026.02.1 | 24 pass 1 skip / 18 pass | 25 pass / 18 pass | 25 pass / 18 pass |

Without everest: totals 343 PASS, 38 KNOWN_ISSUE, 6 SKIP_UNSUPPORTED, 0 FAIL, 0 INFRA_ERROR.

| Station | CSMS | Test | Status | Evidence |
|---|---|---|---|---|
| mobilityhouse | chargetime | `v201.from-station.*` after the first CALL (16), `v201.from-csms.*` (17), `v201.connect.close` | KNOWN_ISSUE (34) | [Java-WebSocket 1.6.0 `PerMessageDeflateExtension`](https://github.com/TooTallNate/Java-WebSocket/blob/da3cf2a777aed862f2f5b5cf060cae7969958667/src/main/java/org/java_websocket/extensions/permessage_deflate/PerMessageDeflateExtension.java#L335-L340) |
| lorenzodonini | all three | `v201.from-station.notify-report` | KNOWN_ISSUE | [ocpp-go `notify_report.go`](https://github.com/lorenzodonini/ocpp-go/blob/a1eec917af884db0585752f909a61893aa667480/ocpp2.0.1/provisioning/notify_report.go#L98-L104) |
| lorenzodonini | mobilityhouse | `v201.from-csms.reset-immediate` | KNOWN_ISSUE | [ocpp-go `reset.go`](https://github.com/lorenzodonini/ocpp-go/blob/a1eec917af884db0585752f909a61893aa667480/ocpp2.0.1/provisioning/reset.go#L61-L64) |
| chargetime | all three | `v201.fixture-callerror.from-csms` | SKIP_UNSUPPORTED | chargetime does not declare `fixtureCallErrors` |
| all three | chargetime | `v201.fixture-callerror.from-station` | SKIP_UNSUPPORTED | chargetime does not declare `fixtureCallErrors` |

The mobilityhouse station and the chargetime 2.0.1 CSMS do connect, and
one-shot clients and the first CALL of a connection work (BootNotification
passes); every later CALL on the connection fails, because the station
cannot decompress the CSMS's second message. The chargetime station and the
lorenzodonini station do not offer compression and are not affected.

## OCPP 2.1

`mobilityhouse` declares 2.1 in both roles and `everest` as a station: the
pinned releases of Java-OCA-OCPP (v2.0) and ocpp-go (v0.19.0) have no 2.1, so
every pairing with them is one `v21.pairing` SKIP_UNSUPPORTED. The everest
station against the mobilityhouse CSMS is the pairing of two independent
2.1 implementations.

| Station ↓ / CSMS → | chargetime v2.0 | lorenzodonini v0.19.0 | mobilityhouse 2.1.0 |
|---|---|---|---|
| chargetime v2.0 | 1 skip / — | 1 skip / — | 1 skip / — |
| lorenzodonini v0.19.0 | 1 skip / — | 1 skip / — | 1 skip / — |
| mobilityhouse 2.1.0 | 1 skip / — | 1 skip / — | 29 pass 3 skip 1 known / 26 pass 1 skip 1 known |
| everest 2026.02.1 | 1 skip / — | 1 skip / — | 28 pass 3 skip 2 known / 27 pass 1 skip |

Without everest: totals 55 PASS, 2 KNOWN_ISSUE, 12 SKIP_UNSUPPORTED, 0 FAIL, 0 INFRA_ERROR.

| Station | CSMS | Test | Status | Evidence |
|---|---|---|---|---|
| mobilityhouse | mobilityhouse | `v21.from-station.reservation-status-update`, `v21.from-csms.cancel-reservation` | KNOWN_ISSUE (2) | [mobilityhouse/ocpp `charge_point.py`](https://github.com/mobilityhouse/ocpp/blob/91f4f89140a2198dc5a300cfea88d947ac91ae28/ocpp/charge_point.py#L161-L193) |
| mobilityhouse | mobilityhouse | `v21.from-station.battery-swap-in` | SKIP_UNSUPPORTED | BatterySwap is not declared: ocpp 2.1.0 cannot send it (see below) |
| mobilityhouse | mobilityhouse | `v21.send.notify-periodic-event-stream` | SKIP_UNSUPPORTED | no adapter declares `send` |
| mobilityhouse | mobilityhouse | `v21.rpc.call-result-error` (from each end) | SKIP_UNSUPPORTED (2) | no adapter declares `callResultError` |
| chargetime, lorenzodonini | all three | `v21.pairing` | SKIP_UNSUPPORTED (8) | 2.1 not declared by one of the two |
| everest | mobilityhouse | `v21.from-station.report-dercontrol` | KNOWN_ISSUE | [libocpp `ocpp::v2::MessageType`](https://github.com/EVerest/everest-core/blob/b61bb12b8cb89ee9d0fab037b05c3bb8cea9b13e/lib/everest/ocpp/include/ocpp/v2/types.hpp#L15-L195) has no ReportDERControl |
| everest | mobilityhouse | `v21.from-station.reservation-status-update` | KNOWN_ISSUE | the mobilityhouse defect above (no answer to an unanswered 2.1 action) |
| everest | mobilityhouse | `v21.from-station.battery-swap-in`, `v21.send.notify-periodic-event-stream`, `v21.rpc.call-result-error` (from each end) | SKIP_UNSUPPORTED (4) | everest does not declare BatterySwap (no CSMS declares that it handles it, so no test would run it); libocpp has no SEND and no CALLRESULTERROR |

Every smoke test ([scripts/smoke.sh](../scripts/smoke.sh)) passes: 128
checks for chargetime and lorenzodonini (1.6 and 2.0.1), 176 for
mobilityhouse (1.6, 2.0.1 and 2.1), 146 for everest (1.6, 2.0.1 and 2.1,
station checks against the mobilityhouse CSMS).

### What a PASS means

The sender's result document is the answer the fixture defines (every field
of the fixture's answer is there with an equal value; `"${now}"` is a date
and time), the request on the wire keeps every field of the sample, and
both ends report the CALL in their events: `call-sent` and `call-result` at
the sender, `call-received` with the request and `call-answered` with the
fixture's answer at the receiver, with the same message id where both
adapters expose it. Interoperability between two references says nothing
about conformance to the specification.

### Observations that are not failures

- Java-OCA-OCPP writes date-times with milliseconds
  (`2026-01-01T10:00:00.000Z`), decimals as `11000.0`, and adds
  `numberPhases: 3`, the default of the field, to a charging schedule
  period that has none. The other end reads all of them; the tests compare
  date-times and numbers by value.
- mobilityhouse writes the decimals of the SetChargingProfile and
  RemoteStartTransaction requests and of the GetCompositeSchedule response
  it sends with one fraction digit (`ocpp.messages._DecimalEncoder`). The
  fixture's limits are whole numbers, so the tests do not exercise it.

## Known upstream defects

| Adapter | Upstream | Defect | Evidence |
|---|---|---|---|
| mobilityhouse (station, 1.6) | ocpp 2.1.0 | `ocpp.v16.call.SecurityEventNotification` declares `tech_info` without a default: a SecurityEventNotification without the optional `techInfo` cannot be built, although the bundled OCA schema makes it optional. Still so on master (2026-10-04) | [source](https://github.com/mobilityhouse/ocpp/blob/91f4f89140a2198dc5a300cfea88d947ac91ae28/ocpp/v16/call.py#L232-L236) |
| chargetime (CSMS, 2.0.1), with a station that offers permessage-deflate | Java-OCA-OCPP v2.0 with Java-WebSocket 1.6.0 | `MultiProtocolJSONServer` enables permessage-deflate by default and keeps its compression context (`setServerNoContextTakeover(false)`), but Java-WebSocket 1.6.0 always answers `server_no_context_takeover`; the station cannot decompress the second message and closes the connection (1002). Java-WebSocket's master announces the parameter only when set | [source](https://github.com/TooTallNate/Java-WebSocket/blob/da3cf2a777aed862f2f5b5cf060cae7969958667/src/main/java/org/java_websocket/extensions/permessage_deflate/PerMessageDeflateExtension.java#L335-L340) |
| lorenzodonini (station, 2.0.1), with a CSMS that validates responses | ocpp-go v0.19.0 | `ResetResponse` (and `SetNetworkProfileResponse`) declare `statusInfo` without `omitempty`: the station sends `"statusInfo": null`, which the OCA schema does not allow | [source](https://github.com/lorenzodonini/ocpp-go/blob/a1eec917af884db0585752f909a61893aa667480/ocpp2.0.1/provisioning/reset.go#L61-L64) |
| mobilityhouse (both roles, 2.1) | ocpp 2.1.0 | `charge_point._raise_key_error` raises NotImplemented or NotSupported for 1.6, 2.0 and 2.0.1 only: a 2.1 CALL whose action has no handler gets no answer, and the sender times out. Fixed on master (ae716a1, #744), unreleased | [source](https://github.com/mobilityhouse/ocpp/blob/91f4f89140a2198dc5a300cfea88d947ac91ae28/ocpp/charge_point.py#L161-L193) |
| mobilityhouse (station, 2.1) | ocpp 2.1.0 | `snake_to_camel_case` writes the `soc` and `soh` fields of `BatteryDataType` as `SoC` and `soh` (the schema has `soC`, `soH`), so a BatterySwapRequest fails the upstream's own validation and cannot be sent. Not in known-bugs.tsv: BatterySwap is not declared, so no test runs it. Still so on master (2026-10-04) | [source](https://github.com/mobilityhouse/ocpp/blob/91f4f89140a2198dc5a300cfea88d947ac91ae28/ocpp/charge_point.py#L57-L64) |
| everest (station, 2.1) | libocpp of everest-core 2026.02.1 | `ocpp::v2::MessageType` has no ReportDERControl, GetCertificateChainStatus or NotifyWebPaymentStarted, though libocpp has their message structs: `ocpp::MessageQueue` converts every CALL's action to that enum (`ControlMessage`, `receive`), so libocpp can neither send these CALLs nor read one it receives | [source](https://github.com/EVerest/everest-core/blob/b61bb12b8cb89ee9d0fab037b05c3bb8cea9b13e/lib/everest/ocpp/include/ocpp/v2/types.hpp#L15-L195) |
| lorenzodonini (station, 2.0.1) | ocpp-go v0.19.0 | `NotifyReportRequest.Tbc` is a `bool` with `omitempty`: an explicit `"tbc": false` is dropped from the request (false is the default) | [source](https://github.com/lorenzodonini/ocpp-go/blob/a1eec917af884db0585752f909a61893aa667480/ocpp2.0.1/provisioning/notify_report.go#L98-L104) |

See [tests/known-bugs.tsv](../tests/known-bugs.tsv).
