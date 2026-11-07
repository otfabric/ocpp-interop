// SPDX-License-Identifier: MIT
//! OCPP 2.1 CALLs through ocpp-kit's typed API (Handle::call_with): ocpp-kit
//! decodes the response into its type and, when that fails, answers it with a
//! CALLRESULTERROR (Part 4 4.2.3). One arm per variant of ocpp-kit's
//! v2_1::CsRequest and v2_1::CsmsRequest; the SEND NotifyPeriodicEventStream
//! goes through call_raw. Generated from ocpp-kit 0.3.0; do not edit by hand.

use ocpp_kit::engine::{CallFailure, CallOptions};
use ocpp_kit::transport::Handle;
use ocpp_kit::v2_1::{CsRequest, CsmsRequest};
use serde_json::Value;

fn respond<T: serde::Serialize>(r: Result<T, CallFailure>) -> Result<Value, CallFailure> {
    r.map(|response| serde_json::to_value(&response).unwrap_or(Value::Null))
}

/// A CALL of the station (CS -> CSMS). None for a variant without a typed
/// response (the SEND).
pub async fn call_station(
    handle: &Handle,
    request: CsRequest,
    opts: CallOptions,
) -> Option<Result<Value, CallFailure>> {
    Some(match request {
        CsRequest::Authorize(r) => respond(handle.call_with(r, opts).await),
        CsRequest::BatterySwap(r) => respond(handle.call_with(r, opts).await),
        CsRequest::BootNotification(r) => respond(handle.call_with(r, opts).await),
        CsRequest::ClearedChargingLimit(r) => respond(handle.call_with(r, opts).await),
        CsRequest::ClosePeriodicEventStream(r) => respond(handle.call_with(r, opts).await),
        CsRequest::DataTransfer(r) => respond(handle.call_with(r, opts).await),
        CsRequest::FirmwareStatusNotification(r) => respond(handle.call_with(r, opts).await),
        CsRequest::Get15118EVCertificate(r) => respond(handle.call_with(r, opts).await),
        CsRequest::GetCertificateChainStatus(r) => respond(handle.call_with(r, opts).await),
        CsRequest::GetCertificateStatus(r) => respond(handle.call_with(r, opts).await),
        CsRequest::Heartbeat(r) => respond(handle.call_with(r, opts).await),
        CsRequest::LogStatusNotification(r) => respond(handle.call_with(r, opts).await),
        CsRequest::MeterValues(r) => respond(handle.call_with(r, opts).await),
        CsRequest::NotifyChargingLimit(r) => respond(handle.call_with(r, opts).await),
        CsRequest::NotifyCustomerInformation(r) => respond(handle.call_with(r, opts).await),
        CsRequest::NotifyDERAlarm(r) => respond(handle.call_with(r, opts).await),
        CsRequest::NotifyDERStartStop(r) => respond(handle.call_with(r, opts).await),
        CsRequest::NotifyDisplayMessages(r) => respond(handle.call_with(r, opts).await),
        CsRequest::NotifyEVChargingNeeds(r) => respond(handle.call_with(r, opts).await),
        CsRequest::NotifyEVChargingSchedule(r) => respond(handle.call_with(r, opts).await),
        CsRequest::NotifyEvent(r) => respond(handle.call_with(r, opts).await),
        CsRequest::NotifyMonitoringReport(r) => respond(handle.call_with(r, opts).await),
        CsRequest::NotifyPriorityCharging(r) => respond(handle.call_with(r, opts).await),
        CsRequest::NotifyReport(r) => respond(handle.call_with(r, opts).await),
        CsRequest::NotifySettlement(r) => respond(handle.call_with(r, opts).await),
        CsRequest::NotifyWebPaymentStarted(r) => respond(handle.call_with(r, opts).await),
        CsRequest::OpenPeriodicEventStream(r) => respond(handle.call_with(r, opts).await),
        CsRequest::PublishFirmwareStatusNotification(r) => respond(handle.call_with(r, opts).await),
        CsRequest::PullDynamicScheduleUpdate(r) => respond(handle.call_with(r, opts).await),
        CsRequest::ReportChargingProfiles(r) => respond(handle.call_with(r, opts).await),
        CsRequest::ReportDERControl(r) => respond(handle.call_with(r, opts).await),
        CsRequest::ReservationStatusUpdate(r) => respond(handle.call_with(r, opts).await),
        CsRequest::SecurityEventNotification(r) => respond(handle.call_with(r, opts).await),
        CsRequest::SignCertificate(r) => respond(handle.call_with(r, opts).await),
        CsRequest::StatusNotification(r) => respond(handle.call_with(r, opts).await),
        CsRequest::TransactionEvent(r) => respond(handle.call_with(r, opts).await),
        CsRequest::VatNumberValidation(r) => respond(handle.call_with(r, opts).await),
        _ => return None,
    })
}

