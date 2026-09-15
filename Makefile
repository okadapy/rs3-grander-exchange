# One command brings the whole thing up: nine backend services, the
# built frontend, and the nginx that puts them on a single origin.
COMPOSE := docker compose
PUBLIC_PORT ?= 80

.DEFAULT_GOAL := up

.PHONY: up
up: spec ## Build and start everything, then wait for it to answer
	$(COMPOSE) up -d --build
	@echo "waiting for the stack to answer on :$(PUBLIC_PORT) ..."
	@for i in $$(seq 1 60); do \
	  if curl -fsS "http://localhost:$(PUBLIC_PORT)/health" >/dev/null 2>&1; then \
	    echo "up: http://localhost:$(PUBLIC_PORT)"; exit 0; \
	  fi; \
	  sleep 2; \
	done; \
	echo "the stack did not answer in 120s; try 'make logs'"; exit 1

.PHONY: down
down: ## Stop everything, keeping the database and cache volumes
	$(COMPOSE) down

.PHONY: destroy
destroy: ## Stop everything and delete the database and cache volumes
	$(COMPOSE) down -v

.PHONY: logs
logs: ## Follow the logs of every service
	$(COMPOSE) logs -f --tail=100

.PHONY: ps
ps: ## Show what is running
	$(COMPOSE) ps

.PHONY: spec
spec: ## Regenerate the OpenAPI spec and hand the frontend its copy
	$(MAKE) -C backend openapi
	cp backend/openapi/combined.yaml web/combined.yaml

.PHONY: test
test: ## Run the backend test suite
	$(MAKE) -C backend test

.PHONY: rebuild
rebuild: ## Rebuild one service, e.g. make rebuild SERVICE=calc-service
	@test -n "$(SERVICE)" || { echo "usage: make rebuild SERVICE=<name>"; exit 2; }
	$(COMPOSE) up -d --build $(SERVICE)

.PHONY: help
help: ## List the targets
	@grep -hE '^[a-z][a-z-]*:.*?## ' $(MAKEFILE_LIST) \
	  | awk -F':.*?## ' '{printf "  %-10s %s\n", $$1, $$2}'
