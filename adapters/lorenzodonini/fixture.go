// SPDX-License-Identifier: MIT
package main

// Fixture loading, validation and answers (docs/FIXTURES.md). The checks are
// those of fixtures/schema/ocpp-fixture.schema.json and of
// scripts/validate-fixtures.py that matter for OCPP 1.6 and 2.0.1, written
// out by hand so that the image needs no JSON Schema library. None of this is
// OCPP protocol logic: it decides what the fixture answers, not how.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/lorenzodonini/ocpp-go/ocpp"
	"github.com/lorenzodonini/ocpp-go/ocppj"
)

const fixturesDir = "/fixtures"

var versionDirs = map[string]string{"1.6": "v16", "2.0.1": "v201", "2.1": "v21"}

var (
	identityPattern = regexp.MustCompile(`^[A-Za-z0-9*\-_=:+|@.]{1,48}$`)
	tokenStatuses   = set("Accepted", "Blocked", "Expired", "Invalid", "ConcurrentTx", "NoCredit",
		"NotAllowedTypeEVSE", "NotAtThisLocation", "NotAtThisTime", "Unknown")
)

func set(values ...string) map[string]bool {
	m := map[string]bool{}
	for _, v := range values {
		m[v] = true
	}
	return m
}

func defaultFixturePath(version string) string {
	return fixturesDir + "/" + versionDirs[version] + "/baseline/fixture.json"
}

// callErrorAnswer: the fixture answers with this CALLERROR.
type callErrorAnswer struct{ code, description string }

type fixture struct {
	doc          map[string]any
	name         string
	version      string
	stations     map[string]map[string]any
	firstStation string
	idTokens     map[string]string // 1.6: idTag -> status
	typedTokens  map[string]string // 2.x: idToken + "\x00" + type -> status
	mu           sync.Mutex
	variables    []map[string]any // 2.x device model; SetVariables changes values for the process lifetime
}

func (f *fixture) answers(role string) map[string]any {
	return f.doc["answers"].(map[string]any)[role].(map[string]any)
}

func (f *fixture) password(station string) string {
	s, ok := f.stations[station]
	if !ok {
		return ""
	}
	p, _ := s["password"].(string)
	return p
}

