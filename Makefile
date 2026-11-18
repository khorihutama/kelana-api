.PHONY: tidy fmt vet test test-int build run migrate-up migrate-down migrate-status seed swagger docker-up docker-down

tidy:
	go mod tidy

fmt:
	gofmt -w .

vet:
	go vet ./...

test:
	go test ./...

test-int:
	go test -tags=integration ./...

build:
	go build -o bin/api ./cmd/api

run:
	go run ./cmd/api

migrate-up:
	go run ./cmd/migrate up

migrate-down:
	go run ./cmd/migrate down 1

migrate-status:
	go run ./cmd/migrate status

seed:
	SEED_ENABLED=true go run ./cmd/seed

swagger:
	swag init -g cmd/api/main.go -o docs

docker-up:
	docker compose up -d postgres redis

docker-down:
	docker compose down