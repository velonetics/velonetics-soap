package soap

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/velonetics/lura/v2/config"
	"github.com/velonetics/lura/v2/logging"
	"github.com/velonetics/lura/v2/proxy"
)

func TestBackendFactory_noConfig(t *testing.T) {
	called := false
	bf := BackendFactory(logging.NoOp, func(_ *config.Backend) proxy.Proxy {
		return func(_ context.Context, _ *proxy.Request) (*proxy.Response, error) {
			called = true
			return &proxy.Response{}, nil
		}
	})

	p := bf(&config.Backend{URLPattern: "/test"})
	_, _ = p(context.Background(), &proxy.Request{})
	if !called {
		t.Fatal("expected inner proxy to be called")
	}
}

func TestRender_paramsAndPath(t *testing.T) {
	tmpl := `<?xml version="1.0"?><Country><Code>{{ .req_params.Country }}</Code><Path>{{ .req_path }}</Path></Country>`
	cfg := mustConfig(t, tmpl, "")

	body, _, err := cfg.render(logging.NoOp, "[TEST]", &proxy.Request{
		Params: map[string]string{"Country": "US"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := string(body)
	if !strings.Contains(got, "<Code>US</Code>") {
		t.Fatalf("unexpected body: %s", got)
	}
	if !strings.Contains(got, "<Path>/service.wso</Path>") {
		t.Fatalf("unexpected path in body: %s", got)
	}
}

func TestRender_headersAndQuery(t *testing.T) {
	tmpl := `<Req><H>{{ index .req_headers "X-Token" }}</H><Q>{{ .req_querystring.limit }}</Q></Req>`
	cfg := mustConfig(t, tmpl, "")

	body, _, err := cfg.render(logging.NoOp, "[TEST]", &proxy.Request{
		Headers: map[string][]string{
			"X-Token": {"secret"},
		},
		Query: url.Values{"limit": []string{"10"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := string(body)
	if !strings.Contains(got, "<H>[secret]</H>") {
		t.Fatalf("unexpected headers in body: %s", got)
	}
	if !strings.Contains(got, "<Q>10</Q>") {
		t.Fatalf("unexpected query in body: %s", got)
	}
}

func TestRender_jsonBody(t *testing.T) {
	tmpl := `<User>{{ .req_body.name }}</User>`
	cfg := mustConfig(t, tmpl, "")

	body, _, err := cfg.render(logging.NoOp, "[TEST]", &proxy.Request{
		Headers: map[string][]string{"Content-Type": {"application/json"}},
		Body:    io.NopCloser(strings.NewReader(`{"name":"alice"}`)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != `<User>alice</User>` {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestRender_xmlBody(t *testing.T) {
	tmpl := `<Out>{{ .req_body.Envelope.Body.name }}</Out>`
	cfg := mustConfig(t, tmpl, "")

	body, _, err := cfg.render(logging.NoOp, "[TEST]", &proxy.Request{
		Headers: map[string][]string{"Content-Type": {"application/xml"}},
		Body:    io.NopCloser(strings.NewReader(`<Envelope><Body><name>bob</name></Body></Envelope>`)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "bob") {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestRender_formBody(t *testing.T) {
	tmpl := `<F>{{ .req_body.foo }}</F>`
	cfg := mustConfig(t, tmpl, "")

	body, _, err := cfg.render(logging.NoOp, "[TEST]", &proxy.Request{
		Headers: map[string][]string{"Content-Type": {"application/x-www-form-urlencoded"}},
		Body:    io.NopCloser(strings.NewReader("foo=bar")),
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != `<F>bar</F>` {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestRender_plainBody(t *testing.T) {
	tmpl := `<T>{{ .req_body }}</T>`
	cfg := mustConfig(t, tmpl, "")

	body, _, err := cfg.render(logging.NoOp, "[TEST]", &proxy.Request{
		Headers: map[string][]string{"Content-Type": {"text/plain"}},
		Body:    io.NopCloser(strings.NewReader("hello")),
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != `<T>hello</T>` {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestRender_multipartBody(t *testing.T) {
	tmpl := `<F>{{ .req_body.field }}</F>`
	cfg := mustConfig(t, tmpl, "")

	var buf bytes.Buffer
	w := multipartWriter(t, &buf)
	_ = w.WriteField("field", "value")
	w.Close()

	body, _, err := cfg.render(logging.NoOp, "[TEST]", &proxy.Request{
		Headers: map[string][]string{
			"Content-Type": {w.FormDataContentType()},
		},
		Body: io.NopCloser(bytes.NewReader(buf.Bytes())),
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != `<F>value</F>` {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestRender_bodyMissingContentType(t *testing.T) {
	cfg := mustConfig(t, `<T>{{ .req_body }}</T>`, "")
	_, _, err := cfg.render(logging.NoOp, "[TEST]", &proxy.Request{
		Body: io.NopCloser(strings.NewReader("data")),
	})
	if err != errMissingContentType {
		t.Fatalf("expected errMissingContentType, got %v", err)
	}
}

func TestParseConfig_inlineTemplate(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte(`<x>{{ .req_params.Id }}</x>`))
	cfg, err := parseConfig(&config.Backend{
		URLPattern: "/svc",
		ExtraConfig: config.ExtraConfig{
			Namespace: map[string]interface{}{
				"template":     encoded,
				"content_type": "application/xml",
			},
		},
	}, logging.NoOp, "[TEST]")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ContentType != "application/xml" {
		t.Fatalf("unexpected content type: %s", cfg.ContentType)
	}
}

func TestParseConfig_pathTemplate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "soap.xml")
	if err := os.WriteFile(path, []byte(`<x/>`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := parseConfig(&config.Backend{
		URLPattern: "/svc",
		ExtraConfig: config.ExtraConfig{
			Namespace: map[string]interface{}{
				"path": path,
			},
		},
	}, logging.NoOp, "[TEST]")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.holder.get() == nil {
		t.Fatal("expected parsed template")
	}
}

func TestBackendFactory_replacesBody(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte(`<soap>{{ .req_params.Code }}</soap>`))
	var captured *proxy.Request
	bf := BackendFactory(logging.NoOp, func(remote *config.Backend) proxy.Proxy {
		return func(_ context.Context, r *proxy.Request) (*proxy.Response, error) {
			captured = r
			return &proxy.Response{}, nil
		}
	})

	p := bf(&config.Backend{
		URLPattern: "/flag",
		ExtraConfig: config.ExtraConfig{
			Namespace: map[string]interface{}{
				"template": encoded,
			},
		},
	})

	_, err := p(context.Background(), &proxy.Request{
		Params: map[string]string{"Code": "DE"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if captured == nil {
		t.Fatal("inner proxy not called")
	}
	b, err := io.ReadAll(captured.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "<soap>DE</soap>" {
		t.Fatalf("unexpected body: %s", b)
	}
	if captured.Headers["Content-Type"][0] != "text/xml" {
		t.Fatalf("unexpected content-type: %v", captured.Headers["Content-Type"])
	}
}

func TestBackendFactory_setsSOAPAction(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte(`<soap>{{ .req_params.Code }}</soap>`))
	var captured *proxy.Request
	bf := BackendFactory(logging.NoOp, func(remote *config.Backend) proxy.Proxy {
		return func(_ context.Context, r *proxy.Request) (*proxy.Response, error) {
			captured = r
			return &proxy.Response{}, nil
		}
	})

	p := bf(&config.Backend{
		URLPattern: "/flag",
		ExtraConfig: config.ExtraConfig{
			Namespace: map[string]interface{}{
				"template":    encoded,
				"soap_action": "http://example.com/CountryFlag",
			},
		},
	})

	_, err := p(context.Background(), &proxy.Request{Params: map[string]string{"Code": "US"}})
	if err != nil {
		t.Fatal(err)
	}
	if captured.Headers["SOAPAction"][0] != `"http://example.com/CountryFlag"` {
		t.Fatalf("SOAPAction: %v", captured.Headers["SOAPAction"])
	}
}

func mustConfig(t *testing.T, tmpl, contentType string) *Config {
	t.Helper()
	ecfg := map[string]interface{}{
		"template": base64.StdEncoding.EncodeToString([]byte(tmpl)),
	}
	if contentType != "" {
		ecfg["content_type"] = contentType
	}
	cfg, err := parseConfig(&config.Backend{
		URLPattern: "/service.wso",
		ExtraConfig: config.ExtraConfig{
			Namespace: ecfg,
		},
	}, logging.NoOp, "[TEST]")
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func multipartWriter(t *testing.T, buf *bytes.Buffer) *multipartWriterCompat {
	t.Helper()
	return newMultipartWriter(buf)
}
