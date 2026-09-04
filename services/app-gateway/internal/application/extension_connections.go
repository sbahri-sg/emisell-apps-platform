package application

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ids"
	"emisell-app-platform/services/app-gateway/internal/ports"
)

type ExtensionConnectionService struct {
	repository ports.ExtensionConnectionRepository
	cipher     SecretCipher
	keyVersion int
	id         ids.Generator
	now        func() time.Time
	pilot      DevelopmentResourcePilot
}

var extensionConnectionRequestID = regexp.MustCompile(`^[A-Za-z0-9._:-]{16,128}$`)

func NewExtensionConnectionService(repository ports.ExtensionConnectionRepository, cipher SecretCipher, keyVersion int, id ids.Generator, now func() time.Time, pilot DevelopmentResourcePilot) *ExtensionConnectionService {
	return &ExtensionConnectionService{repository, cipher, keyVersion, id, now, pilot}
}

type ExtensionConnectionResult struct {
	Mode         string                      `json:"mode"`
	Connection   *domain.ExtensionConnection `json:"connection"`
	RuntimeToken string                      `json:"runtimeToken,omitempty"`
}

type ProvisionExtensionConnection struct {
	Selector             ports.ExtensionConnectionSelector
	ActorID, RuntimeName string
	Revision             int64
	Scopes               []string
	Secret               map[string]string
	RuntimeExpiresAt     time.Time
}

type ResolvedExtensionCredential struct {
	ConnectionID       string            `json:"connectionId"`
	AppID              string            `json:"appId"`
	InstallationID     string            `json:"installationId"`
	ExtensionID        string            `json:"extensionId"`
	MerchantID         string            `json:"merchantId"`
	InstalledVersionID string            `json:"installedVersionId"`
	Scope              string            `json:"scope"`
	Secret             map[string]string `json:"secret"`
}

// The encrypted envelope authenticates the credential's purpose and binding even
// if ciphertext is copied between otherwise valid database records.
type extensionSecretEnvelope struct {
	Purpose, ConnectionID, AppID, InstallationID, ExtensionID, RuntimeName string
	Secret                                                                 map[string]string
}

func validateConnectionSelector(sel ports.ExtensionConnectionSelector) error {
	for _, value := range []string{sel.OrganizationID, sel.AppID, sel.InstallationID, sel.ExtensionID} {
		if !uuidValue.MatchString(value) {
			return fmt.Errorf("%w: invalid connection identifier", domain.ErrValidation)
		}
	}
	if sel.TokenHash != "" {
		return domain.ErrValidation
	}
	return nil
}

func connectionMetadata(value *domain.ExtensionConnection) *domain.ExtensionConnection {
	if value == nil {
		return nil
	}
	copy := *value
	copy.Scopes = append([]string{}, value.Scopes...)
	copy.TokenHash, copy.Ciphertext, copy.KeyVersion = "", nil, 0
	return &copy
}

func (s *ExtensionConnectionService) Get(ctx context.Context, sel ports.ExtensionConnectionSelector) (ExtensionConnectionResult, error) {
	result := ExtensionConnectionResult{Mode: "external"}
	if err := validateConnectionSelector(sel); err != nil {
		return result, err
	}
	err := s.repository.WithExtensionConnection(ctx, sel, func(state *ports.ExtensionConnectionState) error {
		result.Connection = connectionMetadata(state.Connection)
		if state.Connection != nil {
			result.Mode = "emisell_managed"
		}
		return nil
	})
	return result, err
}

func activeExtensionConnection(state *ports.ExtensionConnectionState) error {
	if !state.OrganizationActive || !state.Entitled || state.App.Status != domain.AppStatusActive || state.Installation.Status != domain.InstallationStatusActive || state.Extension.Status == domain.ExtensionStatusDisabled {
		return domain.ErrForbidden
	}
	// The installed immutable snapshot, not the app's newest release, is authoritative.
	for _, extension := range state.Version.Snapshot.Extensions {
		if extension.ExtensionID == state.Extension.ID {
			return nil
		}
	}
	return domain.ErrForbidden
}

func (s *ExtensionConnectionService) scopeAllowed(state *ports.ExtensionConnectionState, scope string) bool {
	if !containsExact(state.Installation.GrantedScopes, scope) {
		return false
	}
	declared := false
	for _, item := range state.Version.Snapshot.Scopes {
		if item.Scope == scope {
			declared = true
		}
	}
	return declared && s.pilot.unavailable([]domain.SnapshotScope{{Scope: scope}}, domain.MerchantIdentity{MerchantID: state.Installation.MerchantID, Environment: state.Installation.Environment}) == ""
}

