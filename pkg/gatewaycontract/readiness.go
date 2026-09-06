package gatewaycontract

import "time"

// Readiness is an inventory of this Platform build, never a live Core probe or
// an authorization registry. No resource adapter/grant pipeline is enabled yet.
type Readiness struct {
	Profile          string               `json:"profile"`
	CheckedAt        time.Time            `json:"checkedAt"`
	Verification     string               `json:"verification"`
	CoreChecked      bool                 `json:"coreChecked"`
	Scopes           []ScopeReadiness     `json:"scopes"`
	ContractRevision string               `json:"contractRevision"`
	Environment      string               `json:"environment"`
	Operations       []OperationReadiness `json:"operations"`
}

// Operation readiness and whole-scope readiness are distinct. An implemented
// read procedure alone does not imply complete coverage of read/write scope.
type OperationReadiness struct {
	Procedure      string   `json:"procedure"`
	Version        string   `json:"version"`
	RequiredScope  string   `json:"requiredScope"`
	AcceptedScopes []string `json:"acceptedScopes"`
	Status         string   `json:"status"`
	Blockers       []string `json:"blockers"`
}
type ScopeReadiness struct {
	Handle         string   `json:"handle"`
	Status         string   `json:"status"`
	Grantable      bool     `json:"grantable"`
	ContractStatus string   `json:"contractStatus"`
	Operations     []string `json:"operations"`
	Blockers       []string `json:"blockers"`
}

func VerifyReadiness(now time.Time) Readiness {
	h := Reference()
	v := Readiness{Profile: h.ScopeProfile, CheckedAt: now.UTC(), Verification: "platform_build_inventory", Scopes: []ScopeReadiness{}, ContractRevision: h.ContractRevision, Environment: "local", Operations: []OperationReadiness{}}
	for _, o := range h.Operations {
		v.Operations = append(v.Operations, OperationReadiness{Procedure: o.Procedure, Version: o.Version, RequiredScope: o.RequiredScope, AcceptedScopes: o.AcceptedScopes, Status: "planned", Blockers: []string{"Implementasi dan bukti uji gateway Core belum terverifikasi.", "Adapter Platform dan delegasi resource belum tersedia."}})
	}
	for _, c := range h.Coverage {
		contract := "Kontrak operasi belum tersedia."
		if c.ContractStatus == "partial" {
			contract = "Kontrak baru parsial; List/Get produk dasar saja."
		} else if c.ContractStatus == "reference_only" {
			contract = "Referensi perlu evaluasi relevansi sebelum dikontrakkan."
		}
		v.Scopes = append(v.Scopes, ScopeReadiness{
			Handle: c.Scope, Status: "planned", ContractStatus: c.ContractStatus, Operations: c.Operations,
			Blockers: []string{contract, "Adapter resource Platform belum diaktifkan.", "Consent/consume/grant/token resource belum tersedia.", "Implementasi dan bukti uji gateway Core belum terverifikasi."},
		})
	}
	return v
}
