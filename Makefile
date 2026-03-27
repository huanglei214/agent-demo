GO ?= go
CLI_APP := ./cmd/cli
WEB_APP := ./cmd/web
CACHE_ENV := GOMODCACHE=$(CURDIR)/.gomodcache GOCACHE=$(CURDIR)/.gocache
PROVIDER ?= ark
MODEL ?=
WORKSPACE ?= $(CURDIR)
HOST ?= 127.0.0.1
PORT ?= 8080

.PHONY: help build tidy run chat serve dev inspect session-inspect replay resume tools debug-events verify-scenarios web-dev web-build clean-runtime clean-cache

help:
	@$(CACHE_ENV) $(GO) run $(CLI_APP) --help

build:
	@$(CACHE_ENV) $(GO) build ./...

tidy:
	@$(CACHE_ENV) $(GO) mod tidy

run:
	@if [ -z "$(ARGS)" ]; then echo "usage: make run ARGS='your instruction'"; exit 1; fi
	@$(CACHE_ENV) $(GO) run $(CLI_APP) --workspace "$(WORKSPACE)" --provider "$(PROVIDER)" $(if $(MODEL),--model "$(MODEL)",) run $(if $(SESSION),--session "$(SESSION)",) $(if $(SKILL),--skill "$(SKILL)",) "$(ARGS)"

chat:
	@$(CACHE_ENV) $(GO) run $(CLI_APP) --workspace "$(WORKSPACE)" --provider "$(PROVIDER)" $(if $(MODEL),--model "$(MODEL)",) chat $(if $(SESSION),--session "$(SESSION)",) $(if $(SKILL),--skill "$(SKILL)",) $(if $(DEBUG),--debug,)

serve:
	@$(CACHE_ENV) $(GO) run $(WEB_APP) --workspace "$(WORKSPACE)" --provider "$(PROVIDER)" $(if $(MODEL),--model "$(MODEL)",) --host "$(HOST)" --port "$(PORT)"

dev:
	@chmod +x scripts/dev.sh
	@./scripts/dev.sh

inspect:
	@if [ -z "$(RUN)" ]; then echo "usage: make inspect RUN=<run-id>"; exit 1; fi
	@$(CACHE_ENV) $(GO) run $(CLI_APP) --workspace "$(WORKSPACE)" inspect "$(RUN)"

session-inspect:
	@if [ -z "$(SESSION)" ]; then echo "usage: make session-inspect SESSION=<session-id>"; exit 1; fi
	@$(CACHE_ENV) $(GO) run $(CLI_APP) --workspace "$(WORKSPACE)" session inspect "$(SESSION)"

replay:
	@if [ -z "$(RUN)" ]; then echo "usage: make replay RUN=<run-id>"; exit 1; fi
	@$(CACHE_ENV) $(GO) run $(CLI_APP) --workspace "$(WORKSPACE)" replay "$(RUN)"

resume:
	@if [ -z "$(RUN)" ]; then echo "usage: make resume RUN=<run-id>"; exit 1; fi
	@$(CACHE_ENV) $(GO) run $(CLI_APP) --workspace "$(WORKSPACE)" resume "$(RUN)"

tools:
	@$(CACHE_ENV) $(GO) run $(CLI_APP) --workspace "$(WORKSPACE)" tools list

debug-events:
	@if [ -z "$(RUN)" ]; then echo "usage: make debug-events RUN=<run-id>"; exit 1; fi
	@$(CACHE_ENV) $(GO) run $(CLI_APP) --workspace "$(WORKSPACE)" debug events "$(RUN)"

verify-scenarios:
	@$(CACHE_ENV) $(GO) test ./internal/app -run TestScenarioRegression -v

web-dev:
	@cd web && npm run dev

web-build:
	@cd web && npm run build

clean-runtime:
	@rm -rf .runtime

clean-cache:
	@rm -rf .gocache .gomodcache
