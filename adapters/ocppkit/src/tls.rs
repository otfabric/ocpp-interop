// SPDX-License-Identifier: MIT
//! Security profiles 2 and 3 (docs/CONTAINER_CONTRACT.md, Transport
//! security): ocpp-kit's ServerTls and ClientTls (rustls) from the PEM files
//! of --tls-dir. The adapter only names the files; the handshake and the
//! verification of the chain and the host name are rustls's.

use std::path::{Path, PathBuf};

use ocpp_kit::transport::{ClientTls, ServerTls};

fn file(dir: &str, name: &str) -> Result<PathBuf, String> {
    let path = Path::new(dir).join(name);
    if path.is_file() {
        Ok(path)
    } else {
        Err(format!("--tls-dir {dir}: {name} is missing"))
    }
}

pub fn server_tls(dir: &str, profile: u8) -> Result<ServerTls, String> {
    let cert = file(dir, "server.crt")?;
    let key = file(dir, "server.key")?;
    let tls = if profile == 3 {
        let ca = file(dir, "ca.crt")?;
        ServerTls::with_client_auth(&cert, &key, &ca)
    } else {
        ServerTls::new(&cert, &key)
    };
    tls.map_err(|e| format!("--tls-dir {dir}: {e}"))
}

pub fn client_tls(dir: &str, profile: u8) -> Result<ClientTls, String> {
    let ca = file(dir, "ca.crt")?;
    let mut builder = ClientTls::builder()
        .root_file(&ca)
        .map_err(|e| format!("--tls-dir {dir}: ca.crt: {e}"))?;
    if profile == 3 {
        let cert = file(dir, "client.crt")?;
        let key = file(dir, "client.key")?;
        builder = builder
            .client_certificate(&cert, &key)
            .map_err(|e| format!("--tls-dir {dir}: client.crt, client.key: {e}"))?;
    }
    builder.build().map_err(|e| format!("--tls-dir {dir}: {e}"))
}

/// The CommonName of a DER certificate (the identity of security profile 3).
pub fn common_name(der: &[u8]) -> Option<String> {
    let (_, cert) = x509_parser::parse_x509_certificate(der).ok()?;
    let cn = cert
        .subject()
        .iter_common_name()
        .next()?
        .as_str()
        .ok()?
        .to_string();
    Some(cn)
}
