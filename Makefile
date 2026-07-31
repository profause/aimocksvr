.PHONY: build run test vet fmt lint db-up db-down migrate-check

build:
	go build ./...

run:
	go run ./cmd/server

test:
	go test ./...

test-integration:
	MOCKSVR_TEST_DATABASE_URL=$(MOCKSVR_TEST_DATABASE_URL) go test ./... -run Repository -v

vet:
	go vet ./...

fmt:
	gofmt -w .

db-up:
	docker compose up -d postgres

db-down:
	docker compose down
