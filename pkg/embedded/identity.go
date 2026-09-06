// Package embedded implements the v1 embedded identity token contract.
// Identity is not a resource grant. Callers must check current authorization.
package embedded

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"
)

const TTL = time.Minute
const TokenType = "emisell-embedded-id+jwt"

var ErrInvalid = errors.New("invalid embedded identity")

type Identity struct {
	MerchantID     string `json:"merchantId"`
	ActorID        string `json:"sub"`
	AppID          string `json:"appId"`
	InstallationID string `json:"installationId"`
}

type Claims struct {
	Identity
	Issuer    string `json:"iss"`
	Audience  string `json:"aud"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
	ID        string `json:"jti"`
}

type header struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
	KeyID     string `json:"kid"`
}

func valid(v string) bool {
	if len(v) == 0 || len(v) > 256 {
		return false
	}
	for _, r := range v {
		if r < 33 || r > 126 {
			return false
		}
	}
	return true
}

func (i Identity) Valid() bool {
	return valid(i.MerchantID) && valid(i.ActorID) && valid(i.AppID) && valid(i.InstallationID)
}

// Issue must only be called after current staff membership, installation,
// approved launch binding and grant have been authorized server-side.
func Issue(key ed25519.PrivateKey, keyID, issuer, audience string, id Identity, now time.Time) (string, error) {
	if len(key) != ed25519.PrivateKeySize || !valid(keyID) || !valid(issuer) || !valid(audience) || !id.Valid() || now.Unix() <= 0 {
		return "", ErrInvalid
	}
	nonce := make([]byte, 24)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	h, _ := json.Marshal(header{"EdDSA", TokenType, keyID})
	c, _ := json.Marshal(Claims{id, issuer, audience, now.Unix(), now.Add(TTL).Unix(), base64.RawURLEncoding.EncodeToString(nonce)})
	input := base64.RawURLEncoding.EncodeToString(h) + "." + base64.RawURLEncoding.EncodeToString(c)
	return input + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(key, []byte(input))), nil
}

func decode(s string, dst any) error {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil || base64.RawURLEncoding.EncodeToString(b) != s {
		return ErrInvalid
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(dst) != nil {
		return ErrInvalid
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return ErrInvalid
	}
	return nil
}

// Verify pins issuer, app-client audience, identity and locally trusted keys.
// It never follows token-supplied URLs. Signature validity does not establish
// current membership or grant: use the application's current-access check too.
func Verify(token string, keys map[string]ed25519.PublicKey, issuer, audience string, expected Identity, now time.Time) (Claims, error) {
	var c Claims
	if len(token) > 4096 || !expected.Valid() || !valid(issuer) || !valid(audience) {
		return c, ErrInvalid
	}
	p := strings.Split(token, ".")
	if len(p) != 3 {
		return c, ErrInvalid
	}
	var h header
	if decode(p[0], &h) != nil || h.Algorithm != "EdDSA" || h.Type != TokenType || !valid(h.KeyID) {
		return c, ErrInvalid
	}
	k := keys[h.KeyID]
	sig, err := base64.RawURLEncoding.DecodeString(p[2])
	if err != nil || len(k) != ed25519.PublicKeySize || !ed25519.Verify(k, []byte(p[0]+"."+p[1]), sig) {
		return c, ErrInvalid
	}
	if decode(p[1], &c) != nil || c.Identity != expected || c.Issuer != issuer || c.Audience != audience || c.IssuedAt <= 0 || c.IssuedAt > now.Unix() || c.ExpiresAt <= now.Unix() || c.ExpiresAt-c.IssuedAt != int64(TTL/time.Second) || len(c.ID) != 32 {
		return Claims{}, ErrInvalid
	}
	return c, nil
}
