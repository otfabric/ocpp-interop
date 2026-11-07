// SPDX-License-Identifier: MIT
//! ocpp-interop is the ocpp-interop adapter for hupe1980/ocpp-kit (Rust),
//! behind the container contract (docs/CONTAINER_CONTRACT.md): a reference
//! CSMS and station for OCPP 1.6J, 2.0.1 and 2.1, with the OCPP 2.1 SEND and
//! CALLRESULTERROR message types and permessage-deflate.
//!
//! The adapter is glue: argument parsing, fixture loading, the control socket
//! and JSON output. Framing, payload validation, request routing, pending
//! requests, timeouts, SEND, CALLRESULTERROR, the WebSocket transport and TLS
//! are ocpp-kit's (transport::Csms, transport::Station, engine).

mod actions;
mod client;
mod control;
mod endpoint;
mod fixture;
mod out;
mod server;
mod tls;
mod typed21;

use std::collections::HashMap;
use std::net::IpAddr;
use std::sync::Arc;
use std::time::Duration;

use regex::Regex;
use serde_json::{Value, json};

use crate::fixture::{Fixture, FixtureError};

pub const ADAPTER: &str = "ocppkit";
const SCHEMA_VERSION: &str = "1.0";
const CONTRACT_VERSION: &str = "0.7";
const READY_FILE: &str = "/run/ocpp-interop/ready";
const CONTROL_SOCKET: &str = "/run/ocpp-interop/control.sock";
const VERSIONS: [&str; 3] = ["1.6", "2.0.1", "2.1"];

// Set at build time (Dockerfile).
const ADAPTER_VERSION: &str = match option_env!("ADAPTER_VERSION") {
    Some(v) => v,
    None => "dev",
};
const UPSTREAM_VERSION: &str = match option_env!("UPSTREAM_VERSION") {
    Some(v) => v,
    None => "unknown",
};
const UPSTREAM_REVISION: &str = match option_env!("UPSTREAM_REVISION") {
    Some(v) => v,
    None => "0000000000000000000000000000000000000000",
};

const USAGE: &str = "usage:
  ocpp-interop server --version V [--fixture PATH] [--bind-address ADDR] [--port N]
                      [--security-profile N] [--tls-dir DIR] [--ready-file PATH] [--control-socket PATH]
  ocpp-interop client run --version V --url URL [--station ID] [--fixture PATH] [--security-profile N]
                      [--tls-dir DIR] [--compression] [--connect-timeout-ms N] [--timeout-ms N] [--hold-ms N]
                      [--ready-file PATH] [--control-socket PATH]
  ocpp-interop client <Action> --version V --url URL [--station ID] [--fixture PATH] [--payload JSON]
                      [--security-profile N] [--tls-dir DIR] [--compression] [--connect-timeout-ms N]
                      [--timeout-ms N]
  ocpp-interop call [--station ID] <Action> [--payload JSON] [--timeout-ms N] [--control-socket PATH]
  ocpp-interop print-capabilities
  ocpp-interop print-fixture --version V [NAME]
  ocpp-interop version";

/// How a command ends before it ran: the exit codes of the contract.
enum Failure {
    Usage(String),       // 2, nothing was sent
    Fixture(String),     // 4, the fixture is missing or invalid
    Unsupported(String), // 5
    Exit(i32),           // the command wrote its output
}

type Outcome = Result<(), Failure>;

fn usage(msg: impl Into<String>) -> Failure {
    Failure::Usage(msg.into())
}

/// `--name value` pairs and the positional arguments.
struct Flags {
    given: HashMap<String, String>,
    positional: Vec<String>,
}

/// The flags without a value.
const SWITCHES: [&str; 1] = ["compression"];

