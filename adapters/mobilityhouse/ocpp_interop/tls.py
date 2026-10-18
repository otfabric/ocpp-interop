# SPDX-License-Identifier: MIT
"""Security profiles 2 and 3: the ssl contexts that websockets uses for
wss:// (docs/CONTAINER_CONTRACT.md, Transport security). The adapter only
loads the PEM files of --tls-dir into Python's ssl module; the TLS handshake,
the verification of the peer's certificate chain and of the server's host
name are Python's (OpenSSL)."""

from __future__ import annotations

import os
import ssl


class TlsError(Exception):
    """The files of --tls-dir are missing or not valid PEM (a usage error)."""


def _path(tls_dir: str, name: str) -> str:
    path = os.path.join(tls_dir, name)
    if not os.path.isfile(path):
        raise TlsError(f"--tls-dir {tls_dir}: {name} is missing")
    return path


def server_context(tls_dir: str, profile: int) -> ssl.SSLContext:
    """TLS 1.2 or later with server.crt and server.key; profile 3 requires a
    client certificate that verifies against ca.crt."""
    context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
    context.minimum_version = ssl.TLSVersion.TLSv1_2
    try:
        context.load_cert_chain(_path(tls_dir, "server.crt"), _path(tls_dir, "server.key"))
        if profile == 3:
            context.load_verify_locations(cafile=_path(tls_dir, "ca.crt"))
            context.verify_mode = ssl.CERT_REQUIRED
    except (ssl.SSLError, OSError, ValueError) as e:
        if isinstance(e, TlsError):
            raise
        raise TlsError(f"--tls-dir {tls_dir}: {e}") from e
    return context


def client_context(tls_dir: str, profile: int) -> ssl.SSLContext:
    """TLS 1.2 or later; trusts ca.crt only and checks the server's host name.
    Profile 3 presents client.crt with client.key."""
    try:
        context = ssl.create_default_context(ssl.Purpose.SERVER_AUTH, cafile=_path(tls_dir, "ca.crt"))
        context.minimum_version = ssl.TLSVersion.TLSv1_2
        if profile == 3:
            context.load_cert_chain(_path(tls_dir, "client.crt"), _path(tls_dir, "client.key"))
    except (ssl.SSLError, OSError, ValueError) as e:
        if isinstance(e, TlsError):
            raise
        raise TlsError(f"--tls-dir {tls_dir}: {e}") from e
    return context


def peer_common_name(connection) -> str | None:
    """The subject CommonName of the certificate the peer presented (verified
    by the handshake), or None."""
    transport = getattr(connection, "transport", None)
    ssl_object = transport.get_extra_info("ssl_object") if transport else None
    certificate = ssl_object.getpeercert() if ssl_object else None
    if not certificate:
        return None
    for rdn in certificate.get("subject", ()):
        for key, value in rdn:
            if key == "commonName":
                return value
    return None


def compression_negotiated(connection) -> bool:
    """Whether permessage-deflate was negotiated on this connection."""
    extensions = getattr(getattr(connection, "protocol", None), "extensions", None) or []
    return any(getattr(e, "name", "") == "permessage-deflate" for e in extensions)
