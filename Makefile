.PHONY: fmt lint test check js-check

GO_CACHE ?= /tmp/televault-go-cache

fmt:
	cd backend && gofmt -w $$(find . -name '*.go' -not -path './var/*')

js-check:
	perl -0ne 'print $$1 if /<script>(.*)<\/script>/s' backend/internal/httpserver/static/index.html | node --check

lint: js-check
	cd backend && GOCACHE=$(GO_CACHE) go vet ./...
	git diff --check

test:
	cd backend && GOCACHE=$(GO_CACHE) go test ./... -count=1

check: lint test
