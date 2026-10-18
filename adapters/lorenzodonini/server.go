// SPDX-License-Identifier: MIT
package main

// `server`: a CSMS. OCPP 1.6: ocpp16.CentralSystem; OCPP 2.0.1: ocpp2.CSMS;
// both on an ocppj.Server over ocpp-go's ws.Server.

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
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

// The path ocpp-go's ws.Server serves (a gorilla/mux route): every path.
// ocpp-go takes the station identity from its last segment.
const listenPath = "/{path:.*}"

func commandServer(args []string) error {
	f, err := parseFlags(args, "version", "fixture", "bind-address", "port", "security-profile", "tls-dir",
		"ready-file", "control-socket")
	if err != nil {
		return err
	}
	if len(f.positional) > 0 {
		return usagef("unexpected argument %q", f.positional[0])
	}
	port, err := f.number("port", 9000, 1, 65535)
	if err != nil {
		return err
	}
	version, err := versionOf(f)
	if err != nil {
		return err
	}
	profile, err := securityProfileOf(f)
	if err != nil {
		return err
	}
	if bind := f.get("bind-address", "0.0.0.0"); bind != "0.0.0.0" {
		return unsupportedf("--bind-address %s: ocpp-go's ws.Server listens on all addresses", bind)
	}
	var wsServer *ws.Server
	if profile >= 2 {
		config, certPath, keyPath, err := serverTLS(f.get("tls-dir", ""), profile)
		if err != nil {
			return err
		}
		wsServer = ws.NewTLSServer(certPath, keyPath, config)
	} else {
		wsServer = ws.NewServer()
	}
	fx, err := fixtureOf(f, version)
	if err != nil {
		return err
	}
	return exitCode(runServer(version, fx, wsServer, int(port), profile, f.get("ready-file", readyFile),
		f.get("control-socket", controlSocket)))
}

