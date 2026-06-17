package soap

import (
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
