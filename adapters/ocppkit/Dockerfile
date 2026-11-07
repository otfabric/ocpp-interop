# SPDX-License-Identifier: MIT
#
# ocpp-interop adapter for hupe1980/ocpp-kit (Rust).
#
# Build from the repository root:
#   docker build -f adapters/ocppkit/Dockerfile -t ocpp-interop-ocppkit:dev .
#
# ocpp-kit is MIT OR Apache-2.0. The adapter (MIT) links it and its
# dependencies statically; Cargo.lock pins every crate by version and
# checksum. The image carries the licence of every crate in the binary under
# /licenses. The pin is UPSTREAM_VERSION below (the crates.io version, built
# from the git tag) and UPSTREAM_SHA (its commit, recorded by cargo publish in
# the crate's .cargo_vcs_info.json), mirrored in Cargo.toml, Cargo.lock and
# versions.yaml.

# Where the base images come from. They are pinned by digest, so a mirror of
# Docker Hub serves exactly the same images; CI uses one to stay clear of
# Docker Hub's pull rate limit (see the Makefile).
ARG BASE_REGISTRY=docker.io/library
# rust:1.99.1-slim-trixie and debian:trixie-slim, multi-arch indexes
# (linux/amd64 and linux/arm64).
ARG RUST_DIGEST=sha256:7eab7ce19273ce785e0ca7578bdf7bc1fcbae621775a8197379a04c9f6e03cd5
ARG DEBIAN_DIGEST=sha256:a72b4433de1cf7d6bae0c4cee74d0547f58601a643b3375f070c51600d63b6b2

# ---------------------------------------------------------------------------
# Stage 1: build the adapter, ocpp-kit linked in
# ---------------------------------------------------------------------------
FROM ${BASE_REGISTRY}/rust:1.99.1-slim-trixie@${RUST_DIGEST} AS build

# The release pin: ocpp-kit 0.3.0 from crates.io and the commit of its tag
# v0.3.0. A candidate build ("make candidate-ocppkit CANDIDATE_VERSION=...
# CANDIDATE_SHA=...") overrides both.
ARG UPSTREAM_VERSION=0.3.0
ARG UPSTREAM_SHA=b64ce83ab46c742dfbd99e925cd0acfacbf2ddc9
ARG ADAPTER_VERSION=dev

ENV CARGO_TERM_COLOR=never CARGO_NET_RETRY=5
WORKDIR /src
COPY adapters/ocppkit/Cargo.toml adapters/ocppkit/Cargo.lock ./

