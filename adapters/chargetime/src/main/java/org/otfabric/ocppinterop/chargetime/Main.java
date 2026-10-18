// SPDX-License-Identifier: MIT
package org.otfabric.ocppinterop.chargetime;

import com.google.gson.JsonArray;
import com.google.gson.JsonElement;
import com.google.gson.JsonObject;
import com.google.gson.JsonParser;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.TreeSet;
import java.util.regex.Pattern;

/** Command line of the adapter: the container contract (docs/CONTAINER_CONTRACT.md). */
public final class Main {
  static final String READY_FILE = "/run/ocpp-interop/ready";
  static final String CONTROL_SOCKET = "/run/ocpp-interop/control.sock";
  static final Set<String> SUPPORTED_VERSIONS = Set.of("1.6", "2.0.1");
  private static final Path BUILD_INFO = Path.of("/app/build-info.json");
  private static final Pattern NAME = Pattern.compile("^[a-z0-9][a-z0-9-]*$");
  private static final Pattern ACTION = Pattern.compile("^[A-Z][A-Za-z0-9]+$");

  static final String USAGE =
      String.join(
          "\n",
          "usage:",
          "  ocpp-interop server --version V [--fixture PATH] [--bind-address ADDR] [--port N]",
          "                      [--security-profile N] [--tls-dir DIR] [--ready-file PATH] [--control-socket PATH]",
          "  ocpp-interop client run --version V --url URL [--station ID] [--fixture PATH] [--security-profile N]",
          "                      [--tls-dir DIR] [--connect-timeout-ms N] [--timeout-ms N] [--hold-ms N]",
          "                      [--ready-file PATH] [--control-socket PATH]",
          "  ocpp-interop client <Action> --version V --url URL [--station ID] [--fixture PATH] [--payload JSON]",
          "                      [--security-profile N] [--tls-dir DIR] [--connect-timeout-ms N] [--timeout-ms N]",
          "  ocpp-interop call [--station ID] <Action> [--payload JSON] [--timeout-ms N] [--control-socket PATH]",
          "  ocpp-interop print-capabilities",
          "  ocpp-interop print-fixture --version V [NAME]",
          "  ocpp-interop version");

  // What the self-tests exercise (scripts/smoke.sh, scripts/interop.sh), per
  // version: every action of the version in the direction the specification
  // gives it (Registry), for each of which Java-OCA-OCPP has a feature, less
  // NOT_SENT. The baseline fixtures have a sample and an answer for each.
  // Nothing else is declared (REQUIREMENTS.md R4.2).
  /** Actions the upstream has a feature for but cannot send as the fixture's sample asks (README.md). */
  static final Map<String, Set<String>> NOT_SENT = Map.of("1.6", Set.of(), "2.0.1", Set.of());

  /** The actions a role sends in a version, sorted. */
  static Set<String> declaredSends(String version, boolean station) {
    Set<String> out = new TreeSet<>();
    for (String action : station ? Registry.fromStation(version) : Registry.fromCsms(version)) {
      if (Registry.feature(version, action, null) != null && !NOT_SENT.get(version).contains(action)) {
        out.add(action);
      }
    }
    return out;
  }

  /** Exit 2: nothing was sent. */
  static final class UsageException extends Exception {
    private static final long serialVersionUID = 1L;

    UsageException(String message) {
      super(message);
    }
  }

  /** Exit 5: a version, role, profile or action this adapter does not support. */
  static final class UnsupportedException extends Exception {
    private static final long serialVersionUID = 1L;

    UnsupportedException(String message) {
      super(message);
    }
  }

  private Main() {}

  public static void main(String[] argv) {
    // stdout is for the contract's JSON only (Out writes to the file
    // descriptor directly); anything a library prints goes to stderr.
    System.setOut(System.err);
    int code;
    try {
      code = run(argv);
    } catch (UsageException e) {
      System.err.println("usage error: " + e.getMessage() + "\n" + USAGE);
      code = 2;
    } catch (UnsupportedException | Fixture.UnsupportedFixtureException e) {
      System.err.println("unsupported: " + e.getMessage());
      code = 5;
    } catch (Fixture.FixtureException e) {
      System.err.println("invalid fixture: " + e.getMessage());
      code = 4;
    } catch (Exception e) {
      e.printStackTrace();
      code = 1;
    }
    System.exit(code);
  }

  static int run(String[] argv) throws Exception {
    if (argv.length == 0) {
      throw new UsageException("no command");
    }
    List<String> args = List.of(argv).subList(1, argv.length);
    switch (argv[0]) {
      case "server":
        return ServerCommand.run(args);
      case "client":
        return ClientCommand.run(args);
      case "call":
        return call(args);
      case "print-capabilities":
        if (!args.isEmpty()) {
          throw new UsageException("print-capabilities takes no arguments");
        }
        Out.line(capabilities());
        return 0;
      case "print-fixture":
        return printFixture(args);
      case "version":
        if (!args.isEmpty()) {
          throw new UsageException("version takes no arguments");
        }
        JsonObject info = buildInfo();
        Out.lineText(
            "ocpp-interop-chargetime " + string(info, "adapterVersion", "dev") + " (Java-OCA-OCPP "
                + string(info, "upstreamVersion", "?") + ", contract " + Out.CONTRACT_VERSION + ")");
        return 0;
      default:
        throw new UsageException("unknown command '" + argv[0] + "'");
    }
  }

