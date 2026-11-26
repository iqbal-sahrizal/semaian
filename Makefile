.PHONY: help run test vet fmt docker-up docker-down

help:
	@echo "run         Run the Go API"
	@echo "test        Run Go tests"
	@echo "vet         Run go vet"
	@echo "fmt         Format Go source"
	@echo "docker-up  Start local PostgreSQL"
	@echo "docker-down Stop local services"

run:
	cd services/api && go run ./cmd/api

test:
	cd services/api && go test ./...

vet:
	cd services/api && go vet ./...

fmt:
	gofmt -w services/api

docker-up:
	docker compose -f infrastructure/local/docker-compose.yml up -d postgres

docker-down:
	docker compose -f infrastructure/local/docker-compose.yml down
