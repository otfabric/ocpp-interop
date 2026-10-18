// SPDX-License-Identifier: MIT
package main

import (
	"github.com/lorenzodonini/ocpp-go/ocpp"
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
	types16 "github.com/lorenzodonini/ocpp-go/ocpp1.6/types"
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
	types201 "github.com/lorenzodonini/ocpp-go/ocpp2.0.1/types"
)

// versionInfo is what the adapter needs of one OCPP version: the subprotocol,
// ocpp-go's feature profiles (the ones its NewCentralSystem, NewChargePoint,
// NewCSMS and NewChargingStation register by default), and which role sends
// which action, from the specification (as scripts/validate-fixtures.py).
type versionInfo struct {
	subprotocol           string
	profiles              []*ocpp.Profile
	fromStation, fromCSMS map[string]bool
}

var versions = map[string]versionInfo{
	"1.6": {
		subprotocol: types16.V16Subprotocol,
		profiles: []*ocpp.Profile{
			core.Profile, localauth16.Profile, firmware16.Profile, reservation16.Profile, remotetrigger.Profile,
			smartcharging16.Profile, logging16.Profile, security16.Profile, extendedtriggermessage.Profile,
			certificates.Profile, securefirmware.Profile,
		},
		fromStation: set(
			"Authorize", "BootNotification", "DataTransfer", "DiagnosticsStatusNotification",
			"FirmwareStatusNotification", "Heartbeat", "MeterValues", "StartTransaction",
			"StatusNotification", "StopTransaction",
			"LogStatusNotification", "SecurityEventNotification", "SignCertificate",
			"SignedFirmwareStatusNotification"),
		fromCSMS: set(
			"CancelReservation", "ChangeAvailability", "ChangeConfiguration", "ClearCache",
			"ClearChargingProfile", "DataTransfer", "GetCompositeSchedule", "GetConfiguration",
			"GetDiagnostics", "GetLocalListVersion", "RemoteStartTransaction",
			"RemoteStopTransaction", "ReserveNow", "Reset", "SendLocalList", "SetChargingProfile",
			"TriggerMessage", "UnlockConnector", "UpdateFirmware",
			"CertificateSigned", "DeleteCertificate", "ExtendedTriggerMessage",
			"GetInstalledCertificateIds", "GetLog", "InstallCertificate", "SignedUpdateFirmware"),
	},
	"2.0.1": {
		subprotocol: types201.V201Subprotocol,
		profiles: []*ocpp.Profile{
			authorization.Profile, availability.Profile, data.Profile, diagnostics.Profile, display.Profile,
			firmware201.Profile, iso15118.Profile, localauth201.Profile, meter.Profile, provisioning.Profile,
			remotecontrol.Profile, reservation201.Profile, security201.Profile, smartcharging201.Profile,
			tariffcost.Profile, transactions.Profile,
		},
		fromStation: set(
			"Authorize", "BootNotification", "ClearedChargingLimit", "DataTransfer",
			"FirmwareStatusNotification", "Get15118EVCertificate", "GetCertificateStatus",
			"Heartbeat", "LogStatusNotification", "MeterValues", "NotifyChargingLimit",
			"NotifyCustomerInformation", "NotifyDisplayMessages", "NotifyEVChargingNeeds",
			"NotifyEVChargingSchedule", "NotifyEvent", "NotifyMonitoringReport", "NotifyReport",
			"PublishFirmwareStatusNotification", "ReportChargingProfiles",
			"ReservationStatusUpdate", "SecurityEventNotification", "SignCertificate",
			"StatusNotification", "TransactionEvent"),
		fromCSMS: set(
			"CancelReservation", "CertificateSigned", "ChangeAvailability", "ClearCache",
			"ClearChargingProfile", "ClearDisplayMessage", "ClearVariableMonitoring",
			"CostUpdated", "CustomerInformation", "DataTransfer", "DeleteCertificate",
			"GetBaseReport", "GetChargingProfiles", "GetCompositeSchedule", "GetDisplayMessages",
			"GetInstalledCertificateIds", "GetLocalListVersion", "GetLog", "GetMonitoringReport",
			"GetReport", "GetTransactionStatus", "GetVariables", "InstallCertificate",
			"PublishFirmware", "RequestStartTransaction", "RequestStopTransaction", "ReserveNow",
			"Reset", "SendLocalList", "SetChargingProfile", "SetDisplayMessage",
			"SetMonitoringBase", "SetMonitoringLevel", "SetNetworkProfile",
			"SetVariableMonitoring", "SetVariables", "TriggerMessage", "UnlockConnector",
			"UnpublishFirmware", "UpdateFirmware"),
	},
}

// feature is ocpp-go's feature (request and response types) of an action in
// a version, or nil when ocpp-go has none.
func feature(version, action string) ocpp.Feature {
	for _, p := range versions[version].profiles {
		if f := p.GetFeature(action); f != nil {
			return f
		}
	}
	return nil
}
