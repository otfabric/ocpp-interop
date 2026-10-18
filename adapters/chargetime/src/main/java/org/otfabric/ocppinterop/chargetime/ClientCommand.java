// SPDX-License-Identifier: MIT
package org.otfabric.ocppinterop.chargetime;

import com.google.gson.JsonObject;
import eu.chargetime.ocpp.ClientEvents;
import eu.chargetime.ocpp.JSONClient;
import eu.chargetime.ocpp.JSONConfiguration;
import eu.chargetime.ocpp.feature.profile.ClientCoreEventHandler;
import eu.chargetime.ocpp.feature.profile.ClientCoreProfile;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.HashSet;
import java.util.List;
import java.util.Set;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;

/**
 * {@code client run} and {@code client <Action>}: a station. The upstream's {@link JSONClient}
 * (OCPP 1.6 JSON over Java-WebSocket) with the Core profile and the fixture's profile.
 */
final class ClientCommand {
  private ClientCommand() {}

  /** An open connection. */
  private static final class Connection {
    JSONClient client;
    final CompletableFuture<Void> opened = new CompletableFuture<>();
    final CompletableFuture<Void> closed = new CompletableFuture<>();
  }

  static int run(List<String> args) throws Exception {
    if (args.isEmpty() || args.get(0).startsWith("--")) {
      throw new Main.UsageException("client needs 'run' or an action");
    }
    String mode = args.get(0);
    boolean run = mode.equals("run");
    Set<String> known =
        new HashSet<>(Set.of("version", "url", "station", "fixture", "security-profile", "tls-dir",
            "connect-timeout-ms", "timeout-ms"));
    known.addAll(run ? Set.of("hold-ms", "ready-file", "control-socket") : Set.of("payload"));
    Main.Flags flags = Main.flags(args.subList(1, args.size()), known);
    if (!flags.positional.isEmpty()) {
      throw new Main.UsageException("unexpected argument '" + flags.positional.get(0) + "'");
    }
    String action = run ? null : Main.action(mode);
    if (!flags.has("url")) {
      throw new Main.UsageException("--url is required");
    }
    String url = flags.get("url", "");
    if (!url.startsWith("ws://") && !url.startsWith("wss://")) {
      throw new Main.UsageException("--url must start with ws:// or wss://");
    }
    long connectTimeoutMs = flags.number("connect-timeout-ms", 5000, 1, 3600000);
    long timeoutMs = flags.number("timeout-ms", 10000, 1, 3600000);
    long holdMs = run ? flags.number("hold-ms", 0, 0, Integer.MAX_VALUE) : 0;
    JsonObject payload = run ? new JsonObject() : Main.payload(flags);
    String version = Main.version(flags);
    int profile = Main.securityProfile(flags);
    if (url.startsWith("wss://")) {
      throw new Main.UnsupportedException("wss:// (security profiles 2 and 3) is not supported by this adapter yet");
    }
    Fixture fixture = Main.fixture(flags, version);
    String station = flags.get("station", fixture.firstStation);
    if (profile == 1 && !fixture.stations.containsKey(station)) {
      throw new Main.UsageException(
          "--station '" + station + "' is not in the fixture: security profile 1 needs its password");
    }
    String fullUrl = station.isEmpty() ? url : url.replaceAll("/+$", "") + "/" + station;

    if (run) {
      return clientRun(version, url, fullUrl, station, fixture, profile, connectTimeoutMs, holdMs,
          Path.of(flags.get("ready-file", Main.READY_FILE)),
          Path.of(flags.get("control-socket", Main.CONTROL_SOCKET)));
    }
    if (Registry.feature(action, null) == null) {
      Out.line(Out.result(version, action, false, payload, null, null, "unsupported",
          "Java-OCA-OCPP has no OCPP " + version + " feature for " + action, 0));
      return 5;
    }
    long started = System.nanoTime();
    Endpoint endpoint = new Endpoint(version, fixture, "station", session -> station, false);
    Connection connection = connect(endpoint, url, station, fixture, profile, connectTimeoutMs);
    JsonObject document;
    if (!connection.opened.isDone()) {
      document = Out.result(version, action, false, payload, null, null, "connect-failed",
          "the WebSocket handshake with " + fullUrl + " failed or was refused (the upstream logs why on stderr)",
          TimeUnit.NANOSECONDS.toMillis(System.nanoTime() - started));
    } else {
      document = endpoint.execute(action, payload, timeoutMs, station, connection.client::send, connection.closed);
      disconnect(connection);
      document.addProperty("elapsedMs", TimeUnit.NANOSECONDS.toMillis(System.nanoTime() - started));
    }
    Out.line(document);
    return Out.exitCode(document);
  }

