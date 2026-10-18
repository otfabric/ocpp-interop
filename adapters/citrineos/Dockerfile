# SPDX-License-Identifier: MIT
#
# ocpp-interop adapter for CitrineOS (TypeScript), a CSMS only.
#
# Build from the repository root:
#   docker build -f adapters/citrineos/Dockerfile -t ocpp-interop-citrineos:dev .
#
# CitrineOS core is Apache-2.0. The pin is UPSTREAM_VERSION below (the release
# tag of citrineos/citrineos-core) and UPSTREAM_SHA (its commit), mirrored in
# versions.yaml. The build follows the upstream's own image recipe
# (apps/ocpp-server/deploy.Dockerfile of that tag): pnpm, at the version the
# release's package.json names, installs the workspace from its lockfile
# (every package by version and integrity hash), builds @citrineos/ocpp-server
# and deploys its production dependencies. CitrineOS needs PostgreSQL with
# PostGIS and RabbitMQ; the runtime image has both, from Debian bookworm, and
# runs them inside the container next to CitrineOS (adapters/citrineos/README.md).
# The adapter (MIT) is JavaScript run by the same Node.js.

# Where the base images come from. They are pinned by digest, so a mirror of
# Docker Hub serves exactly the same images (see the Makefile).
ARG BASE_REGISTRY=docker.io/library
# node:24.16.0-bookworm (the upstream's build image, node:24.16.0), multi-arch
# index (linux/amd64 and linux/arm64): the build stage. Not trixie: node-gyp
# 8.4.1, which the lockfile builds bufferutil and utf-8-validate with, fails
# with trixie's Python 3.13.
ARG NODE_DIGEST=sha256:40ad9f3064e67d6860b4bc3fe1880b2953934fd6320ada990e45fe0efa6badd7
# node:24.16.0-bookworm-slim (the upstream's runtime, node:24.16.0-slim),
# multi-arch index: the runtime.
ARG NODE_SLIM_DIGEST=sha256:2c87ef9bd3c6a3bd4b472b4bec2ce9d16354b0c574f736c476489d09f560a203

# ---------------------------------------------------------------------------
# Stage 1: CitrineOS from the pinned release
# ---------------------------------------------------------------------------
FROM ${BASE_REGISTRY}/node:24.16.0-bookworm@${NODE_DIGEST} AS upstream

# The release pin: citrineos-core release v2.0.1 and its commit. A candidate
# build ("make candidate-citrineos CANDIDATE_VERSION=... CANDIDATE_SHA=...")
# overrides both.
ARG UPSTREAM_VERSION=v2.0.1
ARG UPSTREAM_SHA=1a83638c31eebdb1dd8d6cdb0f5b8a9f142a1372
WORKDIR /src
RUN set -eu; \
    echo "${UPSTREAM_SHA}" | grep -Eq '^[0-9a-f]{40}$'; \
    git -c advice.detachedHead=false clone -q --depth 1 --branch "${UPSTREAM_VERSION}" \
        https://github.com/citrineos/citrineos-core citrineos-core; \
    got="$(git -C citrineos-core rev-parse HEAD)"; \
    [ "${got}" = "${UPSTREAM_SHA}" ] || { echo "citrineos-core ${UPSTREAM_VERSION} is ${got}, not ${UPSTREAM_SHA}" >&2; exit 1; }
WORKDIR /src/citrineos-core
RUN corepack enable && pnpm install --frozen-lockfile
RUN pnpm --filter "@citrineos/ocpp-server..." build \
    && pnpm --filter "@citrineos/ocpp-server" deploy --legacy --prod /deploy \
    && cp apps/ocpp-server/.sequelizerc /deploy/.sequelizerc

# The licences of everything deployed: CitrineOS (Apache-2.0: its LICENSE and
# the LICENSES directory of the release) and the licence files of every
# package in /deploy's node_modules (BUNDLED-PACKAGES.txt lists them all). A
# package without a licence file is listed, with the licence its package.json
# names, in WITHOUT-LICENSE-FILE.txt.
RUN set -eu; \
    mkdir -p /out/licenses/citrineos-core /out/licenses/node_modules; \
    cp LICENSE /out/licenses/citrineos-core/; \
    cp -r LICENSES /out/licenses/citrineos-core/LICENSES; \
    : > /out/licenses/node_modules/WITHOUT-LICENSE-FILE.txt; \
    cd /deploy/node_modules/.pnpm; \
    for dir in */node_modules/*/ */node_modules/@*/*/; do \
        [ -f "${dir}package.json" ] || continue; \
        pkg="${dir#*/node_modules/}"; pkg="${pkg%/}"; \
        case "${pkg}" in @citrineos/*) continue ;; esac; \
        version="$(node -p "require('./${dir}package.json').version")"; \
        grep -qxF "${pkg}@${version}" /tmp/seen 2>/dev/null && continue; \
        echo "${pkg}@${version}" >> /tmp/seen; \
        files="$(find "${dir}" -maxdepth 1 -type f \( -iname 'licen[cs]e*' -o -iname 'copying*' -o -iname 'notice*' \))"; \
        if [ -z "${files}" ]; then \
            echo "${pkg}@${version} $(node -p "JSON.stringify(require('./${dir}package.json').license ?? null)")" \
                >> /out/licenses/node_modules/WITHOUT-LICENSE-FILE.txt; \
            continue; \
        fi; \
        target="/out/licenses/node_modules/${pkg}@${version}"; \
        mkdir -p "${target}"; \
        for f in ${files}; do cp "${f}" "${target}/"; done; \
    done; \
    sort /tmp/seen > /out/licenses/BUNDLED-PACKAGES.txt; \
    rm /tmp/seen

