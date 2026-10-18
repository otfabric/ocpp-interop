// SPDX-License-Identifier: MIT
package org.otfabric.ocppinterop.chargetime;

import com.google.gson.JsonArray;
import com.google.gson.JsonElement;
import com.google.gson.JsonObject;
import com.google.gson.JsonParser;
import com.google.gson.JsonPrimitive;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.time.temporal.ChronoUnit;
import java.util.ArrayList;
import java.util.HashSet;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Locale;
import java.util.Map;
import java.util.Set;
import java.util.regex.Pattern;

/**
 * Fixture loading, validation and answers (docs/FIXTURES.md).
 *
 * <p>The checks are those of fixtures/schema/ocpp-fixture.schema.json and of
 * scripts/validate-fixtures.py that matter for OCPP 1.6, written out by hand so that the image
 * needs no JSON Schema library.
 */
final class Fixture {
  static final Path FIXTURES = Path.of("/fixtures");
  static final Map<String, String> VERSION_DIRS = Map.of("1.6", "v16", "2.0.1", "v201", "2.1", "v21");

  private static final Pattern NAME = Pattern.compile("^[a-z0-9][a-z0-9-]*$");
  private static final Pattern ACTION = Pattern.compile("^[A-Z][A-Za-z0-9]+$");
  private static final Pattern IDENTITY = Pattern.compile("^[A-Za-z0-9*\\-_=:+|@.]{1,48}$");
  private static final Set<String> TOKEN_STATUSES =
      Set.of(
          "Accepted", "Blocked", "Expired", "Invalid", "ConcurrentTx", "NoCredit",
          "NotAllowedTypeEVSE", "NotAtThisLocation", "NotAtThisTime", "Unknown");

  /** The fixture is missing or invalid: exit 4. */
  static final class FixtureException extends Exception {
    private static final long serialVersionUID = 1L;

    FixtureException(String message) {
      super(message);
    }
  }

  /** The fixture asks for something the upstream cannot do: exit 5. */
  static final class UnsupportedFixtureException extends Exception {
    private static final long serialVersionUID = 1L;

    UnsupportedFixtureException(String message) {
      super(message);
    }
  }

  final JsonObject doc;
  final String name;
  final String version;
  final Map<String, JsonObject> stations = new LinkedHashMap<>();
  final String firstStation;
  private final Map<String, String> idTokens = new LinkedHashMap<>();
  private final List<JsonObject> configuration = new ArrayList<>();

  private Fixture(JsonObject doc) {
    this.doc = doc;
    this.name = doc.get("name").getAsString();
    this.version = doc.get("ocppVersion").getAsString();
    for (JsonElement s : doc.getAsJsonArray("stations")) {
      stations.put(s.getAsJsonObject().get("id").getAsString(), s.getAsJsonObject());
    }
    this.firstStation = stations.keySet().iterator().next();
    for (JsonElement t : doc.getAsJsonArray("idTokens")) {
      JsonObject token = t.getAsJsonObject();
      idTokens.put(token.get("idToken").getAsString(), token.get("status").getAsString());
    }
    if (doc.has("configuration")) {
      for (JsonElement c : doc.getAsJsonArray("configuration")) {
        configuration.add(c.getAsJsonObject());
      }
    }
  }

  static Path defaultPath(String version) {
    return FIXTURES.resolve(VERSION_DIRS.get(version)).resolve("baseline").resolve("fixture.json");
  }

  JsonObject answers(String role) {
    return doc.getAsJsonObject("answers").getAsJsonObject(role);
  }

  String password(String station) {
    JsonObject s = stations.get(station);
    return s == null ? null : s.get("password").getAsString();
  }

  /** The answer of {@code role} to a CALL, or null when the fixture has none. */
  JsonObject answer(String role, String action, JsonObject request) {
    JsonElement entry = answers(role).get(action);
    if (entry == null) {
      return null;
    }
    JsonObject e = entry.getAsJsonObject();
    JsonObject answer;
    if (!e.has("derived")) {
      return substitute(e.getAsJsonObject("payload").deepCopy()).getAsJsonObject();
    }
    switch (e.get("derived").getAsString()) {
      case "idTokens":
        {
          // 1.6: idTagInfo with the status of the table; an unknown token is Invalid.
          JsonElement tag = request.get("idTag");
          String status =
              tag != null && tag.isJsonPrimitive() ? idTokens.getOrDefault(tag.getAsString(), "Invalid") : "Invalid";
          JsonObject info = new JsonObject();
          info.addProperty("status", status);
          answer = new JsonObject();
          answer.add("idTagInfo", info);
          break;
        }
      case "configuration":
        answer = configurationAnswer(request);
        break;
      default:
        throw new IllegalStateException("derived answer " + e.get("derived") + " is not for OCPP " + version);
    }
    if (e.has("payload")) {
      for (Map.Entry<String, JsonElement> field : substitute(e.getAsJsonObject("payload").deepCopy()).getAsJsonObject().entrySet()) {
        answer.add(field.getKey(), field.getValue());
      }
    }
    return answer;
  }

