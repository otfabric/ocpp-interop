// SPDX-License-Identifier: MIT
package main

// Security profiles 2 and 3 (docs/CONTAINER_CONTRACT.md, Transport
// security): the tls.Config that ocpp-go's ws.NewTLSServer and
// ws.NewTLSClient take. The adapter only loads the PEM files of --tls-dir;
// the handshake, the verification of the peer's chain and of the server's
// host name are Go's crypto/tls (and gorilla/websocket's dialer, which sets
// the server name from the URL).

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"
)

// tlsFile is the path of one file of --tls-dir; a missing file is a usage error.
func tlsFile(dir, name string) (string, error) {
	path := filepath.Join(dir, name)
	if info, err := os.Stat(path); err != nil || info.IsDir() {
		return "", usagef("--tls-dir %s: %s is missing", dir, name)
	}
	return path, nil
}

func caPool(dir string) (*x509.CertPool, error) {
	path, err := tlsFile(dir, "ca.crt")
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, usagef("--tls-dir %s: %v", dir, err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(data) {
		return nil, usagef("--tls-dir %s: ca.crt holds no PEM certificate", dir)
	}
	return pool, nil
}

func keyPair(dir, certName, keyName string) (tls.Certificate, error) {
	certPath, err := tlsFile(dir, certName)
	if err != nil {
		return tls.Certificate{}, err
	}
	keyPath, err := tlsFile(dir, keyName)
	if err != nil {
		return tls.Certificate{}, err
	}
	pair, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return tls.Certificate{}, usagef("--tls-dir %s: %s, %s: %v", dir, certName, keyName, err)
	}
	return pair, nil
}

// serverTLS is the server's configuration: TLS 1.2 or later with server.crt
// and server.key; profile 3 requires a client certificate that verifies
// against ca.crt. It also returns the paths ws.NewTLSServer loads.
func serverTLS(dir string, profile int) (config *tls.Config, certPath, keyPath string, err error) {
	if _, err = keyPair(dir, "server.crt", "server.key"); err != nil {
		return nil, "", "", err
	}
	certPath, _ = tlsFile(dir, "server.crt")
	keyPath, _ = tlsFile(dir, "server.key")
	config = &tls.Config{MinVersion: tls.VersionTLS12}
	if profile == 3 {
		pool, err := caPool(dir)
		if err != nil {
			return nil, "", "", err
		}
		config.ClientAuth = tls.RequireAndVerifyClientCert
		config.ClientCAs = pool
	}
	return config, certPath, keyPath, nil
}

// clientTLS is the station's configuration: TLS 1.2 or later, trusting
// ca.crt only; profile 3 presents client.crt with client.key.
func clientTLS(dir string, profile int) (*tls.Config, error) {
	pool, err := caPool(dir)
	if err != nil {
		return nil, err
	}
	config := &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: pool}
	if profile == 3 {
		pair, err := keyPair(dir, "client.crt", "client.key")
		if err != nil {
			return nil, err
		}
		config.Certificates = []tls.Certificate{pair}
	}
	return config, nil
}

// peerCommonName is the subject CommonName of the certificate the station
// presented, verified by the handshake.
func peerCommonName(state *tls.ConnectionState) (string, error) {
	if state == nil || len(state.PeerCertificates) == 0 {
		return "", fmt.Errorf("no client certificate")
	}
	return state.PeerCertificates[0].Subject.CommonName, nil
}
