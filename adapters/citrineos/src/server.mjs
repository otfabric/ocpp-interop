// SPDX-License-Identifier: MIT
//
// `server`: CitrineOS as the CSMS of docs/CONTAINER_CONTRACT.md.
//
// The container runs, as children of this process, PostgreSQL (with PostGIS)
// and RabbitMQ, the two services CitrineOS cannot run without, and CitrineOS
// itself in this process: its CitrineOSServer, unmodified, in mode `all` (the
// WebSocket servers, every module and every API). The adapter is glue around
// it:
//
// - configuration: one WebSocket server (websocket-servers.json) for the
//   requested port, version and security profile, and the system
//   configuration (database, broker, the fixture's heartbeat interval);
// - the fixture's stations, Basic authentication passwords and id tokens,
//   written into CitrineOS's database with CitrineOS's own repositories;
// - events: read off CitrineOS's router and network connection, which this
//   process wraps without changing what they do;
// - `call`: a CSMS CALL through CitrineOS's own REST message API, with a
//   callback URL that CitrineOS posts the station's answer to.
//
// CitrineOS answers the station's CALLs itself, as an application: the
// fixture decides only what CitrineOS takes from its data (see
// adapters/citrineos/README.md).

import fs from 'node:fs';
import http from 'node:http';
import net from 'node:net';
import os from 'node:os';
import path from 'node:path';
import crypto from 'node:crypto';
import { spawn } from 'node:child_process';
import { Console } from 'node:console';
import {
  ADAPTER, CONTROL_SOCKET, READY_FILE, SUBPROTOCOLS, ExitError, diagnostic, errorObject, event,
  parseFlags, resultDocument, unsupported, usageError, versionOf,
} from './common.mjs';
import { defaultFixturePath, loadFixture } from './fixture.mjs';
import { SUPPORTED_VERSIONS } from './capabilities.mjs';

const TENANT_ID = 1;
const PG_DATA = '/var/lib/ocpp-interop/postgres';
const PG_PORT = 5432;
const AMQP_PORT = 5672;
const STARTUP_TIMEOUT_MS = 60000;

// ---- arguments ----

function readPem(dir, name, kind) {
  const file = path.join(dir, name);
  let text;
  try {
    text = fs.readFileSync(file, 'utf8');
  } catch {
    throw usageError(`--tls-dir: ${file} cannot be read`);
  }
  try {
    if (kind === 'key') crypto.createPrivateKey(text);
    else new crypto.X509Certificate(text);
  } catch {
    throw usageError(`--tls-dir: ${file} is not a valid PEM ${kind === 'key' ? 'private key' : 'certificate'}`);
  }
  return file;
}

function parseServerArgs(args) {
  const flags = parseFlags(args, ['version', 'fixture', 'bind-address', 'port', 'security-profile', 'tls-dir',
    'ready-file', 'control-socket']);
  if (flags.positional.length > 0) throw usageError(`unexpected argument ${JSON.stringify(flags.positional[0])}`);
  const port = flags.number('port', 9000, 1, 65535);
  const profile = flags.number('security-profile', 0, 0, 3);
  if (flags.has('tls-dir') && profile < 2) throw usageError('--tls-dir is for security profiles 2 and 3');
  if (profile >= 2 && !flags.has('tls-dir')) throw usageError(`security profile ${profile} needs --tls-dir`);
  const bindAddress = flags.get('bind-address', '0.0.0.0');
  if (net.isIP(bindAddress) === 0) throw usageError(`--bind-address: ${JSON.stringify(bindAddress)} is not an IP address`);
  let tls = null;
  if (profile >= 2) {
    const dir = flags.get('tls-dir');
    tls = { cert: readPem(dir, 'server.crt', 'cert'), key: readPem(dir, 'server.key', 'key') };
    if (profile === 3) tls.ca = readPem(dir, 'ca.crt', 'cert');
  }
  const version = versionOf(flags, SUPPORTED_VERSIONS);
  const fixturePath = flags.get('fixture', defaultFixturePath(version));
  return {
    version, port, profile, bindAddress, tls, fixturePath,
    readyFile: flags.get('ready-file', READY_FILE),
    controlSocket: flags.get('control-socket', CONTROL_SOCKET),
  };
}

