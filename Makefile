.PHONY: up down logs migrate-inventory-up migrate-inventory-down test-inventory test-inventory-integration

up:
	docker compose up -d

down:
	docker compose down

logs:
	docker compose logs -f

migrate-inventory-up:
	docker compose run --rm --entrypoint /bin/sh inventory-migrate -c 'migrate -path=/migrations -database "postgres://$${INVENTORY_DB_USER}:$${INVENTORY_DB_PASSWORD}@$${INVENTORY_DB_HOST}:$${INVENTORY_DB_PORT}/$${INVENTORY_DB_NAME}?sslmode=disable" up'

migrate-inventory-down:
	docker compose run --rm --entrypoint /bin/sh inventory-migrate -c 'migrate -path=/migrations -database "postgres://$${INVENTORY_DB_USER}:$${INVENTORY_DB_PASSWORD}@$${INVENTORY_DB_HOST}:$${INVENTORY_DB_PORT}/$${INVENTORY_DB_NAME}?sslmode=disable" down -all'

test-inventory:
	docker run --rm -v "$(CURDIR)/services/inventory-service:/app" -w /app golang:1.25-alpine go test ./...

test-inventory-integration:
	@set -eu; \
	cleanup() { \
		docker compose --profile test stop inventory-test-db >/dev/null; \
		docker compose --profile test rm -f inventory-test-db >/dev/null; \
	}; \
	trap cleanup EXIT INT TERM; \
	docker compose --profile test up -d --wait inventory-test-db; \
	docker compose --profile test run --rm --no-deps \
		-e INVENTORY_DB_HOST=inventory-test-db \
		-e INVENTORY_DB_NAME="$${INVENTORY_TEST_DB_NAME:-inventory_test_db}" \
		--entrypoint /bin/sh inventory-migrate \
		-c 'migrate -path=/migrations -database "postgres://$${INVENTORY_DB_USER}:$${INVENTORY_DB_PASSWORD}@$${INVENTORY_DB_HOST}:$${INVENTORY_DB_PORT}/$${INVENTORY_DB_NAME}?sslmode=disable" up'; \
	docker compose --profile test run --rm --no-deps inventory-test-runner \
		go test -race -tags=integration ./tests/integration
