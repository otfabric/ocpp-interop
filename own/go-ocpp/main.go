// SPDX-License-Identifier: MIT

// Command ocpp-interop is OT Fabric's own OCPP stack, go-ocpp, behind the
// ocpp-interop container contract (docs/CONTAINER_CONTRACT.md): the image
// ocpp-interop-otfabric, for others to test against. It is not an
// independent implementation (README.md).
//
// The adapter is glue: argument parsing, fixture loading, the control socket
// and JSON output. Framing, decoding and validation, request routing, pending
// requests, timeouts and the WebSocket transport are go-ocpp's (packages
// server, client, ocpp16, ocpp201, ocpp21).
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Set at build time (-ldflags -X).
var (
	adapterVersion   = "dev"
	upstreamVersion  = "unknown"
	upstreamRevision = "0000000000000000000000000000000000000000"
)

const (
	adapterName     = "otfabric"
	schemaVersion   = "1.0"
	contractVersion = "0.5"
	readyFile       = "/run/ocpp-interop/ready"
	controlSocket   = "/run/ocpp-interop/control.sock"
)

var (
	actionName  = regexp.MustCompile(`^[A-Z][A-Za-z0-9]+$`)
	fixtureName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
)

const usage = `usage:
  ocpp-interop server --version V [--fixture PATH] [--bind-address ADDR] [--port N]
                      [--security-profile N] [--tls-dir DIR] [--ready-file PATH] [--control-socket PATH]
  ocpp-interop client run --version V --url URL [--station ID] [--fixture PATH] [--security-profile N]
                      [--tls-dir DIR] [--compression] [--connect-timeout-ms N] [--timeout-ms N] [--hold-ms N]
                      [--ready-file PATH] [--control-socket PATH]
  ocpp-interop client <Action> --version V --url URL [--station ID] [--fixture PATH] [--payload JSON]
                      [--security-profile N] [--tls-dir DIR] [--compression] [--connect-timeout-ms N]
                      [--timeout-ms N]
  ocpp-interop call [--station ID] <Action> [--payload JSON] [--timeout-ms N] [--control-socket PATH]
  ocpp-interop print-capabilities
  ocpp-interop print-fixture --version V [NAME]
  ocpp-interop version`

// usageError: exit 2, nothing was sent.
type usageError struct{ msg string }

func (e usageError) Error() string { return e.msg }

// unsupportedError: exit 5, a version, role, profile, action or fixture
// answer this adapter does not support.
type unsupportedError struct{ msg string }

func (e unsupportedError) Error() string { return e.msg }

// fixtureError: exit 4, the fixture is missing or invalid.
type fixtureError struct{ msg string }

func (e fixtureError) Error() string { return e.msg }

func usagef(format string, args ...any) error { return usageError{fmt.Sprintf(format, args...)} }

func unsupportedf(format string, args ...any) error {
	return unsupportedError{fmt.Sprintf(format, args...)}
}

// flagSet holds `--name value` pairs and the positional arguments.
type flagSet struct {
	given      map[string]string
	positional []string
}

// switches are the flags without a value (docs/CONTAINER_CONTRACT.md, Commands).
var switches = map[string]bool{"compression": true}

func parseFlags(args []string, known ...string) (*flagSet, error) {
	allowed := map[string]bool{}
	for _, k := range known {
		allowed[k] = true
	}
	f := &flagSet{given: map[string]string{}}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "--") {
			f.positional = append(f.positional, arg)
			continue
		}
		name := arg[2:]
		if !allowed[name] {
			return nil, usagef("unknown flag %q", arg)
		}
		if switches[name] {
			if _, dup := f.given[name]; dup {
				return nil, usagef("%s is given more than once", arg)
			}
			f.given[name] = "true"
			continue
		}
		if i+1 >= len(args) {
			return nil, usagef("%s needs a value", arg)
		}
		if _, dup := f.given[name]; dup {
			return nil, usagef("%s is given more than once", arg)
		}
		f.given[name] = args[i+1]
		i++
	}
	return f, nil
}

func (f *flagSet) get(name, fallback string) string {
	if v, ok := f.given[name]; ok {
		return v
	}
	return fallback
}

func (f *flagSet) has(name string) bool { _, ok := f.given[name]; return ok }

func (f *flagSet) number(name string, fallback, low, high int64) (int64, error) {
	text, ok := f.given[name]
	if !ok {
		return fallback, nil
	}
	if !regexp.MustCompile(`^[0-9]{1,12}$`).MatchString(text) {
		return 0, usagef("--%s: %q is not a number", name, text)
	}
	v, _ := strconv.ParseInt(text, 10, 64)
	if v < low || v > high {
		return 0, usagef("--%s: %d is not within %d..%d", name, v, low, high)
	}
	return v, nil
}

