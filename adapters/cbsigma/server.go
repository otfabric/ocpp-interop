// SPDX-License-Identifier: MIT
package main

// `server`: a CSMS, cB-Sigma's csms.Server for one OCPP version.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/cB-Sigma/ocpp"
	"github.com/cB-Sigma/ocpp/csms"
	"github.com/coder/websocket"
)

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
	bind := f.get("bind-address", "0.0.0.0")
	if net.ParseIP(bind) == nil {
		return usagef("--bind-address: %q is not an IP address", bind)
	}
	version, err := versionOf(f)
	if err != nil {
		return err
	}
	profile, err := securityProfileOf(f)
	if err != nil {
		return err
	}
	var tlsDir string
	if profile >= 2 {
		tlsDir = f.get("tls-dir", "")
		if _, err := serverTLS(tlsDir, profile); err != nil {
			return err
		}
	}
	fx, err := fixtureOf(f, version)
	if err != nil {
		return err
	}
	return exitCode(runServer(version, fx, bind, int(port), profile, tlsDir, f.get("ready-file", readyFile),
		f.get("control-socket", controlSocket)))
}

// offers reports whether the opening handshake offers the subprotocol.
func offers(values []string, subprotocol string) bool {
	for _, v := range values {
		for _, token := range strings.Split(v, ",") {
			if strings.TrimSpace(token) == subprotocol {
				return true
			}
		}
	}
	return false
}

// offersDeflate reports whether the handshake offers permessage-deflate.
func offersDeflate(r *http.Request) bool {
	for _, v := range r.Header.Values("Sec-WebSocket-Extensions") {
		for _, ext := range strings.Split(v, ",") {
			name, _, _ := strings.Cut(ext, ";")
			if strings.EqualFold(strings.TrimSpace(name), "permessage-deflate") {
				return true
			}
		}
	}
	return false
}

func runServer(version string, fx *fixture, bind string, port, profile int, tlsDir, ready, controlPath string) int {
	vi := versions[version]
	e := newEndpoint(version, fx, "csms", true)

	var mu sync.Mutex
	type handshake struct {
		remote   string
		compress any // false when not offered; null when offered (cB-Sigma does not say whether it was negotiated)
	}
	handshakes := map[string]handshake{} // station -> its last admitted handshake
	open := map[*csms.Station]bool{}

	srv := csms.New(csms.Config{
		Versions: []string{vi.wire},
		// Every CALL has the deadline of its --timeout-ms.
		Timeout: time.Hour,
		// cB-Sigma checks every request and answer against the OCA schemas;
		// a fixture with invalidAnswers needs its answers sent as they are.
		SkipValidation: e.invalidAnswers(),
		// The fixture decides who may connect. cB-Sigma refuses with HTTP
		// 401 for any error.
		Authenticate: func(r *http.Request, id string) error {
			if _, known := fx.stations[id]; !known {
				event("connection-rejected", id, "reason", "unknown-station", "remote", r.RemoteAddr)
				return errors.New("unknown station")
			}
			switch profile {
			case 1, 2:
				pass, ok := ocpp.BasicAuthPassword(r, id)
				if !ok || !ocpp.PasswordOK(fx.password(id), pass) {
					event("connection-rejected", id, "reason", "authentication", "remote", r.RemoteAddr)
					return errors.New("wrong credentials")
				}
			case 3:
				// The handshake verified the certificate against ca.crt; its
				// subject must name this station.
				if !ocpp.ClientCertNames(r.TLS, id) {
					event("connection-rejected", id, "reason", "authentication", "remote", r.RemoteAddr)
					return errors.New("the client certificate is not this station's")
				}
			}
			offered := r.Header.Values("Sec-WebSocket-Protocol")
			if !offers(offered, vi.wire) {
				// cB-Sigma refuses the handshake (no subprotocol) or closes
				// the connection at once (another one).
				event("connection-rejected", id, "reason", "subprotocol", "remote", r.RemoteAddr,
					"offered", ocpp.OfferedSubprotocols(r))
				return nil
			}
			var compress any = false
			if offersDeflate(r) {
				compress = nil
			}
			mu.Lock()
			handshakes[id] = handshake{remote: r.RemoteAddr, compress: compress}
			mu.Unlock()
			return nil
		},
		OnConnect: func(st *csms.Station) {
			st.Conn().OnResultError = e.onResultError(st.ID())
			mu.Lock()
			h := handshakes[st.ID()]
			open[st] = true
			mu.Unlock()
			event("connection-opened", st.ID(), "subprotocol", st.Version(), "remote", nullable(h.remote),
				"compression", h.compress)
		},
		OnDisconnect: func(st *csms.Station, err error) {
			mu.Lock()
			wasOpen := open[st]
			delete(open, st)
			mu.Unlock()
			if wasOpen {
				code, reason := closeOf(err)
				event("connection-closed", st.ID(), "code", code, "reason", reason)
			}
		},
		// A CSMS accepts permessage-deflate whenever it is offered.
		AcceptOptions: websocket.AcceptOptions{CompressionMode: websocket.CompressionNoContextTakeover},
	})
	all := map[string]bool{}
	for a := range vi.fromStation {
		all[a] = true
	}
	for a := range vi.fromCSMS {
		all[a] = true
	}
	for action := range all {
		action := action
		srv.HandleVersion(vi.wire, action, func(ctx context.Context, st *csms.Station, payload json.RawMessage) (any, error) {
			return e.handle(ctx, st.ID(), action, payload)
		})
	}

	listener, err := net.Listen("tcp", net.JoinHostPort(bind, fmt.Sprint(port)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "server: cannot listen on %s port %d: %v\n", bind, port, err)
		return 3
	}
	hs := &http.Server{Handler: srv, ReadHeaderTimeout: 10 * time.Second, ErrorLog: nil}
	if profile >= 2 {
		hs.TLSConfig, _ = serverTLS(tlsDir, profile)
	}
	served := make(chan error, 1)
	go func() {
		if profile >= 2 {
			served <- hs.ServeTLS(listener, "", "")
		} else {
			served <- hs.Serve(listener)
		}
	}()

	control, err := serveControl(controlPath, func(station *string, action string, payload map[string]any,
		timeoutMs int64) resultDoc {
		id := ""
		if station != nil {
			id = *station
		}
		st := srv.Station(id)
		if st == nil {
			return resultDoc{version: version, operation: action, request: payload, errCode: "not-connected",
				errMsg: fmt.Sprintf("no open connection for station %q", id)}
		}
		return e.execute(id, action, payload, timeoutMs, st, st.Done())
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
	code := 0
	select {
	case <-signals:
	case err := <-served:
		fmt.Fprintf(os.Stderr, "server: %v\n", err)
		code = 1
	}

	_ = os.Remove(ready)
	control.Close()
	for _, st := range srv.Stations() {
		st.Close("server shutting down")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	_ = hs.Shutdown(ctx)
	cancel()
	// Let the sessions report their close.
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		mu.Lock()
		n := len(open)
		mu.Unlock()
		if n == 0 {
			break
		}
	}
	event("stopped", "")
	return code
}

// closeOf is the close code and reason of a lost connection, where the peer
// sent a close frame (websocket.CloseError), and null otherwise.
func closeOf(err error) (code, reason any) {
	var ce websocket.CloseError
	if errors.As(err, &ce) {
		return int(ce.Code), ce.Reason
	}
	return nil, nil
}
