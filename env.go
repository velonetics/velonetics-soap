package soap

import (
	"os"
	"regexp"
	"strings"
)

var envPattern = regexp.MustCompile(`\{\{\s*env\s+"([^"]+)"\s*\}\}`)

func resolveEnvValue(s string) string {
	return envPattern.ReplaceAllStringFunc(s, func(match string) string {
		sub := envPattern.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		if v := os.Getenv(sub[1]); v != "" {
			return v
		}
		return ""
	})
}

func resolveEnvMap(m map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		switch t := v.(type) {
		case string:
			out[k] = resolveEnvValue(t)
		case map[string]interface{}:
			out[k] = resolveEnvMap(t)
		default:
			out[k] = v
		}
	}
	return out
}

func stringVal(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return resolveEnvValue(strings.TrimSpace(v))
	}
	return ""
}
