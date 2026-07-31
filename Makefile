.PHONY: all help build test cover fixtures vet fmt lint md-lint tidy hooks check sample clean

all: check ## Default: run the full local check suite

help: ## Show available targets
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'

build: ## Build every package
	go build -o /dev/null ./...

test: ## Run the test suite
	go test ./...

cover: ## Run tests with a coverage profile
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

fixtures: ## Replay the golden fixtures captured from the libtcod C build
	go test -count=1 -v ./internal/fixtures/...

vet: ## go vet
	go vet ./...

fmt: ## Format Go sources
	gofmt -w .

lint: ## golangci-lint (config: .golangci.yml)
	golangci-lint run

md-lint: ## markdownlint (config: .markdownlint-cli2.jsonc)
	npx --yes markdownlint-cli2 "**/*.md"

tidy: ## Tidy the module file
	go mod tidy

hooks: ## Install pre-commit hooks
	pre-commit install

check: fmt vet lint test md-lint ## Run the full local check suite

sample: ## Render the sample dungeon
	go run ./cmd/sample

clean: ## Remove generated renders and coverage output
	rm -f sample_dungeon.png sample_dungeon.ans coverage.out coverage.html
