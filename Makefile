.PHONY: dev build test race vet fmt openapi lint-openapi tidy seed clean logs

SERVICES = hiscore-service recipe-service ge-price-service calc-service realtime-service gateway

dev:
	docker compose up --build

logs:
	docker compose logs -f

build:
	@mkdir -p bin
	@for s in $(SERVICES); do \
		echo ">> building $$s"; \
		go build -o bin/$$s ./$$s || exit 1; \
	done

test:
	go test ./...

race:
	go test -race ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w .

# Regenerate the per-service specs from openapi/combined.yaml, which is
# the single source of truth. The contract tests fail if these drift.
openapi:
	python3 scripts/split_openapi.py

# Requires network access for npx.
lint-openapi:
	@for f in openapi/*.yaml; do \
		npx --yes @redocly/cli@latest lint $$f || exit 1; \
	done

tidy:
	go mod tidy

seed:
	go run ./scripts/seed_items.go || true

clean:
	rm -rf bin/
	docker compose down -v
