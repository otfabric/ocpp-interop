// SPDX-License-Identifier: MIT
//
// print-capabilities (docs/CAPABILITIES.md). Only what scripts/smoke.sh and
// scripts/interop.sh exercise is declared.

import {
  ADAPTER, ADAPTER_VERSION, CONTRACT_VERSION, SCHEMA_VERSION, UPSTREAM_REVISION, UPSTREAM_VERSION, writeLine,
} from './common.mjs';

export const SUPPORTED_VERSIONS = ['1.6', '2.0.1', '2.1'];

// Per version, what the CSMS sends (CitrineOS's REST message API has an
// endpoint for it, and the self-tests get the station's answer back) and what
// it handles (CitrineOS's answer is the fixture's).
// Per version, what the CSMS sends (CitrineOS's REST message API has an
// endpoint for it, and the self-tests get the station's answer back) and what
// it handles (CitrineOS's answer is the fixture's), as scripts/interop.sh
// found them with the mobilityhouse station. adapters/citrineos/README.md
// says why each action that is left out is.
const V16_HANDLES = [
    'Authorize', 'BootNotification', 'DiagnosticsStatusNotification', 'FirmwareStatusNotification',
    'Heartbeat', 'MeterValues', 'StatusNotification', 'StopTransaction',
  ];
const V16_SENDS = [
    'ChangeAvailability', 'ChangeConfiguration', 'ClearCache', 'ClearChargingProfile', 'DataTransfer',
    'GetCompositeSchedule', 'GetConfiguration', 'GetDiagnostics', 'GetLocalListVersion',
    'RemoteStartTransaction', 'RemoteStopTransaction', 'Reset', 'SendLocalList', 'SetChargingProfile',
    'TriggerMessage', 'UnlockConnector', 'UpdateFirmware',
  ];
const V2_HANDLES = [
    'BootNotification', 'ClearedChargingLimit', 'FirmwareStatusNotification', 'Heartbeat',
    'LogStatusNotification', 'MeterValues', 'NotifyChargingLimit', 'NotifyEVChargingSchedule', 'NotifyEvent',
    'NotifyMonitoringReport', 'NotifyReport', 'PublishFirmwareStatusNotification', 'ReportChargingProfiles',
    'ReservationStatusUpdate', 'SecurityEventNotification', 'StatusNotification', 'TransactionEvent',
  ];
const V201_SENDS = [
    'CertificateSigned', 'ChangeAvailability', 'ClearCache', 'ClearChargingProfile', 'ClearDisplayMessage',
    'ClearVariableMonitoring', 'CostUpdated', 'CustomerInformation', 'DataTransfer', 'DeleteCertificate',
    'GetBaseReport', 'GetChargingProfiles', 'GetDisplayMessages', 'GetInstalledCertificateIds',
    'GetLocalListVersion', 'GetLog', 'GetMonitoringReport', 'GetReport', 'GetTransactionStatus',
    'GetVariables', 'PublishFirmware', 'RequestStartTransaction', 'RequestStopTransaction', 'ReserveNow',
    'Reset', 'SendLocalList', 'SetChargingProfile', 'SetDisplayMessage', 'SetMonitoringBase',
    'SetMonitoringLevel', 'SetNetworkProfile', 'SetVariableMonitoring', 'SetVariables', 'TriggerMessage',
    'UnlockConnector', 'UnpublishFirmware', 'UpdateFirmware',
  ];
const SERVER = {
  '1.6': { sends: V16_SENDS, handles: V16_HANDLES },
  '2.0.1': { sends: V201_SENDS, handles: V2_HANDLES },
  // OCPP 2.1: of the actions 2.1 adds, CitrineOS's message API sends
  // SetDefaultTariff (and NotifyWebPaymentStarted, whose endpoint does not
  // register), and none of the station's that it answers matches the fixture.
  '2.1': { sends: [...V201_SENDS, 'SetDefaultTariff'].sort(), handles: V2_HANDLES },
};