// ---- child services ----

function portFree(host, port) {
  return new Promise((resolve) => {
    const probe = net.createServer();
    probe.once('error', () => resolve(false));
    probe.listen({ host, port, exclusive: true }, () => probe.close(() => resolve(true)));
  });
}

function freePort() {
  return new Promise((resolve, reject) => {
    const probe = net.createServer();
    probe.once('error', reject);
    probe.listen({ host: '127.0.0.1', port: 0 }, () => {
      const { port } = probe.address();
      probe.close(() => resolve(port));
    });
  });
}

function connectable(port) {
  return new Promise((resolve) => {
    const socket = net.connect({ host: '127.0.0.1', port }, () => {
      socket.destroy();
      resolve(true);
    });
    socket.once('error', () => resolve(false));
  });
}

async function waitFor(what, check, child) {
  const deadline = Date.now() + STARTUP_TIMEOUT_MS;
  while (Date.now() < deadline) {
    if (child && child.exitCode !== null) throw new Error(`${what} exited with ${child.exitCode}`);
    if (await check()) return;
    await new Promise((r) => setTimeout(r, 100));
  }
  throw new Error(`${what} did not start within ${STARTUP_TIMEOUT_MS} ms`);
}

// The children write their logs to this process's stderr.
function startChild(command, args, env) {
  const child = spawn(command, args, { env: { ...process.env, ...env }, stdio: ['ignore', 2, 2] });
  child.on('error', (error) => diagnostic(`${command}: ${error.message}`));
  return child;
}

async function startPostgres(runDir) {
  const socketDir = path.join(runDir, 'postgres');
  fs.mkdirSync(socketDir, { recursive: true });
  const child = startChild('/usr/lib/postgresql/15/bin/postgres', ['-D', PG_DATA, '-p', String(PG_PORT),
    '-c', 'listen_addresses=127.0.0.1', '-c', `unix_socket_directories=${socketDir}`]);
  await waitFor('PostgreSQL', () => connectable(PG_PORT), child);
  return child;
}

async function startRabbitMQ(runDir) {
  const base = path.join(runDir, 'rabbitmq');
  fs.mkdirSync(base, { recursive: true });
  const child = startChild('/usr/lib/rabbitmq/bin/rabbitmq-server', [], {
    HOME: base,
    RABBITMQ_HOME: '/usr/lib/rabbitmq',
    RABBITMQ_MNESIA_BASE: path.join(base, 'mnesia'),
    RABBITMQ_LOG_BASE: path.join(base, 'log'),
    RABBITMQ_LOGS: '-',
    RABBITMQ_PID_FILE: path.join(base, 'rabbitmq.pid'),
    RABBITMQ_NODENAME: 'rabbit@localhost',
    RABBITMQ_NODE_IP_ADDRESS: '127.0.0.1',
    RABBITMQ_NODE_PORT: String(AMQP_PORT),
    RABBITMQ_ENABLED_PLUGINS_FILE: path.join(base, 'enabled_plugins'),
    RABBITMQ_CONFIG_FILE: path.join(base, 'rabbitmq'),
    RABBITMQ_ADVANCED_CONFIG_FILE: path.join(base, 'advanced.config'),
    RABBITMQ_CONF_ENV_FILE: path.join(base, 'rabbitmq-env.conf'),
    ERL_EPMD_ADDRESS: '127.0.0.1',
    RABBITMQ_SERVER_ADDITIONAL_ERL_ARGS: '-kernel inet_dist_use_interface {127,0,0,1}',
  });
  fs.writeFileSync(path.join(base, 'enabled_plugins'), '[].\n');
  await waitFor('RabbitMQ', () => connectable(AMQP_PORT), child);
  return child;
}

function stopChild(child, timeoutMs = 10000) {
  if (!child || child.exitCode !== null) return Promise.resolve();
  return new Promise((resolve) => {
    const timer = setTimeout(() => child.kill('SIGKILL'), timeoutMs);
    child.once('exit', () => {
      clearTimeout(timer);
      resolve();
    });
    child.kill('SIGTERM');
  });
}

