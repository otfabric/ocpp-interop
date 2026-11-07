// SPDX-License-Identifier: MIT
//! Fixture loading and validation (docs/FIXTURES.md): the schema of
//! fixtures/schema/ocpp-fixture.schema.json, checked with the jsonschema
//! crate, and the cross-field rules of scripts/validate-fixtures.py, written
//! out again here. Answers: payloads, derived answers, "${now}", CALLERRORs
//! and the invalid answers of OCPP 2.1.

use std::collections::{HashMap, HashSet};
use std::path::Path;
use std::sync::Mutex;

use ocpp_kit::Version;
use ocpp_kit::rpc::ErrorCode;
use serde_json::{Map, Value, json};

use crate::actions::actions;
use crate::out::now_seconds;

pub const FIXTURES_DIR: &str = "/fixtures";
pub const SCHEMA: &str = "/fixtures/schema/ocpp-fixture.schema.json";

pub fn version_dir(version: &str) -> Option<&'static str> {
    match version {
        "1.6" => Some("v16"),
        "2.0.1" => Some("v201"),
        "2.1" => Some("v21"),
        _ => None,
    }
}

pub fn default_path(version: &str) -> String {
    format!(
        "{FIXTURES_DIR}/{}/baseline/fixture.json",
        version_dir(version).unwrap_or("v16")
    )
}

/// Why a fixture cannot be used: exit 4 (invalid) or 5 (an answer this
/// adapter cannot give).
pub enum FixtureError {
    Invalid(String),
    Unsupported(String),
}

pub enum Answer {
    Payload(Value),
    /// invalidAnswers (2.1): sent as it is, to provoke a CALLRESULTERROR.
    Invalid(Value),
    CallError {
        code: String,
        description: String,
    },
}

pub struct Fixture {
    pub name: String,
    pub version: String,
    pub doc: Value,
    pub first_station: String,
    pub stations: HashMap<String, Value>,
    variables: Mutex<Vec<Value>>,
}

const DERIVED: [(&str, &[&str]); 3] = [
    ("idTokens", &["Authorize", "StartTransaction"]),
    ("configuration", &["GetConfiguration"]),
    ("variables", &["GetVariables", "SetVariables"]),
];

fn id_tag_max(version: &str) -> usize {
    match version {
        "1.6" => 20,
        "2.0.1" => 36,
        _ => 255,
    }
}

fn str_of<'a>(v: &'a Value, key: &str) -> &'a str {
    v.get(key).and_then(Value::as_str).unwrap_or("")
}

fn duplicates<I: IntoIterator<Item = String>>(values: I) -> Vec<String> {
    let mut seen = HashSet::new();
    let mut dup = Vec::new();
    for v in values {
        if !seen.insert(v.clone()) && !dup.contains(&v) {
            dup.push(v);
        }
    }
    dup
}

