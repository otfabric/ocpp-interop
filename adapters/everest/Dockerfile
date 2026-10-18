# SPDX-License-Identifier: MIT
#
# ocpp-interop adapter for EVerest libocpp (C++).
#
# Build from the repository root:
#   docker build -f adapters/everest/Dockerfile -t ocpp-interop-everest:dev .
#
# libocpp is Apache-2.0. It lives in the EVerest monorepo everest-core
# (lib/everest/ocpp) since the standalone EVerest/libocpp repository was
# archived. The pin is UPSTREAM_VERSION below (the everest-core release tag)
# and UPSTREAM_SHA (its commit), mirrored in versions.yaml. The build compiles
# libocpp and the four EVerest libraries it links (log, timer, sqlite,
# evse_security) from that tree, and the adapter (MIT) against them; nothing
# else of everest-core is built. libocpp's own third-party dependencies are
# built from the tags its lib/everest/ocpp/dependencies.yaml names, each
# checked against its commit; Boost, OpenSSL and SQLite are Debian's.

# Where the base images come from. They are pinned by digest, so a mirror of
# Docker Hub serves exactly the same images (see the Makefile).
ARG BASE_REGISTRY=docker.io/library
# debian:trixie-slim, multi-arch index (linux/amd64 and linux/arm64): the
# build stage and the runtime.
ARG DEBIAN_DIGEST=sha256:a29215f6a35e51e22adffa17f89e9d2ef06214e64a2bad10d765c46aea49f11f

