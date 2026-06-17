package soap

import (
	"bytes"
	"context"
	"io"
	"strconv"

	"github.com/velonetics/lura/v2/config"
	"github.com/velonetics/lura/v2/logging"
	"github.com/velonetics/lura/v2/proxy"
)

// BackendFactory returns a proxy.BackendFactory that wraps HTTP backends with SOAP
// template request body generation when backend/soap extra_config is present.
func BackendFactory(l logging.Logger, bf proxy.BackendFactory) proxy.BackendFactory {
	return func(remote *config.Backend) proxy.Proxy {
		logPrefix := "[BACKEND: " + remote.URLPattern + "][SOAP]"
		next := bf(remote)

		cfg, err := parseConfig(remote, l, logPrefix)
		if err != nil {
			if err != errNoConfig {
				l.Error(logPrefix, err)
			}
			return next
		}

		l.Debug(logPrefix, "Component enabled")

		return func(ctx context.Context, r *proxy.Request) (*proxy.Response, error) {
			body, _, err := cfg.render(l, logPrefix, r)
			if err != nil {
				return nil, err
			}

			r.Body = io.NopCloser(bytes.NewReader(body))
			if r.Headers == nil {
				r.Headers = make(map[string][]string)
			}
			r.Headers["Content-Type"] = []string{cfg.ContentType}
			r.Headers["Content-Length"] = []string{strconv.Itoa(len(body))}

			if action := soapActionHeaderValue(cfg.SOAPAction, cfg.WSDLAction); action != "" {
				r.Headers["SOAPAction"] = []string{action}
			}

			return next(ctx, r)
		}
	}
}
