// SPDX-License-Identifier: MIT
package main

// `client run` and `client <Action>`: a station, seanlee0923/ocpp's station.Station.

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/seanlee0923/ocpp/station"
)

func commandClient(args []string) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "--") {
		return usagef("client needs 'run' or an action")
	}
	mode, run := args[0], args[0] == "run"
	known := []string{"version", "url", "station", "fixture", "security-profile", "tls-dir", "compression",
		"connect-timeout-ms", "timeout-ms"}
	if run {
		known = append(known, "hold-ms", "ready-file", "control-socket")
	} else {
		known = append(known, "payload")
	}
	f, err := parseFlags(args[1:], known...)
	if err != nil {
		return err
	}
	if len(f.positional) > 0 {
		return usagef("unexpected argument %q", f.positional[0])
	}
	action := ""
	if !run {
		if action, err = actionOf(mode); err != nil {
			return err
		}
	}
	if !f.has("url") {
		return usagef("--url is required")
	}
	rawURL := f.get("url", "")
	if !strings.HasPrefix(rawURL, "ws://") && !strings.HasPrefix(rawURL, "wss://") {
		return usagef("--url must start with ws:// or wss://")
	}
	connectTimeoutMs, err := f.number("connect-timeout-ms", 5000, 1, 3600000)
	if err != nil {
		return err
	}
	timeoutMs, err := f.number("timeout-ms", 10000, 1, 3600000)
	if err != nil {
		return err
	}
	var holdMs int64
	if run {
		if holdMs, err = f.number("hold-ms", 0, 0, 1<<31); err != nil {
			return err
		}
	}
	payload := map[string]any{}
	if !run {
		if payload, err = payloadOf(f); err != nil {
			return err
		}
	}
	version, err := versionOf(f)
	if err != nil {
		return err
	}
	profile, err := securityProfileOf(f)
	if err != nil {
		return err
	}
	if strings.HasPrefix(rawURL, "wss://") != (profile >= 2) {
		return usagef("--url must be wss:// with security profiles 2 and 3, and ws:// otherwise")
	}
	opts := connectOptions{profile: profile}
	if profile >= 2 {
		if opts.tls, err = clientTLS(f.get("tls-dir", ""), profile); err != nil {
			return err
		}
	}
	if f.has("compression") {
		// station.Config has no way to offer permessage-deflate.
		return unsupportedError{"--compression: seanlee0923/ocpp's station cannot offer permessage-deflate"}
	}
	if f.has("station") && f.get("station", "") == "" {
		return unsupportedError{"--station \"\": seanlee0923/ocpp's station requires an identity and appends it to the URL"}
	}
	fx, err := fixtureOf(f, version)
	if err != nil {
		return err
	}
	base := strings.TrimRight(rawURL, "/")
	id := f.get("station", fx.firstStation)
	if _, ok := fx.stations[id]; (profile == 1 || profile == 2) && !ok {
		return usagef("--station %q is not in the fixture: security profiles 1 and 2 need its password", id)
	}
	fullURL := base + "/" + url.PathEscape(id)
	connectTimeout := time.Duration(connectTimeoutMs) * time.Millisecond
	if run {
		return exitCode(clientRun(version, base, fullURL, id, fx, opts, connectTimeout, holdMs,
			f.get("ready-file", readyFile), f.get("control-socket", controlSocket)))
	}

	started := time.Now()
	e := newEndpoint(version, fx, "station", false)
	// What the station cannot send is refused before any network access.
	if doc := e.execute(id, action, payload, timeoutMs, nil, nil); doc.errCode == "unsupported" {
		writeLine(doc.object())
		return exitCode(5)
	}
	conn, err := connectStation(base, id, e, opts, connectTimeout)
	var doc resultDoc
	if err != nil {
		doc = resultDoc{version: version, operation: action, request: payload, errCode: "connect-failed",
			errMsg: fmt.Sprintf("%s: %v", fullURL, err)}
	} else {
		doc = e.execute(id, action, payload, timeoutMs, &target{station: conn.station}, conn.closed)
		conn.close()
	}
	doc.elapsedMs = time.Since(started).Milliseconds()
	writeLine(doc.object())
	return exitCode(exitCodeOf(doc.errCode))
}

// stationConn is an open connection of a station.
type stationConn struct {
	station *station.Station
	cancel  context.CancelFunc // ends Run
	stopped chan struct{}      // Run returned
	closed  chan struct{}      // the connection is gone
	once    sync.Once
	mu      sync.Mutex
	err     error // why the connection was lost
}

// close stops the station: seanlee0923/ocpp closes the TCP connection
// without a close frame.
func (c *stationConn) close() {
	c.station.Stop()
	select {
	case <-c.stopped:
	case <-time.After(5 * time.Second):
	}
	c.cancel()
}