fn parse_flags(args: &[String], known: &[&str]) -> Result<Flags, Failure> {
    let mut f = Flags {
        given: HashMap::new(),
        positional: vec![],
    };
    let mut i = 0;
    while i < args.len() {
        let arg = &args[i];
        let Some(name) = arg.strip_prefix("--") else {
            f.positional.push(arg.clone());
            i += 1;
            continue;
        };
        if !known.contains(&name) {
            return Err(usage(format!("unknown flag {arg:?}")));
        }
        if f.given.contains_key(name) {
            return Err(usage(format!("{arg} is given more than once")));
        }
        if SWITCHES.contains(&name) {
            f.given.insert(name.into(), "true".into());
            i += 1;
            continue;
        }
        let Some(value) = args.get(i + 1) else {
            return Err(usage(format!("{arg} needs a value")));
        };
        f.given.insert(name.into(), value.clone());
        i += 2;
    }
    Ok(f)
}

impl Flags {
    fn get(&self, name: &str, fallback: &str) -> String {
        self.given
            .get(name)
            .cloned()
            .unwrap_or_else(|| fallback.to_string())
    }

    fn has(&self, name: &str) -> bool {
        self.given.contains_key(name)
    }

    fn number(&self, name: &str, fallback: u64, low: u64, high: u64) -> Result<u64, Failure> {
        let Some(text) = self.given.get(name) else {
            return Ok(fallback);
        };
        if text.is_empty() || text.len() > 12 || !text.bytes().all(|b| b.is_ascii_digit()) {
            return Err(usage(format!("--{name}: {text:?} is not a number")));
        }
        let v: u64 = text.parse().expect("digits");
        if v < low || v > high {
            return Err(usage(format!("--{name}: {v} is not within {low}..{high}")));
        }
        Ok(v)
    }

    fn version(&self) -> Result<String, Failure> {
        if !self.has("version") {
            return Err(usage("--version is required"));
        }
        let v = self.get("version", "");
        if !VERSIONS.contains(&v.as_str()) {
            return Err(usage(format!("--version: {v:?} is not 1.6, 2.0.1 or 2.1")));
        }
        Ok(v)
    }

    fn security_profile(&self) -> Result<u8, Failure> {
        let p = self.number("security-profile", 0, 0, 3)? as u8;
        if self.has("tls-dir") && p < 2 {
            return Err(usage("--tls-dir is for security profiles 2 and 3"));
        }
        if p >= 2 && !self.has("tls-dir") {
            return Err(usage(format!("security profile {p} needs --tls-dir")));
        }
        Ok(p)
    }

    fn payload(&self) -> Result<Value, Failure> {
        let text = self.get("payload", "{}");
        let v: Value = serde_json::from_str(&text)
            .map_err(|e| usage(format!("--payload is not one JSON value: {e}")))?;
        if !v.is_object() {
            return Err(usage("--payload must be a JSON object"));
        }
        Ok(v)
    }

    fn fixture(&self, version: &str) -> Result<Arc<Fixture>, Failure> {
        let path = self.get("fixture", &fixture::default_path(version));
        match fixture::load(&path, version) {
            Ok(f) => Ok(Arc::new(f)),
            Err(FixtureError::Invalid(m)) => Err(Failure::Fixture(m)),
            Err(FixtureError::Unsupported(m)) => Err(Failure::Unsupported(m)),
        }
    }
}

fn action_of(text: &str) -> Result<String, Failure> {
    let re = Regex::new(r"^[A-Z][A-Za-z0-9]+$").expect("regex");
    if re.is_match(text) {
        Ok(text.to_string())
    } else {
        Err(usage(format!("{text:?} is not an OCPP action name")))
    }
}

fn main() {
    let args: Vec<String> = std::env::args().skip(1).collect();
    let code = match dispatch(&args) {
        Ok(()) => 0,
        Err(Failure::Exit(c)) => c,
        Err(Failure::Usage(m)) => {
            eprintln!("usage error: {m}\n{USAGE}");
            2
        }
        Err(Failure::Unsupported(m)) => {
            eprintln!("unsupported: {m}");
            5
        }
        Err(Failure::Fixture(m)) => {
            eprintln!("invalid fixture: {m}");
            4
        }
    };
    std::process::exit(code);
}

fn runtime() -> tokio::runtime::Runtime {
    tokio::runtime::Builder::new_multi_thread()
        .enable_all()
        .build()
        .expect("tokio runtime")
}

