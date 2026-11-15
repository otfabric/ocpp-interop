// SPDX-License-Identifier: MIT
package main

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lorenzodonini/ocpp-go/ws"
)

// TestAServerThatStopsServingExits1: when ocpp-go's ws.Server stops serving,
// the server removes its ready file, writes stopped and exits 1 instead of
// waiting for a signal.
func TestAServerThatStopsServingExits1(t *testing.T) {
	dir, err := os.MkdirTemp("", "ocpp") // short: the control socket path has a length limit
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	ready, control := filepath.Join(dir, "ready"), filepath.Join(dir, "c.sock")
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	stdout := os.Stdout
	os.Stdout = w
	wsServer := ws.NewServer()
	code := make(chan int, 1)
	go func() { code <- runServer("1.6", &fixture{name: "test"}, wsServer, port, 0, ready, control) }()

	events := bufio.NewScanner(r)
	for events.Scan() {
		var ev map[string]any
		if json.Unmarshal(events.Bytes(), &ev) == nil && ev["event"] == "ready" {
			break
		}
	}
	if _, err := os.Stat(ready); err != nil {
		t.Fatalf("no ready file after the ready event: %v", err)
	}
	wsServer.Stop() // ws.Server.Start returns: the server no longer serves

	select {
	case got := <-code:
		os.Stdout = stdout
		w.Close()
		if got != 1 {
			t.Errorf("exit %d, want 1", got)
		}
	case <-time.After(20 * time.Second):
		os.Stdout = stdout
		t.Fatal("the server still runs after ws.Server stopped serving")
	}
	if _, err := os.Stat(ready); !os.IsNotExist(err) {
		t.Errorf("the ready file is still there: %v", err)
	}
}
