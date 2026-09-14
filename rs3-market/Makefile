.PHONY: dev build test tidy seed clean logs

SERVICES = hiscore-service recipe-service ge-price-service calc-service realtime-service

dev:
	docker compose up --build

logs:
	docker compose logs -f

build:
	@mkdir -p bin
	@for s in $(SERVICES); do \
		echo ">> building $$s"; \
		(cd $$s && go build -o ../bin/$$s .) ; \
	done

test:
	go test ./...

tidy:
	go mod tidy

seed:
	go run ./scripts/seed_items.go || true

clean:
	rm -rf bin/
	docker compose down -v