func (s *ExtensionConnectionService) validateExpiry(expiry time.Time) error {
	now := s.now().UTC()
	if !expiry.After(now) || expiry.After(now.Add(30*24*time.Hour)) {
		return fmt.Errorf("%w: runtimeExpiresAt must be in the next 30 days", domain.ErrValidation)
	}
	return nil
}

func (s *ExtensionConnectionService) Provision(ctx context.Context, cmd ProvisionExtensionConnection) (ExtensionConnectionResult, error) {
	cmd.RuntimeName = strings.TrimSpace(cmd.RuntimeName)
	if err := validateConnectionSelector(cmd.Selector); err != nil {
		return ExtensionConnectionResult{}, err
	}
	if !uuidValue.MatchString(cmd.ActorID) || cmd.Revision < 0 || len(cmd.RuntimeName) < 3 || len(cmd.RuntimeName) > 80 || !externalIdentityValue.MatchString(cmd.RuntimeName) || len(cmd.Scopes) < 1 || len(cmd.Scopes) > 32 || len(cmd.Secret) < 1 || len(cmd.Secret) > 32 {
		return ExtensionConnectionResult{}, domain.ErrValidation
	}
	if err := s.validateExpiry(cmd.RuntimeExpiresAt); err != nil {
		return ExtensionConnectionResult{}, err
	}
	secretBytes, _ := json.Marshal(cmd.Secret)
	if len(secretBytes) > 8192 {
		return ExtensionConnectionResult{}, fmt.Errorf("%w: credential exceeds 8192 bytes", domain.ErrValidation)
	}
	for key, value := range cmd.Secret {
		if !externalIdentityValue.MatchString(key) || value == "" {
			return ExtensionConnectionResult{}, domain.ErrValidation
		}
	}
	scopes := append([]string{}, cmd.Scopes...)
	sort.Strings(scopes)
	for index, scope := range scopes {
		if scope == "" || (index > 0 && scope == scopes[index-1]) {
			return ExtensionConnectionResult{}, domain.ErrValidation
		}
	}
	id, err := s.id()
	if err != nil {
		return ExtensionConnectionResult{}, err
	}
	token, err := randomToken("er_", 32)
	if err != nil {
		return ExtensionConnectionResult{}, err
	}
	result := ExtensionConnectionResult{Mode: "emisell_managed"}
	err = s.repository.WithExtensionConnection(ctx, cmd.Selector, func(state *ports.ExtensionConnectionState) error {
		if err := activeExtensionConnection(state); err != nil {
			return err
		}
		for _, scope := range scopes {
			if !s.scopeAllowed(state, scope) {
				return domain.ErrForbidden
			}
		}
		now := s.now().UTC()
		value := domain.ExtensionConnection{ID: id, AppID: state.App.ID, InstallationID: state.Installation.ID, ExtensionID: state.Extension.ID, CreatedAt: now}
		if state.Connection != nil {
			value = *state.Connection
		}
		if value.Revision != cmd.Revision {
			return fmt.Errorf("%w: connection revision changed", domain.ErrConflict)
		}
		value.RuntimeName, value.Scopes, value.Status = strings.TrimSpace(cmd.RuntimeName), scopes, "active"
		value.Revision++
		value.UpdatedAt = now
		value.RuntimeExpiresAt = cmd.RuntimeExpiresAt.UTC()
		value.TokenHash, value.KeyVersion = tokenDigest(token), s.keyVersion
		encoded, _ := json.Marshal(extensionSecretEnvelope{"emisell-extension-credential-v1", value.ID, value.AppID, value.InstallationID, value.ExtensionID, value.RuntimeName, cmd.Secret})
		value.Ciphertext, err = s.cipher.Encrypt(encoded)
		if err != nil {
			return fmt.Errorf("encrypt extension credential: %w", err)
		}
		state.Write = &value
		state.Audit = ports.MutationMeta{ActorID: cmd.ActorID, Action: "extension_connection.provisioned"}
		result.Connection = connectionMetadata(&value)
		return nil
	})
	if err != nil {
		return ExtensionConnectionResult{}, err
	}
	result.RuntimeToken = token // Returned only after commit; only its digest is persisted.
	return result, nil
}

