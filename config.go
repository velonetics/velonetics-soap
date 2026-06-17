package soap

import (
	"encoding/base64"
	"errors"
	"os"
	"text/template"

	"github.com/velonetics/lura/v2/config"
)

const (
	Namespace            = "github.com/velonetics/velonetics-soap/v2"
	defaultContentType   = "text/xml"
)

var (
	errNoConfig       = errors.New("soap: no extra config defined")
	errBadConfig      = errors.New("soap: unable to parse extra config")
	errMissingTemplate = errors.New("soap: path or template is required")
	errLoadTemplate   = errors.New("soap: unable to load template")
)

// Config holds parsed backend/soap settings for a backend.
type Config struct {
	ContentType string
	Debug       bool
	tmpl        *template.Template
	URLPattern  string
}

func parseConfig(remote *config.Backend) (*Config, error) {
	v, ok := remote.ExtraConfig[Namespace]
	if !ok {
		return nil, errNoConfig
	}

	ecfg, ok := v.(map[string]interface{})
	if !ok {
		return nil, errBadConfig
	}

	cfg := &Config{
		ContentType: defaultContentType,
		URLPattern:  remote.URLPattern,
	}

	if ct, ok := ecfg["content_type"].(string); ok && ct != "" {
		cfg.ContentType = ct
	}
	if debug, ok := ecfg["debug"].(bool); ok {
		cfg.Debug = debug
	}

	var raw string
	if path, ok := ecfg["path"].(string); ok && path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, errLoadTemplate
		}
		raw = string(b)
	} else if encoded, ok := ecfg["template"].(string); ok && encoded != "" {
		b, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, errLoadTemplate
		}
		raw = string(b)
	} else {
		return nil, errMissingTemplate
	}

	tmpl, err := template.New("soap").Parse(raw)
	if err != nil {
		return nil, errLoadTemplate
	}
	cfg.tmpl = tmpl

	return cfg, nil
}
