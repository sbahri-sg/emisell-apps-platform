package main

import (
	"bufio"
	"context"
	"crypto/rand"
	developerrepo "emisell.app/platform/internal/developer/postgres"
	"emisell.app/platform/internal/identity"
	identityrepo "emisell.app/platform/internal/identity/postgres"
	"emisell.app/platform/internal/platform/localfiles"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"io"
	"os"
	"strings"
)

type portalCredential struct {
	User             identity.PortalPrincipal `json:"user"`
	Password         string                   `json:"password"`
	OrganizationID   string                   `json:"organizationId,omitempty"`
	OrganizationName string                   `json:"organizationName,omitempty"`
}

func updateAdmin(ctx context.Context, pool *pgxpool.Pool, email string, input io.Reader) error {
	const path = ".local/portals.json"
	var accounts []portalCredential
	if err := localfiles.Read(path, &accounts); err != nil {
		return err
	}
	index := -1
	for i := range accounts {
		if accounts[i].User.ID == "portal-local-admin" {
			if index != -1 {
				return errors.New("duplicate primary admin")
			}
			index = i
		}
	}
	if index == -1 {
		return errors.New("primary admin not provisioned")
	}
	// Password comes from bounded stdin, never argv or logging.
	r := bufio.NewReader(io.LimitReader(input, 258))
	password, err := r.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return errors.New("password input failed")
	}
	password = strings.TrimSuffix(strings.TrimSuffix(password, "\n"), "\r")
	account := &accounts[index]
	if err = (identityrepo.Repository{Pool: pool}).UpdatePrimaryAdmin(ctx, account.User, account.Password, email, password); err != nil {
		return err
	}
	account.User.Email = strings.ToLower(strings.TrimSpace(email))
	account.Password = password
	if err = localfiles.Write(path, accounts, true); err != nil {
		return errors.New("database credential updated; private file write failed. Retry update-admin with the same email/password to reconcile; do not run init-portals yet")
	}
	fmt.Println("Primary Admin credential updated; old sessions revoked. Other accounts and installations preserved.")
	return nil
}

func initPortals(ctx context.Context, pool *pgxpool.Pool) error {
	const path = ".local/portals.json"
	accounts := []portalCredential{}
	err := localfiles.Read(path, &accounts)
	if errors.Is(err, os.ErrNotExist) {
		accounts = []portalCredential{
			{User: identity.PortalPrincipal{ID: "portal-local-admin", Email: "admin@emisell.local", Surface: "admin", Role: "administrator"}, Password: rand.Text() + rand.Text()},
			{User: identity.PortalPrincipal{ID: "portal-local-reviewer", Email: "reviewer@emisell.local", Surface: "admin", Role: "reviewer"}, Password: rand.Text() + rand.Text()},
			{User: identity.PortalPrincipal{ID: "portal-local-operator", Email: "operator@emisell.local", Surface: "admin", Role: "operator"}, Password: rand.Text() + rand.Text()},
			{User: identity.PortalPrincipal{ID: "portal-local-developer", Email: "developer@emisell.local", Surface: "developer", Role: "developer"}, Password: rand.Text() + rand.Text(), OrganizationID: "dev-emisell-local", OrganizationName: "Emisell Developer"},
			{User: identity.PortalPrincipal{ID: "portal-local-developer-two", Email: "developer-two@emisell.local", Surface: "developer", Role: "developer"}, Password: rand.Text() + rand.Text(), OrganizationID: "dev-independent-local", OrganizationName: "Independent Developer"},
		}
		if err = localfiles.Write(path, accounts, false); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	for _, account := range accounts {
		if err = (identityrepo.Repository{Pool: pool}).SeedPortal(ctx, account.User, account.Password); err != nil {
			return err
		}
		if account.User.Surface == "developer" {
			if err = (developerrepo.Repository{Pool: pool}).SeedOrganization(ctx, account.User.ID, account.OrganizationID, account.OrganizationName); err != nil {
				return err
			}
		}
	}
	fmt.Println("Separate local portal accounts ready. Credentials: .local/portals.json (private). Existing credentials, merchant accounts, and installations preserved.")
	return nil
}
