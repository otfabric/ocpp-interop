// SPDX-License-Identifier: MIT
//
// Shared declarations of the ocpp-interop adapter for EVerest libocpp.
#ifndef OCPP_INTEROP_COMMON_HPP
#define OCPP_INTEROP_COMMON_HPP

#include <cstdint>
#include <functional>
#include <map>
#include <optional>
#include <set>
#include <stdexcept>
#include <string>
#include <vector>

#include <nlohmann/json.hpp>

namespace interop {

// libocpp's JSON type (nlohmann::json, keys sorted) for everything that goes
// through libocpp; ordered JSON for the documents this adapter writes, so
// that their fields keep the contract's order.
using json = nlohmann::json;
using ojson = nlohmann::ordered_json;

constexpr const char* adapter_name = "everest";
constexpr const char* schema_version = "1.0";
constexpr const char* contract_version = "0.4";
constexpr const char* default_ready_file = "/run/ocpp-interop/ready";
constexpr const char* default_control_socket = "/run/ocpp-interop/control.sock";
constexpr const char* fixtures_dir = "/fixtures";

// Exit 2: a usage error, nothing was sent.
struct UsageError : std::runtime_error {
    using std::runtime_error::runtime_error;
};
// Exit 5: a version, role, security profile, action or fixture answer this
// adapter does not support.
struct Unsupported : std::runtime_error {
    using std::runtime_error::runtime_error;
};
// Exit 4: a fixture that is missing or invalid.
struct FixtureError : std::runtime_error {
    using std::runtime_error::runtime_error;
};
// Ends a command with this exit code, after it wrote its output.
struct ExitCode {
    int code;
};

// ---- output (output.cpp) ----

// stdout carries JSON only, one document per line, flushed as it is written.
void write_line(const ojson& document);
void write_text(const std::string& text);
void write_raw(const std::string& data);
// RFC 3339 UTC with milliseconds.
std::string timestamp();
// RFC 3339 UTC in whole seconds: what "${now}" stands for.
std::string now_seconds();
// One event; fields are added in order. An empty station is left out.
void event(const std::string& name, const std::string& station, const ojson& fields = ojson::object());
// Converts libocpp's JSON (or any nlohmann::json) to ordered JSON.
ojson ordered(const json& value);
json unordered(const ojson& value);

// A result document (schemas/client-result.schema.json).
struct Result {
    std::string version;
    std::string operation;
    bool ok = false;
    std::string message_id;            // left out when empty
    std::optional<json> request;       // the payload as sent, or as given
    std::optional<json> response;      // null when absent
    std::optional<ojson> call_error;   // {code, description, details}
    std::string error_code;            // empty: no error
    std::string error_message;
    std::int64_t elapsed_ms = 0;

    ojson document() const;
};
// The exit code that goes with an error code of a result document.
int exit_code_of(const std::string& error_code);
ojson call_error_object(const std::string& code, const std::string& description, const json& details);

// Logging of libocpp (Boost.Log, through EVerest's liblog) to stderr, at the
// level OCPP_INTEROP_LOG_LEVEL names (default: warnings).
void setup_logging();

// ---- command line (cli.cpp) ----

// `--name value` pairs (and the switch --compression) and the positional
// arguments.
struct Flags {
    std::map<std::string, std::string> given;
    std::vector<std::string> positional;

    bool has(const std::string& name) const;
    std::string get(const std::string& name, const std::string& fallback) const;
    std::int64_t number(const std::string& name, std::int64_t fallback, std::int64_t low, std::int64_t high) const;
};
Flags parse_flags(const std::vector<std::string>& args, const std::set<std::string>& known);
// --version: usage error unless 1.6, 2.0.1 or 2.1.
std::string version_of(const Flags& flags);
// The fixture directory name of a version (v16, v201, v21).
std::string version_dir(const std::string& version);
// --payload: a JSON object, or a usage error.
json payload_of(const Flags& flags);
// An action name, or a usage error.
std::string action_of(const std::string& text);
bool is_action_name(const std::string& text);

// ---- fixtures (fixture.cpp) ----

// The actions a role sends in a version (scripts/validate-fixtures.py).
const std::set<std::string>& actions_from(const std::string& version, const std::string& role);
// Actions a version sends as a SEND (message type 6): never answered.
bool is_send_action(const std::string& version, const std::string& action);

// What the fixture answers to a CALL.
struct Answer {
    enum class Kind { None, Payload, CallError } kind = Kind::None;
    json payload;
    std::string code;
    std::string description;
};

class Fixture {
public:
    // Loads and validates; FixtureError (exit 4) or Unsupported (exit 5).
    static Fixture load(const std::string& path, const std::string& version);

    std::string name;
    std::string version;
    std::string first_station;
    bool has_station(const std::string& id) const;
    std::string password(const std::string& id) const;
    // The answer of `role` ("csms" or "station") to a CALL of `action`
    // with this request (as libocpp read it).
    Answer answer(const std::string& role, const std::string& action, const json& request);

private:
    json doc;
    std::map<std::string, json> stations;
    std::vector<json> variables; // 2.x device model; SetVariables changes values for the process lifetime
    json configuration_answer(const json& request) const;
    json variables_answer(const std::string& action, const json& request);
};
std::string default_fixture_path(const std::string& version);

// ---- libocpp's message types (registry_v16.cpp, registry_v2.cpp) ----

// One action of one version, through libocpp's own types: JSON is read into
// the libocpp request or response struct (its from_json) and written back
// (its to_json). Reading throws what libocpp throws for a payload it cannot
// read.
struct Codec {
    std::function<json(const json&)> request;
    std::function<json(const json&)> response;
    // A CALL frame [2, id, action, payload] as libocpp's Call<T> writes it,
    // with a message id libocpp generates.
    std::function<json(const json&)> call;
    // A CALLRESULT frame [3, id, payload] as libocpp's CallResult<T> writes it.
    std::function<json(const json&, const std::string&)> call_result;
};
// The codec of an action, or nullptr when libocpp has no such message in
// that version.
const Codec* codec(const std::string& version, const std::string& action);
const Codec* codec_v16(const std::string& action);
const Codec* codec_v2(const std::string& action, bool v21);

} // namespace interop

#endif
