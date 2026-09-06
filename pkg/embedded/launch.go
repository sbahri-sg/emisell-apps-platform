package embedded

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/url"
)

// Launch is immutable reviewed release metadata, not browser input.
type Launch struct {
	AppID         string `json:"appId"`
	ClientID      string `json:"clientId"`
	ReleaseDigest string `json:"releaseDigest"`
	URL           string `json:"url"`
	ParentOrigin  string `json:"parentOrigin"`
	// Empty preserves signed historical embedded metadata byte-for-byte.
	Mode string `json:"mode,omitempty"`
}

func (l Launch) DisplayMode() string {
	if l.Mode == "" {
		return "embedded"
	}
	return l.Mode
}

func origin(raw string, local bool) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
		return nil, ErrInvalid
	}
	if u.Scheme != "https" && !(local && u.Scheme == "http" && u.Hostname() == "127.0.0.1") {
		return nil, ErrInvalid
	}
	return u, nil
}

func (l Launch) Validate(local bool) error {
	if l.DisplayMode() != "embedded" && l.DisplayMode() != "external" {
		return ErrInvalid
	}
	a, err := origin(l.URL, local)
	if err != nil {
		return err
	}
	p, err := origin(l.ParentOrigin, local)
	if err != nil {
		return err
	}
	if p.Path != "" || p.RawPath != "" || !valid(l.AppID) || !valid(l.ClientID) || len(l.ReleaseDigest) != 64 || a.Scheme+"://"+a.Host == l.ParentOrigin {
		return ErrInvalid
	}
	for _, c := range l.ReleaseDigest {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return ErrInvalid
		}
	}
	return nil
}

func launchBytes(l Launch) []byte {
	b, _ := json.Marshal(l)
	return append([]byte("emisell.embedded-launch/v1\x00"), b...)
}

func SignLaunch(l Launch, key ed25519.PrivateKey, local bool) (string, error) {
	if l.Validate(local) != nil || len(key) != ed25519.PrivateKeySize {
		return "", ErrInvalid
	}
	return base64.RawURLEncoding.EncodeToString(ed25519.Sign(key, launchBytes(l))), nil
}

func VerifyLaunch(l Launch, signature string, key ed25519.PublicKey, local bool) error {
	if l.Validate(local) != nil || len(key) != ed25519.PublicKeySize {
		return ErrInvalid
	}
	b, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil || !ed25519.Verify(key, launchBytes(l), b) {
		return ErrInvalid
	}
	return nil
}
