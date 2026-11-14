.PHONY: run dev build test dbtest live sql backup deploy token

run: ## run against the REAL database, with Telegram polling and the ticker off
	@set -a; . ./.env; set +a; BOT_ENABLED=false go build -o bin/api ./cmd/api && BOT_ENABLED=false ./bin/api

dev: ## run against the "dev" schema; outgoing Telegram messages are logged, not sent
	@set -a; . ./.env; set +a; go build -o bin/api ./cmd/api && DB_SCHEMA=dev DRY_SEND=true BOT_ENABLED=false PORT=18080 ./bin/api

build:
	go build ./... && go vet ./...

test:
	go test ./...

dbtest: ## the store's SQL against a throwaway schema (created and dropped by the test)
	@set -a; . ./.env; set +a; TEST_DATABASE_URL="$$DATABASE_URL" go test -tags db ./internal/store -count=1

backup: ## export every table to ~/companion-backups and prove the file restores
	@mkdir -p $(HOME)/companion-backups
	@set -a; . ./.env; set +a; f=$(HOME)/companion-backups/companion-$$(date +%Y%m%d-%H%M).jsonl.gz; \
		go run ./cmd/backup export $$f && go run ./cmd/backup verify $$f

live: ## fixture + web lookup tests against the real API (costs about $0.50)
	@set -a; . ./.env; set +a; go test -tags live ./internal/ai -run TestLive -v -count=1 -timeout 20m

sql: ## read-only query:  make sql Q="select id, status from thoughts"
	@set -a; . ./.env; set +a; go run ./cmd/dbq "$(Q)"

deploy:
	fly deploy

token: ## print a random API token
	@openssl rand -hex 32
