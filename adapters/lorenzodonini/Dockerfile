# SPDX-License-Identifier: MIT
#
# ocpp-interop adapter for lorenzodonini/ocpp-go (Go).
#
# Build from the repository root:
#   docker build -f adapters/lorenzodonini/Dockerfile -t ocpp-interop-lorenzodonini:dev .
#
# ocpp-go is MIT. The adapter (MIT) links it and its dependencies statically;
# go.mod pins every module and go.sum their checksums. The image carries the
# licence of every module in the binary under /licenses. The pin is
# UPSTREAM_VERSION below (the git tag, which is the Go module version) and
# UPSTREAM_SHA (its commit), mirrored in go.mod and versions.yaml.

# Where the base images come from. They are pinned by digest, so a mirror of
# Docker Hub serves exactly the same images; CI uses one to stay clear of
# Docker Hub's pull rate limit (see the Makefile).
ARG BASE_REGISTRY=docker.io/library
# golang:1.27.2-trixie and debian:trixie-slim, multi-arch indexes
# (linux/amd64 and linux/arm64).
ARG GOLANG_DIGEST=sha256:e58d6f83b3416618d8bcac2b3dde1b7f7e3c4a77d25e88637f8bbae81536c48d
ARG DEBIAN_DIGEST=sha256:a29215f6a35e51e22adffa17f89e9d2ef06214e64a2bad10d765c46aea49f11f

# ---------------------------------------------------------------------------
# Stage 1: build the adapter, ocpp-go linked in
# ---------------------------------------------------------------------------
FROM ${BASE_REGISTRY}/golang:1.27.2-trixie@${GOLANG_DIGEST} AS build

# The release pin: tag v0.19.0 of lorenzodonini/ocpp-go, which is the Go
# module version, and its commit. A candidate build ("make candidate-lorenzodonini
# CANDIDATE_VERSION=... CANDIDATE_SHA=...") overrides both.
ARG UPSTREAM_VERSION=v0.19.0
ARG UPSTREAM_SHA=a1eec917af884db0585752f909a61893aa667480
ARG ADAPTER_VERSION=dev
ARG GOPROXY=https://proxy.golang.org

ENV CGO_ENABLED=0 GOPROXY=${GOPROXY} GOSUMDB=sum.golang.org GOFLAGS=-mod=readonly GOTOOLCHAIN=local
WORKDIR /src
COPY adapters/lorenzodonini/go.mod adapters/lorenzodonini/go.sum ./

# At the pin, every module must match go.sum. A candidate moves ocpp-go (and
# what its go.mod requires) to UPSTREAM_VERSION; go.sum is then checked
# against the checksum database instead. Either way the commit the module
# proxy records for the version (Origin.Hash) must be UPSTREAM_SHA.
RUN set -eu; \
    echo "${UPSTREAM_SHA}" | grep -Eq '^[0-9a-f]{40}$'; \
    pinned="$(go list -m -f '{{.Version}}' github.com/lorenzodonini/ocpp-go)"; \
    if [ "${UPSTREAM_VERSION}" = "${pinned}" ]; then \
        go mod download && go mod verify; \
    else \
        echo "candidate build: ocpp-go ${UPSTREAM_VERSION} instead of the pin ${pinned}" >&2; \
        GOFLAGS=-mod=mod go get "github.com/lorenzodonini/ocpp-go@${UPSTREAM_VERSION}"; \
        GOFLAGS=-mod=mod go mod tidy; \
    fi; \
    origin="$(go mod download -json "github.com/lorenzodonini/ocpp-go@${UPSTREAM_VERSION}" \
        | sed -nE 's/^[[:space:]]*"Hash": "([0-9a-f]{40})".*/\1/p')"; \
    if [ -n "${origin}" ] && [ "${origin}" != "${UPSTREAM_SHA}" ]; then \
        echo "ocpp-go ${UPSTREAM_VERSION} is commit ${origin}, not ${UPSTREAM_SHA}" >&2; exit 1; \
    fi

COPY adapters/lorenzodonini/*.go ./
COPY adapters/lorenzodonini/licenses /licenses-extra
RUN set -eu; \
    test -z "$(gofmt -l .)"; \
    go vet ./...; \
    version="$(go list -m -f '{{.Version}}' github.com/lorenzodonini/ocpp-go)"; \
    test "${version}" = "${UPSTREAM_VERSION}"; \
    go build -trimpath -ldflags "-s -w \
        -X main.adapterVersion=${ADAPTER_VERSION} \
        -X main.upstreamVersion=${version} \
        -X main.upstreamRevision=${UPSTREAM_SHA}" \
        -o /out/ocpp-interop .; \
    mkdir -p /out/licenses/go; \
    cp "$(go env GOROOT)/LICENSE" /out/licenses/go/LICENSE; \
    go version -m /out/ocpp-interop | awk '$1 == "dep" { print $2, $3 }' > /out/licenses/BUNDLED-MODULES.txt; \
    test -s /out/licenses/BUNDLED-MODULES.txt; \
    while read -r module modversion; do \
        dir="$(go list -m -f '{{.Dir}}' "${module}")"; \
        mkdir -p "/out/licenses/${module}"; \
        for file in "${dir}"/LICENSE* "${dir}"/LICENCE* "${dir}"/COPYING* "${dir}"/NOTICE*; do \
            if [ -f "${file}" ]; then cp "${file}" "/out/licenses/${module}/"; fi; \
        done; \
        if [ -z "$(ls "/out/licenses/${module}")" ] && [ -d "/licenses-extra/${module}" ]; then \
            cp "/licenses-extra/${module}"/* "/out/licenses/${module}/"; \
        fi; \
        test -n "$(ls "/out/licenses/${module}")" || { echo "no licence file in ${module} ${modversion}" >&2; exit 1; }; \
    done < /out/licenses/BUNDLED-MODULES.txt

# ---------------------------------------------------------------------------
# Stage 2: runtime
# ---------------------------------------------------------------------------
FROM ${BASE_REGISTRY}/debian:trixie-slim@${DEBIAN_DIGEST} AS runtime

ARG ADAPTER_VERSION=dev

RUN useradd --uid 10001 --no-create-home --shell /usr/sbin/nologin interop \
    && mkdir -p /run/ocpp-interop \
    && chown interop /run/ocpp-interop

COPY --from=build /out/ocpp-interop /usr/local/bin/ocpp-interop
COPY --from=build /out/licenses /licenses
COPY LICENSE /licenses/ocpp-interop/LICENSE
COPY THIRD_PARTY_NOTICES.md /licenses/THIRD_PARTY_NOTICES.md
COPY fixtures /fixtures

LABEL org.opencontainers.image.title="ocpp-interop-lorenzodonini" \
      org.opencontainers.image.description="OCPP 1.6J and 2.0.1 reference CSMS and station built on lorenzodonini/ocpp-go" \
      org.opencontainers.image.source="https://github.com/otfabric/ocpp-interop" \
      org.opencontainers.image.licenses="MIT AND BSD-2-Clause AND BSD-3-Clause" \
      org.opencontainers.image.version="${ADAPTER_VERSION}"

USER interop
EXPOSE 9000

HEALTHCHECK --interval=2s --timeout=2s --start-period=10s --retries=15 \
  CMD test -f /run/ocpp-interop/ready

ENTRYPOINT ["/usr/local/bin/ocpp-interop"]
CMD ["server", "--version", "1.6"]