  // ---- argument parsing ----

  /** Parsed {@code --name value} flags and positional arguments. */
  static final class Flags {
    final Map<String, String> given = new LinkedHashMap<>();
    final List<String> positional = new ArrayList<>();

    String get(String name, String fallback) {
      return given.getOrDefault(name, fallback);
    }

    boolean has(String name) {
      return given.containsKey(name);
    }

    long number(String name, long fallback, long low, long high) throws UsageException {
      String text = given.get(name);
      if (text == null) {
        return fallback;
      }
      if (!text.matches("[0-9]{1,12}")) {
        throw new UsageException("--" + name + ": '" + text + "' is not a number");
      }
      long value = Long.parseLong(text);
      if (value < low || value > high) {
        throw new UsageException("--" + name + ": " + value + " is not within " + low + ".." + high);
      }
      return value;
    }
  }

  static Flags flags(List<String> args, Set<String> known) throws UsageException {
    Flags flags = new Flags();
    for (int i = 0; i < args.size(); i++) {
      String arg = args.get(i);
      if (!arg.startsWith("--")) {
        flags.positional.add(arg);
        continue;
      }
      String name = arg.substring(2);
      if (!known.contains(name)) {
        throw new UsageException("unknown flag '" + arg + "'");
      }
      if (i + 1 >= args.size()) {
        throw new UsageException(arg + " needs a value");
      }
      if (flags.given.containsKey(name)) {
        throw new UsageException(arg + " is given more than once");
      }
      flags.given.put(name, args.get(++i));
    }
    return flags;
  }

  static String version(Flags flags) throws UsageException, UnsupportedException {
    if (!flags.has("version")) {
      throw new UsageException("--version is required");
    }
    String version = flags.get("version", "");
    if (!Fixture.VERSION_DIRS.containsKey(version)) {
      throw new UsageException("--version: '" + version + "' is not 1.6, 2.0.1 or 2.1");
    }
    if (!SUPPORTED_VERSIONS.contains(version)) {
      throw new UnsupportedException("OCPP " + version + " is not supported by this adapter");
    }
    return version;
  }

  static int securityProfile(Flags flags) throws UsageException, UnsupportedException {
    int profile = (int) flags.number("security-profile", 0, 0, 3);
    if (flags.has("tls-dir") && profile < 2) {
      throw new UsageException("--tls-dir is for security profiles 2 and 3");
    }
    if (profile >= 2) {
      throw new UnsupportedException("security profile " + profile + " (TLS) is not supported by this adapter yet");
    }
    return profile;
  }

  static JsonObject payload(Flags flags) throws UsageException {
    JsonElement parsed;
    try {
      parsed = JsonParser.parseString(flags.get("payload", "{}"));
    } catch (RuntimeException e) {
      throw new UsageException("--payload is not JSON: " + e.getMessage());
    }
    if (!parsed.isJsonObject()) {
      throw new UsageException("--payload must be a JSON object");
    }
    return parsed.getAsJsonObject();
  }

  static String action(String text) throws UsageException {
    if (!ACTION.matcher(text).matches()) {
      throw new UsageException("'" + text + "' is not an OCPP action name");
    }
    return text;
  }

  static Fixture fixture(Flags flags, String version)
      throws Fixture.FixtureException, Fixture.UnsupportedFixtureException {
    String path = flags.get("fixture", Fixture.defaultPath(version).toString());
    return Fixture.load(Path.of(path), version);
  }

  // ---- commands ----

  private static int call(List<String> args) throws UsageException {
    Flags flags = flags(args, Set.of("station", "payload", "timeout-ms", "control-socket"));
    if (flags.positional.size() != 1) {
      throw new UsageException("exactly one action is required");
    }
    String action = action(flags.positional.get(0));
    JsonObject payload = payload(flags);
    long timeoutMs = flags.number("timeout-ms", 10000, 1, 3600000);
    return Control.call(
        Path.of(flags.get("control-socket", CONTROL_SOCKET)), flags.given.get("station"), action, payload, timeoutMs);
  }

  private static int printFixture(List<String> args) throws UsageException {
    Flags flags = flags(args, Set.of("version"));
    String version = flags.get("version", "");
    if (!Fixture.VERSION_DIRS.containsKey(version)) {
      throw new UsageException("--version 1.6, 2.0.1 or 2.1 is required");
    }
    if (flags.positional.size() > 1) {
      throw new UsageException("at most one fixture name");
    }
    String name = flags.positional.isEmpty() ? "baseline" : flags.positional.get(0);
    if (!NAME.matcher(name).matches()) {
      throw new UsageException("'" + name + "' is not a fixture name");
    }
    Path path = Fixture.FIXTURES.resolve(Fixture.VERSION_DIRS.get(version)).resolve(name).resolve("fixture.json");
    try {
      Out.raw(Files.readAllBytes(path));
    } catch (IOException e) {
      System.err.println("print-fixture: no fixture '" + name + "' for OCPP " + version + " in this image");
      return 1;
    }
    return 0;
  }

