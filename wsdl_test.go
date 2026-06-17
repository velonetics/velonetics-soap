package soap

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseWSDLFixture(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "country.wsdl"))
	if err != nil {
		t.Fatal(err)
	}
	info, err := parseWSDL(data, "CountryInfoService", "CountryInfoPort", "CountryFlag")
	if err != nil {
		t.Fatal(err)
	}
	if info.SOAPAction != "http://www.example.com/CountryFlag" {
		t.Fatalf("soapAction: %s", info.SOAPAction)
	}
	if info.Location != "http://127.0.0.1:8081/CountryInfoService.wso" {
		t.Fatalf("location: %s", info.Location)
	}
	host, path := splitLocation(info.Location)
	if host != "http://127.0.0.1:8081" || path != "/CountryInfoService.wso" {
		t.Fatalf("split: %s %s", host, path)
	}
}

func TestGenerateWSDLTemplate(t *testing.T) {
	raw := generateWSDLTemplate(&WSDLInfo{Operation: "CountryFlag", Namespace: "http://ex"})
	if raw == "" || !contains(raw, "CountryFlag") {
		t.Fatalf("template: %s", raw)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
