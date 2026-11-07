// SPDX-License-Identifier: MIT
//! `local-controller`: ocpp-kit's `transport::LocalController` (OCPP 2.x
//! Part 4 chapter 6) between stations and one CSMS (contract 0.8). The
//! controller forwards every frame unchanged; its `Relay` writes a `relayed`
//! event for each one. It has no hook for a leg that opens or closes.

use std::net::SocketAddr;
use std::time::Duration;

use ocpp_kit::Version;
use ocpp_kit::rpc::Frame;
use ocpp_kit::transport::{Direction, LocalController, Relay, RelayDecision, UpstreamCredentials};
use ocpp_kit::types::Identity;
use serde_json::{Value, json};
use tokio::net::TcpListener;
use tokio::signal::unix::{SignalKind, signal};

use crate::out::event;
use crate::{ADAPTER, tls};

pub struct LcOptions {
    pub version: String,
    pub upstream: String,
    pub bind: String,
    pub port: u16,
    pub profile: u8,
    pub tls_dir: Option<String>,
    pub upstream_tls_dir: Option<String>,
    pub ready_file: String,
}

/// Writes a `relayed` event for every frame and forwards it.
struct Report;

impl Relay for Report {
    fn inspect(
        &self,
        identity: &Identity,
        direction: Direction,
        frame: &Frame<'_>,
    ) -> RelayDecision {
        let (kind, action) = match frame {
            Frame::Call { action, .. } => ("CALL", Some(action.to_string())),
            Frame::CallResult { .. } => ("CALLRESULT", None),
            Frame::CallError { .. } => ("CALLERROR", None),
            Frame::CallResultError { .. } => ("CALLRESULTERROR", None),
            Frame::Send { action, .. } => ("SEND", Some(action.to_string())),
            #[allow(unreachable_patterns)]
            _ => ("other", None),
        };
        event(
            "relayed",
            Some(identity.as_str()),
            vec![
                (
                    "direction",
                    json!(match direction {
                        Direction::Northbound => "northbound",
                        Direction::Southbound => "southbound",
                    }),
                ),
                ("messageType", json!(kind)),
                ("messageId", json!(frame.id().to_string())),
                ("action", action.map_or(Value::Null, Value::String)),
            ],
        );
        RelayDecision::Forward
    }
}

pub async fn run(opts: LcOptions) -> i32 {
    let version: Version = opts.version.parse().expect("checked");
    let addr: SocketAddr = match format!("{}:{}", opts.bind, opts.port).parse() {
        Ok(a) => a,
        Err(e) => {
            eprintln!("local-controller: {e}");
            return 2;
        }
    };
    let mut builder = LocalController::builder()
        .bind(addr)
        .upstream(opts.upstream.clone())
        .versions([version])
        .relay(Report);
    if let Some(dir) = &opts.tls_dir {
        match tls::server_tls(dir, opts.profile) {
            Ok(t) => builder = builder.server_tls(t),
            Err(e) => {
                eprintln!("local-controller: {e}");
                return 2;
            }
        }
    }
    if let Some(dir) = &opts.upstream_tls_dir {
        // Northbound the controller is the TLS client; with profile 3 it
        // presents the client certificate of --upstream-tls-dir and sends no
        // Authorization header (§6.5).
        match tls::client_tls(dir, opts.profile) {
            Ok(t) => builder = builder.client_tls(t),
            Err(e) => {
                eprintln!("local-controller: {e}");
                return 2;
            }
        }
    }
    if opts.profile == 3 {
        builder = builder.upstream_credentials(UpstreamCredentials::None);
    }
    let controller = match builder.build() {
        Ok(c) => c,
        Err(e) => {
            eprintln!("local-controller: {e}");
            return 2;
        }
    };
    let listener = match TcpListener::bind(addr).await {
        Ok(l) => l,
        Err(e) => {
            eprintln!("local-controller: cannot listen on {addr}: {e}");
            return 3;
        }
    };
    let served = tokio::spawn(controller.serve_on(listener));
    if let Err(e) = std::fs::write(&opts.ready_file, "ready\n") {
        eprintln!("local-controller: cannot write {}: {e}", opts.ready_file);
        return 1;
    }
    event(
        "ready",
        None,
        vec![
            ("adapter", json!(ADAPTER)),
            ("version", json!(opts.version)),
            ("port", json!(opts.port)),
            ("upstream", json!(opts.upstream)),
            ("securityProfile", json!(opts.profile)),
        ],
    );
    let mut term = signal(SignalKind::terminate()).expect("signal handler");
    let mut int = signal(SignalKind::interrupt()).expect("signal handler");
    let mut code = 0;
    tokio::select! {
        _ = term.recv() => {}
        _ = int.recv() => {}
        r = served => {
            eprintln!("local-controller: {r:?}");
            code = 1;
        }
    }
    let _ = std::fs::remove_file(&opts.ready_file);
    tokio::time::sleep(Duration::from_millis(50)).await;
    event("stopped", None, vec![]);
    code
}
