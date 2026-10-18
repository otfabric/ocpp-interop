# chargetime adapter

OCPP 1.6J and 2.0.1 reference CSMS and station built on
[ChargeTimeEU Java-OCA-OCPP](https://github.com/ChargeTimeEU/Java-OCA-OCPP),
behind the [container contract](../../docs/CONTAINER_CONTRACT.md).

| | |
|---|---|
| Upstream | ChargeTimeEU Java-OCA-OCPP (Java), its own protocol engine |
| Version | v2.0, released 2025-12-12 (git tag `v2.0`, commit `f27fe5bfb03e01491100597d6859080f5ae07545`) |
| Artifacts | `eu.chargetime.ocpp:ocpp16j:2.0` (OCPP 1.6J) and `eu.chargetime.ocpp:ocpp2:2.0` (OCPP 2.0.1) from Maven Central, with `ocpp16`, `ocpp-json` and `ocpp-common` 2.0. Their `-sources` jars are the tree of the tag (apart from line endings) |
| Other dependencies | from the upstream's POMs: Java-WebSocket 1.6.0 (the transport), Gson 2.13.2, jaxb-api 2.3.1, javax.activation-api 1.2.0, slf4j-api 2.0.17, error_prone_annotations 2.41.0, jsr305 3.0.2; and slf4j-simple 2.0.17 for logging to stderr |
| Licences | Java-OCA-OCPP MIT; Java-WebSocket MIT; Gson, error_prone_annotations and jsr305 Apache-2.0; jaxb-api and javax.activation-api CDDL-1.1 or GPL-2.0 with the Classpath Exception; slf4j MIT |
| Base images | `maven:3.9.16-eclipse-temurin-25-noble` (build), `eclipse-temurin:25.0.4.1_1-jre-noble` (runtime), pinned by digest in the [Dockerfile](Dockerfile) |
| OCPP | 1.6J and 2.0.1, both roles, security profiles 0 to 3; compression in 2.0.1 (with known upstream defects), not in 1.6. No 2.1: see below |

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

OCPP 1.6: server `JSONServer` with a `ServerCoreProfile`; client
`JSONClient` with a `ClientCoreProfile`. Both get one more profile, the
adapter's
[Endpoint](src/main/java/org/otfabric/ocppinterop/chargetime/Endpoint.java),
whose features are the upstream's own `*Feature` classes (Core, Firmware
Management, Local Auth List, Remote Trigger, Reservation, Smart Charging and
the Security Whitepaper extensions), for the actions the fixture answers and
the actions the role sends. The Core profiles' event handlers delegate to it.

OCPP 2.0.1: server `MultiProtocolJSONServer`, client
`MultiProtocolJSONClient`, each for the one protocol version
`ProtocolVersion.OCPP2_0_1`, with the Endpoint as their one `Function`,
whose features are the upstream's `eu.chargetime.ocpp.v201.feature` classes.
There is no forced Core profile: an action without a fixture answer is
answered `NotImplemented` by the upstream.

Raw JSON becomes the upstream's request and confirmation types, and goes
back to JSON, with `JSONCommunicator.unpackPayload` and `packPayload`: the
Gson setup the upstream uses on the wire. `validate()`, framing, routing,
the `NotImplemented` CALLERROR for an action without a feature, and Basic
authentication parsing are the upstream's. `ServerEvents.authenticateSession`
refuses unknown identities (404) and, with profiles 1 and 2, wrong
credentials (401; Java-WebSocket sends 404, see below).

## Transport security and compression

Security profiles 2 and 3 use the upstream's own WSS support. The CSMS
calls `enableWSS` with a `WssFactoryBuilder` that builds Java-WebSocket's
`SSLParametersWebSocketServerFactory` from a JSSE `SSLContext` (key manager
from `server.crt` and `server.key`; with profile 3 a trust manager from
`ca.crt` and `needClientAuth`), TLS 1.3 and 1.2. The factory keeps the
`SSLEngine` of each connection by remote address, because
`ServerEvents.authenticateSession` gets the address, not the connection: with
profile 3 the adapter reads the verified client certificate there and
refuses one whose CommonName is not the identity. The station calls
`enableWSS` with the upstream's `BaseWssSocketBuilder` and an
`SSLSocketFactory` that trusts `ca.crt` (and presents `client.crt` with
profile 3); Java-WebSocket's `WebSocketClient` turns on host name
verification (`onSetSSLParameters`, endpoint identification `HTTPS`). Keys
must be PKCS #8 PEM, RSA or EC.

