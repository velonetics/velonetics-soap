package soap

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/url"
	"strings"

	"github.com/clbanning/mxj/v2"
	"github.com/pucora/lura/v2/proxy"
	"golang.org/x/net/html/charset"
)

var (
	errUnsupportedContentType = errors.New("soap: unsupported request body content type")
	errMissingContentType     = errors.New("soap: Content-Type header required to parse request body")
)

func readRequestBody(r *proxy.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	defer r.Body.Close()
	return io.ReadAll(r.Body)
}

func restoreRequestBody(r *proxy.Request, body []byte) {
	if len(body) == 0 {
		r.Body = nil
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
}

func contentTypeFromHeaders(headers map[string][]string) string {
	for k, vs := range headers {
		if strings.EqualFold(k, "Content-Type") && len(vs) > 0 {
			ct, _, err := mime.ParseMediaType(vs[0])
			if err != nil {
				return strings.TrimSpace(vs[0])
			}
			return ct
		}
	}
	return ""
}

func parseRequestBody(headers map[string][]string, body []byte) (interface{}, error) {
	if len(body) == 0 {
		return nil, nil
	}

	ct := contentTypeFromHeaders(headers)
	if ct == "" {
		return nil, errMissingContentType
	}

	switch ct {
	case "application/json":
		var data map[string]interface{}
		if err := json.Unmarshal(body, &data); err != nil {
			return nil, err
		}
		return data, nil
	case "application/xml", "text/xml":
		mxj.XmlCharsetReader = charset.NewReaderLabel
		mv, err := mxj.NewMapXml(body)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}(mv), nil
	case "application/x-www-form-urlencoded":
		values, err := url.ParseQuery(string(body))
		if err != nil {
			return nil, err
		}
		flat := make(map[string]string, len(values))
		for k, vs := range values {
			if len(vs) > 0 {
				flat[k] = vs[0]
			}
		}
		return flat, nil
	case "multipart/form-data":
		var params map[string]string
		for k, vs := range headers {
			if strings.EqualFold(k, "Content-Type") && len(vs) > 0 {
				_, p, err := mime.ParseMediaType(vs[0])
				if err != nil {
					return nil, err
				}
				params = p
				break
			}
		}
		boundary := params["boundary"]
		if boundary == "" {
			return nil, errUnsupportedContentType
		}
		reader := multipart.NewReader(bytes.NewReader(body), boundary)
		flat := make(map[string]string)
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, err
			}
			name := part.FormName()
			if name == "" {
				continue
			}
			b, err := io.ReadAll(part)
			part.Close()
			if err != nil {
				return nil, err
			}
			flat[name] = string(b)
		}
		return flat, nil
	case "text/plain":
		return string(body), nil
	default:
		return nil, errUnsupportedContentType
	}
}

func flattenQueryString(query map[string][]string) map[string]string {
	if len(query) == 0 {
		return map[string]string{}
	}
	flat := make(map[string]string, len(query))
	for k, vs := range query {
		if len(vs) > 0 {
			flat[k] = vs[0]
		}
	}
	return flat
}

func buildTemplateData(r *proxy.Request, urlPattern string, bodyBytes []byte) (map[string]interface{}, error) {
	reqBody, err := parseRequestBody(r.Headers, bodyBytes)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"req_params":      r.Params,
		"req_headers":     r.Headers,
		"req_querystring": flattenQueryString(r.Query),
		"req_path":        urlPattern,
		"req_body":        reqBody,
	}, nil
}
