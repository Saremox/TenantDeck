.PHONY: verify build run test fmt vet frontend-build frontend-verify clean

# Scoped explicitly rather than "./..." from the repo root: that pattern
# also walks web/node_modules (no go.mod boundary there) and can pick up
# stray vendored .go files - see docs/implementation-plan.md.
GO_PACKAGES := ./cmd/... ./internal/...

# Fast, deterministic checks a contributor (and CI) runs before every
# change - docs/spec/01-working-agreement.md "make verify" requirement.
# Helm lint/template/schema validation is added here once the chart exists
# (Phase 4, docs/implementation-plan.md) - not yet.
verify: fmt vet test frontend-verify

fmt:
	@echo "==> gofmt"
	@test -z "$$(gofmt -l $$(find . -name '*.go' -not -path './web/node_modules/*'))" || \
		(echo "gofmt found unformatted files - run: gofmt -w <file>"; \
		 gofmt -l $$(find . -name '*.go' -not -path './web/node_modules/*'); exit 1)

vet:
	@echo "==> go vet"
	go vet $(GO_PACKAGES)

test:
	@echo "==> go test (race)"
	go test $(GO_PACKAGES) -race

frontend-verify:
	@echo "==> frontend: typecheck, lint, test, build"
	cd web && npm run typecheck
	cd web && npm run lint
	cd web && npx vitest run
	cd web && npm run build

frontend-build:
	cd web && npm ci && npm run build

# Builds the real binary with the real frontend embedded - run
# frontend-build first (or `make build`, which does both).
build: frontend-build
	go build -o bin/tenantdeck ./cmd/tenantdeck

# Runs the built server. Requires the environment variables documented in
# docs/local-development.md to already be set (e.g. via `set -a && source
# .env && set +a`) - there are no insecure defaults to fall back to.
run:
	go run ./cmd/tenantdeck

clean:
	rm -rf bin web/dist/* web/node_modules
	touch web/dist/.gitkeep
