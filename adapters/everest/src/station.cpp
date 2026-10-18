// SPDX-License-Identifier: MIT
//
// `client run` and `client <Action>`: a station on libocpp's protocol layer.
//
// libocpp's ChargePoint classes (ocpp::v16::ChargePoint, ocpp::v2::ChargePoint)
// are a charging station's firmware logic: they send BootNotification,
// StatusNotification and Heartbeat by themselves and answer the CSMS from
// their device model. The contract needs a station that sends only the CALLs
// a test asks for and answers from the fixture, so the adapter uses the parts
// of libocpp below ChargePoint, as ChargePoint itself does:
//
//   ocpp::Websocket          the WebSocket client (libwebsockets): handshake,
//                            subprotocol, Basic authentication, TLS
//   ocpp::MessageQueue<M>    sends CALLs one at a time, matches CALLRESULT and
//                            CALLERROR to them, times them out, sends answers
//   the message types        every request and response is read and written
//                            by libocpp's own structs (from_json, to_json),
//                            framed by its Call<T> and CallResult<T>
//
// What the adapter adds is what ChargePoint would decide: which answer to
// give (the fixture's) and when to send (when `call` asks).
#include "station.hpp"

#include <atomic>
#include <chrono>
#include <cmath>
#include <condition_variable>
#include <csignal>
#include <cstdio>
#include <filesystem>
#include <fstream>
#include <mutex>
#include <thread>
#include <unistd.h>

#include <everest/database/sqlite/connection.hpp>
#include <ocpp/common/call_types.hpp>
#include <ocpp/common/database/database_handler_common.hpp>
#include <ocpp/common/evse_security.hpp>
#include <ocpp/common/message_queue.hpp>
#include <ocpp/common/ocpp_logging.hpp>
#include <ocpp/common/types.hpp>
#include <ocpp/common/websocket/websocket.hpp>
#include <ocpp/v16/types.hpp>
#include <ocpp/v2/types.hpp>

namespace interop {

const Codec* codec(const std::string& version, const std::string& action) {
    if (version == "1.6") {
        return codec_v16(action);
    }
    return codec_v2(action, version == "2.1");
}

namespace {

using clock = std::chrono::steady_clock;

std::int64_t elapsed_since(clock::time_point start) {
    return std::chrono::duration_cast<std::chrono::milliseconds>(clock::now() - start).count();
}

std::string truncated(const std::string& text) {
    return text.size() > 4096 ? text.substr(0, 4096) : text;
}

ojson nullable(const std::string& text) {
    return text.empty() ? ojson(nullptr) : ojson(text);
}

// ---- what the frames show: events, and the outcome of a sent CALL ----

// Tracker turns the frames that pass between libocpp's websocket and its
// message queue into events, and remembers what a result needs. It reads
// frames; it does not take part in the protocol.
class Tracker {
public:
    Tracker(bool emit, std::string station) : emit(emit), station(std::move(station)) {
    }

    void incoming(const std::string& text) {
        json frame;
        try {
            frame = json::parse(text);
        } catch (const json::exception& e) {
            report("protocol-error", {{"detail", std::string("not JSON: ") + e.what()}, {"frame", truncated(text)}});
            return;
        }
        if (!frame.is_array() || frame.size() < 3 || !frame[0].is_number_integer() || !frame[1].is_string()) {
            report("protocol-error", {{"detail", "not an OCPP-J message"}, {"frame", truncated(text)}});
            return;
        }
        const int type = frame[0].get<int>();
        const std::string id = frame[1].get<std::string>();
        const std::lock_guard<std::mutex> lock(mutex);
        if (type == 2 && frame.size() == 4 && frame[2].is_string()) {
            received[id] = frame[2].get<std::string>();
            report("call-received", {{"messageId", id}, {"action", frame[2]}, {"payload", ordered(frame[3])}});
        } else if (type == 3 && frame.size() == 3) {
            results.insert(id);
            report("call-result", {{"messageId", id}, {"action", action_of(id)}, {"payload", ordered(frame[2])}});
        } else if (type == 4 && frame.size() >= 4) {
            const auto error = error_of(frame);
            call_errors[id] = error;
            report("call-result", {{"messageId", id}, {"action", action_of(id)}, {"error", error}});
        } else if (type == 5 || type == 6) {
            report("protocol-error",
                   {{"detail", std::string("message type ") + std::to_string(type) +
                                   (type == 5 ? " (CALLRESULTERROR)" : " (SEND)") +
                                   ": libocpp knows the OCPP-J message types 2 to 4 only"},
                    {"frame", truncated(text)}});
        } else {
            report("protocol-error", {{"detail", "not an OCPP-J CALL, CALLRESULT or CALLERROR"}, {"frame", truncated(text)}});
        }
    }

