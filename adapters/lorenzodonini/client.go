// SPDX-License-Identifier: MIT
package main

// `client run` and `client <Action>`: a station. OCPP 1.6:
// ocpp16.ChargePoint; OCPP 2.0.1: ocpp2.ChargingStation; both on an
// ocppj.Client over ocpp-go's ws.Client.

import (
	"crypto/tls"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/lorenzodonini/ocpp-go/ocpp"
	ocpp16 "github.com/lorenzodonini/ocpp-go/ocpp1.6"
	ocpp2 "github.com/lorenzodonini/ocpp-go/ocpp2.0.1"
	"github.com/lorenzodonini/ocpp-go/ocppj"
	"github.com/lorenzodonini/ocpp-go/ws"
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
	station := f.get("station", fx.firstStation)
	if _, ok := fx.stations[station]; (profile == 1 || profile == 2) && !ok {
		return usagef("--station %q is not in the fixture: security profiles 1 and 2 need its password", station)
	}
	base := strings.TrimRight(url, "/")
	fullURL := base
	if station != "" {
		fullURL = base + "/" + station
	}
	connectTimeout := time.Duration(connectTimeoutMs) * time.Millisecond
	if run {
		return exitCode(clientRun(version, base, fullURL, station, fx, opts, connectTimeout, holdMs,
			f.get("ready-file", readyFile), f.get("control-socket", controlSocket)))
	}

	started := time.Now()
	unsupported := ""
	if feature(version, action) == nil {
		unsupported = fmt.Sprintf("ocpp-go has no OCPP %s feature for %s", version, action)
	} else if !versions[version].fromStation[action] {
		unsupported = fmt.Sprintf("the station does not send %s in OCPP %s", action, version)
	}
	if unsupported != "" {
		writeLine(resultDoc{version: version, operation: action, request: payload, errCode: "unsupported",
			errMsg: unsupported}.object())
		return exitCode(5)
	}
	conn, err := connectStation(version, base, station, fx, opts, connectTimeout, false)
	var doc resultDoc
	if err != nil {
		doc = resultDoc{version: version, operation: action, request: payload, errCode: "connect-failed",
			errMsg: fmt.Sprintf("%s: %v", fullURL, err)}
	} else {
		doc = conn.e.execute(station, action, payload, timeoutMs, conn.send, conn.closed)
		conn.stop()
	}
	doc.elapsedMs = time.Since(started).Milliseconds()
	writeLine(doc.object())
	return exitCode(exitCodeOf(doc.errCode))
}

// stationConn is an open connection of a station.
type stationConn struct {
	e      *endpoint
	send   sendFunc
	stop   func()
	closed chan struct{}
	mu     sync.Mutex
	code   any
	reason any
}

// connectOptions are the transport settings of a station.
type connectOptions struct {
	profile     int
	tls         *tls.Config // profiles 2 and 3
	compression bool        // offer permessage-deflate
}

