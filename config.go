package soap

import (
	"encoding/base64"
	"errors"
	"time"

	"github.com/pucora/lura/v2/config"
	"github.com/pucora/lura/v2/logging"
)

const (
	Namespace          = "github.com/pucora/velonetics-soap/v2"
	defaultContentType = "text/xml"
)

var (
	errNoConfig        = errors.New("soap: no extra config defined")
	errBadConfig       = errors.New("soap: unable to parse extra config")
	errMissingTemplate = errors.New("soap: path, template, or wsdl.generate_template required")
	errLoadTemplate    = errors.New("soap: unable to load template")
)

// Config holds parsed backend/soap settings for a backend.
type Config struct {
	ContentType   string
	Debug         bool
	URLPattern    string
	SOAPAction    string
	WSDLAction    string
	WSDLInfo      *WSDLInfo
	WSSecurity    *WSSecurityConfig
	holder        *templateHolder
}

func parseConfig(remote *config.Backend, logger logging.Logger, logPrefix string, startBackground bool) (*Config, error) {
	v, ok := remote.ExtraConfig[Namespace]
	if !ok {
		return nil, errNoConfig
	}

	ecfg, ok := v.(map[string]interface{})
	if !ok {
		return nil, errBadConfig
	}
	ecfg = resolveEnvMap(ecfg)

	cfg := &Config{
		ContentType: defaultContentType,
		URLPattern:  remote.URLPattern,
		holder:      newTemplateHolder(logger, logPrefix),
	}

	if ct, ok := ecfg["content_type"].(string); ok && ct != "" {
		cfg.ContentType = ct
	}
	if debug, ok := ecfg["debug"].(bool); ok {
		cfg.Debug = debug
	}
	cfg.SOAPAction = stringVal(ecfg, "soap_action")

	var wsdlInfo *WSDLInfo
	if wsdlRaw, ok := ecfg["wsdl"].(map[string]interface{}); ok {
		data, err := loadWSDL(stringVal(wsdlRaw, "path"), stringVal(wsdlRaw, "url"))
		if err != nil {
			return nil, err
		}
		op := stringVal(wsdlRaw, "operation")
		wsdlInfo, err = parseWSDL(data, stringVal(wsdlRaw, "service"), stringVal(wsdlRaw, "port"), op)
		if err != nil {
			return nil, err
		}
		cfg.WSDLInfo = wsdlInfo
		cfg.WSDLAction = wsdlInfo.SOAPAction
		if host, path := splitLocation(wsdlInfo.Location); host != "" {
			logger.Debug(logPrefix, "WSDL location hint:", host+path)
		}

		gen, _ := wsdlRaw["generate_template"].(bool)
		if gen && stringVal(ecfg, "path") == "" && stringVal(ecfg, "template") == "" {
			if err := cfg.holder.loadFromRaw(generateWSDLTemplate(wsdlInfo)); err != nil {
				return nil, errLoadTemplate
			}
		}
	}

	if wsRaw, ok := ecfg["ws_security"].(map[string]interface{}); ok {
		cfg.WSSecurity = parseWSSecurity(wsRaw)
	}

	path := stringVal(ecfg, "path")
	encoded := stringVal(ecfg, "template")

	if cfg.holder.get() == nil {
		switch {
		case path != "":
			if err := cfg.holder.loadFromPath(path); err != nil {
				return nil, errLoadTemplate
			}
		case encoded != "":
			b, err := base64.StdEncoding.DecodeString(encoded)
			if err != nil {
				return nil, errLoadTemplate
			}
			if err := cfg.holder.setInline(string(b)); err != nil {
				return nil, errLoadTemplate
			}
		default:
			return nil, errMissingTemplate
		}
	}

	watch, _ := ecfg["watch_template"].(bool)
	var interval time.Duration
	if s := stringVal(ecfg, "reload_interval"); s != "" {
		d, err := time.ParseDuration(s)
		if err != nil {
			return nil, errBadConfig
		}
		interval = d
	}
	if startBackground {
		cfg.holder.startBackground(watch, interval)
	}

	return cfg, nil
}

func parseWSSecurity(m map[string]interface{}) *WSSecurityConfig {
	cfg := &WSSecurityConfig{}
	if ut, ok := m["username_token"].(map[string]interface{}); ok {
		cfg.UsernameToken = &UsernameTokenConfig{
			Username: stringVal(ut, "username"),
			Password: stringVal(ut, "password"),
		}
	}
	if saml, ok := m["saml_assertion"].(map[string]interface{}); ok {
		if p := stringVal(saml, "path"); p != "" {
			cfg.SAMLAssertion = &SAMLAssertionConfig{Path: p}
		}
	}
	if x509, ok := m["x509"].(map[string]interface{}); ok {
		cfg.X509 = &X509Config{
			CertPath:    stringVal(x509, "cert_path"),
			KeyPath:     stringVal(x509, "key_path"),
			KeyPassword: stringVal(x509, "key_password"),
		}
	}
	if !cfg.enabled() {
		return nil
	}
	return cfg
}

func (c *Config) close() {
	if c.holder != nil {
		c.holder.close()
	}
}