# At the pin, every crate must match Cargo.lock (--locked: cargo checks each
# download against its checksum). A candidate moves ocpp-kit to
# UPSTREAM_VERSION (Cargo.toml's exact requirement and the lock entry).
# Either way the commit cargo publish recorded for the crate must be
# UPSTREAM_SHA.
RUN set -eu; \
    echo "${UPSTREAM_SHA}" | grep -Eq '^[0-9a-f]{40}$'; \
    pinned="$(sed -nE 's/^ocpp-kit = \{ version = "=([^"]+)".*/\1/p' Cargo.toml)"; \
    test -n "${pinned}"; \
    mkdir -p src && echo 'fn main() {}' > src/main.rs; \
    if [ "${UPSTREAM_VERSION}" = "${pinned}" ]; then \
        cargo fetch --locked; \
    else \
        echo "candidate build: ocpp-kit ${UPSTREAM_VERSION} instead of the pin ${pinned}" >&2; \
        sed -i -E "s/^ocpp-kit = \{ version = \"=[^\"]+\"/ocpp-kit = { version = \"=${UPSTREAM_VERSION}\"/" Cargo.toml; \
        cargo update -p ocpp-kit --precise "${UPSTREAM_VERSION}"; \
        cargo fetch --locked; \
    fi; \
    vcs="$(ls -d "${CARGO_HOME}"/registry/src/*/"ocpp-kit-${UPSTREAM_VERSION}")/.cargo_vcs_info.json"; \
    origin="$(sed -nE 's/^[[:space:]]*"sha1": "([0-9a-f]{40})".*/\1/p' "${vcs}")"; \
    if [ "${origin}" != "${UPSTREAM_SHA}" ]; then \
        echo "ocpp-kit ${UPSTREAM_VERSION} is commit ${origin:-unknown}, not ${UPSTREAM_SHA}" >&2; exit 1; \
    fi

COPY adapters/ocppkit/src ./src
RUN set -eu; \
    touch src/main.rs; \
    rustup component add rustfmt clippy; \
    cargo fmt --check; \
    cargo clippy --release --locked --offline -- -D warnings; \
    version="$(cargo tree --locked --offline -e normal --depth 1 --prefix none -f '{p}' -i ocpp-kit | sed -nE 's/^ocpp-kit v([^ ]+).*/\1/p' | head -1)"; \
    test "${version}" = "${UPSTREAM_VERSION}"; \
    ADAPTER_VERSION="${ADAPTER_VERSION}" UPSTREAM_VERSION="${version}" UPSTREAM_REVISION="${UPSTREAM_SHA}" \
        cargo build --release --locked --offline; \
    mkdir -p /out/licenses/rust; \
    cp target/release/ocpp-interop /out/ocpp-interop; \
    cp "$(rustc --print sysroot)/share/doc/rust/LICENSE-MIT" /out/licenses/rust/ 2>/dev/null \
        || echo "Rust standard library: MIT OR Apache-2.0 (https://github.com/rust-lang/rust)" > /out/licenses/rust/LICENSE-NOTE; \
    # The crates linked into the binary: normal dependencies, not build
    # scripts or proc-macros, which run at build time only.
    cargo tree --locked --offline -e normal --prefix none -f '{p}|{l}' \
        | grep -v '(proc-macro)' | grep -v '^ocpp-interop-ocppkit ' | sed 's/ (\*)//' | sort -u \
        | sed -E 's/^([^ ]+) v([^ |]+)[^|]*\|(.*)$/\1 \2 \3/' > /out/licenses/BUNDLED-CRATES.txt; \
    test -s /out/licenses/BUNDLED-CRATES.txt; \
    while read -r crate crateversion license; do \
        dir="$(ls -d "${CARGO_HOME}"/registry/src/*/"${crate}-${crateversion}" | head -1)"; \
        mkdir -p "/out/licenses/${crate}-${crateversion}"; \
        for file in "${dir}"/LICENSE* "${dir}"/LICENCE* "${dir}"/COPYING* "${dir}"/NOTICE* "${dir}"/COPYRIGHT*; do \
            if [ -f "${file}" ]; then cp "${file}" "/out/licenses/${crate}-${crateversion}/"; fi; \
        done; \
        if [ -z "$(ls "/out/licenses/${crate}-${crateversion}")" ]; then \
            # A crate without a licence file: its declared licence and authors.
            { echo "${crate} ${crateversion}: ${license}"; \
              echo "The crate has no licence file; Cargo.toml declares the licence above."; \
              sed -nE '/^authors/,/\]/p' "${dir}/Cargo.toml"; } > "/out/licenses/${crate}-${crateversion}/LICENSE-DECLARED"; \
        fi; \
    done < /out/licenses/BUNDLED-CRATES.txt

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

LABEL org.opencontainers.image.title="ocpp-interop-ocppkit" \
      org.opencontainers.image.description="OCPP 1.6J, 2.0.1 and 2.1 reference CSMS and station built on hupe1980/ocpp-kit" \
      org.opencontainers.image.source="https://github.com/otfabric/ocpp-interop" \
      org.opencontainers.image.licenses="MIT AND Apache-2.0 AND ISC AND BSD-3-Clause AND Unicode-3.0 AND Zlib AND CDLA-Permissive-2.0" \
      org.opencontainers.image.version="${ADAPTER_VERSION}"

USER interop
EXPOSE 9000

HEALTHCHECK --interval=2s --timeout=2s --start-period=10s --retries=15 \
  CMD test -f /run/ocpp-interop/ready

ENTRYPOINT ["/usr/local/bin/ocpp-interop"]
CMD ["server", "--version", "1.6"]
