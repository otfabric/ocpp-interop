// SPDX-License-Identifier: MIT
//
// A Codec (common.hpp) from libocpp's request and response types of one
// action. Everything here is libocpp's: its from_json reads a payload into
// the typed message and throws when it cannot, its to_json writes it back,
// and its Call<T> and CallResult<T> write the OCPP-J frames.
#ifndef OCPP_INTEROP_CODEC_HPP
#define OCPP_INTEROP_CODEC_HPP

#include "common.hpp"

#include <ocpp/common/call_types.hpp>

namespace interop {

template <typename Request, typename Response> Codec make_codec() {
    Codec codec;
    codec.request = [](const json& payload) {
        const Request request = payload;
        return json(request);
    };
    codec.response = [](const json& payload) {
        const Response response = payload;
        return json(response);
    };
    codec.call = [](const json& payload) {
        const Request request = payload;
        return json(ocpp::Call<Request>(request));
    };
    codec.call_result = [](const json& payload, const std::string& message_id) {
        const Response response = payload;
        return json(ocpp::CallResult<Response>(response, ocpp::MessageId(message_id)));
    };
    return codec;
}

} // namespace interop

#endif
