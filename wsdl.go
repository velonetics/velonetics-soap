package soap

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

// WSDLInfo holds parsed WSDL metadata for a SOAP operation.
type WSDLInfo struct {
	Location   string
	SOAPAction string
	Namespace  string
	Operation  string
}

type wsdlDefinitions struct {
	XMLName  xml.Name         `xml:"definitions"`
	Services []wsdlService      `xml:"service"`
	Bindings []wsdlBinding      `xml:"binding"`
	Messages []wsdlMessage      `xml:"message"`
	Ports    []wsdlPortType     `xml:"portType"`
	Imports  []wsdlImport       `xml:"import"`
}

type wsdlImport struct {
	Location string `xml:"location,attr"`
}

type wsdlService struct {
	Name  string     `xml:"name,attr"`
	Ports []wsdlPort `xml:"port"`
}

type wsdlPort struct {
	Name    string      `xml:"name,attr"`
	Binding string      `xml:"binding,attr"`
	Address wsdlAddress `xml:"http://schemas.xmlsoap.org/wsdl/soap/ address"`
}

type wsdlAddress struct {
	Location string `xml:"location,attr"`
}

type wsdlBinding struct {
	Name        string          `xml:"name,attr"`
	Type        string          `xml:"type,attr"`
	SoapBinding wsdlSoapBinding `xml:"http://schemas.xmlsoap.org/wsdl/soap/ binding"`
	Operations  []wsdlBindingOp `xml:"operation"`
}

type wsdlSoapBinding struct {
	Transport string `xml:"transport,attr"`
}

type wsdlBindingOp struct {
	Name          string            `xml:"name,attr"`
	SoapOperation wsdlSoapOperation `xml:"http://schemas.xmlsoap.org/wsdl/soap/ operation"`
}

type wsdlSoapOperation struct {
	SOAPAction string `xml:"soapAction,attr"`
}

type wsdlPortType struct {
	Name       string           `xml:"name,attr"`
	Operations []wsdlPortTypeOp `xml:"operation"`
}

type wsdlPortTypeOp struct {
	Name string `xml:"name,attr"`
}

type wsdlMessage struct {
	Name string `xml:"name,attr"`
}

func loadWSDL(path, rawURL string) ([]byte, error) {
	if path != "" {
		return os.ReadFile(path)
	}
	if rawURL == "" {
		return nil, fmt.Errorf("soap: wsdl path or url required")
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(rawURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("soap: wsdl fetch status %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func parseWSDL(data []byte, serviceName, portName, operationName string) (*WSDLInfo, error) {
	var defs wsdlDefinitions
	if err := xml.Unmarshal(data, &defs); err != nil {
		return nil, err
	}

	info := &WSDLInfo{Operation: operationName}

	svc := pickService(defs.Services, serviceName)
	if svc == nil {
		return nil, fmt.Errorf("soap: wsdl service not found")
	}
	port := pickPort(svc.Ports, portName)
	if port == nil {
		return nil, fmt.Errorf("soap: wsdl port not found")
	}
	info.Location = port.Address.Location
	if info.Location == "" {
		info.Location = regexLocation(string(data))
	}

	bindingName := localName(port.Binding)
	binding := pickBinding(defs.Bindings, bindingName)
	if binding == nil {
		return nil, fmt.Errorf("soap: wsdl binding %q not found", bindingName)
	}

	for _, op := range binding.Operations {
		if op.Name == operationName {
			info.SOAPAction = strings.TrimSpace(op.SoapOperation.SOAPAction)
			break
		}
	}
	if info.SOAPAction == "" && operationName != "" {
		info.SOAPAction = regexSOAPAction(string(data), operationName)
	}
	if info.SOAPAction == "" && operationName != "" {
		return nil, fmt.Errorf("soap: wsdl operation %q not found", operationName)
	}

	info.Namespace = guessNamespace(defs, binding, operationName)
	return info, nil
}

func pickService(services []wsdlService, name string) *wsdlService {
	if len(services) == 0 {
		return nil
	}
	if name == "" {
		return &services[0]
	}
	for i := range services {
		if services[i].Name == name {
			return &services[i]
		}
	}
	return nil
}

func pickPort(ports []wsdlPort, name string) *wsdlPort {
	if len(ports) == 0 {
		return nil
	}
	if name == "" {
		return &ports[0]
	}
	for i := range ports {
		if ports[i].Name == name {
			return &ports[i]
		}
	}
	return nil
}

func pickBinding(bindings []wsdlBinding, name string) *wsdlBinding {
	for i := range bindings {
		if bindings[i].Name == name {
			return &bindings[i]
		}
	}
	return nil
}

func localName(qname string) string {
	if idx := strings.Index(qname, ":"); idx >= 0 {
		return qname[idx+1:]
	}
	if idx := strings.LastIndex(qname, ":"); idx >= 0 {
		return qname[idx+1:]
	}
	return qname
}

func guessNamespace(defs wsdlDefinitions, binding *wsdlBinding, operation string) string {
	typeName := localName(binding.Type)
	for _, pt := range defs.Ports {
		if pt.Name == typeName {
			for _, op := range pt.Operations {
				if op.Name == operation {
					return "http://www.example.com/" + strings.ToLower(typeName)
				}
			}
		}
	}
	return "http://www.example.com/soap"
}

func splitLocation(location string) (host string, path string) {
	u, err := url.Parse(location)
	if err != nil {
		return "", location
	}
	host = u.Scheme + "://" + u.Host
	path = u.Path
	if path == "" {
		path = "/"
	}
	return host, path
}

var (
	reSOAPAction = regexp.MustCompile(`(?i)soapAction\s*=\s*"([^"]+)"`)
	reLocation   = regexp.MustCompile(`(?i)location\s*=\s*"([^"]+)"`)
)

func regexSOAPAction(data, operation string) string {
	// Prefer operation block match when possible
	opPattern := regexp.MustCompile(`(?is)<operation[^>]*name\s*=\s*"` + regexp.QuoteMeta(operation) + `"[^>]*>.*?soapAction\s*=\s*"([^"]+)"`)
	if m := opPattern.FindStringSubmatch(data); len(m) > 1 {
		return m[1]
	}
	if m := reSOAPAction.FindStringSubmatch(data); len(m) > 1 {
		return m[1]
	}
	return ""
}

func regexLocation(data string) string {
	if m := reLocation.FindStringSubmatch(data); len(m) > 1 {
		return m[1]
	}
	return ""
}

func generateWSDLTemplate(info *WSDLInfo) string {
	op := info.Operation
	ns := info.Namespace
	if ns == "" {
		ns = "http://www.example.com/soap"
	}
	return fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Body>
    <%s xmlns="%s">
      <!-- inject fields using template variables -->
    </%s>
  </soap:Body>
</soap:Envelope>
`, op, ns, op)
}
