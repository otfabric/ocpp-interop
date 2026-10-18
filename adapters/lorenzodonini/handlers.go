// SPDX-License-Identifier: MIT
package main

// The handlers ocpp-go's high-level API calls for each received CALL, one
// method per action of each feature profile (generated from ocpp-go's
// *Handler interfaces). Every method answers from the fixture through
// endpoint.answer; ocpp-go validates and sends the answer, or turns the
// returned error into a CALLERROR.

import (
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/certificates"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/core"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/extendedtriggermessage"
	firmware16 "github.com/lorenzodonini/ocpp-go/ocpp1.6/firmware"
	localauth16 "github.com/lorenzodonini/ocpp-go/ocpp1.6/localauth"
	logging16 "github.com/lorenzodonini/ocpp-go/ocpp1.6/logging"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/remotetrigger"
	reservation16 "github.com/lorenzodonini/ocpp-go/ocpp1.6/reservation"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/securefirmware"
	security16 "github.com/lorenzodonini/ocpp-go/ocpp1.6/security"
	smartcharging16 "github.com/lorenzodonini/ocpp-go/ocpp1.6/smartcharging"
	"github.com/lorenzodonini/ocpp-go/ocpp2.0.1/authorization"
	"github.com/lorenzodonini/ocpp-go/ocpp2.0.1/availability"
	"github.com/lorenzodonini/ocpp-go/ocpp2.0.1/data"
	"github.com/lorenzodonini/ocpp-go/ocpp2.0.1/diagnostics"
	"github.com/lorenzodonini/ocpp-go/ocpp2.0.1/display"
	firmware201 "github.com/lorenzodonini/ocpp-go/ocpp2.0.1/firmware"
	"github.com/lorenzodonini/ocpp-go/ocpp2.0.1/iso15118"
	localauth201 "github.com/lorenzodonini/ocpp-go/ocpp2.0.1/localauth"
	"github.com/lorenzodonini/ocpp-go/ocpp2.0.1/meter"
	"github.com/lorenzodonini/ocpp-go/ocpp2.0.1/provisioning"
	"github.com/lorenzodonini/ocpp-go/ocpp2.0.1/remotecontrol"
	reservation201 "github.com/lorenzodonini/ocpp-go/ocpp2.0.1/reservation"
	security201 "github.com/lorenzodonini/ocpp-go/ocpp2.0.1/security"
	smartcharging201 "github.com/lorenzodonini/ocpp-go/ocpp2.0.1/smartcharging"
	"github.com/lorenzodonini/ocpp-go/ocpp2.0.1/tariffcost"
	"github.com/lorenzodonini/ocpp-go/ocpp2.0.1/transactions"
)

// centralSystem16: the OCPP 1.6 CSMS (ocpp16.CentralSystem).
type centralSystem16 struct{ e *endpoint }

func (h centralSystem16) OnAuthorize(_ string, r *core.AuthorizeRequest) (*core.AuthorizeConfirmation, error) {
	return reply[core.AuthorizeConfirmation](h.e, r)
}

func (h centralSystem16) OnBootNotification(_ string, r *core.BootNotificationRequest) (*core.BootNotificationConfirmation, error) {
	return reply[core.BootNotificationConfirmation](h.e, r)
}

func (h centralSystem16) OnDataTransfer(_ string, r *core.DataTransferRequest) (*core.DataTransferConfirmation, error) {
	return reply[core.DataTransferConfirmation](h.e, r)
}

func (h centralSystem16) OnHeartbeat(_ string, r *core.HeartbeatRequest) (*core.HeartbeatConfirmation, error) {
	return reply[core.HeartbeatConfirmation](h.e, r)
}

func (h centralSystem16) OnMeterValues(_ string, r *core.MeterValuesRequest) (*core.MeterValuesConfirmation, error) {
	return reply[core.MeterValuesConfirmation](h.e, r)
}

