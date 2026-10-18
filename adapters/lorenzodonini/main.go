// SPDX-License-Identifier: MIT

// Command ocpp-interop is the ocpp-interop adapter for lorenzodonini/ocpp-go:
// the container contract (docs/CONTAINER_CONTRACT.md) on top of ocpp-go's own
// OCPP 1.6 and 2.0.1 stacks (packages ocpp1.6, ocpp2.0.1, ocppj and ws).
//
// The adapter is glue: argument parsing, fixture loading, the control socket
// and JSON output. Framing, (de)serialisation, validation, request routing,
// pending requests, timeouts and the WebSocket transport are ocpp-go's.
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
	adapterName     = "lorenzodonini"
	schemaVersion   = "1.0"
	contractVersion = "0.3"
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
                      [--tls-dir DIR] [--connect-timeout-ms N] [--timeout-ms N] [--hold-ms N]
                      [--ready-file PATH] [--control-socket PATH]
  ocpp-interop client <Action> --version V --url URL [--station ID] [--fixture PATH] [--payload JSON]
                      [--security-profile N] [--tls-dir DIR] [--connect-timeout-ms N] [--timeout-ms N]
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
	if p >= 2 {
		return 0, unsupportedf("security profile %d (TLS) is not supported by this adapter yet", p)
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
		writeText(fmt.Sprintf("ocpp-interop-%s %s (ocpp-go %s, contract %s)", adapterName, adapterVersion,
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
			set("name", "lorenzodonini ocpp-go").
			set("repository", "https://github.com/lorenzodonini/ocpp-go").
			set("version", upstreamVersion).
			set("revision", upstreamRevision).
			set("license", "MIT").
			set("language", "Go").
			set("independentEngine", true)).
		set("versions", byVersion)
}

func versionCapabilities(version string) *object {
	d := declared[version]
	stationHandles := minus(d.csmsSends, d.unansweredByStation)
	csmsHandles := minus(d.stationSends, d.unansweredByCSMS)
	notes := []string{
		"The adapter uses ocpp-go's own high-level API (ocpp16.CentralSystem and ChargePoint, ocpp2.CSMS and " +
			"ChargingStation) on its ocppj endpoints and ws transport, with the profiles ocpp-go registers by default.",
		"An action of OCPP " + version + " without a fixture answer is answered NotImplemented: the handler returns " +
			"that error and ocpp-go sends it. An action ocpp-go does not know is answered NotSupported by ocpp-go.",
		"The server refuses an identity the fixture does not list, and wrong Basic credentials, with HTTP 401: " +
			"ocpp-go's connection check (CheckClientHandler) can only refuse with 401, not 404.",
		"The server listens on all addresses (ws.Server.Start takes a port only): --bind-address accepts 0.0.0.0 only.",
		"The station does not see the negotiated subprotocol (ws.Client does not expose it), and ocpp-go does not " +
			"check it: connection-opened has subprotocol null. The server accepts only " + versions[version].subprotocol +
			" and closes any other handshake (1002) before the adapter sees it.",
		"The server does not see the close code of a station's connection: connection-closed has code and reason null.",
		"Message ids are ocpp-go's (a random uint32, its default generator); the adapter reads them through " +
			"ocppj.SetMessageIdGenerator.",
	}
	return newObject().
		set("client", newObject().set("supported", true).set("sends", d.stationSends).set("handles", stationHandles)).
		set("server", newObject().set("supported", true).set("sends", d.csmsSends).set("handles", csmsHandles)).
		set("features", newObject().
			set("securityProfiles", []int{0, 1}).
			set("compression", false).
			set("send", false).
			set("callResultError", false).
			set("fixtureCallErrors", true).
			set("messageIds", true)).
		set("notes", notes)
}

func minus(list []string, drop string) []string {
	out := []string{}
	for _, a := range list {
		if a != drop {
			out = append(out, a)
		}
	}
	sort.Strings(out)
	return out
}

// declaredActions is what the self-tests exercise in one version.
type declaredActions struct {
	stationSends, csmsSends               []string
	unansweredByCSMS, unansweredByStation string
}

var declared = map[string]declaredActions{
	"1.6": {
		stationSends: []string{
			"Authorize", "BootNotification", "DataTransfer", "DiagnosticsStatusNotification",
			"FirmwareStatusNotification", "Heartbeat", "MeterValues", "SecurityEventNotification",
			"StartTransaction", "StatusNotification", "StopTransaction",
		},
		csmsSends: []string{
			"CancelReservation", "ChangeAvailability", "ChangeConfiguration", "ClearCache",
			"ClearChargingProfile", "DataTransfer", "ExtendedTriggerMessage", "GetCompositeSchedule",
			"GetConfiguration", "GetDiagnostics", "GetLocalListVersion", "RemoteStartTransaction",
			"RemoteStopTransaction", "ReserveNow", "Reset", "SendLocalList", "SetChargingProfile",
			"TriggerMessage", "UnlockConnector", "UpdateFirmware",
		},
		unansweredByCSMS:    "SecurityEventNotification",
		unansweredByStation: "ExtendedTriggerMessage",
	},
	"2.0.1": {
		stationSends: []string{
			"Authorize", "BootNotification", "DataTransfer", "FirmwareStatusNotification", "Heartbeat",
			"LogStatusNotification", "MeterValues", "NotifyEvent", "NotifyReport", "ReservationStatusUpdate",
			"SecurityEventNotification", "StatusNotification", "TransactionEvent",
		},
		csmsSends: []string{
			"CancelReservation", "ChangeAvailability", "ClearCache", "ClearChargingProfile", "DataTransfer",
			"GetBaseReport", "GetLocalListVersion", "GetVariables", "RequestStartTransaction",
			"RequestStopTransaction", "Reset", "SetChargingProfile", "SetVariables", "TriggerMessage",
			"UnlockConnector",
		},
		unansweredByCSMS:    "ReservationStatusUpdate",
		unansweredByStation: "CancelReservation",
	},
}
