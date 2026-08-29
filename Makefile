.PHONY: build run test vet fmt lint db-up db-down migrate-check \
        web-dev web-build web-lint web-typecheck web-format

build:
	go build ./...

run:
	go run ./cmd/server

test:
	go test ./...

# Run database-backed repository tests serially: they share one test database.
test-integration:
	MOCKSVR_TEST_DATABASE_URL=$(MOCKSVR_TEST_DATABASE_URL) go test -p 1 ./... -run Repository -v

vet:
	go vet ./...

fmt:
	gofmt -w .

db-up:
	docker compose up -d postgres

db-down:
	docker compose down

web-dev:
	cd web && npm run dev

web-build:
	cd web && npm run build

web-lint:
	cd web && npm run lint

web-typecheck:
	cd web && npm run typecheck

web-format:
	cd web && npm run format
