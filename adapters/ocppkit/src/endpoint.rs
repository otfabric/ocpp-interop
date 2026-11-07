// SPDX-License-Identifier: MIT
//! The fixture's side of a connection: answers the peer's CALLs from the
//! fixture (an ocpp-kit Handler), sends CALLs and SENDs for the contract, and
//! writes the events.

use std::collections::HashMap;
use std::sync::{Arc, Mutex};
use std::time::{Duration, Instant};

use ocpp_kit::RawValue;
use ocpp_kit::decode::{DecodeError, DecodeOptions};
use ocpp_kit::engine::{Answer as EngineAnswer, CallFailure, CallOptions, IncomingRequest};
use ocpp_kit::message::MessageKind;
use ocpp_kit::rpc::{CallError, ErrorCode};
use ocpp_kit::transport::{BoxFuture, Ctx, Event, Handle, Handler};
use ocpp_kit::{Version, v1_6, v2_0_1, v2_1};
use serde_json::{Value, json};

use crate::actions::actions;
use crate::fixture::{Answer, Fixture};
use crate::out::{ResultDoc, error_object, event};
use crate::typed21;

pub struct Endpoint {
    pub version: String,
    pub fixture: Arc<Fixture>,
    /// The answering role: "csms" for a server, "station" for a client.
    pub role: &'static str,
    /// Write events (server, client run).
    pub emit: bool,
    answered: Mutex<HashMap<(String, String), String>>,
}

/// ocpp-kit's checks of a payload (transcode_request / transcode_response of
/// the version); None for an action ocpp-kit does not have.
fn transcode(
    version: &str,
    action: &str,
    payload: &RawValue,
    response: bool,
) -> Option<Result<Box<RawValue>, DecodeError>> {
    let o = DecodeOptions::default();
    Some(match version {
        "1.6" => {
            let a = v1_6::Action::from_wire(action)?;
            if response {
                v1_6::transcode_response(a, payload, &o)
            } else {
                v1_6::transcode_request(a, payload, &o)
            }
        }
        "2.0.1" => {
            let a = v2_0_1::Action::from_wire(action)?;
            if response {
                v2_0_1::transcode_response(a, payload, &o)
            } else {
                v2_0_1::transcode_request(a, payload, &o)
            }
        }
        _ => {
            let a = v2_1::Action::from_wire(action)?;
            if response {
                v2_1::transcode_response(a, payload, &o)
            } else {
                v2_1::transcode_request(a, payload, &o)
            }
        }
    })
}

fn raw(v: &Value) -> Box<RawValue> {
    RawValue::from_string(v.to_string())
        .unwrap_or_else(|_| RawValue::from_string("{}".into()).expect("valid"))
}

fn parse(r: &RawValue) -> Value {
    serde_json::from_str(r.get()).unwrap_or(Value::Null)
}

fn wire_code(version: &str, code: &ErrorCode) -> String {
    let v = version.parse::<Version>().unwrap_or(Version::V2_1);
    code.as_wire(v).to_string()
}

/// A CallFailure::Rejected that is ocpp-kit's own refusal of the peer's
/// CALLRESULT (a decoding error; ocpp-kit answered it with a CALLRESULTERROR
/// on 2.1), not the peer's CALLERROR: ocpp-kit's decoding errors carry a
/// "reason" in their details, the CALLERRORs of the fixtures do not.
fn is_own_rejection(e: &CallError) -> bool {
    matches!(
        e.code,
        ErrorCode::FormatViolation
            | ErrorCode::OccurrenceConstraintViolation
            | ErrorCode::TypeConstraintViolation
            | ErrorCode::PropertyConstraintViolation
            | ErrorCode::ProtocolError
    ) && e.details.get("reason").is_some()
}

impl Endpoint {
    pub fn new(version: &str, fixture: Arc<Fixture>, role: &'static str, emit: bool) -> Arc<Self> {
        Arc::new(Endpoint {
            version: version.into(),
            fixture,
            role,
            emit,
            answered: Mutex::new(HashMap::new()),
        })
    }