  /** Connects with the upstream; {@code opened} tells whether it worked. */
  private static Connection connect(
      Endpoint endpoint, String url, String station, Fixture fixture, int profile, long connectTimeoutMs)
      throws InterruptedException {
    JSONConfiguration configuration =
        JSONConfiguration.get().setParameter(JSONConfiguration.CONNECT_TIMEOUT_IN_MS_PARAMETER, (int) connectTimeoutMs);
    if (profile == 1) {
      configuration
          .setParameter(JSONConfiguration.USERNAME_PARAMETER, station)
          .setParameter(JSONConfiguration.PASSWORD_PARAMETER, fixture.password(station));
    }
    Connection connection = new Connection();
    connection.client =
        new JSONClient(
            new ClientCoreProfile(endpoint.handler(ClientCoreEventHandler.class)),
            station.isEmpty() ? null : station,
            configuration);
    connection.client.addFeatureProfile(endpoint);
    // JSONClient.connect() blocks until the handshake succeeds or fails, and
    // reports neither (eu.chargetime.ocpp.WebSocketTransmitter); the events
    // say whether it opened. It runs in its own thread so that a peer that
    // never answers the handshake is bounded by the connect timeout too.
    Thread connector =
        new Thread(
            () ->
                connection.client.connect(
                    url.replaceAll("/+$", ""),
                    new ClientEvents() {
                      @Override
                      public void connectionOpened() {
                        connection.opened.complete(null);
                      }

                      @Override
                      public void connectionClosed() {
                        connection.closed.complete(null);
                      }
                    }),
            "connect");
    connector.setDaemon(true);
    connector.start();
    connector.join(connectTimeoutMs + 1000);
    return connection;
  }

  private static void disconnect(Connection connection) {
    Thread closer = new Thread(connection.client::disconnect, "disconnect");
    closer.setDaemon(true);
    closer.start();
    try {
      closer.join(5000);
    } catch (InterruptedException e) {
      Thread.currentThread().interrupt();
    }
  }

  private static int clientRun(
      String version, String url, String fullUrl, String station, Fixture fixture, int profile,
      long connectTimeoutMs, long holdMs, Path readyFile, Path controlPath) throws Exception {
    CountDownLatch stop = Signals.onTermination();
    Endpoint endpoint = new Endpoint(version, fixture, "station", session -> station, true);
    Connection connection = connect(endpoint, url, station, fixture, profile, connectTimeoutMs);
    if (!connection.opened.isDone()) {
      Out.event("connection-failed", station, "url", fullUrl,
          "detail", "the WebSocket handshake failed or was refused (the upstream logs why on stderr)");
      Out.event("stopped", null);
      return 3;
    }
    Out.event("connection-opened", station, "subprotocol", null, "remote", fullUrl);
    connection.closed.thenRun(stop::countDown);

    Control control;
    try {
      control =
          Control.serve(
              controlPath,
              (wanted, action, payload, timeoutMs) -> {
                if ((wanted != null && !wanted.isEmpty() && !wanted.equals(station)) || connection.closed.isDone()) {
                  return Out.result(version, action, false, payload, null, null, "not-connected",
                      "no open connection for station " + wanted, 0);
                }
                return endpoint.execute(action, payload, timeoutMs, station, connection.client::send, connection.closed);
              });
    } catch (IOException e) {
      System.err.println("client: cannot open the control socket " + controlPath + ": " + e);
      disconnect(connection);
      return 3;
    }
    Files.writeString(readyFile, "ready\n");
    Out.event("ready", station, "version", version, "url", fullUrl);

    if (holdMs > 0) {
      stop.await(holdMs, TimeUnit.MILLISECONDS);
    } else {
      stop.await();
    }
    boolean lost = connection.closed.isDone();
    Files.deleteIfExists(readyFile);
    control.close();
    if (!lost) {
      disconnect(connection);
    }
    Out.event("connection-closed", station, "code", null, "reason", null);
    Out.event("stopped", null);
    return lost ? 1 : 0;
  }
}
