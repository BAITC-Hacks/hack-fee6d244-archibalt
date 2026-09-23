DATABASE_URL ?= postgres://postgres:hack@localhost:5432/hack?sslmode=disable
export DATABASE_URL

.PHONY: run db test build up down

run: ## локальный сервер (нужна БД: make db)
	go run ./cmd/server

db: ## только Postgres в docker
	docker compose up -d db

test: ## unit-тесты; интеграционные store-тесты — с TEST_DATABASE_URL
	go test ./...

build:
	go build -o bin/server ./cmd/server

up: ## всё приложение в docker
	docker compose up --build

down:
	docker compose down
