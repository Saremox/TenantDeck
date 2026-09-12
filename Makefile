.PHONY: verify build run test fmt vet frontend-build frontend-verify helm-verify e2e e2e-up e2e-down clean

# Scoped explicitly rather than "./..." from the repo root: that pattern
# also walks web/node_modules (no go.mod boundary there) and would also
# pick up e2e/ (build-tag gated, needs a live cluster) - see
# docs/implementation-plan.md.
GO_PACKAGES := ./cmd/... ./internal/...

# Fast, deterministic checks a contributor (and CI) runs before every
# change - docs/spec/01-working-agreement.md "make verify" requirement.
verify: fmt vet test frontend-verify helm-verify

helm-verify:
	@echo "==> helm lint/template (example values - values.yaml's own required-but-empty config makes a bare 'helm lint' with no values file fail by design, see values.schema.json)"
	helm lint charts/tenantdeck -f charts/tenantdeck/ci/values-dev.yaml
	helm lint charts/tenantdeck -f charts/tenantdeck/ci/values-prod-example.yaml
	helm template tenantdeck charts/tenantdeck -f charts/tenantdeck/ci/values-dev.yaml > /dev/null

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


# Mandatory disposable full-stack E2E suite
# (docs/spec/07-mandatory-automated-testing.md) - kind + Calico + Capsule
# + Valkey + the real built image/chart + the mock OIDC provider, two
# TenantDeck replicas. Always tears down, even on test failure. See
# e2e/up.sh for exactly what it stands up and e2e/README.md (and
# docs/implementation-plan.md Phase 4 "Known blockers") for what has and
# hasn't actually been run end-to-end yet.
e2e: e2e-up
	@trap '$(MAKE) e2e-down' EXIT; ./e2e/run-tests.sh

e2e-up:
	./e2e/up.sh

e2e-down:
	./e2e/down.sh

clean:
	rm -rf bin web/dist/* web/node_modules
	touch web/dist/.gitkeep
