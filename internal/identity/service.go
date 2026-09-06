package identity

import (
	"context"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"emisell.app/platform/internal/platform/fault"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"time"
)

type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}
type Workspace struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type Repository interface {
	FindUser(context.Context, string) (User, string, error)
	SaveSession(context.Context, string, string, time.Time) error
	SessionUser(context.Context, string) (User, error)
	DeleteSession(context.Context, string) error
	Workspaces(context.Context, string) ([]Workspace, error)
	HasMembership(context.Context, string, string) (bool, error)
}
type Authorizer interface {
	Authorize(context.Context, string, string) error
}
type Service struct{ Repo Repository }

func HashPassword(password string) (string, error) {
	if len(password) < 12 || len(password) > 256 {
		return "", fault.Invalid
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, 600000, 32)
	if err != nil {
		return "", err
	}
	return "pbkdf2-sha256$600000$" + base64.RawStdEncoding.EncodeToString(salt) + "$" + base64.RawStdEncoding.EncodeToString(key), nil
}
func VerifyPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" || parts[1] != "600000" || len(password) > 256 {
		return false
	}
	salt, e1 := base64.RawStdEncoding.DecodeString(parts[2])
	expected, e2 := base64.RawStdEncoding.DecodeString(parts[3])
	if e1 != nil || e2 != nil || len(salt) != 16 || len(expected) != 32 {
		return false
	}
	actual, err := pbkdf2.Key(sha256.New, password, salt, 600000, 32)
	return err == nil && subtle.ConstantTimeCompare(actual, expected) == 1
}
func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
func (s Service) Login(ctx context.Context, email, password string) (User, string, error) {
	if len(email) > 254 || len(password) > 256 {
		return User{}, "", fault.Unauthenticated
	}
	u, hash, err := s.Repo.FindUser(ctx, strings.ToLower(strings.TrimSpace(email)))
	if err != nil {
		if err != fault.NotFound {
			return User{}, "", err
		}
		hash = "pbkdf2-sha256$600000$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	}
	valid := VerifyPassword(password, hash)
	if err != nil || !valid {
		return User{}, "", fault.Unauthenticated
	}
	tokenBytes := make([]byte, 32)
	if _, err = rand.Read(tokenBytes); err != nil {
		return User{}, "", err
	}
	token := base64.RawURLEncoding.EncodeToString(tokenBytes)
	if err = s.Repo.SaveSession(ctx, tokenHash(token), u.ID, time.Now().Add(8*time.Hour)); err != nil {
		return User{}, "", err
	}
	return u, token, nil
}
func (s Service) Authenticate(ctx context.Context, token string) (User, error) {
	if len(token) != 43 {
		return User{}, fault.Unauthenticated
	}
	return s.Repo.SessionUser(ctx, tokenHash(token))
}
func (s Service) Logout(ctx context.Context, token string) error {
	return s.Repo.DeleteSession(ctx, tokenHash(token))
}
func (s Service) Workspaces(ctx context.Context, user string) ([]Workspace, error) {
	return s.Repo.Workspaces(ctx, user)
}
func (s Service) Authorize(ctx context.Context, user, tenant string) error {
	if user == "" {
		return fault.Unauthenticated
	}
	ok, err := s.Repo.HasMembership(ctx, user, tenant)
	if err != nil {
		return err
	}
	if !ok {
		return fault.NotFound
	}
	return nil
}
