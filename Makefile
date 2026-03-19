# Common build commands

.PHONY: all
all: build

.PHONY: build
build:
	go build -o bin/calorie-count ./cmd/server

.PHONY: run
run:
	go run ./cmd/server

.PHONY: test
test:
	go test ./...

.PHONY: lint
lint:
	gofmt -w .
	go vet ./...

.PHONY: clean
clean:
	rm -rf bin/
	rm -rf dist/

.PHONY: install
install:
	go mod tidy

.PHONY: generate
generate:
	# Add code generation commands here

.PHONY: migrate
migrate:
	# Migrations are applied automatically on server startup via RunMigrations.
	# This target is a placeholder for future CLI migration tooling.
	@echo "Migrations run automatically on server start."

.PHONY: docker-build
docker-build:
	docker build -t calorie-count .

.PHONY: docker-run
docker-run:
	docker compose up
