// SPDX-License-Identifier: MIT
//
// OCPP 1.6: libocpp's message types (namespace ocpp::v16), by action.
//
// The list is every message header of include/ocpp/v16/messages in the pinned
// everest-core release; the image build checks that it still is (a candidate
// release with another set of messages fails the build until the list is
// updated).
#include "codec.hpp"

#include <ocpp/v16/messages/Authorize.hpp>
#include <ocpp/v16/messages/BootNotification.hpp>
#include <ocpp/v16/messages/CancelReservation.hpp>
#include <ocpp/v16/messages/CertificateSigned.hpp>
#include <ocpp/v16/messages/ChangeAvailability.hpp>
#include <ocpp/v16/messages/ChangeConfiguration.hpp>
#include <ocpp/v16/messages/ClearCache.hpp>
#include <ocpp/v16/messages/ClearChargingProfile.hpp>
#include <ocpp/v16/messages/DataTransfer.hpp>
#include <ocpp/v16/messages/DeleteCertificate.hpp>
#include <ocpp/v16/messages/DiagnosticsStatusNotification.hpp>
#include <ocpp/v16/messages/ExtendedTriggerMessage.hpp>
#include <ocpp/v16/messages/FirmwareStatusNotification.hpp>
#include <ocpp/v16/messages/GetCompositeSchedule.hpp>
#include <ocpp/v16/messages/GetConfiguration.hpp>
#include <ocpp/v16/messages/GetDiagnostics.hpp>
#include <ocpp/v16/messages/GetInstalledCertificateIds.hpp>
#include <ocpp/v16/messages/GetLocalListVersion.hpp>
#include <ocpp/v16/messages/GetLog.hpp>
#include <ocpp/v16/messages/Heartbeat.hpp>
#include <ocpp/v16/messages/InstallCertificate.hpp>
#include <ocpp/v16/messages/LogStatusNotification.hpp>
#include <ocpp/v16/messages/MeterValues.hpp>
#include <ocpp/v16/messages/RemoteStartTransaction.hpp>
#include <ocpp/v16/messages/RemoteStopTransaction.hpp>
#include <ocpp/v16/messages/ReserveNow.hpp>
#include <ocpp/v16/messages/Reset.hpp>
#include <ocpp/v16/messages/SecurityEventNotification.hpp>
#include <ocpp/v16/messages/SendLocalList.hpp>
#include <ocpp/v16/messages/SetChargingProfile.hpp>
#include <ocpp/v16/messages/SignCertificate.hpp>
#include <ocpp/v16/messages/SignedFirmwareStatusNotification.hpp>
#include <ocpp/v16/messages/SignedUpdateFirmware.hpp>
#include <ocpp/v16/messages/StartTransaction.hpp>
#include <ocpp/v16/messages/StatusNotification.hpp>
#include <ocpp/v16/messages/StopTransaction.hpp>
#include <ocpp/v16/messages/TriggerMessage.hpp>
#include <ocpp/v16/messages/UnlockConnector.hpp>
#include <ocpp/v16/messages/UpdateFirmware.hpp>

#define V16_ACTIONS(X) \
    X(Authorize) \
    X(BootNotification) \
    X(CancelReservation) \
    X(CertificateSigned) \
    X(ChangeAvailability) \
    X(ChangeConfiguration) \
    X(ClearCache) \
    X(ClearChargingProfile) \
    X(DataTransfer) \
    X(DeleteCertificate) \
    X(DiagnosticsStatusNotification) \
    X(ExtendedTriggerMessage) \
    X(FirmwareStatusNotification) \
    X(GetCompositeSchedule) \
    X(GetConfiguration) \
    X(GetDiagnostics) \
    X(GetInstalledCertificateIds) \
    X(GetLocalListVersion) \
    X(GetLog) \
    X(Heartbeat) \
    X(InstallCertificate) \
    X(LogStatusNotification) \
    X(MeterValues) \
    X(RemoteStartTransaction) \
    X(RemoteStopTransaction) \
    X(ReserveNow) \
    X(Reset) \
    X(SecurityEventNotification) \
    X(SendLocalList) \
    X(SetChargingProfile) \
    X(SignCertificate) \
    X(SignedFirmwareStatusNotification) \
    X(SignedUpdateFirmware) \
    X(StartTransaction) \
    X(StatusNotification) \
    X(StopTransaction) \
    X(TriggerMessage) \
    X(UnlockConnector) \
    X(UpdateFirmware) \

namespace interop {

const Codec* codec_v16(const std::string& action) {
    static const std::map<std::string, Codec> codecs = {
#define ENTRY(name) {#name, make_codec<ocpp::v16::name##Request, ocpp::v16::name##Response>()},
        V16_ACTIONS(ENTRY)
#undef ENTRY
    };
    const auto it = codecs.find(action);
    return it == codecs.end() ? nullptr : &it->second;
}

} // namespace interop
