package connectapi

import (
	"context"

	"connectrpc.com/connect"
	"emisell.app/platform/internal/installation/domain"
	"emisell.app/platform/internal/installation/service"
	"emisell.app/platform/internal/platform/fault"
	intent "emisell.app/platform/pkg/sdk/gen/emisell/installation/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type intentServer struct{ Server }

func intentDTO(v domain.InstallIntent) *intent.InstallIntent {
	states := map[string]intent.IntentState{
		"pending":   intent.IntentState_INTENT_STATE_PENDING,
		"consented": intent.IntentState_INTENT_STATE_CONSENTED,
		"denied":    intent.IntentState_INTENT_STATE_DENIED,
		"expired":   intent.IntentState_INTENT_STATE_EXPIRED,
	}
	r := &intent.InstallIntent{
		InstallationPolicy: v.Release.InstallPolicy,
		Id:                 v.ID, TenantId: v.Owner.TenantID, CoreActorId: v.Owner.ActorID,
		AppId: v.Release.AppID, AppName: v.Release.Name, DeveloperId: v.Release.DeveloperID,
		Version: v.Release.Version, ManifestDigest: v.Release.ManifestDigest,
		Scopes: v.Release.Scopes, Capabilities: v.Release.Capabilities, ExecutionProfile: v.Release.ExecutionProfile,
		ConsentDigest: v.ConsentDigest, State: states[v.State],
		CreatedAt: timestamppb.New(v.CreatedAt), ExpiresAt: timestamppb.New(v.ExpiresAt), ExecutionAllowed: false,
	}
	if v.DecidedAt != nil {
		r.DecidedAt = timestamppb.New(*v.DecidedAt)
	}
	return r
}

func (s intentServer) Prepare(ctx context.Context, r *connect.Request[intent.PrepareRequest]) (*connect.Response[intent.PrepareResponse], error) {
	c, ok := ctx.Value(callerKey{}).(caller)
	if !ok {
		return nil, mapError(fault.Unauthenticated)
	}
	p, err := c.BindTenant(r.Msg.TenantId)
	if err != nil {
		return nil, mapError(err)
	}
	v, err := s.Intents.Prepare(ctx, p, r.Msg.CoreActorId, r.Msg.IdempotencyKey, service.PrepareIntent{AppID: r.Msg.AppId, Version: r.Msg.Version})
	if err != nil {
		return nil, mapError(err)
	}
	return connect.NewResponse(&intent.PrepareResponse{Intent: intentDTO(v)}), nil
}

func (s intentServer) Get(ctx context.Context, r *connect.Request[intent.GetRequest]) (*connect.Response[intent.GetResponse], error) {
	c, ok := ctx.Value(callerKey{}).(caller)
	if !ok {
		return nil, mapError(fault.Unauthenticated)
	}
	p, err := c.BindTenant(r.Msg.TenantId)
	if err != nil {
		return nil, mapError(err)
	}
	v, err := s.Intents.Get(ctx, p, r.Msg.CoreActorId, r.Msg.IntentId)
	if err != nil {
		return nil, mapError(err)
	}
	return connect.NewResponse(&intent.GetResponse{Intent: intentDTO(v)}), nil
}

func (s intentServer) Decide(ctx context.Context, r *connect.Request[intent.DecideRequest]) (*connect.Response[intent.DecideResponse], error) {
	c, ok := ctx.Value(callerKey{}).(caller)
	if !ok {
		return nil, mapError(fault.Unauthenticated)
	}
	decision := ""
	switch r.Msg.Decision {
	case intent.ConsentDecision_CONSENT_DECISION_CONSENT:
		decision = "consent"
	case intent.ConsentDecision_CONSENT_DECISION_DENY:
		decision = "deny"
	}
	p, err := c.BindTenant(r.Msg.TenantId)
	if err != nil {
		return nil, mapError(err)
	}
	v, err := s.Intents.Decide(ctx, p, r.Msg.CoreActorId, r.Msg.IdempotencyKey, service.DecideIntent{ID: r.Msg.IntentId, Digest: r.Msg.ConsentDigest, Decision: decision})
	if err != nil {
		return nil, mapError(err)
	}
	return connect.NewResponse(&intent.DecideResponse{Intent: intentDTO(v)}), nil
}
