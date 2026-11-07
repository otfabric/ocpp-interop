// SPDX-License-Identifier: MIT
//! The control socket of `call` (docs/CONTAINER_CONTRACT.md, call): one JSON
//! line each way, `{"station", "action", "payload", "timeoutMs"}` and the
//! result document.

use std::future::Future;
use std::io::{BufRead, BufReader, Write};
use std::path::Path;
use std::pin::Pin;
use std::sync::Arc;
use std::time::Duration;

use serde_json::{Value, json};
use tokio::io::{AsyncBufReadExt, AsyncWriteExt, BufReader as AsyncBufReader};
use tokio::net::UnixListener;

use crate::out::{ResultDoc, exit_code_of, write_raw};

pub struct ControlRequest {
    pub station: Option<String>,
    pub action: String,
    pub payload: Value,
    pub timeout_ms: u64,
}

pub type ControlHandler =
    Arc<dyn Fn(ControlRequest) -> Pin<Box<dyn Future<Output = ResultDoc> + Send>> + Send + Sync>;

/// Serves the control socket at `path` until the task is dropped.
pub fn serve(
    path: &str,
    adapter: &'static str,
    handler: ControlHandler,
) -> std::io::Result<tokio::task::JoinHandle<()>> {
    let _ = std::fs::remove_file(path);
    if let Some(dir) = Path::new(path).parent() {
        std::fs::create_dir_all(dir)?;
    }
    let listener = UnixListener::bind(path)?;
    Ok(tokio::spawn(async move {
        loop {
            let Ok((stream, _)) = listener.accept().await else {
                return;
            };
            let handler = handler.clone();
            tokio::spawn(async move {
                let (read, mut write) = stream.into_split();
                let mut line = String::new();
                if AsyncBufReader::new(read)
                    .read_line(&mut line)
                    .await
                    .is_err()
                    || line.trim().is_empty()
                {
                    return;
                }
                let Ok(v) = serde_json::from_str::<Value>(&line) else {
                    eprintln!("control socket: the request is not JSON");
                    return;
                };
                let request = ControlRequest {
                    station: v.get("station").and_then(Value::as_str).map(str::to_string),
                    action: v
                        .get("action")
                        .and_then(Value::as_str)
                        .unwrap_or("")
                        .to_string(),
                    payload: v
                        .get("payload")
                        .cloned()
                        .filter(Value::is_object)
                        .unwrap_or_else(|| json!({})),
                    timeout_ms: v
                        .get("timeoutMs")
                        .and_then(Value::as_u64)
                        .filter(|t| *t > 0)
                        .unwrap_or(10_000),
                };
                let doc = handler(request).await;
                let mut out = doc.to_value(adapter).to_string();
                out.push('\n');
                let _ = write.write_all(out.as_bytes()).await;
            });
        }
    }))
}

/// `call`: one request to the running process; returns the exit code.
pub fn call(
    path: &str,
    station: Option<String>,
    action: &str,
    payload: Value,
    timeout_ms: u64,
) -> i32 {
    let mut stream = match std::os::unix::net::UnixStream::connect(path) {
        Ok(s) => s,
        Err(e) => {
            eprintln!("call: no running server or client run on {path}: {e}");
            return 3;
        }
    };
    let deadline = Some(Duration::from_millis(timeout_ms) + Duration::from_secs(15));
    let _ = stream.set_read_timeout(deadline);
    let request =
        json!({"station": station, "action": action, "payload": payload, "timeoutMs": timeout_ms});
    if let Err(e) = stream.write_all(format!("{request}\n").as_bytes()) {
        eprintln!("call: {e}");
        return 3;
    }
    let mut line = String::new();
    let _ = BufReader::new(&stream).read_line(&mut line);
    let Ok(doc) = serde_json::from_str::<Value>(line.trim()) else {
        eprintln!("call: the running process did not answer");
        return 1;
    };
    write_raw(format!("{}\n", line.trim()).as_bytes());
    exit_code_of(
        doc.pointer("/error/code")
            .and_then(Value::as_str)
            .unwrap_or(""),
    )
}
