// SPDX-License-Identifier: MIT
//! `server`: a CSMS, ocpp-kit's transport::Csms for one OCPP version.

use std::collections::HashSet;
use std::net::SocketAddr;
use std::sync::{Arc, Mutex};
use std::time::Duration;

use ocpp_kit::Version;
use ocpp_kit::engine::{EngineConfig, HeartbeatPolicy, Role};
use ocpp_kit::transport::{Auth, AuthOutcome, Credentials, Csms, SessionEvent, constant_time_eq};
use ocpp_kit::types::Identity;
use serde_json::{Value, json};
use tokio::net::TcpListener;
use tokio::signal::unix::{SignalKind, signal};

use crate::control::{self, ControlHandler};
use crate::endpoint::{Endpoint, FixtureHandler};
use crate::fixture::Fixture;
use crate::out::{ResultDoc, event};
use crate::{ADAPTER, tls};

pub struct ServerOptions {
    pub version: String,
    pub bind: String,
    pub port: u16,
    pub profile: u8,
    pub tls_dir: Option<String>,
    pub ready_file: String,
    pub control_socket: String,
}

pub fn engine_config(role: Role, version: Version) -> EngineConfig {
    // Every CALL has the deadline of its --timeout-ms; the contract has no
    // CALL on the wire the test did not ask for (no automatic Heartbeat).
    let mut config = EngineConfig::new(role, version)
        .with_call_timeout(Duration::from_secs(3600))
        .with_heartbeat(HeartbeatPolicy::Manual);
    // A CSMS answers whatever a station sends, BootNotification or not
    // (B02.FR.09 enforcement off): the tests send a CALL before a boot.
    if role == Role::Csms {
        config.enforce_boot_gate = false;
    }
    config
}

