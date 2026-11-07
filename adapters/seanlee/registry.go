// SPDX-License-Identifier: MIT
package main

// The OCPP actions of each version and the role that sends each one: OCPP 1.6
// with the Security Whitepaper messages, 2.0.1 Part 2 and 2.1 Edition 2 Part
// 2, the direction tables of scripts/validate-fixtures.py written out here.
// DataTransfer goes both ways. Whether seanlee0923/ocpp knows an action is
// whether it has types for it (actionTable, actions_gen.go).

import "github.com/seanlee0923/ocpp/protocol"

type versionInfo struct {
	wire                  protocol.Version // the subprotocol
	fromStation, fromCSMS map[string]bool
	sendActions           map[string]bool // OCPP 2.1 SEND (message type 6)
}

var versions = map[string]versionInfo{
	"1.6": {
		wire: protocol.OCPP16,
		fromStation: set(
			"Authorize", "BootNotification", "DataTransfer", "DiagnosticsStatusNotification",
			"FirmwareStatusNotification", "Heartbeat", "LogStatusNotification", "MeterValues",
			"SecurityEventNotification", "SignCertificate", "SignedFirmwareStatusNotification", "StartTransaction",
			"StatusNotification", "StopTransaction",
		),
		fromCSMS: set(
			"CancelReservation", "CertificateSigned", "ChangeAvailability", "ChangeConfiguration", "ClearCache",
			"ClearChargingProfile", "DataTransfer", "DeleteCertificate", "ExtendedTriggerMessage",
			"GetCompositeSchedule", "GetConfiguration", "GetDiagnostics", "GetInstalledCertificateIds",
			"GetLocalListVersion", "GetLog", "InstallCertificate", "RemoteStartTransaction", "RemoteStopTransaction",
			"ReserveNow", "Reset", "SendLocalList", "SetChargingProfile", "SignedUpdateFirmware", "TriggerMessage",
			"UnlockConnector", "UpdateFirmware",
		),
		sendActions: set(),
	},
	"2.0.1": {
		wire: protocol.OCPP201,
		fromStation: set(
			"Authorize", "BootNotification", "ClearedChargingLimit", "DataTransfer", "FirmwareStatusNotification",
			"Get15118EVCertificate", "GetCertificateStatus", "Heartbeat", "LogStatusNotification", "MeterValues",
			"NotifyChargingLimit", "NotifyCustomerInformation", "NotifyDisplayMessages", "NotifyEVChargingNeeds",
			"NotifyEVChargingSchedule", "NotifyEvent", "NotifyMonitoringReport", "NotifyReport",
			"PublishFirmwareStatusNotification", "ReportChargingProfiles", "ReservationStatusUpdate",
			"SecurityEventNotification", "SignCertificate", "StatusNotification", "TransactionEvent",
		),
		fromCSMS: set(
			"CancelReservation", "CertificateSigned", "ChangeAvailability", "ClearCache", "ClearChargingProfile",
			"ClearDisplayMessage", "ClearVariableMonitoring", "CostUpdated", "CustomerInformation", "DataTransfer",
			"DeleteCertificate", "GetBaseReport", "GetChargingProfiles", "GetCompositeSchedule", "GetDisplayMessages",
			"GetInstalledCertificateIds", "GetLocalListVersion", "GetLog", "GetMonitoringReport", "GetReport",
			"GetTransactionStatus", "GetVariables", "InstallCertificate", "PublishFirmware", "RequestStartTransaction",
			"RequestStopTransaction", "ReserveNow", "Reset", "SendLocalList", "SetChargingProfile",
			"SetDisplayMessage", "SetMonitoringBase", "SetMonitoringLevel", "SetNetworkProfile",
			"SetVariableMonitoring", "SetVariables", "TriggerMessage", "UnlockConnector", "UnpublishFirmware",
			"UpdateFirmware",
		),
		sendActions: set(),
	},
	"2.1": {
		wire: protocol.OCPP21,
		fromStation: set(
			"Authorize", "BatterySwap", "BootNotification", "ClearedChargingLimit", "ClosePeriodicEventStream",
			"DataTransfer", "FirmwareStatusNotification", "Get15118EVCertificate", "GetCertificateChainStatus",
			"GetCertificateStatus", "Heartbeat", "LogStatusNotification", "MeterValues", "NotifyChargingLimit",
			"NotifyCustomerInformation", "NotifyDERAlarm", "NotifyDERStartStop", "NotifyDisplayMessages",
			"NotifyEVChargingNeeds", "NotifyEVChargingSchedule", "NotifyEvent", "NotifyMonitoringReport",
			"NotifyPeriodicEventStream", "NotifyPriorityCharging", "NotifyReport", "NotifySettlement",
			"OpenPeriodicEventStream", "PublishFirmwareStatusNotification", "PullDynamicScheduleUpdate",
			"ReportChargingProfiles", "ReportDERControl", "ReservationStatusUpdate", "SecurityEventNotification",
			"SignCertificate", "StatusNotification", "TransactionEvent", "VatNumberValidation",
		),
		fromCSMS: set(
			"AFRRSignal", "AdjustPeriodicEventStream", "CancelReservation", "CertificateSigned", "ChangeAvailability",
			"ChangeTransactionTariff", "ClearCache", "ClearChargingProfile", "ClearDERControl", "ClearDisplayMessage",
			"ClearTariffs", "ClearVariableMonitoring", "CostUpdated", "CustomerInformation", "DataTransfer",
			"DeleteCertificate", "GetBaseReport", "GetChargingProfiles", "GetCompositeSchedule", "GetDERControl",
			"GetDisplayMessages", "GetInstalledCertificateIds", "GetLocalListVersion", "GetLog", "GetMonitoringReport",
			"GetPeriodicEventStream", "GetReport", "GetTariffs", "GetTransactionStatus", "GetVariables",
			"InstallCertificate", "NotifyAllowedEnergyTransfer", "NotifyWebPaymentStarted", "PublishFirmware",
			"RequestBatterySwap", "RequestStartTransaction", "RequestStopTransaction", "ReserveNow", "Reset",
			"SendLocalList", "SetChargingProfile", "SetDERControl", "SetDefaultTariff", "SetDisplayMessage",
			"SetMonitoringBase", "SetMonitoringLevel", "SetNetworkProfile", "SetVariableMonitoring", "SetVariables",
			"TriggerMessage", "UnlockConnector", "UnpublishFirmware", "UpdateDynamicSchedule", "UpdateFirmware",
			"UsePriorityCharging",
		),
		sendActions: set("NotifyPeriodicEventStream"),
	},
}

// knows reports whether the action is part of the version and known to
// seanlee0923/ocpp.
func knows(version, action string) bool {
	vi := versions[version]
	_, typed := actionTable[version][action]
	return (vi.fromStation[action] || vi.fromCSMS[action]) && typed
}

// validCode reports whether code is an OCPP-J CALLERROR code of the version:
// the codes seanlee0923/ocpp sends as they are (csms validErrorCode,
// station validCallErrorCode); it sends InternalError for any other.
func validCode(version, code string) bool {
	switch code {
	case "NotImplemented", "NotSupported", "InternalError", "ProtocolError", "SecurityError",
		"PropertyConstraintViolation", "TypeConstraintViolation", "GenericError":
		return true
	}
	if version == "1.6" {
		return code == "FormationViolation" || code == "OccurenceConstraintViolation"
	}
	return code == "FormatViolation" || code == "OccurrenceConstraintViolation" ||
		code == "MessageTypeNotSupported" || code == "RpcFrameworkError"
}
