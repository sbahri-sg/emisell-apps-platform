.PHONY: tools generate contracts test verify

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

verify: contracts test
	go vet ./...
	go build ./...
