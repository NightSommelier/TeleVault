.PHONY: fmt lint test check js-check version version-info version-sync release-check compose-build compose-up

GO_CACHE ?= /tmp/televault-go-cache
SEMVER_ENV := $$(./scripts/semver.sh --env)

fmt:
	cd backend && gofmt -w $$(find . -name '*.go' -not -path './var/*')

js-check:
	node --check backend/internal/httpserver/static/scripts/app.js

lint: js-check
	cd backend && GOCACHE=$(GO_CACHE) go vet ./...
	git diff --check

test:
	cd backend && GOCACHE=$(GO_CACHE) go test ./... -count=1

check: lint test

version-info:
	@./scripts/semver.sh --env

version-sync:
	@./scripts/semver.sh --write-go-stable backend/internal/buildinfo/version_generated.go

version: version-sync

release-check:
	@./scripts/semver.sh --require-clean-release

compose-build:
	@$(MAKE) version-sync; \
	eval "$(SEMVER_ENV)"; \
	docker compose build \
		--build-arg APP_VERSION="$$APP_VERSION" \
		--build-arg APP_COMMIT="$$APP_COMMIT" \
		--build-arg APP_BUILD_DATE="$$APP_BUILD_DATE"

compose-up:
	@$(MAKE) version-sync; \
	eval "$(SEMVER_ENV)"; \
	docker compose build \
		--build-arg APP_VERSION="$$APP_VERSION" \
		--build-arg APP_COMMIT="$$APP_COMMIT" \
		--build-arg APP_BUILD_DATE="$$APP_BUILD_DATE"; \
	docker compose up -d
