// SPDX-License-Identifier: MIT
package main

// The OCPP actions of each version and the role that sends each one: OCPP 1.6
// with the Security Whitepaper messages, 2.0.1 Part 2 and 2.1 Edition 2 Part
// 2, the direction tables of scripts/validate-fixtures.py written out here
// (cB-Sigma/ocpp keeps its own tables unexported). DataTransfer goes both
// ways. Whether cB-Sigma knows an action is ocpp.Known.

import "github.com/cB-Sigma/ocpp"

type versionInfo struct {
	wire                  string // cB-Sigma's version name, the subprotocol
	fromStation, fromCSMS map[string]bool
	sendActions           map[string]bool // OCPP 2.1 SEND (message type 6)
}

var versions = map[string]versionInfo{
	"1.6": {
		wire: ocpp.V16,
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
		wire: ocpp.V201,
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
		wire: ocpp.V21,
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
// cB-Sigma.
func knows(version, action string) bool {
	vi := versions[version]
	return (vi.fromStation[action] || vi.fromCSMS[action]) && ocpp.Known(vi.wire, action)
}

// errKinds are cB-Sigma's version-neutral CALLERROR kinds; ErrKind.Code
// names each in a version.
var errKinds = []ocpp.ErrKind{ocpp.KindFormat, ocpp.KindProperty, ocpp.KindOccurrence, ocpp.KindType,
	ocpp.KindNotImplemented, ocpp.KindNotSupported, ocpp.KindInternal, ocpp.KindProtocol, ocpp.KindSecurity,
	ocpp.KindGeneric, ocpp.KindRPCFramework, ocpp.KindMessageType}

// errKindOf is the kind whose CALLERROR code in the version is code: the
// codes cB-Sigma can answer with.
func errKindOf(version, code string) (ocpp.ErrKind, bool) {
	for _, k := range errKinds {
		if k.Code(versions[version].wire) == code {
			return k, true
		}
	}
	return 0, false
}
