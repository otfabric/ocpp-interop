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

Each cell: results of the CALLs and checks initiated by the station / by
the CSMS. A 1.6 pairing has 25 initiated by the station and 22 by the
CSMS: one per fixture sample, two derived answers (an unknown token,
unknown configuration keys), connectivity, security profile 1, the
fixture's CALLERROR answers, and the transport scenarios `tls.profile2`,
`tls.profile3` and `compression` (counted with the station's). A 2.0.1
pairing has 28 initiated by the station and 18 by the CSMS: one per sample,
the derived answers (an unknown token; unknown component and variable with
case-insensitive names; a read-only variable), connectivity, security
profile 1, the CALLERROR answers and the three transport scenarios. A 2.1
pairing has 36 initiated by the station and 28 by the CSMS: the 2.0.1
tests, one per 2.1 sample, the SEND sample and CALLRESULTERROR (from each
end).

The transport scenarios keep a station connected over security profile 2
(TLS with RSA keys and Basic authentication), security profile 3 (mutual
TLS with ECDSA P-256 keys) or permessage-deflate, and send two CALLs from
each end; the CA and the certificates are generated for the run. They run
where both ends declare the profile or `compression`.

## OCPP 1.6J

| Station ↓ / CSMS → | chargetime v2.0 | lorenzodonini v0.19.0 | mobilityhouse 2.1.0 |
|---|---|---|---|
| chargetime v2.0 | 23 pass 2 skip / 21 pass 1 skip | 24 pass 1 skip / 21 pass 1 skip | 24 pass 1 skip / 21 pass 1 skip |
| lorenzodonini v0.19.0 | 23 pass 2 skip / 22 pass | 24 pass 1 skip / 22 pass | 24 pass 1 skip / 22 pass |
| mobilityhouse 2.1.0 | 22 pass 2 skip 1 known / 22 pass | 23 pass 1 skip 1 known / 22 pass | 24 pass 1 known / 22 pass |

Totals: 406 PASS, 3 KNOWN_ISSUE, 14 SKIP_UNSUPPORTED, 0 FAIL, 0 INFRA_ERROR.
Security profiles 2 and 3 pass in all nine pairings.

| Station | CSMS | Test | Status | Evidence |
|---|---|---|---|---|
| mobilityhouse | all three | `v16.from-station.security-event-minimal` | KNOWN_ISSUE | [mobilityhouse/ocpp `v16/call.py`](https://github.com/mobilityhouse/ocpp/blob/91f4f89140a2198dc5a300cfea88d947ac91ae28/ocpp/v16/call.py#L232-L236) |
| chargetime | all three | `v16.fixture-callerror.from-csms` | SKIP_UNSUPPORTED | chargetime does not declare `fixtureCallErrors` |
| all three | chargetime | `v16.fixture-callerror.from-station` | SKIP_UNSUPPORTED | chargetime does not declare `fixtureCallErrors` |
| all but mobilityhouse with itself | | `v16.compression` | SKIP_UNSUPPORTED (8) | only mobilityhouse declares `compression` in 1.6: chargetime's 1.6 classes have no extensions, lorenzodonini's CSMS cannot accept it |

## OCPP 2.0.1

| Station ↓ / CSMS → | chargetime v2.0 | lorenzodonini v0.19.0 | mobilityhouse 2.1.0 |
|---|---|---|---|
| chargetime v2.0 | 26 pass 1 skip 1 known / 17 pass 1 skip | 27 pass 1 skip / 17 pass 1 skip | 27 pass 1 known / 17 pass 1 skip |
| lorenzodonini v0.19.0 | 25 pass 2 skip 1 known / 18 pass | 26 pass 1 skip 1 known / 18 pass | 26 pass 1 skip 1 known / 17 pass 1 known |
| mobilityhouse 2.1.0 | 26 pass 1 skip 1 known / 18 pass | 27 pass 1 skip / 18 pass | 28 pass / 18 pass |

Totals: 396 PASS, 7 KNOWN_ISSUE, 11 SKIP_UNSUPPORTED, 0 FAIL, 0 INFRA_ERROR.
Security profiles 2 and 3 pass in all nine pairings.

| Station | CSMS | Test | Status | Evidence |
|---|---|---|---|---|
| mobilityhouse, chargetime | chargetime | `v201.compression` | KNOWN_ISSUE (2) | the CSMS keeps its compression context although it announces `server_no_context_takeover`: [Java-WebSocket 1.6.0 `PerMessageDeflateExtension`](https://github.com/TooTallNate/Java-WebSocket/blob/da3cf2a777aed862f2f5b5cf060cae7969958667/src/main/java/org/java_websocket/extensions/permessage_deflate/PerMessageDeflateExtension.java#L335-L340) |
| chargetime | mobilityhouse | `v201.compression` | KNOWN_ISSUE | the station keeps its compression context although it offers `client_no_context_takeover`: [Java-WebSocket 1.6.0 `PerMessageDeflateExtension`](https://github.com/TooTallNate/Java-WebSocket/blob/da3cf2a777aed862f2f5b5cf060cae7969958667/src/main/java/org/java_websocket/extensions/permessage_deflate/PerMessageDeflateExtension.java#L326-L333) |
| lorenzodonini | all three | `v201.from-station.notify-report` | KNOWN_ISSUE | [ocpp-go `notify_report.go`](https://github.com/lorenzodonini/ocpp-go/blob/a1eec917af884db0585752f909a61893aa667480/ocpp2.0.1/provisioning/notify_report.go#L98-L104) |
| lorenzodonini | mobilityhouse | `v201.from-csms.reset-immediate` | KNOWN_ISSUE | [ocpp-go `reset.go`](https://github.com/lorenzodonini/ocpp-go/blob/a1eec917af884db0585752f909a61893aa667480/ocpp2.0.1/provisioning/reset.go#L61-L64) |
| chargetime | all three | `v201.fixture-callerror.from-csms` | SKIP_UNSUPPORTED | chargetime does not declare `fixtureCallErrors` |
| all three | chargetime | `v201.fixture-callerror.from-station` | SKIP_UNSUPPORTED | chargetime does not declare `fixtureCallErrors` |
| lorenzodonini at either end | | `v201.compression` | SKIP_UNSUPPORTED (5) | lorenzodonini does not declare `compression`: ocpp-go's CSMS cannot accept it |

With compression, the first CALL of a connection to or from the chargetime
2.0.1 adapter works (BootNotification passes); the second message in the
direction of the defect cannot be decompressed and the connection closes.
Without `--compression` no station offers it, so every other 2.0.1 test is
unaffected (before contract 0.5 the mobilityhouse station always offered
it, and 34 tests against the chargetime CSMS were KNOWN_ISSUE).

## OCPP 2.1

Only `mobilityhouse` declares 2.1: the pinned releases of Java-OCA-OCPP
(v2.0) and ocpp-go (v0.19.0) have no 2.1, so every pairing with them is one
`v21.pairing` SKIP_UNSUPPORTED.

| Station ↓ / CSMS → | chargetime v2.0 | lorenzodonini v0.19.0 | mobilityhouse 2.1.0 |
|---|---|---|---|
| chargetime v2.0 | 1 skip / — | 1 skip / — | 1 skip / — |
| lorenzodonini v0.19.0 | 1 skip / — | 1 skip / — | 1 skip / — |
| mobilityhouse 2.1.0 | 1 skip / — | 1 skip / — | 32 pass 3 skip 1 known / 26 pass 1 skip 1 known |

Totals: 58 PASS, 2 KNOWN_ISSUE, 12 SKIP_UNSUPPORTED, 0 FAIL, 0 INFRA_ERROR.
`v21.tls.profile2`, `v21.tls.profile3` and `v21.compression` pass.

| Station | CSMS | Test | Status | Evidence |
|---|---|---|---|---|
| mobilityhouse | mobilityhouse | `v21.from-station.reservation-status-update`, `v21.from-csms.cancel-reservation` | KNOWN_ISSUE (2) | [mobilityhouse/ocpp `charge_point.py`](https://github.com/mobilityhouse/ocpp/blob/91f4f89140a2198dc5a300cfea88d947ac91ae28/ocpp/charge_point.py#L161-L193) |
| mobilityhouse | mobilityhouse | `v21.from-station.battery-swap-in` | SKIP_UNSUPPORTED | BatterySwap is not declared: ocpp 2.1.0 cannot send it (see below) |
| mobilityhouse | mobilityhouse | `v21.send.notify-periodic-event-stream` | SKIP_UNSUPPORTED | no adapter declares `send` |
| mobilityhouse | mobilityhouse | `v21.rpc.call-result-error` (from each end) | SKIP_UNSUPPORTED (2) | no adapter declares `callResultError` |
| chargetime, lorenzodonini | all three | `v21.pairing` | SKIP_UNSUPPORTED (8) | 2.1 not declared by one of the two |

Every smoke test ([scripts/smoke.sh](../scripts/smoke.sh)) passes: 187
checks for lorenzodonini and 188 for chargetime (1.6 and 2.0.1; chargetime
also has its 2.0.1 compression session reported as a known upstream bug),
268 for mobilityhouse (1.6, 2.0.1 and 2.1). They include, per version,
security profiles 2 and 3 with RSA and ECDSA keys and what must not connect
(wrong password, a client certificate of another identity, from another CA
or none, a server certificate from another CA or for another host name,
`ws://` to a TLS server), and compression.

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
| chargetime (station, 2.0.1), with `--compression` | Java-OCA-OCPP v2.0 with Java-WebSocket 1.6.0 | `MultiProtocolJSONClient` offers permessage-deflate with `client_no_context_takeover` (Java-WebSocket always adds it), but keeps its compression context: `encodeFrame` resets the deflater only when `serverNoContextTakeover` is set, which the client sets false. A CSMS that takes the offer cannot decompress the station's second message | [source](https://github.com/TooTallNate/Java-WebSocket/blob/da3cf2a777aed862f2f5b5cf060cae7969958667/src/main/java/org/java_websocket/extensions/permessage_deflate/PerMessageDeflateExtension.java#L326-L333), [MultiProtocolJSONClient](https://github.com/ChargeTimeEU/Java-OCA-OCPP/blob/f27fe5bfb03e01491100597d6859080f5ae07545/ocpp-v2/src/main/java/eu/chargetime/ocpp/MultiProtocolJSONClient.java#L94-L100) |
| lorenzodonini (station, 2.0.1), with a CSMS that validates responses | ocpp-go v0.19.0 | `ResetResponse` (and `SetNetworkProfileResponse`) declare `statusInfo` without `omitempty`: the station sends `"statusInfo": null`, which the OCA schema does not allow | [source](https://github.com/lorenzodonini/ocpp-go/blob/a1eec917af884db0585752f909a61893aa667480/ocpp2.0.1/provisioning/reset.go#L61-L64) |
| mobilityhouse (both roles, 2.1) | ocpp 2.1.0 | `charge_point._raise_key_error` raises NotImplemented or NotSupported for 1.6, 2.0 and 2.0.1 only: a 2.1 CALL whose action has no handler gets no answer, and the sender times out. Fixed on master (ae716a1, #744), unreleased | [source](https://github.com/mobilityhouse/ocpp/blob/91f4f89140a2198dc5a300cfea88d947ac91ae28/ocpp/charge_point.py#L161-L193) |
| mobilityhouse (station, 2.1) | ocpp 2.1.0 | `snake_to_camel_case` writes the `soc` and `soh` fields of `BatteryDataType` as `SoC` and `soh` (the schema has `soC`, `soH`), so a BatterySwapRequest fails the upstream's own validation and cannot be sent. Not in known-bugs.tsv: BatterySwap is not declared, so no test runs it. Still so on master (2026-10-04) | [source](https://github.com/mobilityhouse/ocpp/blob/91f4f89140a2198dc5a300cfea88d947ac91ae28/ocpp/charge_point.py#L57-L64) |
| lorenzodonini (station, 2.0.1) | ocpp-go v0.19.0 | `NotifyReportRequest.Tbc` is a `bool` with `omitempty`: an explicit `"tbc": false` is dropped from the request (false is the default) | [source](https://github.com/lorenzodonini/ocpp-go/blob/a1eec917af884db0585752f909a61893aa667480/ocpp2.0.1/provisioning/notify_report.go#L98-L104) |

See [tests/known-bugs.tsv](../tests/known-bugs.tsv).
