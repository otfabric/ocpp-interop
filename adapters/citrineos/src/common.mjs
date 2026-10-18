// SPDX-License-Identifier: MIT
//
// Glue shared by the commands: errors and exit codes, flags, JSON output.

import fs from 'node:fs';

export const ADAPTER = 'citrineos';
export const SCHEMA_VERSION = '1.0';
export const CONTRACT_VERSION = '0.5';
export const READY_FILE = '/run/ocpp-interop/ready';
export const CONTROL_SOCKET = '/run/ocpp-interop/control.sock';
export const FIXTURE_ROOT = '/fixtures';
export const VERSION_DIRS = { '1.6': 'v16', '2.0.1': 'v201', '2.1': 'v21' };
export const SUBPROTOCOLS = { '1.6': 'ocpp1.6', '2.0.1': 'ocpp2.0.1', '2.1': 'ocpp2.1' };
export const ACTION_NAME = /^[A-Z][A-Za-z0-9]+$/;

let buildInfo = {};
try {
  buildInfo = JSON.parse(fs.readFileSync(new URL('./build-info.json', import.meta.url), 'utf8'));
} catch {
  // Not built by the Dockerfile: the defaults below.
}
export const ADAPTER_VERSION = buildInfo.adapterVersion ?? 'dev';
export const UPSTREAM_VERSION = buildInfo.upstreamVersion ?? 'unknown';
export const UPSTREAM_REVISION = buildInfo.upstreamRevision ?? '0'.repeat(40);

// An error that ends the command with an exit code (docs/CONTAINER_CONTRACT.md, Exit codes).
export class ExitError extends Error {
  constructor(code, message) {
    super(message);
    this.code = code;
  }
}
export const usageError = (message) => new ExitError(2, `usage error: ${message}`);
export const unsupported = (message) => new ExitError(5, `unsupported: ${message}`);
export const fixtureError = (message) => new ExitError(4, `invalid fixture: ${message}`);

// The flags without a value (docs/CONTAINER_CONTRACT.md, Commands).
const SWITCHES = new Set(['compression']);

// parseFlags: `--name value` pairs and the positional arguments, refusing
// unknown flags and flags given twice.
export function parseFlags(args, known) {
  const given = new Map();
  const positional = [];
  for (let i = 0; i < args.length; i++) {
    const arg = args[i];
    if (!arg.startsWith('--')) {
      positional.push(arg);
      continue;
    }
    const name = arg.slice(2);
    if (!known.includes(name)) throw usageError(`unknown flag ${arg}`);
    if (given.has(name)) throw usageError(`${arg} is given more than once`);
    if (SWITCHES.has(name)) {
      given.set(name, 'true');
      continue;
    }
    if (i + 1 >= args.length) throw usageError(`${arg} needs a value`);
    given.set(name, args[++i]);
  }
  return {
    positional,
    has: (name) => given.has(name),
    get: (name, fallback) => (given.has(name) ? given.get(name) : fallback),
    number(name, fallback, low, high) {
      if (!given.has(name)) return fallback;
      const text = given.get(name);
      if (!/^[0-9]{1,12}$/.test(text)) throw usageError(`--${name}: ${JSON.stringify(text)} is not a number`);
      const value = Number(text);
      if (value < low || value > high) throw usageError(`--${name}: ${value} is not within ${low}..${high}`);
      return value;
    },
  };
}

export function versionOf(flags, supported) {
  if (!flags.has('version')) throw usageError('--version is required');
  const version = flags.get('version');
  if (!(version in VERSION_DIRS)) throw usageError(`--version: ${JSON.stringify(version)} is not 1.6, 2.0.1 or 2.1`);
  if (!supported.includes(version)) throw unsupported(`OCPP ${version} is not supported by this adapter`);
  return version;
}

export function payloadOf(flags) {
  let value;
  try {
    value = JSON.parse(flags.get('payload', '{}'));
  } catch (error) {
    throw usageError(`--payload is not JSON: ${error.message}`);
  }
  if (value === null || typeof value !== 'object' || Array.isArray(value)) {
    throw usageError('--payload must be a JSON object');
  }
  return value;
}

export function actionOf(text) {
  if (!ACTION_NAME.test(text ?? '')) throw usageError(`${JSON.stringify(text)} is not an OCPP action name`);
  return text;
}

// ---- output: stdout carries JSON only ----

export function writeLine(document) {
  fs.writeSync(1, JSON.stringify(document) + '\n');
}

export const now = () => new Date().toISOString();

export function event(name, station, fields = {}) {
  const document = { event: name, time: now() };
  if (station !== undefined && station !== null) document.station = station;
  writeLine(Object.assign(document, fields));
}

export function errorObject(code, description, details) {
  return { code, description: description ?? '', details: details ?? {} };
}

// resultDocument: schemas/client-result.schema.json.
export function resultDocument({ version, operation, ok, messageId, request, response, callError, errorCode, errorMessage, elapsedMs }) {
  const document = { schemaVersion: SCHEMA_VERSION, adapter: ADAPTER, version, operation, ok };
  if (messageId) document.messageId = messageId;
  document.request = request ?? null;
  document.response = response ?? null;
  document.callError = callError ?? null;
  document.error = errorCode ? { code: errorCode, message: errorMessage ?? '' } : null;
  document.elapsedMs = Math.max(0, Math.round(elapsedMs ?? 0));
  return document;
}

export function exitCodeOf(errorCode) {
  if (!errorCode) return 0;
  if (errorCode === 'connect-failed') return 3;
  if (errorCode === 'unsupported') return 5;
  return 1;
}

export function diagnostic(message) {
  fs.writeSync(2, `${now()} ocpp-interop: ${message}\n`);
}
