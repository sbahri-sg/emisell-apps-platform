package endpointproof

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"net/url"
	"strings"
	"time"
)

// NewDevelopment accepts operator-owned configuration, never app-supplied input.
// It replaces public verification with one .test host, loopback:443, and an
// exact leaf certificate. Ordinary TLS chain, hostname and expiry checks remain.
func NewDevelopment(environment, origin string, certificatePEM []byte) (*Verifier, error) {
	u, err := url.Parse(origin)
	if err != nil || environment != "development" || u.Scheme != "https" || u.Host != u.Hostname() || !hostname.MatchString(u.Hostname()) || !strings.HasSuffix(u.Hostname(), ".test") || u.User != nil || u.Path != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.Contains(origin, "#") {
		return nil, denied
	}
	block, rest := pem.Decode(certificatePEM)
	if block == nil || block.Type != "CERTIFICATE" || len(strings.TrimSpace(string(rest))) != 0 {
		return nil, denied
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	now := time.Now()
	if err != nil || cert.IsCA || cert.VerifyHostname(u.Hostname()) != nil || now.Before(cert.NotBefore) || !now.Before(cert.NotAfter) {
		return nil, denied
	}
	v := New()
	v.localHost = u.Hostname()
	v.roots = x509.NewCertPool()
	v.roots.AddCert(cert)
	pin := sha256.Sum256(cert.Raw)
	v.localTLS = func(s tls.ConnectionState) error {
		if len(s.PeerCertificates) == 0 || sha256.Sum256(s.PeerCertificates[0].Raw) != pin {
			return denied
		}
		return nil
	}
	return v, nil
}
