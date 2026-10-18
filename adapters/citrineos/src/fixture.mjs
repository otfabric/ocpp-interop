// SPDX-License-Identifier: MIT
//
// Fixture loading and validation (docs/FIXTURES.md): the schema of
// fixtures/schema/ocpp-fixture.schema.json, checked with Ajv (a dependency of
// CitrineOS, in its node_modules), and the cross-field rules of
// scripts/validate-fixtures.py, written out again here.

import fs from 'node:fs';
import path from 'node:path';
import { createRequire } from 'node:module';
import { FIXTURE_ROOT, VERSION_DIRS, fixtureError, unsupported } from './common.mjs';

const require = createRequire(import.meta.url);

const V16_FROM_STATION = [
  'Authorize', 'BootNotification', 'DataTransfer', 'DiagnosticsStatusNotification',
  'FirmwareStatusNotification', 'Heartbeat', 'MeterValues', 'StartTransaction',
  'StatusNotification', 'StopTransaction',
  'LogStatusNotification', 'SecurityEventNotification', 'SignCertificate',
  'SignedFirmwareStatusNotification',
];
const V16_FROM_CSMS = [
  'CancelReservation', 'ChangeAvailability', 'ChangeConfiguration', 'ClearCache',
  'ClearChargingProfile', 'DataTransfer', 'GetCompositeSchedule', 'GetConfiguration',
  'GetDiagnostics', 'GetLocalListVersion', 'RemoteStartTransaction',
  'RemoteStopTransaction', 'ReserveNow', 'Reset', 'SendLocalList', 'SetChargingProfile',
  'TriggerMessage', 'UnlockConnector', 'UpdateFirmware',
  'CertificateSigned', 'DeleteCertificate', 'ExtendedTriggerMessage',
  'GetInstalledCertificateIds', 'GetLog', 'InstallCertificate', 'SignedUpdateFirmware',
];
const V201_FROM_STATION = [
  'Authorize', 'BootNotification', 'ClearedChargingLimit', 'DataTransfer',
  'FirmwareStatusNotification', 'Get15118EVCertificate', 'GetCertificateStatus',
  'Heartbeat', 'LogStatusNotification', 'MeterValues', 'NotifyChargingLimit',
  'NotifyCustomerInformation', 'NotifyDisplayMessages', 'NotifyEVChargingNeeds',
  'NotifyEVChargingSchedule', 'NotifyEvent', 'NotifyMonitoringReport', 'NotifyReport',
  'PublishFirmwareStatusNotification', 'ReportChargingProfiles',
  'ReservationStatusUpdate', 'SecurityEventNotification', 'SignCertificate',
  'StatusNotification', 'TransactionEvent',
];
const V201_FROM_CSMS = [
  'CancelReservation', 'CertificateSigned', 'ChangeAvailability', 'ClearCache',
  'ClearChargingProfile', 'ClearDisplayMessage', 'ClearVariableMonitoring',
  'CostUpdated', 'CustomerInformation', 'DataTransfer', 'DeleteCertificate',
  'GetBaseReport', 'GetChargingProfiles', 'GetCompositeSchedule', 'GetDisplayMessages',
  'GetInstalledCertificateIds', 'GetLocalListVersion', 'GetLog', 'GetMonitoringReport',
  'GetReport', 'GetTransactionStatus', 'GetVariables', 'InstallCertificate',
  'PublishFirmware', 'RequestStartTransaction', 'RequestStopTransaction', 'ReserveNow',
  'Reset', 'SendLocalList', 'SetChargingProfile', 'SetDisplayMessage',
  'SetMonitoringBase', 'SetMonitoringLevel', 'SetNetworkProfile',
  'SetVariableMonitoring', 'SetVariables', 'TriggerMessage', 'UnlockConnector',
  'UnpublishFirmware', 'UpdateFirmware',
];
const V21_FROM_STATION = [
  ...V201_FROM_STATION,
  'BatterySwap', 'ClosePeriodicEventStream', 'GetCertificateChainStatus', 'NotifyDERAlarm',
  'NotifyDERStartStop', 'NotifyPeriodicEventStream', 'NotifyPriorityCharging', 'NotifySettlement',
  'OpenPeriodicEventStream', 'PullDynamicScheduleUpdate', 'ReportDERControl', 'VatNumberValidation',
];
const V21_FROM_CSMS = [
  ...V201_FROM_CSMS,
  'AFRRSignal', 'AdjustPeriodicEventStream', 'ChangeTransactionTariff', 'ClearDERControl',
  'ClearTariffs', 'GetDERControl', 'GetPeriodicEventStream', 'GetTariffs',
  'NotifyAllowedEnergyTransfer', 'NotifyWebPaymentStarted', 'RequestBatterySwap', 'SetDERControl',
  'SetDefaultTariff', 'UpdateDynamicSchedule', 'UsePriorityCharging',
];
export const ACTIONS = {
  '1.6': { station: new Set(V16_FROM_STATION), csms: new Set(V16_FROM_CSMS) },
  '2.0.1': { station: new Set(V201_FROM_STATION), csms: new Set(V201_FROM_CSMS) },
  '2.1': { station: new Set(V21_FROM_STATION), csms: new Set(V21_FROM_CSMS) },
};
export const SEND_ACTIONS = { '1.6': new Set(), '2.0.1': new Set(), '2.1': new Set(['NotifyPeriodicEventStream']) };
const DERIVED = {
  idTokens: new Set(['Authorize', 'StartTransaction']),
  configuration: new Set(['GetConfiguration']),
  variables: new Set(['GetVariables', 'SetVariables']),
};
const ID_TAG_MAX = { '1.6': 20, '2.0.1': 36, '2.1': 255 };
// The error codes of OCPP-J (1.6 and 2.x, Part 4).
const OCPPJ_ERROR_CODES = new Set([
  'NotImplemented', 'NotSupported', 'InternalError', 'ProtocolError', 'SecurityError',
  'FormationViolation', 'FormatViolation', 'PropertyConstraintViolation',
  'OccurenceConstraintViolation', 'OccurrenceConstraintViolation', 'TypeConstraintViolation',
  'GenericError', 'MessageTypeNotSupported', 'RpcFrameworkError',
]);