func (h centralSystem16) OnStatusNotification(_ string, r *core.StatusNotificationRequest) (*core.StatusNotificationConfirmation, error) {
	return reply[core.StatusNotificationConfirmation](h.e, r)
}

func (h centralSystem16) OnStartTransaction(_ string, r *core.StartTransactionRequest) (*core.StartTransactionConfirmation, error) {
	return reply[core.StartTransactionConfirmation](h.e, r)
}

func (h centralSystem16) OnStopTransaction(_ string, r *core.StopTransactionRequest) (*core.StopTransactionConfirmation, error) {
	return reply[core.StopTransactionConfirmation](h.e, r)
}

func (h centralSystem16) OnDiagnosticsStatusNotification(_ string, r *firmware16.DiagnosticsStatusNotificationRequest) (*firmware16.DiagnosticsStatusNotificationConfirmation, error) {
	return reply[firmware16.DiagnosticsStatusNotificationConfirmation](h.e, r)
}

func (h centralSystem16) OnFirmwareStatusNotification(_ string, r *firmware16.FirmwareStatusNotificationRequest) (*firmware16.FirmwareStatusNotificationConfirmation, error) {
	return reply[firmware16.FirmwareStatusNotificationConfirmation](h.e, r)
}

func (h centralSystem16) OnLogStatusNotification(_ string, r *logging16.LogStatusNotificationRequest) (*logging16.LogStatusNotificationResponse, error) {
	return reply[logging16.LogStatusNotificationResponse](h.e, r)
}

func (h centralSystem16) OnSecurityEventNotification(_ string, r *security16.SecurityEventNotificationRequest) (*security16.SecurityEventNotificationResponse, error) {
	return reply[security16.SecurityEventNotificationResponse](h.e, r)
}

func (h centralSystem16) OnSignCertificate(_ string, r *security16.SignCertificateRequest) (*security16.SignCertificateResponse, error) {
	return reply[security16.SignCertificateResponse](h.e, r)
}

func (h centralSystem16) OnSignedFirmwareStatusNotification(_ string, r *securefirmware.SignedFirmwareStatusNotificationRequest) (*securefirmware.SignedFirmwareStatusNotificationResponse, error) {
	return reply[securefirmware.SignedFirmwareStatusNotificationResponse](h.e, r)
}

var (
	_ core.CentralSystemHandler            = centralSystem16{}
	_ firmware16.CentralSystemHandler      = centralSystem16{}
	_ localauth16.CentralSystemHandler     = centralSystem16{}
	_ logging16.CentralSystemHandler       = centralSystem16{}
	_ reservation16.CentralSystemHandler   = centralSystem16{}
	_ remotetrigger.CentralSystemHandler   = centralSystem16{}
	_ smartcharging16.CentralSystemHandler = centralSystem16{}
	_ security16.CentralSystemHandler      = centralSystem16{}
	_ securefirmware.CentralSystemHandler  = centralSystem16{}
)

// chargePoint16: the OCPP 1.6 station (ocpp16.ChargePoint).
type chargePoint16 struct{ e *endpoint }

func (h chargePoint16) OnChangeAvailability(r *core.ChangeAvailabilityRequest) (*core.ChangeAvailabilityConfirmation, error) {
	return reply[core.ChangeAvailabilityConfirmation](h.e, r)
}

func (h chargePoint16) OnChangeConfiguration(r *core.ChangeConfigurationRequest) (*core.ChangeConfigurationConfirmation, error) {
	return reply[core.ChangeConfigurationConfirmation](h.e, r)
}

func (h chargePoint16) OnClearCache(r *core.ClearCacheRequest) (*core.ClearCacheConfirmation, error) {
	return reply[core.ClearCacheConfirmation](h.e, r)
}

func (h chargePoint16) OnDataTransfer(r *core.DataTransferRequest) (*core.DataTransferConfirmation, error) {
	return reply[core.DataTransferConfirmation](h.e, r)
}

func (h chargePoint16) OnGetConfiguration(r *core.GetConfigurationRequest) (*core.GetConfigurationConfirmation, error) {
	return reply[core.GetConfigurationConfirmation](h.e, r)
}

