// SPDX-License-Identifier: MIT
package main

// `client run` and `client <Action>`: a station, go-ocpp's client.Client.

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/otfabric/go-ocpp"
	"github.com/otfabric/go-ocpp/client"
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
	opts := connectOptions{profile: profile, compression: f.has("compression")}
	if profile >= 2 {
		if opts.tls, err = clientTLS(f.get("tls-dir", ""), profile); err != nil {
			return err
		}
	}
	fx, err := fixtureOf(f, version)
	if err != nil {
		return err
	}
	base := strings.TrimRight(url, "/")
	station := f.get("station", fx.firstStation)
	fullURL := base
	if station != "" {
		fullURL = base + "/" + station
	} else {
		// go-ocpp takes the identity from the last segment of the URL.
		station = base[strings.LastIndex(base, "/")+1:]
	}
	if _, ok := fx.stations[station]; (profile == 1 || profile == 2) && !ok {
		return usagef("--station %q is not in the fixture: security profiles 1 and 2 need its password", station)
	}
	connectTimeout := time.Duration(connectTimeoutMs) * time.Millisecond
	if run {
		return exitCode(clientRun(version, fullURL, station, fx, opts, connectTimeout, holdMs,
			f.get("ready-file", readyFile), f.get("control-socket", controlSocket)))
	}

	started := time.Now()
	e := &endpoint{version: version, fixture: fx, role: "station", tracker: newTracker(false)}
	// What the station cannot send is refused before any network access.
	if doc := e.execute(station, action, payload, timeoutMs, refuseCaller{}, nil); doc.errCode == "unsupported" {
		writeLine(doc.object())
		return exitCode(5)
	}
	conn, err := connectStation(fullURL, station, e, opts, connectTimeout)
	var doc resultDoc
	if err != nil {
		doc = resultDoc{version: version, operation: action, request: payload, errCode: "connect-failed",
			errMsg: fmt.Sprintf("%s: %v", fullURL, err)}
	} else {
		doc = e.execute(station, action, payload, timeoutMs, conn.client, conn.closed)
		conn.client.Close()
	}
	doc.elapsedMs = time.Since(started).Milliseconds()
	writeLine(doc.object())
	return exitCode(exitCodeOf(doc.errCode))
}

// refuseCaller is never called: execute refuses an unsupported action
// before it calls.
type refuseCaller struct{}

func (refuseCaller) Call(context.Context, string, any, any) error { return ocpp.ErrNotConnected }

// stationConn is an open connection of a station.
type stationConn struct {
	client *client.Client
	closed chan struct{}
	once   sync.Once
	mu     sync.Mutex
	err    error // why the connection was lost; nil after a local Close
	// compression: whether the handshake negotiated permessage-deflate
	// (go-ocpp's ConnInfo.Compression, from OnConnect).
	compression bool
}

// connectOptions are the transport settings of a station.
type connectOptions struct {
	profile     int
	tls         *tls.Config // profiles 2 and 3
	compression bool        // offer permessage-deflate
}

// connectStation connects with go-ocpp; the error says why it could not.
func connectStation(url, station string, e *endpoint, opts connectOptions, connectTimeout time.Duration) (*stationConn, error) {
	vi := versions[e.version]
	conn := &stationConn{closed: make(chan struct{})}
	m := &metrics{t: e.tracker, connected: func(info ocpp.ConnInfo) {
		conn.mu.Lock()
		conn.compression = info.Compression
		conn.mu.Unlock()
	}, disconnected: func(_ ocpp.ConnInfo, err error) {
		conn.once.Do(func() {
			conn.mu.Lock()
			conn.err = err
			conn.mu.Unlock()
			close(conn.closed)
		})
	}}
	options := []client.Option{
		client.WithHandler(vi.version, e.dispatcher()),
		client.WithVersions(vi.version),
		client.WithHandshakeTimeout(connectTimeout),
		client.WithResponseTimeout(time.Hour), // every call has the deadline of its --timeout-ms
		client.WithFrameObserver(e.tracker.observe),
		client.WithCallInterceptor(e.tracker.intercept),
		client.WithMetrics(m),
		client.WithLogger(logger()),
	}
	if opts.tls != nil {
		options = append(options, client.WithTLS(opts.tls))
	} else {
		options = append(options, client.WithInsecureTransport())
	}
	if opts.profile == 1 || opts.profile == 2 {
		options = append(options, client.WithBasicAuth(station, e.fixture.password(station)))
	}
	if opts.compression {
		options = append(options, client.WithCompression())
	}
	c, err := client.New(url, options...)
	if err != nil {
		return nil, err
	}
	conn.client = c
	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()
	if err := c.Connect(ctx); err != nil {
		c.Close()
		return nil, err
	}
	return conn, nil
}

func clientRun(version, fullURL, station string, fx *fixture, opts connectOptions, connectTimeout time.Duration,
	holdMs int64, ready, controlPath string) int {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)
	e := &endpoint{version: version, fixture: fx, role: "station", tracker: newTracker(true)}
	conn, err := connectStation(fullURL, station, e, opts, connectTimeout)
	if err != nil {
		event("connection-failed", station, "url", fullURL, "detail", err.Error())
		event("stopped", "")
		return 3
	}
	conn.mu.Lock()
	compression := conn.compression
	conn.mu.Unlock()
	event("connection-opened", station, "subprotocol", conn.client.Version().Subprotocol(), "remote", fullURL,
		"compression", compression)
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
		conn.client.Close()
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	_ = conn.client.Shutdown(ctx)
	cancel()
	// go-ocpp closes the connection with status 1000 and no reason.
	event("connection-closed", station, "code", 1000, "reason", "")
	event("stopped", "")
	return 0
}
