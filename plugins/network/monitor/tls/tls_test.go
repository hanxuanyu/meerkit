package tlsmonitor

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"testing"
	"time"
)

func TestExecuteReturnsVerifiedCertificateDetails(t *testing.T) {
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	certificate, rootPEM := testCertificate(t, now.Add(-time.Hour), now.Add(48*time.Hour))
	client, server := net.Pipe()
	serverDone := make(chan error, 1)
	go func() {
		connection := tls.Server(server, &tls.Config{Certificates: []tls.Certificate{certificate}})
		serverDone <- connection.Handshake()
		_ = connection.Close()
	}()
	module := &Module{Dial: func(context.Context, string, string) (net.Conn, error) { return client, nil }, Now: func() time.Time { return now }}
	raw, _ := json.Marshal(map[string]any{"host": "127.0.0.1", "server_name": "example.com", "root_ca_pem": string(rootPEM), "minimum_tls_version": "1.2"})
	observation, err := module.Execute(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	if serverErr := <-serverDone; serverErr != nil {
		t.Fatal(serverErr)
	}
	connection := observation.ResultSets["connection"]
	certificateResult := observation.ResultSets["certificate"]
	if connection["handshake_completed"] != true || certificateResult["valid"] != true || certificateResult["hostname_valid"] != true {
		t.Fatalf("unexpected TLS result: %#v %#v", connection, certificateResult)
	}
	if certificateResult["days_remaining"] != int64(2) {
		t.Fatalf("unexpected days remaining: %#v", certificateResult["days_remaining"])
	}
}

func TestValidationRejectsInvalidRootCA(t *testing.T) {
	raw := json.RawMessage(`{"host":"example.com","root_ca_pem":"not pem"}`)
	if err := New().ValidateConfig(raw); err == nil {
		t.Fatal("expected root CA validation error")
	}
}

func testCertificate(t *testing.T, notBefore, notAfter time.Time) (tls.Certificate, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(42), Subject: pkix.Name{CommonName: "example.com"}, DNSNames: []string{"example.com"}, NotBefore: notBefore, NotAfter: notAfter, KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, IsCA: true, BasicConstraintsValid: true}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	certificate, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	return certificate, certPEM
}