fn dispatch(args: &[String]) -> Outcome {
    let Some(command) = args.first() else {
        return Err(usage("no command"));
    };
    let rest = &args[1..];
    match command.as_str() {
        "server" => command_server(rest),
        "client" => command_client(rest),
        "call" => command_call(rest),
        "print-capabilities" => {
            if !rest.is_empty() {
                return Err(usage("print-capabilities takes no arguments"));
            }
            out::write_line(&capabilities());
            Ok(())
        }
        "print-fixture" => command_print_fixture(rest),
        "version" => {
            if !rest.is_empty() {
                return Err(usage("version takes no arguments"));
            }
            out::write_raw(
                format!("ocpp-interop-{ADAPTER} {ADAPTER_VERSION} (hupe1980/ocpp-kit {UPSTREAM_VERSION}, contract {CONTRACT_VERSION})\n")
                    .as_bytes(),
            );
            Ok(())
        }
        other => Err(usage(format!("unknown command {other:?}"))),
    }
}

fn command_server(args: &[String]) -> Outcome {
    let f = parse_flags(
        args,
        &[
            "version",
            "fixture",
            "bind-address",
            "port",
            "security-profile",
            "tls-dir",
            "ready-file",
            "control-socket",
        ],
    )?;
    if let Some(p) = f.positional.first() {
        return Err(usage(format!("unexpected argument {p:?}")));
    }
    let port = f.number("port", 9000, 1, 65535)? as u16;
    let bind = f.get("bind-address", "0.0.0.0");
    if bind.parse::<IpAddr>().is_err() {
        return Err(usage(format!(
            "--bind-address: {bind:?} is not an IP address"
        )));
    }
    let version = f.version()?;
    let profile = f.security_profile()?;
    let tls_dir = if profile >= 2 {
        let dir = f.get("tls-dir", "");
        tls::server_tls(&dir, profile).map_err(usage)?;
        Some(dir)
    } else {
        None
    };
    let fixture = f.fixture(&version)?;
    let opts = server::ServerOptions {
        version,
        bind,
        port,
        profile,
        tls_dir,
        ready_file: f.get("ready-file", READY_FILE),
        control_socket: f.get("control-socket", CONTROL_SOCKET),
    };
    Err(Failure::Exit(
        runtime().block_on(server::run(opts, fixture)),
    ))
}

fn command_client(args: &[String]) -> Outcome {
    let Some(mode) = args.first().filter(|a| !a.starts_with("--")) else {
        return Err(usage("client needs 'run' or an action"));
    };
    let run = mode == "run";
    let mut known = vec![
        "version",
        "url",
        "station",
        "fixture",
        "security-profile",
        "tls-dir",
        "compression",
        "connect-timeout-ms",
        "timeout-ms",
    ];
    if run {
        known.extend(["hold-ms", "ready-file", "control-socket"]);
    } else {
        known.push("payload");
    }
    let f = parse_flags(&args[1..], &known)?;
    if let Some(p) = f.positional.first() {
        return Err(usage(format!("unexpected argument {p:?}")));
    }
    let action = if run { String::new() } else { action_of(mode)? };
    if !f.has("url") {
        return Err(usage("--url is required"));
    }
    let url = f.get("url", "");
    if !url.starts_with("ws://") && !url.starts_with("wss://") {
        return Err(usage("--url must start with ws:// or wss://"));
    }
    let connect_timeout_ms = f.number("connect-timeout-ms", 5000, 1, 3_600_000)?;
    let timeout_ms = f.number("timeout-ms", 10_000, 1, 3_600_000)?;
    let hold_ms = if run {
        f.number("hold-ms", 0, 0, 1 << 31)?
    } else {
        0
    };
    let payload = if run { json!({}) } else { f.payload()? };
    let version = f.version()?;
    let profile = f.security_profile()?;
    if url.starts_with("wss://") != (profile >= 2) {
        return Err(usage(
            "--url must be wss:// with security profiles 2 and 3, and ws:// otherwise",
        ));
    }
    let tls = if profile >= 2 {
        Some(client::client_tls(&f.get("tls-dir", ""), profile).map_err(usage)?)
    } else {
        None
    };
    if f.has("station") && f.get("station", "").is_empty() {
        return Err(Failure::Unsupported(
            "--station \"\": ocpp-kit's station always appends its identity to the URL".into(),
        ));
    }
    let fixture = f.fixture(&version)?;
    if profile == 0 {
        // OCPP 2.0.1 Part 2 A00 and the 1.6 Security Whitepaper define
        // profiles 1 to 3; ocpp-kit's station has no connection without
        // credentials (client.securityProfiles).
        return Err(Failure::Unsupported(
            "--security-profile 0: ocpp-kit's station always authenticates (profiles 1 to 3)"
                .into(),
        ));
    }
    let station = f.get("station", &fixture.first_station);
    let opts = client::ConnectOptions {
        version,
        base: url.trim_end_matches('/').to_string(),
        station,
        profile,
        tls,
        compression: f.has("compression"),
        connect_timeout: Duration::from_millis(connect_timeout_ms),
    };
    let rt = runtime();
    let code = if run {
        rt.block_on(client::run(
            opts,
            fixture,
            hold_ms,
            f.get("ready-file", READY_FILE),
            f.get("control-socket", CONTROL_SOCKET),
        ))
    } else {
        rt.block_on(client::one_shot(
            opts, fixture, &action, payload, timeout_ms,
        ))
    };
    rt.shutdown_timeout(Duration::from_millis(500));
    Err(Failure::Exit(code))
}

