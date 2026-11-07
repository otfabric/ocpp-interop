// SPDX-License-Identifier: MIT
//! `client run` and `client <Action>`: a station, ocpp-kit's
//! transport::Station.

use std::sync::Arc;
use std::time::{Duration, Instant};

use ocpp_kit::Version;
use ocpp_kit::engine::Role;
use ocpp_kit::transport::{BasicAuthPassword, ClientTls, Event, Handle, SecurityProfile, Station};
use serde_json::{Value, json};
use tokio::signal::unix::{SignalKind, signal};

use crate::control::{self, ControlHandler};
use crate::endpoint::{Endpoint, FixtureHandler};
use crate::fixture::Fixture;
use crate::out::{ResultDoc, event};
use crate::server::engine_config;
use crate::{ADAPTER, tls};

pub struct ConnectOptions {
    pub version: String,
    pub base: String,
    pub station: String,
    pub profile: u8,
    pub tls: Option<ClientTls>,
    pub compression: bool,
    pub connect_timeout: Duration,
}

/// Opens the station's connection: one attempt, no reconnect.
pub async fn connect(
    opts: ConnectOptions,
    endpoint: Arc<Endpoint>,
    fixture: &Fixture,
) -> Result<Handle, String> {
    let version: Version = opts.version.parse().expect("checked");
    let mut builder = Station::builder()
        .identity(&opts.station)
        .map_err(|e| format!("identity {:?}: {e}", opts.station))?
        .url(opts.base.clone())
        .versions([version])
        .compression(opts.compression)
        .reconnect(false)
        .engine_config(engine_config(Role::ChargingStation, version))
        .handler(FixtureHandler(endpoint));
    match opts.profile {
        1 => builder = builder.security_profile(SecurityProfile::BasicAuth),
        2 => builder = builder.security_profile(SecurityProfile::TlsBasicAuth),
        3 => builder = builder.security_profile(SecurityProfile::TlsClientCertificate),
        _ => {}
    }
    if opts.profile == 1 || opts.profile == 2 {
        let password = BasicAuthPassword::utf8(fixture.password(&opts.station))
            .map_err(|e| format!("password: {e}"))?;
        builder = builder.password(password);
    }
    if let Some(t) = opts.tls {
        builder = builder.tls(t);
    }
    let station = builder.build().map_err(|e| e.to_string())?;
    let handle = station.spawn().map_err(|e| e.to_string())?;
    let mut events = handle.events();
    let deadline = Instant::now() + opts.connect_timeout;
    loop {
        if handle.state().connected {
            return Ok(handle);
        }
        let left = deadline.saturating_duration_since(Instant::now());
        if left.is_zero() {
            handle.shutdown(Duration::from_millis(100)).await;
            return Err(
                "the WebSocket handshake did not complete within --connect-timeout-ms".into(),
            );
        }
        match tokio::time::timeout(left.min(Duration::from_millis(50)), events.recv()).await {
            Ok(Ok(Event::Connected { .. })) => return Ok(handle),
            Ok(Ok(Event::Disconnected { reason })) => {
                handle.shutdown(Duration::from_millis(100)).await;
                return Err(reason);
            }
            _ => {}
        }
    }
}

/// `client <Action>`: connect, send one CALL, print the result document.
pub async fn one_shot(
    opts: ConnectOptions,
    fixture: Arc<Fixture>,
    action: &str,
    payload: Value,
    timeout_ms: u64,
) -> i32 {
    let started = Instant::now();
    let endpoint = Endpoint::new(&opts.version, fixture.clone(), "station", false);
    let station = opts.station.clone();
    let version = opts.version.clone();
    let url = format!("{}/{}", opts.base, opts.station);
    // What the station cannot send is refused before any network access.
    let check = endpoint
        .execute(&station, action, payload.clone(), timeout_ms, None)
        .await;
    if check.err_code == "unsupported" || check.err_code == "invalid-request" {
        crate::out::write_line(&check.to_value(ADAPTER));
        return crate::out::exit_code_of(&check.err_code);
    }
    let mut doc = match connect(opts, endpoint.clone(), &fixture).await {
        Ok(handle) => {
            let doc = endpoint
                .execute(&station, action, payload, timeout_ms, Some(handle.clone()))
                .await;
            handle.shutdown(Duration::from_secs(2)).await;
            doc
        }
        Err(e) => ResultDoc::failed(
            &version,
            action,
            payload,
            "connect-failed",
            format!("{url}: {e}"),
        ),
    };
    doc.elapsed_ms = started.elapsed().as_millis();
    crate::out::write_line(&doc.to_value(ADAPTER));
    crate::out::exit_code_of(&doc.err_code)
}

