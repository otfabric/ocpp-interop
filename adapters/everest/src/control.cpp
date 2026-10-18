// SPDX-License-Identifier: MIT
//
// The control socket (docs/CONTAINER_CONTRACT.md, `call`): one JSON request
// line in, one result document line out.
//
//   {"station": "ID" | null, "action": "Reset", "payload": {...}, "timeoutMs": 10000}
#include "station.hpp"

#include <atomic>
#include <cerrno>
#include <cstdio>
#include <cstring>
#include <filesystem>
#include <thread>

#include <poll.h>
#include <sys/socket.h>
#include <sys/un.h>
#include <unistd.h>

namespace interop {

namespace {

bool read_line(int fd, std::string& line, int timeout_ms) {
    char buffer[4096];
    while (true) {
        const auto newline = line.find('\n');
        if (newline != std::string::npos) {
            line.resize(newline);
            return true;
        }
        pollfd p{fd, POLLIN, 0};
        const int ready = poll(&p, 1, timeout_ms);
        if (ready <= 0) {
            return false;
        }
        const auto n = read(fd, buffer, sizeof(buffer));
        if (n <= 0) {
            return !line.empty();
        }
        line.append(buffer, static_cast<std::size_t>(n));
        if (line.size() > (16u << 20)) {
            return false;
        }
    }
}

bool write_all(int fd, const std::string& data) {
    std::size_t done = 0;
    while (done < data.size()) {
        const auto n = write(fd, data.data() + done, data.size() - done);
        if (n < 0 && errno == EINTR) {
            continue;
        }
        if (n <= 0) {
            return false;
        }
        done += static_cast<std::size_t>(n);
    }
    return true;
}

sockaddr_un address_of(const std::string& path) {
    sockaddr_un address{};
    address.sun_family = AF_UNIX;
    std::strncpy(address.sun_path, path.c_str(), sizeof(address.sun_path) - 1);
    return address;
}

} // namespace

struct ControlServer::Impl {
    std::string path;
    Handler handler;
    int fd = -1;
    std::atomic<bool> running{true};
    std::thread acceptor;

    void serve_one(int client) {
        std::string line;
        if (read_line(client, line, 60000)) {
            try {
                const auto request = json::parse(line);
                std::optional<std::string> station;
                if (request.contains("station") && request.at("station").is_string()) {
                    station = request.at("station").get<std::string>();
                }
                const auto action = request.value("action", std::string());
                json payload = request.value("payload", json::object());
                if (!payload.is_object()) {
                    payload = json::object();
                }
                std::int64_t timeout_ms = request.value("timeoutMs", static_cast<std::int64_t>(10000));
                if (timeout_ms <= 0) {
                    timeout_ms = 10000;
                }
                const auto result = handler(station, action, payload, timeout_ms);
                write_all(client, result.document().dump(-1, ' ', false, ojson::error_handler_t::replace) + "\n");
            } catch (const std::exception& e) {
                std::fprintf(stderr, "control socket: %s\n", e.what());
            }
        }
        ::close(client);
    }
};

ControlServer::ControlServer(const std::string& path, Handler handler) : impl(std::make_unique<Impl>()) {
    impl->path = path;
    impl->handler = std::move(handler);
    std::error_code ignored;
    std::filesystem::remove(path, ignored);
    std::filesystem::create_directories(std::filesystem::path(path).parent_path(), ignored);
    impl->fd = socket(AF_UNIX, SOCK_STREAM | SOCK_CLOEXEC, 0);
    const auto address = address_of(path);
    if (impl->fd < 0 || bind(impl->fd, reinterpret_cast<const sockaddr*>(&address), sizeof(address)) != 0 ||
        listen(impl->fd, 16) != 0) {
        std::fprintf(stderr, "client: cannot open the control socket %s: %s\n", path.c_str(), std::strerror(errno));
        return;
    }
    Impl* state = impl.get();
    impl->acceptor = std::thread([state] {
        while (state->running) {
            pollfd p{state->fd, POLLIN, 0};
            if (poll(&p, 1, 100) <= 0) {
                continue;
            }
            const int client = accept4(state->fd, nullptr, nullptr, SOCK_CLOEXEC);
            if (client < 0) {
                continue;
            }
            std::thread([state, client] { state->serve_one(client); }).detach();
        }
    });
}

void ControlServer::stop() {
    if (!impl->running.exchange(false)) {
        return;
    }
    if (impl->acceptor.joinable()) {
        impl->acceptor.join();
    }
    if (impl->fd >= 0) {
        ::close(impl->fd);
        impl->fd = -1;
    }
    std::error_code ignored;
    std::filesystem::remove(impl->path, ignored);
}

ControlServer::~ControlServer() {
    stop();
}

int call_control(const std::string& path, const std::optional<std::string>& station, const std::string& action,
                 const json& payload, std::int64_t timeout_ms) {
    const int fd = socket(AF_UNIX, SOCK_STREAM | SOCK_CLOEXEC, 0);
    const auto address = address_of(path);
    if (fd < 0 || connect(fd, reinterpret_cast<const sockaddr*>(&address), sizeof(address)) != 0) {
        std::fprintf(stderr, "call: no running server or client run on %s: %s\n", path.c_str(), std::strerror(errno));
        return 3;
    }
    ojson request = ojson::object();
    request["station"] = station.has_value() ? ojson(*station) : ojson(nullptr);
    request["action"] = action;
    request["payload"] = ordered(payload);
    request["timeoutMs"] = timeout_ms;
    if (!write_all(fd, request.dump() + "\n")) {
        std::fprintf(stderr, "call: cannot write to %s\n", path.c_str());
        ::close(fd);
        return 3;
    }
    std::string line;
    const bool got = read_line(fd, line, static_cast<int>(timeout_ms + 15000));
    ::close(fd);
    ojson document;
    try {
        if (!got) {
            throw std::runtime_error("no answer");
        }
        document = ojson::parse(line);
    } catch (const std::exception&) {
        std::fprintf(stderr, "call: the running process did not answer\n");
        return 1;
    }
    write_raw(line + "\n");
    std::string error_code;
    if (document.contains("error") && document.at("error").is_object()) {
        error_code = document.at("error").value("code", std::string());
    }
    return exit_code_of(error_code);
}

} // namespace interop
