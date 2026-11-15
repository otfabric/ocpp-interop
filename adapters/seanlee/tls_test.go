// SPDX-License-Identifier: MIT
package main

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"testing"
)

func TestCertNamesTakesTheCommonNameOnly(t *testing.T) {
	for _, c := range []struct {
		name     string
		cn       string
		dnsNames []string
		want     bool
	}{
		{name: "CommonName is the station", cn: "CP001", want: true},
		{name: "CommonName is the station, another DNS name", cn: "CP001", dnsNames: []string{"CP002"}, want: true},
		{name: "another CommonName", cn: "CP002"},
		{name: "another CommonName, the station as DNS name", cn: "CP002", dnsNames: []string{"CP001"}},
		{name: "no CommonName, the station as DNS name", dnsNames: []string{"CP001"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			cert := &x509.Certificate{Subject: pkix.Name{CommonName: c.cn}, DNSNames: c.dnsNames}
			if got := certNames(cert, "CP001"); got != c.want {
				t.Fatalf("certNames = %v, want %v", got, c.want)
			}
		})
	}
}