fn semantic_errors(fx: &Value, path: &Path, errors: &mut Vec<String>) {
    let version = str_of(fx, "ocppVersion");
    let dir = path
        .parent()
        .and_then(Path::file_name)
        .and_then(|s| s.to_str())
        .unwrap_or("");
    let vdir = path
        .parent()
        .and_then(Path::parent)
        .and_then(Path::file_name)
        .and_then(|s| s.to_str())
        .unwrap_or("");
    if str_of(fx, "name") != dir {
        errors.push(format!(
            "name {:?} is not the directory name {dir:?}",
            str_of(fx, "name")
        ));
    }
    if version_dir(version) != Some(vdir) {
        errors.push(format!(
            "ocppVersion {version:?} does not belong in {vdir}/"
        ));
    }
    let Some(acts) = actions(version) else {
        errors.push(format!(
            "ocppVersion {version:?}: no action list, so the fixture cannot be checked"
        ));
        return;
    };
    let empty = vec![];
    let list = |key: &str| {
        fx.get(key)
            .and_then(Value::as_array)
            .unwrap_or(&empty)
            .clone()
    };
    for d in duplicates(list("stations").iter().map(|s| str_of(s, "id").to_string())) {
        errors.push(format!("stations: identity {d:?} appears twice"));
    }
    for d in duplicates(
        list("idTokens")
            .iter()
            .map(|t| str_of(t, "idToken").to_string()),
    ) {
        errors.push(format!("idTokens: {d:?} appears twice"));
    }
    for t in list("idTokens") {
        let id = str_of(&t, "idToken");
        if id.chars().count() > id_tag_max(version) {
            errors.push(format!(
                "idTokens: {id:?} is longer than {} characters",
                id_tag_max(version)
            ));
        }
        if version != "1.6" && t.get("type").is_none() {
            errors.push(format!(
                "idTokens: {id:?} has no type, which OCPP 2.x requires"
            ));
        }
    }
    for d in duplicates(
        list("configuration")
            .iter()
            .map(|c| str_of(c, "key").to_string()),
    ) {
        errors.push(format!("configuration: key {d:?} appears twice"));
    }
    let named = |v: &Value, k: &str| {
        let n = v.get(k).cloned().unwrap_or(Value::Null);
        format!("{}\u{0}{}", str_of(&n, "name"), str_of(&n, "instance"))
    };
    for d in duplicates(
        list("variables")
            .iter()
            .map(|v| format!("{}\u{0}{}", named(v, "component"), named(v, "variable"))),
    ) {
        errors.push(format!(
            "variables: {} appears twice",
            d.replace('\u{0}', "/")
        ));
    }
    for s in list("stations") {
        let evses = s
            .get("evses")
            .and_then(Value::as_array)
            .cloned()
            .unwrap_or_default();
        for d in duplicates(
            evses
                .iter()
                .map(|e| e.get("id").map(Value::to_string).unwrap_or_default()),
        ) {
            errors.push(format!(
                "stations {:?}: EVSE {d} appears twice",
                str_of(&s, "id")
            ));
        }
    }
    let receives = |role: &str| -> &'static [&'static str] {
        if role == "csms" {
            acts.from_station
        } else {
            acts.from_csms
        }
    };
    if let Some(answers) = fx.get("answers").and_then(Value::as_object) {
        for (role, table) in answers {
            for (action, answer) in table.as_object().cloned().unwrap_or_default() {
                if !receives(role).contains(&action.as_str()) {
                    errors.push(format!("answers.{role}.{action}: the {role} does not receive {action} in OCPP {version}"));
                } else if acts.send.contains(&action.as_str()) {
                    errors.push(format!("answers.{role}.{action}: {action} is a SEND in OCPP {version}, which has no answer"));
                }
                if let Some(derived) = answer.get("derived").and_then(Value::as_str) {
                    let fits = DERIVED
                        .iter()
                        .any(|(d, a)| *d == derived && a.contains(&action.as_str()));
                    if !fits {
                        errors.push(format!(
                            "answers.{role}.{action}: cannot be derived from {derived}"
                        ));
                    }
                    if fx
                        .get(derived)
                        .and_then(Value::as_array)
                        .is_none_or(|a| a.is_empty())
                    {
                        errors.push(format!(
                            "answers.{role}.{action}: derived from {derived}, which is empty"
                        ));
                    }
                }
            }
        }
    }
    if let Some(invalid) = fx.get("invalidAnswers").and_then(Value::as_object) {
        for (role, table) in invalid {
            for action in table
                .as_object()
                .map(|t| t.keys().cloned().collect::<Vec<_>>())
                .unwrap_or_default()
            {
                if version != "2.1" {
                    errors.push(format!("invalidAnswers.{role}.{action}: invalid answers are for OCPP 2.1 only (CALLRESULTERROR)"));
                } else if !receives(role).contains(&action.as_str()) {
                    errors.push(format!("invalidAnswers.{role}.{action}: the {role} does not receive {action} in OCPP {version}"));
                } else if acts.send.contains(&action.as_str()) {
                    errors.push(format!("invalidAnswers.{role}.{action}: {action} is a SEND in OCPP {version}, which has no answer"));
                }
            }
        }
    }
    if let Some(samples) = fx.get("samples").and_then(Value::as_object) {
        for (name, sample) in samples {
            let from = str_of(sample, "from");
            let sends: &[&str] = if from == "station" {
                acts.from_station
            } else {
                acts.from_csms
            };
            if !sends.contains(&str_of(sample, "action")) {
                errors.push(format!(
                    "samples.{name}: the {from} does not send {} in OCPP {version}",
                    str_of(sample, "action")
                ));
            }
        }
    }
}

/// A CALLERROR code ocpp-kit can send in the version: one of its codes, in
/// the version's spelling.
pub fn valid_error_code(version: &str, code: &str) -> bool {
    let Ok(v) = version.parse::<Version>() else {
        return false;
    };
    let parsed = ErrorCode::parse(code);
    !matches!(parsed, ErrorCode::Other(_)) && parsed.as_wire(v) == code
}