func (h chargePoint16) OnRemoteStartTransaction(r *core.RemoteStartTransactionRequest) (*core.RemoteStartTransactionConfirmation, error) {
	return reply[core.RemoteStartTransactionConfirmation](h.e, r)
}

func (h chargePoint16) OnRemoteStopTransaction(r *core.RemoteStopTransactionRequest) (*core.RemoteStopTransactionConfirmation, error) {
	return reply[core.RemoteStopTransactionConfirmation](h.e, r)
}

func (h chargePoint16) OnReset(r *core.ResetRequest) (*core.ResetConfirmation, error) {
	return reply[core.ResetConfirmation](h.e, r)
}

func (h chargePoint16) OnUnlockConnector(r *core.UnlockConnectorRequest) (*core.UnlockConnectorConfirmation, error) {
	return reply[core.UnlockConnectorConfirmation](h.e, r)
}

func (h chargePoint16) OnGetDiagnostics(r *firmware16.GetDiagnosticsRequest) (*firmware16.GetDiagnosticsConfirmation, error) {
	return reply[firmware16.GetDiagnosticsConfirmation](h.e, r)
}

func (h chargePoint16) OnUpdateFirmware(r *firmware16.UpdateFirmwareRequest) (*firmware16.UpdateFirmwareConfirmation, error) {
	return reply[firmware16.UpdateFirmwareConfirmation](h.e, r)
}

func (h chargePoint16) OnGetLocalListVersion(r *localauth16.GetLocalListVersionRequest) (*localauth16.GetLocalListVersionConfirmation, error) {
	return reply[localauth16.GetLocalListVersionConfirmation](h.e, r)
}

func (h chargePoint16) OnSendLocalList(r *localauth16.SendLocalListRequest) (*localauth16.SendLocalListConfirmation, error) {
	return reply[localauth16.SendLocalListConfirmation](h.e, r)
}

func (h chargePoint16) OnGetLog(r *logging16.GetLogRequest) (*logging16.GetLogResponse, error) {
	return reply[logging16.GetLogResponse](h.e, r)
}

func (h chargePoint16) OnReserveNow(r *reservation16.ReserveNowRequest) (*reservation16.ReserveNowConfirmation, error) {
	return reply[reservation16.ReserveNowConfirmation](h.e, r)
}

func (h chargePoint16) OnCancelReservation(r *reservation16.CancelReservationRequest) (*reservation16.CancelReservationConfirmation, error) {
	return reply[reservation16.CancelReservationConfirmation](h.e, r)
}

func (h chargePoint16) OnTriggerMessage(r *remotetrigger.TriggerMessageRequest) (*remotetrigger.TriggerMessageConfirmation, error) {
	return reply[remotetrigger.TriggerMessageConfirmation](h.e, r)
}

func (h chargePoint16) OnSetChargingProfile(r *smartcharging16.SetChargingProfileRequest) (*smartcharging16.SetChargingProfileConfirmation, error) {
	return reply[smartcharging16.SetChargingProfileConfirmation](h.e, r)
}

func (h chargePoint16) OnClearChargingProfile(r *smartcharging16.ClearChargingProfileRequest) (*smartcharging16.ClearChargingProfileConfirmation, error) {
	return reply[smartcharging16.ClearChargingProfileConfirmation](h.e, r)
}

func (h chargePoint16) OnGetCompositeSchedule(r *smartcharging16.GetCompositeScheduleRequest) (*smartcharging16.GetCompositeScheduleConfirmation, error) {
	return reply[smartcharging16.GetCompositeScheduleConfirmation](h.e, r)
}

func (h chargePoint16) OnCertificateSigned(r *security16.CertificateSignedRequest) (*security16.CertificateSignedResponse, error) {
	return reply[security16.CertificateSignedResponse](h.e, r)
}

