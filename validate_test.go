package soap

import (
	"testing"

	"github.com/pucora/lura/v2/config"
)

func TestValidateConfig_missingTemplate(t *testing.T) {
	cfg := &config.ServiceConfig{
		Endpoints: []*config.EndpointConfig{{
			Endpoint: "/soap",
			Backend: []*config.Backend{{
				URLPattern: "/ignored",
				ExtraConfig: config.ExtraConfig{
					Namespace: map[string]interface{}{},
				},
			}},
		}},
	}
	if err := ValidateConfig(cfg); err == nil {
		t.Fatal("expected validation error for missing template")
	}
}

func TestValidateConfig_noSOAP(t *testing.T) {
	if err := ValidateConfig(&config.ServiceConfig{}); err != nil {
		t.Fatal(err)
	}
}
