// SPDX-License-Identifier: MIT
package org.otfabric.ocppinterop.chargetime;

import eu.chargetime.ocpp.AuthenticationException;
import eu.chargetime.ocpp.JSONConfiguration;
import eu.chargetime.ocpp.JSONServer;
import eu.chargetime.ocpp.MultiProtocolJSONServer;
import eu.chargetime.ocpp.ProtocolVersion;
import eu.chargetime.ocpp.ServerEvents;
import eu.chargetime.ocpp.feature.profile.ServerCoreEventHandler;
import eu.chargetime.ocpp.feature.profile.ServerCoreProfile;
import eu.chargetime.ocpp.model.SessionInformation;
import java.io.IOException;
import java.net.InetSocketAddress;
import java.net.ServerSocket;
import java.net.Socket;
import java.net.URLDecoder;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.UUID;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.CountDownLatch;

/**
 * {@code server}: a CSMS. OCPP 1.6: the upstream's {@link JSONServer} (OCPP 1.6 JSON over
 * Java-WebSocket) with the Core profile and the fixture's profile. OCPP 2.0.1: the upstream's
 * {@link MultiProtocolJSONServer} for that one protocol version, with the fixture's function.
 */
final class ServerCommand {
  private ServerCommand() {}

  /** What the command needs of either upstream server class. */
  private interface Upstream {
    void open(String host, int port, ServerEvents events);

    void close();

    Endpoint.Sender sender(UUID session);
  }

  private static Upstream upstream(String version, Endpoint endpoint, Tls.Server tls) {
    if (version.equals("1.6")) {
      JSONServer server =
          new JSONServer(new ServerCoreProfile(endpoint.handler(ServerCoreEventHandler.class)), JSONConfiguration.get());
      server.addFeatureProfile(endpoint.profile());
      if (tls != null) {
        server.enableWSS(tls);
      }
      return new Upstream() {
        public void open(String host, int port, ServerEvents events) {
          server.open(host, port, events);
        }

        public void close() {
          server.close();
        }

        public Endpoint.Sender sender(UUID session) {
          return request -> server.send(session, request);
        }
      };
    }
    // JSONConfiguration.WEBSOCKET_COMPRESSION_SUPPORT is true by default: the
    // server accepts permessage-deflate whenever a station offers it.
    MultiProtocolJSONServer server =
        new MultiProtocolJSONServer(List.of(ProtocolVersion.OCPP2_0_1), JSONConfiguration.get());
    server.addFunction(ProtocolVersion.OCPP2_0_1, endpoint.function());
    if (tls != null) {
      server.enableWSS(tls);
    }
    return new Upstream() {
      public void open(String host, int port, ServerEvents events) {
        server.open(host, port, events);
      }

      public void close() {
        server.close();
      }

      public Endpoint.Sender sender(UUID session) {
        return request -> server.send(session, request);
      }
    };
  }

  /** The station identity: the last segment of the request path. */
  static String stationOf(String resource) {
    String path = resource == null ? "" : resource;
    int query = path.indexOf('?');
    if (query >= 0) {
      path = path.substring(0, query);
    }
    while (path.endsWith("/")) {
      path = path.substring(0, path.length() - 1);
    }
    return URLDecoder.decode(path.substring(path.lastIndexOf('/') + 1), StandardCharsets.UTF_8);
  }

  static String remote(InetSocketAddress address) {
    return address == null ? null : address.getAddress().getHostAddress() + ":" + address.getPort();
  }

