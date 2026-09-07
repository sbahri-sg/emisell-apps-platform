package connectapi

import (
	"connectrpc.com/connect"
	"context"
	"emisell.app/platform/internal/platform/fault"
	testing "emisell.app/platform/pkg/sdk/gen/emisell/testing/v1"
	"regexp"
)

type testingServer struct{ Server }

var testingActor = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:@-]{0,127}$`)

func (s testingServer) StopAssignment(ctx context.Context, r *connect.Request[testing.StopAssignmentRequest]) (*connect.Response[testing.StopAssignmentResponse], error) {
	c, ok := ctx.Value(callerKey{}).(caller)
	if !ok {
		return nil, mapError(fault.Unauthenticated)
	}
	if !c.PlatformFull {
		return nil, mapError(fault.Forbidden)
	}
	p, err := c.BindTenant(r.Msg.MerchantId)
	if err != nil {
		return nil, mapError(err)
	}
	if !testingActor.MatchString(r.Msg.CoreActorId) {
		return nil, mapError(fault.Invalid)
	}
	if err = s.Accounts.Authorize(ctx, p.ID, p.TenantID); err != nil {
		return nil, mapError(err)
	}
	a, err := s.Testing.StopForMerchant(ctx, p.TenantID, p.ID, r.Msg.CoreActorId, r.Msg.AssignmentId, r.Msg.IdempotencyKey)
	if err != nil {
		return nil, mapError(err)
	}
	return connect.NewResponse(&testing.StopAssignmentResponse{MerchantId: p.TenantID, AssignmentId: a.ID, Status: a.Status}), nil
}

func (s testingServer) ListAssignments(ctx context.Context, r *connect.Request[testing.ListAssignmentsRequest]) (*connect.Response[testing.ListAssignmentsResponse], error) {
	c, ok := ctx.Value(callerKey{}).(caller)
	if !ok {
		return nil, mapError(fault.Unauthenticated)
	}
	if !c.PlatformFull {
		return nil, mapError(fault.Forbidden)
	}
	p, err := c.BindTenant(r.Msg.MerchantId)
	if err != nil {
		return nil, mapError(err)
	}
	if !testingActor.MatchString(r.Msg.CoreActorId) {
		return nil, mapError(fault.Invalid)
	}
	if err = s.Accounts.Authorize(ctx, p.ID, p.TenantID); err != nil {
		return nil, mapError(err)
	}
	apps, next, err := s.Testing.ForMerchant(ctx, p.TenantID, r.Msg.AfterId, int(r.Msg.PageSize))
	if err != nil {
		return nil, mapError(err)
	}
	out := &testing.ListAssignmentsResponse{MerchantId: p.TenantID, NextAfterId: next, Apps: []*testing.TestApp{}}
	for _, a := range apps {
		out.Apps = append(out.Apps, &testing.TestApp{AssignmentId: a.AssignmentID, AppId: a.AppID, AppName: a.AppName, Version: a.Version, Capability: a.Capability, ExecutionProfile: a.ExecutionProfile,
			Readiness: &testing.TestReadiness{ConfigurationReady: a.Readiness.ConfigurationReady, RequiredScopesReady: a.Readiness.RequiredScopesReady, Installable: a.Readiness.Installable, Blockers: a.Readiness.Blockers}})
	}
	return connect.NewResponse(out), nil
}