pub async fn run(opts: ServerOptions, fixture: Arc<Fixture>) -> i32 {
    let version: Version = opts.version.parse().expect("checked");
    let endpoint = Endpoint::new(&opts.version, fixture.clone(), "csms", true);
    let profile = opts.profile;

    let auth_fixture = fixture.clone();
    let authenticate = move |auth: Auth| {
        let fixture = auth_fixture.clone();
        async move {
            let id = auth.identity.as_str().to_string();
            if !fixture.stations.contains_key(&id) {
                return AuthOutcome::Unknown; // HTTP 404
            }
            match (profile, &auth.credentials) {
                (1 | 2, Credentials::Basic { password, .. }) => {
                    if !constant_time_eq(password, fixture.password(&id).as_bytes()) {
                        return AuthOutcome::Reject; // HTTP 401
                    }
                }
                (1 | 2, _) => return AuthOutcome::Reject,
                (3, Credentials::ClientCertificate { der }) => {
                    // The handshake verified the chain against ca.crt; its
                    // subject must name this station.
                    if tls::common_name(der).as_deref() != Some(id.as_str()) {
                        return AuthOutcome::Reject;
                    }
                }
                (3, _) => return AuthOutcome::Reject,
                _ => {}
            }
            AuthOutcome::accept()
        }
    };

    let addr: SocketAddr = match format!("{}:{}", opts.bind, opts.port).parse() {
        Ok(a) => a,
        Err(e) => {
            eprintln!("server: {e}");
            return 2;
        }
    };
    let mut builder = Csms::builder()
        .bind(addr)
        .versions([version])
        .authenticate(authenticate)
        .handler(FixtureHandler(endpoint.clone()))
        // A CSMS accepts permessage-deflate whenever a station offers it.
        .compression(true)
        .engine_config(engine_config(Role::Csms, version));
    if profile >= 2 {
        match tls::server_tls(opts.tls_dir.as_deref().unwrap_or(""), profile) {
            Ok(t) => builder = builder.tls(t),
            Err(e) => {
                eprintln!("server: {e}");
                return 2;
            }
        }
    }
    let csms = match builder.build() {
        Ok(c) => c,
        Err(e) => {
            eprintln!("server: {e}");
            return 1;
        }
    };
    let listener = match TcpListener::bind(addr).await {
        Ok(l) => l,
        Err(e) => {
            eprintln!(
                "server: cannot listen on {} port {}: {e}",
                opts.bind, opts.port
            );
            return 3;
        }
    };
    let handle = csms.handle();
    let open: Arc<Mutex<HashSet<String>>> = Arc::new(Mutex::new(HashSet::new()));

    // Session events: connection-opened, -closed, -rejected.
    let mut events = handle.events();
    let open_events = open.clone();
    let subprotocol = format!("ocpp{version}");
    let sessions = handle.clone();
    let watcher = endpoint.clone();
    tokio::spawn(async move {
        while let Ok(e) = events.recv().await {
            match e {
                SessionEvent::Opened {
                    identity, remote, ..
                } => {
                    open_events
                        .lock()
                        .unwrap_or_else(|p| p.into_inner())
                        .insert(identity.as_str().to_string());
                    if let Some(session) = sessions.session(&identity).await {
                        watcher.watch(identity.as_str().to_string(), &session);
                    }
                    // ocpp-kit does not say whether permessage-deflate was negotiated.
                    event(
                        "connection-opened",
                        Some(identity.as_str()),
                        vec![
                            ("subprotocol", json!(subprotocol)),
                            ("remote", json!(remote.to_string())),
                            ("compression", Value::Null),
                        ],
                    );
                }
                SessionEvent::Closed { identity, .. } => {
                    if open_events
                        .lock()
                        .unwrap_or_else(|p| p.into_inner())
                        .remove(identity.as_str())
                    {
                        // ocpp-kit reports the reason for logs only, not the close frame.
                        event(
                            "connection-closed",
                            Some(identity.as_str()),
                            vec![("code", Value::Null), ("reason", Value::Null)],
                        );
                    }
                }
                SessionEvent::Refused {
                    identity,
                    status,
                    remote,
                } => {
                    let reason = match status {
                        404 => "unknown-station",
                        401 => "authentication",
                        _ => "subprotocol",
                    };
                    let station = identity.map(|i| i.as_str().to_string()).unwrap_or_default();
                    event(
                        "connection-rejected",
                        Some(&station),
                        vec![
                            ("reason", json!(reason)),
                            ("remote", json!(remote.to_string())),
                        ],
                    );
                }
                _ => {}
            }
        }
    });

    let served = tokio::spawn(csms.serve_on(listener));

    let control_handle = handle.clone();
    let control_endpoint = endpoint.clone();
    let version_name = opts.version.clone();
    let handler: ControlHandler = Arc::new(move |request| {
        let handle = control_handle.clone();
        let endpoint = control_endpoint.clone();
        let version = version_name.clone();
        Box::pin(async move {
            let id = request.station.clone().unwrap_or_default();
            let session = match Identity::new(id.clone()) {
                Ok(identity) => handle.session(&identity).await,
                Err(_) => None,
            };
            match session {
                Some(session) => {
                    endpoint
                        .execute(
                            &id,
                            &request.action,
                            request.payload,
                            request.timeout_ms,
                            Some(session),
                        )
                        .await
                }
                None => ResultDoc::failed(
                    &version,
                    &request.action,
                    request.payload,
                    "not-connected",
                    format!("no open connection for station {id:?}"),
                ),
            }
        })
    });
    let control = match control::serve(&opts.control_socket, ADAPTER, handler) {
        Ok(c) => c,
        Err(e) => {
            eprintln!(
                "server: cannot open the control socket {}: {e}",
                opts.control_socket
            );
            return 3;
        }
    };

    if let Err(e) = std::fs::write(&opts.ready_file, "ready\n") {
        eprintln!("server: cannot write {}: {e}", opts.ready_file);
        return 1;
    }
    event(
        "ready",
        None,
        vec![
            ("adapter", json!(ADAPTER)),
            ("version", json!(opts.version)),
            ("port", json!(opts.port)),
            ("fixture", json!(fixture.name)),
            ("securityProfile", json!(profile)),
        ],
    );

    let mut term = signal(SignalKind::terminate()).expect("signal handler");
    let mut int = signal(SignalKind::interrupt()).expect("signal handler");
    let mut code = 0;
    tokio::select! {
        _ = term.recv() => {}
        _ = int.recv() => {}
        r = served => {
            eprintln!("server: {r:?}");
            code = 1;
        }
    }
    let _ = std::fs::remove_file(&opts.ready_file);
    control.abort();
    handle.shutdown(Duration::from_secs(5)).await;
    // Let the sessions report their close.
    for _ in 0..250 {
        if open.lock().unwrap_or_else(|p| p.into_inner()).is_empty() {
            break;
        }
        tokio::time::sleep(Duration::from_millis(20)).await;
    }
    event("stopped", None, vec![]);
    code
}