func (h chargePoint16) OnSignedUpdateFirmware(r *securefirmware.SignedUpdateFirmwareRequest) (*securefirmware.SignedUpdateFirmwareResponse, error) {
	return reply[securefirmware.SignedUpdateFirmwareResponse](h.e, r)
}

func (h chargePoint16) OnExtendedTriggerMessage(r *extendedtriggermessage.ExtendedTriggerMessageRequest) (*extendedtriggermessage.ExtendedTriggerMessageResponse, error) {
	return reply[extendedtriggermessage.ExtendedTriggerMessageResponse](h.e, r)
}

func (h chargePoint16) OnDeleteCertificate(r *certificates.DeleteCertificateRequest) (*certificates.DeleteCertificateResponse, error) {
	return reply[certificates.DeleteCertificateResponse](h.e, r)
}

func (h chargePoint16) OnGetInstalledCertificateIds(r *certificates.GetInstalledCertificateIdsRequest) (*certificates.GetInstalledCertificateIdsResponse, error) {
	return reply[certificates.GetInstalledCertificateIdsResponse](h.e, r)
}

func (h chargePoint16) OnInstallCertificate(r *certificates.InstallCertificateRequest) (*certificates.InstallCertificateResponse, error) {
	return reply[certificates.InstallCertificateResponse](h.e, r)
}

var (
	_ core.ChargePointHandler                   = chargePoint16{}
	_ firmware16.ChargePointHandler             = chargePoint16{}
	_ localauth16.ChargePointHandler            = chargePoint16{}
	_ logging16.ChargePointHandler              = chargePoint16{}
	_ reservation16.ChargePointHandler          = chargePoint16{}
	_ remotetrigger.ChargePointHandler          = chargePoint16{}
	_ smartcharging16.ChargePointHandler        = chargePoint16{}
	_ security16.ChargePointHandler             = chargePoint16{}
	_ securefirmware.ChargePointHandler         = chargePoint16{}
	_ extendedtriggermessage.ChargePointHandler = chargePoint16{}
	_ certificates.ChargePointHandler           = chargePoint16{}
)

// csms201: the OCPP 2.0.1 CSMS (ocpp2.CSMS).
type csms201 struct{ e *endpoint }

func (h csms201) OnAuthorize(_ string, r *authorization.AuthorizeRequest) (*authorization.AuthorizeResponse, error) {
	return reply[authorization.AuthorizeResponse](h.e, r)
}

func (h csms201) OnHeartbeat(_ string, r *availability.HeartbeatRequest) (*availability.HeartbeatResponse, error) {
	return reply[availability.HeartbeatResponse](h.e, r)
}

func (h csms201) OnStatusNotification(_ string, r *availability.StatusNotificationRequest) (*availability.StatusNotificationResponse, error) {
	return reply[availability.StatusNotificationResponse](h.e, r)
}

func (h csms201) OnDataTransfer(_ string, r *data.DataTransferRequest) (*data.DataTransferResponse, error) {
	return reply[data.DataTransferResponse](h.e, r)
}

func (h csms201) OnLogStatusNotification(_ string, r *diagnostics.LogStatusNotificationRequest) (*diagnostics.LogStatusNotificationResponse, error) {
	return reply[diagnostics.LogStatusNotificationResponse](h.e, r)
}

func (h csms201) OnNotifyCustomerInformation(_ string, r *diagnostics.NotifyCustomerInformationRequest) (*diagnostics.NotifyCustomerInformationResponse, error) {
	return reply[diagnostics.NotifyCustomerInformationResponse](h.e, r)
}

func (h csms201) OnNotifyEvent(_ string, r *diagnostics.NotifyEventRequest) (*diagnostics.NotifyEventResponse, error) {
	return reply[diagnostics.NotifyEventResponse](h.e, r)
}

func (h csms201) OnNotifyMonitoringReport(_ string, r *diagnostics.NotifyMonitoringReportRequest) (*diagnostics.NotifyMonitoringReportResponse, error) {
	return reply[diagnostics.NotifyMonitoringReportResponse](h.e, r)
}

