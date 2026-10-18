// SPDX-License-Identifier: MIT
//
// ocpp-interop adapter for CitrineOS (docs/CONTAINER_CONTRACT.md): a CSMS
// only. The commands that need no CitrineOS (usage, client, call,
// print-capabilities, print-fixture, version) do not load it.

import fs from 'node:fs';
import net from 'node:net';
import {
  ADAPTER_VERSION, CONTROL_SOCKET, ExitError, UPSTREAM_REVISION, UPSTREAM_VERSION, VERSION_DIRS, actionOf,
  diagnostic, exitCodeOf, parseFlags, payloadOf, resultDocument, unsupported, usageError, versionOf, writeLine,
} from './common.mjs';
import { fixtureFile } from './fixture.mjs';
import { SUPPORTED_VERSIONS, printCapabilities } from './capabilities.mjs';

const USAGE = `usage:
  ocpp-interop server --version V [--fixture PATH] [--bind-address ADDR] [--port N]
                      [--security-profile N] [--tls-dir DIR] [--ready-file PATH] [--control-socket PATH]
  ocpp-interop client ...            (not supported: CitrineOS is a CSMS only)
  ocpp-interop call [--station ID] <Action> [--payload JSON] [--timeout-ms N] [--control-socket PATH]
  ocpp-interop print-capabilities
  ocpp-interop print-fixture --version V [NAME]
  ocpp-interop version`;

const CLIENT_FLAGS = ['version', 'url', 'station', 'fixture', 'payload', 'security-profile', 'tls-dir', 'compression',
  'connect-timeout-ms', 'timeout-ms', 'hold-ms', 'ready-file', 'control-socket'];

// client: the usage checks of the contract, then exit 5 (no station role).
function commandClient(args) {
  const flags = parseFlags(args, CLIENT_FLAGS);
  if (flags.positional.length !== 1) throw usageError('client takes `run` or an action');
  const operation = flags.positional[0] === 'run' ? 'run' : actionOf(flags.positional[0]);
  if (!flags.has('url')) throw usageError('--url is required');
  if (!flags.has('version')) throw usageError('--version is required');
  const version = flags.get('version');
  if (!(version in VERSION_DIRS)) throw usageError(`--version: ${JSON.stringify(version)} is not 1.6, 2.0.1 or 2.1`);
  const profile = flags.number('security-profile', 0, 0, 3);
  const url = flags.get('url');
  const scheme = profile >= 2 ? 'wss://' : 'ws://';
  if (!url.startsWith(scheme)) throw usageError(`--url must be a ${scheme} URL with security profile ${profile}`);
  if (flags.has('tls-dir') && profile < 2) throw usageError('--tls-dir is for security profiles 2 and 3');
  if (profile >= 2 && !flags.has('tls-dir')) throw usageError(`security profile ${profile} needs --tls-dir`);
  for (const name of ['connect-timeout-ms', 'timeout-ms', 'hold-ms']) flags.number(name, 0, 0, 3600000);
  if (operation === 'run') {
    if (flags.has('payload')) throw usageError('client run takes no --payload');
    throw unsupported('CitrineOS is a CSMS only: there is no station role');
  }
  const payload = payloadOf(flags);
  writeLine(resultDocument({
    version, operation, ok: false, request: payload, errorCode: 'unsupported',
    errorMessage: 'CitrineOS is a CSMS only: there is no station role',
  }));
  throw new ExitError(5, 'unsupported: CitrineOS is a CSMS only');
}

// call: one CALL through the control socket of a running server.
async function commandCall(args) {
  const flags = parseFlags(args, ['station', 'payload', 'timeout-ms', 'control-socket']);
  if (flags.positional.length !== 1) throw usageError('call takes exactly one action');
  const action = actionOf(flags.positional[0]);
  const payload = payloadOf(flags);
  const timeoutMs = flags.number('timeout-ms', 10000, 1, 3600000);
  const request = { station: flags.has('station') ? flags.get('station') : null, action, payload, timeoutMs };
  const file = flags.get('control-socket', CONTROL_SOCKET);
  const line = await new Promise((resolve, reject) => {
    const conn = net.connect(file);
    let data = '';
    conn.on('connect', () => conn.write(JSON.stringify(request) + '\n'));
    conn.on('data', (chunk) => { data += chunk.toString('utf8'); });
    conn.on('end', () => resolve(data.trim()));
    conn.on('error', (error) => reject(new ExitError(3, `no server behind ${file}: ${error.message}`)));
  });
  let document;
  try {
    document = JSON.parse(line);
  } catch {
    throw new ExitError(1, `the server answered no result document: ${line.slice(0, 200)}`);
  }
  writeLine(document);
  const code = exitCodeOf(document.error?.code);
  if (code !== 0) throw new ExitError(code, '');
}

function commandPrintFixture(args) {
  const flags = parseFlags(args, ['version']);
  if (!flags.has('version')) throw usageError('--version is required');
  const version = versionOf(flags, Object.keys(VERSION_DIRS));
  if (flags.positional.length > 1) throw usageError('print-fixture takes at most one name');
  const name = flags.positional[0] ?? 'baseline';
  if (!/^[a-z0-9][a-z0-9-]*$/.test(name)) throw new ExitError(1, `no fixture ${JSON.stringify(name)}`);
  let data;
  try {
    data = fs.readFileSync(fixtureFile(version, name));
  } catch {
    throw new ExitError(1, `no fixture ${name} for OCPP ${version}`);
  }
  fs.writeSync(1, data);
}

async function dispatch(args) {
  const [command, ...rest] = args;
  switch (command) {
    case 'server': {
      const { commandServer } = await import('./server.mjs');
      return commandServer(rest);
    }
    case 'client':
      return commandClient(rest);
    case 'call':
      return commandCall(rest);
    case 'print-capabilities':
      if (rest.length > 0) throw usageError('print-capabilities takes no arguments');
      return printCapabilities();
    case 'print-fixture':
      return commandPrintFixture(rest);
    case 'version':
      if (rest.length > 0) throw usageError('version takes no arguments');
      fs.writeSync(1, `ocpp-interop-citrineos ${ADAPTER_VERSION} (CitrineOS ${UPSTREAM_VERSION} ${UPSTREAM_REVISION})\n`);
      return undefined;
    case undefined:
    case '':
      throw usageError('no command');
    default:
      throw usageError(`unknown command ${JSON.stringify(command)}`);
  }
}

try {
  await dispatch(process.argv.slice(2));
  process.exitCode = 0;
} catch (error) {
  if (error instanceof ExitError) {
    if (error.message) diagnostic(error.code === 2 ? `${error.message}\n${USAGE}` : error.message);
    process.exit(error.code);
  }
  diagnostic(`error: ${error?.stack ?? error}`);
  process.exit(1);
}