func (s *ExtensionConnectionService) Rotate(ctx context.Context, sel ports.ExtensionConnectionSelector, actor string, revision int64, expires time.Time) (ExtensionConnectionResult, error) {
	if err := validateConnectionSelector(sel); err != nil {
		return ExtensionConnectionResult{}, err
	}
	if !uuidValue.MatchString(actor) || revision < 1 {
		return ExtensionConnectionResult{}, domain.ErrValidation
	}
	if err := s.validateExpiry(expires); err != nil {
		return ExtensionConnectionResult{}, err
	}
	token, err := randomToken("er_", 32)
	if err != nil {
		return ExtensionConnectionResult{}, err
	}
	result := ExtensionConnectionResult{Mode: "emisell_managed"}
	err = s.repository.WithExtensionConnection(ctx, sel, func(state *ports.ExtensionConnectionState) error {
		if state.Connection == nil {
			return domain.ErrNotFound
		}
		if err := activeExtensionConnection(state); err != nil {
			return err
		}
		value := *state.Connection
		if value.Status != "active" || value.Revision != revision {
			return domain.ErrConflict
		}
		value.TokenHash = tokenDigest(token)
		value.RuntimeExpiresAt = expires.UTC()
		value.Revision++
		value.UpdatedAt = s.now().UTC()
		state.Write = &value
		state.Audit = ports.MutationMeta{ActorID: actor, Action: "extension_connection.runtime_rotated"}
		result.Connection = connectionMetadata(&value)
		return nil
	})
	if err != nil {
		return ExtensionConnectionResult{}, err
	}
	result.RuntimeToken = token
	return result, nil
}

func (s *ExtensionConnectionService) Revoke(ctx context.Context, sel ports.ExtensionConnectionSelector, actor string, revision int64) error {
	if err := validateConnectionSelector(sel); err != nil {
		return err
	}
	if !uuidValue.MatchString(actor) || revision < 1 {
		return domain.ErrValidation
	}
	return s.repository.WithExtensionConnection(ctx, sel, func(state *ports.ExtensionConnectionState) error {
		if state.Connection == nil {
			return domain.ErrNotFound
		}
		value := *state.Connection
		if value.Revision != revision {
			return domain.ErrConflict
		}
		value.Status, value.TokenHash, value.Ciphertext = "revoked", "", nil
		value.Revision++
		value.UpdatedAt = s.now().UTC()
		state.Write = &value
		state.Audit = ports.MutationMeta{ActorID: actor, Action: "extension_connection.revoked"}
		return nil
	})
}

func (s *ExtensionConnectionService) Resolve(ctx context.Context, token, scope, requestID string) (ResolvedExtensionCredential, error) {
	if !strings.HasPrefix(token, "er_") || len(token) != 46 {
		return ResolvedExtensionCredential{}, domain.ErrUnauthorized
	}
	if scope == "" || !extensionConnectionRequestID.MatchString(requestID) {
		return ResolvedExtensionCredential{}, domain.ErrValidation
	}
	hash := tokenDigest(token)
	var result ResolvedExtensionCredential
	err := s.repository.WithExtensionConnection(ctx, ports.ExtensionConnectionSelector{TokenHash: hash}, func(state *ports.ExtensionConnectionState) error {
		value := state.Connection
		if value == nil || value.Status != "active" || !value.RuntimeExpiresAt.After(s.now()) || subtle.ConstantTimeCompare([]byte(value.TokenHash), []byte(hash)) != 1 {
			return domain.ErrUnauthorized
		}
		if err := activeExtensionConnection(state); err != nil {
			return err
		}
		if !containsExact(value.Scopes, scope) || !s.scopeAllowed(state, scope) {
			return domain.ErrForbidden
		}
		if value.KeyVersion != s.keyVersion {
			return fmt.Errorf("unsupported extension credential key version")
		}
		plain, err := s.cipher.Decrypt(value.Ciphertext)
		if err != nil {
			return fmt.Errorf("cannot decrypt extension credential")
		}
		var sealed extensionSecretEnvelope
		if json.Unmarshal(plain, &sealed) != nil || sealed.Purpose != "emisell-extension-credential-v1" || sealed.ConnectionID != value.ID || sealed.AppID != state.App.ID || sealed.InstallationID != state.Installation.ID || sealed.ExtensionID != state.Extension.ID || sealed.RuntimeName != value.RuntimeName || len(sealed.Secret) == 0 {
			return fmt.Errorf("extension credential binding mismatch")
		}
		state.AccessScope, state.RequestID = scope, requestID
		result = ResolvedExtensionCredential{value.ID, state.App.ID, state.Installation.ID, state.Extension.ID, state.Installation.MerchantID, state.Version.ID, scope, sealed.Secret}
		return nil
	})
	if err != nil {
		return ResolvedExtensionCredential{}, err
	}
	return result, nil
}