export const defaultFixturePath = (version) => path.join(FIXTURE_ROOT, VERSION_DIRS[version], 'baseline', 'fixture.json');

let validateSchema;
function schemaErrors(fixture) {
  if (!validateSchema) {
    const Ajv2020 = require('ajv/dist/2020').default;
    const addFormats = require('ajv-formats');
    const ajv = new Ajv2020({ allErrors: true, strict: false });
    (addFormats.default ?? addFormats)(ajv);
    const schema = JSON.parse(fs.readFileSync(path.join(FIXTURE_ROOT, 'schema', 'ocpp-fixture.schema.json'), 'utf8'));
    validateSchema = ajv.compile(schema);
  }
  if (validateSchema(fixture)) return [];
  return validateSchema.errors.map((e) => `${e.instancePath || '(root)'}: ${e.message}`.slice(0, 200));
}

function duplicates(values) {
  const seen = new Set();
  const dup = new Set();
  for (const v of values) (seen.has(v) ? dup : seen).add(v);
  return [...dup].sort();
}

function semanticErrors(fx, file) {
  const errors = [];
  const version = fx.ocppVersion;
  const dir = path.basename(path.dirname(file));
  const versionDir = path.basename(path.dirname(path.dirname(file)));
  if (fx.name !== dir) errors.push(`name ${JSON.stringify(fx.name)} is not the directory name ${JSON.stringify(dir)}`);
  if (VERSION_DIRS[version] !== versionDir) errors.push(`ocppVersion ${JSON.stringify(version)} does not belong in ${versionDir}/`);
  if (!ACTIONS[version]) {
    errors.push(`ocppVersion ${JSON.stringify(version)}: no action list`);
    return errors;
  }
  const stations = fx.stations ?? [];
  for (const d of duplicates(stations.map((s) => s.id))) errors.push(`stations: identity ${JSON.stringify(d)} appears twice`);
  for (const d of duplicates((fx.idTokens ?? []).map((t) => t.idToken))) errors.push(`idTokens: ${JSON.stringify(d)} appears twice`);
  for (const t of fx.idTokens ?? []) {
    if ((t.idToken ?? '').length > ID_TAG_MAX[version]) {
      errors.push(`idTokens: ${JSON.stringify(t.idToken)} is longer than ${ID_TAG_MAX[version]} characters`);
    }
    if (version !== '1.6' && !('type' in t)) errors.push(`idTokens: ${JSON.stringify(t.idToken)} has no type, which OCPP 2.x requires`);
  }
  for (const d of duplicates((fx.configuration ?? []).map((c) => c.key))) errors.push(`configuration: key ${JSON.stringify(d)} appears twice`);
  for (const d of duplicates((fx.variables ?? []).filter((v) => v.component && v.variable)
    .map((v) => JSON.stringify([v.component.name, v.component.instance ?? null, v.variable.name, v.variable.instance ?? null])))) {
    errors.push(`variables: ${d} appears twice`);
  }
  for (const s of stations) {
    for (const d of duplicates((s.evses ?? []).map((e) => e.id))) errors.push(`stations ${JSON.stringify(s.id)}: EVSE ${d} appears twice`);
  }
  const receives = { csms: ACTIONS[version].station, station: ACTIONS[version].csms };
  for (const [role, table] of Object.entries(fx.answers ?? {})) {
    for (const [action, answer] of Object.entries(table ?? {})) {
      if (!receives[role]?.has(action)) {
        errors.push(`answers.${role}.${action}: the ${role} does not receive ${action} in OCPP ${version}`);
      } else if (SEND_ACTIONS[version].has(action)) {
        errors.push(`answers.${role}.${action}: ${action} is a SEND in OCPP ${version}, which has no answer`);
      }
      const derived = answer && typeof answer === 'object' ? answer.derived : undefined;
      if (derived && !DERIVED[derived]?.has(action)) errors.push(`answers.${role}.${action}: cannot be derived from ${derived}`);
      if (derived && !(fx[derived]?.length)) errors.push(`answers.${role}.${action}: derived from ${derived}, which is empty`);
    }
  }
  for (const [name, sample] of Object.entries(fx.samples ?? {})) {
    if (!ACTIONS[version][sample.from]?.has(sample.action)) {
      errors.push(`samples.${name}: the ${sample.from} does not send ${sample.action} in OCPP ${version}`);
    }
  }
  return errors;
}

// loadFixture: the fixture at `file` for `version`; exit 4 when it is missing
// or invalid, exit 5 when it has an answer the adapter cannot give.
export function loadFixture(file, version) {
  let fixture;
  try {
    fixture = JSON.parse(fs.readFileSync(file, 'utf8'));
  } catch (error) {
    throw fixtureError(`${file}: ${error.message}`);
  }
  const errors = [...schemaErrors(fixture), ...semanticErrors(fixture, file)];
  if (fixture.ocppVersion !== version) errors.push(`ocppVersion ${JSON.stringify(fixture.ocppVersion)} is not --version ${version}`);
  if (errors.length > 0) throw fixtureError(`${file}: ${errors.join('; ')}`);
  for (const [role, table] of Object.entries(fixture.answers ?? {})) {
    for (const [action, answer] of Object.entries(table ?? {})) {
      if (answer.callError && !OCPPJ_ERROR_CODES.has(answer.callError.code)) {
        throw unsupported(`answers.${role}.${action}: ${answer.callError.code} is not an OCPP-J error code`);
      }
    }
  }
  return fixture;
}

export function fixtureFile(version, name) {
  return path.join(FIXTURE_ROOT, VERSION_DIRS[version], name, 'fixture.json');
}
