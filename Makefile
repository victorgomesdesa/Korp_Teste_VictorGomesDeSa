.PHONY: up down logs migrate-inventory-up migrate-inventory-down

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
