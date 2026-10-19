# citrineos adapter

A CSMS for OCPP 1.6J, 2.0.1 and 2.1, built on
[CitrineOS core](https://github.com/citrineos/citrineos-core) release
**v2.0.1** (commit `1a83638c31eebdb1dd8d6cdb0f5b8a9f142a1372`, Apache-2.0,
TypeScript). It is the second independent OCPP 2.1 CSMS of this repository,
after `mobilityhouse`. CitrineOS has no charging station: `client` exits 5
after the usage checks.

```bash
docker build -f adapters/citrineos/Dockerfile -t ghcr.io/otfabric/ocpp-interop-citrineos:dev .
docker run --rm -p 127.0.0.1:9000:9000 ghcr.io/otfabric/ocpp-interop-citrineos:dev server --version 2.1
```

## What runs in the container

CitrineOS needs PostgreSQL with PostGIS (its migrations use `geometry`) and
RabbitMQ (AMQP is the only message broker it has: `CitrineOSServer` refuses
to start without one). Neither Hasura (the operator UI's data access), S3 nor
Redis is needed: file access is `local` and the cache `memory`. The image
holds all of it:

| Part | From | Runs as |
|---|---|---|
| CitrineOS `@citrineos/ocpp-server` v2.0.1 | built from the tag with pnpm 10.19.0 and the release's lockfile, as `apps/ocpp-server/deploy.Dockerfile` does | in the adapter's Node.js process |
| PostgreSQL 15, PostGIS 3 | Debian bookworm | a child process, on 127.0.0.1:5432 |
| RabbitMQ 3.10 (Erlang 25) | Debian bookworm | a child process, on 127.0.0.1:5672 |

The database is created (with the PostGIS extension, as the
`postgis/postgis` image of the upstream's compose file does) and migrated
with CitrineOS's own migrations when the image is built, so each container
starts from an empty, migrated database. `server` starts PostgreSQL and
RabbitMQ, then CitrineOS (mode `all`: the WebSocket server, every module and
every API), and is ready after about 15 seconds. Everything CitrineOS and the
two services log goes to stderr.

## How the contract maps onto CitrineOS

CitrineOS is an application, not a library: it answers every CALL from a
station itself, from its database and configuration. The adapter does not
replace those answers (that would test the adapter, not CitrineOS). It
gives CitrineOS what CitrineOS takes from its data:

| Fixture | CitrineOS |
|---|---|
| `stations[].id` | a charging station (`createOrUpdateChargingStation`); the only stations the WebSocket server accepts (`allowUnknownChargingStations: false`: an unknown identity is refused with HTTP 404) |
| `stations[].connectors` (1.6) | a connector each, on an EVSE of its own (`autoCommissionEvseForOcpp16Connector`, `createOrUpdateOcpp16Connector`) |
| `stations[].password` | the device model variable `SecurityCtrlr.BasicAuthPassword`, which CitrineOS checks for security profiles 1 and 2 (HTTP 401 otherwise) |
| `idTokens` | CitrineOS's Authorizations (`createAuthorization`) |
| `answers.csms.BootNotification` | `ocpp.heartbeatInterval` (the `interval`) and `ocpp.unknownChargerStatus` (the `status`) |

All of it goes through CitrineOS's own repositories, from its documented
extension point (`CitrineOSServer.onInitialized`), before the server is
ready. The other answers are CitrineOS's. `handles` in `print-capabilities`
lists the actions whose answers the self-tests found equal to the fixture's;
the rest are in the capability notes and in
[docs/COMPATIBILITY.md](../../docs/COMPATIBILITY.md). For the same reason the
adapter declares `fixtureCallErrors: false` (a fixture with a CSMS `callError`
answer exits 5) and `fixtureAnswers: false` (an action that the fixture does
not answer is still answered by CitrineOS, not with `NotImplemented`).

The WebSocket server is configured per run in `websocket-servers.json`: the
requested port and address, the one subprotocol of `--version`, the security
profile, and for profiles 2 and 3 the files of `--tls-dir` (copied into
CitrineOS's file storage). Profile 3 is CitrineOS's mutual TLS: Node.js
`https` with `requestCert` and `rejectUnauthorized`, `ca.crt` as the trusted
CA, and the identity checked against the client certificate's CommonName.
CitrineOS's configuration schema also requires a CA key path for profile 3;
it only stores it (it would sign station certificates with it), so the
adapter gives a name without a file.

**`call`** sends through CitrineOS's REST message API
(`POST /ocpp/<version>/<module>/<action>?identifier=<station>&tenantId=1`,
the paths CitrineOS registers, read when it registers them) with a
`callbackUrl` on a local HTTP server of the adapter, which CitrineOS posts
the station's answer to. A request CitrineOS's schema validation refuses is
`invalid-request` (HTTP 400); an endpoint that does not send the CALL is
`failed`; an action without an endpoint is `unsupported` (exit 5).

**Events** come from CitrineOS's own objects, which the adapter wraps
without changing what they do: the router's `onMessage` and network hook
(every frame in and out: `call-received`, `call-answered`, `call-sent`,
`call-result`, `protocol-error`, with the message ids CitrineOS reads and
writes), `registerConnection` and the network connection's close handler
(`connection-opened` with the negotiated subprotocol, the remote address and
whether permessage-deflate was negotiated; `connection-closed` with the close
code), and the authenticator and subprotocol check (`connection-rejected`
with reason `unknown-station`, `authentication` or `subprotocol`).

## Limitations

- **A CSMS only.** `client` exits 5.
- **Answers are CitrineOS's** (above). Not handled, because CitrineOS
  answers otherwise than the fixture:
  - StartTransaction (1.6): CitrineOS numbers transactions itself
    (`transactionId` 1, the fixture says 1001).
  - DataTransfer: `Rejected` (the fixture: `UnknownVendorId`).
  - Get15118EVCertificate, GetCertificateStatus: `Failed` (no V2G or OCSP
    service is configured).
  - NotifyEVChargingNeeds: `Rejected`; NotifySettlement: no `receiptId`;
    VatNumberValidation: `Rejected`.
  - Authorize in 2.0.1 and 2.1: CitrineOS answers the baseline's accepted,
    blocked and expired tokens with the fixture's status, but a token it does
    not know with `Unknown`, a valid OCPP 2.x status, where the fixture
    derives `Invalid`
    ([authorize-request-ocpp-21-handler.ts](https://github.com/citrineos/citrineos-core/blob/1a83638c31eebdb1dd8d6cdb0f5b8a9f142a1372/packages/ocpp/src/handlers/requests/2/authorize-request-ocpp-21-handler.ts#L292-L293)).
    It refuses an ISO14443 token that is not 8 or 14 hexadecimal characters
    with `PropertyConstraintViolation` (OCPP 2.x defines ISO14443 as a 4 or 7
    byte UID in hexadecimal); the baseline's tokens are valid ones since
    v0.4.0.
  - NotifyCustomerInformation, NotifyDisplayMessages:
    `PropertyConstraintViolation` for a `requestId` CitrineOS did not send.
  - SignCertificate: `FormatViolation` (CitrineOS parses the CSR).
  - 1.6 Security Whitepaper messages from the station (SecurityEventNotification,
    LogStatusNotification, SignCertificate, SignedFirmwareStatusNotification):
    `NotImplemented`.
  - OCPP 2.1 NotifyDERAlarm, NotifyDERStartStop, ReportDERControl,
    OpenPeriodicEventStream, ClosePeriodicEventStream, NotifyPriorityCharging,
    PullDynamicScheduleUpdate, GetCertificateChainStatus: `FormatViolation`
    for the baseline samples, which validate against the OCA 2.1 schemas.
- **Not sent:** CitrineOS's message API has no endpoint for the 1.6
  reservation messages (ReserveNow, CancelReservation) and Security
  Whitepaper messages, nor for most actions OCPP 2.1 adds (all but
  SetDefaultTariff). Endpoints that exist but do not send the baseline
  sample: CancelReservation (2.x; only for a reservation CitrineOS made),
  GetCompositeSchedule (2.x; HTTP 500), InstallCertificate (it refuses the
  fixture's certificate), SignedUpdateFirmware (1.6; HTTP 500).
- **One outstanding CALL per station** (CitrineOS's router); a station that
  does not answer within CitrineOS's `maxCallLengthSeconds` (20) is
  disconnected by CitrineOS.
- **A second connection for a connected identity** is refused with HTTP 401
  (CitrineOS's ConnectedStationFilter), reported as `connection-rejected`
  with reason `authentication`.
- **Security profile 3:** once a BootNotification has given CitrineOS a
  serial number, it requires the client certificate's CommonName to equal
  that serial number, not the identity
  ([client-certificate-filter.ts](https://github.com/citrineos/citrineos-core/blob/1a83638c31eebdb1dd8d6cdb0f5b8a9f142a1372/packages/ocpp/src/transport/network-connection/authenticator/client-certificate-filter.ts)).
- **No SEND, no CALLRESULTERROR:** CitrineOS's router knows the message types
  2 to 4 only and answers any other with `FormatViolation`.
- **Startup** takes about 15 seconds (PostgreSQL, RabbitMQ, CitrineOS).
