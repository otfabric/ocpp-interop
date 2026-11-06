// SPDX-License-Identifier: MIT
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/otfabric/go-ocpp"
	"github.com/otfabric/go-ocpp/ocppj"
)

// ---- observation of the frames go-ocpp sends and receives ----

// tracker turns the frames that go-ocpp's frame observer (WithFrameObserver)
// sees into events, and remembers what the result of a CALL needs: the
// payload as sent, and whether the peer answered with a CALLRESULT or a
// CALLERROR frame. It reads frames; it does not take part in the protocol.
type tracker struct {
	mu         sync.Mutex
	emit       bool
	received   map[string]string          // station, id -> action of a CALL received
	sent       map[string]json.RawMessage // station, id -> payload of a CALL sent
	sentAction map[string]string          // station, id -> action of a CALL sent
	results    map[string]bool            // station, id -> a CALLRESULT frame arrived
	callErrors map[string]*object         // station, id -> the CALLERROR that arrived
	last       map[string]string          // session -> the last frame received, for protocol-error
	// opened is called before the first event of a session that has not
	// been reported as opened yet (the server's connection-opened).
	opened func(ocpp.ConnInfo)
}

func newTracker(emit bool) *tracker {
	return &tracker{emit: emit, received: map[string]string{}, sent: map[string]json.RawMessage{},
		sentAction: map[string]string{}, results: map[string]bool{}, callErrors: map[string]*object{},
		last: map[string]string{}}
}

func key(station, id string) string { return station + "\x00" + id }

// frame splits an OCPP-J message into its elements: type, id and the rest.
func frame(data []byte) (int, string, []json.RawMessage, bool) {
	var parts []json.RawMessage
	if json.Unmarshal(data, &parts) != nil || len(parts) < 3 {
		return 0, "", nil, false
	}
	var kind int
	var id string
	if json.Unmarshal(parts[0], &kind) != nil || json.Unmarshal(parts[1], &id) != nil {
		return 0, "", nil, false
	}
	return kind, id, parts[2:], true
}

func errorOf(rest []json.RawMessage) *object {
	var code, description string
	var details any = map[string]any{}
	if len(rest) > 0 {
		_ = json.Unmarshal(rest[0], &code)
	}
	if len(rest) > 1 {
		_ = json.Unmarshal(rest[1], &description)
	}
	if len(rest) > 2 {
		details = rest[2]
	}
	return errorObject(code, description, details)
}

// observe is the go-ocpp FrameObserver. It must not block or keep frame.
func (t *tracker) observe(info ocpp.ConnInfo, dir ocpp.Direction, data []byte) {
	if t.opened != nil {
		t.opened(info)
	}
	if dir == ocpp.Incoming {
		t.incoming(info, append([]byte(nil), data...))
	} else {
		t.outgoing(info.StationID, append([]byte(nil), data...))
	}
}

func (t *tracker) incoming(info ocpp.ConnInfo, data []byte) {
	station := info.StationID
	t.mu.Lock()
	defer t.mu.Unlock()
	raw := string(data)
	if len(raw) > 4096 {
		raw = raw[:4096]
	}
	t.last[info.SessionID] = raw
	kind, id, rest, ok := frame(data)
	if !ok {
		return // go-ocpp reports it through Metrics.OnProtocolError
	}
	k := key(station, id)
	switch {
	case kind == 2 && len(rest) == 2:
		var action string
		_ = json.Unmarshal(rest[0], &action)
		t.received[k] = action
		t.event("call-received", station, "messageId", id, "action", action, "payload", rest[1])
	case kind == 3:
		t.results[k] = true
		t.event("call-result", station, "messageId", id, "action", t.actionOf(k), "payload", rest[0])
	case kind == 4:
		t.callErrors[k] = errorOf(rest)
		t.event("call-result", station, "messageId", id, "action", t.actionOf(k), "error", t.callErrors[k])
	}
}