pub fn load(path: &str, version: &str) -> Result<Fixture, FixtureError> {
    let text =
        std::fs::read_to_string(path).map_err(|e| FixtureError::Invalid(format!("{path}: {e}")))?;
    let doc: Value =
        serde_json::from_str(&text).map_err(|e| FixtureError::Invalid(format!("{path}: {e}")))?;
    let schema_text = std::fs::read_to_string(SCHEMA).map_err(|e| {
        FixtureError::Invalid(format!("cannot read the fixture schema {SCHEMA}: {e}"))
    })?;
    let schema: Value = serde_json::from_str(&schema_text)
        .map_err(|e| FixtureError::Invalid(format!("the fixture schema {SCHEMA}: {e}")))?;
    let validator = jsonschema::draft202012::new(&schema)
        .map_err(|e| FixtureError::Invalid(format!("the fixture schema {SCHEMA}: {e}")))?;
    let mut errors: Vec<String> = validator
        .iter_errors(&doc)
        .map(|e| {
            let at = e.instance_path.to_string();
            let msg = e.to_string();
            let msg: String = msg.chars().take(200).collect();
            format!(
                "{}: {msg}",
                if at.is_empty() { "(root)".into() } else { at }
            )
        })
        .collect();
    let abs = std::fs::canonicalize(path).unwrap_or_else(|_| Path::new(path).to_path_buf());
    if errors.is_empty() {
        semantic_errors(&doc, &abs, &mut errors);
    }
    if errors.is_empty() && str_of(&doc, "ocppVersion") != version {
        errors.push(format!(
            "ocppVersion is {:?}, but --version is {version}",
            str_of(&doc, "ocppVersion")
        ));
    }
    if !errors.is_empty() {
        return Err(FixtureError::Invalid(format!(
            "{path}: {}",
            errors.join("; ")
        )));
    }
    // An answer ocpp-kit can give: a CALLERROR of an OCPP-J code of the version.
    for role in ["csms", "station"] {
        if let Some(table) = doc
            .pointer(&format!("/answers/{role}"))
            .and_then(Value::as_object)
        {
            for (action, answer) in table {
                if let Some(code) = answer.pointer("/callError/code").and_then(Value::as_str) {
                    if !valid_error_code(version, code) {
                        return Err(FixtureError::Unsupported(format!(
                            "{path}: answers.{role}.{action}: {code:?} is not an OCPP-J error code of OCPP {version}"
                        )));
                    }
                }
            }
        }
    }
    let stations: HashMap<String, Value> = doc
        .get("stations")
        .and_then(Value::as_array)
        .map(|a| {
            a.iter()
                .map(|s| (str_of(s, "id").to_string(), s.clone()))
                .collect()
        })
        .unwrap_or_default();
    let first_station = doc
        .pointer("/stations/0/id")
        .and_then(Value::as_str)
        .unwrap_or("")
        .to_string();
    let variables = doc
        .get("variables")
        .and_then(Value::as_array)
        .cloned()
        .unwrap_or_default();
    Ok(Fixture {
        name: str_of(&doc, "name").to_string(),
        version: version.to_string(),
        first_station,
        stations,
        variables: Mutex::new(variables),
        doc,
    })
}

fn substitute(v: &Value) -> Value {
    match v {
        Value::String(s) if s == "${now}" => json!(now_seconds()),
        Value::Object(o) => {
            Value::Object(o.iter().map(|(k, x)| (k.clone(), substitute(x))).collect())
        }
        Value::Array(a) => Value::Array(a.iter().map(substitute).collect()),
        other => other.clone(),
    }
}

fn same(a: &Value, b: &Value, key: &str) -> bool {
    str_of(a, key).eq_ignore_ascii_case(str_of(b, key))
}

impl Fixture {
    /// The password of a station; one no station has for an identity the
    /// fixture does not list (contract 0.7: the server refuses the identity).
    pub fn password(&self, station: &str) -> String {
        self.stations
            .get(station)
            .map(|s| str_of(s, "password").to_string())
            .unwrap_or_else(
                || // 16 characters: the shortest password OCPP allows, and within the
            // 20 of OCPP 1.6 (Java-OCA-OCPP refuses a longer one before
            // anything else).
            "OTFABRIC-NO-PASS".to_string(),
            )
    }

    /// The answer of `role` to a CALL; None when the fixture has none.
    pub fn answer(&self, role: &str, action: &str, request: &Value) -> Option<Answer> {
        if let Some(e) = self
            .doc
            .pointer(&format!("/invalidAnswers/{role}/{action}"))
        {
            return Some(Answer::Invalid(substitute(&e["payload"])));
        }
        let entry = self.doc.pointer(&format!("/answers/{role}/{action}"))?;
        if let Some(ce) = entry.get("callError") {
            return Some(Answer::CallError {
                code: str_of(ce, "code").to_string(),
                description: str_of(ce, "description").to_string(),
            });
        }
        let Some(derived) = entry.get("derived").and_then(Value::as_str) else {
            return Some(Answer::Payload(substitute(&entry["payload"])));
        };
        let mut answer = match derived {
            "idTokens" => self.id_token_answer(request),
            "configuration" => self.configuration_answer(request),
            _ => self.variables_answer(action, request),
        };
        if let (Some(extra), Value::Object(o)) =
            (entry.get("payload").and_then(Value::as_object), &mut answer)
        {
            for (k, v) in extra {
                o.insert(k.clone(), substitute(v));
            }
        }
        Some(Answer::Payload(answer))
    }