// answer is the answer of role to a CALL: a payload, a callErrorAnswer, or
// nil when the fixture has none (NotImplemented).
func (f *fixture) answer(role, action string, request map[string]any) any {
	f.mu.Lock()
	defer f.mu.Unlock()
	entry, ok := f.answers(role)[action].(map[string]any)
	if !ok {
		return nil
	}
	if ce, ok := entry["callError"].(map[string]any); ok {
		return callErrorAnswer{str(ce["code"]), str(ce["description"])}
	}
	derived, isDerived := entry["derived"].(string)
	if !isDerived {
		return substitute(deepCopy(entry["payload"])).(map[string]any)
	}
	var answer map[string]any
	switch derived {
	case "idTokens":
		if f.version == "1.6" {
			status, ok := f.idTokens[str(request["idTag"])]
			if !ok {
				status = "Invalid"
			}
			answer = map[string]any{"idTagInfo": map[string]any{"status": status}}
		} else {
			token, _ := request["idToken"].(map[string]any)
			status, ok := f.typedTokens[str(token["idToken"])+"\x00"+str(token["type"])]
			if !ok {
				status = "Invalid"
			}
			answer = map[string]any{"idTokenInfo": map[string]any{"status": status}}
		}
	case "configuration":
		answer = f.configurationAnswer(request)
	case "variables":
		answer = f.variablesAnswer(action, request)
	}
	if extra, ok := entry["payload"].(map[string]any); ok {
		for k, v := range substitute(deepCopy(extra)).(map[string]any) {
			answer[k] = v
		}
	}
	return answer
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

func sameName(a, b map[string]any, key string) bool {
	return strings.EqualFold(str(a[key]), str(b[key]))
}

// configurationAnswer: GetConfiguration, the requested keys or all of them;
// keys compare case-insensitively (CiString50Type).
func (f *fixture) configurationAnswer(request map[string]any) map[string]any {
	configuration, _ := f.doc["configuration"].([]any)
	var wanted []string
	if keys, ok := request["key"].([]any); ok && len(keys) > 0 {
		for _, k := range keys {
			wanted = append(wanted, str(k))
		}
	} else {
		for _, c := range configuration {
			wanted = append(wanted, str(c.(map[string]any)["key"]))
		}
	}
	known, unknown := []any{}, []any{}
	for _, key := range wanted {
		var item map[string]any
		for _, c := range configuration {
			if strings.EqualFold(str(c.(map[string]any)["key"]), key) {
				item = c.(map[string]any)
				break
			}
		}
		if item == nil {
			unknown = append(unknown, key)
			continue
		}
		known = append(known, map[string]any{"key": item["key"], "readonly": item["readonly"], "value": item["value"]})
	}
	answer := map[string]any{"configurationKey": known}
	if len(unknown) > 0 {
		answer["unknownKey"] = unknown
	}
	return answer
}

// variablesAnswer: GetVariables and SetVariables, a result per requested
// component and variable. Names and instances compare case-insensitively.
func (f *fixture) variablesAnswer(action string, request map[string]any) map[string]any {
	get := action == "GetVariables"
	key := "setVariableData"
	if get {
		key = "getVariableData"
	}
	items, _ := request[key].([]any)
	results := []any{}
	for _, it := range items {
		item, _ := it.(map[string]any)
		component, _ := item["component"].(map[string]any)
		variable, _ := item["variable"].(map[string]any)
		status := "UnknownComponent"
		var entry map[string]any
		for _, v := range f.variables {
			vc := v["component"].(map[string]any)
			vv := v["variable"].(map[string]any)
			if sameName(vc, component, "name") && sameName(vc, component, "instance") {
				if status == "UnknownComponent" {
					status = "UnknownVariable"
				}
				if sameName(vv, variable, "name") && sameName(vv, variable, "instance") {
					entry, status = v, ""
					break
				}
			}
		}
		attributeType, hasType := item["attributeType"]
		if status == "" && hasType && attributeType != "Actual" {
			status = "NotSupportedAttributeType"
		}
		forbidden := "ReadOnly"
		if get {
			forbidden = "WriteOnly"
		}
		if status == "" && entry["mutability"] == forbidden {
			status = "Rejected"
		}
		if status == "" && !get {
			entry["value"] = item["attributeValue"]
		}
		result := map[string]any{"component": item["component"], "variable": item["variable"]}
		if hasType {
			result["attributeType"] = attributeType
		}
		if status == "" {
			result["attributeStatus"] = "Accepted"
			if get {
				result["attributeValue"] = entry["value"]
			}
		} else {
			result["attributeStatus"] = status
		}
		results = append(results, result)
	}
	if get {
		return map[string]any{"getVariableResult": results}
	}
	return map[string]any{"setVariableResult": results}
}

// substitute replaces "${now}" with the current UTC time, RFC 3339, whole seconds.
func substitute(v any) any {
	switch t := v.(type) {
	case string:
		if t == "${now}" {
			return time.Now().UTC().Format("2006-01-02T15:04:05Z")
		}
		return t
	case map[string]any:
		out := map[string]any{}
		for k, x := range t {
			out[k] = substitute(x)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, x := range t {
			out[i] = substitute(x)
		}
		return out
	default:
		return v
	}
}

func deepCopy(v any) any {
	data, _ := json.Marshal(v)
	var out any
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	_ = dec.Decode(&out)
	return out
}

// ---- loading and validation ----

func loadFixture(path, version string) (*fixture, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fixtureError{fmt.Sprintf("%s: %v", path, err)}
	}
	var doc map[string]any
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&doc); err != nil || doc == nil {
		return nil, fixtureError{fmt.Sprintf("%s: not a JSON object: %v", path, err)}
	}
	var errs []string
	validate(doc, path, version, &errs)
	if len(errs) > 0 {
		return nil, fixtureError{fmt.Sprintf("%s: %s", path, strings.Join(errs, "; "))}
	}
	f := &fixture{doc: doc, name: str(doc["name"]), version: version, stations: map[string]map[string]any{},
		idTokens: map[string]string{}, typedTokens: map[string]string{}}
	for i, s := range doc["stations"].([]any) {
		st := s.(map[string]any)
		f.stations[str(st["id"])] = st
		if i == 0 {
			f.firstStation = str(st["id"])
		}
	}
	for _, t := range doc["idTokens"].([]any) {
		tok := t.(map[string]any)
		f.idTokens[str(tok["idToken"])] = str(tok["status"])
		if typ, ok := tok["type"].(string); ok {
			f.typedTokens[str(tok["idToken"])+"\x00"+typ] = str(tok["status"])
		}
	}
	if vars, ok := doc["variables"].([]any); ok {
		for _, v := range deepCopy(vars).([]any) {
			f.variables = append(f.variables, v.(map[string]any))
		}
	}
	// A callError answer must be one ocpp-go can send: its CALLERROR
	// validation accepts the error codes of the OCPP-J specification only.
	for _, role := range []string{"csms", "station"} {
		for action, a := range f.answers(role) {
			if ce, ok := a.(map[string]any)["callError"].(map[string]any); ok {
				probe := ocppj.CallError{MessageTypeId: ocppj.CALL_ERROR, UniqueId: "probe",
					ErrorCode: ocpp.ErrorCode(str(ce["code"])), ErrorDescription: str(ce["description"])}
				if ocppj.Validate.Struct(probe) != nil {
					return nil, unsupportedError{fmt.Sprintf("%s: answers.%s.%s: ocpp-go cannot send the CALLERROR code %q",
						path, role, action, str(ce["code"]))}
				}
			}
		}
	}
	return f, nil
}

