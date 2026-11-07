// SPDX-License-Identifier: MIT
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/seanlee0923/ocpp/csms"
	"github.com/seanlee0923/ocpp/station"
)

// endpoint is the fixture's side of a connection: it answers CALLs from the
// fixture through seanlee0923/ocpp's typed handlers, sends CALLs for the
// contract, and writes the events. The library has no frame hook and does
// not give the message id of a CALL to its handler, nor the id of a CALL it
// sends (messageIds false): the events come from the handlers (a CALL or
// SEND received, and the answer the handler gives) and from the calls the
// adapter makes.
type endpoint struct {
	version string
	fixture *fixture
	role    string // the answering role: "csms" for a server, "station" for a client
	emit    bool   // write events (server, client run)
}

func newEndpoint(version string, fx *fixture, role string, emit bool) *endpoint {
	return &endpoint{version: version, fixture: fx, role: role, emit: emit}
}

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

// answerer is the answerFunc of one action for a station's connection.
func (e *endpoint) answerer(station, action string) answerFunc {
	return func(_ context.Context, payload json.RawMessage) (json.RawMessage, error) {
		e.event("call-received", station, "messageId", nil, "action", action, "payload", payload)
		answer, err := e.answer(action, payload)
		if err != nil {
			code, description := "InternalError", err.Error()
			var ae *answerError
			if errors.As(err, &ae) {
				code, description = ae.code, ae.description
			}
			e.event("call-answered", station, "messageId", nil, "action", action,
				"error", errorObject(code, description, map[string]any{}))
			return nil, err
		}
		e.event("call-answered", station, "messageId", nil, "action", action, "payload", answer)
		return answer, nil
	}
}

// taker is the sendFunc of one SEND action.
func (e *endpoint) taker(station, action string) sendFunc {
	return func(_ context.Context, payload json.RawMessage) {
		e.event("send-received", station, "messageId", nil, "action", action, "payload", payload)
	}
}

// answer is the fixture's answer to a CALL: a payload, or an *answerError
// for its CALLERROR. The library checks the request against the OCA schema
// before the handler runs, and the answer after.
func (e *endpoint) answer(action string, payload json.RawMessage) (json.RawMessage, error) {
	if !e.receives()[action] {
		return nil, &answerError{"NotSupported", action + " is not sent to this role"}
	}
	var fields map[string]any
	_ = decodeNumbers(payload, &fields)
	switch a := e.fixture.answer(e.role, action, fields).(type) {
	case nil:
		return nil, &answerError{"NotImplemented", "the fixture has no answer for " + action}
	case callErrorAnswer:
		return nil, &answerError{a.code, a.description}
	case invalidAnswer:
		return marshal(a.payload)
	case map[string]any:
		return marshal(a)
	default:
		return nil, fmt.Errorf("no answer for %s", action)
	}
}

// target is a connection to send on: a CSMS session or a station.
type target struct {
	session *csms.Session
	station *station.Station
}

// execute sends one CALL with the library and returns the result document.
func (e *endpoint) execute(stationID, action string, payload map[string]any, timeoutMs int64, t *target,
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
			errMsg: fmt.Sprintf("seanlee0923/ocpp has no OCPP %s action %s", e.version, action)})
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
	ops := actionTable[e.version][action]
	if ops.send {
		return done(resultDoc{request: payload, errCode: "unsupported",
			errMsg: fmt.Sprintf("seanlee0923/ocpp cannot send the SEND %s", action)})
	}
	body, err := marshal(payload)
	if err != nil {
		return done(resultDoc{request: payload, errCode: "invalid-request", errMsg: err.Error()})
	}
	if err := ops.check(body); err != nil {
		return done(resultDoc{request: payload, errCode: "invalid-request",
			errMsg: "seanlee0923/ocpp refused the request: " + err.Error()})
	}
	if t == nil {
		return done(resultDoc{request: payload, errCode: "not-connected", errMsg: "not connected"})
	}
	select {
	case <-closed:
		return done(resultDoc{request: payload, errCode: "not-connected", errMsg: "the connection is closed"})
	default:
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	e.event("call-sent", stationID, "messageId", nil, "action", action, "payload", json.RawMessage(body))
	var response json.RawMessage
	if t.session != nil {
		response, err = ops.csmsCall(ctx, t.session, body)
	} else {
		response, err = ops.stationCall(ctx, t.station, body)
	}
	if err == nil {
		e.event("call-result", stationID, "messageId", nil, "action", action, "payload", response)
		return done(resultDoc{ok: true, request: json.RawMessage(body), response: response})
	}
	var callError *object
	code, desc := "", ""
	var details json.RawMessage
	var cr *csms.RemoteCallError
	var sr *station.RemoteCallError
	var re *requestError
	var rs *responseError
	switch {
	case errors.As(err, &cr):
		code, desc, details = string(cr.Code), cr.Description, cr.Details
	case errors.As(err, &sr):
		code, desc, details = sr.Code, sr.Description, sr.Details
	case errors.As(err, &re):
		return done(resultDoc{request: json.RawMessage(body), errCode: "invalid-request",
			errMsg: "seanlee0923/ocpp refused the request: " + re.Error()})
	case errors.As(err, &rs):
		// The library does not give the confirmation it refused.
		return done(resultDoc{request: json.RawMessage(body), errCode: "invalid-response",
			errMsg: "seanlee0923/ocpp rejected the response: " + rs.Error()})
	case errors.Is(err, context.DeadlineExceeded):
		return done(resultDoc{request: json.RawMessage(body), errCode: "timeout",
			errMsg: fmt.Sprintf("no response within %d ms", timeoutMs)})
	case errors.Is(err, csms.ErrSessionClosed), errors.Is(err, csms.ErrSessionReplaced),
		errors.Is(err, csms.ErrServerShutdown), errors.Is(err, station.ErrNotConnected):
		return done(resultDoc{request: json.RawMessage(body), errCode: "connection-lost",
			errMsg: "the connection closed while waiting for the response: " + err.Error()})
	}
	if code != "" {
		var d any = map[string]any{}
		if len(details) > 0 {
			var v any
			if decodeNumbers(details, &v) == nil && v != nil {
				d = v
			}
		}
		callError = errorObject(code, desc, d)
		e.event("call-result", stationID, "messageId", nil, "action", action, "error", callError)
		return done(resultDoc{request: json.RawMessage(body), callError: callError,
			errCode: "call-error", errMsg: "CALLERROR " + code})
	}
	select {
	case <-closed:
		return done(resultDoc{request: json.RawMessage(body), errCode: "connection-lost",
			errMsg: "the connection closed while waiting for the response: " + err.Error()})
	default:
	}
	return done(resultDoc{request: json.RawMessage(body), errCode: "failed", errMsg: err.Error()})
}