func versionOf(f *flagSet) (string, error) {
	if !f.has("version") {
		return "", usagef("--version is required")
	}
	v := f.get("version", "")
	if _, ok := versionDirs[v]; !ok {
		return "", usagef("--version: %q is not 1.6, 2.0.1 or 2.1", v)
	}
	if _, ok := versions[v]; !ok {
		return "", unsupportedf("OCPP %s is not supported by this adapter", v)
	}
	return v, nil
}

func securityProfileOf(f *flagSet) (int, error) {
	p, err := f.number("security-profile", 0, 0, 3)
	if err != nil {
		return 0, err
	}
	if f.has("tls-dir") && p < 2 {
		return 0, usagef("--tls-dir is for security profiles 2 and 3")
	}
	if p >= 2 && !f.has("tls-dir") {
		return 0, usagef("security profile %d needs --tls-dir", p)
	}
	return int(p), nil
}

func payloadOf(f *flagSet) (map[string]any, error) {
	var v any
	dec := json.NewDecoder(strings.NewReader(f.get("payload", "{}")))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return nil, usagef("--payload is not JSON: %v", err)
	}
	if dec.More() {
		return nil, usagef("--payload is not one JSON value")
	}
	obj, ok := v.(map[string]any)
	if !ok {
		return nil, usagef("--payload must be a JSON object")
	}
	return obj, nil
}

func actionOf(text string) (string, error) {
	if !actionName.MatchString(text) {
		return "", usagef("%q is not an OCPP action name", text)
	}
	return text, nil
}

func fixtureOf(f *flagSet, version string) (*fixture, error) {
	return loadFixture(f.get("fixture", defaultFixturePath(version)), version)
}

func main() {
	setupLogging()
	code := run(os.Args[1:])
	os.Exit(code)
}

