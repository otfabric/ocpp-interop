// SPDX-License-Identifier: MIT
package org.otfabric.ocppinterop.chargetime;

import com.google.gson.JsonElement;
import com.google.gson.JsonNull;
import com.google.gson.JsonObject;
import com.google.gson.JsonParser;
import eu.chargetime.ocpp.CallErrorException;
import eu.chargetime.ocpp.JSONCommunicator;
import eu.chargetime.ocpp.NotConnectedException;
import eu.chargetime.ocpp.OccurenceConstraintException;
import eu.chargetime.ocpp.UnsupportedFeatureException;
import eu.chargetime.ocpp.feature.Feature;
import eu.chargetime.ocpp.feature.FunctionFeature;
import eu.chargetime.ocpp.feature.ProfileFeature;
import eu.chargetime.ocpp.feature.function.Function;
import eu.chargetime.ocpp.feature.profile.Profile;
import eu.chargetime.ocpp.model.Confirmation;
import eu.chargetime.ocpp.model.Request;
import java.lang.reflect.Proxy;
import java.util.ArrayList;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Set;
import java.util.UUID;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.CompletionStage;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.TimeoutException;

/**
 * The fixture's side of a connection: an upstream {@link Profile} (OCPP 1.6) or {@link Function}
 * (OCPP 2.0.1) whose features are the upstream's own feature classes, and the sending of CALLs for
 * the contract.
 *
 * <p>Raw JSON becomes upstream request and confirmation types, and goes back to JSON, through
 * {@link JSONCommunicator#unpackPayload} and {@link JSONCommunicator#packPayload}: the Gson setup
 * the upstream uses on the wire. Validation ({@code validate()}), framing, routing and the
 * CALLERRORs the upstream sends on its own are the upstream's.
 */
final class Endpoint {
  /** Sends one request with the upstream: JSONClient.send or JSONServer.send. */
  interface Sender {
    CompletionStage<Confirmation> send(Request request)
        throws OccurenceConstraintException, UnsupportedFeatureException, NotConnectedException;
  }

  /** The CALLERROR the upstream sends when a handler fails (eu.chargetime.ocpp.ConfirmationHandler). */
  private static final String INTERNAL_ERROR_DESCRIPTION =
      "An internal error occurred and the receiver was not able to process the requested Action successfully";

  private final String version;
  private final Fixture fixture;
  private final String role;
  private final java.util.function.Function<UUID, String> stationOf;
  private final boolean emit;
  private final JSONCommunicator codec = new JSONCommunicator(null);
  private final Feature[] features;
  private final Object owner;

  /**
   * @param role the answering role: "csms" for a server, "station" for a client
   * @param stationOf the station identity of an upstream session
   */
  Endpoint(String version, Fixture fixture, String role, java.util.function.Function<UUID, String> stationOf, boolean emit) {
    this.version = version;
    this.fixture = fixture;
    this.role = role;
    this.stationOf = stationOf;
    this.emit = emit;
    // Registered: what the fixture answers, and what this role sends. An
    // action outside both is unknown to the upstream, which answers it with
    // NotImplemented itself.
    Set<String> actions = new LinkedHashSet<>();
    fixture.answers(role).keySet().forEach(actions::add);
    actions.addAll(role.equals("csms") ? Registry.fromCsms(version) : Registry.fromStation(version));
    // The upstream's features call their owner to handle a request.
    this.owner = version.equals("1.6") ? new ProfileView() : new FunctionView();
    List<Feature> list = new ArrayList<>();
    for (String action : actions) {
      Feature feature = Registry.feature(version, action, owner);
      if (feature != null) {
        list.add(feature);
      }
    }
    this.features = list.toArray(new Feature[0]);
  }

  /** OCPP 1.6: the endpoint as an upstream feature profile. */
  private final class ProfileView implements Profile {
    @Override
    public ProfileFeature[] getFeatureList() {
      return java.util.Arrays.stream(features).map(f -> (ProfileFeature) f).toArray(ProfileFeature[]::new);
    }

    @Override
    public Confirmation handleRequest(UUID session, Request request) {
      return Endpoint.this.handleRequest(session, request);
    }
  }

