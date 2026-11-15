// SPDX-License-Identifier: MIT
package main

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/lorenzodonini/ocpp-go/ocpp"
	"github.com/lorenzodonini/ocpp-go/ocppj"
)

// queuedDispatcher does what ocpp-go's dispatcher does with the CALLs of one
// station: it writes them one at a time, in the order they were sent, and
// reads its one timeout right after it wrote a CALL (ocppj/dispatcher.go,
// dispatchNextRequest). The first CALL stays unanswered until release is
// closed; then every CALL fails as timed out, one after the other.
type queuedDispatcher struct {
	tr      *tracker
	station string
	timeout time.Duration // SetTimeout; read by the pump without a lock, as in ocpp-go
	queue   chan queuedCall
	sent    chan struct{} // one value per CALL sent
	release chan struct{}

	mu  sync.Mutex
	got map[string]time.Duration // message id -> the timeout the CALL was written with
}

type queuedCall struct {
	id, action string
	done       func(ocpp.Response, error)
}

func (d *queuedDispatcher) send(request ocpp.Request, done func(ocpp.Response, error)) error {
	d.queue <- queuedCall{id: nextMessageID(), action: request.GetFeatureName(), done: done}
	d.sent <- struct{}{}
	return nil
}

func (d *queuedDispatcher) pump() {
	first := true
	for c := range d.queue {
		d.tr.outgoing(d.station, []byte(fmt.Sprintf(`[2,%q,%q,{}]`, c.id, c.action)))
		timeout := d.timeout
		d.mu.Lock()
		d.got[c.id] = timeout
		d.mu.Unlock()
		if first {
			<-d.release
			first = false
		}
		c.done(nil, ocpp.NewError(ocppj.GenericError, "Request timed out", c.id))
	}
}

// TestEveryCallKeepsItsOwnTimeout: CALLs that wait in ocpp-go's queue behind
// another are written with their own --timeout-ms.
func TestEveryCallKeepsItsOwnTimeout(t *testing.T) {
	tr := newTracker(false)
	d := &queuedDispatcher{tr: tr, station: "CP001", queue: make(chan queuedCall, 8), sent: make(chan struct{}, 8),
		release: make(chan struct{}), got: map[string]time.Duration{}}
	e := &endpoint{version: "1.6", role: "csms", tracker: tr, setTimeout: func(t time.Duration) { d.timeout = t }}
	pumped := make(chan struct{})
	go func() {
		d.pump()
		close(pumped)
	}()
	closed := make(chan struct{})

	// The first CALL is written at once; the others wait behind it.
	timeouts := []int64{60000, 1000, 30000, 2000}
	docs := make([]resultDoc, len(timeouts))
	var wg sync.WaitGroup
	for i, ms := range timeouts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			docs[i] = e.execute("CP001", "ClearCache", map[string]any{}, ms, d.send, closed)
		}()
		<-d.sent // sent before the next one
	}
	close(d.release)
	wg.Wait()
	close(d.queue)
	<-pumped

	d.mu.Lock()
	defer d.mu.Unlock()
	for i, ms := range timeouts {
		if docs[i].errCode != "timeout" {
			t.Errorf("CALL %d: error %q, want timeout", i, docs[i].errCode)
		}
		if got, want := d.got[docs[i].messageID], time.Duration(ms)*time.Millisecond; got != want {
			t.Errorf("CALL %d (--timeout-ms %d) was written with the timeout %v", i, ms, got)
		}
	}
}
