// SPDX-License-Identifier: MIT
//
// A station's connection to its CSMS, on libocpp's websocket client
// (ocpp::Websocket, libwebsockets) and message queue (ocpp::MessageQueue).
#ifndef OCPP_INTEROP_STATION_HPP
#define OCPP_INTEROP_STATION_HPP

#include "common.hpp"

#include <memory>

namespace interop {

struct Settings {
    std::string version;
    std::string url;     // the --url, without the station identity
    std::string station; // the identity, appended to the URL as the last path segment
    int security_profile = 0;
    std::string tls_dir; // profiles 2 and 3
    std::string password;
    std::int64_t connect_timeout_ms = 5000;
    bool emit = false; // write events (client run)
};

class Connection {
public:
    virtual ~Connection() = default;
    // Connects; false with a reason when it cannot.
    virtual bool connect(std::string& detail) = 0;
    // Sends one CALL and waits for its outcome.
    virtual Result execute(const std::string& action, const json& payload, std::int64_t timeout_ms) = 0;
    // The negotiated subprotocol, null when libocpp does not know it.
    virtual ojson subprotocol() const = 0;
    virtual bool closed() const = 0;
    // Waits until the connection closes or `ms` pass; true when it closed.
    virtual bool wait_closed(std::int64_t ms) = 0;
    virtual void close() = 0;
};

std::unique_ptr<Connection> make_connection(const Settings& settings, Fixture& fixture);

// Commands (station.cpp, control.cpp, main.cpp).
int command_client(const std::vector<std::string>& args);

// The control socket: serves `call` requests until stop() (control.cpp).
class ControlServer {
public:
    using Handler = std::function<Result(const std::optional<std::string>& station, const std::string& action,
                                         const json& payload, std::int64_t timeout_ms)>;
    ControlServer(const std::string& path, Handler handler);
    ~ControlServer();
    void stop();

private:
    struct Impl;
    std::unique_ptr<Impl> impl;
};
// The `call` command: one request over the control socket.
int call_control(const std::string& path, const std::optional<std::string>& station, const std::string& action,
                 const json& payload, std::int64_t timeout_ms);

} // namespace interop

#endif
