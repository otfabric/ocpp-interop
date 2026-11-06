// SPDX-License-Identifier: MIT
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/cB-Sigma/ocpp"
	"github.com/cB-Sigma/ocpp/chargepoint"
)

// endpoint is the fixture's side of a connection: it answers CALLs from the
// fixture through cB-Sigma's handlers, sends CALLs and SENDs for the
// contract, and writes the events. cB-Sigma has no frame hook, so the events
// come from its handlers (a CALL received and the answer the handler gives;
// ocpp.MessageID and ocpp.IsSend of the handler's context), from the calls the
// adapter makes (the message id is the adapter's, given to cB-Sigma with
// ocpp.WithMessageID), and from OnResultError (a CALLRESULTERROR received).
type endpoint struct {
	version string
	fixture *fixture
	role    string // the answering role: "csms" for a server, "station" for a client
	emit    bool   // write events (server, client run)

	mu       sync.Mutex
	answered map[string]string // station, message id -> action this end answered
}

func newEndpoint(version string, fx *fixture, role string, emit bool) *endpoint {
	return &endpoint{version: version, fixture: fx, role: role, emit: emit, answered: map[string]string{}}
}

func key(station, id string) string { return station + "\x00" + id }

func (e *endpoint) event(name, station string, fields ...any) {
	if e.emit {
		event(name, station, fields...)
	}
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// receives: the actions this end receives in its version.
func (e *endpoint) receives() map[string]bool {
	if e.role == "csms" {
		return versions[e.version].fromStation
	}
	return versions[e.version].fromCSMS
}

// invalidAnswers reports whether the fixture has invalidAnswers for this
// end: cB-Sigma then runs with SkipValidation, so that it sends them.
func (e *endpoint) invalidAnswers() bool {
	invalid, ok := e.fixture.doc["invalidAnswers"].(map[string]any)
	if !ok {
		return false
	}
	table, ok := invalid[e.role].(map[string]any)
	return ok && len(table) > 0
}

// rpcError is the CALLERROR of a code, as cB-Sigma sends it (ocpp.RPCError).
func (e *endpoint) rpcError(code, description string) error {
	kind, ok := errKindOf(e.version, code)
	if !ok {
		kind = ocpp.KindGeneric
	}
	return ocpp.RPCErrorf(kind, "%s", description)
}

// handle answers one CALL, or takes one SEND, of the peer: the handler of
// every action this end receives (csms.Server.Handle, chargepoint.Client.Handle).
func (e *endpoint) handle(ctx context.Context, station, action string, payload json.RawMessage) (any, error) {
	vi := versions[e.version]
	id := ocpp.MessageID(ctx)
	if ocpp.IsSend(ctx) {
		e.event("send-received", station, "messageId", nullable(id), "action", action, "payload", payload)
		return nil, nil
	}
	e.event("call-received", station, "messageId", nullable(id), "action", action, "payload", payload)
	answer, err := e.answer(action, payload)
	if err != nil {
		var re *ocpp.RPCError
		code, description := ocpp.KindInternal.Code(vi.wire), err.Error()
		if errors.As(err, &re) {
			code, description = re.Kind.Code(vi.wire), re.Desc
		}
		e.event("call-answered", station, "messageId", nullable(id), "action", action,
			"error", errorObject(code, description, map[string]any{}))
		return nil, err
	}
	e.mu.Lock()
	e.answered[key(station, id)] = action
	e.mu.Unlock()
	e.event("call-answered", station, "messageId", nullable(id), "action", action, "payload", answer)
	return answer, nil
}

// answer is the fixture's answer to a CALL, as cB-Sigma sends it: a raw
// payload, or an *ocpp.RPCError for its CALLERROR. cB-Sigma checks the
// request against the OCA schema before the handler runs (unless its
// validation is off for invalidAnswers; then the adapter checks it with
// ocpp.ValidateRequest), and the answer after.
func (e *endpoint) answer(action string, payload json.RawMessage) (json.RawMessage, error) {
	vi := versions[e.version]
	if !e.receives()[action] {
		return nil, ocpp.RPCErrorf(ocpp.KindNotSupported, "%s is not sent to this role", action)
	}
	if e.invalidAnswers() {
		if verr := ocpp.ValidateRequest(vi.wire, action, payload); verr != nil {
			return nil, verr
		}
	}
	var fields map[string]any
	_ = decodeNumbers(payload, &fields)
	switch a := e.fixture.answer(e.role, action, fields).(type) {
	case nil:
		return nil, ocpp.RPCErrorf(ocpp.KindNotImplemented, "the fixture has no answer for %s", action)
	case callErrorAnswer:
		return nil, e.rpcError(a.code, a.description)
	case invalidAnswer:
		return marshal(a.payload)
	case map[string]any:
		return marshal(a)
	default:
		return nil, fmt.Errorf("no answer for %s", action)
	}
}

// onResultError is cB-Sigma's OnResultError: the peer answered one of this
// end's CALLRESULTs with a CALLRESULTERROR (OCPP 2.1).
func (e *endpoint) onResultError(station string) func(id, code, desc string) {
	return func(id, code, desc string) {
		e.mu.Lock()
		action := e.answered[key(station, id)]
		e.mu.Unlock()
		e.event("call-result-error", station, "messageId", id, "action", nullable(action),
			"error", errorObject(code, desc, map[string]any{}))
	}
}

// caller is what execute needs of csms.Station and chargepoint.Client.
type caller interface {
	Call(ctx context.Context, action string, req any) (json.RawMessage, error)
	Send(ctx context.Context, action string, payload any) error
}

// execute sends one CALL (or SEND) with cB-Sigma and returns the result
// document.
func (e *endpoint) execute(station, action string, payload map[string]any, timeoutMs int64, c caller,
	closed <-chan struct{}) resultDoc {
	started := time.Now()
	done := func(r resultDoc) resultDoc {
		r.version, r.operation = e.version, action
		r.elapsedMs = time.Since(started).Milliseconds()
		return r
	}
	vi := versions[e.version]
	if !knows(e.version, action) {
		return done(resultDoc{request: payload, errCode: "unsupported",
			errMsg: fmt.Sprintf("cB-Sigma/ocpp has no OCPP %s action %s", e.version, action)})
	}
	// A client sends as the station, a server as the CSMS.
	sender, sends := "station", vi.fromStation
	if e.role == "csms" {
		sender, sends = "CSMS", vi.fromCSMS
	}
	if !sends[action] {
		return done(resultDoc{request: payload, errCode: "unsupported",
			errMsg: fmt.Sprintf("the %s does not send %s in OCPP %s", sender, action, e.version)})
	}
	body, err := marshal(payload)
	if err != nil {
		return done(resultDoc{request: payload, errCode: "invalid-request", errMsg: err.Error()})
	}
	if verr := ocpp.ValidateRequest(vi.wire, action, body); verr != nil {
		return done(resultDoc{request: payload, errCode: "invalid-request",
			errMsg: "cB-Sigma refused the request: " + verr.Error()})
	}
	if c == nil {
		return done(resultDoc{request: payload, errCode: "not-connected", errMsg: "not connected"})
	}
	select {
	case <-closed:
		return done(resultDoc{request: payload, errCode: "not-connected", errMsg: "the connection is closed"})
	default:
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	if vi.sendActions[action] {
		// An OCPP 2.1 SEND: cB-Sigma picks its message id itself and does
		// not say which.
		e.event("send-sent", station, "messageId", nil, "action", action, "payload", json.RawMessage(body))
		if err := c.Send(ctx, action, json.RawMessage(body)); err != nil {
			return done(resultDoc{request: json.RawMessage(body), errCode: "failed", errMsg: err.Error()})
		}
		return done(resultDoc{ok: true, request: json.RawMessage(body), response: nil})
	}

	id := ocpp.NewMessageID()
	e.event("call-sent", station, "messageId", id, "action", action, "payload", json.RawMessage(body))
	response, err := c.Call(ocpp.WithMessageID(ctx, id), action, json.RawMessage(body))
	if err == nil {
		e.event("call-result", station, "messageId", id, "action", action, "payload", response)
		return done(resultDoc{ok: true, messageID: id, request: json.RawMessage(body), response: response})
	}
	var ce *ocpp.CallError
	var re *ocpp.ResultError
	var rpc *ocpp.RPCError
	switch {
	case errors.As(err, &ce):
		var details any = map[string]any{}
		if ce.Details != nil {
			details = ce.Details
		}
		callError := errorObject(ce.Code, ce.Description, details)
		e.event("call-result", station, "messageId", id, "action", action, "error", callError)
		return done(resultDoc{messageID: id, request: json.RawMessage(body), callError: callError,
			errCode: "call-error", errMsg: "CALLERROR " + ce.Code})
	case errors.As(err, &re):
		// An answer outside the schema; on OCPP 2.1 cB-Sigma has answered it
		// with a CALLRESULTERROR (its CheckResult).
		r := resultDoc{messageID: id, request: json.RawMessage(body), errCode: "invalid-response",
			errMsg: "cB-Sigma rejected the response: " + re.Error()}
		if vi.wire == ocpp.V21 && !e.invalidAnswers() {
			r.callResultError = errorObject(re.Code, re.Description, map[string]any{})
		}
		e.event("call-result", station, "messageId", id, "action", action,
			"error", errorObject(re.Code, re.Description, map[string]any{}))
		return done(r)
	case errors.As(err, &rpc):
		return done(resultDoc{messageID: id, request: json.RawMessage(body), errCode: "invalid-request",
			errMsg: "cB-Sigma refused the request: " + rpc.Error()})
	case errors.Is(err, ocpp.ErrTimeout), errors.Is(err, context.DeadlineExceeded):
		return done(resultDoc{messageID: id, request: json.RawMessage(body), errCode: "timeout",
			errMsg: fmt.Sprintf("no response within %d ms", timeoutMs)})
	case errors.Is(err, chargepoint.ErrNotConnected):
		return done(resultDoc{messageID: id, request: json.RawMessage(body), errCode: "not-connected", errMsg: err.Error()})
	case errors.Is(err, ocpp.ErrClosed):
		return done(resultDoc{messageID: id, request: json.RawMessage(body), errCode: "connection-lost",
			errMsg: "the connection closed while waiting for the response: " + err.Error()})
	}
	select {
	case <-closed:
		return done(resultDoc{messageID: id, request: json.RawMessage(body), errCode: "connection-lost",
			errMsg: "the connection closed while waiting for the response: " + err.Error()})
	default:
	}
	return done(resultDoc{messageID: id, request: json.RawMessage(body), errCode: "failed", errMsg: err.Error()})
}
