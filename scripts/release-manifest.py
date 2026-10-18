#!/usr/bin/env python3
# SPDX-License-Identifier: MIT
"""Writes manifest.json and the body of the GitHub release.

    scripts/release-manifest.py <version> <commit> <digests file> <manifest out> <body out>

The digests file holds one "<adapter> <sha256:...>" line per published
multi-arch image (make digests); the release workflow refuses an empty one.
Upstream pins are read from the adapter Dockerfiles, the contract and fixture
versions from versions.yaml.
"""

import json
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
REGISTRY = "ghcr.io/otfabric/ocpp-interop-"


def arg(adapter, name):
    text = (ROOT / "adapters" / adapter / "Dockerfile").read_text()
    match = re.search(rf"^ARG {name}=(.+)$", text, re.M)
    return match.group(1).strip() if match else ""


def manifest_value(key):
    match = re.search(rf'^{key}: "([^"]+)"', (ROOT / "versions.yaml").read_text(), re.M)
    if not match:
        raise SystemExit(f"versions.yaml has no {key}")
    return match.group(1)


def main():
    if len(sys.argv) != 6:
        raise SystemExit(__doc__)
    version, commit, digests_file, manifest_out, body_out = sys.argv[1:]
    if not re.fullmatch(r"v\d+\.\d+\.\d+", version):
        raise SystemExit(f"{version!r} is not a vX.Y.Z version")
    if manifest_value("interop_version") != version[1:]:
        raise SystemExit(f"versions.yaml interop_version is not {version[1:]}")
    digests = dict(line.split() for line in Path(digests_file).read_text().splitlines() if line.strip())

    upstream, images = {}, {}
    for adapter, digest in digests.items():
        if not re.fullmatch(r"sha256:[0-9a-f]{64}", digest):
            raise SystemExit(f"{adapter}: {digest!r} is not a digest")
        pin = {"version": arg(adapter, "UPSTREAM_VERSION")}
        if not pin["version"]:
            raise SystemExit(f"adapters/{adapter}/Dockerfile has no ARG UPSTREAM_VERSION")
        if revision := arg(adapter, "UPSTREAM_SHA"):
            pin["revision"] = revision
        upstream[adapter] = pin
        images[adapter] = f"{REGISTRY}{adapter}@{digest}"

    Path(manifest_out).write_text(json.dumps({
        "version": version,
        "commit": commit,
        "contract": manifest_value("contract_version"),
        "fixtureSchema": manifest_value("fixture_schema"),
        "platforms": ["linux/amd64", "linux/arm64"],
        "upstream": upstream,
        "images": images,
    }, indent=2) + "\n")

    if not images:
        body = [
            "## No images in this release",
            "",
            "This release publishes the container contract "
            f"(version {manifest_value('contract_version')}), the document schemas and the fixtures. "
            "No adapter is built yet, so there is nothing to pin. See `PLAN.md`.",
            "",
            "Notes: `RELEASE.md`.",
            "",
        ]
        Path(body_out).write_text("\n".join(body))
        return

    body = ["## Images", "", "| Image | Upstream | Digest |", "|-------|----------|--------|"]
    for adapter, digest in digests.items():
        body.append(f"| `{REGISTRY}{adapter}:{version}` | {upstream[adapter]['version']} | `{digest}` |")
    body += ["", "## Pinning in a consumer", "", "```bash"]
    for adapter, image in images.items():
        body.append(f"OCPP_INTEROP_{adapter.upper()}_IMAGE={image}")
    body += [
        "```",
        "",
        "`manifest.json` records the upstream versions these images were built from. "
        "The images contain third-party software under its own licences; see "
        "`THIRD_PARTY_NOTICES.md` and `/licenses` in each image. Notes: `RELEASE.md`.",
        "",
    ]
    Path(body_out).write_text("\n".join(body))


if __name__ == "__main__":
    main()
