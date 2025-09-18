SHELL := /bin/bash
GO ?= go
GOTOOLCHAIN ?= local
GOMODCACHE ?= $(CURDIR)/.cache/gomod
GOCACHE ?= $(CURDIR)/.cache/gocache
CMD := ./cmd/postgres-mcp
BIN_DIR := $(CURDIR)/bin
BIN := $(BIN_DIR)/postgres-mcp
LISTEN ?= :8080

CACHE_ENV := GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) GOTOOLCHAIN=$(GOTOOLCHAIN)
CACHE_DIRS := $(GOCACHE) $(GOMODCACHE)

.PHONY: build test run-stdio run-http fmt tidy cache-dirs clean

cache-dirs:
	@mkdir -p $(CACHE_DIRS)

build: cache-dirs $(BIN)
	@echo "Built $(BIN)"

$(BIN_DIR):
	@mkdir -p $(BIN_DIR)

$(BIN): cache-dirs | $(BIN_DIR)
	$(CACHE_ENV) $(GO) build -o $@ ./cmd/postgres-mcp

clean:
	rm -rf $(BIN_DIR)

test: cache-dirs
	$(CACHE_ENV) $(GO) test ./...

run-stdio: cache-dirs
	@if [ -z "$(DATABASE_URL)" ]; then \
		echo "DATABASE_URL must be set" >&2; \
		exit 1; \
	fi
	$(CACHE_ENV) $(GO) run $(CMD) --mode=stdio --database-url "$(DATABASE_URL)" $(EXTRA_FLAGS)

run-http: cache-dirs
	@if [ -z "$(DATABASE_URL)" ]; then \
		echo "DATABASE_URL must be set" >&2; \
		exit 1; \
	fi
	$(CACHE_ENV) $(GO) run $(CMD) --mode=http --listen "$(LISTEN)" --database-url "$(DATABASE_URL)" $(EXTRA_FLAGS)

fmt:
	$(GO) fmt ./...

tidy: cache-dirs
	$(CACHE_ENV) $(GO) mod tidy
