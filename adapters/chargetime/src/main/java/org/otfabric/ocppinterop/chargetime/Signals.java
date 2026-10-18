// SPDX-License-Identifier: MIT
package org.otfabric.ocppinterop.chargetime;

import java.util.concurrent.CountDownLatch;
import sun.misc.Signal;

/**
 * SIGTERM and SIGINT stop a running server or {@code client run} cleanly. Without a handler the JVM
 * exits with 143; the contract wants the {@code stopped} event and exit 0.
 */
final class Signals {
  private Signals() {}

  static CountDownLatch onTermination() {
    CountDownLatch stop = new CountDownLatch(1);
    Signal.handle(new Signal("TERM"), signal -> stop.countDown());
    Signal.handle(new Signal("INT"), signal -> stop.countDown());
    return stop;
  }
}