fn command_call(args: &[String]) -> Outcome {
    let f = parse_flags(
        args,
        &["station", "payload", "timeout-ms", "control-socket"],
    )?;
    if f.positional.len() != 1 {
        return Err(usage("exactly one action is required"));
    }
    let action = action_of(&f.positional[0])?;
    let payload = f.payload()?;
    let timeout_ms = f.number("timeout-ms", 10_000, 1, 3_600_000)?;
    let station = f.given.get("station").cloned();
    Err(Failure::Exit(control::call(
        &f.get("control-socket", CONTROL_SOCKET),
        station,
        &action,
        payload,
        timeout_ms,
    )))
}

fn command_print_fixture(args: &[String]) -> Outcome {
    let f = parse_flags(args, &["version"])?;
    let version = f.get("version", "");
    let Some(dir) = fixture::version_dir(&version) else {
        return Err(usage("--version 1.6, 2.0.1 or 2.1 is required"));
    };
    if f.positional.len() > 1 {
        return Err(usage("at most one fixture name"));
    }
    let name = f
        .positional
        .first()
        .cloned()
        .unwrap_or_else(|| "baseline".into());
    let re = Regex::new(r"^[a-z0-9][a-z0-9-]*$").expect("regex");
    if !re.is_match(&name) {
        return Err(usage(format!("{name:?} is not a fixture name")));
    }
    match std::fs::read(format!(
        "{}/{dir}/{name}/fixture.json",
        fixture::FIXTURES_DIR
    )) {
        Ok(data) => {
            out::write_raw(&data);
            Ok(())
        }
        Err(_) => {
            eprintln!("print-fixture: no fixture {name:?} for OCPP {version} in this image");
            Err(Failure::Exit(1))
        }
    }
}

/// The print-capabilities document. Only what the self-tests exercise is
/// declared (REQUIREMENTS.md R4.2).
fn capabilities() -> Value {
    let mut by_version = serde_json::Map::new();
    for v in VERSIONS {
        by_version.insert(v.into(), version_capabilities(v));
    }
    json!({
        "schemaVersion": SCHEMA_VERSION,
        "adapter": ADAPTER,
        "adapterVersion": ADAPTER_VERSION,
        "contractVersion": CONTRACT_VERSION,
        "upstream": {
            "name": "hupe1980/ocpp-kit",
            "repository": "https://github.com/hupe1980/ocpp-kit",
            "version": UPSTREAM_VERSION,
            "revision": UPSTREAM_REVISION,
            "license": "MIT OR Apache-2.0",
            "language": "Rust",
            "independentEngine": true,
        },
        "versions": by_version,
    })
}