func (h csms201) OnNotifyDisplayMessages(_ string, r *display.NotifyDisplayMessagesRequest) (*display.NotifyDisplayMessagesResponse, error) {
	return reply[display.NotifyDisplayMessagesResponse](h.e, r)
}

func (h csms201) OnFirmwareStatusNotification(_ string, r *firmware201.FirmwareStatusNotificationRequest) (*firmware201.FirmwareStatusNotificationResponse, error) {
	return reply[firmware201.FirmwareStatusNotificationResponse](h.e, r)
}

func (h csms201) OnPublishFirmwareStatusNotification(_ string, r *firmware201.PublishFirmwareStatusNotificationRequest) (*firmware201.PublishFirmwareStatusNotificationResponse, error) {
	return reply[firmware201.PublishFirmwareStatusNotificationResponse](h.e, r)
}

func (h csms201) OnGet15118EVCertificate(_ string, r *iso15118.Get15118EVCertificateRequest) (*iso15118.Get15118EVCertificateResponse, error) {
	return reply[iso15118.Get15118EVCertificateResponse](h.e, r)
}

func (h csms201) OnGetCertificateStatus(_ string, r *iso15118.GetCertificateStatusRequest) (*iso15118.GetCertificateStatusResponse, error) {
	return reply[iso15118.GetCertificateStatusResponse](h.e, r)
}

func (h csms201) OnMeterValues(_ string, r *meter.MeterValuesRequest) (*meter.MeterValuesResponse, error) {
	return reply[meter.MeterValuesResponse](h.e, r)
}

func (h csms201) OnBootNotification(_ string, r *provisioning.BootNotificationRequest) (*provisioning.BootNotificationResponse, error) {
	return reply[provisioning.BootNotificationResponse](h.e, r)
}

func (h csms201) OnNotifyReport(_ string, r *provisioning.NotifyReportRequest) (*provisioning.NotifyReportResponse, error) {
	return reply[provisioning.NotifyReportResponse](h.e, r)
}

func (h csms201) OnReservationStatusUpdate(_ string, r *reservation201.ReservationStatusUpdateRequest) (*reservation201.ReservationStatusUpdateResponse, error) {
	return reply[reservation201.ReservationStatusUpdateResponse](h.e, r)
}

func (h csms201) OnSecurityEventNotification(_ string, r *security201.SecurityEventNotificationRequest) (*security201.SecurityEventNotificationResponse, error) {
	return reply[security201.SecurityEventNotificationResponse](h.e, r)
}

func (h csms201) OnSignCertificate(_ string, r *security201.SignCertificateRequest) (*security201.SignCertificateResponse, error) {
	return reply[security201.SignCertificateResponse](h.e, r)
}

func (h csms201) OnClearedChargingLimit(_ string, r *smartcharging201.ClearedChargingLimitRequest) (*smartcharging201.ClearedChargingLimitResponse, error) {
	return reply[smartcharging201.ClearedChargingLimitResponse](h.e, r)
}

func (h csms201) OnNotifyChargingLimit(_ string, r *smartcharging201.NotifyChargingLimitRequest) (*smartcharging201.NotifyChargingLimitResponse, error) {
	return reply[smartcharging201.NotifyChargingLimitResponse](h.e, r)
}

func (h csms201) OnNotifyEVChargingNeeds(_ string, r *smartcharging201.NotifyEVChargingNeedsRequest) (*smartcharging201.NotifyEVChargingNeedsResponse, error) {
	return reply[smartcharging201.NotifyEVChargingNeedsResponse](h.e, r)
}

func (h csms201) OnNotifyEVChargingSchedule(_ string, r *smartcharging201.NotifyEVChargingScheduleRequest) (*smartcharging201.NotifyEVChargingScheduleResponse, error) {
	return reply[smartcharging201.NotifyEVChargingScheduleResponse](h.e, r)
}

func (h csms201) OnReportChargingProfiles(_ string, r *smartcharging201.ReportChargingProfilesRequest) (*smartcharging201.ReportChargingProfilesResponse, error) {
	return reply[smartcharging201.ReportChargingProfilesResponse](h.e, r)
}

