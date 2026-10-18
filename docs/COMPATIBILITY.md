# Compatibility

Results of the reference-to-reference self-tests
([scripts/interop.sh](../scripts/interop.sh)): every station (`client`)
against every CSMS (`server`), for OCPP 1.6J and 2.0.1, with CALLs sent by
the station and by the CSMS. The matrix below is the output of `make matrix` (from
`reports/report-interop.json`, written by `make documents`); CI writes the
same matrix for each architecture into its job summary.

Run: 2026-10-18, images built from this repository on linux/arm64
(macOS, Docker 29.8), `make ci`. The linux/amd64 images have not been
self-tested locally; CI does that natively.

Each cell: results of the CALLs and checks initiated by the station / by
the CSMS. A 1.6 pairing has 22 of each: one per fixture sample, two derived
answers (an unknown token, unknown configuration keys), connectivity,
security profile 1 and the fixture's CALLERROR answers. A 2.0.1 pairing has
25 initiated by the station and 18 by the CSMS: one per sample, the derived
answers (an unknown token; unknown component and variable with
case-insensitive names; a read-only variable), connectivity, security
profile 1 and the CALLERROR answers.

## OCPP 1.6J

| Station ↓ / CSMS → | chargetime v2.0 | lorenzodonini v0.19.0 | mobilityhouse 2.1.0 |
|---|---|---|---|
| chargetime v2.0 | 21 pass 1 skip / 21 pass 1 skip | 22 pass / 21 pass 1 skip | 22 pass / 21 pass 1 skip |
| lorenzodonini v0.19.0 | 21 pass 1 skip / 22 pass | 22 pass / 22 pass | 22 pass / 22 pass |
| mobilityhouse 2.1.0 | 20 pass 1 skip 1 known / 22 pass | 21 pass 1 known / 22 pass | 21 pass 1 known / 22 pass |

Totals: 387 PASS, 3 KNOWN_ISSUE, 6 SKIP_UNSUPPORTED, 0 FAIL, 0 INFRA_ERROR.

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

Totals: 343 PASS, 38 KNOWN_ISSUE, 6 SKIP_UNSUPPORTED, 0 FAIL, 0 INFRA_ERROR.

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

Every smoke test ([scripts/smoke.sh](../scripts/smoke.sh)) passes: 127
checks per image, for both versions.

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
| lorenzodonini (station, 2.0.1) | ocpp-go v0.19.0 | `NotifyReportRequest.Tbc` is a `bool` with `omitempty`: an explicit `"tbc": false` is dropped from the request (false is the default) | [source](https://github.com/lorenzodonini/ocpp-go/blob/a1eec917af884db0585752f909a61893aa667480/ocpp2.0.1/provisioning/notify_report.go#L98-L104) |

See [tests/known-bugs.tsv](../tests/known-bugs.tsv).
