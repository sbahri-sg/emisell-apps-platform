// Package recovery holds the shared request primitive, not module business policy.
package recovery

import (
	"crypto/sha256"
	"emisell.app/platform/internal/platform/fault"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

type Request struct {
	Reason           string `json:"reason"`
	ExpectedRevision int64  `json:"expectedRevision"`
}
type Result struct {
	Status   string `json:"status"`
	Revision int64  `json:"revision"`
}

var keyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{16,128}$`)

func (r Request) Validate(key string) error {
	if !keyPattern.MatchString(key) || r.ExpectedRevision < 0 || !utf8.ValidString(r.Reason) || len(strings.TrimSpace(r.Reason)) < 8 || len(r.Reason) > 240 {
		return fault.Invalid
	}
	for _, c := range r.Reason {
		if unicode.IsControl(c) {
			return fault.Invalid
		}
	}
	return nil
}
func (r Request) Hash(id string) string {
	raw, _ := json.Marshal(struct {
		ID      string
		Request Request
	}{id, r})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
