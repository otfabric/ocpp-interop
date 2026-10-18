// SPDX-License-Identifier: MIT
package org.otfabric.ocppinterop.chargetime;

import eu.chargetime.ocpp.feature.Feature;
import eu.chargetime.ocpp.feature.function.Function;
import eu.chargetime.ocpp.feature.profile.Profile;
import java.util.Map;
import java.util.Set;

/**
 * The upstream's feature classes, by OCPP version and action name.
 *
 * <p>Java-OCA-OCPP describes each action as a {@code Feature}: its action name, its request type
 * and its confirmation type, each class named {@code <Action>Feature}. OCPP 1.6 features are
 * {@code ProfileFeature}s, owned by a {@link Profile}, in {@code eu.chargetime.ocpp.feature} (Core,
 * Firmware Management, Local Auth List, Remote Trigger, Reservation, Smart Charging) and in {@code
 * eu.chargetime.ocpp.feature.securityext} (the Security Whitepaper extensions). OCPP 2.0.1
 * features are {@code FunctionFeature}s, owned by a {@link Function}, in {@code
 * eu.chargetime.ocpp.v201.feature} (the ocpp2 artifact).
 */
final class Registry {
  /** OCPP 1.6 actions by the role that sends them, with the Security Whitepaper extensions. */
  private static final Set<String> V16_FROM_STATION =
      Set.of(
          "Authorize", "BootNotification", "DataTransfer", "DiagnosticsStatusNotification",
          "FirmwareStatusNotification", "Heartbeat", "MeterValues", "StartTransaction",
          "StatusNotification", "StopTransaction", "LogStatusNotification",
          "SecurityEventNotification", "SignCertificate", "SignedFirmwareStatusNotification");

  private static final Set<String> V16_FROM_CSMS =
      Set.of(
          "CancelReservation", "ChangeAvailability", "ChangeConfiguration", "ClearCache",
          "ClearChargingProfile", "DataTransfer", "GetCompositeSchedule", "GetConfiguration",
          "GetDiagnostics", "GetLocalListVersion", "RemoteStartTransaction",
          "RemoteStopTransaction", "ReserveNow", "Reset", "SendLocalList", "SetChargingProfile",
          "TriggerMessage", "UnlockConnector", "UpdateFirmware", "CertificateSigned",
          "DeleteCertificate", "ExtendedTriggerMessage", "GetInstalledCertificateIds", "GetLog",
          "InstallCertificate", "SignedUpdateFirmware");

  /** OCPP 2.0.1 actions by the role that sends them (OCPP 2.0.1 Part 2). */
  private static final Set<String> V201_FROM_STATION =
      Set.of(
          "Authorize", "BootNotification", "ClearedChargingLimit", "DataTransfer",
          "FirmwareStatusNotification", "Get15118EVCertificate", "GetCertificateStatus",
          "Heartbeat", "LogStatusNotification", "MeterValues", "NotifyChargingLimit",
          "NotifyCustomerInformation", "NotifyDisplayMessages", "NotifyEVChargingNeeds",
          "NotifyEVChargingSchedule", "NotifyEvent", "NotifyMonitoringReport", "NotifyReport",
          "PublishFirmwareStatusNotification", "ReportChargingProfiles",
          "ReservationStatusUpdate", "SecurityEventNotification", "SignCertificate",
          "StatusNotification", "TransactionEvent");

  private static final Set<String> V201_FROM_CSMS =
      Set.of(
          "CancelReservation", "CertificateSigned", "ChangeAvailability", "ClearCache",
          "ClearChargingProfile", "ClearDisplayMessage", "ClearVariableMonitoring", "CostUpdated",
          "CustomerInformation", "DataTransfer", "DeleteCertificate", "GetBaseReport",
          "GetChargingProfiles", "GetCompositeSchedule", "GetDisplayMessages",
          "GetInstalledCertificateIds", "GetLocalListVersion", "GetLog", "GetMonitoringReport",
          "GetReport", "GetTransactionStatus", "GetVariables", "InstallCertificate",
          "PublishFirmware", "RequestStartTransaction", "RequestStopTransaction", "ReserveNow",
          "Reset", "SendLocalList", "SetChargingProfile", "SetDisplayMessage",
          "SetMonitoringBase", "SetMonitoringLevel", "SetNetworkProfile",
          "SetVariableMonitoring", "SetVariables", "TriggerMessage", "UnlockConnector",
          "UnpublishFirmware", "UpdateFirmware");

  private static final Map<String, Set<String>> FROM_STATION =
      Map.of("1.6", V16_FROM_STATION, "2.0.1", V201_FROM_STATION);
  private static final Map<String, Set<String>> FROM_CSMS =
      Map.of("1.6", V16_FROM_CSMS, "2.0.1", V201_FROM_CSMS);

  private static final Map<String, String[]> PACKAGES =
      Map.of(
          "1.6", new String[] {"eu.chargetime.ocpp.feature.", "eu.chargetime.ocpp.feature.securityext."},
          "2.0.1", new String[] {"eu.chargetime.ocpp.v201.feature."});

  private Registry() {}

  /** The actions the station sends in a version. */
  static Set<String> fromStation(String version) {
    return FROM_STATION.getOrDefault(version, Set.of());
  }

  /** The actions the CSMS sends in a version. */
  static Set<String> fromCsms(String version) {
    return FROM_CSMS.getOrDefault(version, Set.of());
  }

  /**
   * The upstream feature of an action in a version, owned by {@code owner} (a {@link Profile} for
   * OCPP 1.6, a {@link Function} for OCPP 2.0.1, or null for a feature that only sends); null if
   * the upstream has none.
   */
  static Feature feature(String version, String action, Object owner) {
    if (!action.matches("^[A-Z][A-Za-z0-9]+$") || !PACKAGES.containsKey(version)) {
      return null;
    }
    Class<?> ownerType = version.equals("1.6") ? Profile.class : Function.class;
    for (String prefix : PACKAGES.get(version)) {
      try {
        Class<?> cls = Class.forName(prefix + action + "Feature");
        if (!Feature.class.isAssignableFrom(cls)) {
          continue;
        }
        Feature feature = (Feature) cls.getConstructor(ownerType).newInstance(owner);
        return action.equals(feature.getAction()) ? feature : null;
      } catch (ClassNotFoundException e) {
        // try the next package
      } catch (ReflectiveOperationException e) {
        return null;
      }
    }
    return null;
  }
}