// ---- the CSMS ----

export async function commandServer(args) {
  const options = parseServerArgs(args);
  const fixture = loadFixture(options.fixturePath, options.version);
  for (const [action, answer] of Object.entries(fixture.answers?.csms ?? {})) {
    if (answer.callError) {
      throw unsupported(`answers.csms.${action}: CitrineOS answers every CALL itself and cannot answer with a chosen CALLERROR (fixtureCallErrors)`);
    }
  }
  if (!(await portFree(options.bindAddress, options.port))) {
    throw new ExitError(3, `port ${options.port} on ${options.bindAddress} cannot be bound`);
  }
  // CitrineOS logs through console.log: everything but the adapter's own
  // JSON lines goes to stderr (docs/CONTAINER_CONTRACT.md, Output rules).
  const stderrConsole = new Console(process.stderr, process.stderr);
  for (const method of ['log', 'info', 'debug', 'warn', 'error', 'trace', 'dir', 'table']) {
    globalThis.console[method] = stderrConsole[method].bind(stderrConsole);
  }
  const csms = new Csms(options, fixture);
  try {
    await csms.start();
  } catch (error) {
    diagnostic(`server: ${error.stack ?? error}`);
    await csms.stopChildren();
    throw new ExitError(error instanceof ExitError ? error.code : 3, error.message);
  }
  await csms.done;
}

class Csms {
  constructor(options, fixture) {
    this.options = options;
    this.fixture = fixture;
    this.version = options.version;
    this.protocol = SUBPROTOCOLS[options.version];
    this.runDir = fs.mkdtempSync(path.join(os.tmpdir(), 'ocpp-interop-citrineos-'));
    this.open = new Map(); // station -> {remote, compression}
    this.handshakes = new Map(); // station -> {remote, compression} of the connection being set up
    this.received = new Map(); // `${station} ${messageId}` -> action of a CALL from the station
    this.sent = new Map(); // `${station} ${messageId}` -> pending CSMS CALL
    this.pending = new Map(); // callback token -> pending CSMS CALL
    this.routes = new Map(); // action -> REST path of CitrineOS's message API for this version
    this.done = new Promise((resolve) => { this.finish = resolve; });
    this.stopping = false;
  }

  async start() {
    this.postgres = await startPostgres(this.runDir);
    this.rabbitmq = await startRabbitMQ(this.runDir);
    this.callbackPort = await freePort();
    this.apiPort = await freePort();
    this.writeWebsocketConfig();
    await this.startCitrineOS();
    await this.startCallbackServer();
    this.startControlSocket();
    for (const signal of ['SIGTERM', 'SIGINT']) process.once(signal, () => this.stop());
    fs.mkdirSync(path.dirname(this.options.readyFile), { recursive: true });
    fs.writeFileSync(this.options.readyFile, '');
    event('ready', undefined, {
      adapter: ADAPTER, version: this.version, port: this.options.port,
      fixture: this.fixture.name, securityProfile: this.options.profile,
    });
  }

  writeWebsocketConfig() {
    const dir = path.join(this.runDir, 'files');
    fs.mkdirSync(dir, { recursive: true });
    const server = {
      id: 'ocpp-interop',
      host: this.options.bindAddress,
      port: this.options.port,
      pingInterval: 60,
      protocols: [this.protocol],
      securityProfile: this.options.profile,
      allowUnknownChargingStations: false,
      tenantId: TENANT_ID,
      dynamicTenantResolution: false,
      perMessageDeflate: true,
    };
    if (this.options.tls) {
      // CitrineOS reads these through its file storage, relative to its root.
      fs.copyFileSync(this.options.tls.key, path.join(dir, 'server.key'));
      fs.copyFileSync(this.options.tls.cert, path.join(dir, 'server.crt'));
      server.tlsKeyFilePath = 'server.key';
      server.tlsCertificateChainFilePath = 'server.crt';
      if (this.options.tls.ca) {
        fs.copyFileSync(this.options.tls.ca, path.join(dir, 'ca.crt'));
        server.rootCACertificateFilePath = 'ca.crt';
        // Required by CitrineOS's configuration schema for profile 3, but only
        // stored: it is the key CitrineOS would sign station certificates
        // with, which no test asks for. There is no such key in --tls-dir.
        server.mtlsCertificateAuthorityKeyFilePath = 'no-ca-key.pem';
      }
    }
    fs.writeFileSync(path.join(dir, 'websocket-servers.json'), JSON.stringify([server], null, 2));
    this.filesDir = dir;
  }

