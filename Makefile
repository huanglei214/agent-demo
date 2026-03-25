GO ?= go
APP := ./cmd/harness
CACHE_ENV := GOMODCACHE=$(CURDIR)/.gomodcache GOCACHE=$(CURDIR)/.gocache
PROVIDER ?= ark
MODEL ?=
WORKSPACE ?= $(CURDIR)

.PHONY: help build tidy run chat inspect replay resume tools debug-events clean-runtime clean-cache

help:
	@$(CACHE_ENV) $(GO) run $(APP) --help

build:
	@$(CACHE_ENV) $(GO) build ./...

tidy:
	@$(CACHE_ENV) $(GO) mod tidy

run:
	@if [ -z "$(ARGS)" ]; then echo "usage: make run ARGS='your instruction'"; exit 1; fi
	@$(CACHE_ENV) $(GO) run $(APP) --workspace "$(WORKSPACE)" --provider "$(PROVIDER)" $(if $(MODEL),--model "$(MODEL)",) run $(if $(SESSION),--session "$(SESSION)",) "$(ARGS)"

chat:
	@$(CACHE_ENV) $(GO) run $(APP) --workspace "$(WORKSPACE)" --provider "$(PROVIDER)" $(if $(MODEL),--model "$(MODEL)",) chat $(if $(SESSION),--session "$(SESSION)",)

inspect:
	@if [ -z "$(RUN)" ]; then echo "usage: make inspect RUN=<run-id>"; exit 1; fi
	@$(CACHE_ENV) $(GO) run $(APP) --workspace "$(WORKSPACE)" inspect "$(RUN)"

replay:
	@if [ -z "$(RUN)" ]; then echo "usage: make replay RUN=<run-id>"; exit 1; fi
	@$(CACHE_ENV) $(GO) run $(APP) --workspace "$(WORKSPACE)" replay "$(RUN)"

resume:
	@if [ -z "$(RUN)" ]; then echo "usage: make resume RUN=<run-id>"; exit 1; fi
	@$(CACHE_ENV) $(GO) run $(APP) --workspace "$(WORKSPACE)" resume "$(RUN)"

tools:
	@$(CACHE_ENV) $(GO) run $(APP) --workspace "$(WORKSPACE)" tools list

debug-events:
	@if [ -z "$(RUN)" ]; then echo "usage: make debug-events RUN=<run-id>"; exit 1; fi
	@$(CACHE_ENV) $(GO) run $(APP) --workspace "$(WORKSPACE)" debug events "$(RUN)"

clean-runtime:
	@rm -rf .runtime

clean-cache:
	@rm -rf .gocache .gomodcache
