# chargetime adapter

OCPP 1.6J reference CSMS and station built on
[ChargeTimeEU Java-OCA-OCPP](https://github.com/ChargeTimeEU/Java-OCA-OCPP),
behind the [container contract](../../docs/CONTAINER_CONTRACT.md).

| | |
|---|---|
| Upstream | ChargeTimeEU Java-OCA-OCPP (Java), its own protocol engine |
| Version | v2.0, released 2025-12-12 (git tag `v2.0`, commit `f27fe5bfb03e01491100597d6859080f5ae07545`) |
| Artifacts | `eu.chargetime.ocpp:ocpp16j:2.0` from Maven Central, with `ocpp16`, `ocpp-json` and `ocpp-common` 2.0. Their `-sources` jars are the tree of the tag (apart from line endings) |
| Other dependencies | from the upstream's POMs: Java-WebSocket 1.6.0 (the transport), Gson 2.13.2, jaxb-api 2.3.1, javax.activation-api 1.2.0, slf4j-api 2.0.17, error_prone_annotations 2.41.0; and slf4j-simple 2.0.17 for logging to stderr |
| Licences | Java-OCA-OCPP MIT; Java-WebSocket MIT; Gson and error_prone_annotations Apache-2.0; jaxb-api and javax.activation-api CDDL-1.1 or GPL-2.0 with the Classpath Exception; slf4j MIT |
| Base images | `maven:3.9.16-eclipse-temurin-25-noble` (build), `eclipse-temurin:25.0.4.1_1-jre-noble` (runtime), pinned by digest in the [Dockerfile](Dockerfile) |
| OCPP | 1.6J, both roles, security profiles 0 and 1 |

The adapter code is MIT. `/licenses` in the image holds the licence and
notice files the jars carry, the texts in [licenses/](licenses) for those
that carry none, this repository's licence and THIRD_PARTY_NOTICES.md.

## Build

`mvn -C` fails on a download whose checksum does not match Maven Central's,
and [dependencies.sha256](dependencies.sha256) pins the sha256 of every jar
on the runtime class path: a release build fails on any difference, missing
or extra jar. A candidate build (`make candidate-chargetime
CANDIDATE_VERSION=vX.Y CANDIDATE_SHA=<commit>`) resolves that version from
Maven Central and prints the hashes instead. `UPSTREAM_SHA` is recorded as
`upstream.revision`; Maven Central cannot prove it.

`OCPP_INTEROP_LOG_LEVEL=debug` (or `trace`, which logs every frame) makes the
upstream's logging on stderr more verbose.

## How the upstream is used

Server: `JSONServer` (1.6) with a `ServerCoreProfile`; client: `JSONClient`
with a `ClientCoreProfile`. Both get one more profile, the adapter's
[Endpoint](src/main/java/org/otfabric/ocppinterop/chargetime/Endpoint.java),
whose features are the upstream's own `*Feature` classes (Core, Firmware
Management, Local Auth List, Remote Trigger, Reservation, Smart Charging and
the Security Whitepaper extensions), for the actions the fixture answers and
the actions the role sends. The Core profiles' event handlers delegate to it.

Raw JSON becomes the upstream's request and confirmation types, and goes
back to JSON, with `JSONCommunicator.unpackPayload` and `packPayload`: the
Gson setup the upstream uses on the wire. `validate()`, framing, routing,
the `NotImplemented` CALLERROR for an action without a feature, and Basic
authentication parsing are the upstream's. `ServerEvents.authenticateSession`
refuses unknown identities (404) and, with profile 1, wrong credentials (401).

## Limitations of the upstream

These are why the adapter declares `fixtureCallErrors: false` and
`messageIds: false` (see its `notes` and
[the contract's gaps](../../docs/CONTAINER_CONTRACT.md#gaps-before-10)):

- A handler can return a confirmation or fail; a failure is always answered
  with `InternalError` (`eu.chargetime.ocpp.ConfirmationHandler`). A fixture
  with `callError` answers is refused with exit 5.
- `JSONServer` and `JSONClient` always register the Core profile, so a Core
  action the fixture does not answer is answered with `InternalError`, not
  `NotImplemented`.
- The message id of a sent CALL, the negotiated subprotocol and the close
  code are not exposed: they are absent from result documents and `null` in
  events. Unparsable frames are logged, not reported (`protocol-error`).
- The 1.6 `JSONServer` also accepts a handshake without a subprotocol, and
  refuses Basic credentials whose password is not 16 to 20 bytes before the
  adapter sees them.
- `JSONClient.connect` reports neither success nor failure; the adapter
  watches `ClientEvents.connectionOpened` and bounds the handshake itself.
- When it sends a SetChargingProfile, the upstream adds `numberPhases: 3`
  (the field's default in `ChargingSchedulePeriod`) if the payload has none.