  systemConfig() {
    const boot = this.fixture.answers?.csms?.BootNotification?.payload ?? {};
    const level = (process.env.OCPP_INTEROP_LOG_LEVEL ?? '').toUpperCase();
    const ocpp = { getBaseReportOnPending: false };
    // What CitrineOS lets the deployment decide about its BootNotification
    // answer: the heartbeat interval and the status of a station it has no
    // boot configuration for.
    if (Number.isInteger(boot.interval) && boot.interval > 0) ocpp.heartbeatInterval = boot.interval;
    if (['Accepted', 'Pending', 'Rejected'].includes(boot.status)) ocpp.unknownChargerStatus = boot.status;
    return {
      env: 'production',
      host: '127.0.0.1',
      port: this.apiPort,
      database: { host: '127.0.0.1', port: PG_PORT, database: 'citrine', username: 'citrine', password: 'citrine' },
      messageBroker: { amqp: { url: `amqp://guest:guest@127.0.0.1:${AMQP_PORT}`, exchange: 'citrineos' } },
      fileAccess: { type: 'local', local: { defaultFilePath: this.filesDir } },
      cache: { type: 'memory' },
      // Swagger as CitrineOS defaults it: its message routes register their
      // schemas through it. The logo path is read relative to the working directory.
      swagger: { logoPath: '/opt/citrineos/dist/assets/logo.png' },
      logLevel: level === 'DEBUG' || level === 'TRACE' ? 2 : 3,
      ocpp,
    };
  }

  async startCitrineOS() {
    const { configSchema } = await import('@citrineos/types');
    const base = await import('@citrineos/base');
    const { CitrineOSServer } = await import('@citrineos/ocpp');
    this.base = base;
    const config = configSchema.parse(this.systemConfig());

    // The paths of CitrineOS's REST message API, as its APIs register them.
    const adapter = this;
    const api = base.AbstractMessageEndpointApi.prototype;
    const addRoute = api._addMessageRoute;
    api._addMessageRoute = function (route, endpoint, version) {
      if (version === adapter.protocol && route.bodySchema(version)) {
        adapter.routes.set(route.action, this._toMessagePath(route.eventGroup, route.action, version));
      }
      return addRoute.call(this, route, endpoint, version);
    };

    class AdapterServer extends CitrineOSServer {
      // The adapter handles SIGTERM and SIGINT (stop()).
      registerShutdownHandlers() {}

      async onInitialized() {
        adapter.observe(this._container);
        await adapter.seed(this._container);
      }
    }
    this.citrineos = new AdapterServer('all', config);
    await this.citrineos.run();
  }

  // ---- events, from CitrineOS's own objects ----

