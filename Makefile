.DEFAULT_GOAL := help

.PHONY: help up down logs ps frontend worker build reset migrate-up migrate-down migrate-inventory-up migrate-inventory-down migrate-billing-up migrate-billing-down test test-all test-inventory test-inventory-integration test-billing test-billing-integration test-billing-e2e test-frontend test-frontend-e2e test-worker

help:
	@printf '%s\n' \
		'Uso: make <target>' \
		'' \
		'Ambiente:' \
		'  up                          Inicia o ambiente de desenvolvimento' \
		'  down                        Para os containers sem remover dados' \
		'  logs                        Acompanha os logs do Docker Compose' \
		'  ps                          Exibe o estado dos containers' \
		'  frontend                    Inicia o servidor de desenvolvimento Angular' \
		'  worker                      Inicia o assistente com Workers AI' \
		'  build                       Compila backend, frontend e valida o Worker' \
		'  reset                       Recria o ambiente e APAGA os dados locais' \
		'' \
		'Migrations:' \
		'  migrate-up                  Aplica todas as migrations' \
		'  migrate-down                Reverte todas as migrations' \
		'  migrate-inventory-up        Aplica migrations do Inventory' \
		'  migrate-inventory-down      Reverte migrations do Inventory' \
		'  migrate-billing-up          Aplica migrations do Billing' \
		'  migrate-billing-down        Reverte migrations do Billing' \
		'' \
		'Testes:' \
		'  test                        Executa testes do backend, frontend e Worker' \
		'  test-all                    Executa testes unitários, integração e E2E' \
		'  test-inventory              Executa testes unitários do Inventory' \
		'  test-inventory-integration  Executa integração do Inventory com race detector' \
		'  test-billing                Executa testes unitários do Billing' \
		'  test-billing-integration    Executa integração do Billing com race detector' \
		'  test-billing-e2e            Executa E2E Billing para Inventory com race detector' \
		'  test-frontend               Executa testes unitários do frontend' \
		'  test-frontend-e2e           Executa a jornada E2E principal do frontend' \
		'  test-worker                 Testa tipos, regras e empacotamento do assistente IA'

up:
	docker compose up -d

down:
	docker compose down

logs:
	docker compose logs -f

ps:
	docker compose ps

frontend:
	npm --prefix frontend start

worker:
	npm --prefix worker run dev

build:
	docker compose build inventory-service billing-service
	npm --prefix frontend run build
	npm --prefix worker run check

reset:
	@echo 'ATENÇÃO: este comando apagará todos os dados locais do projeto.'
	docker compose down -v
	$(MAKE) up
	$(MAKE) migrate-up

migrate-up: migrate-inventory-up migrate-billing-up

migrate-down: migrate-billing-down migrate-inventory-down

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

test-frontend:
	npm --prefix frontend test -- --watch=false

test-worker:
	npm --prefix worker run check

test: test-inventory test-billing test-frontend test-worker

test-all: test test-inventory-integration test-billing-integration test-billing-e2e test-frontend-e2e

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

test-frontend-e2e:
	@set -eu; \
	cleanup() { \
		invoices=$$(docker compose exec -T billing-db psql -U $${BILLING_DB_USER:-billing} -d $${BILLING_DB_NAME:-billing_db} -t -A \
			-c "SELECT DISTINCT invoice_id FROM invoice_items WHERE product_description LIKE 'E2E %'" | paste -sd, -); \
		if [ -n "$$invoices" ]; then \
			docker compose exec -T inventory-db psql -U $${INVENTORY_DB_USER:-inventory} -d $${INVENTORY_DB_NAME:-inventory_db} \
				-c "DELETE FROM stock_operations WHERE invoice_id IN ($$invoices)" >/dev/null; \
			docker compose exec -T billing-db psql -U $${BILLING_DB_USER:-billing} -d $${BILLING_DB_NAME:-billing_db} \
				-c "DELETE FROM invoice_close_operations WHERE invoice_id IN ($$invoices)" \
				-c "DELETE FROM invoice_items WHERE invoice_id IN ($$invoices)" \
				-c "DELETE FROM invoices WHERE id IN ($$invoices)" >/dev/null; \
		fi; \
		docker compose exec -T inventory-db psql -U $${INVENTORY_DB_USER:-inventory} -d $${INVENTORY_DB_NAME:-inventory_db} \
			-c "DELETE FROM products WHERE description LIKE 'E2E %'" >/dev/null; \
	}; \
	trap cleanup EXIT INT TERM; \
	docker compose up -d --build --wait inventory-service billing-service; \
	$(MAKE) migrate-inventory-up; \
	$(MAKE) migrate-billing-up; \
	npm --prefix frontend run e2e
