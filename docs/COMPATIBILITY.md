# Compatibility

Results of the reference-to-reference self-tests
([scripts/interop.sh](../scripts/interop.sh)): every station (`client`)
against every CSMS (`server`), for OCPP 1.6J, 2.0.1 and 2.1, with CALLs sent by
the station and by the CSMS. The matrix below is the output of `make matrix` (from
`reports/report-interop.json`, written by `make documents`); CI writes the
same matrix for each architecture into its job summary.

Run: 2026-11-07 on linux/arm64 (macOS, Docker 29.9), the seven adapters:
the six images of v0.7.0 and the ocppkit image built from this repository,
in seven shards (`INTEROP_PAIRS=<CSMS>@<version>`, one per CSMS) merged
with `scripts/merge-reports.py`. The ocppkit station's profile 3 session
with the chargetime CSMS hung now and then (about one handshake in seven) on
a race in Java-WebSocket 1.6.0's TLS channel, which the chargetime adapter
now avoids (adapters/chargetime/README.md, Transport security); these
tables have the chargetime shard run with that fix. The
linux/amd64 images have not been self-tested locally; CI does that
natively.

`everest` (EVerest libocpp, everest-core 2026.02.2) is a station only, and
`citrineos` (CitrineOS core v2.1.0) a CSMS only. Every pairing with `everest`
as the CSMS or `citrineos` as the station is one `pairing` SKIP_UNSUPPORTED
(not in the tables). The tables are from the run of the seven adapters on
contract 0.7 (2026-11-07, linux/arm64): 6425 PASS, 47 KNOWN_ISSUE, 559
SKIP_UNSUPPORTED, 0 FAIL, 0 INFRA_ERROR (v0.7.0, six adapters: 4190 PASS,
37 KNOWN_ISSUE, 423 SKIP_UNSUPPORTED; v0.6.3, five
adapters: 2455 PASS, 28 KNOWN_ISSUE, 300 SKIP_UNSUPPORTED; v0.4.0: 2196 PASS,
29 KNOWN_ISSUE, 558 SKIP_UNSUPPORTED; the everest station now declares
every action libocpp can exchange, see RELEASE.md v0.4.1). The everest skips
are compression, which libocpp cannot offer, and in 2.1 the SEND, the
CALLRESULTERROR tests and the two actions libocpp's message queue does not
know (adapters/everest/README.md). Most of
the citrineos skips are actions it does not declare: CitrineOS answers by
itself, and its lists are the actions whose answer is the fixture's, or
that its REST message API sends ([adapters/citrineos/README.md](../adapters/citrineos/README.md));
in the pairings with it, 756 PASS, 6 KNOWN_ISSUE, 315 SKIP_UNSUPPORTED.
`cbsigma` (cB-Sigma/ocpp v0.2.0) has both roles in the three versions; in
the pairings with it, 1735 PASS, 9 KNOWN_ISSUE (the upstream defects of its
peers listed below), 123 SKIP_UNSUPPORTED (compression, which its station
cannot offer, what its peers do not declare, and SEND and CALLRESULTERROR
with peers that have neither). It was the first reference with the OCPP 2.1
SEND and CALLRESULTERROR, in v0.7.0. `ocppkit` (hupe1980/ocpp-kit 0.3.0,
Rust) has both roles in the three versions, SEND, CALLRESULTERROR and
compression; in the pairings with it, 2235 PASS, 10 KNOWN_ISSUE (the
upstream defects of its peers, and one of ocpp-kit's own, below), 136
SKIP_UNSUPPORTED. Its station always authenticates, so its pairings run
with security profile 1 at both ends (contract 0.7). With it,
`v21.send.notify-periodic-event-stream` and `v21.rpc.call-result-error`
(both directions) pass between two independent implementations, cbsigma
and ocppkit, in both roles; the own image runs them against both
([own/go-ocpp/README.md](../own/go-ocpp/README.md)).

Each cell: results of the CALLs and checks initiated by the station / by
the CSMS. The baseline fixtures have a sample of every action of their
version, so a pairing sends every action both ends declare. A 1.6 pairing
has 26 tests initiated by the station and 29 by the CSMS: one per fixture
sample, two derived answers (an unknown token, unknown configuration keys),
connectivity, security profile 1, the fixture's CALLERROR answers and
NotImplemented (from a fixture copy without one answer per side). A 2.0.1
pairing has 38 and 44: one per sample, the derived answers (an unknown
token; unknown component and variable with case-insensitive names; a
read-only variable), connectivity, security profile 1, the CALLERROR
answers and NotImplemented. A 2.1 pairing has 51 and 60: the 2.0.1 tests,
one per 2.1 sample, the SEND sample and CALLRESULTERROR (from each end).

Actions exchanged with a PASS in at least one pairing: 39 of 39 in 1.6, 64
of 64 in 2.0.1 and 91 of 91 in 2.1 (NotifyPeriodicEventStream, a SEND,
since v0.7.0; [COVERAGE.md](../COVERAGE.md)). BatterySwap passes since v0.4.1, from
the everest station to the mobilityhouse CSMS. Before every
action had a sample (v0.2.0): 30, 27 and 43.

Each pairing also runs the transport scenarios `tls.profile2`,
`tls.profile3` and `compression` (counted with the station's).
The transport scenarios keep a station connected over security profile 2
(TLS with RSA keys and Basic authentication), security profile 3 (mutual
TLS with ECDSA P-256 keys) or permessage-deflate, and send two CALLs from
each end; the CA and the certificates are generated for the run. They run
where both ends declare the profile or `compression`.

## OCPP 1.6J

| Station ↓ / CSMS → | cbsigma v0.2.0 | chargetime v2.0 | citrineos v2.1.0 | lorenzodonini v0.19.0 | mobilityhouse 2.1.0 | ocppkit 0.3.0 |
|---|---|---|---|---|---|---|
| cbsigma v0.2.0 | 28 pass 1 skip / 29 pass | 27 pass 2 skip / 29 pass | 21 pass 8 skip / 20 pass 9 skip | 28 pass 1 skip / 29 pass | 28 pass 1 skip / 29 pass | 28 pass 1 skip / 29 pass |
| chargetime v2.0 | 28 pass 1 skip / 28 pass 1 skip | 27 pass 2 skip / 28 pass 1 skip | 21 pass 8 skip / 19 pass 10 skip | 28 pass 1 skip / 28 pass 1 skip | 28 pass 1 skip / 28 pass 1 skip | 28 pass 1 skip / 28 pass 1 skip |
| everest 2026.02.2 | 28 pass 1 skip / 29 pass | 27 pass 2 skip / 29 pass | 21 pass 8 skip / 20 pass 9 skip | 28 pass 1 skip / 29 pass | 28 pass 1 skip / 29 pass | 28 pass 1 skip / 29 pass |
| lorenzodonini v0.19.0 | 28 pass 1 skip / 29 pass | 27 pass 2 skip / 29 pass | 21 pass 8 skip / 20 pass 9 skip | 28 pass 1 skip / 29 pass | 28 pass 1 skip / 29 pass | 28 pass 1 skip / 29 pass |
| mobilityhouse 2.1.0 | 27 pass 1 skip 1 known / 29 pass | 26 pass 2 skip 1 known / 29 pass | 21 pass 7 skip 1 known / 20 pass 9 skip | 27 pass 1 skip 1 known / 29 pass | 28 pass 1 known / 29 pass | 28 pass 1 known / 29 pass |
| ocppkit 0.3.0 | 27 pass 2 skip / 29 pass | 26 pass 3 skip / 29 pass | 21 pass 8 skip / 20 pass 9 skip | 27 pass 2 skip / 29 pass | 28 pass 1 skip / 29 pass | 28 pass 1 skip / 29 pass |

Totals: 1938 PASS, 6 KNOWN_ISSUE, 157 SKIP_UNSUPPORTED, 0 FAIL, 0 INFRA_ERROR.
Security profiles 2 and 3 pass in all 36 pairings (every station, everest
included, with every CSMS).

| Station | CSMS | Test | Status | Evidence |
|---|---|---|---|---|
| mobilityhouse | all five | `v16.from-station.security-event-minimal` | KNOWN_ISSUE | [mobilityhouse/ocpp `v16/call.py`](https://github.com/mobilityhouse/ocpp/blob/91f4f89140a2198dc5a300cfea88d947ac91ae28/ocpp/v16/call.py#L232-L236) |
| chargetime | all four | `v16.fixture-callerror.from-csms` | SKIP_UNSUPPORTED | chargetime does not declare `fixtureCallErrors` |
| all four | chargetime | `v16.fixture-callerror.from-station` | SKIP_UNSUPPORTED | chargetime does not declare `fixtureCallErrors` |
| all four | citrineos | `v16.not-implemented.from-station`, `v16.fixture-callerror.from-station` | SKIP_UNSUPPORTED (8) | citrineos declares `fixtureAnswers` and `fixtureCallErrors` false: CitrineOS answers by itself |
| all four | citrineos | the actions citrineos does not declare | SKIP_UNSUPPORTED | see [adapters/citrineos/README.md](../adapters/citrineos/README.md) |
| all but the mobilityhouse and ocppkit stations with the mobilityhouse and ocppkit CSMSs | | `v16.compression` | SKIP_UNSUPPORTED | only mobilityhouse and ocppkit declare `compression` in 1.6: chargetime's 1.6 classes have no extensions, lorenzodonini's CSMS cannot accept it, the everest and cbsigma stations cannot offer it |
| ocppkit | all | `v16.profile1.no-credentials` (and in 2.0.1 and 2.1) | SKIP_UNSUPPORTED | the ocppkit station always sends credentials (`client.securityProfiles` `[1, 2, 3]`); its pairings run with security profile 1 at both ends |

## OCPP 2.0.1

| Station ↓ / CSMS → | cbsigma v0.2.0 | chargetime v2.0 | citrineos v2.1.0 | lorenzodonini v0.19.0 | mobilityhouse 2.1.0 | ocppkit 0.3.0 |
|---|---|---|---|---|---|---|
| cbsigma v0.2.0 | 40 pass 1 skip / 44 pass | 39 pass 2 skip / 44 pass | 28 pass 13 skip / 41 pass 3 skip | 40 pass 1 skip / 43 pass 1 known | 40 pass 1 skip / 44 pass | 40 pass 1 skip / 44 pass |
| chargetime v2.0 | 40 pass 1 skip / 43 pass 1 skip | 39 pass 1 skip 1 known / 43 pass 1 skip | 28 pass 12 skip 1 known / 40 pass 4 skip | 40 pass 1 skip / 42 pass 1 skip 1 known | 40 pass 1 known / 43 pass 1 skip | 40 pass 1 known / 43 pass 1 skip |
| everest 2026.02.2 | 40 pass 1 skip / 44 pass | 39 pass 2 skip / 44 pass | 28 pass 13 skip / 41 pass 3 skip | 40 pass 1 skip / 43 pass 1 known | 39 pass 1 skip 1 known / 44 pass | 39 pass 1 skip 1 known / 44 pass |
| lorenzodonini v0.19.0 | 39 pass 1 skip 1 known / 41 pass 3 known | 38 pass 2 skip 1 known / 43 pass 1 known | 27 pass 13 skip 1 known / 39 pass 3 skip 2 known | 39 pass 1 skip 1 known / 43 pass 1 known | 39 pass 1 skip 1 known / 41 pass 3 known | 39 pass 1 skip 1 known / 43 pass 1 known |
| mobilityhouse 2.1.0 | 40 pass 1 skip / 44 pass | 39 pass 1 skip 1 known / 44 pass | 29 pass 12 skip / 41 pass 3 skip | 40 pass 1 skip / 43 pass 1 known | 41 pass / 44 pass | 41 pass / 44 pass |
| ocppkit 0.3.0 | 39 pass 2 skip / 44 pass | 38 pass 2 skip 1 known / 44 pass | 28 pass 13 skip / 41 pass 3 skip | 39 pass 2 skip / 43 pass 1 known | 40 pass 1 skip / 44 pass | 40 pass 1 skip / 44 pass |

Totals: 2898 PASS, 30 KNOWN_ISSUE, 145 SKIP_UNSUPPORTED, 0 FAIL, 0 INFRA_ERROR.
Security profiles 2 and 3 pass in all 36 pairings.

| Station | CSMS | Test | Status | Evidence |
|---|---|---|---|---|
| mobilityhouse, chargetime | chargetime | `v201.compression` | KNOWN_ISSUE (2) | the CSMS keeps its compression context although it announces `server_no_context_takeover`: [Java-WebSocket 1.6.0 `PerMessageDeflateExtension`](https://github.com/TooTallNate/Java-WebSocket/blob/da3cf2a777aed862f2f5b5cf060cae7969958667/src/main/java/org/java_websocket/extensions/permessage_deflate/PerMessageDeflateExtension.java#L335-L340) |
| chargetime | mobilityhouse | `v201.compression` | KNOWN_ISSUE | the station keeps its compression context although it offers `client_no_context_takeover`: [Java-WebSocket 1.6.0 `PerMessageDeflateExtension`](https://github.com/TooTallNate/Java-WebSocket/blob/da3cf2a777aed862f2f5b5cf060cae7969958667/src/main/java/org/java_websocket/extensions/permessage_deflate/PerMessageDeflateExtension.java#L326-L333) |
| lorenzodonini | all four | `v201.from-station.notify-report` | KNOWN_ISSUE | [ocpp-go `notify_report.go`](https://github.com/lorenzodonini/ocpp-go/blob/a1eec917af884db0585752f909a61893aa667480/ocpp2.0.1/provisioning/notify_report.go#L98-L104) |
| lorenzodonini | mobilityhouse, citrineos, cbsigma | `v201.from-csms.reset-immediate`, `v201.from-csms.set-network-profile` | KNOWN_ISSUE (6) | [ocpp-go `reset.go`](https://github.com/lorenzodonini/ocpp-go/blob/a1eec917af884db0585752f909a61893aa667480/ocpp2.0.1/provisioning/reset.go#L61-L64) |
| lorenzodonini, or any station with lorenzodonini | all four, or lorenzodonini | `v201.from-csms.get-composite-schedule` | KNOWN_ISSUE (6) | [ocpp-go `get_composite_schedule.go`](https://github.com/lorenzodonini/ocpp-go/blob/a1eec917af884db0585752f909a61893aa667480/ocpp2.0.1/smartcharging/get_composite_schedule.go#L31-L50) |
| everest | ocppkit | `v201.from-station.notify-evcharging-needs` | KNOWN_ISSUE | ocpp-kit reads the integer `energyAmount` as `i32` and refuses `20000.0` (`TypeConstraintViolation`), a valid integer in the draft-06 OCA schemas: [ocpp-kit `v2_0_1/types.rs`](https://github.com/hupe1980/ocpp-kit/blob/b64ce83ab46c742dfbd99e925cd0acfacbf2ddc9/src/v2_0_1/types.rs#L31), [hupe1980/ocpp-kit#2](https://github.com/hupe1980/ocpp-kit/issues/2) (tracked in #27) |
| everest | mobilityhouse | `v201.from-station.notify-evcharging-needs` | KNOWN_ISSUE | the CSMS validates the draft-06 OCA schemas with jsonschema's `Draft4Validator`, so the integer `energyAmount` written `20000.0` (libocpp's `float`, a valid integer in draft-06) gets `TypeConstraintViolation`: [mobilityhouse/ocpp `messages.py`](https://github.com/mobilityhouse/ocpp/blob/91f4f89140a2198dc5a300cfea88d947ac91ae28/ocpp/messages.py#L130-L173) |
| chargetime | all four | `v201.fixture-callerror.from-csms` | SKIP_UNSUPPORTED | chargetime does not declare `fixtureCallErrors` |
| all four | chargetime | `v201.fixture-callerror.from-station` | SKIP_UNSUPPORTED | chargetime does not declare `fixtureCallErrors` |
| chargetime | citrineos | `v201.compression` | KNOWN_ISSUE | the chargetime station defect above |
| all four | citrineos | `v201.not-implemented.from-station`, `v201.fixture-callerror.from-station`, undeclared actions | SKIP_UNSUPPORTED | as in 1.6 |
| lorenzodonini at either end | | `v201.compression` | SKIP_UNSUPPORTED (5) | lorenzodonini does not declare `compression`: ocpp-go's CSMS cannot accept it |

With compression, the first CALL of a connection to or from the chargetime
2.0.1 adapter works (BootNotification passes); the second message in the
direction of the defect cannot be decompressed and the connection closes.
Without `--compression` no station offers it, so every other 2.0.1 test is
unaffected (before contract 0.5 the mobilityhouse station always offered
it, and 34 tests against the chargetime CSMS were KNOWN_ISSUE).

## OCPP 2.1

`mobilityhouse`, `cbsigma` and `ocppkit` declare 2.1 in both roles,
`everest` as a station and `citrineos` as a CSMS: the pinned releases of
Java-OCA-OCPP (v2.0) and ocpp-go (v0.19.0) have no 2.1, so every pairing
with them is one `v21.pairing` SKIP_UNSUPPORTED. Every 2.1 pairing of two
different adapters joins two independent 2.1 implementations: 4 stations
with 4 CSMSs.

| Station ↓ / CSMS → | cbsigma v0.2.0 | chargetime v2.0 | citrineos v2.1.0 | lorenzodonini v0.19.0 | mobilityhouse 2.1.0 | ocppkit 0.3.0 |
|---|---|---|---|---|---|---|
| cbsigma v0.2.0 | 53 pass 1 skip / 60 pass | 1 skip / — | 28 pass 26 skip / 42 pass 18 skip | 1 skip / — | 50 pass 3 skip 1 known / 59 pass 1 skip | 53 pass 1 skip / 59 pass 1 skip |
| chargetime v2.0 | 1 skip / — | 1 skip / — | 1 skip / — | 1 skip / — | 1 skip / — | 1 skip / — |
| everest 2026.02.2 | 49 pass 4 skip 1 known / 58 pass 2 skip | 1 skip / — | 28 pass 26 skip / 42 pass 18 skip | 1 skip / — | 48 pass 4 skip 2 known / 58 pass 2 skip | 49 pass 4 skip 1 known / 58 pass 2 skip |
| lorenzodonini v0.19.0 | 1 skip / — | 1 skip / — | 1 skip / — | 1 skip / — | 1 skip / — | 1 skip / — |
| mobilityhouse 2.1.0 | 50 pass 4 skip / 58 pass 1 skip 1 known | 1 skip / — | 29 pass 25 skip / 41 pass 18 skip 1 known | 1 skip / — | 50 pass 3 skip 1 known / 58 pass 1 skip 1 known | 51 pass 3 skip / 57 pass 2 skip 1 known |
| ocppkit 0.3.0 | 52 pass 2 skip / 59 pass 1 skip | 1 skip / — | 28 pass 26 skip / 42 pass 18 skip | 1 skip / — | 50 pass 3 skip 1 known / 58 pass 2 skip | 53 pass 1 skip / 59 pass 1 skip |

Totals: 1589 PASS, 11 KNOWN_ISSUE, 257 SKIP_UNSUPPORTED, 0 FAIL, 0 INFRA_ERROR.
`v21.tls.profile2` and `v21.tls.profile3` pass for the four stations with
the four CSMSs (mobilityhouse, citrineos, cbsigma, ocppkit);
`v21.compression` passes for the mobilityhouse and ocppkit stations with the
mobilityhouse, citrineos and ocppkit CSMSs (everest and cbsigma cannot offer
compression). `v21.send.notify-periodic-event-stream` and
`v21.rpc.call-result-error` (from each end) pass in the four pairings of
cbsigma and ocppkit: between two independent implementations in both
directions, and each with itself.

| Station | CSMS | Test | Status | Evidence |
|---|---|---|---|---|
| mobilityhouse | mobilityhouse | `v21.not-implemented.from-station`, `v21.not-implemented.from-csms` | KNOWN_ISSUE (2) | [mobilityhouse/ocpp `charge_point.py`](https://github.com/mobilityhouse/ocpp/blob/91f4f89140a2198dc5a300cfea88d947ac91ae28/ocpp/charge_point.py#L161-L193) |
| mobilityhouse | mobilityhouse, citrineos | `v21.from-station.battery-swap-in` | SKIP_UNSUPPORTED | the mobilityhouse station does not declare BatterySwap: ocpp 2.1.0 cannot send it (see below). The everest station sends it to the mobilityhouse CSMS: PASS |
| all but cbsigma and ocppkit with each other | | `v21.send.notify-periodic-event-stream`, `v21.rpc.call-result-error` | SKIP_UNSUPPORTED | `send` and `callResultError`: only cbsigma and ocppkit declare them (mobilityhouse, everest and citrineos know the message types 2 to 4 only) |
| ocppkit | all | `v21.from-csms.notify-web-payment-started` | SKIP_UNSUPPORTED | the ocppkit station does not declare NotifyWebPaymentStarted, nor its CSMS: ocpp-kit 0.3.0 has it as a station message (below) |
| everest | ocppkit | `v21.from-station.report-dercontrol` | KNOWN_ISSUE | the libocpp defect below |
| chargetime, lorenzodonini | all five | `v21.pairing` | SKIP_UNSUPPORTED (10) | 2.1 not declared by one of the two |
| everest | mobilityhouse | `v21.from-station.report-dercontrol` | KNOWN_ISSUE | [libocpp `ocpp::v2::MessageType`](https://github.com/EVerest/everest-core/blob/45dba2c049223d05beb7611c75d59f6639c46ee0/lib/everest/ocpp/include/ocpp/v2/types.hpp#L15-L195) has no ReportDERControl |
| everest | mobilityhouse | `v21.not-implemented.from-station` | KNOWN_ISSUE | the mobilityhouse defect above (no answer to an unanswered 2.1 action) |
| mobilityhouse | citrineos | `v21.not-implemented.from-csms` | KNOWN_ISSUE | the mobilityhouse defect above, in its station |
| mobilityhouse, everest | citrineos | `v21.not-implemented.from-station`, `v21.fixture-callerror.from-station`, undeclared actions | SKIP_UNSUPPORTED | as in 1.6; of the actions 2.1 adds, citrineos declares only SetDefaultTariff |
| everest | mobilityhouse | `v21.send.notify-periodic-event-stream`, `v21.rpc.call-result-error` (from each end), `v21.from-station.get-certificate-chain-status`, `v21.from-csms.notify-web-payment-started`, `v21.compression` | SKIP_UNSUPPORTED (6) | libocpp cannot offer compression, and has no SEND and no CALLRESULTERROR; everest does not declare GetCertificateChainStatus and NotifyWebPaymentStarted, which libocpp's message queue does not know (below) |

Every smoke test ([scripts/smoke.sh](../scripts/smoke.sh)) passes: 187
checks for lorenzodonini and 188 for chargetime (1.6 and 2.0.1; chargetime
also has its 2.0.1 compression session reported as a known upstream bug),
268 for mobilityhouse (1.6, 2.0.1 and 2.1). They include, per version,
security profiles 2 and 3 with RSA and ECDSA keys and what must not connect
(wrong password, a client certificate of another identity, from another CA
or none, a server certificate from another CA or for another host name,
`ws://` to a TLS server), and compression. everest: 147 (1.6, 2.0.1 and 2.1, station checks against
the mobilityhouse CSMS). citrineos: 242 (1.6, 2.0.1 and 2.1, server checks
with the mobilityhouse station, security profiles 2 and 3 and compression
included).

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
| lorenzodonini (station, 2.0.1), with a CSMS that validates responses | ocpp-go v0.19.0 | `ResetResponse` and `SetNetworkProfileResponse` declare `statusInfo` without `omitempty`: the station sends `"statusInfo": null`, which the OCA schema does not allow. The fixture answers `{"status": "Accepted"}`; ocpp-go's `encoding/json` writes the null (the only two `*StatusInfo` fields of its 2.0.1 package without `omitempty`). Unchanged on master (b22429c, 2025-08-17), no later release | [source](https://github.com/lorenzodonini/ocpp-go/blob/a1eec917af884db0585752f909a61893aa667480/ocpp2.0.1/provisioning/reset.go#L61-L64) |
| mobilityhouse (both roles, 2.1) | ocpp 2.1.0 | `charge_point._raise_key_error` raises NotImplemented or NotSupported for 1.6, 2.0 and 2.0.1 only: a 2.1 CALL whose action has no handler gets no answer, and the sender times out. Fixed on master (ae716a1, #744), unreleased | [source](https://github.com/mobilityhouse/ocpp/blob/91f4f89140a2198dc5a300cfea88d947ac91ae28/ocpp/charge_point.py#L161-L193) |
| mobilityhouse (station, 2.1) | ocpp 2.1.0 | `snake_to_camel_case` writes the `soc` and `soh` fields of `BatteryDataType` as `SoC` and `soh` (the schema has `soC`, `soH`), so a BatterySwapRequest fails the upstream's own validation and cannot be sent. The baseline sample has `soC` and `soH`, as the OCA 2.1 schema the upstream bundles (identical to the OCA's): the upstream reads them as `soc` and `soh` and writes them back as `SoC` and `soh` ("Additional properties are not allowed ('SoC', 'soh' were unexpected)"). Not in known-bugs.tsv: the station does not declare BatterySwap, so no test runs it; the CSMS receives it fine (everest station: PASS). Still so on master (2026-10-04) | [source](https://github.com/mobilityhouse/ocpp/blob/91f4f89140a2198dc5a300cfea88d947ac91ae28/ocpp/charge_point.py#L57-L64) |
| lorenzodonini (both roles, 2.0.1) | ocpp-go v0.19.0 | `GetCompositeScheduleResponse` does not have the OCA structure: a top-level `evseId`, always written, which the schema does not allow; no `statusInfo`; a `schedule` of `{startDateTime, chargingSchedule}` instead of `CompositeScheduleType`. The station cannot answer with a schedule, and the CSMS drops the schedule of a correct answer. Master removed the `evseId` and added `statusInfo` ([9356607](https://github.com/lorenzodonini/ocpp-go/commit/9356607e1ec2a128184064317c2ea719b550d003), 2025-03-28) but kept the `schedule` type; not released | [source](https://github.com/lorenzodonini/ocpp-go/blob/a1eec917af884db0585752f909a61893aa667480/ocpp2.0.1/smartcharging/get_composite_schedule.go#L31-L50) |
| mobilityhouse (both roles, 2.1) | ocpp 2.1.0 | `snake_to_camel_case` also misspells other optional 2.1 fields: `receiptUrl` (NotifySettlement, written `receiptURL`), the per-phase `limit_L2`-style fields (`limitL2`), `fixedPF...`, `prioritizedEMAIDs`, `taxRuleID`, `soC` of `LimitAtSoCType`; a payload with one fails the upstream's own validation. Not in known-bugs.tsv: the baseline's payloads use none of them (NotifySettlement is answered with `receiptId` only) | [source](https://github.com/mobilityhouse/ocpp/blob/91f4f89140a2198dc5a300cfea88d947ac91ae28/ocpp/charge_point.py#L46-L80) |
| everest (station, 2.1) | libocpp of everest-core 2026.02.2 | `ocpp::v2::MessageType` has no ReportDERControl, GetCertificateChainStatus or NotifyWebPaymentStarted, though libocpp has their message structs: `ocpp::MessageQueue` converts every CALL's action to that enum (`ControlMessage`, `receive`), so libocpp can neither send these CALLs nor read one it receives; libocpp has no other path for a CALL. everest-core's main added ReportDERControl ([04ebcd2a](https://github.com/EVerest/everest-core/commit/04ebcd2aaaaa4bfa59c5a33ec1f743002b31117c), #2100, 2026-06-03), in no release; the other two are still missing on main (0da38568, 2026-10-16). Only ReportDERControl is declared | [source](https://github.com/EVerest/everest-core/blob/45dba2c049223d05beb7611c75d59f6639c46ee0/lib/everest/ocpp/include/ocpp/v2/types.hpp#L15-L195) |
| lorenzodonini (station, 2.0.1) | ocpp-go v0.19.0 | `NotifyReportRequest.Tbc` is a `bool` with `omitempty`: an explicit `"tbc": false` is dropped from the request (false is the default) | [source](https://github.com/lorenzodonini/ocpp-go/blob/a1eec917af884db0585752f909a61893aa667480/ocpp2.0.1/provisioning/notify_report.go#L98-L104) |
| mobilityhouse (CSMS, 2.0.1), with the everest station | ocpp 2.1.0 | `get_validator` validates every payload with jsonschema's `Draft4Validator`, although the bundled OCA 2.0.1 schemas declare draft-06, in which `20000.0` is an integer: a NotifyEVChargingNeeds with the integer `energyAmount` written `20000.0` is answered `TypeConstraintViolation`. libocpp writes it so: its `ocpp::v2` types come from the OCPP 2.1 schemas, where `energyAmount` is a number (`ACChargingParameters::energyAmount` is a `float`); a valid value, so the defect is the validator's. Still `Draft4Validator` on master (2026-10-04) | [source](https://github.com/mobilityhouse/ocpp/blob/91f4f89140a2198dc5a300cfea88d947ac91ae28/ocpp/messages.py#L130-L173) |
| ocppkit (CSMS, 2.0.1), with the everest station | ocpp-kit 0.3.0 | The integer fields of the generated OCA types are `i32` (serde): `20000.0`, a valid integer in the draft-06 OCA schemas, is refused with `TypeConstraintViolation`. libocpp writes `energyAmount` of NotifyEVChargingNeeds so (see the mobilityhouse entry above). Unchanged on main (b64ce83, the release commit) | [source](https://github.com/hupe1980/ocpp-kit/blob/b64ce83ab46c742dfbd99e925cd0acfacbf2ddc9/src/v2_0_1/types.rs#L31), [hupe1980/ocpp-kit#2](https://github.com/hupe1980/ocpp-kit/issues/2) (tracked in #27) |
| ocppkit (both roles, 2.1) | ocpp-kit 0.3.0 | NotifyWebPaymentStarted has `Origin::ChargingStation`; OCPP 2.1 Part 2 (use case C, payment) has the CSMS send it, as every other 2.1 reference here does. ocpp-kit's CSMS refuses to send it ("not valid in this direction") and its station answers it `NotSupported` before the handler. Not in known-bugs.tsv: ocppkit does not declare it | [source](https://github.com/hupe1980/ocpp-kit/blob/b64ce83ab46c742dfbd99e925cd0acfacbf2ddc9/src/v2_1/action.rs#L578), [hupe1980/ocpp-kit#1](https://github.com/hupe1980/ocpp-kit/issues/1) (tracked in #26) |

| citrineos (CSMS, 2.1) | CitrineOS v2.1.0 | `OCPP2_1_CALL_SCHEMA_RECORD` has no schema for NotifyDERAlarm, NotifyDERStartStop, ReportDERControl, OpenPeriodicEventStream, ClosePeriodicEventStream, NotifyPriorityCharging, PullDynamicScheduleUpdate and GetCertificateChainStatus; `validateOCPPRequest` returns "invalid" when it finds no schema, so the router answers a valid CALL with `FormatViolation`, not `NotImplemented` (OCPP-J). Not in known-bugs.tsv: citrineos does not declare these actions, so no test runs them | [source](https://github.com/citrineos/citrineos-core/blob/4fd7599b91c2fb6e6065e7c9b94be69880cced27/packages/base/src/interfaces/modules/ocpp-validator.ts#L129-L190) |

Not defects, but why citrineos declares fewer actions: CitrineOS answers by
itself, from its own data and policy. It answers an id token it does not
know with `Unknown` (a valid OCPP 2.x status), where the fixture derives
`Invalid`, so Authorize is not declared in 2.0.1 and 2.1 (the accepted,
blocked and expired tokens get the fixture's status); it
numbers 1.6 transactions itself; and it answers DataTransfer,
Get15118EVCertificate, GetCertificateStatus, NotifyEVChargingNeeds,
NotifySettlement and VatNumberValidation otherwise than the fixture. The
full list is in [adapters/citrineos/README.md](../adapters/citrineos/README.md).

See [tests/known-bugs.tsv](../tests/known-bugs.tsv).