func (h csms201) OnTransactionEvent(_ string, r *transactions.TransactionEventRequest) (*transactions.TransactionEventResponse, error) {
	return reply[transactions.TransactionEventResponse](h.e, r)
}

var (
	_ authorization.CSMSHandler    = csms201{}
	_ availability.CSMSHandler     = csms201{}
	_ data.CSMSHandler             = csms201{}
	_ diagnostics.CSMSHandler      = csms201{}
	_ display.CSMSHandler          = csms201{}
	_ firmware201.CSMSHandler      = csms201{}
	_ iso15118.CSMSHandler         = csms201{}
	_ localauth201.CSMSHandler     = csms201{}
	_ meter.CSMSHandler            = csms201{}
	_ provisioning.CSMSHandler     = csms201{}
	_ remotecontrol.CSMSHandler    = csms201{}
	_ reservation201.CSMSHandler   = csms201{}
	_ security201.CSMSHandler      = csms201{}
	_ smartcharging201.CSMSHandler = csms201{}
	_ tariffcost.CSMSHandler       = csms201{}
	_ transactions.CSMSHandler     = csms201{}
)

// chargingStation201: the OCPP 2.0.1 station (ocpp2.ChargingStation).
type chargingStation201 struct{ e *endpoint }

func (h chargingStation201) OnClearCache(r *authorization.ClearCacheRequest) (*authorization.ClearCacheResponse, error) {
	return reply[authorization.ClearCacheResponse](h.e, r)
}

func (h chargingStation201) OnChangeAvailability(r *availability.ChangeAvailabilityRequest) (*availability.ChangeAvailabilityResponse, error) {
	return reply[availability.ChangeAvailabilityResponse](h.e, r)
}

func (h chargingStation201) OnDataTransfer(r *data.DataTransferRequest) (*data.DataTransferResponse, error) {
	return reply[data.DataTransferResponse](h.e, r)
}

func (h chargingStation201) OnClearVariableMonitoring(r *diagnostics.ClearVariableMonitoringRequest) (*diagnostics.ClearVariableMonitoringResponse, error) {
	return reply[diagnostics.ClearVariableMonitoringResponse](h.e, r)
}

func (h chargingStation201) OnCustomerInformation(r *diagnostics.CustomerInformationRequest) (*diagnostics.CustomerInformationResponse, error) {
	return reply[diagnostics.CustomerInformationResponse](h.e, r)
}

func (h chargingStation201) OnGetLog(r *diagnostics.GetLogRequest) (*diagnostics.GetLogResponse, error) {
	return reply[diagnostics.GetLogResponse](h.e, r)
}

func (h chargingStation201) OnGetMonitoringReport(r *diagnostics.GetMonitoringReportRequest) (*diagnostics.GetMonitoringReportResponse, error) {
	return reply[diagnostics.GetMonitoringReportResponse](h.e, r)
}

func (h chargingStation201) OnSetMonitoringBase(r *diagnostics.SetMonitoringBaseRequest) (*diagnostics.SetMonitoringBaseResponse, error) {
	return reply[diagnostics.SetMonitoringBaseResponse](h.e, r)
}

func (h chargingStation201) OnSetMonitoringLevel(r *diagnostics.SetMonitoringLevelRequest) (*diagnostics.SetMonitoringLevelResponse, error) {
	return reply[diagnostics.SetMonitoringLevelResponse](h.e, r)
}

func (h chargingStation201) OnSetVariableMonitoring(r *diagnostics.SetVariableMonitoringRequest) (*diagnostics.SetVariableMonitoringResponse, error) {
	return reply[diagnostics.SetVariableMonitoringResponse](h.e, r)
}

func (h chargingStation201) OnClearDisplay(r *display.ClearDisplayRequest) (*display.ClearDisplayResponse, error) {
	return reply[display.ClearDisplayResponse](h.e, r)
}