    fn event(&self, name: &str, station: &str, fields: Vec<(&str, Value)>) {
        if self.emit {
            event(name, Some(station), fields);
        }
    }

    /// The actions this end receives in its version.
    fn receives(&self) -> &'static [&'static str] {
        let a = actions(&self.version).expect("a supported version");
        if self.role == "csms" {
            a.from_station
        } else {
            a.from_csms
        }
    }

    /// Answers one CALL or takes one SEND of the peer.
    fn handle(&self, station: &str, request: IncomingRequest) -> Result<Box<RawValue>, CallError> {
        let id = request.id.to_string();
        let payload = parse(&request.payload);
        let action = request.action.clone();
        if request.kind == MessageKind::Send {
            self.event(
                "send-received",
                station,
                vec![
                    ("messageId", json!(id)),
                    ("action", json!(action)),
                    ("payload", payload),
                ],
            );
            return Ok(raw(&json!({})));
        }
        self.event(
            "call-received",
            station,
            vec![
                ("messageId", json!(id)),
                ("action", json!(action)),
                ("payload", payload.clone()),
            ],
        );
        let result = self.answer(&action, &request.payload, &payload);
        match &result {
            Ok(answer) => {
                self.answered
                    .lock()
                    .unwrap_or_else(|e| e.into_inner())
                    .insert((station.to_string(), id.clone()), action.clone());
                self.event(
                    "call-answered",
                    station,
                    vec![
                        ("messageId", json!(id)),
                        ("action", json!(action)),
                        ("payload", parse(answer)),
                    ],
                );
            }
            Err(e) => {
                let code = wire_code(&self.version, &e.code);
                self.event(
                    "call-answered",
                    station,
                    vec![
                        ("messageId", json!(id)),
                        ("action", json!(action)),
                        (
                            "error",
                            error_object(&code, &e.description, e.details.clone()),
                        ),
                    ],
                );
            }
        }
        result
    }

    fn answer(
        &self,
        action: &str,
        raw_payload: &RawValue,
        payload: &Value,
    ) -> Result<Box<RawValue>, CallError> {
        let Some(checked) = transcode(&self.version, action, raw_payload, false) else {
            return Err(CallError::not_implemented(action));
        };
        if !self.receives().contains(&action) {
            return Err(CallError::new(
                ErrorCode::NotSupported,
                format!("{action} is not sent to this role"),
            ));
        }
        if let Err(e) = checked {
            return Err(CallError::from(e));
        }
        match self.fixture.answer(self.role, action, payload) {
            None => Err(CallError::new(
                ErrorCode::NotImplemented,
                format!("the fixture has no answer for {action}"),
            )),
            Some(Answer::CallError { code, description }) => {
                Err(CallError::new(ErrorCode::parse(&code), description))
            }
            // Sent as it is: the peer is meant to refuse it with a CALLRESULTERROR.
            Some(Answer::Invalid(v)) => Ok(raw(&v)),
            Some(Answer::Payload(v)) => Ok(raw(&v)),
        }
    }

    /// Follows the events of one session (ocpp-kit publishes them on the
    /// session's channel; it does not call Handler::on_event) until it ends.
    pub fn watch(self: &Arc<Self>, station: String, handle: &Handle) {
        let mut events = handle.events();
        let endpoint = self.clone();
        tokio::spawn(async move {
            loop {
                match events.recv().await {
                    Ok(e) => endpoint.on_event(&station, &e),
                    Err(tokio::sync::broadcast::error::RecvError::Lagged(_)) => {}
                    Err(tokio::sync::broadcast::error::RecvError::Closed) => return,
                }
            }
        });
    }

    /// A CALLRESULTERROR the peer sent about one of this end's CALLRESULTs.
    fn on_event(&self, station: &str, event: &Event) {
        if let Event::ResultRejected { id, error } = event {
            let id = id.to_string();
            let action = self
                .answered
                .lock()
                .unwrap_or_else(|e| e.into_inner())
                .get(&(station.to_string(), id.clone()))
                .cloned();
            let code = wire_code(&self.version, &error.code);
            self.event(
                "call-result-error",
                station,
                vec![
                    ("messageId", json!(id)),
                    ("action", action.map(Value::String).unwrap_or(Value::Null)),
                    (
                        "error",
                        error_object(&code, &error.description, error.details.clone()),
                    ),
                ],
            );
        }
    }

    /// Sends one CALL (or SEND) with ocpp-kit and returns the result document.
    pub async fn execute(
        &self,
        station: &str,
        action: &str,
        payload: Value,
        timeout_ms: u64,
        handle: Option<Handle>,
    ) -> ResultDoc {
        let started = Instant::now();
        let v = self.version.clone();
        let done = |mut r: ResultDoc| {
            r.version = v.clone();
            r.operation = action.to_string();
            r.elapsed_ms = started.elapsed().as_millis();
            r
        };
        let acts = actions(&self.version).expect("a supported version");
        let (sender, sends) = if self.role == "csms" {
            ("CSMS", acts.from_csms)
        } else {
            ("station", acts.from_station)
        };
        let raw_payload = raw(&payload);
        let Some(checked) = transcode(&self.version, action, &raw_payload, false) else {
            return done(ResultDoc::failed(
                &v,
                action,
                payload,
                "unsupported",
                format!("ocpp-kit has no OCPP {v} action {action}"),
            ));
        };
        if !sends.contains(&action) {
            return done(ResultDoc::failed(
                &v,
                action,
                payload,
                "unsupported",
                format!("the {sender} does not send {action} in OCPP {v}"),
            ));
        }
        let request = match checked {
            Ok(r) => r,
            Err(e) => {
                return done(ResultDoc::failed(
                    &v,
                    action,
                    payload,
                    "invalid-request",
                    format!("ocpp-kit refused the request: {e}"),
                ));
            }
        };
        let Some(handle) = handle else {
            return done(ResultDoc::failed(
                &v,
                action,
                payload,
                "not-connected",
                "not connected",
            ));
        };
        let sent = parse(&request);
        let wait = Duration::from_millis(timeout_ms);
        let failure = |f: CallFailure, sent: &Value| -> ResultDoc {
            let (code, msg) = match f {
                CallFailure::Timeout => ("timeout", format!("no response within {timeout_ms} ms")),
                CallFailure::Disconnected | CallFailure::ShuttingDown => (
                    "connection-lost",
                    format!("the connection closed while waiting for the response: {f}"),
                ),
                other => ("failed", other.to_string()),
            };
            ResultDoc::failed(&v, action, sent.clone(), code, msg)
        };

        if acts.send.contains(&action) {
            // An OCPP 2.1 SEND (contract 0.6): ocpp-kit picks its message id
            // and does not say which.
            self.event(
                "send-sent",
                station,
                vec![
                    ("messageId", Value::Null),
                    ("action", json!(action)),
                    ("payload", sent.clone()),
                ],
            );
            return done(
                match tokio::time::timeout(
                    wait,
                    handle.call_raw(action, request, CallOptions::triggered()),
                )
                .await
                {
                    Ok(Ok(_)) => ResultDoc {
                        ok: true,
                        request: Some(sent),
                        ..Default::default()
                    },
                    Ok(Err(f)) => failure(f, &sent),
                    Err(_) => ResultDoc::failed(
                        &v,
                        action,
                        sent,
                        "timeout",
                        format!("not sent within {timeout_ms} ms"),
                    ),
                },
            );
        }

        self.event(
            "call-sent",
            station,
            vec![
                ("messageId", Value::Null),
                ("action", json!(action)),
                ("payload", sent.clone()),
            ],
        );
        let outcome: Result<Value, CallFailure> = if self.version == "2.1" {
            // Typed, so that ocpp-kit answers an invalid CALLRESULT with a
            // CALLRESULTERROR (Handle::call_with).
            let a = v2_1::Action::from_wire(action).expect("checked above");
            let o = DecodeOptions::default();
            let call = async {
                if self.role == "csms" {
                    match v2_1::CsmsRequest::decode(a, &request, &o) {
                        Ok(r) => typed21::call_csms(&handle, r, CallOptions::triggered()).await,
                        Err(e) => Some(Err(CallFailure::Rejected(CallError::from(e)))),
                    }
                } else {
                    match v2_1::CsRequest::decode(a, &request, &o) {
                        Ok(r) => typed21::call_station(&handle, r, CallOptions::triggered()).await,
                        Err(e) => Some(Err(CallFailure::Rejected(CallError::from(e)))),
                    }
                }
            };
            match tokio::time::timeout(wait, call).await {
                Ok(Some(r)) => r,
                Ok(None) => {
                    return done(ResultDoc::failed(
                        &v,
                        action,
                        sent,
                        "unsupported",
                        format!("ocpp-kit has no typed {action}"),
                    ));
                }
                Err(_) => Err(CallFailure::Timeout),
            }
        } else {
            match tokio::time::timeout(
                wait,
                handle.call_raw(action, request, CallOptions::triggered()),
            )
            .await
            {
                Ok(Ok(EngineAnswer::Result(result))) => {
                    match transcode(&self.version, action, &result, true) {
                        Some(Ok(_)) | None => Ok(parse(&result)),
                        Some(Err(e)) => {
                            let ce = CallError::from(e);
                            self.event(
                                "call-result",
                                station,
                                vec![
                                    ("messageId", Value::Null),
                                    ("action", json!(action)),
                                    ("payload", parse(&result)),
                                ],
                            );
                            return done(ResultDoc::failed(
                                &v,
                                action,
                                sent,
                                "invalid-response",
                                format!("ocpp-kit rejected the response: {ce}"),
                            ));
                        }
                    }
                }
                Ok(Ok(_)) => Ok(Value::Null),
                Ok(Err(f)) => Err(f),
                Err(_) => Err(CallFailure::Timeout),
            }
        };
        match outcome {
            Ok(response) => {
                self.event(
                    "call-result",
                    station,
                    vec![
                        ("messageId", Value::Null),
                        ("action", json!(action)),
                        ("payload", response.clone()),
                    ],
                );
                done(ResultDoc {
                    ok: true,
                    request: Some(sent),
                    response: Some(response),
                    ..Default::default()
                })
            }
            Err(CallFailure::Rejected(e)) if is_own_rejection(&e) => {
                let code = wire_code(&self.version, &e.code);
                let mut r = ResultDoc::failed(
                    &v,
                    action,
                    sent,
                    "invalid-response",
                    format!("ocpp-kit rejected the response: {e}"),
                );
                if self.version == "2.1" {
                    r.call_result_error =
                        Some(error_object(&code, &e.description, e.details.clone()));
                }
                done(r)
            }
            Err(CallFailure::Rejected(e)) => {
                let code = wire_code(&self.version, &e.code);
                let call_error = error_object(&code, &e.description, e.details.clone());
                self.event(
                    "call-result",
                    station,
                    vec![
                        ("messageId", Value::Null),
                        ("action", json!(action)),
                        ("error", call_error.clone()),
                    ],
                );
                let mut r =
                    ResultDoc::failed(&v, action, sent, "call-error", format!("CALLERROR {code}"));
                r.call_error = Some(call_error);
                done(r)
            }
            Err(f) => done(failure(f, &sent)),
        }
    }
}

/// The ocpp-kit Handler of an endpoint.
pub struct FixtureHandler(pub Arc<Endpoint>);

impl Handler for FixtureHandler {
    fn on_request(
        &self,
        ctx: Ctx,
        request: IncomingRequest,
    ) -> BoxFuture<'_, Result<Box<RawValue>, CallError>> {
        let e = self.0.clone();
        Box::pin(async move { e.handle(ctx.identity().as_str(), request) })
    }
}