const COMMON_NOTES = [
  'A CSMS only: CitrineOS has no charging station. client exits 5 after the usage checks.',
  'CitrineOS is an application and answers every CALL from a station itself. The fixture decides only what CitrineOS takes from its data: the stations it accepts, their Basic authentication passwords (SecurityCtrlr.BasicAuthPassword), the id tokens and their status (its Authorizations), the heartbeat interval and the status of its BootNotification answer. handles lists the actions whose answer the self-tests found equal to the fixture\'s.',
  'No fixtureAnswers: an action the fixture does not answer is still answered by CitrineOS (or refused as it decides), not with NotImplemented. No fixtureCallErrors: CitrineOS cannot answer with a chosen CALLERROR; a fixture with a callError answer for the CSMS exits 5.',
  'Not handled (CitrineOS answers otherwise than the fixture): StartTransaction (1.6: CitrineOS numbers transactions itself), DataTransfer (Rejected), Get15118EVCertificate and GetCertificateStatus (Failed: no V2G or OCSP service), NotifyEVChargingNeeds (Rejected), NotifySettlement, VatNumberValidation (Rejected); 2.x Authorize: CitrineOS answers the fixture\'s accepted, blocked and expired tokens as the fixture does, but a token it does not know with Unknown (a valid OCPP 2.x status), where the fixture derives Invalid; NotifyCustomerInformation and NotifyDisplayMessages for a requestId it did not send; SignCertificate (it parses the CSR); 1.6 Security Whitepaper messages (NotImplemented); the 2.1 DER, periodic event stream, NotifyPriorityCharging, PullDynamicScheduleUpdate and GetCertificateChainStatus requests (FormatViolation).',
  'Not sent: actions without an endpoint in CitrineOS\'s message API (1.6 reservations and Security Whitepaper messages; most of what 2.1 adds), and CancelReservation (CitrineOS sends it only for a reservation it made), GetCompositeSchedule in 2.x (its endpoint fails with HTTP 500), InstallCertificate (its endpoint refuses the fixture\'s certificate), SignedUpdateFirmware (1.6, HTTP 500).',
  'compression: the CSMS accepts permessage-deflate (CitrineOS\'s ws server, perMessageDeflate); there is no station role.',
  'CitrineOS refuses (HTTP 401) a second connection for an identity that is connected, and reports it as connection-rejected with reason authentication.',
  'call sends through CitrineOS\'s REST message API (/ocpp/<version>/<module>/<action>) with a callback URL; an action without an endpoint is unsupported (exit 5). CitrineOS has one outstanding CALL per station.',
  'With security profile 3 CitrineOS requires the client certificate\'s CommonName to equal the station\'s serial number once a BootNotification has told it one (the identity before that).',
  'The container runs PostgreSQL (PostGIS) and RabbitMQ, which CitrineOS needs, next to it; the server takes several seconds to become ready.',
];

export function capabilities() {
  const versions = {};
  for (const version of SUPPORTED_VERSIONS) {
    versions[version] = {
      client: { supported: false, sends: [], handles: [] },
      server: { supported: true, sends: SERVER[version].sends, handles: SERVER[version].handles },
      features: {
        securityProfiles: [0, 1, 2, 3],
        compression: true,
        send: false,
        callResultError: false,
        fixtureCallErrors: false,
        fixtureAnswers: false,
        messageIds: true,
      },
      notes: COMMON_NOTES,
    };
  }
  return {
    schemaVersion: SCHEMA_VERSION,
    adapter: ADAPTER,
    adapterVersion: ADAPTER_VERSION,
    contractVersion: CONTRACT_VERSION,
    upstream: {
      name: 'CitrineOS core',
      repository: 'https://github.com/citrineos/citrineos-core',
      version: UPSTREAM_VERSION,
      revision: UPSTREAM_REVISION,
      license: 'Apache-2.0',
      language: 'TypeScript',
      independentEngine: true,
    },
    versions,
  };
}

export function printCapabilities() {
  writeLine(capabilities());
}
