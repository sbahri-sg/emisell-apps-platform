.PHONY: tools generate contracts test verify test-remote-lifecycle

tools:
	GOBIN=$(CURDIR)/bin go install github.com/bufbuild/buf/cmd/buf@v1.72.0
	GOBIN=$(CURDIR)/bin go install github.com/nats-io/nats-server/v2@v2.14.6

generate:
	bin/buf generate

contracts:
	bin/buf format --diff --exit-code
	bin/buf lint
	bin/buf breaking --against api/proto-baseline.binpb

test:
	go test -race ./...

# Refuse a green run caused only by skipped database/broker integration tests.
test-remote-lifecycle:
	@test -n "$(EMISELL_TEST_DATABASE_URL)" || (echo "Set EMISELL_TEST_DATABASE_URL to the isolated loopback emisell_local_test database"; exit 1)
	@test -x "$(EMISELL_NATS_SERVER)" || (echo "Set EMISELL_NATS_SERVER to an executable pinned NATS server"; exit 1)
	go test -race ./pkg/appapi ./internal/installation/domain
	go test -race ./internal/bootstrap -run 'Test(ConsentInstallationRemoteHandshakeAndCleanup|RemoteOAuthInvocationWebhooksAndCleanup|RemoteWebhookJetStreamDurability|RemoteOAuthExpiryPKCECodeReplayAndTenantBinding|ConsentInstallationTokenUninstallEndToEnd)$$' -count=1 -v

verify: contracts test
	go vet ./...
	go build ./...