func (h chargingStation201) OnGetDisplayMessages(r *display.GetDisplayMessagesRequest) (*display.GetDisplayMessagesResponse, error) {
	return reply[display.GetDisplayMessagesResponse](h.e, r)
}

func (h chargingStation201) OnSetDisplayMessage(r *display.SetDisplayMessageRequest) (*display.SetDisplayMessageResponse, error) {
	return reply[display.SetDisplayMessageResponse](h.e, r)
}

func (h chargingStation201) OnPublishFirmware(r *firmware201.PublishFirmwareRequest) (*firmware201.PublishFirmwareResponse, error) {
	return reply[firmware201.PublishFirmwareResponse](h.e, r)
}

func (h chargingStation201) OnUnpublishFirmware(r *firmware201.UnpublishFirmwareRequest) (*firmware201.UnpublishFirmwareResponse, error) {
	return reply[firmware201.UnpublishFirmwareResponse](h.e, r)
}

func (h chargingStation201) OnUpdateFirmware(r *firmware201.UpdateFirmwareRequest) (*firmware201.UpdateFirmwareResponse, error) {
	return reply[firmware201.UpdateFirmwareResponse](h.e, r)
}

func (h chargingStation201) OnDeleteCertificate(r *iso15118.DeleteCertificateRequest) (*iso15118.DeleteCertificateResponse, error) {
	return reply[iso15118.DeleteCertificateResponse](h.e, r)
}

func (h chargingStation201) OnGetInstalledCertificateIds(r *iso15118.GetInstalledCertificateIdsRequest) (*iso15118.GetInstalledCertificateIdsResponse, error) {
	return reply[iso15118.GetInstalledCertificateIdsResponse](h.e, r)
}

func (h chargingStation201) OnInstallCertificate(r *iso15118.InstallCertificateRequest) (*iso15118.InstallCertificateResponse, error) {
	return reply[iso15118.InstallCertificateResponse](h.e, r)
}

func (h chargingStation201) OnGetLocalListVersion(r *localauth201.GetLocalListVersionRequest) (*localauth201.GetLocalListVersionResponse, error) {
	return reply[localauth201.GetLocalListVersionResponse](h.e, r)
}

func (h chargingStation201) OnSendLocalList(r *localauth201.SendLocalListRequest) (*localauth201.SendLocalListResponse, error) {
	return reply[localauth201.SendLocalListResponse](h.e, r)
}

func (h chargingStation201) OnGetBaseReport(r *provisioning.GetBaseReportRequest) (*provisioning.GetBaseReportResponse, error) {
	return reply[provisioning.GetBaseReportResponse](h.e, r)
}

func (h chargingStation201) OnGetReport(r *provisioning.GetReportRequest) (*provisioning.GetReportResponse, error) {
	return reply[provisioning.GetReportResponse](h.e, r)
}

func (h chargingStation201) OnGetVariables(r *provisioning.GetVariablesRequest) (*provisioning.GetVariablesResponse, error) {
	return reply[provisioning.GetVariablesResponse](h.e, r)
}

func (h chargingStation201) OnReset(r *provisioning.ResetRequest) (*provisioning.ResetResponse, error) {
	return reply[provisioning.ResetResponse](h.e, r)
}

func (h chargingStation201) OnSetNetworkProfile(r *provisioning.SetNetworkProfileRequest) (*provisioning.SetNetworkProfileResponse, error) {
	return reply[provisioning.SetNetworkProfileResponse](h.e, r)
}

func (h chargingStation201) OnSetVariables(r *provisioning.SetVariablesRequest) (*provisioning.SetVariablesResponse, error) {
	return reply[provisioning.SetVariablesResponse](h.e, r)
}

func (h chargingStation201) OnRequestStartTransaction(r *remotecontrol.RequestStartTransactionRequest) (*remotecontrol.RequestStartTransactionResponse, error) {
	return reply[remotecontrol.RequestStartTransactionResponse](h.e, r)
}

