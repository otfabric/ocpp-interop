// SPDX-License-Identifier: MIT
package org.otfabric.ocppinterop.chargetime;

import com.google.gson.Gson;
import com.google.gson.GsonBuilder;
import com.google.gson.JsonElement;
import com.google.gson.JsonNull;
import com.google.gson.JsonObject;
import java.io.PrintStream;
import java.nio.charset.StandardCharsets;
import java.time.Instant;
import java.time.temporal.ChronoUnit;

/** JSON on stdout: events (JSON Lines) and result documents. Nothing else goes to stdout. */
final class Out {
  static final String ADAPTER = "chargetime";
  static final String SCHEMA_VERSION = "1.0";
  static final String CONTRACT_VERSION = "0.5";

  /** Glue only: the adapter's own documents. OCPP payloads go through the upstream's Gson. */
  static final Gson GSON = new GsonBuilder().disableHtmlEscaping().serializeNulls().create();

  private static final PrintStream STDOUT =
      new PrintStream(new java.io.FileOutputStream(java.io.FileDescriptor.out), true, StandardCharsets.UTF_8);

  private Out() {}

  static String now() {
    return Instant.now().truncatedTo(ChronoUnit.MILLIS).toString();
  }

  static synchronized void line(JsonObject document) {
    STDOUT.print(GSON.toJson(document) + "\n");
    STDOUT.flush();
  }

  /** One line of text: the {@code version} command. */
  static synchronized void lineText(String text) {
    STDOUT.print(text + "\n");
    STDOUT.flush();
  }

  /** Bytes as they are: the {@code print-fixture} command. */
  static synchronized void raw(byte[] data) {
    STDOUT.write(data, 0, data.length);
    STDOUT.flush();
  }

  /** An event. {@code fields} alternates names and values (String, Number, Boolean, JsonElement or null). */
  static void event(String name, String station, Object... fields) {
    JsonObject document = new JsonObject();
    document.addProperty("event", name);
    document.addProperty("time", now());
    if (station != null) {
      document.addProperty("station", station);
    }
    for (int i = 0; i + 1 < fields.length; i += 2) {
      document.add((String) fields[i], element(fields[i + 1]));
    }
    line(document);
  }

  static JsonElement element(Object value) {
    if (value == null) {
      return JsonNull.INSTANCE;
    }
    if (value instanceof JsonElement) {
      return (JsonElement) value;
    }
    return GSON.toJsonTree(value);
  }

  /** A result document (schemas/client-result.schema.json). */
  static JsonObject result(
      String version,
      String operation,
      boolean ok,
      JsonElement request,
      JsonElement response,
      JsonObject callError,
      String errorCode,
      String errorMessage,
      long elapsedMs) {
    JsonObject document = new JsonObject();
    document.addProperty("schemaVersion", SCHEMA_VERSION);
    document.addProperty("adapter", ADAPTER);
    document.addProperty("version", version);
    document.addProperty("operation", operation);
    document.addProperty("ok", ok);
    // The upstream does not expose the message id of a CALL it sends, so
    // "messageId" is left out (docs/CONTAINER_CONTRACT.md).
    if (request != null) {
      document.add("request", request);
    }
    document.add("response", response == null ? JsonNull.INSTANCE : response);
    document.add("callError", callError == null ? JsonNull.INSTANCE : callError);
    if (errorCode == null) {
      document.add("error", JsonNull.INSTANCE);
    } else {
      JsonObject error = new JsonObject();
      error.addProperty("code", errorCode);
      error.addProperty("message", errorMessage == null ? "" : errorMessage);
      document.add("error", error);
    }
    document.addProperty("elapsedMs", Math.max(0, elapsedMs));
    return document;
  }

  static int exitCode(JsonObject document) {
    JsonElement error = document.get("error");
    if (error == null || error.isJsonNull()) {
      return 0;
    }
    String code = error.getAsJsonObject().get("code").getAsString();
    switch (code) {
      case "connect-failed":
        return 3;
      case "unsupported":
        return 5;
      default:
        return 1;
    }
  }
}
