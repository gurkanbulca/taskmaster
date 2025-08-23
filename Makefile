.PHONY: help proto update-proto generate run test build

help:
	@echo "Available commands:"
	@echo "  make proto        - Generate Go code from proto submodule"
	@echo "  make update-proto - Update proto submodule to latest"
	@echo "  make generate     - Generate all code (Ent + Proto)"
	@echo "  make run          - Run the server"
	@echo "  make test         - Run tests"
	@echo "  make build        - Build the server"

proto:
	@./scripts/generate-from-proto.sh

update-proto:
	@echo "Updating proto submodule..."
	@git submodule update --remote proto
	@cd proto && git pull origin main
	@echo "Proto submodule updated!"

generate: proto
	@echo "Generating Ent code..."
	@go generate ./ent
	@echo "All code generated!"

run:
	go run cmd/server/main.go

test:
	go test -v -race ./...

build:
	go build -o bin/server cmd/server/main.go

docker-build:
	docker build -t taskmaster-backend .

docker-run:
	docker-compose up -d