/// A CALL of the CSMS (CSMS -> CS).
pub async fn call_csms(
    handle: &Handle,
    request: CsmsRequest,
    opts: CallOptions,
) -> Option<Result<Value, CallFailure>> {
    Some(match request {
        CsmsRequest::AFRRSignal(r) => respond(handle.call_with(r, opts).await),
        CsmsRequest::AdjustPeriodicEventStream(r) => respond(handle.call_with(r, opts).await),
        CsmsRequest::CancelReservation(r) => respond(handle.call_with(r, opts).await),
        CsmsRequest::CertificateSigned(r) => respond(handle.call_with(r, opts).await),
        CsmsRequest::ChangeAvailability(r) => respond(handle.call_with(r, opts).await),
        CsmsRequest::ChangeTransactionTariff(r) => respond(handle.call_with(r, opts).await),
        CsmsRequest::ClearCache(r) => respond(handle.call_with(r, opts).await),
        CsmsRequest::ClearChargingProfile(r) => respond(handle.call_with(r, opts).await),
        CsmsRequest::ClearDERControl(r) => respond(handle.call_with(r, opts).await),
        CsmsRequest::ClearDisplayMessage(r) => respond(handle.call_with(r, opts).await),
        CsmsRequest::ClearTariffs(r) => respond(handle.call_with(r, opts).await),
        CsmsRequest::ClearVariableMonitoring(r) => respond(handle.call_with(r, opts).await),
        CsmsRequest::CostUpdated(r) => respond(handle.call_with(r, opts).await),
        CsmsRequest::CustomerInformation(r) => respond(handle.call_with(r, opts).await),
        CsmsRequest::DataTransfer(r) => respond(handle.call_with(r, opts).await),
        CsmsRequest::DeleteCertificate(r) => respond(handle.call_with(r, opts).await),
        CsmsRequest::GetBaseReport(r) => respond(handle.call_with(r, opts).await),
        CsmsRequest::GetChargingProfiles(r) => respond(handle.call_with(r, opts).await),
        CsmsRequest::GetCompositeSchedule(r) => respond(handle.call_with(r, opts).await),
        CsmsRequest::GetDERControl(r) => respond(handle.call_with(r, opts).await),
        CsmsRequest::GetDisplayMessages(r) => respond(handle.call_with(r, opts).await),
        CsmsRequest::GetInstalledCertificateIds(r) => respond(handle.call_with(r, opts).await),
        CsmsRequest::GetLocalListVersion(r) => respond(handle.call_with(r, opts).await),
        CsmsRequest::GetLog(r) => respond(handle.call_with(r, opts).await),
        CsmsRequest::GetMonitoringReport(r) => respond(handle.call_with(r, opts).await),
        CsmsRequest::GetPeriodicEventStream(r) => respond(handle.call_with(r, opts).await),
        CsmsRequest::GetReport(r) => respond(handle.call_with(r, opts).await),
        CsmsRequest::GetTariffs(r) => respond(handle.call_with(r, opts).await),
        CsmsRequest::GetTransactionStatus(r) => respond(handle.call_with(r, opts).await),
        CsmsRequest::GetVariables(r) => respond(handle.call_with(r, opts).await),
        CsmsRequest::InstallCertificate(r) => respond(handle.call_with(r, opts).await),
        CsmsRequest::NotifyAllowedEnergyTransfer(r) => respond(handle.call_with(r, opts).await),
        CsmsRequest::PublishFirmware(r) => respond(handle.call_with(r, opts).await),
        CsmsRequest::RequestBatterySwap(r) => respond(handle.call_with(r, opts).await),
        CsmsRequest::RequestStartTransaction(r) => respond(handle.call_with(r, opts).await),
        CsmsRequest::RequestStopTransaction(r) => respond(handle.call_with(r, opts).await),
        CsmsRequest::ReserveNow(r) => respond(handle.call_with(r, opts).await),
        CsmsRequest::Reset(r) => respond(handle.call_with(r, opts).await),
        CsmsRequest::SendLocalList(r) => respond(handle.call_with(r, opts).await),
        CsmsRequest::SetChargingProfile(r) => respond(handle.call_with(r, opts).await),
        CsmsRequest::SetDERControl(r) => respond(handle.call_with(r, opts).await),
        CsmsRequest::SetDefaultTariff(r) => respond(handle.call_with(r, opts).await),
        CsmsRequest::SetDisplayMessage(r) => respond(handle.call_with(r, opts).await),
        CsmsRequest::SetMonitoringBase(r) => respond(handle.call_with(r, opts).await),
        CsmsRequest::SetMonitoringLevel(r) => respond(handle.call_with(r, opts).await),
        CsmsRequest::SetNetworkProfile(r) => respond(handle.call_with(r, opts).await),
        CsmsRequest::SetVariableMonitoring(r) => respond(handle.call_with(r, opts).await),
        CsmsRequest::SetVariables(r) => respond(handle.call_with(r, opts).await),
        CsmsRequest::TriggerMessage(r) => respond(handle.call_with(r, opts).await),
        CsmsRequest::UnlockConnector(r) => respond(handle.call_with(r, opts).await),
        CsmsRequest::UnpublishFirmware(r) => respond(handle.call_with(r, opts).await),
        CsmsRequest::UpdateDynamicSchedule(r) => respond(handle.call_with(r, opts).await),
        CsmsRequest::UpdateFirmware(r) => respond(handle.call_with(r, opts).await),
        CsmsRequest::UsePriorityCharging(r) => respond(handle.call_with(r, opts).await),
        _ => return None,
    })
}