func isString(o map[string]any, key string) bool {
	_, ok := o[key].(string)
	return ok
}

func intOf(o map[string]any, key string) (int64, bool) {
	n, ok := o[key].(json.Number)
	if !ok {
		return 0, false
	}
	i, err := n.Int64()
	return i, err == nil
}

func validate(doc map[string]any, path, version string, errs *[]string) {
	add := func(format string, args ...any) { *errs = append(*errs, fmt.Sprintf(format, args...)) }
	allowed := set("schemaVersion", "name", "ocppVersion", "description", "stations", "idTokens",
		"configuration", "variables", "answers", "samples")
	for k := range doc {
		if !allowed[k] {
			add("unknown property %s", k)
		}
	}
	if str(doc["schemaVersion"]) != "1.0" {
		add(`schemaVersion must be "1.0"`)
	}
	if !fixtureName.MatchString(str(doc["name"])) {
		add("name is missing or not a fixture name")
	}
	if str(doc["description"]) == "" {
		add("description is missing")
	}
	fv := str(doc["ocppVersion"])
	if _, ok := versionDirs[fv]; !ok {
		add("ocppVersion must be 1.6, 2.0.1 or 2.1")
		return
	}
	if fv != version {
		add("ocppVersion is %s, but --version is %s", fv, version)
		return
	}
	info, ok := versions[version]
	if !ok {
		add("OCPP %s fixtures cannot be checked by this adapter", version)
		return
	}
	v16 := version == "1.6"
	abs, _ := filepath.Abs(path)
	dir := filepath.Dir(abs)
	if filepath.Base(dir) != str(doc["name"]) {
		add("name is not the directory name")
	}
	if filepath.Base(filepath.Dir(dir)) != versionDirs[version] {
		add("ocppVersion %s does not belong in this directory", version)
	}
	if v16 && doc["variables"] != nil {
		add("variables are for OCPP 2.x")
	}
	if !v16 && doc["configuration"] != nil {
		add("configuration is for OCPP 1.6")
	}

	stations, _ := doc["stations"].([]any)
	if len(stations) == 0 {
		add("stations must be a non-empty array")
	}
	ids := map[string]bool{}
	for _, s := range stations {
		st, ok := s.(map[string]any)
		if !ok {
			add("stations: not an object")
			continue
		}
		for _, key := range []string{"id", "vendor", "model", "serialNumber", "firmwareVersion", "password"} {
			if !isString(st, key) {
				add("stations: %s is missing", key)
			}
		}
		id := str(st["id"])
		if !identityPattern.MatchString(id) {
			add("stations: identity %q is not valid", id)
		}
		if ids[id] {
			add("stations: identity %s appears twice", id)
		}
		ids[id] = true
		if n := len(str(st["password"])); n < 16 || n > 40 {
			add("stations: password must have 16 to 40 characters")
		}
		if v16 {
			if n, ok := intOf(st, "connectors"); !ok || n < 1 || n > 16 {
				add("stations: connectors must be 1..16 in OCPP 1.6")
			}
			if st["evses"] != nil {
				add("stations: evses are for OCPP 2.x")
			}
			continue
		}
		if st["connectors"] != nil {
			add("stations: connectors are for OCPP 1.6")
		}
		evses, _ := st["evses"].([]any)
		if len(evses) == 0 {
			add("stations: evses must be a non-empty array in OCPP 2.x")
		}
		seen := map[int64]bool{}
		for _, e := range evses {
			evse, _ := e.(map[string]any)
			n, ok := intOf(evse, "id")
			connectors, _ := evse["connectors"].([]any)
			if !ok || n < 1 || len(connectors) == 0 {
				add("stations: an EVSE needs an id of 1 or more and connectors")
				continue
			}
			if seen[n] {
				add("stations: EVSE %d appears twice", n)
			}
			seen[n] = true
		}
	}

	tokens, _ := doc["idTokens"].([]any)
	if len(tokens) == 0 {
		add("idTokens must be a non-empty array")
	}
	seenTokens := map[string]bool{}
	for _, t := range tokens {
		tok, _ := t.(map[string]any)
		if !isString(tok, "idToken") || !isString(tok, "status") {
			add("idTokens: idToken and status are required")
			continue
		}
		id := str(tok["idToken"])
		max := 36
		if v16 {
			max = 20
		}
		if len(id) == 0 || len(id) > max {
			add("idTokens: %s must have 1 to %d characters in OCPP %s", id, max, version)
		}
		if !v16 && !isString(tok, "type") {
			add("idTokens: %s has no type, which OCPP 2.x requires", id)
		}
		if seenTokens[id] {
			add("idTokens: %s appears twice", id)
		}
		seenTokens[id] = true
		if !tokenStatuses[str(tok["status"])] {
			add("idTokens: %s has an unknown status", id)
		}
	}

	if v16 {
		configuration, ok := doc["configuration"].([]any)
		if !ok {
			add("configuration is required in OCPP 1.6")
		}
		keys := map[string]bool{}
		for _, c := range configuration {
			item, _ := c.(map[string]any)
			_, isBool := item["readonly"].(bool)
			if !isString(item, "key") || !isString(item, "value") || !isBool {
				add("configuration: key, value and readonly are required")
				continue
			}
			if len(str(item["key"])) > 50 || len(str(item["value"])) > 500 {
				add("configuration: %s is too long", str(item["key"]))
			}
			if keys[str(item["key"])] {
				add("configuration: key %s appears twice", str(item["key"]))
			}
			keys[str(item["key"])] = true
		}
	} else {
		variables, ok := doc["variables"].([]any)
		if !ok {
			add("variables are required in OCPP 2.x")
		}
		seen := map[string]bool{}
		for _, v := range variables {
			item, _ := v.(map[string]any)
			c, cok := named(item["component"])
			n, nok := named(item["variable"])
			if !cok || !nok || !isString(item, "value") ||
				!set("ReadOnly", "WriteOnly", "ReadWrite")[str(item["mutability"])] {
				add("variables: component, variable, value and mutability are required")
				continue
			}
			if len(str(item["value"])) > 2500 {
				add("variables: a value is longer than 2500 characters")
			}
			key := strings.Join([]string{str(c["name"]), str(c["instance"]), str(n["name"]), str(n["instance"])}, "\x00")
			if seen[key] {
				add("variables: %s/%s appears twice", str(c["name"]), str(n["name"]))
			}
			seen[key] = true
		}
	}

	answers, ok := doc["answers"].(map[string]any)
	if !ok {
		add("answers is required")
	}
	for _, role := range []string{"csms", "station"} {
		table, ok := answers[role].(map[string]any)
		if !ok {
			add("answers.%s is required", role)
			continue
		}
		receives := info.fromStation
		if role == "station" {
			receives = info.fromCSMS
		}
		for action, a := range table {
			where := "answers." + role + "." + action
			if !receives[action] {
				add("%s: the %s does not receive %s in OCPP %s", where, role, action, version)
			}
			e, ok := a.(map[string]any)
			if !ok {
				add("%s: not an object", where)
				continue
			}
			_, hasPayload := e["payload"]
			_, hasDerived := e["derived"]
			_, hasError := e["callError"]
			forms := 0
			for _, b := range []bool{hasError, hasDerived, hasPayload && !hasDerived} {
				if b {
					forms++
				}
			}
			if forms != 1 {
				add("%s: must be exactly one of payload, derived or callError", where)
				continue
			}
			if hasPayload {
				if _, ok := e["payload"].(map[string]any); !ok {
					add("%s: payload must be an object", where)
				}
			}
			if hasError {
				ce, _ := e["callError"].(map[string]any)
				if !isString(ce, "code") || !isString(ce, "description") {
					add("%s: callError needs code and description", where)
				}
			}
			if hasDerived {
				derived := str(e["derived"])
				fits := (derived == "idTokens" && (action == "Authorize" || action == "StartTransaction")) ||
					(derived == "configuration" && action == "GetConfiguration") ||
					(derived == "variables" && (action == "GetVariables" || action == "SetVariables"))
				if !fits {
					add("%s: cannot be derived from %s", where, derived)
				} else if table, _ := doc[derived].([]any); len(table) == 0 {
					add("%s: derived from %s, which is empty", where, derived)
				}
			}
		}
	}

	samples, ok := doc["samples"].(map[string]any)
	if !ok {
		add("samples is required")
	}
	for name, s := range samples {
		sample, _ := s.(map[string]any)
		_, payloadOK := sample["payload"].(map[string]any)
		if !isString(sample, "action") || !isString(sample, "from") || !payloadOK || !actionName.MatchString(str(sample["action"])) {
			add("samples.%s: action, from and payload are required", name)
			continue
		}
		sends := map[string]bool{}
		switch str(sample["from"]) {
		case "station":
			sends = info.fromStation
		case "csms":
			sends = info.fromCSMS
		}
		if !sends[str(sample["action"])] {
			add("samples.%s: the %s does not send %s", name, str(sample["from"]), str(sample["action"]))
		}
	}
}

func named(v any) (map[string]any, bool) {
	o, ok := v.(map[string]any)
	if !ok {
		return nil, false
	}
	for k := range o {
		if k != "name" && k != "instance" {
			return nil, false
		}
	}
	if !isString(o, "name") || len(str(o["name"])) > 50 {
		return nil, false
	}
	if _, has := o["instance"]; has && (!isString(o, "instance") || len(str(o["instance"])) > 50) {
		return nil, false
	}
	return o, true
}