// connectStation connects with ocpp-go; the error says why it could not.
func connectStation(version, base, station string, fx *fixture, opts connectOptions, connectTimeout time.Duration,
	emit bool) (*stationConn, error) {
	info := versions[version]
	tr := newTracker(emit)
	var wsClient *ws.Client
	if opts.tls != nil {
		wsClient = ws.NewTLSClient(opts.tls)
	} else {
		wsClient = ws.NewClient()
	}
	timeouts := ws.NewClientTimeoutConfig()
	timeouts.HandshakeTimeout = connectTimeout
	wsClient.SetTimeoutConfig(timeouts)
	if opts.profile == 1 || opts.profile == 2 {
		wsClient.SetBasicAuth(station, fx.password(station))
	}
	if opts.compression {
		wsClient.AddOption(func(dialer *websocket.Dialer) { dialer.EnableCompression = true })
	}
	observed := &observedClient{WsClient: wsClient, t: tr, station: station}
	dispatcher := ocppj.NewDefaultClientDispatcher(ocppj.NewFIFOClientQueue(0))
	client := ocppj.NewClient(station, observed, dispatcher, nil, info.profiles...)
	client.SetInvalidMessageHook(func(err *ocpp.Error, raw string, _ []any) *ocpp.Error {
		return tr.invalidMessage(station, err, raw)
	})
	conn := &stationConn{closed: make(chan struct{})}
	conn.e = &endpoint{version: version, fixture: fx, role: "station", tracker: tr, setTimeout: dispatcher.SetTimeout}

	var start func(string) error
	switch version {
	case "1.6":
		cp := ocpp16.NewChargePoint(station, client, observed)
		h := chargePoint16{conn.e}
		cp.SetCoreHandler(h)
		cp.SetLocalAuthListHandler(h)
		cp.SetFirmwareManagementHandler(h)
		cp.SetReservationHandler(h)
		cp.SetRemoteTriggerHandler(h)
		cp.SetSmartChargingHandler(h)
		cp.SetSecurityHandler(h)
		cp.SetLogHandler(h)
		cp.SetExtendedTriggerMessageHandler(h)
		cp.SetSecureFirmwareHandler(h)
		cp.SetCertificateHandler(h)
		conn.send, start, conn.stop = cp.SendRequestAsync, cp.Start, cp.Stop
	default:
		cs := ocpp2.NewChargingStation(station, client, observed)
		h := chargingStation201{conn.e}
		cs.SetAuthorizationHandler(h)
		cs.SetAvailabilityHandler(h)
		cs.SetDataHandler(h)
		cs.SetDiagnosticsHandler(h)
		cs.SetDisplayHandler(h)
		cs.SetFirmwareHandler(h)
		cs.SetISO15118Handler(h)
		cs.SetLocalAuthListHandler(h)
		cs.SetMeterHandler(h)
		cs.SetProvisioningHandler(h)
		cs.SetRemoteControlHandler(h)
		cs.SetReservationHandler(h)
		cs.SetSecurityHandler(h)
		cs.SetSmartChargingHandler(h)
		cs.SetTariffCostHandler(h)
		cs.SetTransactionsHandler(h)
		conn.send, start, conn.stop = cs.SendRequestAsync, cs.Start, cs.Stop
	}
	// A connection that the CSMS closes, or that breaks. (ocpp-go would then
	// reconnect by itself; the contract's station reports it and stops.)
	var once sync.Once
	client.SetOnDisconnectedHandler(func(err error) {
		once.Do(func() {
			conn.mu.Lock()
			var closeErr *websocket.CloseError
			if errors.As(err, &closeErr) {
				conn.code, conn.reason = closeErr.Code, closeErr.Text
			} else {
				conn.code, conn.reason = websocket.CloseAbnormalClosure, ""
			}
			conn.mu.Unlock()
			close(conn.closed)
		})
	})
	if err := start(base); err != nil {
		return nil, err
	}
	return conn, nil
}

func clientRun(version, base, fullURL, station string, fx *fixture, opts connectOptions, connectTimeout time.Duration,
	holdMs int64, ready, controlPath string) int {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)
	conn, err := connectStation(version, base, station, fx, opts, connectTimeout, true)
	if err != nil {
		event("connection-failed", station, "url", fullURL, "detail", err.Error())
		event("stopped", "")
		return 3
	}
	// ws.Client exposes neither the negotiated subprotocol nor whether
	// permessage-deflate was negotiated.
	var compression any
	if !opts.compression {
		compression = false
	}
	event("connection-opened", station, "subprotocol", nil, "remote", fullURL, "compression", compression)
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
		return conn.e.execute(station, action, payload, timeoutMs, conn.send, conn.closed)
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "client: cannot open the control socket %s: %v\n", controlPath, err)
		conn.stop()
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
		event("connection-closed", station, "code", conn.code, "reason", conn.reason)
		conn.mu.Unlock()
		event("stopped", "")
		return 1
	}
	conn.stop()
	// The close handshake of a connection the station ends is not exposed.
	event("connection-closed", station, "code", nil, "reason", nil)
	event("stopped", "")
	return 0
}