func run(args []string) int {
	err := dispatch(args)
	var code exitCode
	var usage usageError
	var unsupported unsupportedError
	var invalid fixtureError
	switch {
	case err == nil:
		return 0
	case errors.As(err, &code):
		return int(code)
	case errors.As(err, &usage):
		fmt.Fprintf(os.Stderr, "usage error: %s\n%s\n", usage.msg, usageText())
		return 2
	case errors.As(err, &unsupported):
		fmt.Fprintf(os.Stderr, "unsupported: %s\n", unsupported.msg)
		return 5
	case errors.As(err, &invalid):
		fmt.Fprintf(os.Stderr, "invalid fixture: %s\n", invalid.msg)
		return 4
	default:
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
}

func usageText() string { return usage }

// exitCode ends a command with that exit code, after it wrote its output.
type exitCode int

func (e exitCode) Error() string { return "exit " + strconv.Itoa(int(e)) }

func dispatch(args []string) error {
	if len(args) == 0 {
		return usagef("no command")
	}
	rest := args[1:]
	switch args[0] {
	case "server":
		return commandServer(rest)
	case "client":
		return commandClient(rest)
	case "call":
		return commandCall(rest)
	case "print-capabilities":
		if len(rest) > 0 {
			return usagef("print-capabilities takes no arguments")
		}
		writeLine(capabilities())
		return nil
	case "print-fixture":
		return commandPrintFixture(rest)
	case "version":
		if len(rest) > 0 {
			return usagef("version takes no arguments")
		}
		writeText(fmt.Sprintf("ocpp-interop-%s %s (go-ocpp %s, contract %s)", adapterName, adapterVersion,
			upstreamVersion, contractVersion))
		return nil
	default:
		return usagef("unknown command %q", args[0])
	}
}

func commandCall(args []string) error {
	f, err := parseFlags(args, "station", "payload", "timeout-ms", "control-socket")
	if err != nil {
		return err
	}
	if len(f.positional) != 1 {
		return usagef("exactly one action is required")
	}
	action, err := actionOf(f.positional[0])
	if err != nil {
		return err
	}
	payload, err := payloadOf(f)
	if err != nil {
		return err
	}
	timeoutMs, err := f.number("timeout-ms", 10000, 1, 3600000)
	if err != nil {
		return err
	}
	var station *string
	if s, ok := f.given["station"]; ok {
		station = &s
	}
	return exitCode(callControl(f.get("control-socket", controlSocket), station, action, payload, timeoutMs))
}

func commandPrintFixture(args []string) error {
	f, err := parseFlags(args, "version")
	if err != nil {
		return err
	}
	dir, ok := versionDirs[f.get("version", "")]
	if !ok {
		return usagef("--version 1.6, 2.0.1 or 2.1 is required")
	}
	if len(f.positional) > 1 {
		return usagef("at most one fixture name")
	}
	name := "baseline"
	if len(f.positional) == 1 {
		name = f.positional[0]
	}
	if !fixtureName.MatchString(name) {
		return usagef("%q is not a fixture name", name)
	}
	data, err := os.ReadFile(fixturesDir + "/" + dir + "/" + name + "/fixture.json")
	if err != nil {
		fmt.Fprintf(os.Stderr, "print-fixture: no fixture %q for OCPP %s in this image\n", name, f.get("version", ""))
		return exitCode(1)
	}
	writeRaw(data)
	return nil
}

// capabilities is the print-capabilities document. Only what the self-tests
// exercise is declared (REQUIREMENTS.md R4.2).
func capabilities() *object {
	names := make([]string, 0, len(versions))
	for v := range versions {
		names = append(names, v)
	}
	sort.Strings(names)
	byVersion := newObject()
	for _, v := range names {
		byVersion.set(v, versionCapabilities(v))
	}
	return newObject().
		set("schemaVersion", schemaVersion).
		set("adapter", adapterName).
		set("adapterVersion", adapterVersion).
		set("contractVersion", contractVersion).
		set("upstream", newObject().
			set("name", "OT Fabric go-ocpp").
			set("repository", "https://github.com/otfabric/go-ocpp").
			set("version", upstreamVersion).
			set("revision", upstreamRevision).
			set("license", "MIT").
			set("language", "Go").
			// OT Fabric's own stack: results with it are not evidence of an
			// independent implementation (own/go-ocpp/README.md).
			set("independentEngine", false)).
		set("versions", byVersion)
}

func versionCapabilities(version string) *object {
	stationSends, csmsSends := declaredSends(version)
	// An end handles what the other end sends.
	stationHandles, csmsHandles := csmsSends, stationSends
	notes := []string{
		"OT Fabric's own stack (go-ocpp), not an independent implementation: upstream.independentEngine is false.",
		"The adapter uses go-ocpp's public API: server.Server and client.Client, with a raw-payload dispatcher " +
			"(ocpp.DispatcherFunc) that decodes and validates each request with the version package's ValidateRequest " +
			"and each fixture answer with its ValidateResponse, as go-ocpp's typed dispatchers do. Requests are sent " +
			"with Call as raw JSON after ValidateRequest; responses are checked with ValidateResponse.",
		"An action of OCPP " + version + " the receiving role gets without a fixture answer is answered NotImplemented; " +
			"an action the role does not receive NotSupported; an action go-ocpp does not know NotImplemented, as " +
			"go-ocpp's own dispatchers answer.",
		"The server refuses an identity the fixture does not list with HTTP 404 (server.ErrUnknownStation), and wrong " +
			"credentials or a client certificate of another identity with HTTP 401.",
		"Compression: go-ocpp's CSMS accepts permessage-deflate whenever it is offered, and its station offers it with " +
			"--compression (client.WithCompression); go-ocpp does not expose whether it was negotiated, so " +
			"connection-opened has compression null (false for a station without --compression).",
		"Message ids are go-ocpp's (UUIDs), read through a call interceptor (ocpp.CallInfo.MessageID).",
		"go-ocpp implements SEND and CALLRESULTERROR (OCPP 2.1), but the contract has no operation to send a SEND or " +
			"provoke a CALLRESULTERROR yet: the features send and callResultError are false, and a SEND action is " +
			"refused (unsupported).",
	}
	return newObject().
		set("client", newObject().set("supported", true).set("sends", stationSends).set("handles", stationHandles)).
		set("server", newObject().set("supported", true).set("sends", csmsSends).set("handles", csmsHandles)).
		set("features", newObject().
			set("securityProfiles", []int{0, 1, 2, 3}).
			set("compression", true).
			set("send", false).
			set("callResultError", false).
			set("fixtureCallErrors", true).
			set("messageIds", true)).
		set("notes", notes)
}

// declaredSends is what the self-tests exercise in one version: every action
// of the version in the direction go-ocpp gives it, except the SEND actions
// (OCPP 2.1), less notSent. The baseline fixtures have a sample and an answer
// for each.
func declaredSends(version string) (station, csms []string) {
	vi := versions[version]
	list := func(actions map[string]bool) []string {
		out := []string{}
		for a := range actions {
			if !vi.sendActions[a] && !notSent[version][a] {
				out = append(out, a)
			}
		}
		sort.Strings(out)
		return out
	}
	return list(vi.fromStation), list(vi.fromCSMS)
}

// notSent: actions of a version that go-ocpp has but that the self-tests do
// not send, with the reason (README.md). None.
var notSent = map[string]map[string]bool{}
