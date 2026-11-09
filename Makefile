# Repo-level tasks. Component-specific targets live in cli/Makefile etc.
BUF_VERSION := v1.75.0
PROTOC_GEN_GO_VERSION := v1.36.12
PROTOC_GEN_CONNECT_GO_VERSION := v1.21.1
GOBIN := $(CURDIR)/.tools/bin
export PATH := $(GOBIN):$(PATH)

.PHONY: tools proto proto-go proto-ts proto-check test

tools:
	GOBIN=$(GOBIN) go install github.com/bufbuild/buf/cmd/buf@$(BUF_VERSION)
	GOBIN=$(GOBIN) go install google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOC_GEN_GO_VERSION)
	GOBIN=$(GOBIN) go install connectrpc.com/connect/cmd/protoc-gen-connect-go@$(PROTOC_GEN_CONNECT_GO_VERSION)

## proto: lint the contract and regenerate all client/server code
proto: proto-go proto-ts

## proto-go: Go code for the server and CLI modules
proto-go: tools
	buf lint
	buf generate --template buf.gen.server.yaml
	buf generate --template buf.gen.cli.yaml

## proto-ts: TypeScript for mobile + web (needs `npm ci` in mobile/ for protoc-gen-es)
proto-ts: tools
	@if [ -x mobile/node_modules/.bin/protoc-gen-es ]; then \
		buf generate --template buf.gen.ts.yaml; \
	else \
		echo "skipping TS codegen: run 'npm ci' in mobile/ first"; \
	fi

## proto-check: fail if generated code is stale or the contract broke compatibility
proto-check: proto
	git diff --exit-code -- server/internal/gen cli/internal/gen mobile/src/gen web/src/gen
	@test -z "$$(git status --porcelain -- server/internal/gen cli/internal/gen mobile/src/gen web/src/gen)" || \
		{ echo "generated code is not committed:"; git status --short -- server/internal/gen cli/internal/gen mobile/src/gen web/src/gen; exit 1; }
	if git cat-file -e origin/main:proto 2>/dev/null; then buf breaking --against '.git#branch=origin/main'; fi

## test: run every test suite
test:
	cd server && go test ./...
	cd cli && go test ./...
	cd mobile && npx jest
