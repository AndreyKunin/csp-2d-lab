# ── Configurable ports ────────────────────────────────────────────────────────
LOCAL_PORT   ?= 8080
SERVER_HOST  ?= 0.0.0.0
SERVER_PORT  ?= 8080
DEV_UI_PORT  ?= 3000
DOCKER_IMAGE ?= csp2d
DOCKER_TAG   ?= latest
VERSION      ?= 1.0

# ── Local Go build & run ──────────────────────────────────────────────────────
.PHONY: build
build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o csp2d .

.PHONY: run
run: build
	./csp2d

# ── Frontend build (required before desktop builds) ───────────────────────────
.PHONY: npm-build
npm-build:
	cd static && npm run build

# ── Frontend dev server ───────────────────────────────────────────────────────
.PHONY: npm-start
npm-start:
	cd static && npm start

# ── Desktop builds ────────────────────────────────────────────────────────────
.PHONY: build-windows
build-windows: npm-build
	mkdir -p dist
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
		go build -tags desktop -ldflags="-s -w -H windowsgui" -o dist/csp2d-windows-amd64-$(VERSION).exe .

.PHONY: build-macos
build-macos: npm-build
	mkdir -p dist
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 \
		go build -tags desktop -ldflags="-s -w" -o dist/csp2d-macos-arm64 .

.PHONY: build-desktop
build-desktop: build-windows build-macos

# ── Docker ────────────────────────────────────────────────────────────────────
.PHONY: docker-build
docker-build:
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

.PHONY: docker-run
docker-run:
	docker run --rm -p $(LOCAL_PORT):$(SERVER_PORT) -e SERVER_HOST=$(SERVER_HOST) -e SERVER_PORT=$(SERVER_PORT) $(DOCKER_IMAGE):$(DOCKER_TAG)

.PHONY: docker-run-d
docker-run-d:
	docker run --rm -d -p $(LOCAL_PORT):$(SERVER_PORT) -e SERVER_HOST=$(SERVER_HOST) -e SERVER_PORT=$(SERVER_PORT) $(DOCKER_IMAGE):$(DOCKER_TAG)

.PHONY: docker-build-run
docker-build-run: docker-build docker-run

# ── Tests ─────────────────────────────────────────────────────────────────────
# Tests live under test/. Unit tests reach unexported code through the
# build-tag-gated hook files (ga/testhooks.go, problem/csp2d/testhooks.go).
# godog flags use pflag syntax: --godog.tags=..., --godog.format=...
GODOG_PR_TAGS ?= ~@pending && ~@slow && ~@quality && ~@resource
GODOG_FORMAT  ?= pretty
COVER_DIR     ?= coverage

.PHONY: test
test: test-unit test-functional

.PHONY: test-unit
test-unit:
	go test -tags testhooks -coverpkg=csp2d/... ./test/unit/... ./test/support/...

.PHONY: test-unit-race
test-unit-race:
	CGO_ENABLED=1 go test -tags testhooks -race ./test/unit/... ./test/support/...

.PHONY: test-unit-pending
test-unit-pending:
	CSP2D_RUN_PENDING=1 go test -tags testhooks ./test/unit/...

.PHONY: test-functional
test-functional:
	go test -count=1 ./test/functional/ -args --godog.tags='$(GODOG_PR_TAGS)' --godog.format=$(GODOG_FORMAT)

.PHONY: test-functional-race
test-functional-race:
	go test -count=1 ./test/functional/ -args --server.race --godog.tags='$(GODOG_PR_TAGS)' --godog.format=$(GODOG_FORMAT)

.PHONY: test-nightly
test-nightly:
	go test -count=1 -timeout 2h ./test/functional/ -args --godog.tags='~@pending' --godog.format=$(GODOG_FORMAT)

.PHONY: test-pending
test-pending:
	-go test -count=1 ./test/functional/ -args --godog.tags='@pending' --godog.format=$(GODOG_FORMAT)

.PHONY: test-functional-cover
test-functional-cover:
	rm -rf $(COVER_DIR)/functional && mkdir -p $(COVER_DIR)/functional
	go test -count=1 ./test/functional/ -args --server.coverdir=$(abspath $(COVER_DIR))/functional --godog.tags='$(GODOG_PR_TAGS)' --godog.format=progress
	go tool covdata percent -i=$(COVER_DIR)/functional

.PHONY: bench
bench:
	go test -tags testhooks -run='^$$' -bench=. -benchmem ./test/bench/...

# ── Cleanup ───────────────────────────────────────────────────────────────────
.PHONY: clean
clean:
	rm -f csp2d csp2d-windows-amd64.exe csp2d-macos-arm64
	cd static && npx gulp clean