// SPDX-License-Identifier: MIT
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/lorenzodonini/ocpp-go/ocppj"
	"github.com/lorenzodonini/ocpp-go/ws"
)

// stdout carries JSON only (events, result documents, capabilities), one
// document per line, flushed as it is written.
var stdoutMu sync.Mutex

// object is a JSON object that keeps the order of its fields.
type object struct {
	keys   []string
	values map[string]any
}

func newObject() *object { return &object{values: map[string]any{}} }

func (o *object) set(key string, value any) *object {
	if _, ok := o.values[key]; !ok {
		o.keys = append(o.keys, key)
	}
	o.values[key] = value
	return o
}

func (o *object) get(key string) any { return o.values[key] }

func (o *object) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, k := range o.keys {
		if i > 0 {
			b.WriteByte(',')
		}
		key, _ := json.Marshal(k)
		b.Write(key)
		b.WriteByte(':')
		value, err := marshal(o.values[k])
		if err != nil {
			return nil, err
		}
		b.Write(value)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// marshal is encoding/json without HTML escaping and without the newline.
func marshal(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(b.Bytes(), "\n"), nil
}

func writeLine(document any) {
	data, err := marshal(document)
	if err != nil {
		fmt.Fprintf(os.Stderr, "output: %v\n", err)
		return
	}
	stdoutMu.Lock()
	defer stdoutMu.Unlock()
	os.Stdout.Write(append(data, '\n'))
}

func writeText(text string) {
	stdoutMu.Lock()
	defer stdoutMu.Unlock()
	os.Stdout.WriteString(text + "\n")
}

func writeRaw(data []byte) {
	stdoutMu.Lock()
	defer stdoutMu.Unlock()
	os.Stdout.Write(data)
}

func timestamp() string { return time.Now().UTC().Format("2006-01-02T15:04:05.000Z") }

// event writes one event. station may be empty; fields alternate names and values.
func event(name, station string, fields ...any) {
	o := newObject().set("event", name).set("time", timestamp())
	if station != "" {
		o.set("station", station)
	}
	for i := 0; i+1 < len(fields); i += 2 {
		o.set(fields[i].(string), fields[i+1])
	}
	writeLine(o)
}

// errorObject is {code, description, details} of a CALLERROR.
func errorObject(code, description string, details any) *object {
	if details == nil {
		details = map[string]any{}
	}
	return newObject().set("code", code).set("description", description).set("details", details)
}

// resultDoc is a result document (schemas/client-result.schema.json).
type resultDoc struct {
	version, operation string
	ok                 bool
	messageID          string
	request            any
	response           any
	callError          *object
	errCode, errMsg    string
	elapsedMs          int64
}

func (r resultDoc) object() *object {
	o := newObject().
		set("schemaVersion", schemaVersion).
		set("adapter", adapterName).
		set("version", r.version).
		set("operation", r.operation).
		set("ok", r.ok)
	if r.messageID != "" {
		o.set("messageId", r.messageID)
	}
	if r.request != nil {
		o.set("request", r.request)
	}
	o.set("response", r.response)
	if r.callError != nil {
		o.set("callError", r.callError)
	} else {
		o.set("callError", nil)
	}
	if r.errCode == "" {
		o.set("error", nil)
	} else {
		o.set("error", newObject().set("code", r.errCode).set("message", r.errMsg))
	}
	if r.elapsedMs < 0 {
		r.elapsedMs = 0
	}
	return o.set("elapsedMs", r.elapsedMs)
}

// exitCodeOf is the exit code that goes with a result document.
func exitCodeOf(errCode string) int {
	switch errCode {
	case "":
		return 0
	case "connect-failed":
		return 3
	case "unsupported":
		return 5
	default:
		return 1
	}
}

// stderrLogger is the logging.Logger ocpp-go writes to, on stderr, when
// OCPP_INTEROP_LOG_LEVEL asks for it (debug or info). By default ocpp-go
// logs nothing.
type stderrLogger struct{ debug bool }

func (l stderrLogger) out(level string, msg string) {
	fmt.Fprintf(os.Stderr, "%s %s ocpp-go: %s\n", time.Now().UTC().Format(time.RFC3339Nano), level, msg)
}
func (l stderrLogger) Debug(args ...any) {
	if l.debug {
		l.out("DEBUG", fmt.Sprint(args...))
	}
}
func (l stderrLogger) Debugf(format string, args ...any) {
	if l.debug {
		l.out("DEBUG", fmt.Sprintf(format, args...))
	}
}
func (l stderrLogger) Info(args ...any)                 { l.out("INFO", fmt.Sprint(args...)) }
func (l stderrLogger) Infof(format string, args ...any) { l.out("INFO", fmt.Sprintf(format, args...)) }
func (l stderrLogger) Error(args ...any)                { l.out("ERROR", fmt.Sprint(args...)) }
func (l stderrLogger) Errorf(format string, args ...any) {
	l.out("ERROR", fmt.Sprintf(format, args...))
}

func setupLogging() {
	switch strings.ToLower(os.Getenv("OCPP_INTEROP_LOG_LEVEL")) {
	case "debug", "trace":
		ocppj.SetLogger(stderrLogger{debug: true})
		ws.SetLogger(stderrLogger{debug: true})
	case "info":
		ocppj.SetLogger(stderrLogger{})
		ws.SetLogger(stderrLogger{})
	}
}