  /** OCPP 2.0.1: the endpoint as an upstream function (functional block). */
  private final class FunctionView implements Function {
    @Override
    public FunctionFeature[] getFeatureList() {
      return java.util.Arrays.stream(features).map(f -> (FunctionFeature) f).toArray(FunctionFeature[]::new);
    }

    @Override
    public Confirmation handleRequest(UUID session, Request request) {
      return Endpoint.this.handleRequest(session, request);
    }
  }

  String version() {
    return version;
  }

  /** OCPP 1.6: the upstream profile to add to JSONServer or JSONClient. */
  Profile profile() {
    return (Profile) owner;
  }

  /** OCPP 2.0.1: the upstream function to add to MultiProtocolJSONServer or MultiProtocolJSONClient. */
  Function function() {
    return (Function) owner;
  }

  /** An upstream event handler interface (ServerCoreEventHandler, ClientCoreEventHandler) answered by this profile. */
  <T> T handler(Class<T> type) {
    return type.cast(
        Proxy.newProxyInstance(
            type.getClassLoader(),
            new Class<?>[] {type},
            (proxy, method, args) -> {
              if (method.getDeclaringClass() == Object.class) {
                return method.invoke(this, args);
              }
              UUID session = null;
              Request request = null;
              for (Object arg : args == null ? new Object[0] : args) {
                if (arg instanceof UUID) {
                  session = (UUID) arg;
                } else if (arg instanceof Request) {
                  request = (Request) arg;
                }
              }
              return handleRequest(session, request);
            }));
  }

  private String actionOf(Request request) {
    for (Feature f : features) {
      if (f.getRequestType().isInstance(request)) {
        return f.getAction();
      }
    }
    String name = request.getClass().getSimpleName();
    return name.endsWith("Request") ? name.substring(0, name.length() - "Request".length()) : name;
  }

  private Class<? extends Confirmation> confirmationOf(String action) {
    for (Feature f : features) {
      if (f.getAction().equals(action)) {
        return f.getConfirmationType();
      }
    }
    return null;
  }

  JsonElement toJson(Object upstreamPayload) {
    return JsonParser.parseString((String) codec.packPayload(upstreamPayload));
  }

  Confirmation handleRequest(UUID session, Request request) {
    String station = stationOf.apply(session);
    String action = actionOf(request);
    String messageId = request.getOcppMessageId();
    JsonObject payload = toJson(request).getAsJsonObject();
    if (emit) {
      Out.event("call-received", station, "messageId", messageId, "action", action, "payload", payload);
    }
    JsonObject answer = fixture.answer(role, action, payload);
    Class<? extends Confirmation> type = confirmationOf(action);
    Confirmation confirmation = null;
    String failure = null;
    if (answer == null) {
      failure = "the fixture has no answer for " + action;
    } else if (type == null) {
      failure = "no upstream feature for " + action;
    } else {
      try {
        confirmation = codec.unpackPayload(answer.toString(), type);
      } catch (Exception e) {
        failure = "the upstream cannot read the fixture's answer: " + e;
      }
    }
    if (failure != null) {
      // The upstream turns a failing handler into this CALLERROR.
      if (emit) {
        JsonObject error = new JsonObject();
        error.addProperty("code", "InternalError");
        error.addProperty("description", INTERNAL_ERROR_DESCRIPTION);
        error.add("details", new JsonObject());
        Out.event("call-answered", station, "messageId", messageId, "action", action, "error", error);
      }
      System.err.println("chargetime: " + failure);
      throw new IllegalStateException(failure);
    }
    if (emit) {
      Out.event("call-answered", station, "messageId", messageId, "action", action, "payload", toJson(confirmation));
    }
    return confirmation;
  }

