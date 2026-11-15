// SPDX-License-Identifier: MIT
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"testing"
)

// events runs f with stdout captured and returns the JSON lines it wrote.
func events(t *testing.T, f func()) []map[string]any {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = stdout }()
	lines := make(chan []map[string]any)
	go func() {
		var out []map[string]any
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			var line map[string]any
			if json.Unmarshal(scanner.Bytes(), &line) == nil {
				out = append(out, line)
			}
		}
		lines <- out
	}()
	f()
	os.Stdout = stdout
	w.Close()
	out := <-lines
	r.Close()
	return out
}

// TestCallAnsweredReportsWhatIsSent: an answer of the fixture that cB-Sigma
// refuses after the handler is answered with an InternalError CALLERROR, and
// call-answered reports that CALLERROR, not the payload.
func TestCallAnsweredReportsWhatIsSent(t *testing.T) {
	for _, c := range []struct {
		name, version, role, action string
		answer                      map[string]any
		invalidAnswers              bool
		refused                     bool
	}{
		{name: "1.6 CSMS, valid", version: "1.6", role: "csms", action: "Heartbeat",
			answer: map[string]any{"currentTime": "2026-11-15T00:00:00Z"}},
		{name: "1.6 CSMS, outside the schema", version: "1.6", role: "csms", action: "Heartbeat",
			answer: map[string]any{"currentTime": 5}, refused: true},
		{name: "2.0.1 CSMS, outside the schema", version: "2.0.1", role: "csms", action: "Heartbeat",
			answer: map[string]any{"currentTime": 5}, refused: true},
		{name: "1.6 station, outside the schema", version: "1.6", role: "station", action: "ClearCache",
			answer: map[string]any{"status": "Bogus"}, refused: true},
		{name: "2.0.1 station, valid", version: "2.0.1", role: "station", action: "ClearCache",
			answer: map[string]any{"status": "Accepted"}},
		{name: "2.1 CSMS, invalidAnswers (no check)", version: "2.1", role: "csms", action: "Heartbeat",
			answer: map[string]any{"currentTime": 5}, invalidAnswers: true},
	} {
		t.Run(c.name, func(t *testing.T) {
			table := map[string]any{c.role: map[string]any{c.action: map[string]any{"payload": c.answer}}}
			doc := map[string]any{"answers": table}
			if c.invalidAnswers {
				doc = map[string]any{"answers": map[string]any{c.role: map[string]any{}}, "invalidAnswers": table}
			}
			e := newEndpoint(c.version, &fixture{doc: doc, version: c.version}, c.role, true)
			var err error
			got := events(t, func() {
				_, err = e.handle(context.Background(), "CP001", c.action, json.RawMessage(`{}`))
			})
			var answered map[string]any
			for _, ev := range got {
				if ev["event"] == "call-answered" {
					answered = ev
				}
			}
			if answered == nil {
				t.Fatalf("no call-answered event in %v", got)
			}
			if c.refused {
				ce, _ := answered["error"].(map[string]any)
				if err == nil || ce["code"] != "InternalError" || answered["payload"] != nil {
					t.Fatalf("handle error %v, call-answered %v: want the InternalError CALLERROR", err, answered)
				}
				return
			}
			if err != nil || answered["error"] != nil || answered["payload"] == nil {
				t.Fatalf("handle error %v, call-answered %v: want the payload", err, answered)
			}
		})
	}
}
