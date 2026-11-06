// SPDX-License-Identifier: MIT
package main

// `client run` and `client <Action>`: a station, cB-Sigma's chargepoint.Client.

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/cB-Sigma/ocpp/chargepoint"
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
	url := f.get("url", "")
	if !strings.HasPrefix(url, "ws://") && !strings.HasPrefix(url, "wss://") {
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
	if strings.HasPrefix(url, "wss://") != (profile >= 2) {
		return usagef("--url must be wss:// with security profiles 2 and 3, and ws:// otherwise")
	}
	opts := connectOptions{profile: profile}
	if profile >= 2 {
		if opts.tls, err = clientTLS(f.get("tls-dir", ""), profile); err != nil {
			return err
		}
	}
	if f.has("compression") {
		// chargepoint.Config has no way to offer permessage-deflate.
		return unsupportedError{"--compression: cB-Sigma/ocpp's station cannot offer permessage-deflate"}
	}
	if f.has("station") && f.get("station", "") == "" {
		return unsupportedError{"--station \"\": cB-Sigma/ocpp always appends the station identity to the URL"}
	}
	fx, err := fixtureOf(f, version)
	if err != nil {
		return err
	}
	base := strings.TrimRight(url, "/")
	station := f.get("station", fx.firstStation)
	if _, ok := fx.stations[station]; (profile == 1 || profile == 2) && !ok {
		return usagef("--station %q is not in the fixture: security profiles 1 and 2 need its password", station)
	}
	fullURL := chargepoint.EndpointURL(base, station)
	connectTimeout := time.Duration(connectTimeoutMs) * time.Millisecond
	if run {
		return exitCode(clientRun(version, base, fullURL, station, fx, opts, connectTimeout, holdMs,
			f.get("ready-file", readyFile), f.get("control-socket", controlSocket)))
	}

	started := time.Now()
	e := newEndpoint(version, fx, "station", false)
	// What the station cannot send is refused before any network access.
	if doc := e.execute(station, action, payload, timeoutMs, nil, nil); doc.errCode == "unsupported" {
		writeLine(doc.object())
		return exitCode(5)
	}
	conn, err := connectStation(base, station, e, opts, connectTimeout)
	var doc resultDoc
	if err != nil {
		doc = resultDoc{version: version, operation: action, request: payload, errCode: "connect-failed",
			errMsg: fmt.Sprintf("%s: %v", fullURL, err)}
	} else {
		doc = e.execute(station, action, payload, timeoutMs, conn.client, conn.closed)
		conn.close("")
	}
	doc.elapsedMs = time.Since(started).Milliseconds()
	writeLine(doc.object())
	return exitCode(exitCodeOf(doc.errCode))
}

// stationConn is an open connection of a station.
type stationConn struct {
	client *chargepoint.Client
	cancel context.CancelFunc // ends the connection's serving
	closed chan struct{}
	once   sync.Once
	mu     sync.Mutex
	err    error // why the connection was lost; nil after a local close
	local  bool  // the adapter closed it
}

func (c *stationConn) close(reason string) {
	c.mu.Lock()
	c.local = true
	c.mu.Unlock()
	c.client.Close(reason)
	select {
	case <-c.closed:
	case <-time.After(5 * time.Second):
	}
	c.cancel()
}

// connectOptions are the transport settings of a station.
type connectOptions struct {
	profile int
	tls     *tls.Config // profiles 2 and 3
}

// connectStation connects with cB-Sigma; the error says why it could not.
func connectStation(base, station string, e *endpoint, opts connectOptions, connectTimeout time.Duration) (*stationConn, error) {
	vi := versions[e.version]
	conn := &stationConn{closed: make(chan struct{})}
	cfg := chargepoint.Config{
		URL:      base,
		ID:       station,
		Versions: []string{vi.wire},
		TLS:      opts.tls,
		// Every CALL has the deadline of its --timeout-ms.
		Timeout: time.Hour,
		// cB-Sigma checks every request and answer against the OCA schemas;
		// a fixture with invalidAnswers needs its answers sent as they are.
		SkipValidation: e.invalidAnswers(),
		OnDisconnect: func(_ *chargepoint.Client, err error) {
			conn.once.Do(func() {
				conn.mu.Lock()
				conn.err = err
				conn.mu.Unlock()
				close(conn.closed)
			})
		},
		OnResultError: e.onResultError(station),
	}
	if opts.profile == 1 || opts.profile == 2 {
		cfg.Password = e.fixture.password(station)
	}
	c := chargepoint.New(cfg)
	all := map[string]bool{}
	for a := range vi.fromStation {
		all[a] = true
	}
	for a := range vi.fromCSMS {
		all[a] = true
	}
	for action := range all {
		action := action
		c.HandleVersion(vi.wire, action, func(ctx context.Context, _ *chargepoint.Client, payload json.RawMessage) (any, error) {
			return e.handle(ctx, station, action, payload)
		})
	}
	conn.client = c
	// Connect serves the connection until its context ends: the context
	// lives as long as the connection, and the handshake has a timer of its
	// own.
	ctx, cancel := context.WithCancel(context.Background())
	conn.cancel = cancel
	result := make(chan error, 1)
	go func() { result <- c.Connect(ctx) }()
	select {
	case err := <-result:
		if err != nil {
			cancel()
			return nil, err
		}
	case <-time.After(connectTimeout):
		cancel()
		return nil, errors.New("the WebSocket handshake did not complete within --connect-timeout-ms")
	}
	return conn, nil
}

func clientRun(version, base, fullURL, station string, fx *fixture, opts connectOptions, connectTimeout time.Duration,
	holdMs int64, ready, controlPath string) int {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)
	e := newEndpoint(version, fx, "station", true)
	conn, err := connectStation(base, station, e, opts, connectTimeout)
	if err != nil {
		event("connection-failed", station, "url", fullURL, "detail", err.Error())
		event("stopped", "")
		return 3
	}
	// chargepoint.Config cannot offer permessage-deflate.
	event("connection-opened", station, "subprotocol", conn.client.Version(), "remote", fullURL, "compression", false)
	control, err := serveControl(controlPath, func(wanted *string, action string, payload map[string]any,
		timeoutMs int64) resultDoc {
		select {
		case <-conn.closed:
			return resultDoc{version: version, operation: action, request: payload, errCode: "not-connected",
				errMsg: "the connection is closed"}
		default:
		}
		if wanted != nil && *wanted != "" && *wanted != station {
			return resultDoc{version: version, operation: action, request: payload, errCode: "not-connected",
				errMsg: fmt.Sprintf("no open connection for station %q", *wanted)}
		}
		return e.execute(station, action, payload, timeoutMs, conn.client, conn.closed)
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "client: cannot open the control socket %s: %v\n", controlPath, err)
		conn.close("")
		return 3
	}
	if err := os.WriteFile(ready, []byte("ready\n"), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "client: cannot write %s: %v\n", ready, err)
		return 1
	}
	event("ready", station, "version", version, "url", fullURL)

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
		event("connection-closed", station, "code", code, "reason", reason)
		event("stopped", "")
		return 1
	}
	// cB-Sigma closes with status 1000 and the reason given.
	conn.close("")
	event("connection-closed", station, "code", 1000, "reason", "")
	event("stopped", "")
	return 0
}