func (t *tracker) outgoing(station string, data []byte) {
	kind, id, rest, ok := frame(data)
	if !ok {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	k := key(station, id)
	switch {
	case kind == 2 && len(rest) == 2:
		if _, reported := t.sentAction[k]; reported {
			return // reported by intercept, before the frame was written
		}
		var action string
		_ = json.Unmarshal(rest[0], &action)
		t.sent[k] = rest[1]
		t.sentAction[k] = action
		t.event("call-sent", station, "messageId", id, "action", action, "payload", rest[1])
	case kind == 3:
		action := t.received[k]
		delete(t.received, k)
		t.event("call-answered", station, "messageId", id, "action", nullable(action), "payload", rest[0])
	case kind == 4:
		action := t.received[k]
		delete(t.received, k)
		t.event("call-answered", station, "messageId", id, "action", nullable(action), "error", errorOf(rest))
	}
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func (t *tracker) actionOf(k string) any { return nullable(t.sentAction[k]) }

func (t *tracker) event(name, station string, fields ...any) {
	if t.emit {
		event(name, station, fields...)
	}
}

// outcome of a CALL as the frames show it.
func (t *tracker) outcome(station, id string) (sent json.RawMessage, result bool, callError *object) {
	t.mu.Lock()
	defer t.mu.Unlock()
	k := key(station, id)
	return t.sent[k], t.results[k], t.callErrors[k]
}

// metrics is the go-ocpp Metrics of an endpoint: protocol errors become
// protocol-error events, and a lost connection is passed on.
type metrics struct {
	ocpp.NopMetrics
	t            *tracker
	disconnected func(ocpp.ConnInfo, error)
}

func (m *metrics) OnProtocolError(info ocpp.ConnInfo, err error) {
	if errors.Is(err, ocpp.ErrUnexpectedResponse) {
		return // a late response after a timeout: not a frame go-ocpp could not read
	}
	m.t.mu.Lock()
	raw := m.t.last[info.SessionID]
	m.t.mu.Unlock()
	m.t.event("protocol-error", info.StationID, "detail", err.Error(), "frame", raw)
}

func (m *metrics) OnDisconnect(info ocpp.ConnInfo, err error) {
	if m.disconnected != nil {
		m.disconnected(info, err)
	}
}

// ---- message ids ----

// callKey carries, in the context of a call, the payload execute sends and
// where the call interceptor writes the message id go-ocpp generated for it.
type callKey struct{}

type outgoingCall struct {
	id      string
	payload json.RawMessage
}

// intercept is the go-ocpp call interceptor (WithCallInterceptor). It reads
// the message id of each CALL this end sends (ocpp.CallInfo.MessageID), and
// reports the CALL before go-ocpp writes it: the frame observer sees a frame
// only after it was written, and a peer's response can be read before that.
// go-ocpp sends a json.RawMessage payload as it is.
func (t *tracker) intercept(ctx context.Context, info ocpp.CallInfo, next func(context.Context) error) error {
	if c, ok := ctx.Value(callKey{}).(*outgoingCall); ok {
		c.id = info.MessageID
		k := key(info.StationID, info.MessageID)
		t.mu.Lock()
		t.sent[k] = c.payload
		t.sentAction[k] = info.Action
		t.event("call-sent", info.StationID, "messageId", info.MessageID, "action", info.Action, "payload", c.payload)
		t.mu.Unlock()
	}
	return next(ctx)
}

// ---- answering and sending ----

// endpoint is the fixture's side of a connection: it answers CALLs from the
// fixture, and sends CALLs for the contract.
type endpoint struct {
	version string
	fixture *fixture
	role    string // the answering role: "csms" for a server, "station" for a client
	tracker *tracker
}

// receives: the actions this end receives in its version.
func (e *endpoint) receives() map[string]bool {
	if e.role == "csms" {
		return versions[e.version].fromStation
	}
	return versions[e.version].fromCSMS
}

// dispatcher is the go-ocpp Dispatcher of the endpoint. It works on raw
// payloads (ocpp.DispatcherFunc), as the typed dispatchers of go-ocpp's
// version packages do on typed ones: the request is decoded and validated by
// the version package (ValidateRequest), the fixture's answer by its
// ValidateResponse, and go-ocpp sends the result or the CALLERROR.
func (e *endpoint) dispatcher() ocpp.Dispatcher {
	vi := versions[e.version]
	return ocpp.DispatcherFunc(func(_ context.Context, _ ocpp.Peer, action string, payload json.RawMessage) (json.RawMessage, error) {
		if !knows(e.version, action) {
			return nil, ocpp.NewCallError(ocpp.NotImplemented, "")
		}
		if !e.receives()[action] {
			return nil, ocpp.NewCallError(ocpp.NotSupported, action+" is not sent to this role")
		}
		if err := vi.validateRequest(action, payload); err != nil {
			var ve *ocpp.ValidationError
			if errors.As(err, &ve) {
				return nil, ve.CallError()
			}
			return nil, ocpp.NewCallError(ocppj.FormatErrorCode(vi.version), err.Error())
		}
		var fields map[string]any
		_ = decodeNumbers(payload, &fields)
		switch a := e.fixture.answer(e.role, action, fields).(type) {
		case nil:
			return nil, ocpp.NewCallError(ocpp.NotImplemented, "the fixture has no answer for "+action)
		case callErrorAnswer:
			return nil, ocpp.NewCallError(ocpp.ErrorCode(a.code), a.description)
		case map[string]any:
			body, err := marshal(a)
			if err != nil {
				return nil, err
			}
			if err := vi.validateResponse(action, body); err != nil {
				return nil, fmt.Errorf("the fixture's answer for %s is not valid for go-ocpp: %w", action, err)
			}
			return body, nil
		default:
			return nil, fmt.Errorf("no answer for %s", action)
		}
	})
}

// execute sends one CALL with go-ocpp and returns the result document.
func (e *endpoint) execute(station, action string, payload map[string]any, timeoutMs int64, caller ocpp.Caller,
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
			errMsg: fmt.Sprintf("go-ocpp has no OCPP %s action %s", e.version, action)})
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
	if vi.sendActions[action] {
		return done(resultDoc{request: payload, errCode: "unsupported",
			errMsg: action + " is a SEND, which the contract has no operation for (the send feature is not declared)"})
	}
	body, err := marshal(payload)
	if err != nil {
		return done(resultDoc{request: payload, errCode: "invalid-request", errMsg: err.Error()})
	}
	if err := vi.validateRequest(action, body); err != nil {
		return done(resultDoc{request: payload, errCode: "invalid-request",
			errMsg: "go-ocpp refused the request: " + err.Error()})
	}
	select {
	case <-closed:
		return done(resultDoc{request: payload, errCode: "not-connected", errMsg: "the connection is closed"})
	default:
	}

	call := &outgoingCall{payload: body}
	ctx, cancel := context.WithTimeout(context.WithValue(context.Background(), callKey{}, call),
		time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()
	var response json.RawMessage
	err = caller.Call(ctx, action, json.RawMessage(body), &response)
	id := call.id
	asSent := func() any {
		if sent, _, _ := e.tracker.outcome(station, id); sent != nil {
			return sent
		}
		return json.RawMessage(body)
	}
	if err == nil {
		if verr := vi.validateResponse(action, response); verr != nil {
			return done(resultDoc{messageID: id, request: asSent(), errCode: "invalid-response",
				errMsg: "go-ocpp rejected the response: " + verr.Error()})
		}
		return done(resultDoc{ok: true, messageID: id, request: asSent(), response: response})
	}
	var ce *ocpp.CallError
	switch {
	case errors.As(err, &ce):
		var details any = map[string]any{}
		if len(ce.Details) > 0 {
			details = ce.Details
		}
		return done(resultDoc{messageID: id, request: asSent(), callError: errorObject(string(ce.Code), ce.Description, details),
			errCode: "call-error", errMsg: fmt.Sprintf("CALLERROR %s", ce.Code)})
	case errors.Is(err, ocpp.ErrInvalidMessage):
		return done(resultDoc{messageID: id, request: asSent(), errCode: "invalid-response",
			errMsg: "go-ocpp rejected the response: " + err.Error()})
	case errors.Is(err, ocpp.ErrTimeout), errors.Is(err, context.DeadlineExceeded):
		return done(resultDoc{messageID: id, request: asSent(), errCode: "timeout",
			errMsg: fmt.Sprintf("no response within %d ms", timeoutMs)})
	case errors.Is(err, ocpp.ErrNotConnected):
		return done(resultDoc{messageID: id, request: asSent(), errCode: "not-connected", errMsg: err.Error()})
	case errors.Is(err, ocpp.ErrConnectionLost), errors.Is(err, ocpp.ErrClosed):
		return done(resultDoc{messageID: id, request: asSent(), errCode: "connection-lost",
			errMsg: "the connection closed while waiting for the response: " + err.Error()})
	}
	return done(resultDoc{messageID: id, request: asSent(), errCode: "failed", errMsg: err.Error()})
}

// closeOf is the close code and reason of a lost connection, where the peer
// sent a close frame (ocpp.CloseError), and null otherwise.
func closeOf(err error) (code, reason any) {
	var ce *ocpp.CloseError
	if errors.As(err, &ce) {
		return ce.Code, ce.Reason
	}
	return nil, nil
}