  /** Sends one CALL with the upstream and returns a result document. */
  JsonObject execute(
      String action, JsonObject payload, long timeoutMs, String station, Sender sender, CompletableFuture<?> closed) {
    long started = System.nanoTime();
    Feature feature = Registry.feature(version, action, null);
    if (feature == null) {
      return done(started, action, false, payload, null, null, "unsupported",
          "Java-OCA-OCPP has no OCPP " + version + " feature for " + action);
    }
    Request request;
    try {
      request = codec.unpackPayload(payload.toString(), feature.getRequestType());
    } catch (Exception e) {
      return done(started, action, false, payload, null, null, "invalid-request",
          "the upstream cannot read the payload as " + feature.getRequestType().getSimpleName() + ": " + e.getMessage());
    }
    JsonElement wire = toJson(request);
    boolean valid;
    try {
      valid = request.validate();
    } catch (RuntimeException e) {
      valid = false;
    }
    if (!valid) {
      return done(started, action, false, wire, null, null, "invalid-request",
          feature.getRequestType().getSimpleName() + ".validate() is false: a required field is missing or a value is out of range");
    }
    if (closed.isDone()) {
      return done(started, action, false, wire, null, null, "not-connected", "the connection is closed");
    }
    if (emit) {
      Out.event("call-sent", station, "messageId", null, "action", action, "payload", wire);
    }
    CompletableFuture<Confirmation> response;
    try {
      response = sender.send(request).toCompletableFuture();
    } catch (OccurenceConstraintException e) {
      return done(started, action, false, wire, null, null, "invalid-request", "the upstream refused the request");
    } catch (UnsupportedFeatureException e) {
      return done(started, action, false, wire, null, null, "unsupported",
          "the upstream has no feature for " + action + " in this role");
    } catch (NotConnectedException e) {
      return done(started, action, false, wire, null, null, "not-connected", "not connected");
    }
    try {
      CompletableFuture.anyOf(response, closed).get(timeoutMs, TimeUnit.MILLISECONDS);
    } catch (TimeoutException e) {
      return done(started, action, false, wire, null, null, "timeout", "no response within " + timeoutMs + " ms");
    } catch (InterruptedException e) {
      Thread.currentThread().interrupt();
      return done(started, action, false, wire, null, null, "failed", "interrupted");
    } catch (ExecutionException e) {
      // handled below, from the future itself
    }
    if (!response.isDone()) {
      return done(started, action, false, wire, null, null, "connection-lost",
          "the connection closed while waiting for the response");
    }
    try {
      JsonElement body = toJson(response.get());
      if (emit) {
        Out.event("call-result", station, "messageId", null, "action", action, "payload", body);
      }
      return done(started, action, true, wire, body, null, null, null);
    } catch (InterruptedException e) {
      Thread.currentThread().interrupt();
      return done(started, action, false, wire, null, null, "failed", "interrupted");
    } catch (ExecutionException e) {
      Throwable cause = e.getCause();
      if (cause instanceof CallErrorException) {
        CallErrorException ce = (CallErrorException) cause;
        if ("Not connected".equals(ce.getErrorCode())) {
          // Not a CALLERROR from the peer: the upstream reports a send without
          // a connection this way (eu.chargetime.ocpp.Communicator).
          return done(started, action, false, wire, null, null, "connection-lost", ce.getErrorDescription());
        }
        JsonObject callError = new JsonObject();
        callError.addProperty("code", ce.getErrorCode());
        callError.addProperty("description", ce.getErrorDescription());
        callError.add("details", details(ce.getPayload()));
        if (emit) {
          Out.event("call-result", station, "messageId", null, "action", action, "error", callError);
        }
        return done(started, action, false, wire, null, callError, "call-error", "CALLERROR " + ce.getErrorCode());
      }
      return done(started, action, false, wire, null, null, "failed", String.valueOf(cause));
    }
  }

  private static JsonElement details(Object payload) {
    if (payload == null) {
      return JsonNull.INSTANCE;
    }
    try {
      return JsonParser.parseString(payload.toString());
    } catch (RuntimeException e) {
      return Out.element(payload.toString());
    }
  }

  private JsonObject done(
      long started, String action, boolean ok, JsonElement request, JsonElement response,
      JsonObject callError, String errorCode, String message) {
    long elapsed = TimeUnit.NANOSECONDS.toMillis(System.nanoTime() - started);
    return Out.result(version, action, ok, request, response, callError, errorCode, message, elapsed);
  }
}
