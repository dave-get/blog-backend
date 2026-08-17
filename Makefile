.PHONY: run test test-race lint migrate-up migrate-down migrate-local-up migrate-local-down docker-up docker-down

run:
	go run ./cmd/api

test:
	go test -cover ./...

test-race:
	CGO_ENABLED=1 go test -race ./...

lint:
	go vet ./...

migrate-up:
	docker compose run --rm migrate -path=/migrations -database "$${DATABASE_URL:-postgres://blog:blog@db:5432/blog?sslmode=disable}" up

migrate-down:
	docker compose run --rm migrate -path=/migrations -database "$${DATABASE_URL:-postgres://blog:blog@db:5432/blog?sslmode=disable}" down 1

migrate-local-up:
	go run ./cmd/migrate up

migrate-local-down:
	go run ./cmd/migrate down

docker-up:
	docker compose up --build

docker-down:
	docker compose down
