package bootstrap_test

import (
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"connectrpc.com/connect"
	"emisell.app/platform/internal/app/service"
	"emisell.app/platform/internal/bootstrap"
	"emisell.app/platform/pkg/sdk"
	testingv1 "emisell.app/platform/pkg/sdk/gen/emisell/testing/v1"
)

func TestMerchantStopsTesting(t *testing.T) {
	f, dev, _, admin, signer := testingFixture(t)
	ctx := context.Background()
	id := approvedTestAssignment(t, dev, admin, f.tenant)
	otherID := approvedTestAssignment(t, dev, admin, f.other)
	_, _, secret, _ := lifecycleClient(t, f)
	rpc := httptest.NewServer(bootstrap.InternalHandlerWithReleases(f.pool, f.caps, slog.New(slog.NewTextHandler(io.Discard, nil)), signer))
	t.Cleanup(rpc.Close)
	client, err := sdk.NewLocalClient(rpc.URL, secret)
	if err != nil {
		t.Fatal(err)
	}
	stop := func(merchant, actor, assignment, requestKey string) error {
		_, e := client.TestDistribution.StopAssignment(ctx, connect.NewRequest(&testingv1.StopAssignmentRequest{MerchantId: merchant, CoreActorId: actor, AssignmentId: assignment, IdempotencyKey: requestKey}))
		return e
	}
	rpcCode(t, stop(f.tenant, "staff", otherID, key()), connect.CodeNotFound)
	rpcCode(t, stop(f.tenant, "", id, key()), connect.CodeInvalidArgument)
	rpcCode(t, stop(f.tenant, "staff", id, ""), connect.CodeInvalidArgument)
	legacy, _, _ := serviceClient(t, f, f.tenant, intentScopes())
	_, err = legacy.TestDistribution.StopAssignment(ctx, connect.NewRequest(&testingv1.StopAssignmentRequest{MerchantId: f.tenant, CoreActorId: "staff", AssignmentId: id, IdempotencyKey: key()}))
	rpcCode(t, err, connect.CodePermissionDenied)
	retry := key()
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- stop(f.tenant, "staff", id, retry) }()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	if err = stop(f.tenant, "staff", id, key()); err != nil {
		t.Fatal("already stopped", err)
	}
	list, err := client.TestDistribution.ListAssignments(ctx, connect.NewRequest(&testingv1.ListAssignmentsRequest{MerchantId: f.tenant, CoreActorId: "staff"}))
	if err != nil || len(list.Msg.Apps) != 0 {
		t.Fatal("stopped assignment remains", err)
	}
	otherList, err := client.TestDistribution.ListAssignments(ctx, connect.NewRequest(&testingv1.ListAssignmentsRequest{MerchantId: f.other, CoreActorId: "staff"}))
	if err != nil || len(otherList.Msg.Apps) != 1 || otherList.Msg.Apps[0].AssignmentId != otherID {
		t.Fatal("other merchant changed", err)
	}
	detail := pexpect(t, admin, "GET", "/api/v1/admin/test-assignments/"+id, nil, "", 200)
	history := detail["history"].([]any)
	if len(history) != 3 {
		t.Fatal("retries duplicate audit", len(history))
	}
	last := history[2].(map[string]any)
	if last["action"] != "revoked" || !strings.HasPrefix(last["actorId"].(string), "core:") || last["reason"] != "Seller ended testing for this store." {
		t.Fatal("missing seller audit")
	}
	// No reactivation/re-approval by replay. Requested assignments are not stoppable.
	release := signedClientRelease(t, dev, admin)
	requested := pexpect(t, dev, "POST", "/api/v1/developer/test-assignments", service.AssignmentInput{ReleaseID: release, MerchantID: f.tenant, Reason: "Not approved"}, key(), 200)["assignment"].(map[string]any)["id"].(string)
	rpcCode(t, stop(f.tenant, "staff", requested, key()), connect.CodeAlreadyExists)
	rpcCode(t, stop(f.tenant, "staff", requested, retry), connect.CodeAlreadyExists)
}