  static JsonObject buildInfo() {
    try {
      return JsonParser.parseString(Files.readString(BUILD_INFO, StandardCharsets.UTF_8)).getAsJsonObject();
    } catch (IOException | RuntimeException e) {
      return new JsonObject();
    }
  }

  private static String string(JsonObject o, String key, String fallback) {
    return o.has(key) ? o.get(key).getAsString() : fallback;
  }

  private static JsonArray array(Iterable<String> values) {
    JsonArray array = new JsonArray();
    values.forEach(array::add);
    return array;
  }

  private static JsonObject versionCapabilities(String version) {
    Set<String> stationSends = declaredSends(version, true);
    Set<String> csmsSends = declaredSends(version, false);
    // An end handles what the other end sends.
    Set<String> stationHandles = csmsSends;
    Set<String> csmsHandles = stationSends;

    JsonObject client = new JsonObject();
    client.addProperty("supported", true);
    client.add("sends", array(stationSends));
    client.add("handles", array(stationHandles));
    JsonObject server = new JsonObject();
    server.addProperty("supported", true);
    server.add("sends", array(csmsSends));
    server.add("handles", array(csmsHandles));
    JsonObject features = new JsonObject();
    JsonArray profiles = new JsonArray();
    profiles.add(0);
    profiles.add(1);
    features.add("securityProfiles", profiles);
    features.addProperty("compression", false);
    features.addProperty("send", false);
    features.addProperty("callResultError", false);
    features.addProperty("fixtureCallErrors", false);
    features.addProperty("messageIds", false);
    List<String> notes = new ArrayList<>();
    notes.add(
        "Java-OCA-OCPP answers a CALL with a CALLERROR only on its own terms (NotImplemented for an "
            + "action it has no feature for, InternalError when a handler fails), so a fixture with "
            + "callError answers is refused with exit 5.");
    if (version.equals("1.6")) {
      notes.add(
          "JSONServer and JSONClient always register the Core profile: a Core action without a fixture "
              + "answer is answered with InternalError, not NotImplemented.");
    } else {
      notes.add(
          "OCPP 2.0.1 runs on MultiProtocolJSONServer and MultiProtocolJSONClient (artifact ocpp2), for "
              + "that one protocol version. The server offers permessage-deflate by default (upstream "
              + "default); compression is not declared because no self-test checks it.");
    }
    notes.add(
        "The message id of a CALL the upstream sends is not exposed: result documents have no "
            + "messageId, and call-sent and call-result events carry null.");
    notes.add(
        "The upstream does not expose the negotiated subprotocol, or the close code and reason: "
            + "connection-opened has subprotocol null, connection-closed has code and reason null.");
    if (version.equals("1.6")) {
      notes.add("The 1.6 JSONServer also accepts a handshake that offers no subprotocol (upstream behaviour).");
      notes.add(
          "The 1.6 JSONServer refuses Basic credentials whose password is not 16 to 20 bytes long before "
              + "the adapter sees them (no connection-rejected event).");
    } else {
      notes.add(
          "The 2.0.1 server refuses Basic credentials whose password is not 16 to 40 bytes long before "
              + "the adapter sees them (no connection-rejected event).");
    }
    JsonObject document = new JsonObject();
    document.add("client", client);
    document.add("server", server);
    document.add("features", features);
    document.add("notes", array(notes));
    return document;
  }

  static JsonObject capabilities() {
    JsonObject info = buildInfo();
    JsonObject upstream = new JsonObject();
    upstream.addProperty("name", "ChargeTimeEU Java-OCA-OCPP");
    upstream.addProperty("repository", "https://github.com/ChargeTimeEU/Java-OCA-OCPP");
    upstream.addProperty("version", string(info, "upstreamVersion", "unknown"));
    upstream.addProperty("revision", string(info, "upstreamRevision", "0".repeat(40)));
    upstream.addProperty("license", "MIT");
    upstream.addProperty("language", "Java");
    upstream.addProperty("independentEngine", true);

    JsonObject versions = new JsonObject();
    for (String version : new TreeSet<>(SUPPORTED_VERSIONS)) {
      versions.add(version, versionCapabilities(version));
    }

    JsonObject document = new JsonObject();
    document.addProperty("schemaVersion", Out.SCHEMA_VERSION);
    document.addProperty("adapter", Out.ADAPTER);
    document.addProperty("adapterVersion", string(info, "adapterVersion", "dev"));
    document.addProperty("contractVersion", Out.CONTRACT_VERSION);
    document.add("upstream", upstream);
    document.add("versions", versions);
    return document;
  }
}