# ---------------------------------------------------------------------------
# Stage 1: toolchain, the pinned sources and libocpp's dependencies
# ---------------------------------------------------------------------------
FROM ${BASE_REGISTRY}/debian:trixie-slim@${DEBIAN_DIGEST} AS deps

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates git cmake ninja-build g++ pkg-config \
        libboost-dev libboost-filesystem-dev libboost-log-dev libboost-program-options-dev \
        libboost-regex-dev libboost-system-dev libboost-thread-dev \
        libssl-dev libsqlite3-dev \
    && rm -rf /var/lib/apt/lists/*

# fetch <repository> <tag> <commit> <directory>: a shallow clone of the tag,
# which must be that commit.
RUN printf '%s\n' '#!/bin/sh' 'set -eu' \
        'git -c advice.detachedHead=false clone -q --depth 1 --branch "$2" "$1" "$4"' \
        'got="$(git -C "$4" rev-parse HEAD)"' \
        '[ "${got}" = "$3" ] || { echo "$1 $2 is commit ${got}, not $3" >&2; exit 1; }' \
        > /usr/local/bin/fetch \
    && chmod +x /usr/local/bin/fetch

# libocpp's third-party dependencies, at the tags of
# lib/everest/ocpp/dependencies.yaml of the pinned everest-core release, with
# the options it gives them.
WORKDIR /src
RUN set -eu; \
    fetch https://github.com/nlohmann/json v3.12.0 55f93686c01528224f448c19128836e7df245f72 json; \
    cmake -S json -B json/build -G Ninja -DCMAKE_BUILD_TYPE=Release -DCMAKE_INSTALL_PREFIX=/opt/deps \
        -DJSON_BuildTests=OFF -DJSON_MultipleHeaders=ON; \
    cmake --build json/build --target install
RUN set -eu; \
    fetch https://github.com/pboettch/json-schema-validator 2.4.0 c780404a84dd9ba978ba26bc58d17cb43fa7bc80 json-schema-validator; \
    cmake -S json-schema-validator -B json-schema-validator/build -G Ninja -DCMAKE_BUILD_TYPE=Release \
        -DCMAKE_INSTALL_PREFIX=/opt/deps -DCMAKE_PREFIX_PATH=/opt/deps -DCMAKE_POSITION_INDEPENDENT_CODE=ON \
        -DJSON_VALIDATOR_INSTALL=ON -DJSON_VALIDATOR_BUILD_TESTS=OFF -DJSON_VALIDATOR_BUILD_EXAMPLES=OFF \
        -DJSON_VALIDATOR_BUILD_SHARED_LIBS=OFF; \
    cmake --build json-schema-validator/build --target install
RUN set -eu; \
    fetch https://github.com/HowardHinnant/date v3.0.4 f94b8f36c6180be0021876c4a397a054fe50c6f2 date; \
    cmake -S date -B date/build -G Ninja -DCMAKE_BUILD_TYPE=Release -DCMAKE_INSTALL_PREFIX=/opt/deps \
        -DCMAKE_POSITION_INDEPENDENT_CODE=ON -DBUILD_SHARED_LIBS=OFF \
        -DBUILD_TZ_LIB=ON -DHAS_REMOTE_API=0 -DUSE_AUTOLOAD=0 -DUSE_SYSTEM_TZ_DB=ON; \
    cmake --build date/build --target install
# libwebsockets: libocpp links its shared library (target websockets_shared).
RUN set -eu; \
    fetch https://github.com/warmcat/libwebsockets v4.5.2 85c6f7959fd40d8aaf7a50be3c9b75f08389a01c libwebsockets; \
    cmake -S libwebsockets -B libwebsockets/build -G Ninja -DCMAKE_BUILD_TYPE=Release \
        -DCMAKE_INSTALL_PREFIX=/opt/deps -DCMAKE_INSTALL_LIBDIR=lib \
        -DCMAKE_POLICY_VERSION_MINIMUM=3.5 -DCMAKE_POLICY_DEFAULT_CMP0077=NEW \
        -DLWS_ROLE_RAW_FILE=OFF -DLWS_UNIX_SOCK=OFF -DLWS_IPV6=ON -DLWS_WITH_SYS_STATE=OFF \
        -DLWS_WITH_SYS_SMD=OFF -DLWS_WITH_UPNG=OFF -DLWS_WITH_JPEG=OFF -DLWS_WITH_DLO=OFF \
        -DLWS_WITH_SECURE_STREAMS=OFF -DLWS_WITH_STATIC=OFF -DLWS_WITH_LHP=OFF -DLWS_WITH_LEJP_CONF=OFF \
        -DLWS_WITH_MINIMAL_EXAMPLES=OFF -DLWS_WITH_CACHE_NSCOOKIEJAR=OFF -DLWS_WITHOUT_TESTAPPS=ON \
        -DLWS_WITHOUT_TEST_SERVER=ON -DLWS_WITHOUT_TEST_SERVER_EXTPOLL=ON -DLWS_WITHOUT_TEST_PING=ON \
        -DLWS_WITHOUT_TEST_CLIENT=ON -DDISABLE_WERROR=ON; \
    cmake --build libwebsockets/build --target install
# everest-cmake: the CMake helpers every EVerest library starts with (build
# only). v0.6.0 is the release that was current when everest-core 2026.02.1
# was tagged (2026-07-16); everest-core itself fetches its main branch.
RUN fetch https://github.com/EVerest/everest-cmake v0.6.0 e378100829014ad4c3a721a7aa1c02ef56736c61 everest-cmake

# The release pin: everest-core release 2026.02.1 and its commit. A candidate
# build ("make candidate-everest CANDIDATE_VERSION=... CANDIDATE_SHA=...")
# overrides both. Only lib/everest of the tree is checked out.
ARG UPSTREAM_VERSION=2026.02.1
ARG UPSTREAM_SHA=b61bb12b8cb89ee9d0fab037b05c3bb8cea9b13e
RUN set -eu; \
    echo "${UPSTREAM_SHA}" | grep -Eq '^[0-9a-f]{40}$'; \
    git -c advice.detachedHead=false clone -q --depth 1 --filter=blob:none --sparse \
        --branch "${UPSTREAM_VERSION}" https://github.com/EVerest/everest-core everest-core; \
    git -C everest-core sparse-checkout set lib/everest/log lib/everest/timer lib/everest/sqlite \
        lib/everest/evse_security lib/everest/ocpp; \
    got="$(git -C everest-core rev-parse HEAD)"; \
    [ "${got}" = "${UPSTREAM_SHA}" ] || { echo "everest-core ${UPSTREAM_VERSION} is ${got}, not ${UPSTREAM_SHA}" >&2; exit 1; }; \
    test -f everest-core/lib/everest/ocpp/include/ocpp/v21/messages/NotifyPeriodicEventStream.hpp

# ---------------------------------------------------------------------------
# Stage 2: libocpp and the adapter
# ---------------------------------------------------------------------------
FROM deps AS build

ARG UPSTREAM_VERSION=2026.02.1
ARG UPSTREAM_SHA=b61bb12b8cb89ee9d0fab037b05c3bb8cea9b13e
ARG ADAPTER_VERSION=dev
# Parallel compile jobs. libocpp's OCPP 2.x sources need about 1 GiB of
# memory each.
ARG BUILD_JOBS=4

COPY adapters/everest/CMakeLists.txt /src/adapter/CMakeLists.txt
COPY adapters/everest/src /src/adapter/src
# The adapter's action lists name every message header of libocpp, so that
# each action goes through libocpp's own types: a release with other messages
# fails here until src/registry_*.cpp follow it.
RUN set -eu; \
    for dir in v16 v2 v21; do \
        file=/src/adapter/src/registry_v2.cpp; [ "${dir}" = v16 ] && file=/src/adapter/src/registry_v16.cpp; \
        ls "/src/everest-core/lib/everest/ocpp/include/ocpp/${dir}/messages" | sed 's/\.hpp$//' \
            | grep -vx NotifyPeriodicEventStream | sort > /tmp/headers; \
        sed -nE "s|^#include <ocpp/${dir}/messages/([A-Za-z0-9]+)\.hpp>$|\1|p" "${file}" | sort > /tmp/listed; \
        diff /tmp/headers /tmp/listed || { echo "libocpp's ${dir} messages differ from ${file}" >&2; exit 1; }; \
    done
RUN set -eu; \
    cmake -S /src/adapter -B /build -G Ninja -DCMAKE_BUILD_TYPE=Release -DCMAKE_SKIP_BUILD_RPATH=ON \
        -DCMAKE_PREFIX_PATH=/opt/deps -Deverest-cmake_DIR=/src/everest-cmake \
        -DEVEREST_LIB_DIR=/src/everest-core/lib/everest \
        -DADAPTER_VERSION="${ADAPTER_VERSION}" -DUPSTREAM_VERSION="${UPSTREAM_VERSION}" \
        -DUPSTREAM_REVISION="${UPSTREAM_SHA}"; \
    cmake --build /build --target ocpp-interop --parallel "${BUILD_JOBS}"; \
    mkdir -p /out/bin /out/lib; \
    strip -o /out/bin/ocpp-interop /build/ocpp-interop

# Shared libraries: the executable links libwebsockets (built above) and,
# from Debian, OpenSSL, SQLite and the C++ runtime, which the runtime image
# has. Boost and the other libraries are linked statically. Anything else is
# an error, so that a library never reaches the image without its licence.
RUN set -eu; \
    LD_LIBRARY_PATH=/opt/deps/lib ldd /out/bin/ocpp-interop | tee /tmp/ldd >&2; \
    ! grep -q 'not found' /tmp/ldd; \
    for lib in $(awk '$2 == "=>" { print $3 }' /tmp/ldd); do \
        case "$(basename "${lib}")" in \
            libwebsockets.so.*) cp -L "${lib}" /out/lib/ ;; \
            libc.so.*|libm.so.*|libgcc_s.so.*|libstdc++.so.*|libssl.so.*|libcrypto.so.*|libsqlite3.so.*|libz.so.*|libzstd.so.*) ;; \
            *) echo "unexpected shared library ${lib}" >&2; exit 1 ;; \
        esac; \
    done

# The licence of everything in the executable. The EVerest libraries are
# Apache-2.0 (everest-core has no NOTICE file); libocpp bundles two headers of
# WebSocket++ (3rd_party/websocketpp_utils, BSD-3-Clause, the licence is in
# the headers). Boost is Debian's static build (BSL-1.0).
RUN set -eu; \
    copy() { dir="/out/licenses/$1"; shift; mkdir -p "${dir}"; cp "$@" "${dir}/"; }; \
    lib=/src/everest-core/lib/everest; \
    copy everest-core "${lib}/ocpp/LICENSE"; \
    for part in ocpp log timer sqlite evse_security; do test -f "${lib}/${part}/LICENSE"; done; \
    copy everest-core/ocpp/3rd_party/websocketpp_utils "${lib}/ocpp/3rd_party/websocketpp_utils/uri.hpp" \
        "${lib}/ocpp/3rd_party/websocketpp_utils/base64.hpp"; \
    copy nlohmann-json /src/json/LICENSE.MIT; \
    copy json-schema-validator /src/json-schema-validator/LICENSE; \
    copy date /src/date/LICENSE.txt; \
    copy libwebsockets /src/libwebsockets/LICENSE; \
    copy boost /usr/share/doc/libboost1.83-dev/copyright; \
    printf '%s\n' "everest-core ${UPSTREAM_VERSION} ${UPSTREAM_SHA} (lib/everest: ocpp, log, timer, sqlite, evse_security)" \
        "nlohmann/json v3.12.0 55f93686c01528224f448c19128836e7df245f72" \
        "pboettch/json-schema-validator 2.4.0 c780404a84dd9ba978ba26bc58d17cb43fa7bc80" \
        "HowardHinnant/date v3.0.4 f94b8f36c6180be0021876c4a397a054fe50c6f2" \
        "warmcat/libwebsockets v4.5.2 85c6f7959fd40d8aaf7a50be3c9b75f08389a01c" \
        "Debian trixie: $(dpkg-query -W -f '${Package} ${Version}, ' libboost1.83-dev libssl-dev libsqlite3-dev)" \
        > /out/licenses/BUNDLED.txt

# ---------------------------------------------------------------------------
# Stage 3: runtime
# ---------------------------------------------------------------------------
FROM ${BASE_REGISTRY}/debian:trixie-slim@${DEBIAN_DIGEST} AS runtime

ARG ADAPTER_VERSION=dev

RUN useradd --uid 10001 --no-create-home --shell /usr/sbin/nologin interop \
    && mkdir -p /run/ocpp-interop \
    && chown interop /run/ocpp-interop

COPY --from=build /out/bin/ocpp-interop /usr/local/bin/ocpp-interop
COPY --from=build /out/lib/ /usr/local/lib/
RUN ldconfig && /usr/local/bin/ocpp-interop version
COPY --from=build /out/licenses /licenses
COPY LICENSE /licenses/ocpp-interop/LICENSE
COPY THIRD_PARTY_NOTICES.md /licenses/THIRD_PARTY_NOTICES.md
COPY fixtures /fixtures

LABEL org.opencontainers.image.title="ocpp-interop-everest" \
      org.opencontainers.image.description="OCPP 1.6J, 2.0.1 and 2.1 reference station built on EVerest libocpp" \
      org.opencontainers.image.source="https://github.com/otfabric/ocpp-interop" \
      org.opencontainers.image.licenses="MIT AND Apache-2.0 AND BSD-3-Clause AND BSL-1.0" \
      org.opencontainers.image.version="${ADAPTER_VERSION}"

USER interop
EXPOSE 9000

HEALTHCHECK --interval=2s --timeout=2s --start-period=10s --retries=15 \
  CMD test -f /run/ocpp-interop/ready

ENTRYPOINT ["/usr/local/bin/ocpp-interop"]
CMD ["server", "--version", "1.6"]
