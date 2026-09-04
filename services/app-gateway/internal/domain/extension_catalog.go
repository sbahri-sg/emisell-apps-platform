package domain

// Categories describe discovery, families describe configuration, and
// capabilities describe implemented contracts. None of them grant access.
type ExtensionCatalog struct {
	Version         string                             `json:"version"`
	Categories      []AppCategoryDefinition            `json:"categories"`
	Families        []ExtensionFamilyDefinition        `json:"families"`
	Capabilities    []AppCapabilityDefinition          `json:"capabilities"`
	Surfaces        []AppSurfaceDefinition             `json:"surfaces"`
	RuntimeContract ExtensionRuntimeContractDefinition `json:"runtimeContract"`
}

type AppCategoryDefinition struct {
	ID          CatalogCategory `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Examples    []string        `json:"examples"`
}

type ExtensionFamilyDefinition struct {
	Type                   ExtensionType `json:"type"`
	Name                   string        `json:"name"`
	Description            string        `json:"description"`
	ConfigurationSupported bool          `json:"configurationSupported"`
}

type AppCapabilityDefinition struct {
	ID               string          `json:"id"`
	Name             string          `json:"name"`
	Type             ExtensionType   `json:"type"`
	Availability     string          `json:"availability"`
	ExecutionEnabled bool            `json:"executionEnabled"`
	Invocation       string          `json:"invocation"`
	RequiredScopes   []string        `json:"requiredScopes"`
	Endpoints        []ScopeEndpoint `json:"endpoints"`
	Description      string          `json:"description"`
	Limitations      []string        `json:"limitations"`
	GuideChapter     string          `json:"guideChapter"`
}

type AppSurfaceDefinition struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Availability string `json:"availability"`
	Description  string `json:"description"`
}

// ExtensionRuntimeContractDefinition is reviewed protocol metadata for the
// future platform-to-provider dispatcher. It is deliberately part of the
// existing catalog instead of a second discovery endpoint. Draft metadata
// never activates dispatch, grants a scope, or trusts a caller-supplied URL.
type ExtensionRuntimeContractDefinition struct {
	Version          string                       `json:"version"`
	Status           string                       `json:"status"`
	ExecutionEnabled bool                         `json:"executionEnabled"`
	Transport        string                       `json:"transport"`
	Authentication   string                       `json:"authentication"`
	TokenTTLSeconds  int                          `json:"tokenTtlSeconds"`
	IdentityClaims   []string                     `json:"identityClaims"`
	Headers          []RuntimeHeaderDefinition    `json:"headers"`
	Operations       []RuntimeOperationDefinition `json:"operations"`
	Invariants       []string                     `json:"invariants"`
}

type RuntimeHeaderDefinition struct {
	Name        string `json:"name"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
}

type RuntimeOperationDefinition struct {
	ID              string `json:"id"`
	CapabilityID    string `json:"capabilityId"`
	Mode            string `json:"mode"`
	Mutation        bool   `json:"mutation"`
	Idempotency     string `json:"idempotency"`
	TimeoutMS       int    `json:"timeoutMs"`
	MaximumAttempts int    `json:"maximumAttempts"`
	Description     string `json:"description"`
}