    void outgoing(const json& frame) {
        if (!frame.is_array() || frame.size() < 3 || !frame[1].is_string()) {
            return;
        }
        const int type = frame[0].get<int>();
        const std::string id = frame[1].get<std::string>();
        const std::lock_guard<std::mutex> lock(mutex);
        if (type == 2 && frame.size() == 4) {
            sent_action[id] = frame[2].get<std::string>();
            report("call-sent", {{"messageId", id}, {"action", frame[2]}, {"payload", ordered(frame[3])}});
        } else if (type == 3) {
            const auto action = take_received(id);
            report("call-answered", {{"messageId", id}, {"action", nullable(action)}, {"payload", ordered(frame[2])}});
        } else if (type == 4) {
            const auto action = take_received(id);
            report("call-answered", {{"messageId", id}, {"action", nullable(action)}, {"error", error_of(frame)}});
        }
    }

    std::optional<ojson> call_error(const std::string& id) {
        const std::lock_guard<std::mutex> lock(mutex);
        const auto it = call_errors.find(id);
        if (it == call_errors.end()) {
            return std::nullopt;
        }
        return it->second;
    }

    void protocol_error(const std::string& detail, const std::string& frame) {
        const std::lock_guard<std::mutex> lock(mutex);
        report("protocol-error", {{"detail", detail}, {"frame", truncated(frame)}});
    }

private:
    bool emit;
    std::string station;
    std::mutex mutex;
    std::map<std::string, std::string> received;    // id -> action of a CALL received
    std::map<std::string, std::string> sent_action; // id -> action of a CALL sent
    std::set<std::string> results;                  // ids of CALLRESULTs received
    std::map<std::string, ojson> call_errors;       // id -> CALLERROR received

    static ojson error_of(const json& frame) {
        const std::string code = frame.size() > 2 && frame[2].is_string() ? frame[2].get<std::string>() : "";
        const std::string description = frame.size() > 3 && frame[3].is_string() ? frame[3].get<std::string>() : "";
        const json details = frame.size() > 4 ? frame[4] : json::object();
        return call_error_object(code, description, details);
    }

    ojson action_of(const std::string& id) const {
        const auto it = sent_action.find(id);
        return it == sent_action.end() ? ojson(nullptr) : ojson(it->second);
    }

    std::string take_received(const std::string& id) {
        const auto it = received.find(id);
        if (it == received.end()) {
            return "";
        }
        auto action = it->second;
        received.erase(it);
        return action;
    }

    void report(const std::string& name, const ojson& fields) {
        if (emit) {
            event(name, station, fields);
        }
    }
};

// ---- what libocpp needs around its websocket and queue ----

// The files of --tls-dir, for libocpp's websocket: ca.crt verifies the CSMS
// (profiles 2 and 3), client.crt and client.key are the station's
// certificate (profile 3). Everything else of EVerest's certificate handling
// (installing, signing, OCSP) is not used by a websocket connection.
class TlsFiles : public ocpp::EvseSecurity {
public:
    explicit TlsFiles(std::string dir) : dir(std::move(dir)) {
    }

