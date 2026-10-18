// SPDX-License-Identifier: MIT
//
// OCPP 2.0.1 and 2.1: libocpp's message types, by action.
//
// The list is every message header of include/ocpp/v2/messages and
// include/ocpp/v21/messages in the pinned
// everest-core release; the image build checks that it still is (a candidate
// release with another set of messages fails the build until the list is
// updated).
//
// OCPP 2.0.1 and 2.1 share the types of namespace ocpp::v2 (libocpp
// generates them from the OCPP 2.1 schemas); 2.1 adds the messages of
// namespace ocpp::v21. NotifyPeriodicEventStream, which OCPP 2.1 sends as a
// SEND (message type 6), is one struct without a response in libocpp, and
// libocpp has no SEND: it is not an action here.
#include "codec.hpp"

#include <ocpp/v2/messages/Authorize.hpp>
#include <ocpp/v2/messages/BootNotification.hpp>
#include <ocpp/v2/messages/CancelReservation.hpp>
#include <ocpp/v2/messages/CertificateSigned.hpp>
#include <ocpp/v2/messages/ChangeAvailability.hpp>
#include <ocpp/v2/messages/ClearCache.hpp>
#include <ocpp/v2/messages/ClearChargingProfile.hpp>
#include <ocpp/v2/messages/ClearDisplayMessage.hpp>
#include <ocpp/v2/messages/ClearVariableMonitoring.hpp>
#include <ocpp/v2/messages/ClearedChargingLimit.hpp>
#include <ocpp/v2/messages/CostUpdated.hpp>
#include <ocpp/v2/messages/CustomerInformation.hpp>
#include <ocpp/v2/messages/DataTransfer.hpp>
#include <ocpp/v2/messages/DeleteCertificate.hpp>
#include <ocpp/v2/messages/FirmwareStatusNotification.hpp>
#include <ocpp/v2/messages/Get15118EVCertificate.hpp>
#include <ocpp/v2/messages/GetBaseReport.hpp>
#include <ocpp/v2/messages/GetCertificateStatus.hpp>
#include <ocpp/v2/messages/GetChargingProfiles.hpp>
#include <ocpp/v2/messages/GetCompositeSchedule.hpp>
#include <ocpp/v2/messages/GetDisplayMessages.hpp>
#include <ocpp/v2/messages/GetInstalledCertificateIds.hpp>
#include <ocpp/v2/messages/GetLocalListVersion.hpp>
#include <ocpp/v2/messages/GetLog.hpp>
#include <ocpp/v2/messages/GetMonitoringReport.hpp>
#include <ocpp/v2/messages/GetReport.hpp>
#include <ocpp/v2/messages/GetTransactionStatus.hpp>
#include <ocpp/v2/messages/GetVariables.hpp>
#include <ocpp/v2/messages/Heartbeat.hpp>
#include <ocpp/v2/messages/InstallCertificate.hpp>
#include <ocpp/v2/messages/LogStatusNotification.hpp>
#include <ocpp/v2/messages/MeterValues.hpp>
#include <ocpp/v2/messages/NotifyChargingLimit.hpp>
#include <ocpp/v2/messages/NotifyCustomerInformation.hpp>
#include <ocpp/v2/messages/NotifyDisplayMessages.hpp>
#include <ocpp/v2/messages/NotifyEVChargingNeeds.hpp>
#include <ocpp/v2/messages/NotifyEVChargingSchedule.hpp>
#include <ocpp/v2/messages/NotifyEvent.hpp>
#include <ocpp/v2/messages/NotifyMonitoringReport.hpp>
#include <ocpp/v2/messages/NotifyReport.hpp>
#include <ocpp/v2/messages/PublishFirmware.hpp>
#include <ocpp/v2/messages/PublishFirmwareStatusNotification.hpp>
#include <ocpp/v2/messages/ReportChargingProfiles.hpp>
#include <ocpp/v2/messages/RequestStartTransaction.hpp>
#include <ocpp/v2/messages/RequestStopTransaction.hpp>
#include <ocpp/v2/messages/ReservationStatusUpdate.hpp>
#include <ocpp/v2/messages/ReserveNow.hpp>
#include <ocpp/v2/messages/Reset.hpp>
#include <ocpp/v2/messages/SecurityEventNotification.hpp>
#include <ocpp/v2/messages/SendLocalList.hpp>
#include <ocpp/v2/messages/SetChargingProfile.hpp>
#include <ocpp/v2/messages/SetDisplayMessage.hpp>
#include <ocpp/v2/messages/SetMonitoringBase.hpp>
#include <ocpp/v2/messages/SetMonitoringLevel.hpp>
#include <ocpp/v2/messages/SetNetworkProfile.hpp>
#include <ocpp/v2/messages/SetVariableMonitoring.hpp>
#include <ocpp/v2/messages/SetVariables.hpp>
#include <ocpp/v2/messages/SignCertificate.hpp>
#include <ocpp/v2/messages/StatusNotification.hpp>
#include <ocpp/v2/messages/TransactionEvent.hpp>
#include <ocpp/v2/messages/TriggerMessage.hpp>
#include <ocpp/v2/messages/UnlockConnector.hpp>
#include <ocpp/v2/messages/UnpublishFirmware.hpp>
#include <ocpp/v2/messages/UpdateFirmware.hpp>
#include <ocpp/v21/messages/AFRRSignal.hpp>
#include <ocpp/v21/messages/AdjustPeriodicEventStream.hpp>
#include <ocpp/v21/messages/BatterySwap.hpp>
#include <ocpp/v21/messages/ChangeTransactionTariff.hpp>
#include <ocpp/v21/messages/ClearDERControl.hpp>
#include <ocpp/v21/messages/ClearTariffs.hpp>
#include <ocpp/v21/messages/ClosePeriodicEventStream.hpp>
#include <ocpp/v21/messages/GetCertificateChainStatus.hpp>
#include <ocpp/v21/messages/GetDERControl.hpp>
#include <ocpp/v21/messages/GetPeriodicEventStream.hpp>
#include <ocpp/v21/messages/GetTariffs.hpp>
#include <ocpp/v21/messages/NotifyAllowedEnergyTransfer.hpp>
#include <ocpp/v21/messages/NotifyDERAlarm.hpp>
#include <ocpp/v21/messages/NotifyDERStartStop.hpp>
#include <ocpp/v21/messages/NotifyPriorityCharging.hpp>
#include <ocpp/v21/messages/NotifySettlement.hpp>
#include <ocpp/v21/messages/NotifyWebPaymentStarted.hpp>
#include <ocpp/v21/messages/OpenPeriodicEventStream.hpp>
#include <ocpp/v21/messages/PullDynamicScheduleUpdate.hpp>
#include <ocpp/v21/messages/ReportDERControl.hpp>
#include <ocpp/v21/messages/RequestBatterySwap.hpp>
#include <ocpp/v21/messages/SetDERControl.hpp>
#include <ocpp/v21/messages/SetDefaultTariff.hpp>
#include <ocpp/v21/messages/UpdateDynamicSchedule.hpp>
#include <ocpp/v21/messages/UsePriorityCharging.hpp>
#include <ocpp/v21/messages/VatNumberValidation.hpp>

