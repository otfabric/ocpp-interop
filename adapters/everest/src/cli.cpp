// SPDX-License-Identifier: MIT
//
// The command line of docs/CONTAINER_CONTRACT.md: `--name value` flags, the
// version, the payload and action names.
#include "common.hpp"

#include <regex>

namespace interop {

namespace {
// Flags without a value.
const std::set<std::string> switches = {"compression"};
} // namespace

bool Flags::has(const std::string& name) const {
    return given.count(name) > 0;
}

std::string Flags::get(const std::string& name, const std::string& fallback) const {
    const auto it = given.find(name);
    return it == given.end() ? fallback : it->second;
}

std::int64_t Flags::number(const std::string& name, std::int64_t fallback, std::int64_t low,
                           std::int64_t high) const {
    const auto it = given.find(name);
    if (it == given.end()) {
        return fallback;
    }
    static const std::regex digits("^[0-9]{1,12}$");
    if (!std::regex_match(it->second, digits)) {
        throw UsageError("--" + name + ": \"" + it->second + "\" is not a number");
    }
    const auto value = std::stoll(it->second);
    if (value < low || value > high) {
        throw UsageError("--" + name + ": " + it->second + " is not within " + std::to_string(low) + ".." +
                         std::to_string(high));
    }
    return value;
}

Flags parse_flags(const std::vector<std::string>& args, const std::set<std::string>& known) {
    Flags flags;
    for (std::size_t i = 0; i < args.size(); ++i) {
        const std::string& arg = args[i];
        if (arg.rfind("--", 0) != 0) {
            flags.positional.push_back(arg);
            continue;
        }
        const std::string name = arg.substr(2);
        if (known.count(name) == 0) {
            throw UsageError("unknown flag \"" + arg + "\"");
        }
        if (flags.given.count(name) > 0) {
            throw UsageError(arg + " is given more than once");
        }
        if (switches.count(name) > 0) {
            flags.given[name] = "true";
            continue;
        }
        if (i + 1 >= args.size()) {
            throw UsageError(arg + " needs a value");
        }
        flags.given[name] = args[++i];
    }
    return flags;
}

std::string version_dir(const std::string& version) {
    if (version == "1.6") {
        return "v16";
    }
    if (version == "2.0.1") {
        return "v201";
    }
    if (version == "2.1") {
        return "v21";
    }
    return "";
}

std::string version_of(const Flags& flags) {
    if (!flags.has("version")) {
        throw UsageError("--version is required");
    }
    const auto version = flags.get("version", "");
    if (version_dir(version).empty()) {
        throw UsageError("--version: \"" + version + "\" is not 1.6, 2.0.1 or 2.1");
    }
    return version;
}

json payload_of(const Flags& flags) {
    json payload;
    try {
        payload = json::parse(flags.get("payload", "{}"));
    } catch (const json::exception& e) {
        throw UsageError(std::string("--payload is not JSON: ") + e.what());
    }
    if (!payload.is_object()) {
        throw UsageError("--payload must be a JSON object");
    }
    return payload;
}

bool is_action_name(const std::string& text) {
    static const std::regex action("^[A-Z][A-Za-z0-9]+$");
    return std::regex_match(text, action);
}

std::string action_of(const std::string& text) {
    if (!is_action_name(text)) {
        throw UsageError("\"" + text + "\" is not an OCPP action name");
    }
    return text;
}

} // namespace interop
