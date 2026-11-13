.PHONY: run build test live sql deploy token

run: ## run locally with .env
	@set -a; . ./.env; set +a; go build -o bin/api ./cmd/api && ./bin/api

build:
	go build ./... && go vet ./...

test:
	go test ./...

live: ## fixture + web lookup tests against the real API (costs about $0.50)
	@set -a; . ./.env; set +a; go test -tags live ./internal/ai -run TestLive -v -count=1 -timeout 20m

sql: ## read-only query:  make sql Q="select id, status from thoughts"
	@set -a; . ./.env; set +a; go run ./cmd/dbq "$(Q)"

deploy:
	fly deploy

token: ## print a random API token
	@openssl rand -hex 32
