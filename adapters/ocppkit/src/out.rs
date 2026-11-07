// SPDX-License-Identifier: MIT
//! stdout carries JSON only (events, result documents, capabilities), one
//! document per line, flushed as it is written.

use std::io::Write;
use std::sync::Mutex;
use std::time::{SystemTime, UNIX_EPOCH};

use serde_json::{Map, Value, json};

static STDOUT: Mutex<()> = Mutex::new(());

pub fn write_line(document: &Value) {
    let _guard = STDOUT.lock().unwrap_or_else(|e| e.into_inner());
    let mut out = std::io::stdout().lock();
    let _ = writeln!(out, "{document}");
    let _ = out.flush();
}

pub fn write_raw(data: &[u8]) {
    let _guard = STDOUT.lock().unwrap_or_else(|e| e.into_inner());
    let mut out = std::io::stdout().lock();
    let _ = out.write_all(data);
    let _ = out.flush();
}

/// UTC civil time of a Unix time: (year, month, day, hour, minute, second).
fn civil(secs: u64) -> (i64, u32, u32, u32, u32, u32) {
    let days = (secs / 86_400) as i64;
    let rem = secs % 86_400;
    // Howard Hinnant's days-to-civil.
    let z = days + 719_468;
    let era = z.div_euclid(146_097);
    let doe = z.rem_euclid(146_097);
    let yoe = (doe - doe / 1460 + doe / 36_524 - doe / 146_096) / 365;
    let y = yoe + era * 400;
    let doy = doe - (365 * yoe + yoe / 4 - yoe / 100);
    let mp = (5 * doy + 2) / 153;
    let d = (doy - (153 * mp + 2) / 5 + 1) as u32;
    let m = if mp < 10 { mp + 3 } else { mp - 9 } as u32;
    let y = if m <= 2 { y + 1 } else { y };
    (
        y,
        m,
        d,
        (rem / 3600) as u32,
        (rem % 3600 / 60) as u32,
        (rem % 60) as u32,
    )
}

/// RFC 3339 UTC, whole seconds: the fixture's "${now}".
pub fn now_seconds() -> String {
    let secs = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|d| d.as_secs())
        .unwrap_or(0);
    let (y, mo, d, h, mi, s) = civil(secs);
    format!("{y:04}-{mo:02}-{d:02}T{h:02}:{mi:02}:{s:02}Z")
}

/// RFC 3339 UTC with milliseconds: the time of an event.
pub fn timestamp() -> String {
    let now = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default();
    let (y, mo, d, h, mi, s) = civil(now.as_secs());
    format!(
        "{y:04}-{mo:02}-{d:02}T{h:02}:{mi:02}:{s:02}.{:03}Z",
        now.subsec_millis()
    )
}

/// Writes one event; `fields` are written in the order given.
pub fn event(name: &str, station: Option<&str>, fields: Vec<(&str, Value)>) {
    let mut o = Map::new();
    o.insert("event".into(), json!(name));
    o.insert("time".into(), json!(timestamp()));
    if let Some(station) = station {
        if !station.is_empty() {
            o.insert("station".into(), json!(station));
        }
    }
    for (k, v) in fields {
        o.insert(k.into(), v);
    }
    write_line(&Value::Object(o));
}

pub fn error_object(code: &str, description: &str, details: Value) -> Value {
    let details = if details.is_null() {
        json!({})
    } else {
        details
    };
    json!({"code": code, "description": description, "details": details})
}

/// A result document (docs/CONTAINER_CONTRACT.md, Result document).
#[derive(Default, Clone)]
pub struct ResultDoc {
    pub version: String,
    pub operation: String,
    pub ok: bool,
    pub message_id: Option<String>,
    pub request: Option<Value>,
    pub response: Option<Value>,
    pub call_error: Option<Value>,
    pub call_result_error: Option<Value>,
    pub err_code: String,
    pub err_msg: String,
    pub elapsed_ms: u128,
}

impl ResultDoc {
    pub fn failed(
        version: &str,
        operation: &str,
        request: Value,
        code: &str,
        msg: impl Into<String>,
    ) -> Self {
        ResultDoc {
            version: version.into(),
            operation: operation.into(),
            request: Some(request),
            err_code: code.into(),
            err_msg: msg.into(),
            ..Default::default()
        }
    }

    pub fn to_value(&self, adapter: &str) -> Value {
        let mut o = Map::new();
        o.insert("schemaVersion".into(), json!("1.0"));
        o.insert("adapter".into(), json!(adapter));
        o.insert("version".into(), json!(self.version));
        o.insert("operation".into(), json!(self.operation));
        o.insert("ok".into(), json!(self.ok));
        if let Some(id) = &self.message_id {
            o.insert("messageId".into(), json!(id));
        }
        if let Some(r) = &self.request {
            o.insert("request".into(), r.clone());
        }
        o.insert(
            "response".into(),
            self.response.clone().unwrap_or(Value::Null),
        );
        o.insert(
            "callError".into(),
            self.call_error.clone().unwrap_or(Value::Null),
        );
        if let Some(cre) = &self.call_result_error {
            o.insert("callResultError".into(), cre.clone());
        }
        if self.err_code.is_empty() {
            o.insert("error".into(), Value::Null);
        } else {
            o.insert(
                "error".into(),
                json!({"code": self.err_code, "message": self.err_msg}),
            );
        }
        o.insert("elapsedMs".into(), json!(self.elapsed_ms as u64));
        Value::Object(o)
    }
}

/// The exit code that goes with a result document.
pub fn exit_code_of(err_code: &str) -> i32 {
    match err_code {
        "" => 0,
        "connect-failed" => 3,
        "unsupported" => 5,
        _ => 1,
    }
}