Compression is the upstream's `JSONConfiguration.WEBSOCKET_COMPRESSION_SUPPORT`:
true by default for the 2.0.1 `MultiProtocolJSONServer`, which therefore
accepts permessage-deflate whenever it is offered, and set by `--compression`
for the 2.0.1 `MultiProtocolJSONClient`. The 1.6 `JSONServer` and
`JSONClient` build their `Draft_6455` without extensions
([JSONServer.java](https://github.com/ChargeTimeEU/Java-OCA-OCPP/blob/f27fe5bfb03e01491100597d6859080f5ae07545/ocpp-v1_6-json/src/main/java/eu/chargetime/ocpp/JSONServer.java#L70),
[JSONClient.java](https://github.com/ChargeTimeEU/Java-OCA-OCPP/blob/f27fe5bfb03e01491100597d6859080f5ae07545/ocpp-v1_6-json/src/main/java/eu/chargetime/ocpp/JSONClient.java#L72-L73)):
no compression in 1.6, and `--compression` with `--version 1.6` exits 5.
Whether compression was negotiated is not exposed: `connection-opened` has
`compression` `null` where it may have been, `false` where it cannot have
been.

## Limitations of the upstream

- **No OCPP 2.1 in the release.** The v2.0 tag's `ocpp-v2` module holds
  `eu.chargetime.ocpp.v201` only
  ([tree](https://github.com/ChargeTimeEU/Java-OCA-OCPP/tree/f27fe5bfb03e01491100597d6859080f5ae07545/ocpp-v2/src/main/java/eu/chargetime/ocpp)),
  and `eu.chargetime.ocpp:ocpp2:2.0` is the latest ocpp2 on Maven Central.
  The upstream's master added `eu.chargetime.ocpp.v21` and the 2.1 RPC
  extensions (SEND, CALLRESULTERROR) after that release (commits `febd153b`,
  2025-12-13, and `31aea97a`, 2025-12-19), not released as of 2026-10-18.
  The adapter refuses `--version 2.1` (exit 5) until a release has it.

These are why the adapter declares `fixtureCallErrors: false` and
`messageIds: false` (see its `notes` and
[the contract's gaps](../../docs/CONTAINER_CONTRACT.md#gaps-before-10)):

- A handler can return a confirmation or fail; a failure is always answered
  with `InternalError` (`eu.chargetime.ocpp.ConfirmationHandler`). A fixture
  with `callError` answers is refused with exit 5.
- OCPP 1.6: `JSONServer` and `JSONClient` always register the Core profile,
  so a Core action the fixture does not answer is answered with
  `InternalError`, not `NotImplemented`.
- The message id of a sent CALL, the negotiated subprotocol and the close
  code are not exposed: they are absent from result documents and `null` in
  events. Unparsable frames are logged, not reported (`protocol-error`).
- The 1.6 `JSONServer` also accepts a handshake without a subprotocol, and
  refuses Basic credentials whose password is not 16 to 20 bytes (16 to 40
  for 2.0.1) before the adapter sees them.
- `JSONClient.connect` reports neither success nor failure; the adapter
  watches `ClientEvents.connectionOpened` and bounds the handshake itself.
- When it sends a SetChargingProfile, the upstream adds `numberPhases: 3`
  (the field's default in `ChargingSchedulePeriod`) if the payload has none.
- Java-WebSocket 1.6.0 answers every handshake the adapter refuses with HTTP
  404 ([`WebSocketImpl.closeConnectionDueToWrongHandshake`](https://github.com/TooTallNate/Java-WebSocket/blob/da3cf2a777aed862f2f5b5cf060cae7969958667/src/main/java/org/java_websocket/WebSocketImpl.java#L426-L428)),
  also when the adapter asks for 401 (wrong credentials, a certificate of
  another identity). The `connection-rejected` event has the right reason.

## Known upstream defects

Both are recorded in [tests/known-bugs.tsv](../../tests/known-bugs.tsv), so
that the `v201.compression` tests with this adapter at either end are
`KNOWN_ISSUE`, never `PASS`. Compression works only with a peer that keeps
its context too and does not hold this adapter to what it announces; none
of the reference adapters is such a peer, this one included.

- OCPP 2.0.1 server: `MultiProtocolJSONServer` accepts permessage-deflate by
  default and keeps its compression context between messages
  (`setServerNoContextTakeover(false)`), but Java-WebSocket 1.6.0, which the
  upstream's POM pulls in, always answers the handshake with
  `server_no_context_takeover`
  ([source](https://github.com/TooTallNate/Java-WebSocket/blob/da3cf2a777aed862f2f5b5cf060cae7969958667/src/main/java/org/java_websocket/extensions/permessage_deflate/PerMessageDeflateExtension.java#L335-L340)).
  A station that relies on it (websockets, gorilla/websocket) cannot
  decompress the server's second message and closes the connection. The
  adapter keeps the upstream's default. Java-WebSocket's master announces
  the parameter only when it is set.
- OCPP 2.0.1 station: with `WEBSOCKET_COMPRESSION_SUPPORT`, the
  `MultiProtocolJSONClient` offers permessage-deflate with
  `client_no_context_takeover` (Java-WebSocket always adds it,
  [source](https://github.com/TooTallNate/Java-WebSocket/blob/da3cf2a777aed862f2f5b5cf060cae7969958667/src/main/java/org/java_websocket/extensions/permessage_deflate/PerMessageDeflateExtension.java#L326-L333)),
  but keeps its compression context: `encodeFrame` resets the deflater only
  when `serverNoContextTakeover` is set
  ([L258-L260](https://github.com/TooTallNate/Java-WebSocket/blob/da3cf2a777aed862f2f5b5cf060cae7969958667/src/main/java/org/java_websocket/extensions/permessage_deflate/PerMessageDeflateExtension.java#L258-L260)),
  which `MultiProtocolJSONClient` sets to false
  ([source](https://github.com/ChargeTimeEU/Java-OCA-OCPP/blob/f27fe5bfb03e01491100597d6859080f5ae07545/ocpp-v2/src/main/java/eu/chargetime/ocpp/MultiProtocolJSONClient.java#L94-L100)).
  A CSMS that takes the offer (websockets, and Java-WebSocket itself)
  cannot decompress the station's second message.
