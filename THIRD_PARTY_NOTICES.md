# Third-party notices

## What is licensed how

| Part | Licence |
|---|---|
| This repository (adapters, scripts, fixtures, schemas, documentation) | MIT, see [LICENSE](LICENSE) |
| Upstream OCPP implementations and their dependencies inside the images | their own licences, listed below per image |
| OCA specifications and JSON schemas | CC BY-ND 4.0; **not committed** to this repository. The `mobilityhouse` image contains the OCA JSON schemas the ocpp package ships, unmodified, as part of that package |

Each image carries the full licence texts under `/licenses/<component>/`,
this file as `/licenses/THIRD_PARTY_NOTICES.md`, and this repository's
licence as `/licenses/ocpp-interop/LICENSE`. Base images (Debian, Ubuntu,
Python, Eclipse Temurin) carry their own notices in their usual places.

## `ocpp-interop-mobilityhouse`

Base image `python:3.14.8-slim-trixie` (Debian trixie, CPython 3.14.8).

| Component | Version | Licence | Source |
|---|---|---|---|
| ocpp (The Mobility House) | 2.1.0 | MIT | https://github.com/mobilityhouse/ocpp |
| OCA OCPP JSON schemas (inside the ocpp package: 1.6, 2.0.1, 2.1) | as shipped by ocpp 2.1.0 | CC BY-ND 4.0, © Open Charge Alliance; unmodified | https://openchargealliance.org |
| websockets | 17.2 | BSD-3-Clause | https://github.com/python-websockets/websockets |
| jsonschema | 4.26.0 | MIT | https://github.com/python-jsonschema/jsonschema |
| jsonschema-specifications | 2025.9.1 | MIT | https://github.com/python-jsonschema/jsonschema-specifications |
| referencing | 0.37.0 | MIT | https://github.com/python-jsonschema/referencing |
| rpds-py | 2026.9.1 | MIT | https://github.com/crate-py/rpds |
| attrs | 26.1.0 | MIT | https://github.com/python-attrs/attrs |
| CPython | 3.14.8 | PSF-2.0 | https://www.python.org |

## `ocpp-interop-chargetime`

Base image `eclipse-temurin:25.0.4.1_1-jre-noble` (Ubuntu noble, Eclipse
Temurin JRE 25, GPL-2.0 with the Classpath Exception).

| Component | Version | Licence | Source |
|---|---|---|---|
| Java-OCA-OCPP: ocpp16j, ocpp16, ocpp-json, ocpp-common (ChargeTimeEU) | 2.0 | MIT | https://github.com/ChargeTimeEU/Java-OCA-OCPP |
| Java-WebSocket | 1.6.0 | MIT | https://github.com/TooTallNate/Java-WebSocket |
| Gson | 2.13.2 | Apache-2.0 | https://github.com/google/gson |
| error_prone_annotations | 2.41.0 | Apache-2.0 | https://github.com/google/error-prone |
| jaxb-api (javax.xml.bind) | 2.3.1 | CDDL-1.1 or GPL-2.0 with the Classpath Exception | https://github.com/javaee/jaxb-spec |
| javax.activation-api | 1.2.0 | CDDL-1.1 or GPL-2.0 with the Classpath Exception | https://github.com/javaee/activation |
| SLF4J API and SLF4J Simple | 2.0.17 | MIT | https://www.slf4j.org |
