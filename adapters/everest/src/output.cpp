// SPDX-License-Identifier: MIT
//
// stdout (JSON only: events, result documents, capabilities) and stderr
// (libocpp's logs).
#include "common.hpp"

#include <cctype>
#include <chrono>
#include <cstdio>
#include <cstdlib>
#include <ctime>
#include <iostream>
#include <mutex>

#include <boost/core/null_deleter.hpp>
#include <boost/log/core.hpp>
#include <boost/log/expressions.hpp>
#include <boost/log/sinks/sync_frontend.hpp>
#include <boost/log/sinks/text_ostream_backend.hpp>
#include <boost/make_shared.hpp>
#include <everest/logging.hpp>

namespace interop {

namespace {
std::mutex stdout_mutex;

std::string format_time(bool milliseconds) {
    const auto now = std::chrono::system_clock::now();
    const auto seconds = std::chrono::system_clock::to_time_t(now);
    std::tm utc{};
    gmtime_r(&seconds, &utc);
    char buffer[40];
    std::strftime(buffer, sizeof(buffer), "%Y-%m-%dT%H:%M:%S", &utc);
    std::string text = buffer;
    if (milliseconds) {
        const auto ms =
            std::chrono::duration_cast<std::chrono::milliseconds>(now.time_since_epoch()).count() % 1000;
        char fraction[8];
        std::snprintf(fraction, sizeof(fraction), ".%03d", static_cast<int>(ms));
        text += fraction;
    }
    return text + "Z";
}
} // namespace

void write_raw(const std::string& data) {
    const std::lock_guard<std::mutex> lock(stdout_mutex);
    std::fwrite(data.data(), 1, data.size(), stdout);
    std::fflush(stdout);
}

void write_line(const ojson& document) {
    // Invalid UTF-8 from a peer is replaced, never a reason to lose the line.
    write_raw(document.dump(-1, ' ', false, ojson::error_handler_t::replace) + "\n");
}

void write_text(const std::string& text) {
    write_raw(text + "\n");
}

std::string timestamp() {
    return format_time(true);
}

std::string now_seconds() {
    return format_time(false);
}

ojson ordered(const json& value) {
    return ojson::parse(value.dump(-1, ' ', false, json::error_handler_t::replace));
}

json unordered(const ojson& value) {
    return json::parse(value.dump(-1, ' ', false, ojson::error_handler_t::replace));
}

void event(const std::string& name, const std::string& station, const ojson& fields) {
    ojson e = ojson::object();
    e["event"] = name;
    e["time"] = timestamp();
    if (!station.empty()) {
        e["station"] = station;
    }
    for (const auto& item : fields.items()) {
        e[item.key()] = item.value();
    }
    write_line(e);
}

ojson call_error_object(const std::string& code, const std::string& description, const json& details) {
    ojson e = ojson::object();
    e["code"] = code;
    e["description"] = description;
    e["details"] = details.is_null() ? ojson::object() : ordered(details);
    return e;
}

ojson Result::document() const {
    ojson d = ojson::object();
    d["schemaVersion"] = schema_version;
    d["adapter"] = adapter_name;
    d["version"] = version;
    d["operation"] = operation;
    d["ok"] = ok;
    if (!message_id.empty()) {
        d["messageId"] = message_id;
    }
    if (request.has_value()) {
        d["request"] = ordered(*request);
    }
    d["response"] = response.has_value() ? ordered(*response) : ojson(nullptr);
    d["callError"] = call_error.has_value() ? *call_error : ojson(nullptr);
    if (error_code.empty()) {
        d["error"] = nullptr;
    } else {
        ojson e = ojson::object();
        e["code"] = error_code;
        e["message"] = error_message;
        d["error"] = e;
    }
    d["elapsedMs"] = elapsed_ms < 0 ? 0 : elapsed_ms;
    return d;
}

int exit_code_of(const std::string& error_code) {
    if (error_code.empty()) {
        return 0;
    }
    if (error_code == "connect-failed") {
        return 3;
    }
    if (error_code == "unsupported") {
        return 5;
    }
    return 1;
}

void setup_logging() {
    namespace logging = boost::log;
    namespace sinks = boost::log::sinks;
    namespace expr = boost::log::expressions;

    std::string level;
    if (const char* value = std::getenv("OCPP_INTEROP_LOG_LEVEL")) {
        level = value;
    }
    for (auto& c : level) {
        c = static_cast<char>(std::tolower(static_cast<unsigned char>(c)));
    }
    auto threshold = Everest::Logging::warning;
    if (level == "debug" || level == "trace" || level == "verbose") {
        threshold = Everest::Logging::debug;
    } else if (level == "info") {
        threshold = Everest::Logging::info;
    } else if (level == "error") {
        threshold = Everest::Logging::error;
    }

    // One sink on stderr; with a sink of its own Boost.Log no longer uses
    // its default one.
    using backend_t = sinks::text_ostream_backend;
    auto backend = boost::make_shared<backend_t>();
    backend->add_stream(boost::shared_ptr<std::ostream>(&std::clog, boost::null_deleter()));
    backend->auto_flush(true);
    auto sink = boost::make_shared<sinks::synchronous_sink<backend_t>>(backend);
    sink->set_formatter([](const logging::record_view& record, logging::formatting_ostream& stream) {
        static const char* names[] = {"VERBOSE", "DEBUG", "INFO", "WARNING", "ERROR", "CRITICAL"};
        const auto severity = logging::extract<Everest::Logging::severity_level>("Severity", record);
        int index = severity ? static_cast<int>(*severity) : 2;
        if (index < 0 || index > 5) {
            index = 2;
        }
        stream << format_time(true) << " " << names[index] << " libocpp: " << record[expr::smessage];
    });
    sink->set_filter(expr::attr<Everest::Logging::severity_level>("Severity") >= threshold);
    logging::core::get()->remove_all_sinks();
    logging::core::get()->add_sink(sink);
}

} // namespace interop
