package capability

import (
	"context"
	"emisell.app/platform/internal/installation/domain"
	installation "emisell.app/platform/internal/installation/service"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/pkg/appmanifest"
)

type Gate interface {
	WithActive(context.Context, string, string, string, string, func(domain.Installation) error) error
}
type Runtime interface {
	Execute(string, Request, *Resource) (*Resource, []Rate, error)
}
type RemoteRuntime interface {
	Execute(context.Context, string, string, string, string, string, Request) (*Resource, []Rate, error)
}
type LocalShippingRuntime interface {
	RemoteRuntime
	installation.ShippingReadiness
}
type Repository interface {
	Execute(context.Context, string, string, string, string, string, string, string, Request, func(*Resource) (*Resource, []Rate, error)) (Response, error)
}
type Service struct {
	Installations Gate
	Repo          Repository
	Runtime       Runtime
	Remote        RemoteRuntime
	Shipping      LocalShippingRuntime
}

func (s Service) Invoke(ctx context.Context, user, tenant, cap, key, correlation string, req Request) (Response, error) {
	var result Response
	if !installation.KeyPattern.MatchString(key) {
		return result, fault.Invalid
	}
	scope := ""
	switch cap {
	case "payment/v1":
		switch req.Operation {
		case "create", "capture", "refund":
			scope = "payments.write"
		case "status":
			scope = "payments.read"
		}
	case "shipping/v1":
		switch req.Operation {
		case "create":
			scope = "shipping.write"
		case "get_rates", "track":
			scope = "shipping.read"
		}
	}
	if scope == "" {
		return result, fault.Invalid
	}
	err := s.Installations.WithActive(ctx, user, tenant, cap, scope, func(ins domain.Installation) error {
		if ins.ExecutionProfile == appmanifest.KurirFixtureProfile {
			if cap != "shipping/v1" || req.Operation != "get_rates" || ins.IntentID == "" {
				return fault.Forbidden
			}
			if s.Shipping == nil {
				return fault.Unavailable
			}
			// Recheck configuration even for idempotent replays. A cached quote is
			// not authorization to use a disabled/unconfigured shipping account.
			if err := s.Shipping.Ready(ctx, tenant, ins.ID); err != nil {
				return err
			}
		}
		hash := installation.RequestHash(struct {
			InstallationID, Capability string
			Request                    Request
		}{ins.ID, cap, req})
		var err error
		result, err = s.Repo.Execute(ctx, user, tenant, cap, ins.ID, key, hash, correlation, req, func(current *Resource) (*Resource, []Rate, error) {
			if ins.ExecutionProfile == appmanifest.KurirFixtureProfile {
				return s.Shipping.Execute(ctx, tenant, user, ins.ID, cap, key, req)
			}
			if ins.ExecutionProfile == "local-remote" {
				if s.Remote == nil {
					return nil, nil, fault.Unavailable
				}
				return s.Remote.Execute(ctx, tenant, user, ins.ID, cap, key, req)
			}
			if ins.ExecutionProfile != "local-simulator" || s.Runtime == nil {
				return nil, nil, fault.Unavailable
			}
			return s.Runtime.Execute(cap, req, current)
		})
		return err
	})
	return result, err
}