  observe(container) {
    const { getStationIdFromIdentifier } = this.base;
    const router = container.resolve('router');
    const network = container.resolve('networkConnection');
    const authenticator = container.resolve('authenticator');
    const stationOf = (url) => {
      try {
        return decodeURIComponent(String(url ?? '').split('?')[0].split('/').pop());
      } catch {
        return null;
      }
    };
    const remoteOf = (req) => (req?.socket?.remoteAddress ? `${req.socket.remoteAddress}:${req.socket.remotePort}` : null);

    const authenticate = authenticator.authenticate.bind(authenticator);
    authenticator.authenticate = async (request, tenantId, options) => {
      try {
        return await authenticate(request, tenantId, options);
      } catch (error) {
        const reason = error?.name === 'UpgradeUnknownError' ? 'unknown-station' : 'authentication';
        event('connection-rejected', stationOf(request.url), { reason, remote: remoteOf(request) });
        throw error;
      }
    };
    const handleProtocols = network._handleProtocols.bind(network);
    network._handleProtocols = (protocols, req, ...rest) => {
      const chosen = handleProtocols(protocols, req, ...rest);
      if (chosen === false) event('connection-rejected', stationOf(req.url), { reason: 'subprotocol', remote: remoteOf(req) });
      return chosen;
    };
    const onConnection = network._onConnection.bind(network);
    network._onConnection = (ws, ...rest) => {
      const req = rest[rest.length - 1];
      const extensions = String(ws.extensions ?? '');
      this.handshakes.set(stationOf(req.url), { remote: remoteOf(req), compression: extensions.includes('permessage-deflate') });
      return onConnection(ws, ...rest);
    };
    const registerConnection = router.registerConnection.bind(router);
    router.registerConnection = async (tenantId, station, protocol, ...rest) => {
      const registered = await registerConnection(tenantId, station, protocol, ...rest);
      if (registered) {
        const handshake = this.handshakes.get(station) ?? { remote: null, compression: null };
        this.handshakes.delete(station);
        this.open.set(station, handshake);
        event('connection-opened', station, { subprotocol: protocol ?? null, remote: handshake.remote, compression: handshake.compression });
      }
      return registered;
    };
    const handleClose = network._handleWebsocketClose.bind(network);
    network._handleWebsocketClose = async (identifier, code, reason, ...rest) => {
      const station = getStationIdFromIdentifier(identifier);
      try {
        return await handleClose(identifier, code, reason, ...rest);
      } finally {
        if (this.open.delete(station)) {
          event('connection-closed', station, { code: Number.isInteger(code) ? code : null, reason: reason ? String(reason) : '' });
          this.failPending(station, 'connection-lost', 'the connection closed while waiting');
        }
      }
    };
    const onMessage = router.onMessage.bind(router);
    router.onMessage = async (identifier, message, ...rest) => {
      const station = getStationIdFromIdentifier(identifier);
      const call = this.frame(station, String(message), 'in');
      const processed = await onMessage(identifier, message, ...rest);
      // A CALLRESULT CitrineOS refused (its schema validation of the
      // response): it never reaches the callback URL.
      if (!processed && call && call.frame?.[0] === 3) {
        call.resolve({ errorCode: 'invalid-response', errorMessage: 'CitrineOS refused the response (its log says why)' });
      }
      return processed;
    };
    const networkHook = router._networkHook;
    router.networkHook = async (identifier, message) => {
      await networkHook(identifier, message);
      this.frame(getStationIdFromIdentifier(identifier), String(message), 'out');
    };
  }

  frame(station, raw, direction) {
    let message;
    try {
      message = JSON.parse(raw);
    } catch {
      message = null;
    }
    if (!Array.isArray(message) || ![2, 3, 4].includes(message[0]) || typeof message[1] !== 'string') {
      if (direction === 'in') event('protocol-error', station, { detail: 'not an OCPP-J CALL, CALLRESULT or CALLERROR', frame: raw.slice(0, 4096) });
      return;
    }
    const [type, messageId] = message;
    const key = `${station} ${messageId}`;
    if (direction === 'in' && type === 2) {
      this.received.set(key, message[2]);
      event('call-received', station, { messageId, action: message[2], payload: message[3] ?? {} });
    } else if (direction === 'out' && type === 2) {
      const call = [...this.pending.values()].find((p) => p.station === station && p.action === message[2] && !p.messageId);
      if (call) {
        call.messageId = messageId;
        call.request = message[3] ?? {};
        this.sent.set(key, call);
      }
      event('call-sent', station, { messageId, action: message[2], payload: message[3] ?? {} });
    } else if (direction === 'out' && this.received.has(key)) {
      const action = this.received.get(key);
      this.received.delete(key);
      event('call-answered', station, type === 3
        ? { messageId, action, payload: message[2] ?? {} }
        : { messageId, action, error: errorObject(message[2], message[3], message[4]) });
    } else if (direction === 'in' && (type === 3 || type === 4)) {
      const call = this.sent.get(key);
      if (call) {
        this.sent.delete(key);
        call.frame = message;
      }
      event('call-result', station, type === 3
        ? { messageId, action: call?.action ?? null, payload: message[2] ?? {} }
        : { messageId, action: call?.action ?? null, error: errorObject(message[2], message[3], message[4]) });
      if (call && type === 4) this.complete(call);
      return call;
    }
    return undefined;
  }

