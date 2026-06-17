package soap

import (
	"bytes"
	"encoding/json"

	"github.com/velonetics/lura/v2/logging"
	"github.com/velonetics/lura/v2/proxy"
)

func (c *Config) render(l logging.Logger, logPrefix string, r *proxy.Request) ([]byte, map[string]interface{}, error) {
	bodyBytes, err := readRequestBody(r)
	if err != nil {
		return nil, nil, err
	}
	restoreRequestBody(r, bodyBytes)

	data, err := buildTemplateData(r, c.URLPattern, bodyBytes)
	if err != nil {
		return nil, nil, err
	}

	if c.Debug {
		if encoded, err := json.MarshalIndent(data, " ", " "); err == nil {
			l.Debug(logPrefix, "Template variables:\n", string(encoded))
		}
	}

	tmpl := c.holder.get()
	if tmpl == nil {
		return nil, nil, errLoadTemplate
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, nil, err
	}

	out := buf.Bytes()
	out, err = applyWSSecurity(out, c.WSSecurity)
	if err != nil {
		return nil, nil, err
	}

	if c.Debug {
		l.Debug(logPrefix, "Generated content-type:", c.ContentType)
		l.Debug(logPrefix, "Generated body:\n", string(out))
	}

	return out, data, nil
}