  /** GetConfiguration: the requested keys, or all of them; keys compare case-insensitively. */
  private JsonObject configurationAnswer(JsonObject request) {
    Map<String, JsonObject> byKey = new LinkedHashMap<>();
    for (JsonObject c : configuration) {
      byKey.put(c.get("key").getAsString().toLowerCase(Locale.ROOT), c);
    }
    List<String> wanted = new ArrayList<>();
    JsonElement keys = request.get("key");
    if (keys != null && keys.isJsonArray() && keys.getAsJsonArray().size() > 0) {
      for (JsonElement k : keys.getAsJsonArray()) {
        wanted.add(k.getAsString());
      }
    } else {
      for (JsonObject c : configuration) {
        wanted.add(c.get("key").getAsString());
      }
    }
    JsonArray known = new JsonArray();
    JsonArray unknown = new JsonArray();
    for (String key : wanted) {
      JsonObject item = byKey.get(key.toLowerCase(Locale.ROOT));
      if (item == null) {
        unknown.add(key);
      } else {
        JsonObject kv = new JsonObject();
        kv.add("key", item.get("key"));
        kv.add("readonly", item.get("readonly"));
        kv.add("value", item.get("value"));
        known.add(kv);
      }
    }
    JsonObject answer = new JsonObject();
    answer.add("configurationKey", known);
    if (unknown.size() > 0) {
      answer.add("unknownKey", unknown);
    }
    return answer;
  }

  /** "${now}" becomes the current UTC time, RFC 3339, whole seconds. */
  private static JsonElement substitute(JsonElement value) {
    if (value.isJsonPrimitive() && value.getAsJsonPrimitive().isString() && value.getAsString().equals("${now}")) {
      return new JsonPrimitive(Instant.now().truncatedTo(ChronoUnit.SECONDS).toString());
    }
    if (value.isJsonObject()) {
      JsonObject copy = new JsonObject();
      for (Map.Entry<String, JsonElement> e : value.getAsJsonObject().entrySet()) {
        copy.add(e.getKey(), substitute(e.getValue()));
      }
      return copy;
    }
    if (value.isJsonArray()) {
      JsonArray copy = new JsonArray();
      for (JsonElement e : value.getAsJsonArray()) {
        copy.add(substitute(e));
      }
      return copy;
    }
    return value;
  }

  // ---- loading and validation ----

  static Fixture load(Path path, String version) throws FixtureException, UnsupportedFixtureException {
    JsonObject doc;
    try {
      JsonElement parsed = JsonParser.parseString(Files.readString(path, StandardCharsets.UTF_8));
      if (!parsed.isJsonObject()) {
        throw new FixtureException(path + ": not a JSON object");
      }
      doc = parsed.getAsJsonObject();
    } catch (java.io.IOException | RuntimeException e) {
      throw new FixtureException(path + ": " + e.getMessage());
    }
    List<String> errors = new ArrayList<>();
    validate(doc, path, version, errors);
    if (!errors.isEmpty()) {
      throw new FixtureException(path + ": " + String.join("; ", errors));
    }
    // The upstream answers a CALL with a CALLERROR only on its own terms: a
    // handler that fails gives InternalError, whatever the fixture says.
    for (String role : List.of("csms", "station")) {
      for (Map.Entry<String, JsonElement> a : doc.getAsJsonObject("answers").getAsJsonObject(role).entrySet()) {
        if (a.getValue().getAsJsonObject().has("callError")) {
          throw new UnsupportedFixtureException(
              path + ": answers." + role + "." + a.getKey()
                  + " is a callError; Java-OCA-OCPP cannot answer a CALL with a chosen CALLERROR");
        }
      }
    }
    return new Fixture(doc);
  }

  private static boolean isString(JsonObject o, String key) {
    return o.has(key) && o.get(key).isJsonPrimitive() && o.get(key).getAsJsonPrimitive().isString();
  }

  private static boolean isInt(JsonObject o, String key) {
    if (!o.has(key) || !o.get(key).isJsonPrimitive() || !o.get(key).getAsJsonPrimitive().isNumber()) {
      return false;
    }
    double d = o.get(key).getAsDouble();
    return d == Math.rint(d);
  }