  // ---- the fixture's data, through CitrineOS's repositories ----

  async seed(container) {
    const stations = container.resolve('chargingStationRepository');
    const authorizations = container.resolve('authorizationRepository');
    for (const station of this.fixture.stations ?? []) {
      // Only the identity: CitrineOS takes vendor, model and serial number
      // from the station's BootNotification.
      const saved = await stations.createOrUpdateChargingStation(TENANT_ID, { tenantId: TENANT_ID, ocppConnectionName: station.id });
      // OCPP 1.6: the station's connectors, each on an EVSE of its own as
      // CitrineOS commissions them. Without them CitrineOS fails a
      // StatusNotification or StartTransaction for a connector it does not
      // know. OCPP 2.x stations describe their EVSEs themselves.
      if (this.version === '1.6') {
        for (let connectorId = 1; connectorId <= (station.connectors ?? 0); connectorId++) {
          const { evseId } = await stations.autoCommissionEvseForOcpp16Connector(TENANT_ID, station.id);
          await stations.createOrUpdateOcpp16Connector(TENANT_ID, { tenantId: TENANT_ID, stationId: saved.id, connectorId, evseId });
        }
      }
      if (station.password) await this.seedPassword(container, station.id, station.password);
    }
    for (const token of this.fixture.idTokens ?? []) {
      await authorizations.createAuthorization(TENANT_ID, {
        idToken: token.idToken,
        idTokenType: token.type ?? null,
        status: token.status,
      });
    }
  }

  // The Basic authentication password CitrineOS checks:
  // SecurityCtrlr.BasicAuthPassword in its device model.
  async seedPassword(container, station, password) {
    const deviceModel = container.resolve('deviceModelRepository');
    await deviceModel.createOrUpdateDeviceModelByStationId(TENANT_ID, {
      component: { name: 'SecurityCtrlr' },
      variable: { name: 'BasicAuthPassword' },
      variableAttribute: [{ type: 'Actual', value: password, mutability: 'WriteOnly' }],
      variableCharacteristics: { dataType: 'passwordString', supportsMonitoring: false },
    }, station, new Date().toISOString());
  }

  // ---- `call`: CSMS CALLs through CitrineOS's REST message API ----

  async startCallbackServer() {
    this.callbackServer = http.createServer((req, res) => {
      const chunks = [];
      req.on('data', (chunk) => chunks.push(chunk));
      req.on('end', () => {
        res.writeHead(204).end();
        const token = decodeURIComponent((req.url ?? '').split('/').pop());
        const call = this.pending.get(token);
        if (!call) return;
        try {
          call.callback = JSON.parse(Buffer.concat(chunks).toString('utf8'));
        } catch {
          call.callback = null;
        }
        this.complete(call);
      });
    });
    await new Promise((resolve) => this.callbackServer.listen({ host: '127.0.0.1', port: this.callbackPort }, resolve));
  }

  startControlSocket() {
    const file = this.options.controlSocket;
    fs.mkdirSync(path.dirname(file), { recursive: true });
    try {
      fs.unlinkSync(file);
    } catch {
      // not there
    }
    this.control = net.createServer((conn) => {
      let buffer = '';
      conn.on('data', async (chunk) => {
        buffer += chunk.toString('utf8');
        const newline = buffer.indexOf('\n');
        if (newline < 0) return;
        let request;
        try {
          request = JSON.parse(buffer.slice(0, newline));
        } catch (error) {
          diagnostic(`control socket: ${error.message}`);
          conn.end();
          return;
        }
        const document = await this.call(request);
        conn.end(JSON.stringify(document) + '\n');
      });
      conn.on('error', () => {});
    });
    this.control.listen(file);
  }