func (h chargingStation201) OnRequestStopTransaction(r *remotecontrol.RequestStopTransactionRequest) (*remotecontrol.RequestStopTransactionResponse, error) {
	return reply[remotecontrol.RequestStopTransactionResponse](h.e, r)
}

func (h chargingStation201) OnTriggerMessage(r *remotecontrol.TriggerMessageRequest) (*remotecontrol.TriggerMessageResponse, error) {
	return reply[remotecontrol.TriggerMessageResponse](h.e, r)
}

func (h chargingStation201) OnUnlockConnector(r *remotecontrol.UnlockConnectorRequest) (*remotecontrol.UnlockConnectorResponse, error) {
	return reply[remotecontrol.UnlockConnectorResponse](h.e, r)
}

func (h chargingStation201) OnCancelReservation(r *reservation201.CancelReservationRequest) (*reservation201.CancelReservationResponse, error) {
	return reply[reservation201.CancelReservationResponse](h.e, r)
}

func (h chargingStation201) OnReserveNow(r *reservation201.ReserveNowRequest) (*reservation201.ReserveNowResponse, error) {
	return reply[reservation201.ReserveNowResponse](h.e, r)
}

func (h chargingStation201) OnCertificateSigned(r *security201.CertificateSignedRequest) (*security201.CertificateSignedResponse, error) {
	return reply[security201.CertificateSignedResponse](h.e, r)
}

func (h chargingStation201) OnClearChargingProfile(r *smartcharging201.ClearChargingProfileRequest) (*smartcharging201.ClearChargingProfileResponse, error) {
	return reply[smartcharging201.ClearChargingProfileResponse](h.e, r)
}

func (h chargingStation201) OnGetChargingProfiles(r *smartcharging201.GetChargingProfilesRequest) (*smartcharging201.GetChargingProfilesResponse, error) {
	return reply[smartcharging201.GetChargingProfilesResponse](h.e, r)
}

func (h chargingStation201) OnGetCompositeSchedule(r *smartcharging201.GetCompositeScheduleRequest) (*smartcharging201.GetCompositeScheduleResponse, error) {
	return reply[smartcharging201.GetCompositeScheduleResponse](h.e, r)
}

func (h chargingStation201) OnSetChargingProfile(r *smartcharging201.SetChargingProfileRequest) (*smartcharging201.SetChargingProfileResponse, error) {
	return reply[smartcharging201.SetChargingProfileResponse](h.e, r)
}

func (h chargingStation201) OnCostUpdated(r *tariffcost.CostUpdatedRequest) (*tariffcost.CostUpdatedResponse, error) {
	return reply[tariffcost.CostUpdatedResponse](h.e, r)
}

func (h chargingStation201) OnGetTransactionStatus(r *transactions.GetTransactionStatusRequest) (*transactions.GetTransactionStatusResponse, error) {
	return reply[transactions.GetTransactionStatusResponse](h.e, r)
}

var (
	_ authorization.ChargingStationHandler    = chargingStation201{}
	_ availability.ChargingStationHandler     = chargingStation201{}
	_ data.ChargingStationHandler             = chargingStation201{}
	_ diagnostics.ChargingStationHandler      = chargingStation201{}
	_ display.ChargingStationHandler          = chargingStation201{}
	_ firmware201.ChargingStationHandler      = chargingStation201{}
	_ iso15118.ChargingStationHandler         = chargingStation201{}
	_ localauth201.ChargingStationHandler     = chargingStation201{}
	_ meter.ChargingStationHandler            = chargingStation201{}
	_ provisioning.ChargingStationHandler     = chargingStation201{}
	_ remotecontrol.ChargingStationHandler    = chargingStation201{}
	_ reservation201.ChargingStationHandler   = chargingStation201{}
	_ security201.ChargingStationHandler      = chargingStation201{}
	_ smartcharging201.ChargingStationHandler = chargingStation201{}
	_ tariffcost.ChargingStationHandler       = chargingStation201{}
	_ transactions.ChargingStationHandler     = chargingStation201{}
)
