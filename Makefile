.PHONY: help run test vet fmt docker-up docker-down ci ci-unit ci-integration

help:
	@echo "run             Run the Go API"
	@echo "test            Run Go tests"
	@echo "vet             Run go vet"
	@echo "fmt             Format Go source"
	@echo "docker-up       Start local PostgreSQL"
	@echo "docker-down     Stop local services"
	@echo "ci              Run local CI (unit + disposable integration)"
	@echo "ci-unit         Run local Go format, build, vet, and unit tests"
	@echo "ci-integration  Provision disposable semaian_ci_test, migrate, test, and drop"

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

ci-unit:
	test -z "$$(gofmt -l services/api)"
	cd services/api && go build ./... && go vet ./... && go test -count=1 -race ./...

ci-integration:
	@set -e; \
	db_name="semaian_ci_test_$$(date +%Y%m%d%H%M%S)_$$$$"; \
	created=0; \
	cleanup() { \
		rc=$$?; \
		if [ "$$created" -eq 1 ]; then \
			docker exec -i semaian-postgres psql -U semaian -d postgres -v ON_ERROR_STOP=1 -c "DROP DATABASE IF EXISTS \"$$db_name\";" >/dev/null 2>&1 || true; \
		fi; \
		exit "$$rc"; \
	}; \
	trap cleanup EXIT; \
	docker exec -i semaian-postgres psql -U semaian -d postgres -v ON_ERROR_STOP=1 -c "CREATE DATABASE \"$$db_name\";" >/dev/null; \
	created=1; \
	for f in services/api/internal/platform/migrations/*.up.sql; do \
		docker exec -i semaian-postgres psql -U semaian -d "$$db_name" -v ON_ERROR_STOP=1 < "$$f" >/dev/null; \
	done; \
	cd services/api && DATABASE_URL="postgres://semaian:local_only_change_me@127.0.0.1:5433/$$db_name?sslmode=disable" go test -count=1 -race -tags=integration ./...

ci: ci-unit ci-integration
