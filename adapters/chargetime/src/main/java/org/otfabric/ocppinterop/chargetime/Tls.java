// SPDX-License-Identifier: MIT
package org.otfabric.ocppinterop.chargetime;

import eu.chargetime.ocpp.wss.BaseWssSocketBuilder;
import eu.chargetime.ocpp.wss.WssFactoryBuilder;
import eu.chargetime.ocpp.wss.WssSocketBuilder;
import java.io.IOException;
import java.io.InputStream;
import java.net.InetSocketAddress;
import java.nio.channels.ByteChannel;
import java.nio.channels.SelectionKey;
import java.nio.channels.SocketChannel;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.security.GeneralSecurityException;
import java.security.KeyFactory;
import java.security.KeyStore;
import java.security.PrivateKey;
import java.security.cert.Certificate;
import java.security.cert.CertificateFactory;
import java.security.cert.X509Certificate;
import java.security.spec.PKCS8EncodedKeySpec;
import java.util.ArrayList;
import java.util.Base64;
import java.util.List;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;
import java.util.regex.Matcher;
import java.util.regex.Pattern;
import javax.naming.InvalidNameException;
import javax.naming.ldap.LdapName;
import javax.naming.ldap.Rdn;
import javax.net.ssl.KeyManagerFactory;
import javax.net.ssl.SSLContext;
import javax.net.ssl.SSLEngine;
import javax.net.ssl.SSLParameters;
import javax.net.ssl.SSLPeerUnverifiedException;
import javax.net.ssl.TrustManagerFactory;
import org.java_websocket.WebSocketServerFactory;
import org.java_websocket.interfaces.ISSLChannel;
import org.java_websocket.server.SSLParametersWebSocketServerFactory;

/**
 * Security profiles 2 and 3 (docs/CONTAINER_CONTRACT.md, Transport security): the JSSE {@link
 * SSLContext} that the upstream's WSS support takes ({@code enableWSS}). The adapter only loads the
 * PEM files of {@code --tls-dir}; the handshake and the verification of the peer's chain are the
 * JDK's, and the station's check of the server's host name is Java-WebSocket's ({@code
 * WebSocketClient.onSetSSLParameters} sets the endpoint identification algorithm HTTPS).
 */
final class Tls {
  private Tls() {}

  private static final char[] NO_PASSWORD = new char[0];
  private static final Pattern PKCS8 =
      Pattern.compile("-----BEGIN PRIVATE KEY-----([A-Za-z0-9+/=\\s]+)-----END PRIVATE KEY-----");

  private static Path file(Path dir, String name) throws Main.UsageException {
    Path path = dir.resolve(name);
    if (!Files.isRegularFile(path)) {
      throw new Main.UsageException("--tls-dir " + dir + ": " + name + " is missing");
    }
    return path;
  }

  private static List<X509Certificate> certificates(Path dir, String name) throws Main.UsageException {
    Path path = file(dir, name);
    List<X509Certificate> list = new ArrayList<>();
    try (InputStream in = Files.newInputStream(path)) {
      for (Certificate c : CertificateFactory.getInstance("X.509").generateCertificates(in)) {
        list.add((X509Certificate) c);
      }
    } catch (IOException | GeneralSecurityException e) {
      throw new Main.UsageException("--tls-dir " + dir + ": " + name + ": " + e.getMessage());
    }
    if (list.isEmpty()) {
      throw new Main.UsageException("--tls-dir " + dir + ": " + name + " holds no PEM certificate");
    }
    return list;
  }

  /** An unencrypted PKCS #8 PEM key ("BEGIN PRIVATE KEY"), RSA or EC. */
  private static PrivateKey privateKey(Path dir, String name) throws Main.UsageException {
    Path path = file(dir, name);
    String text;
    try {
      text = Files.readString(path, StandardCharsets.US_ASCII);
    } catch (IOException e) {
      throw new Main.UsageException("--tls-dir " + dir + ": " + name + ": " + e.getMessage());
    }
    Matcher m = PKCS8.matcher(text);
    if (!m.find()) {
      throw new Main.UsageException("--tls-dir " + dir + ": " + name + " is not an unencrypted PKCS #8 PEM key");
    }
    byte[] der = Base64.getMimeDecoder().decode(m.group(1));
    for (String algorithm : List.of("RSA", "EC")) {
      try {
        return KeyFactory.getInstance(algorithm).generatePrivate(new PKCS8EncodedKeySpec(der));
      } catch (GeneralSecurityException e) {
        // try the next algorithm
      }
    }
    throw new Main.UsageException("--tls-dir " + dir + ": " + name + " is neither an RSA nor an EC key");
  }

