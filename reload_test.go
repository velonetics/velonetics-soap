package soap

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/velonetics/lura/v2/logging"
)

func TestTemplateReloadOnChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "t.xml")
	if err := os.WriteFile(path, []byte(`<x>{{ .req_params.Id }}</x>`), 0o644); err != nil {
		t.Fatal(err)
	}

	h := newTemplateHolder(logging.NoOp, "[TEST]")
	if err := h.loadFromPath(path); err != nil {
		t.Fatal(err)
	}
	h.startBackground(true, 200*time.Millisecond)
	defer h.close()

	time.Sleep(100 * time.Millisecond)
	if err := os.WriteFile(path, []byte(`<x>reloaded</x>`), 0o644); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(3 * time.Second)
	var out string
	for time.Now().Before(deadline) {
		tmpl := h.get()
		if tmpl == nil {
			time.Sleep(50 * time.Millisecond)
			continue
		}
		var buf bytes.Buffer
		_ = tmpl.Execute(&buf, map[string]interface{}{"req_params": map[string]string{"Id": "1"}})
		out = buf.String()
		if out == "<x>reloaded</x>" {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("template not reloaded, last output: %q", out)
}
