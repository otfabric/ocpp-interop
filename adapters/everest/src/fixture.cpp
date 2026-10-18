// SPDX-License-Identifier: MIT
//
// Fixture loading, validation and answers (docs/FIXTURES.md). The checks are
// those of fixtures/schema/ocpp-fixture.schema.json and of
// scripts/validate-fixtures.py, written out by hand so that the image needs
// no JSON Schema library. None of this is OCPP protocol logic: it decides what
// the fixture answers, not how libocpp sends it.
#include "common.hpp"

#include <algorithm>
#include <cctype>
#include <filesystem>
#include <fstream>
#include <regex>
#include <sstream>

namespace interop {

namespace {

std::set<std::string> join(const std::set<std::string>& a, const std::set<std::string>& b) {
    std::set<std::string> out = a;
    out.insert(b.begin(), b.end());
    return out;
}

// Actions by the role that sends them: scripts/validate-fixtures.py.
const std::set<std::string> v16_from_station = {
    "Authorize", "BootNotification", "DataTransfer", "DiagnosticsStatusNotification",
    "FirmwareStatusNotification", "Heartbeat", "MeterValues", "StartTransaction",
    "StatusNotification", "StopTransaction",
    "LogStatusNotification", "SecurityEventNotification", "SignCertificate",
    "SignedFirmwareStatusNotification",
};
const std::set<std::string> v16_from_csms = {
    "CancelReservation", "ChangeAvailability", "ChangeConfiguration", "ClearCache",
    "ClearChargingProfile", "DataTransfer", "GetCompositeSchedule", "GetConfiguration",
    "GetDiagnostics", "GetLocalListVersion", "RemoteStartTransaction",
    "RemoteStopTransaction", "ReserveNow", "Reset", "SendLocalList", "SetChargingProfile",
    "TriggerMessage", "UnlockConnector", "UpdateFirmware",
    "CertificateSigned", "DeleteCertificate", "ExtendedTriggerMessage",
    "GetInstalledCertificateIds", "GetLog", "InstallCertificate", "SignedUpdateFirmware",
};
const std::set<std::string> v201_from_station = {
    "Authorize", "BootNotification", "ClearedChargingLimit", "DataTransfer",
    "FirmwareStatusNotification", "Get15118EVCertificate", "GetCertificateStatus",
    "Heartbeat", "LogStatusNotification", "MeterValues", "NotifyChargingLimit",
    "NotifyCustomerInformation", "NotifyDisplayMessages", "NotifyEVChargingNeeds",
    "NotifyEVChargingSchedule", "NotifyEvent", "NotifyMonitoringReport", "NotifyReport",
    "PublishFirmwareStatusNotification", "ReportChargingProfiles",
    "ReservationStatusUpdate", "SecurityEventNotification", "SignCertificate",
    "StatusNotification", "TransactionEvent",
};
const std::set<std::string> v201_from_csms = {
    "CancelReservation", "CertificateSigned", "ChangeAvailability", "ClearCache",
    "ClearChargingProfile", "ClearDisplayMessage", "ClearVariableMonitoring",
    "CostUpdated", "CustomerInformation", "DataTransfer", "DeleteCertificate",
    "GetBaseReport", "GetChargingProfiles", "GetCompositeSchedule", "GetDisplayMessages",
    "GetInstalledCertificateIds", "GetLocalListVersion", "GetLog", "GetMonitoringReport",
    "GetReport", "GetTransactionStatus", "GetVariables", "InstallCertificate",
    "PublishFirmware", "RequestStartTransaction", "RequestStopTransaction", "ReserveNow",
    "Reset", "SendLocalList", "SetChargingProfile", "SetDisplayMessage",
    "SetMonitoringBase", "SetMonitoringLevel", "SetNetworkProfile",
    "SetVariableMonitoring", "SetVariables", "TriggerMessage", "UnlockConnector",
    "UnpublishFirmware", "UpdateFirmware",
};
const std::set<std::string> v21_from_station = join(v201_from_station, {
    "BatterySwap", "ClosePeriodicEventStream", "GetCertificateChainStatus", "NotifyDERAlarm",
    "NotifyDERStartStop", "NotifyPeriodicEventStream", "NotifyPriorityCharging", "NotifySettlement",
    "OpenPeriodicEventStream", "PullDynamicScheduleUpdate", "ReportDERControl", "VatNumberValidation",
});
const std::set<std::string> v21_from_csms = join(v201_from_csms, {
    "AFRRSignal", "AdjustPeriodicEventStream", "ChangeTransactionTariff", "ClearDERControl",
    "ClearTariffs", "GetDERControl", "GetPeriodicEventStream", "GetTariffs",
    "NotifyAllowedEnergyTransfer", "NotifyWebPaymentStarted", "RequestBatterySwap", "SetDERControl",
    "SetDefaultTariff", "UpdateDynamicSchedule", "UsePriorityCharging",
});

const std::set<std::string> token_statuses = {
    "Accepted", "Blocked", "Expired", "Invalid", "ConcurrentTx", "NoCredit",
    "NotAllowedTypeEVSE", "NotAtThisLocation", "NotAtThisTime", "Unknown",
};

std::string lower(std::string text) {
    std::transform(text.begin(), text.end(), text.begin(),
                   [](unsigned char c) { return static_cast<char>(std::tolower(c)); });
    return text;
}

std::string str(const json& object, const std::string& key) {
    if (!object.is_object()) {
        return "";
    }
    const auto it = object.find(key);
    return (it != object.end() && it->is_string()) ? it->get<std::string>() : "";
}

bool is_string(const json& object, const std::string& key) {
    return object.is_object() && object.contains(key) && object.at(key).is_string();
}

bool same_name(const json& a, const json& b, const std::string& key) {
    return lower(str(a, key)) == lower(str(b, key));
}

// "${now}" becomes the current UTC time, RFC 3339, whole seconds.
json substitute(const json& value) {
    if (value.is_string() && value.get<std::string>() == "${now}") {
        return now_seconds();
    }
    if (value.is_object()) {
        json out = json::object();
        for (const auto& item : value.items()) {
            out[item.key()] = substitute(item.value());
        }
        return out;
    }
    if (value.is_array()) {
        json out = json::array();
        for (const auto& item : value) {
            out.push_back(substitute(item));
        }
        return out;
    }
    return value;
}

// A component or variable: {name, instance?}, names of at most 50 characters.
bool named(const json& value) {
    if (!value.is_object()) {
        return false;
    }
    for (const auto& item : value.items()) {
        if (item.key() != "name" && item.key() != "instance") {
            return false;
        }
    }
    if (!is_string(value, "name") || str(value, "name").size() > 50) {
        return false;
    }
    if (value.contains("instance") && (!is_string(value, "instance") || str(value, "instance").size() > 50)) {
        return false;
    }
    return true;
}

bool is_positive_integer(const json& value, std::int64_t low, std::int64_t high) {
    if (!value.is_number_integer()) {
        return false;
    }
    const auto n = value.get<std::int64_t>();
    return n >= low && n <= high;
}

// The OCPP-J error codes of a version: OCPP 1.6 JSON 4.2.3, OCPP 2.0.1 and
// 2.1 Part 4, 4.3. libocpp's CallError takes any string; the adapter refuses
// other codes as every other adapter does (docs/CONTAINER_CONTRACT.md,
// Answers).
bool is_error_code(const std::string& version, const std::string& code) {
    static const std::set<std::string> v16 = {
        "NotImplemented", "NotSupported", "InternalError", "ProtocolError", "SecurityError",
        "FormationViolation", "PropertyConstraintViolation", "OccurenceConstraintViolation",
        "TypeConstraintViolation", "GenericError",
    };
    static const std::set<std::string> v2 = {
        "FormatViolation", "GenericError", "InternalError", "MessageTypeNotSupported", "NotImplemented",
        "NotSupported", "OccurrenceConstraintViolation", "PropertyConstraintViolation", "ProtocolError",
        "RpcFrameworkError", "SecurityError", "TypeConstraintViolation",
    };
    return (version == "1.6" ? v16 : v2).count(code) > 0;
}

void validate(const json& doc, const std::string& path, const std::string& version, std::vector<std::string>& errors) {
    auto add = [&errors](const std::string& message) { errors.push_back(message); };
    static const std::set<std::string> allowed = {"schemaVersion", "name",          "ocppVersion", "description",
                                                  "stations",      "idTokens",      "configuration",
                                                  "variables",     "answers",       "samples"};
    for (const auto& item : doc.items()) {
        if (allowed.count(item.key()) == 0) {
            add("unknown property " + item.key());
        }
    }
    if (str(doc, "schemaVersion") != "1.0") {
        add("schemaVersion must be \"1.0\"");
    }
    static const std::regex fixture_name("^[a-z0-9][a-z0-9-]*$");
    if (!std::regex_match(str(doc, "name"), fixture_name)) {
        add("name is missing or not a fixture name");
    }
    if (str(doc, "description").empty()) {
        add("description is missing");
    }
    const auto fixture_version = str(doc, "ocppVersion");
    if (version_dir(fixture_version).empty()) {
        add("ocppVersion must be 1.6, 2.0.1 or 2.1");
        return;
    }
    if (fixture_version != version) {
        add("ocppVersion is " + fixture_version + ", but --version is " + version);
        return;
    }
    const bool v16 = version == "1.6";
    const auto absolute = std::filesystem::absolute(path).lexically_normal();
    const auto dir = absolute.parent_path();
    if (dir.filename().string() != str(doc, "name")) {
        add("name is not the directory name");
    }
    if (dir.parent_path().filename().string() != version_dir(version)) {
        add("ocppVersion " + version + " does not belong in this directory");
    }
    if (v16 && doc.contains("variables")) {
        add("variables are for OCPP 2.x");
    }
    if (!v16 && doc.contains("configuration")) {
        add("configuration is for OCPP 1.6");
    }

    // Stations.
    const json stations = doc.value("stations", json());
    if (!stations.is_array() || stations.empty()) {
        add("stations must be a non-empty array");
    }
    static const std::regex identity("^[A-Za-z0-9*\\-_=:+|@.]{1,48}$");
    std::set<std::string> ids;
    const json none = json::array();
    for (const auto& station : stations.is_array() ? stations : none) {
        if (!station.is_object()) {
            add("stations: not an object");
            continue;
        }
        for (const auto* key : {"id", "vendor", "model", "serialNumber", "firmwareVersion", "password"}) {
            if (!is_string(station, key)) {
                add(std::string("stations: ") + key + " is missing");
            }
        }
        static const std::set<std::string> station_keys = {"id",       "vendor",     "model", "serialNumber",
                                                           "firmwareVersion", "password", "connectors", "evses"};
        for (const auto& item : station.items()) {
            if (station_keys.count(item.key()) == 0) {
                add("stations: unknown property " + item.key());
            }
        }
        const auto id = str(station, "id");
        if (!std::regex_match(id, identity)) {
            add("stations: identity \"" + id + "\" is not valid");
        }
        if (ids.count(id) > 0) {
            add("stations: identity " + id + " appears twice");
        }
        ids.insert(id);
        const auto password = str(station, "password");
        if (password.size() < 16 || password.size() > 40) {
            add("stations: password must have 16 to 40 characters");
        }
        if (str(station, "vendor").size() > 20 || str(station, "model").size() > 20 ||
            str(station, "serialNumber").size() > 25 || str(station, "firmwareVersion").size() > 50) {
            add("stations: a station field is too long");
        }
        if (v16) {
            if (!is_positive_integer(station.value("connectors", json()), 1, 16)) {
                add("stations: connectors must be 1..16 in OCPP 1.6");
            }
            if (station.contains("evses")) {
                add("stations: evses are for OCPP 2.x");
            }
            continue;
        }
        if (station.contains("connectors")) {
            add("stations: connectors are for OCPP 1.6");
        }
        const json evses = station.value("evses", json());
        if (!evses.is_array() || evses.empty()) {
            add("stations: evses must be a non-empty array in OCPP 2.x");
            continue;
        }
        std::set<std::int64_t> seen;
        for (const auto& evse : evses) {
            const json connectors = evse.is_object() ? evse.value("connectors", json()) : json();
            if (!evse.is_object() || !is_positive_integer(evse.value("id", json()), 1, INT64_MAX) ||
                !connectors.is_array() || connectors.empty()) {
                add("stations: an EVSE needs an id of 1 or more and connectors");
                continue;
            }
            const auto n = evse.at("id").get<std::int64_t>();
            if (seen.count(n) > 0) {
                add("stations: EVSE " + std::to_string(n) + " appears twice");
            }
            seen.insert(n);
        }
    }

    // Id tokens: CiString20Type (1.6), 36 (2.0.1), 255 (2.1).
    const json tokens = doc.value("idTokens", json());
    if (!tokens.is_array() || tokens.empty()) {
        add("idTokens must be a non-empty array");
    }
    const std::size_t max_token = v16 ? 20 : (version == "2.0.1" ? 36 : 255);
    std::set<std::string> seen_tokens;
    for (const auto& token : tokens.is_array() ? tokens : none) {
        if (!is_string(token, "idToken") || !is_string(token, "status")) {
            add("idTokens: idToken and status are required");
            continue;
        }
        const auto id = str(token, "idToken");
        if (id.empty() || id.size() > max_token) {
            add("idTokens: " + id + " must have 1 to " + std::to_string(max_token) + " characters in OCPP " + version);
        }
        if (!v16 && !is_string(token, "type")) {
            add("idTokens: " + id + " has no type, which OCPP 2.x requires");
        }
        if (token.contains("type") && (!is_string(token, "type") || str(token, "type").size() > 20)) {
            add("idTokens: " + id + " has a type that is not a string of at most 20 characters");
        }
        if (seen_tokens.count(id) > 0) {
            add("idTokens: " + id + " appears twice");
        }
        seen_tokens.insert(id);
        if (token_statuses.count(str(token, "status")) == 0) {
            add("idTokens: " + id + " has an unknown status");
        }
    }

    if (v16) {
        const json configuration = doc.value("configuration", json());
        if (!configuration.is_array()) {
            add("configuration is required in OCPP 1.6");
        }
        std::set<std::string> keys;
        for (const auto& item : configuration.is_array() ? configuration : none) {
            if (!is_string(item, "key") || !is_string(item, "value") || !item.contains("readonly") ||
                !item.at("readonly").is_boolean()) {
                add("configuration: key, value and readonly are required");
                continue;
            }
            if (str(item, "key").size() > 50 || str(item, "value").size() > 500) {
                add("configuration: " + str(item, "key") + " is too long");
            }
            if (keys.count(str(item, "key")) > 0) {
                add("configuration: key " + str(item, "key") + " appears twice");
            }
            keys.insert(str(item, "key"));
        }
    } else {
        const json variables = doc.value("variables", json());
        if (!variables.is_array()) {
            add("variables are required in OCPP 2.x");
        }
        std::set<std::string> seen;
        static const std::set<std::string> mutabilities = {"ReadOnly", "WriteOnly", "ReadWrite"};
        for (const auto& item : variables.is_array() ? variables : none) {
            if (!item.is_object() || !named(item.value("component", json())) ||
                !named(item.value("variable", json())) || !is_string(item, "value") ||
                mutabilities.count(str(item, "mutability")) == 0) {
                add("variables: component, variable, value and mutability are required");
                continue;
            }
            if (str(item, "value").size() > 2500) {
                add("variables: a value is longer than 2500 characters");
            }
            const auto& c = item.at("component");
            const auto& v = item.at("variable");
            const auto key = str(c, "name") + '\0' + str(c, "instance") + '\0' + str(v, "name") + '\0' + str(v, "instance");
            if (seen.count(key) > 0) {
                add("variables: " + str(c, "name") + "/" + str(v, "name") + " appears twice");
            }
            seen.insert(key);
        }
    }

    // Answers: given by the role that receives the action.
    const json answers = doc.value("answers", json());
    if (!answers.is_object()) {
        add("answers is required");
    }
    for (const auto* role : {"csms", "station"}) {
        const json table = answers.is_object() ? answers.value(role, json()) : json();
        if (!table.is_object()) {
            add(std::string("answers.") + role + " is required");
            continue;
        }
        const auto& receives = actions_from(version, std::string(role) == "csms" ? "station" : "csms");
        for (const auto& item : table.items()) {
            const auto& action = item.key();
            const auto where = std::string("answers.") + role + "." + action;
            if (receives.count(action) == 0) {
                add(where + ": the " + role + " does not receive " + action + " in OCPP " + version);
            } else if (is_send_action(version, action)) {
                add(where + ": " + action + " is a SEND in OCPP " + version + ", which has no answer");
            }
            const json& entry = item.value();
            if (!entry.is_object()) {
                add(where + ": not an object");
                continue;
            }
            const bool has_payload = entry.contains("payload");
            const bool has_derived = entry.contains("derived");
            const bool has_error = entry.contains("callError");
            const int forms = (has_error ? 1 : 0) + (has_derived ? 1 : 0) + ((has_payload && !has_derived) ? 1 : 0);
            if (forms != 1) {
                add(where + ": must be exactly one of payload, derived or callError");
                continue;
            }
            for (const auto& field : entry.items()) {
                if (field.key() != "payload" && field.key() != "derived" && field.key() != "callError") {
                    add(where + ": unknown property " + field.key());
                }
            }
            if (has_payload && !entry.at("payload").is_object()) {
                add(where + ": payload must be an object");
            }
            if (has_error) {
                const json& error = entry.at("callError");
                if (!is_string(error, "code") || !is_string(error, "description")) {
                    add(where + ": callError needs code and description");
                }
            }
            if (has_derived) {
                const auto derived = str(entry, "derived");
                const bool fits = (derived == "idTokens" && (action == "Authorize" || action == "StartTransaction")) ||
                                  (derived == "configuration" && action == "GetConfiguration") ||
                                  (derived == "variables" && (action == "GetVariables" || action == "SetVariables"));
                if (!fits) {
                    add(where + ": cannot be derived from " + derived);
                } else {
                    const json table_of = doc.value(derived, json());
                    if (!table_of.is_array() || table_of.empty()) {
                        add(where + ": derived from " + derived + ", which is empty");
                    }
                }
            }
        }
    }

    // Samples: sent by the role that sends the action.
    const json samples = doc.value("samples", json());
    if (!samples.is_object()) {
        add("samples is required");
    }
    static const std::regex sample_name("^[a-z][a-zA-Z0-9]*$");
    const json no_samples = json::object();
    for (const auto& item : (samples.is_object() ? samples : no_samples).items()) {
        const json& sample = item.value();
        if (!std::regex_match(item.key(), sample_name)) {
            add("samples." + item.key() + ": not a sample name");
        }
        if (!is_string(sample, "action") || !is_string(sample, "from") || !sample.contains("payload") ||
            !sample.at("payload").is_object() || !is_action_name(str(sample, "action"))) {
            add("samples." + item.key() + ": action, from and payload are required");
            continue;
        }
        const auto from = str(sample, "from");
        if (from != "station" && from != "csms") {
            add("samples." + item.key() + ": from must be station or csms");
            continue;
        }
        if (actions_from(version, from).count(str(sample, "action")) == 0) {
            add("samples." + item.key() + ": the " + from + " does not send " + str(sample, "action"));
        }
    }
}

} // namespace

const std::set<std::string>& actions_from(const std::string& version, const std::string& role) {
    const bool station = role == "station";
    if (version == "1.6") {
        return station ? v16_from_station : v16_from_csms;
    }
    if (version == "2.0.1") {
        return station ? v201_from_station : v201_from_csms;
    }
    return station ? v21_from_station : v21_from_csms;
}

bool is_send_action(const std::string& version, const std::string& action) {
    // OCPP 2.1 Part 2, 1.54 NotifyPeriodicEventStream; Part 4, SEND.
    return version == "2.1" && action == "NotifyPeriodicEventStream";
}

std::string default_fixture_path(const std::string& version) {
    return std::string(fixtures_dir) + "/" + version_dir(version) + "/baseline/fixture.json";
}

Fixture Fixture::load(const std::string& path, const std::string& version) {
    std::ifstream in(path);
    if (!in) {
        throw FixtureError(path + ": cannot be read");
    }
    std::stringstream text;
    text << in.rdbuf();
    json doc;
    try {
        doc = json::parse(text.str());
    } catch (const json::exception& e) {
        throw FixtureError(path + ": not JSON: " + e.what());
    }
    if (!doc.is_object()) {
        throw FixtureError(path + ": not a JSON object");
    }
    std::vector<std::string> errors;
    validate(doc, path, version, errors);
    if (!errors.empty()) {
        std::string message = path + ": ";
        for (std::size_t i = 0; i < errors.size(); ++i) {
            message += (i > 0 ? "; " : "") + errors[i];
        }
        throw FixtureError(message);
    }
    Fixture fixture;
    fixture.doc = doc;
    fixture.name = str(doc, "name");
    fixture.version = version;
    for (const auto& station : doc.at("stations")) {
        if (fixture.first_station.empty()) {
            fixture.first_station = str(station, "id");
        }
        fixture.stations[str(station, "id")] = station;
    }
    if (doc.contains("variables")) {
        for (const auto& v : doc.at("variables")) {
            fixture.variables.push_back(v);
        }
    }
    // A callError answer must use an OCPP-J error code of the version.
    for (const auto* role : {"csms", "station"}) {
        for (const auto& item : doc.at("answers").at(role).items()) {
            if (item.value().contains("callError")) {
                const auto code = str(item.value().at("callError"), "code");
                if (!is_error_code(version, code)) {
                    throw Unsupported(path + ": answers." + role + "." + item.key() + ": \"" + code +
                                      "\" is not an OCPP-J error code of OCPP " + version);
                }
            }
        }
    }
    return fixture;
}

bool Fixture::has_station(const std::string& id) const {
    return stations.count(id) > 0;
}

std::string Fixture::password(const std::string& id) const {
    const auto it = stations.find(id);
    return it == stations.end() ? "" : str(it->second, "password");
}

Answer Fixture::answer(const std::string& role, const std::string& action, const json& request) {
    Answer answer;
    const json& table = doc.at("answers").at(role);
    if (!table.contains(action)) {
        return answer;
    }
    const json& entry = table.at(action);
    if (entry.contains("callError")) {
        answer.kind = Answer::Kind::CallError;
        answer.code = str(entry.at("callError"), "code");
        answer.description = str(entry.at("callError"), "description");
        return answer;
    }
    answer.kind = Answer::Kind::Payload;
    if (!entry.contains("derived")) {
        answer.payload = substitute(entry.at("payload"));
        return answer;
    }
    const auto derived = str(entry, "derived");
    json payload;
    if (derived == "idTokens") {
        std::string status = "Invalid";
        if (version == "1.6") {
            for (const auto& token : doc.at("idTokens")) {
                if (str(token, "idToken") == str(request, "idTag")) {
                    status = str(token, "status");
                    break;
                }
            }
            payload = {{"idTagInfo", {{"status", status}}}};
        } else {
            const json token_of = request.value("idToken", json::object());
            for (const auto& token : doc.at("idTokens")) {
                if (str(token, "idToken") == str(token_of, "idToken") && token.contains("type") &&
                    str(token, "type") == str(token_of, "type")) {
                    status = str(token, "status");
                    break;
                }
            }
            payload = {{"idTokenInfo", {{"status", status}}}};
        }
    } else if (derived == "configuration") {
        payload = configuration_answer(request);
    } else {
        payload = variables_answer(action, request);
    }
    if (entry.contains("payload")) {
        for (const auto& item : substitute(entry.at("payload")).items()) {
            payload[item.key()] = item.value();
        }
    }
    answer.payload = payload;
    return answer;
}

// GetConfiguration: the requested keys, or all of them; keys compare
// case-insensitively (CiString50Type).
json Fixture::configuration_answer(const json& request) const {
    const json& configuration = doc.at("configuration");
    std::vector<std::string> wanted;
    if (request.contains("key") && request.at("key").is_array() && !request.at("key").empty()) {
        for (const auto& key : request.at("key")) {
            wanted.push_back(key.is_string() ? key.get<std::string>() : "");
        }
    } else {
        for (const auto& item : configuration) {
            wanted.push_back(str(item, "key"));
        }
    }
    json known = json::array();
    json unknown = json::array();
    for (const auto& key : wanted) {
        const json* found = nullptr;
        for (const auto& item : configuration) {
            if (lower(str(item, "key")) == lower(key)) {
                found = &item;
                break;
            }
        }
        if (found == nullptr) {
            unknown.push_back(key);
            continue;
        }
        known.push_back({{"key", found->at("key")}, {"readonly", found->at("readonly")}, {"value", found->at("value")}});
    }
    json answer = {{"configurationKey", known}};
    if (!unknown.empty()) {
        answer["unknownKey"] = unknown;
    }
    return answer;
}

// GetVariables and SetVariables: one result per requested component and
// variable; names and instances compare case-insensitively.
json Fixture::variables_answer(const std::string& action, const json& request) {
    const bool get = action == "GetVariables";
    const std::string key = get ? "getVariableData" : "setVariableData";
    json results = json::array();
    const json items = request.value(key, json::array());
    for (const auto& item : items) {
        const json component = item.value("component", json::object());
        const json variable = item.value("variable", json::object());
        std::string status = "UnknownComponent";
        json* entry = nullptr;
        for (auto& v : variables) {
            if (same_name(v.at("component"), component, "name") && same_name(v.at("component"), component, "instance")) {
                if (status == "UnknownComponent") {
                    status = "UnknownVariable";
                }
                if (same_name(v.at("variable"), variable, "name") && same_name(v.at("variable"), variable, "instance")) {
                    entry = &v;
                    status = "";
                    break;
                }
            }
        }
        const bool has_type = item.contains("attributeType");
        if (status.empty() && has_type && item.at("attributeType") != "Actual") {
            status = "NotSupportedAttributeType";
        }
        if (status.empty() && str(*entry, "mutability") == (get ? "WriteOnly" : "ReadOnly")) {
            status = "Rejected";
        }
        if (status.empty() && !get) {
            (*entry)["value"] = item.value("attributeValue", json(""));
        }
        json result = {{"component", item.value("component", json::object())},
                       {"variable", item.value("variable", json::object())}};
        if (has_type) {
            result["attributeType"] = item.at("attributeType");
        }
        if (status.empty()) {
            result["attributeStatus"] = "Accepted";
            if (get) {
                result["attributeValue"] = entry->at("value");
            }
        } else {
            result["attributeStatus"] = status;
        }
        results.push_back(result);
    }
    return get ? json{{"getVariableResult", results}} : json{{"setVariableResult", results}};
}

} // namespace interop
