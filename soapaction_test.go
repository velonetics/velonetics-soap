package soap

import "testing"

func TestSoapActionHeaderValue(t *testing.T) {
	if got := soapActionHeaderValue("http://ex.com/op", ""); got != `"http://ex.com/op"` {
		t.Fatalf("explicit: %s", got)
	}
	if got := soapActionHeaderValue("", "http://wsdl/action"); got != `"http://wsdl/action"` {
		t.Fatalf("wsdl: %s", got)
	}
	if got := soapActionHeaderValue("http://a", "http://b"); got != `"http://a"` {
		t.Fatalf("priority: %s", got)
	}
	if got := soapActionHeaderValue("", ""); got != "" {
		t.Fatalf("empty: %s", got)
	}
}