    ocpp::InstallCertificateResult install_ca_certificate(const std::string&, const ocpp::CaCertificateType&) override {
        return ocpp::InstallCertificateResult::WriteError;
    }
    ocpp::DeleteCertificateResult delete_certificate(const ocpp::CertificateHashDataType&) override {
        return ocpp::DeleteCertificateResult::Failed;
    }
    ocpp::InstallCertificateResult update_leaf_certificate(const std::string&,
                                                           const ocpp::CertificateSigningUseEnum&) override {
        return ocpp::InstallCertificateResult::WriteError;
    }
    ocpp::CertificateValidationResult verify_certificate(const std::string&, const ocpp::LeafCertificateType&) override {
        return ocpp::CertificateValidationResult::Unknown;
    }
    ocpp::CertificateValidationResult verify_certificate(const std::string&,
                                                         const std::vector<ocpp::LeafCertificateType>&) override {
        return ocpp::CertificateValidationResult::Unknown;
    }
    std::vector<ocpp::CertificateHashDataChain> get_installed_certificates(const std::vector<ocpp::CertificateType>&) override {
        return {};
    }
    std::vector<ocpp::OCSPRequestData> get_v2g_ocsp_request_data() override {
        return {};
    }
    std::vector<ocpp::OCSPRequestData> get_mo_ocsp_request_data(const std::string&) override {
        return {};
    }
    void update_ocsp_cache(const ocpp::CertificateHashDataType&, const std::string&) override {
    }
    bool is_ca_certificate_installed(const ocpp::CaCertificateType& type) override {
        return type == ocpp::CaCertificateType::CSMS && !dir.empty() && std::filesystem::exists(dir + "/ca.crt");
    }
    ocpp::GetCertificateSignRequestResult generate_certificate_signing_request(const ocpp::CertificateSigningUseEnum&,
                                                                               const std::string&, const std::string&,
                                                                               const std::string&, bool) override {
        return {ocpp::GetCertificateSignRequestStatus::GenerationError, std::nullopt};
    }
    ocpp::GetCertificateInfoResult get_leaf_certificate_info(const ocpp::CertificateSigningUseEnum& type,
                                                             bool) override {
        ocpp::GetCertificateInfoResult result;
        if (type != ocpp::CertificateSigningUseEnum::ChargingStationCertificate || dir.empty() ||
            !std::filesystem::exists(dir + "/client.crt") || !std::filesystem::exists(dir + "/client.key")) {
            result.status = ocpp::GetCertificateInfoStatus::NotFound;
            return result;
        }
        ocpp::CertificateInfo info{};
        info.certificate_path = dir + "/client.crt";
        info.certificate_count = 1;
        info.key_path = dir + "/client.key";
        result.status = ocpp::GetCertificateInfoStatus::Accepted;
        result.info = info;
        return result;
    }
    bool update_certificate_links(const ocpp::CertificateSigningUseEnum&) override {
        return false;
    }
    std::string get_verify_file(const ocpp::CaCertificateType&) override {
        return dir + "/ca.crt";
    }
    std::string get_verify_location(const ocpp::CaCertificateType&) override {
        return dir + "/ca.crt";
    }
    int get_leaf_expiry_days_count(const ocpp::CertificateSigningUseEnum&) override {
        return 0;
    }

private:
    std::string dir;
};

// The message queue persists transaction-related messages for a station that
// restarts; the adapter's station keeps nothing between runs.
class NoPersistence : public ocpp::common::DatabaseHandlerCommon {
public:
    NoPersistence() :
        ocpp::common::DatabaseHandlerCommon(std::make_unique<everest::db::sqlite::Connection>(":memory:"), "", 0) {
    }
    std::vector<ocpp::common::DBTransactionMessage> get_message_queue_messages(const ocpp::QueueType) override {
        return {};
    }
    void insert_message_queue_message(const ocpp::common::DBTransactionMessage&, const ocpp::QueueType) override {
    }
    void remove_message_queue_message(const std::string&, const ocpp::QueueType) override {
    }
    void clear_message_queue(const ocpp::QueueType) override {
    }

protected:
    void init_sql() override {
    }
};

ocpp::OcppProtocolVersion protocol_of(const std::string& version) {
    if (version == "1.6") {
        return ocpp::OcppProtocolVersion::v16;
    }
    return version == "2.0.1" ? ocpp::OcppProtocolVersion::v201 : ocpp::OcppProtocolVersion::v21;
}

// The OCPP-J error code ChargePoint answers a request it cannot read with
// (ocpp::v16::ChargePointImpl::message_callback, ocpp::v2::ChargePoint::
// message_callback): FormationViolation, in every version.
constexpr const char* unreadable_request_code = "FormationViolation";

template <typename M> class StationConnection : public Connection {
public:
    StationConnection(const Settings& settings, Fixture& fixture) :
        settings(settings), fixture(fixture), tracker(settings.emit, settings.station) {
    }

    ~StationConnection() override = default;

    bool connect(std::string& detail) override {
        ocpp::WebsocketConnectionOptions options{};
        options.ocpp_versions = {protocol_of(settings.version)};
        try {
            options.csms_uri = ocpp::Uri::parse_and_validate(settings.url, settings.station, settings.security_profile);
        } catch (const std::exception& e) {
            detail = std::string("libocpp does not accept the URL: ") + e.what();
            return false;
        }
        options.security_profile = settings.security_profile;
        if (settings.security_profile == 1 || settings.security_profile == 2) {
            options.authorization_key = settings.password;
        }
        options.message_timeout = std::chrono::milliseconds(30000);
        // One attempt: the contract's station does not reconnect.
        options.retry_backoff_random_range_s = 0;
        options.retry_backoff_repeat_times = 0;
        options.retry_backoff_wait_minimum_s = 0;
        options.max_connection_attempts = 0;
        // libocpp's defaults of the OCPP 2.x device model (SecurityCtrlr)
        // and the ECDHE-RSA suites, so that RSA and ECDSA server keys work.
        options.supported_ciphers_12 = "ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-GCM-SHA384:"
                                       "ECDHE-RSA-AES128-GCM-SHA256:ECDHE-RSA-AES256-GCM-SHA384:"
                                       "AES128-GCM-SHA256:AES256-GCM-SHA384";
        options.supported_ciphers_13 = "TLS_AES_256_GCM_SHA384:TLS_AES_128_GCM_SHA256";
        options.ping_interval_s = 0; // no WebSocket pings: every frame is one the test asked for
        options.pong_timeout_s = 0;
        options.use_ssl_default_verify_paths = false;
        options.verify_csms_common_name = true; // the server's certificate must name the host of --url
        options.verify_csms_allow_wildcards = false;
        options.use_tpm_tls = false;

        auto database = std::make_shared<NoPersistence>();
        ocpp::MessageQueueConfig<M> config;
        // A CALL is sent once: a CALLERROR or a timeout is the result the
        // test asked for, never a reason to send it again.
        config.transaction_message_attempts = 1;
        config.transaction_message_retry_interval = 0;
        config.queue_all_messages = false;
        config.message_timeout_seconds = 30;
        // libocpp resends a BootNotification that is answered with a
        // CALLERROR or not at all after this interval: a year.
        config.boot_notification_retry_interval_seconds = 365 * 24 * 3600;
        queue = std::make_unique<ocpp::MessageQueue<M>>([this](json frame) { return send_frame(frame); }, config,
                                                        database);
        queue->start();

        security = std::make_shared<TlsFiles>(settings.tls_dir);
        logging = std::make_shared<ocpp::MessageLogging>(false, "/tmp", "ocpp-interop", false, false, false, false,
                                                         false, false, false, nullptr);
        websocket = std::make_unique<ocpp::Websocket>(options, security, logging);
        websocket->register_connected_callback([this](ocpp::OcppProtocolVersion protocol) {
            {
                const std::lock_guard<std::mutex> lock(mutex);
                connected = true;
                negotiated = protocol;
            }
            queue->resume(std::chrono::seconds(0));
            changed.notify_all();
        });
        websocket->register_disconnected_callback([this]() { on_closed(); });
        websocket->register_stopped_connecting_callback([this](ocpp::WebsocketCloseReason) { on_closed(); });
        websocket->register_connection_failed_callback([this](ocpp::ConnectionFailedReason reason) {
            const std::lock_guard<std::mutex> lock(mutex);
            failure = reason == ocpp::ConnectionFailedReason::FailedToAuthenticateAtCsms
                          ? "the CSMS refused the station's credentials (HTTP 401)"
                          : "the CSMS's certificate does not verify";
        });
        websocket->register_message_callback([this](const std::string& message) { on_message(message); });
        if (!websocket->start_connecting()) {
            detail = "libocpp could not start connecting";
            return false;
        }
        std::unique_lock<std::mutex> lock(mutex);
        const bool settled = changed.wait_for(lock, std::chrono::milliseconds(settings.connect_timeout_ms),
                                              [this] { return connected || stopped; });
        if (connected) {
            return true;
        }
        if (!settled) {
            detail = "no connection within " + std::to_string(settings.connect_timeout_ms) + " ms";
        } else {
            detail = failure.empty() ? "the connection failed (handshake refused or no server; see stderr)" : failure;
        }
        return false;
    }

    ojson subprotocol() const override {
        const std::lock_guard<std::mutex> lock(mutex);
        if (!connected || negotiated == ocpp::OcppProtocolVersion::Unknown) {
            return nullptr;
        }
        return ocpp::conversions::ocpp_protocol_version_to_string(negotiated);
    }

    bool closed() const override {
        const std::lock_guard<std::mutex> lock(mutex);
        return lost;
    }

    bool wait_closed(std::int64_t ms) override {
        std::unique_lock<std::mutex> lock(mutex);
        return changed.wait_for(lock, std::chrono::milliseconds(ms), [this] { return lost; });
    }

    void close() override {
        {
            const std::lock_guard<std::mutex> lock(mutex);
            closing = true;
        }
        if (websocket) {
            websocket->disconnect(ocpp::WebsocketCloseReason::Normal);
        }
    }

    Result execute(const std::string& action, const json& payload, std::int64_t timeout_ms) override {
        const auto started = clock::now();
        try {
            return execute_call(action, payload, timeout_ms, started);
        } catch (const std::exception& e) {
            Result result;
            result.version = settings.version;
            result.operation = action;
            result.request = payload;
            result.error_code = "failed";
            result.error_message = e.what();
            result.elapsed_ms = elapsed_since(started);
            return result;
        }
    }

private:
    Result execute_call(const std::string& action, const json& payload, std::int64_t timeout_ms,
                        clock::time_point started) {
        Result result;
        result.version = settings.version;
        result.operation = action;
        result.request = payload;
        auto done = [&](Result r) {
            r.elapsed_ms = elapsed_since(started);
            return r;
        };
        const Codec* c = codec(settings.version, action);
        if (is_send_action(settings.version, action)) {
            result.error_code = "unsupported";
            result.error_message = action + " is a SEND in OCPP " + settings.version +
                                   ": libocpp knows the OCPP-J message types 2 to 4 only";
            return done(result);
        }
        if (c == nullptr) {
            result.error_code = "unsupported";
            result.error_message = "libocpp has no OCPP " + settings.version + " message " + action;
            return done(result);
        }
        if (actions_from(settings.version, "station").count(action) == 0) {
            result.error_code = "unsupported";
            result.error_message = "the station does not send " + action + " in OCPP " + settings.version;
            return done(result);
        }
        json frame;
        try {
            frame = c->call(payload);
        } catch (const std::exception& e) {
            result.error_code = "invalid-request";
            result.error_message = "libocpp cannot read the payload as " + action + "Request: " + e.what();
            return done(result);
        }
        result.request = frame[3];
        const std::string id = frame[1].get<std::string>();
        if (closed()) {
            result.error_code = "not-connected";
            result.error_message = "the connection is closed";
            return done(result);
        }
        result.message_id = id;

        // libocpp's queue times the CALL out itself, a little after the
        // adapter stops waiting, so that the next CALL is not held up for
        // long by one that was never answered.
        std::future<ocpp::EnhancedMessage<M>> outcome;
        {
            const std::lock_guard<std::mutex> lock(send_mutex);
            queue->update_message_timeout(static_cast<int>(std::ceil(static_cast<double>(timeout_ms) / 1000.0)) + 1);
            outcome = queue->push_call_async(frame);
        }
        const auto deadline = started + std::chrono::milliseconds(timeout_ms);
        while (true) {
            if (outcome.wait_for(std::chrono::milliseconds(10)) == std::future_status::ready) {
                const auto message = outcome.get();
                if (message.messageTypeId == ocpp::MessageTypeId::CALLRESULT) {
                    try {
                        result.response = c->response(message.message.at(ocpp::CALLRESULT_PAYLOAD));
                        result.ok = true;
                    } catch (const std::exception& e) {
                        result.error_code = "invalid-response";
                        result.error_message = std::string("libocpp cannot read the response: ") + e.what();
                    }
                    return done(result);
                }
                if (message.messageTypeId == ocpp::MessageTypeId::CALLERROR) {
                    return done(call_error_result(result, id));
                }
                if (closed()) {
                    result.error_code = "connection-lost";
                    result.error_message = "the connection closed while waiting for the response";
                } else if (message.offline && elapsed_since(started) < 50 && !is_connected()) {
                    result.error_code = "not-connected";
                    result.error_message = "the connection is not open";
                } else {
                    result.error_code = "timeout";
                    result.error_message = "no response within " + std::to_string(timeout_ms) + " ms";
                }
                return done(result);
            }
            // libocpp's queue keeps a BootNotification answered with a
            // CALLERROR for a retry instead of completing it: the frame
            // decides.
            if (tracker.call_error(id).has_value()) {
                return done(call_error_result(result, id));
            }
            if (closed()) {
                result.error_code = "connection-lost";
                result.error_message = "the connection closed while waiting for the response";
                return done(result);
            }
            if (clock::now() >= deadline) {
                result.error_code = "timeout";
                result.error_message = "no response within " + std::to_string(timeout_ms) + " ms";
                return done(result);
            }
        }
    }

    Settings settings;
    Fixture& fixture;
    std::mutex fixture_mutex;
    Tracker tracker;
    std::shared_ptr<TlsFiles> security;
    std::shared_ptr<ocpp::MessageLogging> logging;
    std::unique_ptr<ocpp::Websocket> websocket;
    std::unique_ptr<ocpp::MessageQueue<M>> queue;
    std::mutex send_mutex;

    mutable std::mutex mutex;
    std::condition_variable changed;
    bool connected = false;
    bool stopped = false; // libocpp stopped connecting (before or after a connection)
    bool lost = false;    // the open connection closed
    bool closing = false; // the adapter closes it
    ocpp::OcppProtocolVersion negotiated = ocpp::OcppProtocolVersion::Unknown;
    std::string failure;

    bool is_connected() const {
        const std::lock_guard<std::mutex> lock(mutex);
        return connected && !lost;
    }

    Result call_error_result(Result result, const std::string& id) {
        const auto error = tracker.call_error(id);
        result.call_error = error.value_or(call_error_object("", "", json::object()));
        result.error_code = "call-error";
        result.error_message = "CALLERROR " + result.call_error->value("code", std::string());
        return result;
    }

    void on_closed() {
        {
            const std::lock_guard<std::mutex> lock(mutex);
            if (connected) {
                lost = true;
            }
            stopped = true;
        }
        if (queue) {
            queue->pause();
        }
        changed.notify_all();
    }

    bool send_frame(const json& frame) {
        tracker.outgoing(frame);
        return websocket != nullptr && websocket->send(frame.dump());
    }

    void send_call_error(const std::string& id, const std::string& code, const std::string& description) {
        queue->push_call_error(ocpp::CallError(ocpp::MessageId(id), code, description, json::object()));
    }

    // Every frame from the CSMS: the events, then libocpp's queue (which
    // completes a sent CALL with its CALLRESULT or CALLERROR), then the
    // answer to a CALL.
    void on_message(const std::string& text) {
        tracker.incoming(text);
        ocpp::EnhancedMessage<M> message;
        try {
            message = queue->receive(text);
        } catch (const ocpp::StringToEnumException& e) {
            // A CALL of an action libocpp does not know in this version.
            answer_unknown(text, "NotImplemented", std::string("unknown action: ") + e.what());
            return;
        } catch (const std::exception& e) {
            // ChargePoint answers what its queue cannot read with this code
            // (GenericError in 1.6, RpcFrameworkError in 2.x) and message id
            // "-1"; the adapter answers a CALL it can identify, with its id.
            answer_unknown(text, settings.version == "1.6" ? "GenericError" : "RpcFrameworkError", e.what());
            return;
        }
        if (message.messageTypeId != ocpp::MessageTypeId::CALL) {
            return;
        }
        const std::string id = message.uniqueId.get();
        const std::string action = message.message.at(ocpp::CALL_ACTION).template get<std::string>();
        const json payload = message.message.at(ocpp::CALL_PAYLOAD);
        const Codec* c = codec(settings.version, action);
        if (c == nullptr || actions_from(settings.version, "csms").count(action) == 0) {
            // As ChargePoint::send_not_implemented_error.
            send_call_error(id, "NotImplemented", "");
            return;
        }
        json request;
        try {
            request = c->request(payload);
        } catch (const std::exception& e) {
            send_call_error(id, unreadable_request_code, e.what());
            return;
        }
        Answer answer;
        {
            const std::lock_guard<std::mutex> lock(fixture_mutex);
            answer = fixture.answer("station", action, request);
        }
        switch (answer.kind) {
        case Answer::Kind::None:
            send_call_error(id, "NotImplemented", "the fixture has no answer for " + action);
            return;
        case Answer::Kind::CallError:
            send_call_error(id, answer.code, answer.description);
            return;
        case Answer::Kind::Payload:
            break;
        }
        json frame;
        try {
            frame = c->call_result(answer.payload, id);
        } catch (const std::exception& e) {
            send_call_error(id, "InternalError", std::string("libocpp cannot read the fixture's answer: ") + e.what());
            return;
        }
        queue->push_call_result(frame);
    }

    void answer_unknown(const std::string& text, const std::string& code, const std::string& description) {
        try {
            const auto frame = json::parse(text);
            if (frame.is_array() && frame.size() == 4 && frame[0] == 2 && frame[1].is_string() &&
                frame[1].get<std::string>().size() <= 36) {
                send_call_error(frame[1].get<std::string>(), code, description);
            }
        } catch (const std::exception&) {
            // Not JSON: the tracker reported it.
        }
    }
};

} // namespace

std::unique_ptr<Connection> make_connection(const Settings& settings, Fixture& fixture) {
    if (settings.version == "1.6") {
        return std::make_unique<StationConnection<ocpp::v16::MessageType>>(settings, fixture);
    }
    return std::make_unique<StationConnection<ocpp::v2::MessageType>>(settings, fixture);
}

// ---- the commands ----

namespace {

// Ends the process without running destructors: libocpp's websocket and
// queue join their threads on destruction, which can wait for network
// timeouts. Everything the process has to say is written by now.
[[noreturn]] void finish(int code) {
    std::fflush(stdout);
    std::fflush(stderr);
    _exit(code);
}

std::string with_station(const std::string& url, const std::string& station) {
    std::string base = url;
    while (!base.empty() && base.back() == '/') {
        base.pop_back();
    }
    return station.empty() ? base : base + "/" + station;
}

int client_run(const Settings& settings, Fixture& fixture, std::int64_t hold_ms, const std::string& ready_file,
               const std::string& control_path) {
    // Signals are taken by sigtimedwait below, in this thread only.
    sigset_t signals;
    sigemptyset(&signals);
    sigaddset(&signals, SIGTERM);
    sigaddset(&signals, SIGINT);
    pthread_sigmask(SIG_BLOCK, &signals, nullptr);

    const auto full_url = with_station(settings.url, settings.station);
    auto connection = make_connection(settings, fixture);
    std::string detail;
    if (!connection->connect(detail)) {
        event("connection-failed", settings.station, {{"url", full_url}, {"detail", detail}});
        event("stopped", "");
        finish(3);
    }
    // libocpp offers no permessage-deflate (it creates its libwebsockets
    // context without extensions).
    event("connection-opened", settings.station,
          {{"subprotocol", connection->subprotocol()}, {"remote", full_url}, {"compression", false}});
    ControlServer control(control_path, [&](const std::optional<std::string>& wanted, const std::string& action,
                                            const json& payload, std::int64_t timeout_ms) {
        if (connection->closed()) {
            Result r;
            r.version = settings.version;
            r.operation = action;
            r.request = payload;
            r.error_code = "not-connected";
            r.error_message = "the connection is closed";
            return r;
        }
        if (wanted.has_value() && !wanted->empty() && *wanted != settings.station) {
            Result r;
            r.version = settings.version;
            r.operation = action;
            r.request = payload;
            r.error_code = "not-connected";
            r.error_message = "no open connection for station \"" + *wanted + "\"";
            return r;
        }
        return connection->execute(action, payload, timeout_ms);
    });
    {
        std::ofstream ready(ready_file);
        if (!ready) {
            std::fprintf(stderr, "client: cannot write %s\n", ready_file.c_str());
            finish(1);
        }
        ready << "ready\n";
    }
    event("ready", settings.station, {{"version", settings.version}, {"url", full_url}});

    const auto started = clock::now();
    bool lost = false;
    while (true) {
        timespec wait{0, 100 * 1000 * 1000};
        if (sigtimedwait(&signals, nullptr, &wait) > 0) {
            break;
        }
        if (hold_ms > 0 && elapsed_since(started) >= hold_ms) {
            break;
        }
        if (connection->closed()) {
            lost = true;
            break;
        }
    }
    std::remove(ready_file.c_str());
    control.stop();
    if (lost) {
        // libocpp does not pass the close code and reason to the application.
        event("connection-closed", settings.station, {{"code", nullptr}, {"reason", nullptr}});
        event("stopped", "");
        finish(1);
    }
    connection->close();
    event("connection-closed", settings.station, {{"code", nullptr}, {"reason", nullptr}});
    event("stopped", "");
    finish(0);
}

} // namespace

int command_client(const std::vector<std::string>& args) {
    if (args.empty() || args[0].rfind("--", 0) == 0) {
        throw UsageError("client needs 'run' or an action");
    }
    const bool run = args[0] == "run";
    std::set<std::string> known = {"version", "url", "station", "fixture", "security-profile", "tls-dir",
                                   "compression", "connect-timeout-ms", "timeout-ms"};
    if (run) {
        known.insert({"hold-ms", "ready-file", "control-socket"});
    } else {
        known.insert("payload");
    }
    const Flags flags = parse_flags(std::vector<std::string>(args.begin() + 1, args.end()), known);
    if (!flags.positional.empty()) {
        throw UsageError("unexpected argument \"" + flags.positional[0] + "\"");
    }
    std::string action;
    if (!run) {
        action = action_of(args[0]);
    }
    if (!flags.has("url")) {
        throw UsageError("--url is required");
    }
    const auto url = flags.get("url", "");
    const bool secure = url.rfind("wss://", 0) == 0;
    if (url.rfind("ws://", 0) != 0 && !secure) {
        throw UsageError("--url must start with ws:// or wss://");
    }
    const auto connect_timeout_ms = flags.number("connect-timeout-ms", 5000, 1, 3600000);
    const auto timeout_ms = flags.number("timeout-ms", 10000, 1, 3600000);
    const auto hold_ms = run ? flags.number("hold-ms", 0, 0, 1LL << 31) : 0;
    json payload = json::object();
    if (!run) {
        payload = payload_of(flags);
    }
    const auto version = version_of(flags);
    const auto profile = static_cast<int>(flags.number("security-profile", 0, 0, 3));
    if (flags.has("tls-dir") && profile < 2) {
        throw UsageError("--tls-dir is for security profiles 2 and 3");
    }
    if (profile >= 2 && !flags.has("tls-dir")) {
        throw UsageError("security profile " + std::to_string(profile) + " needs --tls-dir");
    }
    if (secure != (profile >= 2)) {
        throw UsageError("--url must be wss:// with security profiles 2 and 3, and ws:// otherwise");
    }
    std::string tls_dir;
    if (profile >= 2) {
        tls_dir = flags.get("tls-dir", "");
        std::vector<std::string> needed = {"ca.crt"};
        if (profile == 3) {
            needed.insert(needed.end(), {"client.crt", "client.key"});
        }
        for (const auto& file : needed) {
            if (!std::filesystem::is_regular_file(tls_dir + "/" + file)) {
                throw UsageError("--tls-dir " + tls_dir + ": " + file + " is missing");
            }
        }
    }
    if (flags.has("compression")) {
        throw Unsupported("--compression: libocpp cannot offer permessage-deflate (its libwebsockets client "
                          "context has no extensions)");
    }
    auto fixture = Fixture::load(flags.get("fixture", default_fixture_path(version)), version);
    const auto station = flags.get("station", fixture.first_station);
    if ((profile == 1 || profile == 2) && !fixture.has_station(station)) {
        throw UsageError("--station \"" + station + "\" is not in the fixture: security profiles 1 and 2 need its password");
    }
    if (station.empty()) {
        throw Unsupported("--station \"\": libocpp always appends the station identity to the URL");
    }
    Settings settings;
    settings.version = version;
    settings.url = url;
    settings.station = station;
    settings.security_profile = profile;
    settings.tls_dir = tls_dir;
    settings.password = fixture.password(station);
    settings.connect_timeout_ms = connect_timeout_ms;
    settings.emit = run;

    if (run) {
        return client_run(settings, fixture, hold_ms, flags.get("ready-file", default_ready_file),
                          flags.get("control-socket", default_control_socket));
    }

    const auto started = clock::now();
    Result result;
    result.version = version;
    result.operation = action;
    result.request = payload;
    const Codec* c = codec(version, action);
    if (is_send_action(version, action)) {
        result.error_code = "unsupported";
        result.error_message = action + " is a SEND in OCPP " + version + ": libocpp knows the OCPP-J message types 2 to 4 only";
    } else if (c == nullptr) {
        result.error_code = "unsupported";
        result.error_message = "libocpp has no OCPP " + version + " message " + action;
    } else if (actions_from(version, "station").count(action) == 0) {
        result.error_code = "unsupported";
        result.error_message = "the station does not send " + action + " in OCPP " + version;
    }
    if (!result.error_code.empty()) {
        write_line(result.document());
        finish(5);
    }
    auto connection = make_connection(settings, fixture);
    std::string detail;
    if (!connection->connect(detail)) {
        result.error_code = "connect-failed";
        result.error_message = with_station(url, station) + ": " + detail;
    } else {
        result = connection->execute(action, payload, timeout_ms);
    }
    result.elapsed_ms = elapsed_since(started);
    write_line(result.document());
    connection->close();
    finish(exit_code_of(result.error_code));
}

} // namespace interop
