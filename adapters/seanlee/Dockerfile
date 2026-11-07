# SPDX-License-Identifier: MIT
#
# ocpp-interop adapter for seanlee0923/ocpp (Go).
#
# Build from the repository root:
#   docker build -f adapters/seanlee/Dockerfile -t ocpp-interop-seanlee:dev .
#
# seanlee0923/ocpp is MIT. The adapter (MIT) links it and its dependencies statically;
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
ARG GOLANG_DIGEST=sha256:47667934874913a449b96faa7ef4777fa285d9ec29126ee80ed61c45b7cccf0e
ARG DEBIAN_DIGEST=sha256:a72b4433de1cf7d6bae0c4cee74d0547f58601a643b3375f070c51600d63b6b2

# ---------------------------------------------------------------------------
# Stage 1: build the adapter, seanlee0923/ocpp linked in
# ---------------------------------------------------------------------------
FROM ${BASE_REGISTRY}/golang:1.27.2-trixie@${GOLANG_DIGEST} AS build

# The release pin: tag v0.3.0 of seanlee0923/ocpp, which is the Go module
# version, and its commit. A candidate build ("make candidate-seanlee
# CANDIDATE_VERSION=... CANDIDATE_SHA=...") overrides both.
ARG UPSTREAM_VERSION=v0.3.0
ARG UPSTREAM_SHA=7e3759d106605a11e4e3f1ce45fc44e6f595cd3c
ARG ADAPTER_VERSION=dev
ARG GOPROXY=https://proxy.golang.org

ENV CGO_ENABLED=0 GOPROXY=${GOPROXY} GOSUMDB=sum.golang.org GOFLAGS=-mod=readonly GOTOOLCHAIN=local
WORKDIR /src
COPY adapters/seanlee/go.mod adapters/seanlee/go.sum ./

# At the pin, every module must match go.sum. A candidate moves seanlee0923/ocpp (and
# what its go.mod requires) to UPSTREAM_VERSION; go.sum is then checked
# against the checksum database instead. Either way the commit the module
# proxy records for the version (Origin.Hash) must be UPSTREAM_SHA.
RUN set -eu; \
    echo "${UPSTREAM_SHA}" | grep -Eq '^[0-9a-f]{40}$'; \
    pinned="$(go list -m -f '{{.Version}}' github.com/seanlee0923/ocpp)"; \
    echo "${pinned}" > /tmp/pinned; \
    if [ "${UPSTREAM_VERSION}" = "${pinned}" ]; then \
        go mod download && go mod verify; \
    else \
        echo "candidate build: seanlee0923/ocpp ${UPSTREAM_VERSION} instead of the pin ${pinned}" >&2; \
        GOFLAGS=-mod=mod go get "github.com/seanlee0923/ocpp@${UPSTREAM_VERSION}"; \
        GOFLAGS=-mod=mod go mod tidy; \
    fi; \
    origin="$(go mod download -json "github.com/seanlee0923/ocpp@${UPSTREAM_VERSION}" \
        | sed -nE 's/^[[:space:]]*"Hash": "([0-9a-f]{40})".*/\1/p')"; \
    if [ -n "${origin}" ] && [ "${origin}" != "${UPSTREAM_SHA}" ]; then \
        echo "seanlee0923/ocpp ${UPSTREAM_VERSION} is commit ${origin}, not ${UPSTREAM_SHA}" >&2; exit 1; \
    fi

COPY adapters/seanlee/*.go ./
COPY adapters/seanlee/gen ./gen
# actions_gen.go instantiates the library's generic calls and handlers for
# every action it has types for (go run ./gen). At the pin the committed
# file must be what the generator writes; a candidate gets a fresh one.
RUN set -eu; \
    cp actions_gen.go /tmp/actions_gen.go; \
    go run ./gen; \
    if [ "${UPSTREAM_VERSION}" = "$(cat /tmp/pinned)" ]; then cmp actions_gen.go /tmp/actions_gen.go; fi; \
    test -z "$(gofmt -l .)"; \
    go vet ./...; \
    version="$(go list -m -f '{{.Version}}' github.com/seanlee0923/ocpp)"; \
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

LABEL org.opencontainers.image.title="ocpp-interop-seanlee" \
      org.opencontainers.image.description="OCPP 1.6J, 2.0.1 and 2.1 reference CSMS and station built on seanlee0923/ocpp" \
      org.opencontainers.image.source="https://github.com/otfabric/ocpp-interop" \
      org.opencontainers.image.licenses="MIT AND BSD-2-Clause AND BSD-3-Clause" \
      org.opencontainers.image.version="${ADAPTER_VERSION}"

USER interop
EXPOSE 9000

HEALTHCHECK --interval=2s --timeout=2s --start-period=10s --retries=15 \
  CMD test -f /run/ocpp-interop/ready

ENTRYPOINT ["/usr/local/bin/ocpp-interop"]
CMD ["server", "--version", "1.6"]