# ---------------------------------------------------------------------------
# Stage 2: runtime
# ---------------------------------------------------------------------------
FROM ${BASE_REGISTRY}/node:24.16.0-bookworm-slim@${NODE_SLIM_DIGEST} AS runtime

ARG UPSTREAM_VERSION=v2.0.1
ARG UPSTREAM_SHA=1a83638c31eebdb1dd8d6cdb0f5b8a9f142a1372
ARG ADAPTER_VERSION=dev

# PostgreSQL 15 with PostGIS 3 and RabbitMQ, Debian bookworm's.
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        postgresql-15 postgresql-15-postgis-3 rabbitmq-server \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --uid 10001 --create-home --home-dir /var/lib/ocpp-interop --shell /usr/sbin/nologin interop \
    && mkdir -p /run/ocpp-interop \
    && chown interop /run/ocpp-interop \
    && mkdir -p /licenses/debian \
    && for pkg in postgresql-15 postgresql-15-postgis-3 rabbitmq-server erlang-base; do \
        cp "/usr/share/doc/${pkg}/copyright" "/licenses/debian/${pkg}.copyright"; done \
    && dpkg-query -W -f '${Package} ${Version}\n' postgresql-15 postgresql-15-postgis-3 rabbitmq-server erlang-base \
        > /licenses/debian/VERSIONS.txt

COPY --from=upstream /deploy /opt/citrineos
COPY --from=upstream /out/licenses /licenses
COPY adapters/citrineos/src /opt/citrineos/ocpp-interop
COPY fixtures /fixtures
COPY LICENSE /licenses/ocpp-interop/LICENSE
COPY THIRD_PARTY_NOTICES.md /licenses/THIRD_PARTY_NOTICES.md
RUN set -eu; \
    printf '{"adapterVersion":"%s","upstreamVersion":"%s","upstreamRevision":"%s"}\n' \
        "${ADAPTER_VERSION}" "${UPSTREAM_VERSION}" "${UPSTREAM_SHA}" > /opt/citrineos/ocpp-interop/build-info.json; \
    printf '%s\n' '#!/bin/sh' 'exec /usr/local/bin/node /opt/citrineos/ocpp-interop/main.mjs "$@"' \
        > /usr/local/bin/ocpp-interop; \
    chmod 0755 /usr/local/bin/ocpp-interop

# The database, created with PostGIS (as the postgis/postgis image of the
# upstream's compose file does) and migrated with CitrineOS's own migrations
# (as the upstream entrypoint does) at build time, so that every container starts
# from it. Its password is a public test value; it listens on 127.0.0.1 only.
USER interop
RUN set -eu; \
    pg=/usr/lib/postgresql/15/bin; data=/var/lib/ocpp-interop/postgres; sock=/tmp/pg-build; \
    mkdir -p "${sock}"; \
    "${pg}/initdb" -D "${data}" -U interop --auth=trust --encoding=UTF8 --locale=C.UTF-8 >/dev/null; \
    "${pg}/pg_ctl" -D "${data}" -o "-c listen_addresses=127.0.0.1 -c unix_socket_directories=${sock}" -w start >/dev/null; \
    psql -h "${sock}" -U interop -d postgres -v ON_ERROR_STOP=1 -q \
        -c "CREATE ROLE citrine LOGIN SUPERUSER PASSWORD 'citrine'" -c "CREATE DATABASE citrine OWNER citrine"; \
    psql -h "${sock}" -U citrine -d citrine -v ON_ERROR_STOP=1 -q -c "CREATE EXTENSION postgis"; \
    cd /opt/citrineos; \
    status=0; \
    CITRINEOS_DATABASE_HOST=127.0.0.1 ./node_modules/.bin/sequelize-cli db:migrate >/tmp/db.log 2>&1 \
        && CITRINEOS_DATABASE_HOST=127.0.0.1 node dist/scripts/provision-partitions.js >>/tmp/db.log 2>&1 || status=$?; \
    "${pg}/pg_ctl" -D "${data}" -m fast -w stop >/dev/null; \
    [ "${status}" = 0 ] || { cat /tmp/db.log >&2; exit 1; }; \
    rm -rf "${sock}" /tmp/db.log; \
    ocpp-interop version

LABEL org.opencontainers.image.title="ocpp-interop-citrineos" \
      org.opencontainers.image.description="OCPP 1.6J, 2.0.1 and 2.1 reference CSMS built on CitrineOS" \
      org.opencontainers.image.source="https://github.com/otfabric/ocpp-interop" \
      org.opencontainers.image.licenses="MIT AND Apache-2.0" \
      org.opencontainers.image.version="${ADAPTER_VERSION}"

EXPOSE 9000

HEALTHCHECK --interval=2s --timeout=2s --start-period=60s --retries=15 \
  CMD test -f /run/ocpp-interop/ready

ENTRYPOINT ["/usr/local/bin/ocpp-interop"]
CMD ["server", "--version", "1.6"]