  private static KeyManagerFactory keys(Path dir, String cert, String key) throws Main.UsageException {
    try {
      KeyStore store = KeyStore.getInstance("PKCS12");
      store.load(null, null);
      store.setKeyEntry("key", privateKey(dir, key), NO_PASSWORD,
          certificates(dir, cert).toArray(new X509Certificate[0]));
      KeyManagerFactory factory = KeyManagerFactory.getInstance(KeyManagerFactory.getDefaultAlgorithm());
      factory.init(store, NO_PASSWORD);
      return factory;
    } catch (IOException | GeneralSecurityException e) {
      throw new Main.UsageException("--tls-dir " + dir + ": " + cert + ", " + key + ": " + e.getMessage());
    }
  }

  private static TrustManagerFactory trust(Path dir) throws Main.UsageException {
    try {
      KeyStore store = KeyStore.getInstance("PKCS12");
      store.load(null, null);
      int i = 0;
      for (X509Certificate c : certificates(dir, "ca.crt")) {
        store.setCertificateEntry("ca" + i++, c);
      }
      TrustManagerFactory factory = TrustManagerFactory.getInstance(TrustManagerFactory.getDefaultAlgorithm());
      factory.init(store);
      return factory;
    } catch (IOException | GeneralSecurityException e) {
      throw new Main.UsageException("--tls-dir " + dir + ": ca.crt: " + e.getMessage());
    }
  }

  /** The server end: server.crt and server.key, TLS 1.2 or later; profile 3 requires a client
   * certificate that verifies against ca.crt. */
  static Server server(Path dir, int profile) throws Main.UsageException {
    try {
      SSLContext context = SSLContext.getInstance("TLS");
      context.init(keys(dir, "server.crt", "server.key").getKeyManagers(),
          profile == 3 ? trust(dir).getTrustManagers() : null, null);
      SSLParameters parameters = context.getDefaultSSLParameters();
      parameters.setProtocols(new String[] {"TLSv1.3", "TLSv1.2"});
      parameters.setNeedClientAuth(profile == 3);
      return new Server(context, parameters);
    } catch (GeneralSecurityException e) {
      throw new Main.UsageException("--tls-dir " + dir + ": " + e.getMessage());
    }
  }

  /** The station end: trusts ca.crt only; profile 3 presents client.crt with client.key. */
  static WssSocketBuilder client(Path dir, int profile, int connectTimeoutMs) throws Main.UsageException {
    try {
      SSLContext context = SSLContext.getInstance("TLS");
      context.init(profile == 3 ? keys(dir, "client.crt", "client.key").getKeyManagers() : null,
          trust(dir).getTrustManagers(), null);
      return BaseWssSocketBuilder.builder()
          .sslSocketFactory(context.getSocketFactory())
          .connectionTimeout(connectTimeoutMs);
    } catch (GeneralSecurityException e) {
      throw new Main.UsageException("--tls-dir " + dir + ": " + e.getMessage());
    }
  }

  /**
   * The upstream's {@link WssFactoryBuilder}, building Java-WebSocket's own {@link
   * SSLParametersWebSocketServerFactory}. It keeps the SSL engine of each connection by remote
   * address, so that the adapter can read the certificate a station presented (the upstream's
   * {@code ServerEvents} get the remote address, not the connection).
   */
  static final class Server implements WssFactoryBuilder {
    private final SSLContext context;
    private final SSLParameters parameters;
    private final Map<String, SSLEngine> engines = new ConcurrentHashMap<>();

    Server(SSLContext context, SSLParameters parameters) {
      this.context = context;
      this.parameters = parameters;
    }

    @Override
    public WebSocketServerFactory build() {
      return new SSLParametersWebSocketServerFactory(context, parameters) {
        @Override
        public ByteChannel wrapChannel(SocketChannel channel, SelectionKey key) throws IOException {
          ByteChannel wrapped = super.wrapChannel(channel, key);
          if (wrapped instanceof ISSLChannel && channel.getRemoteAddress() instanceof InetSocketAddress) {
            engines.put(ServerCommand.remote((InetSocketAddress) channel.getRemoteAddress()),
                ((ISSLChannel) wrapped).getSSLEngine());
          }
          return wrapped;
        }
      };
    }

    @Override
    public void verify() {}

    /** The subject CommonName of the certificate the station at that address presented, or null. */
    String peerCommonName(String remote) {
      SSLEngine engine = remote == null ? null : engines.remove(remote);
      if (engine == null) {
        return null;
      }
      try {
        Certificate[] chain = engine.getSession().getPeerCertificates();
        if (chain.length == 0 || !(chain[0] instanceof X509Certificate)) {
          return null;
        }
        String subject = ((X509Certificate) chain[0]).getSubjectX500Principal().getName();
        for (Rdn rdn : new LdapName(subject).getRdns()) {
          if (rdn.getType().equalsIgnoreCase("CN")) {
            return rdn.getValue().toString();
          }
        }
      } catch (SSLPeerUnverifiedException | InvalidNameException e) {
        return null;
      }
      return null;
    }
  }
}