  async call({ station, action, payload, timeoutMs }) {
    const started = Date.now();
    station = station ?? this.fixture.stations?.[0]?.id;
    payload = payload ?? {};
    timeoutMs = timeoutMs > 0 ? timeoutMs : 10000;
    const result = (fields) => resultDocument({
      version: this.version, operation: action, request: payload, elapsedMs: Date.now() - started, ...fields,
    });
    const route = this.routes.get(action);
    if (!route) return result({ ok: false, errorCode: 'unsupported', errorMessage: `CitrineOS has no message API for ${action} in OCPP ${this.version}` });
    if (!this.open.has(station)) return result({ ok: false, errorCode: 'not-connected', errorMessage: `${station} is not connected` });

    const token = crypto.randomUUID();
    const call = { token, station, action, request: payload };
    call.done = new Promise((resolve) => { call.resolve = resolve; });
    this.pending.set(token, call);
    const timer = setTimeout(() => call.resolve({ errorCode: 'timeout', errorMessage: `no response within ${timeoutMs} ms` }), timeoutMs);
    try {
      const query = new URLSearchParams({
        identifier: station, tenantId: String(TENANT_ID), callbackUrl: `http://127.0.0.1:${this.callbackPort}/callback/${token}`,
      });
      let response;
      try {
        response = await fetch(`http://127.0.0.1:${this.apiPort}${route}?${query}`, {
          method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload),
          signal: AbortSignal.timeout(timeoutMs),
        });
      } catch (error) {
        return result({ ok: false, errorCode: 'failed', errorMessage: `CitrineOS message API: ${error.message}` });
      }
      const body = await response.text();
      if (response.status === 400) {
        return result({ ok: false, errorCode: 'invalid-request', errorMessage: `CitrineOS refused the request: ${body.slice(0, 500)}` });
      }
      let confirmations;
      try {
        confirmations = JSON.parse(body);
      } catch {
        confirmations = null;
      }
      const confirmation = Array.isArray(confirmations) ? confirmations[0] : null;
      if (!response.ok || !confirmation?.success) {
        if (call.messageId === undefined) {
          return result({ ok: false, errorCode: 'failed', errorMessage: `CitrineOS did not send the CALL: HTTP ${response.status} ${body.slice(0, 500)}` });
        }
      }
      const outcome = await call.done;
      if (outcome?.errorCode) {
        return result({ ok: false, messageId: call.messageId, request: call.request, errorCode: outcome.errorCode, errorMessage: outcome.errorMessage });
      }
      if (call.frame?.[0] === 4) {
        const callError = errorObject(call.frame[2], call.frame[3], call.frame[4]);
        return result({ ok: false, messageId: call.messageId, request: call.request, callError, errorCode: 'call-error', errorMessage: `${callError.code}: ${callError.description}` });
      }
      return result({ ok: true, messageId: call.messageId, request: call.request, response: call.callback ?? call.frame?.[2] ?? null });
    } finally {
      clearTimeout(timer);
      this.pending.delete(token);
      if (call.messageId) this.sent.delete(`${station} ${call.messageId}`);
    }
  }

  complete(call) {
    call.resolve?.(null);
  }

  failPending(station, errorCode, errorMessage) {
    for (const call of this.pending.values()) {
      if (call.station === station) call.resolve({ errorCode, errorMessage });
    }
  }

  // ---- stop ----

  async stop() {
    if (this.stopping) return;
    this.stopping = true;
    try {
      fs.unlinkSync(this.options.readyFile);
    } catch {
      // already gone
    }
    try {
      this.control?.close();
      this.callbackServer?.close();
      await this.citrineos?.shutdown();
    } catch (error) {
      diagnostic(`shutdown: ${error.message}`);
    }
    await this.stopChildren();
    event('stopped');
    this.finish();
    process.exit(0);
  }

  async stopChildren() {
    await Promise.all([stopChild(this.rabbitmq), stopChild(this.postgres)]);
  }
}
