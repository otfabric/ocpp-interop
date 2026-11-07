// SPDX-License-Identifier: MIT
package main

// `server`: a CSMS, seanlee0923/ocpp's csms.Server for one OCPP version.

import (
	"bufio"
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

	"github.com/gorilla/websocket"
	"github.com/seanlee0923/ocpp/csms"
	"github.com/seanlee0923/ocpp/protocol"
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

func offeredList(values []string) []string {
	out := []string{}
	for _, v := range values {
		for _, token := range strings.Split(v, ",") {
			if t := strings.TrimSpace(token); t != "" {
				out = append(out, t)
			}
		}
	}
	return out
}

// statusWriter records the status of a refused handshake; a WebSocket
// upgrade hijacks the connection.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("the response cannot be hijacked")
	}
	return h.Hijack()
}

func runServer(version string, fx *fixture, bind string, port, profile int, tlsDir, ready, controlPath string) int {
	vi := versions[version]
	e := newEndpoint(version, fx, "csms", true)

	var mu sync.Mutex
	remotes := map[string]string{} // station -> the remote address of its last handshake
	open := map[*csms.Session]bool{}

	// The handlers are the router's, shared by every session: a middleware
	// gives them the station of the session.
	router := csms.NewRouter()
	for action, ops := range actionTable[version] {
		if !e.receives()[action] || !knows(version, action) {
			continue
		}
		action, ops := action, ops
		var err error
		if ops.send {
			if vi.sendActions[action] {
				err = ops.csmsHandleSnd(router, func(ctx context.Context, payload json.RawMessage) {
					e.taker(stationOf(ctx), action)(ctx, payload)
				})
			}
		} else {
			err = ops.csmsHandle(router, func(ctx context.Context, payload json.RawMessage) (json.RawMessage, error) {
				return e.answerer(stationOf(ctx), action)(ctx, payload)
			})
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "server: cannot register %s: %v\n", action, err)
			return 1
		}
	}
	router.Use(func(_ protocol.Version, _ string, next csms.Handler) csms.Handler {
		return func(ctx context.Context, s *csms.Session, payload json.RawMessage) (any, error) {
			return next(context.WithValue(ctx, stationKey{}, s.Identity()), s, payload)
		}
	})

	// The fixture decides who may connect. seanlee0923/ocpp checks the Basic
	// user name against the identity and the TLS policy itself, then asks
	// the authenticator.
	authenticate := csms.AuthenticatorFunc(func(_ context.Context, r csms.AuthenticationRequest) (csms.Principal, error) {
		if _, known := fx.stations[r.Identity]; !known {
			return csms.Principal{}, errors.New("unknown station")
		}
		switch profile {
		case 1, 2:
			if r.Basic == nil || !passwordOK(fx.password(r.Identity), string(r.Basic.Password)) {
				return csms.Principal{}, errors.New("wrong credentials")
			}
		case 3:
			// The handshake verified the certificate against ca.crt; its
			// subject must name this station.
			if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 ||
				!certNames(r.TLS.PeerCertificates[0], r.Identity) {
				return csms.Principal{}, errors.New("the client certificate is not this station's")
			}
		}
		return csms.Principal{ID: r.Identity}, nil
	})

	srv, err := csms.New(csms.Config{
		Router:   router,
		Versions: []protocol.Version{vi.wire},
		// Every CALL has the deadline of its --timeout-ms.
		CallTimeout: time.Hour,
		// The harness, not the server, ends idle connections.
		PingInterval: 0,
		Security: csms.SecurityConfig{
			Profile:       csms.SecurityProfile(profile),
			Authenticator: authenticate,
		},
		OnConnect: func(s *csms.Session) {
			mu.Lock()
			remote := remotes[s.Identity()]
			open[s] = true
			mu.Unlock()
			// The library's upgrader does not negotiate permessage-deflate.
			event("connection-opened", s.Identity(), "subprotocol", string(s.Version()), "remote", nullable(remote),
				"compression", false)
		},
		OnDisconnect: func(s *csms.Session, err error) {
			mu.Lock()
			wasOpen := open[s]
			delete(open, s)
			mu.Unlock()
			if wasOpen {
				code, reason := closeOf(err)
				event("connection-closed", s.Identity(), "code", code, "reason", reason)
			}
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "server: %v\n", err)
		return 1
	}

	// The events of refused handshakes: seanlee0923/ocpp refuses with an
	// HTTP status and has no hook for every case.
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The contract's path is /ocpp/<identity>; csms.Server takes the
		// whole path as the identity, so it gets the last segment only.
		path := strings.TrimRight(r.URL.Path, "/")
		id := path[strings.LastIndex(path, "/")+1:]
		mu.Lock()
		remotes[id] = r.RemoteAddr
		mu.Unlock()
		inner := r.Clone(r.Context())
		inner.URL.Path, inner.URL.RawPath = "/"+id, ""
		sw := &statusWriter{ResponseWriter: w}
		srv.ServeHTTP(sw, inner)
		if sw.status < 400 {
			return
		}
		offered := r.Header.Values("Sec-WebSocket-Protocol")
		reason := "authentication"
		switch {
		case !offers(offered, string(vi.wire)):
			event("connection-rejected", id, "reason", "subprotocol", "remote", r.RemoteAddr,
				"offered", offeredList(offered))
			return
		case fx.stations[id] == nil:
			reason = "unknown-station"
		}
		event("connection-rejected", id, "reason", reason, "remote", r.RemoteAddr)
	})

	listener, err := net.Listen("tcp", net.JoinHostPort(bind, fmt.Sprint(port)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "server: cannot listen on %s port %d: %v\n", bind, port, err)
		return 3
	}
	hs := &http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second}
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
		s, ok := srv.Session(id)
		if !ok {
			return resultDoc{version: version, operation: action, request: payload, errCode: "not-connected",
				errMsg: fmt.Sprintf("no open connection for station %q", id)}
		}
		return e.execute(id, action, payload, timeoutMs, &target{session: s}, s.Done())
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

type stationKey struct{}

// stationOf is the identity of the session a handler runs for.
func stationOf(ctx context.Context) string {
	s, _ := ctx.Value(stationKey{}).(string)
	return s
}

// closeOf is the close code and reason of a lost connection, where the peer
// sent a close frame (websocket.CloseError), and null otherwise.
func closeOf(err error) (code, reason any) {
	var ce *websocket.CloseError
	if errors.As(err, &ce) {
		return ce.Code, ce.Text
	}
	return nil, nil
}