  private static void validate(JsonObject doc, Path path, String version, List<String> errors) {
    Set<String> allowed =
        Set.of("schemaVersion", "name", "ocppVersion", "description", "stations", "idTokens",
            "configuration", "variables", "answers", "samples");
    for (String key : doc.keySet()) {
      if (!allowed.contains(key)) {
        errors.add("unknown property " + key);
      }
    }
    if (!isString(doc, "schemaVersion") || !doc.get("schemaVersion").getAsString().equals("1.0")) {
      errors.add("schemaVersion must be \"1.0\"");
    }
    if (!isString(doc, "name") || !NAME.matcher(doc.get("name").getAsString()).matches()) {
      errors.add("name is missing or not a fixture name");
    }
    if (!isString(doc, "description") || doc.get("description").getAsString().isEmpty()) {
      errors.add("description is missing");
    }
    if (!isString(doc, "ocppVersion") || !VERSION_DIRS.containsKey(doc.get("ocppVersion").getAsString())) {
      errors.add("ocppVersion must be 1.6, 2.0.1 or 2.1");
      return;
    }
    String fixtureVersion = doc.get("ocppVersion").getAsString();
    if (!fixtureVersion.equals(version)) {
      errors.add("ocppVersion is " + fixtureVersion + ", but --version is " + version);
      return;
    }
    if (!"1.6".equals(version)) {
      errors.add("only OCPP 1.6 fixtures are checked by this adapter");
      return;
    }
    Path dir = path.toAbsolutePath().normalize().getParent();
    if (isString(doc, "name") && (dir == null || !dir.getFileName().toString().equals(doc.get("name").getAsString()))) {
      errors.add("name is not the directory name");
    }
    if (dir == null || dir.getParent() == null || !dir.getParent().getFileName().toString().equals("v16")) {
      errors.add("ocppVersion 1.6 does not belong in this directory");
    }
    if (doc.has("variables")) {
      errors.add("variables are for OCPP 2.x");
    }

    // stations
    if (!doc.has("stations") || !doc.get("stations").isJsonArray() || doc.getAsJsonArray("stations").size() == 0) {
      errors.add("stations must be a non-empty array");
    } else {
      Set<String> ids = new HashSet<>();
      for (JsonElement s : doc.getAsJsonArray("stations")) {
        if (!s.isJsonObject()) {
          errors.add("stations: not an object");
          continue;
        }
        JsonObject st = s.getAsJsonObject();
        for (String key : List.of("id", "vendor", "model", "serialNumber", "firmwareVersion", "password")) {
          if (!isString(st, key)) {
            errors.add("stations: " + key + " is missing");
          }
        }
        if (isString(st, "id")) {
          String id = st.get("id").getAsString();
          if (!IDENTITY.matcher(id).matches()) {
            errors.add("stations: identity " + id + " is not valid");
          }
          if (!ids.add(id)) {
            errors.add("stations: identity " + id + " appears twice");
          }
        }
        if (isString(st, "password")) {
          int n = st.get("password").getAsString().length();
          if (n < 16 || n > 40) {
            errors.add("stations: password must have 16 to 40 characters");
          }
        }
        if (!isInt(st, "connectors") || st.get("connectors").getAsInt() < 1 || st.get("connectors").getAsInt() > 16) {
          errors.add("stations: connectors must be 1..16 in OCPP 1.6");
        }
        if (st.has("evses")) {
          errors.add("stations: evses are for OCPP 2.x");
        }
      }
    }

    // idTokens
    if (!doc.has("idTokens") || !doc.get("idTokens").isJsonArray() || doc.getAsJsonArray("idTokens").size() == 0) {
      errors.add("idTokens must be a non-empty array");
    } else {
      Set<String> seen = new HashSet<>();
      for (JsonElement t : doc.getAsJsonArray("idTokens")) {
        JsonObject token = t.isJsonObject() ? t.getAsJsonObject() : new JsonObject();
        if (!isString(token, "idToken") || !isString(token, "status")) {
          errors.add("idTokens: idToken and status are required");
          continue;
        }
        String id = token.get("idToken").getAsString();
        if (id.isEmpty() || id.length() > 20) {
          errors.add("idTokens: " + id + " must have 1 to 20 characters in OCPP 1.6");
        }
        if (!seen.add(id)) {
          errors.add("idTokens: " + id + " appears twice");
        }
        if (!TOKEN_STATUSES.contains(token.get("status").getAsString())) {
          errors.add("idTokens: " + id + " has an unknown status");
        }
      }
    }

    // configuration
    if (!doc.has("configuration") || !doc.get("configuration").isJsonArray()) {
      errors.add("configuration is required in OCPP 1.6");
    } else {
      Set<String> keys = new HashSet<>();
      for (JsonElement c : doc.getAsJsonArray("configuration")) {
        JsonObject item = c.isJsonObject() ? c.getAsJsonObject() : new JsonObject();
        if (!isString(item, "key") || !isString(item, "value") || !item.has("readonly")
            || !item.get("readonly").isJsonPrimitive() || !item.get("readonly").getAsJsonPrimitive().isBoolean()) {
          errors.add("configuration: key, value and readonly are required");
          continue;
        }
        if (item.get("key").getAsString().length() > 50 || item.get("value").getAsString().length() > 500) {
          errors.add("configuration: " + item.get("key").getAsString() + " is too long");
        }
        if (!keys.add(item.get("key").getAsString())) {
          errors.add("configuration: key " + item.get("key").getAsString() + " appears twice");
        }
      }
    }

    // answers: given by the role that receives the action
    if (!doc.has("answers") || !doc.get("answers").isJsonObject()) {
      errors.add("answers is required");
    } else {
      JsonObject answers = doc.getAsJsonObject("answers");
      for (String role : List.of("csms", "station")) {
        if (!answers.has(role) || !answers.get(role).isJsonObject()) {
          errors.add("answers." + role + " is required");
          continue;
        }
        Set<String> receives = role.equals("csms") ? Registry.FROM_STATION : Registry.FROM_CSMS;
        for (Map.Entry<String, JsonElement> a : answers.getAsJsonObject(role).entrySet()) {
          String where = "answers." + role + "." + a.getKey();
          if (!receives.contains(a.getKey())) {
            errors.add(where + ": the " + role + " does not receive " + a.getKey() + " in OCPP 1.6");
          }
          if (!a.getValue().isJsonObject()) {
            errors.add(where + ": not an object");
            continue;
          }
          JsonObject e = a.getValue().getAsJsonObject();
          int forms = (e.has("callError") ? 1 : 0) + (e.has("derived") ? 1 : 0)
              + (e.has("payload") && !e.has("derived") ? 1 : 0);
          if (forms != 1) {
            errors.add(where + ": must be exactly one of payload, derived or callError");
            continue;
          }
          if (e.has("payload") && !e.get("payload").isJsonObject()) {
            errors.add(where + ": payload must be an object");
          }
          if (e.has("callError")) {
            JsonObject ce = e.get("callError").isJsonObject() ? e.getAsJsonObject("callError") : new JsonObject();
            if (!isString(ce, "code") || !isString(ce, "description")) {
              errors.add(where + ": callError needs code and description");
            }
          }
          if (e.has("derived")) {
            String derived = isString(e, "derived") ? e.get("derived").getAsString() : "";
            boolean fits =
                (derived.equals("idTokens") && Set.of("Authorize", "StartTransaction").contains(a.getKey()))
                    || (derived.equals("configuration") && a.getKey().equals("GetConfiguration"));
            if (!fits) {
              errors.add(where + ": cannot be derived from " + derived);
            }
            if (derived.equals("configuration")
                && (!doc.has("configuration") || !doc.get("configuration").isJsonArray()
                    || doc.getAsJsonArray("configuration").size() == 0)) {
              errors.add(where + ": derived from configuration, which is empty");
            }
          }
        }
      }
    }

    // samples: sent by the role that sends the action
    if (!doc.has("samples") || !doc.get("samples").isJsonObject()) {
      errors.add("samples is required");
    } else {
      for (Map.Entry<String, JsonElement> s : doc.getAsJsonObject("samples").entrySet()) {
        JsonObject sample = s.getValue().isJsonObject() ? s.getValue().getAsJsonObject() : new JsonObject();
        if (!isString(sample, "action") || !isString(sample, "from") || !sample.has("payload")
            || !sample.get("payload").isJsonObject() || !ACTION.matcher(sample.get("action").getAsString()).matches()) {
          errors.add("samples." + s.getKey() + ": action, from and payload are required");
          continue;
        }
        String from = sample.get("from").getAsString();
        Set<String> sends = from.equals("station") ? Registry.FROM_STATION : from.equals("csms") ? Registry.FROM_CSMS : Set.of();
        if (!sends.contains(sample.get("action").getAsString())) {
          errors.add("samples." + s.getKey() + ": the " + from + " does not send " + sample.get("action").getAsString());
        }
      }
    }
  }
}
