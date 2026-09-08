package identity

import (
	"context"
	"errors"
	"strings"
	"testing"

	"emisell.app/platform/internal/platform/fault"
)

type merchantReferenceRepo struct {
	ServiceAccountRepository
	calls int
	err   error
}

func (r *merchantReferenceRepo) EnsureMerchantReference(_ context.Context, service, merchant, actor string) (bool, error) {
	r.calls++
	return r.err == nil, r.err
}

func TestMerchantReferenceRequiresExplicitFullCorePrincipalAndValidIdentity(t *testing.T) {
	repo := &merchantReferenceRepo{}
	s := ServiceAccounts{Repo: repo}
	p := ServicePrincipal{ID: "platformkey_test", PlatformFull: true}
	if created, err := s.EnsureMerchant(context.Background(), p, "merchant-a", "owner-a"); err != nil || !created {
		t.Fatal("valid Core rejected", err)
	}
	for _, principal := range []ServicePrincipal{{}, {ID: "legacy", PlatformFull: true}, {ID: "platformkey_test"}} {
		if _, err := s.EnsureMerchant(context.Background(), principal, "merchant-a", "owner-a"); !errors.Is(err, fault.Forbidden) {
			t.Fatal("non-Core allowed", err)
		}
	}
	for _, value := range []string{"", "../other", "a\nb", strings.Repeat("a", 129)} {
		if _, err := s.EnsureMerchant(context.Background(), p, value, "owner-a"); !errors.Is(err, fault.Invalid) {
			t.Fatal("invalid merchant allowed", err)
		}
		if _, err := s.EnsureMerchant(context.Background(), p, "merchant-a", value); !errors.Is(err, fault.Invalid) {
			t.Fatal("invalid actor allowed", err)
		}
	}
	if repo.calls != 1 {
		t.Fatal("invalid request reached storage")
	}
	repo.err = fault.Unauthenticated
	if _, err := s.EnsureMerchant(context.Background(), p, "merchant-a", "owner-a"); !errors.Is(err, fault.Unauthenticated) {
		t.Fatal("storage auth rejection swallowed")
	}
	if _, err := (ServiceAccounts{}).EnsureMerchant(context.Background(), p, "merchant-a", "owner-a"); !errors.Is(err, fault.Unavailable) {
		t.Fatal("missing repository extension succeeded")
	}
}