#define V2_ACTIONS(X) \
    X(Authorize) \
    X(BootNotification) \
    X(CancelReservation) \
    X(CertificateSigned) \
    X(ChangeAvailability) \
    X(ClearCache) \
    X(ClearChargingProfile) \
    X(ClearDisplayMessage) \
    X(ClearVariableMonitoring) \
    X(ClearedChargingLimit) \
    X(CostUpdated) \
    X(CustomerInformation) \
    X(DataTransfer) \
    X(DeleteCertificate) \
    X(FirmwareStatusNotification) \
    X(Get15118EVCertificate) \
    X(GetBaseReport) \
    X(GetCertificateStatus) \
    X(GetChargingProfiles) \
    X(GetCompositeSchedule) \
    X(GetDisplayMessages) \
    X(GetInstalledCertificateIds) \
    X(GetLocalListVersion) \
    X(GetLog) \
    X(GetMonitoringReport) \
    X(GetReport) \
    X(GetTransactionStatus) \
    X(GetVariables) \
    X(Heartbeat) \
    X(InstallCertificate) \
    X(LogStatusNotification) \
    X(MeterValues) \
    X(NotifyChargingLimit) \
    X(NotifyCustomerInformation) \
    X(NotifyDisplayMessages) \
    X(NotifyEVChargingNeeds) \
    X(NotifyEVChargingSchedule) \
    X(NotifyEvent) \
    X(NotifyMonitoringReport) \
    X(NotifyReport) \
    X(PublishFirmware) \
    X(PublishFirmwareStatusNotification) \
    X(ReportChargingProfiles) \
    X(RequestStartTransaction) \
    X(RequestStopTransaction) \
    X(ReservationStatusUpdate) \
    X(ReserveNow) \
    X(Reset) \
    X(SecurityEventNotification) \
    X(SendLocalList) \
    X(SetChargingProfile) \
    X(SetDisplayMessage) \
    X(SetMonitoringBase) \
    X(SetMonitoringLevel) \
    X(SetNetworkProfile) \
    X(SetVariableMonitoring) \
    X(SetVariables) \
    X(SignCertificate) \
    X(StatusNotification) \
    X(TransactionEvent) \
    X(TriggerMessage) \
    X(UnlockConnector) \
    X(UnpublishFirmware) \
    X(UpdateFirmware) \

#define V21_ACTIONS(X) \
    X(AFRRSignal) \
    X(AdjustPeriodicEventStream) \
    X(BatterySwap) \
    X(ChangeTransactionTariff) \
    X(ClearDERControl) \
    X(ClearTariffs) \
    X(ClosePeriodicEventStream) \
    X(GetCertificateChainStatus) \
    X(GetDERControl) \
    X(GetPeriodicEventStream) \
    X(GetTariffs) \
    X(NotifyAllowedEnergyTransfer) \
    X(NotifyDERAlarm) \
    X(NotifyDERStartStop) \
    X(NotifyPriorityCharging) \
    X(NotifySettlement) \
    X(NotifyWebPaymentStarted) \
    X(OpenPeriodicEventStream) \
    X(PullDynamicScheduleUpdate) \
    X(ReportDERControl) \
    X(RequestBatterySwap) \
    X(SetDERControl) \
    X(SetDefaultTariff) \
    X(UpdateDynamicSchedule) \
    X(UsePriorityCharging) \
    X(VatNumberValidation) \

namespace interop {

const Codec* codec_v2(const std::string& action, bool v21) {
    static const std::map<std::string, Codec> v2_codecs = {
#define ENTRY(name) {#name, make_codec<ocpp::v2::name##Request, ocpp::v2::name##Response>()},
        V2_ACTIONS(ENTRY)
#undef ENTRY
    };
    static const std::map<std::string, Codec> v21_codecs = {
#define ENTRY(name) {#name, make_codec<ocpp::v21::name##Request, ocpp::v21::name##Response>()},
        V21_ACTIONS(ENTRY)
#undef ENTRY
    };
    if (const auto it = v2_codecs.find(action); it != v2_codecs.end()) {
        return &it->second;
    }
    if (v21) {
        if (const auto it = v21_codecs.find(action); it != v21_codecs.end()) {
            return &it->second;
        }
    }
    return nullptr;
}

} // namespace interop