// connectOptions are the transport settings of a station.
type connectOptions struct {
	profile int
	tls     *tls.Config // profiles 2 and 3
}

// connectStation connects with seanlee0923/ocpp, one attempt (no
// ReconnectPolicy); the error says why it could not.
func connectStation(base, id string, e *endpoint, opts connectOptions, connectTimeout time.Duration) (*stationConn, error) {
	vi := versions[e.version]
	conn := &stationConn{closed: make(chan struct{}), stopped: make(chan struct{})}
	connected := make(chan struct{})
	cfg := station.Config{
		URL:       base,
		Identity:  id,
		Version:   vi.wire,
		TLSConfig: opts.tls,
		// Every CALL has the deadline of its --timeout-ms.
		CallTimeout:      time.Hour,
		HandshakeTimeout: connectTimeout,
		OnConnect:        func(*station.Station) { close(connected) },
		OnDisconnect: func(_ *station.Station, err error) {
			conn.once.Do(func() {
				conn.mu.Lock()
				conn.err = err
				conn.mu.Unlock()
				close(conn.closed)
			})
		},
	}
	if opts.profile == 1 || opts.profile == 2 {
		cfg.BasicAuth = &station.BasicCredentials{Username: id, Password: e.fixture.password(id)}
	}
	st, err := station.New(cfg)
	if err != nil {
		return nil, err
	}
	for action, ops := range actionTable[e.version] {
		if !e.receives()[action] || !knows(e.version, action) || ops.send {
			continue
		}
		if err := ops.stationHandle(st, e.answerer(id, action)); err != nil {
			return nil, fmt.Errorf("cannot register %s: %w", action, err)
		}
	}
	conn.station = st
	ctx, cancel := context.WithCancel(context.Background())
	conn.cancel = cancel
	result := make(chan error, 1)
	go func() {
		result <- st.Run(ctx)
		close(conn.stopped)
	}()
	select {
	case <-connected:
		return conn, nil
	case err := <-result:
		cancel()
		if err == nil {
			err = errors.New("the station stopped before it connected")
		}
		return nil, err
	case <-time.After(connectTimeout):
		st.Stop()
		cancel()
		return nil, errors.New("the WebSocket handshake did not complete within --connect-timeout-ms")
	}
}

func clientRun(version, base, fullURL, id string, fx *fixture, opts connectOptions, connectTimeout time.Duration,
	holdMs int64, ready, controlPath string) int {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)
	e := newEndpoint(version, fx, "station", true)
	conn, err := connectStation(base, id, e, opts, connectTimeout)
	if err != nil {
		event("connection-failed", id, "url", fullURL, "detail", err.Error())
		event("stopped", "")
		return 3
	}
	// station.Config cannot offer permessage-deflate.
	event("connection-opened", id, "subprotocol", string(conn.station.Version()), "remote", fullURL,
		"compression", false)
	control, err := serveControl(controlPath, func(wanted *string, action string, payload map[string]any,
		timeoutMs int64) resultDoc {
		select {
		case <-conn.closed:
			return resultDoc{version: version, operation: action, request: payload, errCode: "not-connected",
				errMsg: "the connection is closed"}
		default:
		}
		if wanted != nil && *wanted != "" && *wanted != id {
			return resultDoc{version: version, operation: action, request: payload, errCode: "not-connected",
				errMsg: fmt.Sprintf("no open connection for station %q", *wanted)}
		}
		return e.execute(id, action, payload, timeoutMs, &target{station: conn.station}, conn.closed)
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "client: cannot open the control socket %s: %v\n", controlPath, err)
		conn.close()
		return 3
	}
	if err := os.WriteFile(ready, []byte("ready\n"), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "client: cannot write %s: %v\n", ready, err)
		return 1
	}
	event("ready", id, "version", version, "url", fullURL)

	var hold <-chan time.Time
	if holdMs > 0 {
		hold = time.After(time.Duration(holdMs) * time.Millisecond)
	}
	lost := false
	select {
	case <-signals:
	case <-hold:
	case <-conn.closed:
		lost = true
	}
	_ = os.Remove(ready)
	control.Close()
	if lost {
		conn.mu.Lock()
		code, reason := closeOf(conn.err)
		conn.mu.Unlock()
		event("connection-closed", id, "code", code, "reason", reason)
		conn.close()
		event("stopped", "")
		return 1
	}
	// seanlee0923/ocpp's Stop closes the TCP connection without a close
	// frame: no code or reason.
	conn.close()
	event("connection-closed", id, "code", nil, "reason", nil)
	event("stopped", "")
	return 0
}