    fn id_token_answer(&self, request: &Value) -> Value {
        let tokens = self
            .doc
            .get("idTokens")
            .and_then(Value::as_array)
            .cloned()
            .unwrap_or_default();
        if self.version == "1.6" {
            let tag = str_of(request, "idTag");
            let status = tokens
                .iter()
                .find(|t| str_of(t, "idToken") == tag)
                .map(|t| str_of(t, "status"))
                .unwrap_or("Invalid");
            return json!({"idTagInfo": {"status": status}});
        }
        let token = request.get("idToken").cloned().unwrap_or(Value::Null);
        let status = tokens
            .iter()
            .find(|t| {
                str_of(t, "idToken") == str_of(&token, "idToken")
                    && str_of(t, "type") == str_of(&token, "type")
            })
            .map(|t| str_of(t, "status"))
            .unwrap_or("Invalid");
        json!({"idTokenInfo": {"status": status}})
    }

    fn configuration_answer(&self, request: &Value) -> Value {
        let configuration = self
            .doc
            .get("configuration")
            .and_then(Value::as_array)
            .cloned()
            .unwrap_or_default();
        let wanted: Vec<String> = match request.get("key").and_then(Value::as_array) {
            Some(keys) if !keys.is_empty() => keys
                .iter()
                .map(|k| k.as_str().unwrap_or("").to_string())
                .collect(),
            _ => configuration
                .iter()
                .map(|c| str_of(c, "key").to_string())
                .collect(),
        };
        let mut known = Vec::new();
        let mut unknown = Vec::new();
        for key in wanted {
            match configuration.iter().find(|c| str_of(c, "key").eq_ignore_ascii_case(&key)) {
                Some(item) => known.push(json!({"key": item["key"], "readonly": item["readonly"], "value": item["value"]})),
                None => unknown.push(json!(key)),
            }
        }
        let mut o = Map::new();
        o.insert("configurationKey".into(), Value::Array(known));
        if !unknown.is_empty() {
            o.insert("unknownKey".into(), Value::Array(unknown));
        }
        Value::Object(o)
    }

    fn variables_answer(&self, action: &str, request: &Value) -> Value {
        let get = action == "GetVariables";
        let key = if get {
            "getVariableData"
        } else {
            "setVariableData"
        };
        let mut variables = self.variables.lock().unwrap_or_else(|e| e.into_inner());
        let mut results = Vec::new();
        for item in request
            .get(key)
            .and_then(Value::as_array)
            .cloned()
            .unwrap_or_default()
        {
            let component = item.get("component").cloned().unwrap_or(Value::Null);
            let variable = item.get("variable").cloned().unwrap_or(Value::Null);
            let mut status = "UnknownComponent";
            let mut found: Option<usize> = None;
            for (i, v) in variables.iter().enumerate() {
                let (vc, vv) = (&v["component"], &v["variable"]);
                if same(vc, &component, "name") && same(vc, &component, "instance") {
                    if status == "UnknownComponent" {
                        status = "UnknownVariable";
                    }
                    if same(vv, &variable, "name") && same(vv, &variable, "instance") {
                        found = Some(i);
                        status = "";
                        break;
                    }
                }
            }
            let attribute_type = item.get("attributeType");
            if status.is_empty() && attribute_type.is_some_and(|t| t != "Actual") {
                status = "NotSupportedAttributeType";
            }
            let forbidden = if get { "WriteOnly" } else { "ReadOnly" };
            if let (true, Some(i)) = (status.is_empty(), found) {
                if str_of(&variables[i], "mutability") == forbidden {
                    status = "Rejected";
                }
            }
            if let (true, false, Some(i)) = (status.is_empty(), get, found) {
                variables[i]["value"] = item.get("attributeValue").cloned().unwrap_or(Value::Null);
            }
            let mut result = Map::new();
            result.insert("component".into(), component);
            result.insert("variable".into(), variable);
            if let Some(t) = attribute_type {
                result.insert("attributeType".into(), t.clone());
            }
            if status.is_empty() {
                result.insert("attributeStatus".into(), json!("Accepted"));
                if get {
                    result.insert(
                        "attributeValue".into(),
                        variables[found.unwrap()]["value"].clone(),
                    );
                }
            } else {
                result.insert("attributeStatus".into(), json!(status));
            }
            results.push(Value::Object(result));
        }
        if get {
            json!({"getVariableResult": results})
        } else {
            json!({"setVariableResult": results})
        }
    }
}
