package soap

import (
	"fmt"

	"github.com/velonetics/lura/v2/config"
	"github.com/velonetics/lura/v2/logging"
)

// ValidateConfig checks backend/soap settings at startup.
func ValidateConfig(cfg *config.ServiceConfig) error {
	if cfg == nil {
		return nil
	}
	logger := logging.NoOp
	for _, ep := range cfg.Endpoints {
		for _, b := range ep.Backend {
			if _, ok := b.ExtraConfig[Namespace]; !ok {
				continue
			}
			if _, err := parseConfig(b, logger, "[VALIDATE: SOAP]", false); err != nil {
				return fmt.Errorf("endpoint %q backend %q: %w", ep.Endpoint, b.URLPattern, err)
			}
		}
	}
	return nil
}
