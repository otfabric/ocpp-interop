# SPDX-License-Identifier: MIT
# ocpp-interop adapter for The Mobility House ocpp (Python).
#
# Build from the repository root:
#   docker build -f adapters/mobilityhouse/Dockerfile -t ocpp-interop-mobilityhouse:dev .
#
# The ocpp package is MIT. It carries the OCA JSON schemas of each OCPP
# version (CC BY-ND 4.0), unmodified, and validates every payload against
# them. Its transport is the websockets package (BSD-3-Clause). The adapter
# is MIT like the rest of the repository. Pins are mirrored in versions.yaml.

# Where the base image comes from. It is pinned by digest, so a mirror of
# Docker Hub serves exactly the same image; CI uses one to stay clear of
# Docker Hub's pull rate limit (see the Makefile).
ARG BASE_REGISTRY=docker.io/library
# python:3.14.8-slim-trixie, multi-arch index (linux/amd64 and linux/arm64).
ARG PYTHON_DIGEST=sha256:a2b82f3c48559aa0a8446d9af49826b6e2b2016f4cd2afabfe6013ec53729170

# ---------------------------------------------------------------------------
# Stage 1: the ocpp package and its dependencies from PyPI, the adapter
# ---------------------------------------------------------------------------
FROM ${BASE_REGISTRY}/python:3.14.8-slim-trixie@${PYTHON_DIGEST} AS build

# The upstream release, and the commit of its git tag (the wheel on PyPI is
# that tree; see adapters/mobilityhouse/README.md). requirements.txt pins the
# same version, and every dependency, by hash. A candidate build overrides
# UPSTREAM_VERSION (and UPSTREAM_SHA): the hashes of another release cannot be
# known here, so it is installed from PyPI without hash checking, and the
# build fails if what is installed is not the requested version.
ARG UPSTREAM_VERSION=2.1.0
ARG UPSTREAM_SHA=91f4f89140a2198dc5a300cfea88d947ac91ae28
ARG ADAPTER_VERSION=dev
ARG PIP_INDEX_URL=https://pypi.org/simple

ENV PIP_DISABLE_PIP_VERSION_CHECK=1 \
    PIP_NO_CACHE_DIR=1 \
    PIP_ROOT_USER_ACTION=ignore \
    PIP_INDEX_URL=${PIP_INDEX_URL}

COPY adapters/mobilityhouse/requirements.txt /src/requirements.txt

RUN set -eu; \
    pinned="$(sed -nE 's/^ocpp==([^ \\]+).*/\1/p' /src/requirements.txt)"; \
    test -n "${pinned}"; \
    target=/opt/ocpp-interop/site-packages; \
    if [ "${UPSTREAM_VERSION}" = "${pinned}" ]; then \
        pip install --no-compile --only-binary :all: --require-hashes --no-deps --target "${target}" \
            -r /src/requirements.txt; \
    else \
        echo "candidate build: ocpp ${UPSTREAM_VERSION} instead of the pin ${pinned}, no hashes to check" >&2; \
        websockets="$(sed -nE 's/^websockets==([^ \\]+).*/\1/p' /src/requirements.txt)"; \
        pip install --no-compile --only-binary :all: --target "${target}" \
            "ocpp==${UPSTREAM_VERSION}" "websockets==${websockets}"; \
    fi; \
    installed="$(PYTHONPATH="${target}" python3 -c 'from importlib.metadata import version; print(version("ocpp"))')"; \
    test "${installed}" = "${UPSTREAM_VERSION}"; \
    echo "${UPSTREAM_SHA}" | grep -Eq '^[0-9a-f]{40}$'; \
    mkdir -p /licenses/python /licenses/ocpp-interop; \
    for info in "${target}"/*.dist-info; do \
        name="$(basename "${info}" | sed -E 's/-[^-]+\.dist-info$//')"; \
        mkdir -p "/licenses/${name}"; \
        if [ -d "${info}/licenses" ]; then cp -r "${info}/licenses/." "/licenses/${name}/"; \
        else cp "${info}"/LICENSE* "${info}"/COPYING* "/licenses/${name}/" 2>/dev/null || true; fi; \
        test -n "$(ls "/licenses/${name}")" || { echo "no licence file in ${info}" >&2; exit 1; }; \
    done; \
    cp /usr/local/lib/python3.14/LICENSE.txt /licenses/python/LICENSE.txt; \
    (cd "${target}" && ls -d *.dist-info) > /licenses/BUNDLED-PACKAGES.txt; \
    printf '{"adapterVersion":"%s","upstreamVersion":"%s","upstreamRevision":"%s"}\n' \
        "${ADAPTER_VERSION}" "${installed}" "${UPSTREAM_SHA}" > /opt/ocpp-interop/build-info.json

COPY adapters/mobilityhouse/ocpp_interop /opt/ocpp-interop/ocpp_interop
COPY adapters/mobilityhouse/ocpp-interop /opt/ocpp-interop/bin/ocpp-interop
COPY scripts/validate-fixtures.py /opt/ocpp-interop/validate_fixtures.py
COPY LICENSE /licenses/ocpp-interop/LICENSE
COPY THIRD_PARTY_NOTICES.md /licenses/THIRD_PARTY_NOTICES.md

# The runtime user cannot write byte code next to the sources: compile now.
RUN python3 -m compileall -q /opt/ocpp-interop \
    && chmod 0755 /opt/ocpp-interop/bin/ocpp-interop

# ---------------------------------------------------------------------------
# Stage 2: runtime
# ---------------------------------------------------------------------------
FROM ${BASE_REGISTRY}/python:3.14.8-slim-trixie@${PYTHON_DIGEST} AS runtime

ARG ADAPTER_VERSION=dev

RUN useradd --uid 10001 --no-create-home --shell /usr/sbin/nologin interop \
    && mkdir -p /run/ocpp-interop \
    && chown interop /run/ocpp-interop

COPY --from=build /opt/ocpp-interop /opt/ocpp-interop
COPY --from=build /opt/ocpp-interop/bin/ocpp-interop /usr/local/bin/ocpp-interop
COPY --from=build /licenses /licenses
COPY fixtures /fixtures

ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1

LABEL org.opencontainers.image.title="ocpp-interop-mobilityhouse" \
      org.opencontainers.image.description="OCPP 1.6J and 2.0.1 reference CSMS and station built on The Mobility House ocpp" \
      org.opencontainers.image.source="https://github.com/otfabric/ocpp-interop" \
      org.opencontainers.image.licenses="MIT AND BSD-3-Clause AND CC-BY-ND-4.0" \
      org.opencontainers.image.version="${ADAPTER_VERSION}"

USER interop
EXPOSE 9000

HEALTHCHECK --interval=2s --timeout=2s --start-period=10s --retries=15 \
  CMD test -f /run/ocpp-interop/ready

ENTRYPOINT ["/usr/local/bin/ocpp-interop"]
CMD ["server", "--version", "1.6"]