/// Actions of a version ocpp-kit cannot exchange in the direction OCPP gives
/// them, left out of `sends` and `handles` (README.md, What it cannot do).
const NOT_EXCHANGED: [(&str, &str); 1] = [
    // OCPP 2.1 Part 2 (use case C, payment): the CSMS sends it. ocpp-kit
    // 0.3.0 has it as a station message (v2_1/action.rs, Origin), so its
    // CSMS refuses to send it and its station refuses to receive it.
    ("2.1", "NotifyWebPaymentStarted"),
];

fn version_capabilities(version: &str) -> Value {
    let a = actions::actions(version).expect("a supported version");
    let is21 = version == "2.1";
    let list = |from: &[&str]| -> Vec<String> {
        let mut out: Vec<String> = from
            .iter()
            .filter(|x| is21 || !a.send.contains(x))
            .filter(|x| !NOT_EXCHANGED.contains(&(version, **x)))
            .map(|x| x.to_string())
            .collect();
        out.sort();
        out
    };
    let station_sends = list(a.from_station);
    let csms_sends = list(a.from_csms);
    let notes = vec![
        "ocpp-kit's transport::Csms (server) and transport::Station (client), one OCPP version per process. A handler for every action of the version answers from the fixture; ocpp-kit decodes every request and answer into its typed messages (generated from the OCA schemas) and answers a request it cannot decode with a CALLERROR before the handler runs (no call-received).".to_string(),
        format!("An action of OCPP {version} the receiving role gets without a fixture answer is answered NotImplemented, an action the role does not receive NotSupported. A callError answer is a CALLERROR with the fixture's code (ocpp-kit's ErrorCode in the version's spelling)."),
        "The server refuses an identity the fixture does not list (HTTP 404), wrong Basic credentials and a client certificate of another identity (HTTP 401) in its authenticate hook.".to_string(),
        "Compression: both ends offer or accept permessage-deflate (CsmsBuilder::compression, StationBuilder::compression); ocpp-kit does not say whether it was negotiated, so connection-opened has compression null when it was offered.".to_string(),
        "client --station \"\" exits 5: ocpp-kit's station appends its identity to the URL.".to_string(),
        "OCPP 2.1 NotifyWebPaymentStarted is not declared: OCPP 2.1 Part 2 has the CSMS send it, ocpp-kit 0.3.0 has it as a station message (its Origin is ChargingStation), so neither end can exchange it with another implementation.".to_string(),
        "The station has security profiles 1 to 3 only (client.securityProfiles, contract 0.7): ocpp-kit's station always sends credentials, so client --security-profile 0 exits 5 and the self-tests run its pairings with profile 1 at both ends. At profiles 1 and 2 a --station the fixture does not list is sent with a password of the adapter's own (the server refuses the identity before it checks one).".to_string(),
        "Message ids are ocpp-kit's, which does not return them (messageIds false); call-received has the id of the peer.".to_string(),
        "No automatic Heartbeat (HeartbeatPolicy::Manual), the CSMS boot gate is off and every CALL of the station is sent as triggered, so that the tests decide what is sent. No protocol-error events: ocpp-kit has no hook for a frame it cannot read. connection-closed has code and reason null: ocpp-kit does not report the close frame.".to_string(),
        if is21 {
            "OCPP 2.1 SEND and CALLRESULTERROR (contract 0.6): SENDs are sent with Handle::send and reach the handler as MessageKind::Send; an invalid CALLRESULT is answered with a CALLRESULTERROR by ocpp-kit (typed calls) and reported, and a CALLRESULTERROR received is an Event::ResultRejected (call-result-error).".to_string()
        } else {
            format!("OCPP {version} has no SEND or CALLRESULTERROR.")
        },
    ];
    json!({
        "client": {"supported": true, "sends": station_sends, "handles": csms_sends, "securityProfiles": [1, 2, 3]},
        "server": {"supported": true, "sends": csms_sends, "handles": station_sends, "securityProfiles": [0, 1, 2, 3]},
        "features": {
            "securityProfiles": [0, 1, 2, 3],
            "compression": true,
            "send": is21,
            "callResultError": is21,
            "fixtureCallErrors": true,
            "messageIds": false,
        },
        "notes": notes,
    })
}
