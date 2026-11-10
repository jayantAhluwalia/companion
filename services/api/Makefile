.PHONY: run build test deploy token

run: ## run locally with .env
	@set -a; . ./.env; set +a; go build -o bin/api ./cmd/api && ./bin/api

build:
	go build ./... && go vet ./...

test:
	go test ./...

deploy:
	fly deploy

token: ## print a random API token
	@openssl rand -hex 32
