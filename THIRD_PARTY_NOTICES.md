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
| Java-OCA-OCPP: ocpp16j, ocpp16, ocpp2, ocpp-json, ocpp-common (ChargeTimeEU) | 2.0 | MIT | https://github.com/ChargeTimeEU/Java-OCA-OCPP |
| Java-WebSocket | 1.6.0 | MIT | https://github.com/TooTallNate/Java-WebSocket |
| Gson | 2.13.2 | Apache-2.0 | https://github.com/google/gson |
| error_prone_annotations | 2.41.0 | Apache-2.0 | https://github.com/google/error-prone |
| jsr305 (com.google.code.findbugs) | 3.0.2 | Apache-2.0 | http://findbugs.sourceforge.net (code.google.com/p/jsr-305) |
| jaxb-api (javax.xml.bind) | 2.3.1 | CDDL-1.1 or GPL-2.0 with the Classpath Exception | https://github.com/javaee/jaxb-spec |
| javax.activation-api | 1.2.0 | CDDL-1.1 or GPL-2.0 with the Classpath Exception | https://github.com/javaee/activation |
| SLF4J API and SLF4J Simple | 2.0.17 | MIT | https://www.slf4j.org |

## `ocpp-interop-lorenzodonini`

Base image `debian:trixie-slim` (Debian trixie). One statically linked Go
binary (built with Go 1.27.2); `/licenses/BUNDLED-MODULES.txt` lists the
modules in it, as `go version -m` reports them.

| Component | Version | Licence | Source |
|---|---|---|---|
| ocpp-go (lorenzodonini) | v0.19.0 | MIT | https://github.com/lorenzodonini/ocpp-go |
| gorilla/websocket | v1.5.3 | BSD-2-Clause | https://github.com/gorilla/websocket |
| gorilla/mux | v1.8.1 | BSD-3-Clause | https://github.com/gorilla/mux |
| go-playground/validator (gopkg.in/go-playground/validator.v9) | v9.30.0 | MIT | https://github.com/go-playground/validator |
| go-playground/universal-translator | v0.16.0 | MIT | https://github.com/go-playground/universal-translator |
| go-playground/locales | v0.12.1 | MIT | https://github.com/go-playground/locales |
| leodido/go-urn | v1.1.0 | MIT (the module has no licence file; the text in `/licenses` is the one the project added with v1.2.0) | https://github.com/leodido/go-urn |
| relvacode/iso8601 | v1.6.0 | MIT | https://github.com/relvacode/iso8601 |
| Go standard library and runtime | 1.27.2 | BSD-3-Clause | https://go.dev |

## `ocpp-interop-everest`

Base image `debian:trixie-slim` (Debian trixie), whose OpenSSL 3, SQLite 3,
zlib, zstd and C++ runtime the executable links dynamically. One executable
built from source, and libwebsockets as a shared library;
`/licenses/BUNDLED.txt` lists every source by tag and commit.

| Component | Version | Licence | Source |
|---|---|---|---|
| libocpp (EVerest, `lib/everest/ocpp` of everest-core) | everest-core 2026.02.1 (libocpp 0.31.1) | Apache-2.0 | https://github.com/EVerest/everest-core |
| EVerest liblog, libtimer, everest-sqlite, libevse-security (`lib/everest/log`, `timer`, `sqlite`, `evse_security`) | everest-core 2026.02.1 | Apache-2.0 | https://github.com/EVerest/everest-core |
| WebSocket++ `uri.hpp` and `base64.hpp` (bundled in libocpp's `3rd_party/websocketpp_utils`) | as bundled | BSD-3-Clause (the licence is in the headers) | https://github.com/zaphoyd/websocketpp |
| libwebsockets | 4.5.2 | MIT (some bundled sources BSD-3-Clause, BSD-2-Clause; see its LICENSE) | https://github.com/warmcat/libwebsockets |
| nlohmann/json | 3.12.0 | MIT | https://github.com/nlohmann/json |
| json-schema-validator (pboettch) | 2.4.0 | MIT | https://github.com/pboettch/json-schema-validator |
| date (Howard Hinnant) | 3.0.4 | MIT | https://github.com/HowardHinnant/date |
| Boost (log and the libraries it uses: thread, filesystem, regex, chrono, atomic; Debian's static libraries) | 1.83 | BSL-1.0 | https://www.boost.org |
