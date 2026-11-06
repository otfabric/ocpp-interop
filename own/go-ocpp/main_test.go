// SPDX-License-Identifier: MIT
package main

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/otfabric/go-ocpp"
	"github.com/otfabric/go-ocpp/server"
)

// fixturePath is the repository's baseline fixture of a version: ../../fixtures
// in a checkout, fixtures/ in the image build (the Dockerfile copies them).
func fixturePath(t *testing.T, version string) string {
	t.Helper()
	for _, root := range []string{"../../fixtures", "fixtures"} {
		p := filepath.Join(root, versionDirs[version], "baseline", "fixture.json")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	t.Fatalf("no baseline fixture for OCPP %s", version)
	return ""
}

func TestParseFlags(t *testing.T) {
	f, err := parseFlags([]string{"--version", "1.6", "--compression", "Heartbeat"}, "version", "compression")
	if err != nil || f.get("version", "") != "1.6" || !f.has("compression") || len(f.positional) != 1 {
		t.Fatalf("parseFlags: %v %+v", err, f)
	}
	for _, args := range [][]string{
		{"--version", "1.6", "--version", "1.6"},
		{"--nope", "1"},
		{"--version"},
		{"--compression", "--compression"},
	} {
		if _, err := parseFlags(args, "version", "compression"); err == nil {
			t.Errorf("parseFlags(%q): no error", args)
		}
	}
}

func TestExitCodes(t *testing.T) {
	for args, want := range map[string]int{
		"":                               2,
		"frobnicate":                     2,
		"server":                         2,
		"server --version 9.9":           2,
		"server --version 1.6 --port x":  2,
		"call":                           2,
		"print-capabilities extra":       2,
		"server --version 1.6 --tls-dir": 2,
	} {
		var list []string
		if args != "" {
			list = splitArgs(args)
		} else {
			list = []string{""}
		}
		if got := run(list); got != want {
			t.Errorf("run(%q) = %d, want %d", args, got, want)
		}
	}
}

func splitArgs(s string) []string {
	var out []string
	word := ""
	for _, r := range s {
		if r == ' ' {
			out = append(out, word)
			word = ""
			continue
		}
		word += string(r)
	}
	return append(out, word)
}

func TestCapabilities(t *testing.T) {
	data, err := marshal(capabilities())
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Adapter  string `json:"adapter"`
		Upstream struct {
			IndependentEngine bool `json:"independentEngine"`
		} `json:"upstream"`
		Versions map[string]struct {
			Client struct{ Sends, Handles []string } `json:"client"`
			Server struct{ Sends, Handles []string } `json:"server"`
		} `json:"versions"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Adapter != "otfabric" || doc.Upstream.IndependentEngine {
		t.Errorf("adapter %q, independentEngine %v", doc.Adapter, doc.Upstream.IndependentEngine)
	}
	for _, v := range []string{"1.6", "2.0.1", "2.1"} {
		c, ok := doc.Versions[v]
		if !ok || len(c.Client.Sends) == 0 || len(c.Server.Sends) == 0 {
			t.Fatalf("OCPP %s: %+v", v, c)
		}
		sendsSEND := false
		for _, a := range c.Client.Sends {
			if !versions[v].fromStation[a] {
				t.Errorf("OCPP %s: the station declares %s", v, a)
			}
			sendsSEND = sendsSEND || versions[v].sendActions[a]
		}
		// The SEND actions (OCPP 2.1) are declared with the send feature.
		if sendsSEND != (v == "2.1") {
			t.Errorf("OCPP %s: the station declares a SEND action: %v", v, sendsSEND)
		}
	}
}

func TestBaselineFixtures(t *testing.T) {
	for _, v := range []string{"1.6", "2.0.1", "2.1"} {
		if _, err := loadFixture(fixturePath(t, v), v); err != nil {
			t.Errorf("OCPP %s: %v", v, err)
		}
	}
}

// TestRoundTrip: a station and a CSMS of this adapter, on go-ocpp, answer
// each other from the fixture, and refuse what go-ocpp refuses.
func TestRoundTrip(t *testing.T) {
	for _, v := range []string{"1.6", "2.0.1", "2.1"} {
		t.Run(v, func(t *testing.T) {
			fx, err := loadFixture(fixturePath(t, v), v)
			if err != nil {
				t.Fatal(err)
			}
			csms := &endpoint{version: v, fixture: fx, role: "csms", tracker: newTracker(false)}
			sessions := make(chan *server.Session, 1)
			srv, err := server.New(server.WithHandler(versions[v].version, csms.dispatcher()),
				server.WithoutAuthentication(), server.WithFrameObserver(csms.tracker.observe),
				server.WithCallInterceptor(csms.tracker.intercept),
				server.WithSessionHandler(func(s *server.Session) { sessions <- s }, nil))
			if err != nil {
				t.Fatal(err)
			}
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			go func() { _ = srv.Serve(ln) }()
			defer srv.Close()

			station := &endpoint{version: v, fixture: fx, role: "station", tracker: newTracker(false)}
			conn, err := connectStation("ws://"+ln.Addr().String()+"/ocpp/"+fx.firstStation, fx.firstStation, station,
				connectOptions{}, 5*time.Second)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.client.Close()
			session := <-sessions

			doc := station.execute(fx.firstStation, "Heartbeat", map[string]any{}, 5000, conn.client, conn.closed)
			if !doc.ok || doc.messageID == "" {
				t.Fatalf("Heartbeat: %+v", doc)
			}
			reset := map[string]any{"type": "Immediate"}
			if v == "1.6" {
				reset = map[string]any{"type": "Soft"}
			}
			doc = csms.execute(fx.firstStation, "Reset", reset, 5000, session, session.Done())
			if !doc.ok {
				t.Fatalf("Reset: %+v", doc)
			}
			var resp map[string]any
			_ = json.Unmarshal(doc.response.(json.RawMessage), &resp)
			if resp["status"] != "Accepted" {
				t.Errorf("Reset: %v", resp)
			}
			doc = csms.execute(fx.firstStation, "Reset", map[string]any{"type": "Sideways"}, 5000, session, session.Done())
			if doc.errCode != "invalid-request" {
				t.Errorf("Reset Sideways: %+v", doc)
			}
			doc = station.execute(fx.firstStation, "Reset", reset, 5000, conn.client, conn.closed)
			if doc.errCode != "unsupported" {
				t.Errorf("a station sending Reset: %+v", doc)
			}
			doc = station.execute(fx.firstStation, "FrobnicateThing", map[string]any{}, 5000, conn.client, conn.closed)
			if doc.errCode != "unsupported" {
				t.Errorf("an unknown action: %+v", doc)
			}
			// A CALL the station's fixture does not answer: NotImplemented.
			_, err = csms.dispatcher().Dispatch(t.Context(), nil, "FrobnicateThing", json.RawMessage(`{}`))
			var ce *ocpp.CallError
			if !asCallError(err, &ce) || ce.Code != ocpp.NotImplemented {
				t.Errorf("an unknown action is answered %v", err)
			}
			_, err = csms.dispatcher().Dispatch(t.Context(), nil, "Reset", json.RawMessage(`{}`))
			if !asCallError(err, &ce) || ce.Code != ocpp.NotSupported {
				t.Errorf("a CSMS receiving Reset answers %v", err)
			}
		})
	}
}

func asCallError(err error, target **ocpp.CallError) bool {
	ce, ok := err.(*ocpp.CallError)
	if ok {
		*target = ce
	}
	return ok
}

// TestCompressionReported: the station's connection and the CSMS's session
// report what the handshake negotiated (connection-opened's compression).
func TestCompressionReported(t *testing.T) {
	fx, err := loadFixture(fixturePath(t, "2.0.1"), "2.0.1")
	if err != nil {
		t.Fatal(err)
	}
	for _, offer := range []bool{true, false} {
		csms := &endpoint{version: "2.0.1", fixture: fx, role: "csms", tracker: newTracker(false)}
		sessions := make(chan *server.Session, 1)
		srv, err := server.New(server.WithHandler(versions["2.0.1"].version, csms.dispatcher()),
			server.WithoutAuthentication(), server.WithSessionHandler(func(s *server.Session) { sessions <- s }, nil))
		if err != nil {
			t.Fatal(err)
		}
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		go func() { _ = srv.Serve(ln) }()
		station := &endpoint{version: "2.0.1", fixture: fx, role: "station", tracker: newTracker(false)}
		conn, err := connectStation("ws://"+ln.Addr().String()+"/ocpp/"+fx.firstStation, fx.firstStation, station,
			connectOptions{compression: offer}, 5*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		conn.mu.Lock()
		got := conn.compression
		conn.mu.Unlock()
		if got != offer {
			t.Errorf("--compression %v: the station reports %v", offer, got)
		}
		if s := <-sessions; s.Compression() != offer {
			t.Errorf("--compression %v: the CSMS reports %v", offer, s.Compression())
		}
		conn.client.Close()
		_ = srv.Close()
	}
}
