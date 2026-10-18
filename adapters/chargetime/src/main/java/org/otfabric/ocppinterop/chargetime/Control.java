// SPDX-License-Identifier: MIT
package org.otfabric.ocppinterop.chargetime;

import com.google.gson.JsonElement;
import com.google.gson.JsonObject;
import com.google.gson.JsonParser;
import java.io.BufferedReader;
import java.io.IOException;
import java.io.InputStreamReader;
import java.io.OutputStream;
import java.net.StandardProtocolFamily;
import java.net.UnixDomainSocketAddress;
import java.nio.channels.Channels;
import java.nio.channels.ServerSocketChannel;
import java.nio.channels.SocketChannel;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;

/**
 * The control socket (docs/CONTAINER_CONTRACT.md, {@code call}): one request line in, one result
 * document line out.
 *
 * <pre>{"station": "ID" | null, "action": "Reset", "payload": {...}, "timeoutMs": 10000}</pre>
 */
final class Control {
  /** Executes one control request and returns the result document. */
  interface Handler {
    JsonObject handle(String station, String action, JsonObject payload, long timeoutMs);
  }

  private final ServerSocketChannel channel;
  private final Path path;

  private Control(ServerSocketChannel channel, Path path) {
    this.channel = channel;
    this.path = path;
  }

  static Control serve(Path path, Handler handler) throws IOException {
    Files.deleteIfExists(path);
    if (path.getParent() != null) {
      Files.createDirectories(path.getParent());
    }
    ServerSocketChannel channel = ServerSocketChannel.open(StandardProtocolFamily.UNIX);
    channel.bind(UnixDomainSocketAddress.of(path));
    Control control = new Control(channel, path);
    Thread acceptor =
        new Thread(
            () -> {
              while (channel.isOpen()) {
                try {
                  SocketChannel connection = channel.accept();
                  Thread worker = new Thread(() -> serveOne(connection, handler), "control");
                  worker.setDaemon(true);
                  worker.start();
                } catch (IOException e) {
                  return;
                }
              }
            },
            "control-accept");
    acceptor.setDaemon(true);
    acceptor.start();
    return control;
  }

  private static void serveOne(SocketChannel connection, Handler handler) {
    try (connection) {
      BufferedReader in =
          new BufferedReader(new InputStreamReader(Channels.newInputStream(connection), StandardCharsets.UTF_8));
      JsonObject request = JsonParser.parseString(in.readLine()).getAsJsonObject();
      JsonElement station = request.get("station");
      JsonElement payload = request.get("payload");
      JsonObject document =
          handler.handle(
              station == null || station.isJsonNull() ? null : station.getAsString(),
              request.get("action").getAsString(),
              payload == null || payload.isJsonNull() ? new JsonObject() : payload.getAsJsonObject(),
              request.has("timeoutMs") ? request.get("timeoutMs").getAsLong() : 10000);
      OutputStream out = Channels.newOutputStream(connection);
      out.write((Out.GSON.toJson(document) + "\n").getBytes(StandardCharsets.UTF_8));
      out.flush();
    } catch (Exception e) {
      System.err.println("control socket: " + e);
    }
  }

  void close() {
    try {
      channel.close();
      Files.deleteIfExists(path);
    } catch (IOException e) {
      // nothing left to do
    }
  }

  /** The {@code call} command. */
  static int call(Path path, String station, String action, JsonObject payload, long timeoutMs) {
    JsonObject request = new JsonObject();
    request.addProperty("station", station);
    request.addProperty("action", action);
    request.add("payload", payload);
    request.addProperty("timeoutMs", timeoutMs);
    String line;
    try (SocketChannel connection = SocketChannel.open(StandardProtocolFamily.UNIX)) {
      connection.connect(UnixDomainSocketAddress.of(path));
      OutputStream out = Channels.newOutputStream(connection);
      out.write((Out.GSON.toJson(request) + "\n").getBytes(StandardCharsets.UTF_8));
      out.flush();
      BufferedReader in =
          new BufferedReader(new InputStreamReader(Channels.newInputStream(connection), StandardCharsets.UTF_8));
      line = in.readLine();
    } catch (IOException e) {
      System.err.println("call: no running server or client run on " + path + ": " + e);
      return 3;
    }
    if (line == null) {
      System.err.println("call: the running process did not answer");
      return 1;
    }
    JsonObject document = JsonParser.parseString(line).getAsJsonObject();
    Out.line(document);
    return Out.exitCode(document);
  }
}
