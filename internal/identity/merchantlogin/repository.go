package merchantlogin

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"emisell.app/platform/internal/app/contact"
	"emisell.app/platform/internal/identity"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/internal/platform/ids"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	CommonID string `json:"commonId"`
}
type Assertion struct {
	Request string  `json:"request"`
	Subject string  `json:"subject"`
	Email   string  `json:"email"`
	Name    string  `json:"name"`
	Stores  []Store `json:"stores"`
}
type Profile struct {
	Email  string  `json:"email"`
	Name   string  `json:"name"`
	Stores []Store `json:"stores"`
}
type Repository struct{ Pool *pgxpool.Pool }

var identifier = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
var requestID = regexp.MustCompile(`^[A-Z2-7]{52}$`)

func Token() string { return rand.Text() + rand.Text() }
func Hash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func (p Repository) Start(ctx context.Context, origin string) (string, string, error) {
	return p.start(ctx, origin, "browser")
}

func (p Repository) StartCLI(ctx context.Context, origin string) (string, string, error) {
	return p.start(ctx, origin, "cli")
}

func (p Repository) start(ctx context.Context, origin, kind string) (string, string, error) {
	request, verifier := Token(), Token()
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return "", "", err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `DELETE FROM platform_identity.developer_login_requests WHERE expires_at<now()`); err != nil {
		return "", "", err
	}
	_, err = tx.Exec(ctx, `INSERT INTO platform_identity.developer_login_requests(id,verifier_hash,portal_origin,expires_at,login_kind) VALUES($1,$2,$3,now()+interval '5 minutes',$4)`, request, Hash(verifier), origin, kind)
	if err != nil {
		return "", "", err
	}
	return request, verifier, tx.Commit(ctx)
}

func ValidAssertion(a Assertion) bool {
	if !requestID.MatchString(a.Request) || !identifier.MatchString(a.Subject) || !contact.ValidEmail(a.Email) || len(a.Name) == 0 || len(a.Name) > 200 || len(a.Stores) == 0 || len(a.Stores) > 200 {
		return false
	}
	seen := map[string]bool{}
	for _, s := range a.Stores {
		if !identifier.MatchString(s.ID) || !identifier.MatchString(s.CommonID) || len(s.Name) == 0 || len(s.Name) > 200 || seen[s.ID] {
			return false
		}
		seen[s.ID] = true
	}
	return true
}

// Only the authenticated first-party backend may attest this identity. This does not issue app grants.
func (p Repository) Approve(ctx context.Context, a Assertion) (string, string, error) {
	if !ValidAssertion(a) {
		return "", "", fault.Invalid
	}
	a.Email = strings.ToLower(a.Email)
	raw, _ := json.Marshal(a.Stores)
	code := Token()
	var origin string
	err := p.Pool.QueryRow(ctx, `UPDATE platform_identity.developer_login_requests SET core_subject=$2,core_email=$3,display_name=$4,stores=$5,code_hash=$6 WHERE id=$1 AND expires_at>now() AND code_hash IS NULL RETURNING portal_origin`, a.Request, a.Subject, a.Email, a.Name, raw, Hash(code)).Scan(&origin)
	if errors.Is(err, pgx.ErrNoRows) {
		err = fault.Conflict
	}
	return origin, code, err
}

func (p Repository) Finish(ctx context.Context, request, verifier, code, origin string) (string, error) {
	if !requestID.MatchString(request) || !requestID.MatchString(verifier) || !requestID.MatchString(code) {
		return "", fault.Unauthenticated
	}
	return p.finish(ctx, request, verifier, code, origin, false)
}

func (p Repository) finish(ctx context.Context, request, verifier, code, origin string, cli bool) (string, error) {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	var a Assertion
	var raw []byte
	err = tx.QueryRow(ctx, `DELETE FROM platform_identity.developer_login_requests WHERE id=$1 AND verifier_hash=$2 AND portal_origin=$4 AND expires_at>now() AND (($5 AND login_kind='cli' AND cli_confirmed AND code_hash IS NOT NULL) OR (NOT $5 AND login_kind='browser' AND code_hash=$3)) RETURNING core_subject,core_email,display_name,stores`, request, Hash(verifier), Hash(code), origin, cli).Scan(&a.Subject, &a.Email, &a.Name, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", fault.Unauthenticated
	}
	if err != nil {
		return "", err
	}
	// Serialize repeated logins for this Core identity and email. Never link by email alone.
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "developer-core:"+a.Subject); err != nil {
		return "", err
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "developer-email:"+a.Email); err != nil {
		return "", err
	}
	var account string
	err = tx.QueryRow(ctx, `SELECT account_id FROM platform_identity.developer_core_links WHERE core_subject=$1`, a.Subject).Scan(&account)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	if account == "" {
		account = ids.New("portal")
		org := ids.New("org")
		// Stable Core subject owns a fresh profile. Email is contact data, never an ownership lookup.
		if _, err = tx.Exec(ctx, `INSERT INTO platform_identity.portal_accounts(id,email,password_hash,surface,role) VALUES($1,$2,'external:emisell','developer','developer')`, account, account+"@core.emisell.invalid"); err != nil {
			return "", err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO platform_developer.organizations(id,name) VALUES($1,$2)`, org, a.Name); err != nil {
			return "", err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO platform_developer.memberships(account_id,organization_id,role) VALUES($1,$2,'owner')`, account, org); err != nil {
			return "", err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO platform_identity.developer_core_links(account_id,core_subject,core_email,display_name,stores) VALUES($1,$2,$3,$4,$5)`, account, a.Subject, a.Email, a.Name, raw); err != nil {
			return "", err
		}
	}
	var enabled bool
	if err = tx.QueryRow(ctx, `SELECT enabled FROM platform_identity.portal_accounts WHERE id=$1 AND surface='developer' AND role='developer' FOR UPDATE`, account).Scan(&enabled); err != nil {
		return "", err
	}
	if !enabled {
		return "", fault.Forbidden
	}
	if _, err = tx.Exec(ctx, `UPDATE platform_identity.developer_core_links SET core_email=$2,display_name=$3,stores=$4,updated_at=now() WHERE account_id=$1`, account, a.Email, a.Name, raw); err != nil {
		return "", err
	}
	token := Token()
	if _, err = tx.Exec(ctx, `INSERT INTO platform_identity.portal_sessions(token_hash,account_id,surface,expires_at) VALUES($1,$2,'developer',$3)`, Hash(token), account, time.Now().Add(time.Hour)); err != nil {
		return "", err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO platform_identity.portal_audit(id,actor_id,action) VALUES($1,$2,'merchant_login')`, ids.New("aud"), account); err != nil {
		return "", err
	}
	return token, tx.Commit(ctx)
}

func (p Repository) Profile(ctx context.Context, principal identity.PortalPrincipal) (*Profile, error) {
	if principal.Surface != "developer" || principal.Role != "developer" {
		return nil, fault.Forbidden
	}
	var profile Profile
	var raw []byte
	err := p.Pool.QueryRow(ctx, `SELECT core_email,display_name,stores FROM platform_identity.developer_core_links WHERE account_id=$1`, principal.ID).Scan(&profile.Email, &profile.Name, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(raw, &profile.Stores)
	return &profile, err
}
