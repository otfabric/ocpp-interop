// SPDX-License-Identifier: MIT
//
// ocpp-interop adapter for EVerest libocpp: the container contract
// (docs/CONTAINER_CONTRACT.md) on top of libocpp's OCPP 1.6, 2.0.1 and 2.1
// protocol layer. libocpp implements the charging station only, so this
// adapter is a station (`client`); `server` exits 5.
//
// The adapter is glue: argument parsing, fixture loading, the control socket
// and JSON output. The WebSocket transport (libwebsockets), the subprotocol,
// Basic authentication and TLS, framing, message ids, pending requests and
// their timeouts, and the reading and writing of every OCPP message are
// libocpp's (see station.cpp).
#include "common.hpp"
#include "station.hpp"

#include <algorithm>
#include <cstdio>
#include <fstream>
#include <regex>
#include <sstream>

namespace interop {

namespace {

const char* usage_text = R"(usage:
  ocpp-interop server --version V [--fixture PATH] [--bind-address ADDR] [--port N]
                      [--security-profile N] [--tls-dir DIR] [--ready-file PATH] [--control-socket PATH]
                      (libocpp implements the charging station only: server exits 5)
  ocpp-interop client run --version V --url URL [--station ID] [--fixture PATH] [--security-profile N]
                      [--tls-dir DIR] [--compression] [--connect-timeout-ms N] [--timeout-ms N] [--hold-ms N]
                      [--ready-file PATH] [--control-socket PATH]
  ocpp-interop client <Action> --version V --url URL [--station ID] [--fixture PATH] [--payload JSON]
                      [--security-profile N] [--tls-dir DIR] [--compression] [--connect-timeout-ms N]
                      [--timeout-ms N]
  ocpp-interop call [--station ID] <Action> [--payload JSON] [--timeout-ms N] [--control-socket PATH]
  ocpp-interop print-capabilities
  ocpp-interop print-fixture --version V [NAME]
  ocpp-interop version)";

// What the self-tests exercise, per version (REQUIREMENTS.md R4.2): the
// actions the station sends, and those it answers from the fixture.
struct Declared {
    std::vector<std::string> sends;
    std::vector<std::string> handles;
};

const std::map<std::string, Declared>& declared() {
    static const std::map<std::string, Declared> table = {
        {"1.6",
         {{"Authorize", "BootNotification", "DataTransfer", "DiagnosticsStatusNotification",
           "FirmwareStatusNotification", "Heartbeat", "MeterValues", "SecurityEventNotification", "StartTransaction",
           "StatusNotification", "StopTransaction"},
          {"CancelReservation", "ChangeAvailability", "ChangeConfiguration", "ClearCache", "ClearChargingProfile",
           "DataTransfer", "GetCompositeSchedule", "GetConfiguration", "GetDiagnostics", "GetLocalListVersion",
           "RemoteStartTransaction", "RemoteStopTransaction", "ReserveNow", "Reset", "SendLocalList",
           "SetChargingProfile", "TriggerMessage", "UnlockConnector", "UpdateFirmware"}}},
        {"2.0.1",
         {{"Authorize", "BootNotification", "DataTransfer", "FirmwareStatusNotification", "Heartbeat",
           "LogStatusNotification", "MeterValues", "NotifyEvent", "NotifyReport", "ReservationStatusUpdate",
           "SecurityEventNotification", "StatusNotification", "TransactionEvent"},
          {"ChangeAvailability", "ClearCache", "ClearChargingProfile", "DataTransfer", "GetBaseReport",
           "GetLocalListVersion", "GetVariables", "RequestStartTransaction", "RequestStopTransaction", "Reset",
           "SetChargingProfile", "SetVariables", "TriggerMessage", "UnlockConnector"}}},
        {"2.1",
         {{"Authorize", "BootNotification", "ClosePeriodicEventStream", "DataTransfer",
           "FirmwareStatusNotification", "Heartbeat", "LogStatusNotification", "MeterValues", "NotifyDERAlarm",
           "NotifyDERStartStop", "NotifyEvent", "NotifyReport", "OpenPeriodicEventStream", "ReportDERControl",
           "ReservationStatusUpdate", "SecurityEventNotification", "StatusNotification", "TransactionEvent"},
          {"AdjustPeriodicEventStream", "ChangeAvailability", "ClearCache", "ClearChargingProfile", "ClearDERControl",
           "ClearTariffs", "DataTransfer", "GetBaseReport", "GetDERControl", "GetLocalListVersion",
           "GetPeriodicEventStream", "GetTariffs", "GetVariables", "RequestBatterySwap", "RequestStartTransaction",
           "RequestStopTransaction", "Reset", "SetChargingProfile", "SetDERControl", "SetDefaultTariff",
           "SetVariables", "TriggerMessage", "UnlockConnector"}}},
    };
    return table;
}

std::vector<std::string> version_notes(const std::string& version) {
    std::vector<std::string> notes = {
        "Station only: libocpp implements the charging station side of OCPP, not a CSMS. server exits 5.",
        "The adapter uses libocpp's protocol layer, as libocpp's own ChargePoint does: ocpp::Websocket "
        "(libwebsockets) for the connection, ocpp::MessageQueue for sending CALLs and matching their CALLRESULT "
        "or CALLERROR, and libocpp's message structs (from_json, to_json, Call<T>, CallResult<T>) for every "
        "payload. It does not use ChargePoint, which sends BootNotification, StatusNotification and Heartbeat by "
        "itself and answers from its device model: here the fixture answers, and only the CALLs `call` asks for "
        "are sent.",
        "Answers: a CALL libocpp cannot read into its request struct is answered FormationViolation (as "
        "ChargePoint does); an action without a fixture answer, or that libocpp has no struct for in this version, "
        "NotImplemented; the fixture's answer is read into libocpp's response struct and written by it, so a "
        "response field libocpp does not know is dropped.",
        "Requests are written by libocpp's structs: an optional field the struct does not know is dropped, and "
        "date-times are written with milliseconds.",
        "libocpp's MessageQueue sends one CALL at a time; the adapter sets its retries to one attempt, so a CALL "
        "answered with a CALLERROR or not at all is never resent. libocpp keeps a BootNotification answered with a "
        "CALLERROR for a resend after a retry interval, which the adapter sets to a year; the result reports the "
        "CALLERROR.",
        "libocpp does not pass close codes to the application: connection-closed has code and reason null.",
        "Security profiles 2 and 3 use libocpp's TLS (OpenSSL through libwebsockets) with the files of --tls-dir: "
        "ca.crt verifies the CSMS and its host name, client.crt and client.key are the station's certificate in "
        "profile 3. They are implemented but not declared until the self-tests exercise them.",
        "No compression: libocpp creates its libwebsockets client context without extensions, so it cannot offer "
        "permessage-deflate; --compression exits 5.",
        "Message ids are libocpp's (a UUID per CALL).",
    };
    if (version == "2.1") {
        notes.push_back(
            "No SEND and no CALLRESULTERROR: libocpp knows the OCPP-J message types 2 to 4 only "
            "(ocpp::MessageTypeId). NotifyPeriodicEventStream (a SEND) is refused with exit 5; a frame of type 5 or 6 "
            "is reported as protocol-error and not answered.");
        notes.push_back(
            "libocpp's ocpp::v2::MessageType has no ReportDERControl, GetCertificateChainStatus or "
            "NotifyWebPaymentStarted although their structs exist: its MessageQueue cannot send them (the result is "
            "`failed`) and cannot read a received CALL of them (answered NotImplemented). ReportDERControl is declared "
            "and its failure is a known upstream defect (tests/known-bugs.tsv).");
        notes.push_back(
            "OCPP 2.0.1 and 2.1 share libocpp's ocpp::v2 types, which libocpp generates from the OCPP 2.1 schemas; "
            "2.1 adds the messages of ocpp::v21.");
    }
    return notes;
}

ojson capabilities() {
    ojson versions = ojson::object();
    for (const auto& [version, actions] : declared()) {
        ojson v = ojson::object();
        ojson client = ojson::object();
        client["supported"] = true;
        client["sends"] = actions.sends;
        client["handles"] = actions.handles;
        ojson server = ojson::object();
        server["supported"] = false;
        server["sends"] = ojson::array();
        server["handles"] = ojson::array();
        ojson features = ojson::object();
        features["securityProfiles"] = {0, 1};
        features["compression"] = false;
        features["send"] = false;
        features["callResultError"] = false;
        features["fixtureCallErrors"] = true;
        features["messageIds"] = true;
        v["client"] = client;
        v["server"] = server;
        v["features"] = features;
        v["notes"] = version_notes(version);
        versions[version] = v;
    }
    ojson upstream = ojson::object();
    upstream["name"] = "EVerest libocpp";
    upstream["repository"] = "https://github.com/EVerest/everest-core";
    upstream["version"] = UPSTREAM_VERSION;
    upstream["revision"] = UPSTREAM_REVISION;
    upstream["license"] = "Apache-2.0";
    upstream["language"] = "C++";
    upstream["independentEngine"] = true;
    ojson doc = ojson::object();
    doc["schemaVersion"] = schema_version;
    doc["adapter"] = adapter_name;
    doc["adapterVersion"] = ADAPTER_VERSION;
    doc["contractVersion"] = contract_version;
    doc["upstream"] = upstream;
    doc["versions"] = versions;
    return doc;
}

int command_server(const std::vector<std::string>& args) {
    const auto flags = parse_flags(args, {"version", "fixture", "bind-address", "port", "security-profile", "tls-dir",
                                          "ready-file", "control-socket"});
    if (!flags.positional.empty()) {
        throw UsageError("unexpected argument \"" + flags.positional[0] + "\"");
    }
    flags.number("port", 9000, 1, 65535);
    version_of(flags);
    const auto profile = flags.number("security-profile", 0, 0, 3);
    if (flags.has("tls-dir") && profile < 2) {
        throw UsageError("--tls-dir is for security profiles 2 and 3");
    }
    throw Unsupported("server: libocpp implements the charging station only, not a CSMS");
}

int command_call(const std::vector<std::string>& args) {
    const auto flags = parse_flags(args, {"station", "payload", "timeout-ms", "control-socket"});
    if (flags.positional.size() != 1) {
        throw UsageError("exactly one action is required");
    }
    const auto action = action_of(flags.positional[0]);
    const auto payload = payload_of(flags);
    const auto timeout_ms = flags.number("timeout-ms", 10000, 1, 3600000);
    std::optional<std::string> station;
    if (flags.has("station")) {
        station = flags.get("station", "");
    }
    return call_control(flags.get("control-socket", default_control_socket), station, action, payload, timeout_ms);
}

int command_print_fixture(const std::vector<std::string>& args) {
    const auto flags = parse_flags(args, {"version"});
    const auto dir = version_dir(flags.get("version", ""));
    if (dir.empty()) {
        throw UsageError("--version 1.6, 2.0.1 or 2.1 is required");
    }
    if (flags.positional.size() > 1) {
        throw UsageError("at most one fixture name");
    }
    const std::string name = flags.positional.empty() ? "baseline" : flags.positional[0];
    static const std::regex fixture_name("^[a-z0-9][a-z0-9-]*$");
    if (!std::regex_match(name, fixture_name)) {
        throw UsageError("\"" + name + "\" is not a fixture name");
    }
    std::ifstream in(std::string(fixtures_dir) + "/" + dir + "/" + name + "/fixture.json", std::ios::binary);
    if (!in) {
        std::fprintf(stderr, "print-fixture: no fixture \"%s\" for OCPP %s in this image\n", name.c_str(),
                     flags.get("version", "").c_str());
        return 1;
    }
    std::stringstream data;
    data << in.rdbuf();
    write_raw(data.str());
    return 0;
}

int dispatch(const std::vector<std::string>& args) {
    if (args.empty()) {
        throw UsageError("no command");
    }
    const std::vector<std::string> rest(args.begin() + 1, args.end());
    const auto& command = args[0];
    if (command == "server") {
        return command_server(rest);
    }
    if (command == "client") {
        return command_client(rest);
    }
    if (command == "call") {
        return command_call(rest);
    }
    if (command == "print-capabilities") {
        if (!rest.empty()) {
            throw UsageError("print-capabilities takes no arguments");
        }
        write_line(capabilities());
        return 0;
    }
    if (command == "print-fixture") {
        return command_print_fixture(rest);
    }
    if (command == "version") {
        if (!rest.empty()) {
            throw UsageError("version takes no arguments");
        }
        write_text(std::string("ocpp-interop-") + adapter_name + " " + ADAPTER_VERSION + " (EVerest libocpp, " +
                   "everest-core " + UPSTREAM_VERSION + ", contract " + contract_version + ")");
        return 0;
    }
    throw UsageError("unknown command \"" + command + "\"");
}

} // namespace

} // namespace interop

int main(int argc, char** argv) {
    using namespace interop;
    setup_logging();
    try {
        return dispatch(std::vector<std::string>(argv + 1, argv + argc));
    } catch (const UsageError& e) {
        std::fprintf(stderr, "usage error: %s\n%s\n", e.what(), usage_text);
        return 2;
    } catch (const Unsupported& e) {
        std::fprintf(stderr, "unsupported: %s\n", e.what());
        return 5;
    } catch (const FixtureError& e) {
        std::fprintf(stderr, "invalid fixture: %s\n", e.what());
        return 4;
    } catch (const std::exception& e) {
        std::fprintf(stderr, "error: %s\n", e.what());
        return 1;
    }
}
