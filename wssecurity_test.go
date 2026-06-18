package soap

import (
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"os"
	"strings"
	"testing"
)

func TestApplyWSSecurityUsernameToken(t *testing.T) {
	body := []byte(`<?xml version="1.0"?>
<Envelope xmlns="http://schemas.xmlsoap.org/soap/envelope/">
  <Body><CountryFlag/></Body>
</Envelope>`)
	cfg := &WSSecurityConfig{
		UsernameToken: &UsernameTokenConfig{Username: "alice", Password: "secret"},
	}
	out, err := applyWSSecurity(body, cfg)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "alice") || !strings.Contains(s, "secret") {
		t.Fatalf("missing credentials: %s", s)
	}
	if !strings.Contains(s, "UsernameToken") {
		t.Fatalf("missing token: %s", s)
	}
}

func TestApplyWSSecuritySAML(t *testing.T) {
	body := []byte(`<Envelope xmlns="http://schemas.xmlsoap.org/soap/envelope/"><Body><X/></Body></Envelope>`)
	cfg := &WSSecurityConfig{
		SAMLAssertion: &SAMLAssertionConfig{Path: "testdata/assertion.xml"},
	}
	out, err := applyWSSecurity(body, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "Assertion") {
		t.Fatalf("missing assertion: %s", out)
	}
}

func TestApplyWSSecurityX509(t *testing.T) {
	body := []byte(`<?xml version="1.0"?>
<Envelope xmlns="http://schemas.xmlsoap.org/soap/envelope/">
  <Body><CountryFlag/></Body>
</Envelope>`)
	cfg := &WSSecurityConfig{
		X509: &X509Config{
			CertPath: "testdata/client.pem",
			KeyPath:  "testdata/client-key.pem",
		},
	}
	out, err := applyWSSecurity(body, cfg)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "Signature") {
		t.Fatalf("missing signature: %s", s)
	}
	if !strings.Contains(s, "CountryFlag") {
		t.Fatalf("body altered unexpectedly: %s", s)
	}
}

func TestLoadTLSKeyPairEncryptedPassword(t *testing.T) {
	certPEM, err := os.ReadFile("testdata/client.pem")
	if err != nil {
		t.Fatal(err)
	}
	keyPEM, err := os.ReadFile("testdata/client-key.pem")
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode(keyPEM)
	if block == nil {
		t.Fatal("failed to decode key PEM")
	}
	encrypted, err := x509.EncryptPEMBlock(rand.Reader, block.Type, block.Bytes, []byte("secret"), x509.PEMCipherDES)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := loadTLSKeyPair(certPEM, pem.EncodeToMemory(encrypted), "secret"); err != nil {
		t.Fatalf("expected encrypted key to load with password: %v", err)
	}
	if _, err := loadTLSKeyPair(certPEM, pem.EncodeToMemory(encrypted), "wrong"); err == nil {
		t.Fatal("expected wrong password to fail")
	}
}
