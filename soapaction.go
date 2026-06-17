package soap

import "strings"

func soapActionHeaderValue(explicit, wsdlDerived string) string {
	action := strings.TrimSpace(explicit)
	if action == "" {
		action = strings.TrimSpace(wsdlDerived)
	}
	if action == "" {
		return ""
	}
	if strings.HasPrefix(action, `"`) && strings.HasSuffix(action, `"`) {
		return action
	}
	return `"` + action + `"`
}
