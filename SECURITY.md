# Security Policy

## Reporting a Vulnerability

If you find a security vulnerability in `ocpp-interop`, please report it
responsibly.

**Do not open a public GitHub issue for security vulnerabilities.**

Instead, email **security@otfabric.io**, or open a private advisory on
GitHub: on the [Security](https://github.com/otfabric/ocpp-interop/security)
tab, choose **Advisories**, then **Report a vulnerability**. Include a
description, the steps to reproduce, and the affected image tag or digest.

Vulnerabilities in the bundled implementations themselves belong upstream.
Report them there, and tell us so that we can move the pin:

- The Mobility House [ocpp](https://github.com/mobilityhouse/ocpp) (mobilityhouse)
- ChargeTimeEU [Java-OCA-OCPP](https://github.com/ChargeTimeEU/Java-OCA-OCPP) (chargetime)
- lorenzodonini [ocpp-go](https://github.com/lorenzodonini/ocpp-go) (lorenzodonini)
- EVerest libocpp, in [everest-core](https://github.com/EVerest/everest-core) (everest)
- [CitrineOS core](https://github.com/citrineos/citrineos-core) (citrineos)
- [cB-Sigma/ocpp](https://github.com/cB-Sigma/ocpp) (cbsigma)
- [hupe1980/ocpp-kit](https://github.com/hupe1980/ocpp-kit) (ocppkit)
- [seanlee0923/ocpp](https://github.com/seanlee0923/ocpp) (seanlee)

## Supported Versions

| Version | Supported |
|---------|-----------|
| Latest release | Yes |
| Older releases | No; pull the latest images |

## Security Considerations

### These are test fixtures, not products

By default the images run an **unauthenticated, unencrypted OCPP CSMS**, and
stations that accept remote commands from whatever CSMS they are pointed at.
That is their purpose.

- **Never connect a reference station to a production CSMS, and never
  connect a real charging station to a reference CSMS.**
- Publish ports on `127.0.0.1` only. The self-tests use private Docker
  networks and publish nothing.
- The station password in the fixtures is public by design.

### TLS material

No private key is committed or baked into an image. TLS scenarios generate
keys and certificates when a test runs, and mount them read-only.

### Image hardening

- Processes run as a non-root user (uid 10001).
- Base images are pinned by digest, and upstreams by exact version and
  commit ([versions.yaml](versions.yaml)).
- Runtime images contain the adapter, its runtime and the fixtures. Build
  tools stay in the build stages.
