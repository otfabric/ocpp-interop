# SPDX-License-Identifier: MIT
#
# ocpp-interop adapter for ChargeTimeEU Java-OCA-OCPP (Java).
#
# Build from the repository root:
#   docker build -f adapters/chargetime/Dockerfile -t ocpp-interop-chargetime:dev .
#
# Java-OCA-OCPP is MIT and is used as published on Maven Central
# (eu.chargetime.ocpp:ocpp16j and what its POM pulls in), unmodified: the image
# carries its jars next to the adapter's (MIT). The pin is UPSTREAM_VERSION
# below (the git tag) and UPSTREAM_SHA (its commit), mirrored in pom.xml
# (ocpp.version), dependencies.sha256 and versions.yaml.

# Where the base images come from. They are pinned by digest, so a mirror of
# Docker Hub serves exactly the same images.
ARG BASE_REGISTRY=docker.io/library
# maven:3.9.16-eclipse-temurin-25-noble and eclipse-temurin:25.0.4.1_1-jre-noble,
# multi-arch indexes (linux/amd64 and linux/arm64).
ARG MAVEN_DIGEST=sha256:93b8a14ea2f412782e4e842651273b4d903e35cc496284f178fbbe2d67d00976
ARG JRE_DIGEST=sha256:3f2223fe04b750aa5a8179e29b5c9a1ddf7665fc9bb08bfe11ad01d143bc13a5

# ---------------------------------------------------------------------------
# Stage 1: build the adapter jar and collect its runtime class path
# ---------------------------------------------------------------------------
FROM ${BASE_REGISTRY}/maven:3.9.16-eclipse-temurin-25-noble@${MAVEN_DIGEST} AS build

# The release pin: git tag v2.0 of ChargeTimeEU/Java-OCA-OCPP, published on
# Maven Central as version 2.0 (its -sources jars are that tree). A candidate
# build ("make candidate-chargetime CANDIDATE_VERSION=... CANDIDATE_SHA=...")
# overrides both.
ARG UPSTREAM_VERSION=v2.0
ARG UPSTREAM_SHA=f27fe5bfb03e01491100597d6859080f5ae07545
ARG ADAPTER_VERSION=dev

WORKDIR /build
COPY adapters/chargetime/pom.xml adapters/chargetime/dependencies.sha256 ./
COPY adapters/chargetime/src ./src
# -C: fail when a downloaded artifact does not match its checksum on Maven
# Central. The release pin is checked against dependencies.sha256 as well: the
# sha256 of every jar on the runtime class path. A candidate has no such list.
RUN set -eu; \
    maven_version="${UPSTREAM_VERSION#v}"; \
    echo "${UPSTREAM_SHA}" | grep -Eq '^[0-9a-f]{40}$'; \
    mvn -B -q -C -Docpp.version="${maven_version}" package -DskipTests; \
    test -f "target/lib/ocpp16j-${maven_version}.jar"; \
    if [ "${UPSTREAM_VERSION}" = "$(sed -nE 's/^# pin: (.+)$/\1/p' dependencies.sha256)" ]; then \
        (cd target/lib && sha256sum -c --strict ../../dependencies.sha256); \
        test "$(ls target/lib | wc -l)" = "$(grep -c '^[0-9a-f]' dependencies.sha256)"; \
    else \
        echo "candidate build: Java-OCA-OCPP ${UPSTREAM_VERSION}, no dependency hashes to check" >&2; \
        (cd target/lib && sha256sum *.jar) >&2; \
    fi; \
    printf '{"adapterVersion":"%s","upstreamVersion":"%s","upstreamRevision":"%s"}\n' \
        "${ADAPTER_VERSION}" "${UPSTREAM_VERSION}" "${UPSTREAM_SHA}" > /build/build-info.json

# The licence and notice files every bundled jar carries, per jar, and the
# list of what is bundled. Java-OCA-OCPP's jars carry none: its LICENSE comes
# from adapters/chargetime/licenses.
RUN set -eu; mkdir -p /licenses; \
    ls target/lib > /licenses/BUNDLED-JARS.txt; \
    for jar in target/lib/*.jar; do \
        name="$(basename "${jar}" .jar)"; \
        files="$(jar tf "${jar}" | grep -iE '^META-INF/(LICENSE|NOTICE|license/)[^/]*$' | grep -v '/$' || true)"; \
        [ -n "${files}" ] || continue; \
        mkdir -p "/licenses/${name}"; \
        (cd "/licenses/${name}" && echo "${files}" | xargs jar xf "/build/${jar}" \
            && mv META-INF/* . && rmdir META-INF); \
    done

# ---------------------------------------------------------------------------
# Stage 2: runtime (JRE only)
# ---------------------------------------------------------------------------
FROM ${BASE_REGISTRY}/eclipse-temurin:25.0.4.1_1-jre-noble@${JRE_DIGEST} AS runtime

ARG ADAPTER_VERSION=dev

# The JVM flags trade peak throughput for a short start: `client <Action>`
# and `call` are one short-lived process each. stdout is for JSON only: the
# JVM's own warnings (-Xlog) and the library's logging (slf4j-simple) go to
# stderr.
RUN useradd --uid 10001 --no-create-home --shell /usr/sbin/nologin interop \
    && mkdir -p /run/ocpp-interop /app \
    && chown interop /run/ocpp-interop \
    && printf '%s\n' '#!/bin/sh' \
       'exec java -Xlog:disable -Xlog:all=warning:stderr -XX:+UseSerialGC -XX:TieredStopAtLevel=1 -Xshare:auto -Dorg.slf4j.simpleLogger.defaultLogLevel="${OCPP_INTEROP_LOG_LEVEL:-warn}" -cp "/app/ocpp-interop-chargetime.jar:/app/lib/*" org.otfabric.ocppinterop.chargetime.Main "$@"' \
       > /usr/local/bin/ocpp-interop \
    && chmod +x /usr/local/bin/ocpp-interop

COPY --from=build /build/target/ocpp-interop-chargetime.jar /app/ocpp-interop-chargetime.jar
COPY --from=build /build/target/lib /app/lib
COPY --from=build /build/build-info.json /app/build-info.json
COPY --from=build /licenses /licenses
COPY adapters/chargetime/licenses /licenses
COPY LICENSE /licenses/ocpp-interop/LICENSE
COPY THIRD_PARTY_NOTICES.md /licenses/THIRD_PARTY_NOTICES.md
COPY fixtures /fixtures

LABEL org.opencontainers.image.title="ocpp-interop-chargetime" \
      org.opencontainers.image.description="OCPP 1.6J reference CSMS and station built on ChargeTimeEU Java-OCA-OCPP" \
      org.opencontainers.image.source="https://github.com/otfabric/ocpp-interop" \
      org.opencontainers.image.licenses="MIT AND Apache-2.0 AND CDDL-1.1" \
      org.opencontainers.image.version="${ADAPTER_VERSION}"

USER interop
EXPOSE 9000

HEALTHCHECK --interval=2s --timeout=2s --start-period=20s --retries=15 \
  CMD test -f /run/ocpp-interop/ready

ENTRYPOINT ["/usr/local/bin/ocpp-interop"]
CMD ["server", "--version", "1.6"]
