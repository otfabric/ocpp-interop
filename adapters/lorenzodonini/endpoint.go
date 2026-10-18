// SPDX-License-Identifier: MIT
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"sync"
	"time"

	"github.com/lorenzodonini/ocpp-go/ocpp"
	"github.com/lorenzodonini/ocpp-go/ocppj"
	"github.com/lorenzodonini/ocpp-go/ws"
	"gopkg.in/go-playground/validator.v9"
)

// ---- observation of the frames ocpp-go sends and receives ----

// tracker turns the frames that pass through ocpp-go's ws layer into events,
// and remembers what the result of a CALL needs: the payload as sent, and
// whether the peer answered with a CALLRESULT or a CALLERROR frame. It reads
// frames; it does not take part in the protocol.
type tracker struct {
	mu         sync.Mutex
	emit       bool
	received   map[string]string          // station, id -> action of a CALL received
	sent       map[string]json.RawMessage // station, id -> payload of a CALL sent
	sentAction map[string]string          // station, id -> action of a CALL sent
	results    map[string]bool            // station, id -> a CALLRESULT frame arrived
	callErrors map[string]*object         // station, id -> the CALLERROR that arrived
}

func newTracker(emit bool) *tracker {
	return &tracker{emit: emit, received: map[string]string{}, sent: map[string]json.RawMessage{},
		sentAction: map[string]string{}, results: map[string]bool{}, callErrors: map[string]*object{}}
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

func (t *tracker) incoming(station string, data []byte) {
	kind, id, rest, ok := frame(data)
	if !ok {
		return // ocpp-go reports it through the invalid message hook
	}
	t.mu.Lock()
	defer t.mu.Unlock()
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

// invalidMessage is ocpp-go's invalid message hook: a frame ocpp-go could not
// parse or validate. The adapter reports it and lets ocpp-go answer as it does.
func (t *tracker) invalidMessage(station string, err *ocpp.Error, raw string) *ocpp.Error {
	frameText := raw
	if len(frameText) > 4096 {
		frameText = frameText[:4096]
	}
	t.event("protocol-error", station, "detail", fmt.Sprintf("%s: %s", err.Code, err.Description), "frame", frameText)
	return nil
}

// observedServer is ocpp-go's ws.Server with the frames it reads and writes
// passed to a tracker first.
type observedServer struct {
	ws.WsServer
	t *tracker
}

func (s *observedServer) SetMessageHandler(handler func(ws.Channel, []byte) error) {
	s.WsServer.SetMessageHandler(func(c ws.Channel, data []byte) error {
		s.t.incoming(c.ID(), data)
		return handler(c, data)
	})
}

func (s *observedServer) Write(id string, data []byte) error {
	// Reported before the frame is handed over, so that the event precedes
	// the peer's answer in the stream.
	s.t.outgoing(id, data)
	return s.WsServer.Write(id, data)
}

// observedClient is ocpp-go's ws.Client, observed the same way.
type observedClient struct {
	ws.WsClient
	t       *tracker
	station string
}

func (c *observedClient) SetMessageHandler(handler func([]byte) error) {
	c.WsClient.SetMessageHandler(func(data []byte) error {
		c.t.incoming(c.station, data)
		return handler(data)
	})
}

func (c *observedClient) Write(data []byte) error {
	c.t.outgoing(c.station, data)
	return c.WsClient.Write(data)
}

// ---- message ids ----

// ocpp-go generates the id of a CALL inside SendRequest. The adapter keeps
// ocpp-go's default generator (fmt.Sprintf("%v", rand.Uint32()), see
// ocppj.SetMessageIdGenerator) and reads the id it produced while it holds
// sendMu, which serialises all sends of the process.
var (
	sendMu sync.Mutex
	lastID string
)

func init() {
	ocppj.SetMessageIdGenerator(func() string {
		lastID = fmt.Sprintf("%v", rand.Uint32())
		return lastID
	})
}

// ---- answering and sending ----

// endpoint is the fixture's side of a connection: it answers CALLs from the
// fixture, and sends CALLs for the contract.
type endpoint struct {
	version    string
	fixture    *fixture
	role       string // the answering role: "csms" for a server, "station" for a client
	tracker    *tracker
	setTimeout func(time.Duration)
}

// answer is what a handler returns: the fixture's answer as ocpp-go's
// response type, or the error ocpp-go turns into a CALLERROR.
func (e *endpoint) answer(request ocpp.Request) (ocpp.Response, error) {
	action := request.GetFeatureName()
	raw, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	var fields map[string]any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	_ = dec.Decode(&fields)
	switch a := e.fixture.answer(e.role, action, fields).(type) {
	case nil:
		return nil, ocpp.NewHandlerError(ocppj.NotImplemented, "the fixture has no answer for "+action)
	case callErrorAnswer:
		return nil, ocpp.NewHandlerError(ocpp.ErrorCode(a.code), a.description)
	case map[string]any:
		f := feature(e.version, action)
		if f == nil {
			return nil, fmt.Errorf("ocpp-go has no feature for %s", action)
		}
		data, _ := json.Marshal(a)
		response := reflect.New(f.GetResponseType()).Interface()
		if err := json.Unmarshal(data, response); err != nil {
			return nil, fmt.Errorf("ocpp-go cannot read the fixture's answer for %s: %v", action, err)
		}
		return response.(ocpp.Response), nil
	default:
		return nil, fmt.Errorf("no answer for %s", action)
	}
}

// reply is the body of every typed handler: the fixture's answer as *T.
func reply[T any](e *endpoint, request ocpp.Request) (*T, error) {
	response, err := e.answer(request)
	if err != nil {
		return nil, err
	}
	typed, ok := any(response).(*T)
	if !ok {
		return nil, fmt.Errorf("the answer for %s is a %T", request.GetFeatureName(), response)
	}
	return typed, nil
}

type sendFunc func(ocpp.Request, func(ocpp.Response, error)) error

type callOutcome struct {
	response ocpp.Response
	err      error
}

// execute sends one CALL with ocpp-go and returns the result document.
func (e *endpoint) execute(station, action string, payload map[string]any, timeoutMs int64, send sendFunc,
	closed <-chan struct{}) resultDoc {
	started := time.Now()
	done := func(r resultDoc) resultDoc {
		r.version, r.operation = e.version, action
		r.elapsedMs = time.Since(started).Milliseconds()
		return r
	}
	f := feature(e.version, action)
	if f == nil {
		return done(resultDoc{request: payload, errCode: "unsupported",
			errMsg: fmt.Sprintf("ocpp-go has no OCPP %s feature for %s", e.version, action)})
	}
	// A client answers and sends as the station, a server as the CSMS.
	sender, sends := "station", versions[e.version].fromStation
	if e.role == "csms" {
		sender, sends = "CSMS", versions[e.version].fromCSMS
	}
	if !sends[action] {
		return done(resultDoc{request: payload, errCode: "unsupported",
			errMsg: fmt.Sprintf("the %s does not send %s in OCPP %s", sender, action, e.version)})
	}
	data, _ := json.Marshal(payload)
	request := reflect.New(f.GetRequestType()).Interface()
	if err := json.Unmarshal(data, request); err != nil {
		return done(resultDoc{request: payload, errCode: "invalid-request",
			errMsg: fmt.Sprintf("ocpp-go cannot read the payload as %s: %v", f.GetRequestType().Name(), err)})
	}
	select {
	case <-closed:
		return done(resultDoc{request: payload, errCode: "not-connected", errMsg: "the connection is closed"})
	default:
	}

	outcome := make(chan callOutcome, 1)
	timeout := time.Duration(timeoutMs) * time.Millisecond
	sendMu.Lock()
	// ocpp-go's dispatcher times the CALL out itself; it takes the timeout of
	// the next CALL it dispatches.
	e.setTimeout(timeout)
	lastID = ""
	err := send(request.(ocpp.Request), func(response ocpp.Response, err error) {
		outcome <- callOutcome{response, err}
	})
	id := lastID
	sendMu.Unlock()
	asSent := func() any {
		if sent, _, _ := e.tracker.outcome(station, id); sent != nil {
			return sent
		}
		wire, _ := json.Marshal(request)
		return json.RawMessage(wire)
	}
	if err != nil {
		var invalid validator.ValidationErrors
		if errors.As(err, &invalid) {
			return done(resultDoc{request: asSent(), errCode: "invalid-request",
				errMsg: "ocpp-go refused the request: " + err.Error()})
		}
		select {
		case <-closed:
			return done(resultDoc{request: asSent(), errCode: "not-connected", errMsg: err.Error()})
		default:
		}
		return done(resultDoc{request: asSent(), errCode: "failed", errMsg: err.Error()})
	}

	var result callOutcome
	select {
	case result = <-outcome:
	case <-closed:
		return done(resultDoc{messageID: id, request: asSent(), errCode: "connection-lost",
			errMsg: "the connection closed while waiting for the response"})
	case <-time.After(timeout + 2*time.Second):
		result = callOutcome{err: errors.New("no outcome")}
	}
	_, sawResult, callError := e.tracker.outcome(station, id)
	if result.err == nil && result.response != nil {
		body, _ := json.Marshal(result.response)
		return done(resultDoc{ok: true, messageID: id, request: asSent(), response: json.RawMessage(body)})
	}
	switch {
	case callError != nil:
		return done(resultDoc{messageID: id, request: asSent(), callError: callError, errCode: "call-error",
			errMsg: fmt.Sprintf("CALLERROR %v", callError.get("code"))})
	case sawResult:
		// A CALLRESULT ocpp-go could not parse or validate: it is never
		// handed over, and the CALL times out.
		return done(resultDoc{messageID: id, request: asSent(), errCode: "invalid-response",
			errMsg: "ocpp-go rejected the response (see the protocol-error event)"})
	}
	// ocpp-go fails the CALLs of a connection that closes before it reports
	// the close.
	select {
	case <-closed:
		return done(resultDoc{messageID: id, request: asSent(), errCode: "connection-lost",
			errMsg: "the connection closed while waiting for the response"})
	case <-time.After(300 * time.Millisecond):
	}
	var ocppErr *ocpp.Error
	if result.err == nil || (errors.As(result.err, &ocppErr) && ocppErr.Description == "Request timed out") ||
		result.err.Error() == "no outcome" {
		return done(resultDoc{messageID: id, request: asSent(), errCode: "timeout",
			errMsg: fmt.Sprintf("no response within %d ms", timeoutMs)})
	}
	return done(resultDoc{messageID: id, request: asSent(), errCode: "failed", errMsg: result.err.Error()})
}
