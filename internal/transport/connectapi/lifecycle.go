package connectapi

import (
	"context"

	"connectrpc.com/connect"
	"emisell.app/platform/internal/installation/domain"
	"emisell.app/platform/internal/oauth/apptoken"
	"emisell.app/platform/internal/platform/fault"
	intent "emisell.app/platform/pkg/sdk/gen/emisell/installation/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type lifecycleServer struct{ Server }

func (s lifecycleServer) ListInstallations(ctx context.Context, r *connect.Request[intent.ListInstallationsRequest]) (*connect.Response[intent.ListInstallationsResponse], error) {
	c, ok := ctx.Value(callerKey{}).(caller)
	if !ok {
		return nil, mapError(fault.Unauthenticated)
	}
	p, err := c.BindTenant(r.Msg.MerchantId)
	if err != nil {
		return nil, mapError(err)
	}
	items, next, err := s.Lifecycle.List(ctx, p, r.Msg.CoreActorId, r.Msg.AfterId, int(r.Msg.PageSize))
	if err != nil {
		return nil, mapError(err)
	}
	result := &intent.ListInstallationsResponse{MerchantId: p.TenantID, NextAfterId: next, Installations: []*intent.InstalledApp{}}
	for _, v := range items {
		result.Installations = append(result.Installations, &intent.InstalledApp{InstallationId: v.ID, AppId: v.AppID, AppName: v.Name, DeveloperId: v.DeveloperID, Version: v.Version, Status: v.Status, GrantState: v.GrantState, ExecutionProfile: v.ExecutionProfile, InstalledAt: timestamppb.New(v.InstalledAt)})
	}
	return connect.NewResponse(result), nil
}

func accessDTO(v domain.AccessResult) *intent.LifecycleResponse {
	a := v.Access
	r := &intent.LifecycleResponse{Installation: &intent.InstallationAccess{
		InstallationId: a.Installation.ID, TenantId: a.Owner.TenantID, AppId: a.Release.AppID, Version: a.Release.Version,
		Status: a.Installation.Status, GrantState: a.GrantState, GrantedScopes: a.GrantedScopes, RequestedScopes: a.Release.Scopes,
		Capabilities: a.Installation.Capabilities, IntentId: a.IntentID, ManifestDigest: a.Release.ManifestDigest,
		ExecutionProfile: a.Release.ExecutionProfile, ConsumedAt: timestamppb.New(a.ConsumedAt),
		LocalEmbeddedAccess: a.LocalEmbeddedAccess,
	}, Replayed: v.Replayed, AppToken: v.Token.Secret, TokenId: v.Token.ID, TokenInvalid: v.Token.Revoked}
	if a.ReviewedUILaunch != nil {
		l := a.ReviewedUILaunch.Launch
		r.Installation.ReviewedUiLaunch = &intent.ReviewedUILaunch{AppId: l.AppID, ClientId: l.ClientID, ReleaseDigest: l.ReleaseDigest, Url: l.URL, ParentOrigin: l.ParentOrigin, Mode: l.DisplayMode()}
	}
	if v.Token.ID != "" {
		r.TokenExpiresAt = timestamppb.New(v.Token.ExpiresAt)
		r.TokenAudience = apptoken.Audience
	}
	return r
}

func (s lifecycleServer) Consume(ctx context.Context, r *connect.Request[intent.ConsumeRequest]) (*connect.Response[intent.ConsumeResponse], error) {
	c, ok := ctx.Value(callerKey{}).(caller)
	if !ok {
		return nil, mapError(fault.Unauthenticated)
	}
	p, err := c.BindTenant(r.Msg.TenantId)
	if err != nil {
		return nil, mapError(err)
	}
	v, err := s.Lifecycle.Consume(ctx, p, r.Msg.CoreActorId, r.Msg.IdempotencyKey, r.Msg.IntentId, r.Msg.ConsentDigest)
	if err != nil {
		return nil, mapError(err)
	}
	return connect.NewResponse(&intent.ConsumeResponse{Result: accessDTO(v)}), nil
}

func (s lifecycleServer) execute(ctx context.Context, target *intent.InstallationRequest, action string) (*intent.LifecycleResponse, error) {
	if target == nil {
		return nil, mapError(fault.Invalid)
	}
	r := connect.NewRequest(target)
	c, ok := ctx.Value(callerKey{}).(caller)
	if !ok {
		return nil, mapError(fault.Unauthenticated)
	}
	p, err := c.BindTenant(r.Msg.TenantId)
	if err != nil {
		return nil, mapError(err)
	}
	var v domain.AccessResult
	if action == "get" {
		v.Access, err = s.Lifecycle.Get(ctx, p, r.Msg.CoreActorId, r.Msg.InstallationId)
	} else {
		v, err = s.Lifecycle.Execute(ctx, p, r.Msg.CoreActorId, r.Msg.IdempotencyKey, r.Msg.InstallationId, action)
	}
	if err != nil {
		return nil, mapError(err)
	}
	return accessDTO(v), nil
}
func (s lifecycleServer) GetInstallation(ctx context.Context, r *connect.Request[intent.GetInstallationRequest]) (*connect.Response[intent.GetInstallationResponse], error) {
	v, err := s.execute(ctx, r.Msg.Target, "get")
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&intent.GetInstallationResponse{Result: v}), nil
}
func (s lifecycleServer) Activate(ctx context.Context, r *connect.Request[intent.ActivateRequest]) (*connect.Response[intent.ActivateResponse], error) {
	v, err := s.execute(ctx, r.Msg.Target, "activate")
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&intent.ActivateResponse{Result: v}), nil
}
func (s lifecycleServer) IssueToken(ctx context.Context, r *connect.Request[intent.IssueTokenRequest]) (*connect.Response[intent.IssueTokenResponse], error) {
	v, err := s.execute(ctx, r.Msg.Target, "issue_token")
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&intent.IssueTokenResponse{Result: v}), nil
}
func (s lifecycleServer) Uninstall(ctx context.Context, r *connect.Request[intent.UninstallRequest]) (*connect.Response[intent.UninstallResponse], error) {
	v, err := s.execute(ctx, r.Msg.Target, "uninstall")
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&intent.UninstallResponse{Result: v}), nil
}