/// `client run`: connect and stay connected.
pub async fn run(
    opts: ConnectOptions,
    fixture: Arc<Fixture>,
    hold_ms: u64,
    ready_file: String,
    control_socket: String,
) -> i32 {
    let mut term = signal(SignalKind::terminate()).expect("signal handler");
    let mut int = signal(SignalKind::interrupt()).expect("signal handler");
    let endpoint = Endpoint::new(&opts.version, fixture.clone(), "station", true);
    let station = opts.station.clone();
    let version = opts.version.clone();
    let compression = opts.compression;
    let url = format!("{}/{}", opts.base, opts.station);
    let handle = match connect(opts, endpoint.clone(), &fixture).await {
        Ok(h) => h,
        Err(e) => {
            event(
                "connection-failed",
                Some(&station),
                vec![("url", json!(url)), ("detail", json!(e))],
            );
            event("stopped", None, vec![]);
            return 3;
        }
    };
    endpoint.watch(station.clone(), &handle);
    // ocpp-kit does not say whether permessage-deflate was negotiated.
    event(
        "connection-opened",
        Some(&station),
        vec![
            ("subprotocol", json!(format!("ocpp{version}"))),
            ("remote", json!(url)),
            (
                "compression",
                if compression {
                    Value::Null
                } else {
                    json!(false)
                },
            ),
        ],
    );
    let control_handle = handle.clone();
    let control_endpoint = endpoint.clone();
    let control_station = station.clone();
    let control_version = version.clone();
    let handler: ControlHandler = Arc::new(move |request| {
        let handle = control_handle.clone();
        let endpoint = control_endpoint.clone();
        let station = control_station.clone();
        let version = control_version.clone();
        Box::pin(async move {
            if let Some(wanted) = &request.station {
                if !wanted.is_empty() && *wanted != station {
                    return ResultDoc::failed(
                        &version,
                        &request.action,
                        request.payload,
                        "not-connected",
                        format!("no open connection for station {wanted:?}"),
                    );
                }
            }
            if !handle.state().connected {
                return ResultDoc::failed(
                    &version,
                    &request.action,
                    request.payload,
                    "not-connected",
                    "the connection is closed",
                );
            }
            endpoint
                .execute(
                    &station,
                    &request.action,
                    request.payload,
                    request.timeout_ms,
                    Some(handle),
                )
                .await
        })
    });
    let control = match control::serve(&control_socket, ADAPTER, handler) {
        Ok(c) => c,
        Err(e) => {
            eprintln!("client: cannot open the control socket {control_socket}: {e}");
            handle.shutdown(Duration::from_secs(1)).await;
            return 3;
        }
    };
    if let Err(e) = std::fs::write(&ready_file, "ready\n") {
        eprintln!("client: cannot write {ready_file}: {e}");
        return 1;
    }
    event(
        "ready",
        Some(&station),
        vec![("version", json!(version)), ("url", json!(url))],
    );

    let mut events = handle.events();
    let lost = async {
        loop {
            match events.recv().await {
                Ok(Event::Disconnected { .. }) => return,
                Err(tokio::sync::broadcast::error::RecvError::Closed) => return,
                _ => {}
            }
        }
    };
    let hold = async {
        if hold_ms > 0 {
            tokio::time::sleep(Duration::from_millis(hold_ms)).await;
        } else {
            std::future::pending::<()>().await;
        }
    };
    let was_lost = tokio::select! {
        _ = term.recv() => false,
        _ = int.recv() => false,
        _ = hold => false,
        _ = lost => true,
    };
    let _ = std::fs::remove_file(&ready_file);
    control.abort();
    if was_lost {
        // ocpp-kit reports the reason for logs only, not the close frame.
        event(
            "connection-closed",
            Some(&station),
            vec![("code", Value::Null), ("reason", Value::Null)],
        );
        event("stopped", None, vec![]);
        return 1;
    }
    handle.shutdown(Duration::from_secs(5)).await;
    event(
        "connection-closed",
        Some(&station),
        vec![("code", Value::Null), ("reason", Value::Null)],
    );
    event("stopped", None, vec![]);
    0
}

pub fn client_tls(dir: &str, profile: u8) -> Result<ClientTls, String> {
    tls::client_tls(dir, profile)
}