func runServer(version string, fx *fixture, wsServer *ws.Server, port, profile int, ready, controlPath string) int {
	info := versions[version]
	tr := newTracker(true)
	var mu sync.Mutex
	sessions := map[string]chan struct{}{} // station -> closed when its connection closes

	observed := &observedServer{WsServer: wsServer, t: tr}
	dispatcher := ocppj.NewDefaultServerDispatcher(ocppj.NewFIFOQueueMap(0))
	server := ocppj.NewServer(observed, dispatcher, nil, info.profiles...)
	server.SetInvalidMessageHook(func(c ws.Channel, err *ocpp.Error, raw string, _ []any) *ocpp.Error {
		return tr.invalidMessage(c.ID(), err, raw)
	})
	e := &endpoint{version: version, fixture: fx, role: "csms", tracker: tr, setTimeout: dispatcher.SetTimeout}

	// ocpp-go's connection check: it can refuse a handshake with HTTP 401 only.
	check := func(id string, r *http.Request) bool {
		if _, known := fx.stations[id]; !known {
			event("connection-rejected", id, "reason", "unknown-station", "remote", r.RemoteAddr)
			return false
		}
		if profile == 3 {
			// The handshake verified the certificate against ca.crt; its
			// subject must name this station.
			if cn, err := peerCommonName(r.TLS); err != nil || cn != id {
				event("connection-rejected", id, "reason", "authentication", "remote", r.RemoteAddr)
				return false
			}
		}
		if profile == 1 || profile == 2 {
			user, password, ok := r.BasicAuth()
			if !ok || user != id || password != fx.password(id) {
				event("connection-rejected", id, "reason", "authentication", "remote", r.RemoteAddr)
				return false
			}
		}
		offered := websocket.Subprotocols(r)
		accepted := false
		for _, p := range offered {
			accepted = accepted || p == info.subprotocol
		}
		if !accepted {
			// ocpp-go upgrades the connection and closes it at once (1002).
			event("connection-rejected", id, "reason", "subprotocol", "remote", r.RemoteAddr, "offered", offered)
		}
		return true
	}
	opened := func(id string, remote net.Addr) {
		mu.Lock()
		sessions[id] = make(chan struct{})
		mu.Unlock()
		// ws.Server's Upgrader never negotiates permessage-deflate.
		event("connection-opened", id, "subprotocol", info.subprotocol, "remote", remote.String(), "compression", false)
	}
	closed := func(id string) {
		mu.Lock()
		if ch, ok := sessions[id]; ok {
			close(ch)
			delete(sessions, id)
		}
		mu.Unlock()
		event("connection-closed", id, "code", nil, "reason", nil)
	}

	var send func(station string) sendFunc
	var start func()
	var stop func()
	switch version {
	case "1.6":
		cs := ocpp16.NewCentralSystem(server, observed)
		h := centralSystem16{e}
		cs.SetCoreHandler(h)
		cs.SetFirmwareManagementHandler(h)
		cs.SetLocalAuthListHandler(h)
		cs.SetReservationHandler(h)
		cs.SetRemoteTriggerHandler(h)
		cs.SetSmartChargingHandler(h)
		cs.SetLogHandler(h)
		cs.SetSecurityHandler(h)
		cs.SetSecureFirmwareHandler(h)
		cs.SetNewChargingStationValidationHandler(check)
		cs.SetNewChargePointHandler(func(c ocpp16.ChargePointConnection) { opened(c.ID(), c.RemoteAddr()) })
		cs.SetChargePointDisconnectedHandler(func(c ocpp16.ChargePointConnection) { closed(c.ID()) })
		send = func(station string) sendFunc {
			return func(r ocpp.Request, cb func(ocpp.Response, error)) error { return cs.SendRequestAsync(station, r, cb) }
		}
		start = func() { cs.Start(port, listenPath) }
		stop = cs.Stop
	default:
		cs := ocpp2.NewCSMS(server, observed)
		h := csms201{e}
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
		cs.SetNewChargingStationValidationHandler(check)
		cs.SetNewChargingStationHandler(func(c ocpp2.ChargingStationConnection) { opened(c.ID(), c.RemoteAddr()) })
		cs.SetChargingStationDisconnectedHandler(func(c ocpp2.ChargingStationConnection) { closed(c.ID()) })
		send = func(station string) sendFunc {
			return func(r ocpp.Request, cb func(ocpp.Response, error)) error { return cs.SendRequestAsync(station, r, cb) }
		}
		start = func() { cs.Start(port, listenPath) }
		stop = cs.Stop
	}

	// ws.Server.Start reports a port it cannot bind only in its log, and
	// returns: check that the port is free first, and that it accepts
	// connections afterwards.
	probe, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		fmt.Fprintf(os.Stderr, "server: cannot listen on port %d: %v\n", port, err)
		return 3
	}
	probe.Close()
	returned := make(chan struct{})
	go func() {
		start()
		close(returned)
	}()
	if !listening(port, returned) {
		fmt.Fprintf(os.Stderr, "server: port %d does not accept connections\n", port)
		return 3
	}

	control, err := serveControl(controlPath, func(station *string, action string, payload map[string]any,
		timeoutMs int64) resultDoc {
		id := ""
		if station != nil {
			id = *station
		}
		mu.Lock()
		ch, ok := sessions[id]
		mu.Unlock()
		if !ok {
			return resultDoc{version: version, operation: action, request: payload, errCode: "not-connected",
				errMsg: fmt.Sprintf("no open connection for station %q", id)}
		}
		return e.execute(id, action, payload, timeoutMs, send(id), ch)
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "server: cannot open the control socket %s: %v\n", controlPath, err)
		return 3
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)
	if err := os.WriteFile(ready, []byte("ready\n"), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "server: cannot write %s: %v\n", ready, err)
		return 1
	}
	event("ready", "", "adapter", adapterName, "version", version, "port", port, "fixture", fx.name,
		"securityProfile", profile)
	<-signals

	_ = os.Remove(ready)
	control.Close()
	stop()
	// Let the connections report their close.
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		mu.Lock()
		open := len(sessions)
		mu.Unlock()
		if open == 0 {
			break
		}
	}
	event("stopped", "")
	return 0
}

func listening(port int, returned <-chan struct{}) bool {
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-returned:
			return false
		default:
		}
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 500*time.Millisecond)
		if err == nil {
			conn.Close()
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}
