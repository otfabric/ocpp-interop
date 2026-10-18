// SPDX-License-Identifier: MIT
package org.otfabric.ocppinterop.chargetime;

import eu.chargetime.ocpp.feature.ProfileFeature;
import eu.chargetime.ocpp.feature.profile.Profile;
import java.util.Set;

/**
 * The upstream's OCPP 1.6 feature classes, by action name.
 *
 * <p>Java-OCA-OCPP describes each action as a {@code Feature}: its action name, its request type
 * and its confirmation type. They live in {@code eu.chargetime.ocpp.feature} (Core, Firmware
 * Management, Local Auth List, Remote Trigger, Reservation, Smart Charging) and in {@code
 * eu.chargetime.ocpp.feature.securityext} (the Security Whitepaper extensions), each named
 * {@code <Action>Feature}.
 */
final class Registry {
  /** Actions by the role that sends them: OCPP 1.6 with the Security Whitepaper extensions. */
  static final Set<String> FROM_STATION =
      Set.of(
          "Authorize", "BootNotification", "DataTransfer", "DiagnosticsStatusNotification",
          "FirmwareStatusNotification", "Heartbeat", "MeterValues", "StartTransaction",
          "StatusNotification", "StopTransaction", "LogStatusNotification",
          "SecurityEventNotification", "SignCertificate", "SignedFirmwareStatusNotification");

  static final Set<String> FROM_CSMS =
      Set.of(
          "CancelReservation", "ChangeAvailability", "ChangeConfiguration", "ClearCache",
          "ClearChargingProfile", "DataTransfer", "GetCompositeSchedule", "GetConfiguration",
          "GetDiagnostics", "GetLocalListVersion", "RemoteStartTransaction",
          "RemoteStopTransaction", "ReserveNow", "Reset", "SendLocalList", "SetChargingProfile",
          "TriggerMessage", "UnlockConnector", "UpdateFirmware", "CertificateSigned",
          "DeleteCertificate", "ExtendedTriggerMessage", "GetInstalledCertificateIds", "GetLog",
          "InstallCertificate", "SignedUpdateFirmware");

  private static final String[] PACKAGES = {
    "eu.chargetime.ocpp.feature.", "eu.chargetime.ocpp.feature.securityext."
  };

  private Registry() {}

  /** The upstream feature of an action, owned by {@code profile}; null if the upstream has none. */
  static ProfileFeature feature(String action, Profile profile) {
    if (!action.matches("^[A-Z][A-Za-z0-9]+$")) {
      return null;
    }
    for (String prefix : PACKAGES) {
      try {
        Class<?> cls = Class.forName(prefix + action + "Feature");
        if (!ProfileFeature.class.isAssignableFrom(cls)) {
          continue;
        }
        ProfileFeature feature =
            (ProfileFeature) cls.getConstructor(Profile.class).newInstance(profile);
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
