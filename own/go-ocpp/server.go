// SPDX-License-Identifier: MIT
package main

// `server`: a CSMS, go-ocpp's server.Server for one OCPP version.

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/otfabric/go-ocpp"
	"github.com/otfabric/go-ocpp/server"
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

func runServer(version string, fx *fixture, bind string, port, profile int, tlsDir, ready, controlPath string) int {
	vi := versions[version]
	subprotocol := vi.version.Subprotocol()
	tr := newTracker(true)
	e := &endpoint{version: version, fixture: fx, role: "csms", tracker: tr}

	var mu sync.Mutex
	remotes := map[string]string{} // station -> remote address of its last accepted handshake
	announced := map[string]bool{} // sessions whose connection-opened was written
	open := map[string]bool{}      // sessions that are open
	announce := func(info ocpp.ConnInfo, remote string) {
		mu.Lock()
		defer mu.Unlock()
		if info.SessionID == "" || announced[info.SessionID] {
			return
		}
		announced[info.SessionID] = true
		open[info.SessionID] = true
		if remote == "" {
			remote = remotes[info.StationID]
		}
		// go-ocpp accepts permessage-deflate whenever a station offers it, but
		// does not say whether it was negotiated.
		event("connection-opened", info.StationID, "subprotocol", subprotocol, "remote", remote, "compression", nil)
	}
	// The first frame of a session can arrive before go-ocpp calls the
	// session handler: the events of a session start with connection-opened.
	tr.opened = func(info ocpp.ConnInfo) { announce(info, "") }

	// The fixture decides who may connect; go-ocpp refuses with 404 for
	// ErrUnknownStation and 401 for any other error.
	auth := server.AuthenticatorFunc(func(_ context.Context, r server.ConnectRequest) error {
		id := r.StationID
		if _, known := fx.stations[id]; !known {
			event("connection-rejected", id, "reason", "unknown-station", "remote", r.RemoteAddr)
			return server.ErrUnknownStation
		}
		switch profile {
		case 1, 2:
			if !r.HasBasicAuth || r.Username != id || string(r.Password) != fx.password(id) {
				event("connection-rejected", id, "reason", "authentication", "remote", r.RemoteAddr)
				return server.ErrUnauthorized
			}
		case 3:
			// The handshake verified the certificate against ca.crt; its
			// subject must name this station.
			if cn, err := peerCommonName(r.TLS); err != nil || cn != id {
				event("connection-rejected", id, "reason", "authentication", "remote", r.RemoteAddr)
				return server.ErrUnauthorized
			}
		}
		offered := r.Request.Header.Values("Sec-WebSocket-Protocol")
		if !offers(offered, subprotocol) {
			// go-ocpp completes the handshake without a subprotocol and
			// closes the connection at once.
			list := []string{}
			for _, v := range offered {
				for _, token := range strings.Split(v, ",") {
					if t := strings.TrimSpace(token); t != "" {
						list = append(list, t)
					}
				}
			}
			event("connection-rejected", id, "reason", "subprotocol", "remote", r.RemoteAddr, "offered", list)
			return nil
		}
		mu.Lock()
		remotes[id] = r.RemoteAddr
		mu.Unlock()
		return nil
	})

	opts := []server.Option{
		server.WithHandler(vi.version, e.dispatcher()),
		server.WithAuthenticator(auth),
		server.WithFrameObserver(tr.observe),
		server.WithCallInterceptor(tr.intercept),
		server.WithMetrics(&metrics{t: tr}),
		server.WithResponseTimeout(time.Hour), // every call has the deadline of its --timeout-ms
		server.WithLogger(logger()),
		server.WithSessionHandler(
			func(s *server.Session) {
				announce(ocpp.ConnInfo{StationID: s.ID(), Version: s.Version(), Role: ocpp.CSMS, SessionID: s.SessionID()},
					s.RemoteAddr())
			},
			func(s *server.Session, err error) {
				mu.Lock()
				wasOpen := open[s.SessionID()]
				delete(open, s.SessionID())
				mu.Unlock()
				if wasOpen {
					code, reason := closeOf(err)
					event("connection-closed", s.ID(), "code", code, "reason", reason)
				}
			}),
	}
	if profile >= 2 {
		config, _ := serverTLS(tlsDir, profile)
		opts = append(opts, server.WithTLS(config))
	}
	srv, err := server.New(opts...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "server: %v\n", err)
		return 1
	}

	listener, err := net.Listen("tcp", net.JoinHostPort(bind, fmt.Sprint(port)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "server: cannot listen on %s port %d: %v\n", bind, port, err)
		return 3
	}
	served := make(chan error, 1)
	go func() { served <- srv.Serve(listener) }()

	control, err := serveControl(controlPath, func(station *string, action string, payload map[string]any,
		timeoutMs int64) resultDoc {
		id := ""
		if station != nil {
			id = *station
		}
		session, ok := srv.Station(id)
		if !ok {
			return resultDoc{version: version, operation: action, request: payload, errCode: "not-connected",
				errMsg: fmt.Sprintf("no open connection for station %q", id)}
		}
		return e.execute(id, action, payload, timeoutMs, session, session.Done())
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	_ = srv.Shutdown(ctx)
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
