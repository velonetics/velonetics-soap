package soap

import (
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"time"

	"github.com/beevik/etree"
	dsig "github.com/russellhaering/goxmldsig"
)

const (
	wsseNS       = "http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd"
	wsuNS        = "http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-utility-1.0.xsd"
	passwordType = "http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-username-token-profile-1.0#PasswordText"
)

type UsernameTokenConfig struct {
	Username string
	Password string
}

type SAMLAssertionConfig struct {
	Path string
}

type X509Config struct {
	CertPath    string
	KeyPath     string
	KeyPassword string
}

type WSSecurityConfig struct {
	UsernameToken *UsernameTokenConfig
	SAMLAssertion *SAMLAssertionConfig
	X509          *X509Config
}

func (w *WSSecurityConfig) enabled() bool {
	if w == nil {
		return false
	}
	return w.UsernameToken != nil || w.SAMLAssertion != nil || w.X509 != nil
}

func applyWSSecurity(body []byte, cfg *WSSecurityConfig) ([]byte, error) {
	if cfg == nil || !cfg.enabled() {
		return body, nil
	}

	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(body); err != nil {
		return nil, fmt.Errorf("soap: ws-security parse: %w", err)
	}
	root := doc.Root()
	if root == nil {
		return nil, fmt.Errorf("soap: ws-security missing envelope")
	}

	header := findOrCreate(root, "Header")
	security := header.FindElement("./Security")
	if security == nil {
		security = header.CreateElement("Security")
	}
	security.CreateAttr("xmlns", wsseNS)

	if cfg.UsernameToken != nil {
		ut := security.CreateElement("UsernameToken")
		ut.CreateElement("Username").SetText(cfg.UsernameToken.Username)
		pw := ut.CreateElement("Password")
		pw.CreateAttr("Type", passwordType)
		pw.SetText(cfg.UsernameToken.Password)
	}

	if cfg.SAMLAssertion != nil {
		raw, err := os.ReadFile(cfg.SAMLAssertion.Path)
		if err != nil {
			return nil, err
		}
		assertionDoc := etree.NewDocument()
		if err := assertionDoc.ReadFromBytes(raw); err != nil {
			return nil, fmt.Errorf("soap: saml assertion parse: %w", err)
		}
		if assertionDoc.Root() != nil {
			security.AddChild(assertionDoc.Root())
		}
	}

	out, err := doc.WriteToBytes()
	if err != nil {
		return nil, err
	}

	if cfg.X509 != nil {
		out, err = signWithX509(out, cfg.X509)
		if err != nil {
			return nil, err
		}
	}

	return out, nil
}

func findOrCreate(root *etree.Element, local string) *etree.Element {
	el := root.FindElement("./" + local)
	if el == nil {
		el = root.FindElement(".//*[local-name()='" + local + "']")
	}
	if el == nil {
		el = root.CreateElement(local)
	}
	return el
}

func signWithX509(body []byte, cfg *X509Config) ([]byte, error) {
	certPEM, err := os.ReadFile(cfg.CertPath)
	if err != nil {
		return nil, err
	}
	keyPEM, err := os.ReadFile(cfg.KeyPath)
	if err != nil {
		return nil, err
	}
	keyPair, err := loadTLSKeyPair(certPEM, keyPEM, cfg.KeyPassword)
	if err != nil {
		return nil, err
	}

	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(body); err != nil {
		return nil, err
	}

	ctx := dsig.NewDefaultSigningContext(dsig.TLSCertKeyStore(keyPair))
	ctx.Hash = crypto.SHA256

	root := doc.Root()
	if root == nil {
		return nil, fmt.Errorf("soap: envelope root missing for signing")
	}

	bodyEl := root.FindElement(".//*[local-name()='Body']")
	if bodyEl != nil {
		id := fmt.Sprintf("body-%d", time.Now().UnixNano())
		bodyEl.CreateAttr("wsu:Id", id)
		bodyEl.CreateAttr("xmlns:wsu", wsuNS)
	}

	signed, err := ctx.SignEnveloped(root)
	if err != nil {
		return nil, err
	}
	out := etree.NewDocument()
	out.SetRoot(signed)
	return out.WriteToBytes()
}

func loadTLSKeyPair(certPEM, keyPEM []byte, password string) (tls.Certificate, error) {
	if password == "" {
		return tls.X509KeyPair(certPEM, keyPEM)
	}
	block, _ := pem.Decode(keyPEM)
	if block == nil {
		return tls.Certificate{}, fmt.Errorf("soap: failed to decode private key PEM")
	}
	if x509.IsEncryptedPEMBlock(block) {
		der, err := x509.DecryptPEMBlock(block, []byte(password))
		if err != nil {
			return tls.Certificate{}, fmt.Errorf("soap: decrypt private key: %w", err)
		}
		keyPEM = pem.EncodeToMemory(&pem.Block{Type: block.Type, Bytes: der})
	}
	return tls.X509KeyPair(certPEM, keyPEM)
}
