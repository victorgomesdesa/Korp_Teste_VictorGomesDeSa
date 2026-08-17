.PHONY: up down logs migrate-inventory-up migrate-inventory-down migrate-billing-up migrate-billing-down test-inventory test-inventory-integration test-billing test-billing-integration test-billing-e2e

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

migrate-billing-up:
	docker compose run --rm --entrypoint /bin/sh billing-migrate -c 'migrate -path=/migrations -database "postgres://$${BILLING_DB_USER}:$${BILLING_DB_PASSWORD}@$${BILLING_DB_HOST}:$${BILLING_DB_PORT}/$${BILLING_DB_NAME}?sslmode=disable" up'

migrate-billing-down:
	docker compose run --rm --entrypoint /bin/sh billing-migrate -c 'migrate -path=/migrations -database "postgres://$${BILLING_DB_USER}:$${BILLING_DB_PASSWORD}@$${BILLING_DB_HOST}:$${BILLING_DB_PORT}/$${BILLING_DB_NAME}?sslmode=disable" down -all'

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

test-billing:
	docker run --rm -v "$(CURDIR)/services/billing-service:/app" -w /app golang:1.25-alpine go test ./...

test-billing-integration:
	@set -eu; \
	cleanup() { \
		docker compose --profile test stop billing-test-db >/dev/null; \
		docker compose --profile test rm -f billing-test-db >/dev/null; \
	}; \
	trap cleanup EXIT INT TERM; \
	docker compose --profile test up -d --wait billing-test-db; \
	docker compose --profile test run --rm --no-deps \
		-e BILLING_DB_HOST=billing-test-db \
		-e BILLING_DB_NAME="$${BILLING_TEST_DB_NAME:-billing_test_db}" \
		--entrypoint /bin/sh billing-migrate \
		-c 'migrate -path=/migrations -database "postgres://$${BILLING_DB_USER}:$${BILLING_DB_PASSWORD}@$${BILLING_DB_HOST}:$${BILLING_DB_PORT}/$${BILLING_DB_NAME}?sslmode=disable" up'; \
	docker compose --profile test run --rm --no-deps billing-test-runner \
		go test -race -tags=integration ./tests/integration

test-billing-e2e:
	@set -eu; \
	cleanup() { \
		docker compose --profile test stop billing-test-service inventory-test-service billing-test-db inventory-test-db >/dev/null; \
		docker compose --profile test rm -f billing-test-service inventory-test-service billing-test-db inventory-test-db >/dev/null; \
	}; \
	trap cleanup EXIT INT TERM; \
	docker compose --profile test up -d --wait billing-test-db inventory-test-db; \
	docker compose --profile test run --rm --no-deps \
		-e INVENTORY_DB_HOST=inventory-test-db \
		-e INVENTORY_DB_NAME="$${INVENTORY_TEST_DB_NAME:-inventory_test_db}" \
		--entrypoint /bin/sh inventory-migrate \
		-c 'migrate -path=/migrations -database "postgres://$${INVENTORY_DB_USER}:$${INVENTORY_DB_PASSWORD}@$${INVENTORY_DB_HOST}:$${INVENTORY_DB_PORT}/$${INVENTORY_DB_NAME}?sslmode=disable" up'; \
	docker compose --profile test run --rm --no-deps \
		-e BILLING_DB_HOST=billing-test-db \
		-e BILLING_DB_NAME="$${BILLING_TEST_DB_NAME:-billing_test_db}" \
		--entrypoint /bin/sh billing-migrate \
		-c 'migrate -path=/migrations -database "postgres://$${BILLING_DB_USER}:$${BILLING_DB_PASSWORD}@$${BILLING_DB_HOST}:$${BILLING_DB_PORT}/$${BILLING_DB_NAME}?sslmode=disable" up'; \
	docker compose --profile test up -d --build --wait inventory-test-service billing-test-service; \
	docker compose --profile test run --rm --no-deps billing-test-runner \
		go test -race -count=1 -tags=e2e ./tests/e2e -run '^TestOnline'; \
	docker compose --profile test stop inventory-test-service; \
	docker compose --profile test run --rm --no-deps billing-test-runner \
		go test -race -count=1 -tags=e2e ./tests/e2e -run '^TestInventoryOffline'