  static int run(List<String> args) throws Exception {
    Main.Flags flags =
        Main.flags(args, Set.of("version", "fixture", "bind-address", "port", "security-profile", "tls-dir",
            "ready-file", "control-socket"));
    if (!flags.positional.isEmpty()) {
      throw new Main.UsageException("unexpected argument '" + flags.positional.get(0) + "'");
    }
    int port = (int) flags.number("port", 9000, 1, 65535);
    String version = Main.version(flags);
    int profile = Main.securityProfile(flags);
    Tls.Server tls = profile >= 2 ? Tls.server(Path.of(flags.get("tls-dir", "")), profile) : null;
    Fixture fixture = Main.fixture(flags, version);
    String bindAddress = flags.get("bind-address", "0.0.0.0");
    Path readyFile = Path.of(flags.get("ready-file", Main.READY_FILE));
    Path controlPath = Path.of(flags.get("control-socket", Main.CONTROL_SOCKET));

    Map<UUID, String> stationBySession = new ConcurrentHashMap<>();
    Map<String, UUID> sessionByStation = new ConcurrentHashMap<>();
    Map<UUID, CompletableFuture<Void>> closedBySession = new ConcurrentHashMap<>();

    Endpoint endpoint = new Endpoint(version, fixture, "csms", stationBySession::get, true);
    Upstream server = upstream(version, endpoint, tls);

    ServerEvents events =
        new ServerEvents() {
          @Override
          public void authenticateSession(SessionInformation information, String username, byte[] password)
              throws AuthenticationException {
            String station = stationOf(information.getIdentifier());
            String from = remote(information.getAddress());
            if (!fixture.stations.containsKey(station)) {
              Out.event("connection-rejected", station, "reason", "unknown-station", "remote", from);
              throw new AuthenticationException(404, "Unknown station");
            }
            String commonName = tls == null ? null : tls.peerCommonName(from);
            if (profile == 3 && !station.equals(commonName)) {
              // The handshake verified the certificate against ca.crt; its
              // subject must name this station.
              Out.event("connection-rejected", station, "reason", "authentication", "remote", from);
              throw new AuthenticationException(401, "Unauthorized");
            }
            if (profile == 1 || profile == 2) {
              String expected = fixture.password(station);
              boolean ok = station.equals(username) && password != null
                  && expected.equals(new String(password, StandardCharsets.UTF_8));
              if (!ok) {
                Out.event("connection-rejected", station, "reason", "authentication", "remote", from);
                throw new AuthenticationException(401, "Unauthorized");
              }
            }
          }

          @Override
          public void newSession(UUID session, SessionInformation information) {
            String station = stationOf(information.getIdentifier());
            stationBySession.put(session, station);
            sessionByStation.put(station, session);
            closedBySession.put(session, new CompletableFuture<>());
            // The 1.6 JSONServer never negotiates permessage-deflate; the 2.0.1
            // server does when offered, but does not expose it.
            Out.event("connection-opened", station, "subprotocol", null, "remote", remote(information.getAddress()),
                "compression", version.equals("1.6") ? Boolean.FALSE : null);
          }

          @Override
          public void lostSession(UUID session) {
            String station = stationBySession.remove(session);
            if (station != null) {
              sessionByStation.remove(station, session);
            }
            CompletableFuture<Void> closed = closedBySession.remove(session);
            if (closed != null) {
              closed.complete(null);
            }
            Out.event("connection-closed", station, "code", null, "reason", null);
          }
        };

    // JSONServer.open() binds in a background thread and reports a failure
    // only in its log: check that the port is free first, and that it
    // accepts connections afterwards.
    try (ServerSocket probe = new ServerSocket()) {
      probe.bind(new InetSocketAddress(bindAddress, port));
    } catch (IOException e) {
      System.err.println("server: cannot listen on " + bindAddress + ":" + port + ": " + e.getMessage());
      return 3;
    }
    server.open(bindAddress, port, events);
    if (!listening(bindAddress, port, 15000)) {
      System.err.println("server: " + bindAddress + ":" + port + " does not accept connections");
      server.close();
      return 3;
    }

    Control control;
    try {
      control =
          Control.serve(
              controlPath,
              (station, action, payload, timeoutMs) -> {
                UUID session = station == null ? null : sessionByStation.get(station);
                CompletableFuture<Void> closed = session == null ? null : closedBySession.get(session);
                if (session == null || closed == null) {
                  return Out.result(version, action, false, payload, null, null, "not-connected",
                      "no open connection for station " + station, 0);
                }
                return endpoint.execute(action, payload, timeoutMs, station, server.sender(session), closed);
              });
    } catch (IOException e) {
      System.err.println("server: cannot open the control socket " + controlPath + ": " + e);
      server.close();
      return 3;
    }

    CountDownLatch stop = Signals.onTermination();
    Files.writeString(readyFile, "ready\n");
    Out.event("ready", null, "adapter", Out.ADAPTER, "version", version, "port", port, "fixture", fixture.name,
        "securityProfile", profile);
    stop.await();

    Files.deleteIfExists(readyFile);
    control.close();
    server.close();
    Out.event("stopped", null);
    return 0;
  }

  private static boolean listening(String bindAddress, int port, long timeoutMs) throws InterruptedException {
    String host = bindAddress.equals("0.0.0.0") || bindAddress.equals("::") ? "127.0.0.1" : bindAddress;
    long deadline = System.currentTimeMillis() + timeoutMs;
    while (System.currentTimeMillis() < deadline) {
      try (Socket socket = new Socket()) {
        socket.connect(new InetSocketAddress(host, port), 500);
        return true;
      } catch (IOException e) {
        Thread.sleep(50);
      }
    }
    return false;
  }
}
