# Ethiopian Neobank - Build & Development Makefile
# Usage: make help

.PHONY: all build test lint vet fmt tidy clean migrate up down help \
        build-api api run-ui dev dev-api dev-ui dev-stop dev-fresh ui-build ui-lint \
        docker-build logs check seed-loan-capital seed-credit-profile run-admin dev-admin

UI_DIR := ../neo-ui
DATABASE_URL ?= postgres://neobank:neobank_dev@localhost:5432/neobank?sslmode=disable
FORMANCE_URL ?= http://localhost:3068

# ---- Build ----

all: lint test build ## Run lint, test, and build

build: ## Build all Go binaries to ./bin/
	@mkdir -p bin
	go build -o bin/neo-api ./cmd/api
	go build -o bin/neo-ethswitch-gateway ./cmd/ethswitch-gateway
	go build -o bin/neo-recon-worker ./cmd/recon-worker
	go build -o bin/neo-lending-worker ./cmd/lending-worker
	go build -o bin/neo-telegram-bot ./cmd/telegram-bot
	@echo "✓ All binaries built in ./bin/"

build-api: ## Build just the API server
	go build -o bin/neo-api ./cmd/api

# ---- Quality ----

test: ## Run all Go tests
	go test -p 1 ./... -v --failfast -race -timeout 600s

test-cover: ## Run Go tests with coverage
	go test ./... -count=1 -coverprofile=coverage.out
	go tool cover -func=coverage.out
	@echo "HTML report: go tool cover -html=coverage.out"

lint: vet fmt ## Run vet and check formatting

vet: ## Run go vet
	go vet ./...

fmt: ## Check gofmt compliance
	@test -z "$$(gofmt -l .)" || (echo "Files need formatting:" && gofmt -l . && exit 1)

tidy: ## Run go mod tidy
	go mod tidy

vulncheck: ## Run Go vulnerability scanner
	@command -v govulncheck >/dev/null 2>&1 || go install golang.org/x/vuln/cmd/govulncheck@latest
	govulncheck ./...

check: lint test vulncheck ui-lint ui-build ## Full CI check: Go lint+test+vulncheck, UI lint+build

# ---- Database ----

migrate: ## Run all migrations (requires migrate CLI)
	migrate -path migrations -database "$(DATABASE_URL)" up

migrate-down: ## Roll back the last migration
	migrate -path migrations -database "$(DATABASE_URL)" down 1

migrate-status: ## Show migration status
	migrate -path migrations -database "$(DATABASE_URL)" version

# ---- Docker ----

up: ## Start infra (Postgres + Formance) via Docker Compose
	docker compose -f deploy/docker-compose.yml up -d

down: ## Stop Docker Compose stack
	docker compose -f deploy/docker-compose.yml down -v --remove-orphans

logs: ## Tail logs from all containers
	docker compose -f deploy/docker-compose.yml logs -f

docker-build: ## Build all Docker images
	docker build -f deploy/Dockerfile --target api -t neo-api .
	docker build -f deploy/Dockerfile --target ethswitch-gateway -t neo-ethswitch-gateway .
	docker build -f deploy/Dockerfile --target recon-worker -t neo-recon-worker .
	docker build -f deploy/Dockerfile --target lending-worker -t neo-lending-worker .
	docker build -f deploy/Dockerfile --target telegram-bot -t neo-telegram-bot .

# ---- Development ----

api: ## Run the API server locally (reads .env.local)
	go run ./cmd/api

run-ui: ## Run the Next.js UI dev server (foreground)
	cd $(UI_DIR) && npm run dev

dev: ## Start API + UI dev servers in parallel (Ctrl-C stops both)
	@echo "Starting API on :8080 and UI on :3000..."
	@rm -f $(UI_DIR)/.next/dev/lock 2>/dev/null || true
	@trap 'kill 0' INT TERM; \
	( go run ./cmd/api ) & \
	( cd $(UI_DIR) && npm run dev ) & \
	wait

dev-stop: ## Kill processes on ports 8080, 3000, and 3001
	@lsof -ti :8080 | xargs kill -9 2>/dev/null || true
	@lsof -ti :3000 | xargs kill -9 2>/dev/null || true
	@lsof -ti :3001 | xargs kill -9 2>/dev/null || true
	@echo "✓ Stopped processes on 8080, 3000, and 3001"

dev-fresh: dev-stop ## Clean caches and start dev (use when npm run dev fails)
	@rm -rf $(UI_DIR)/.next/dev/lock $(UI_DIR)/.next/cache 2>/dev/null || true
	@echo "✓ Cleared Next.js lock and Turbopack cache"
	@$(MAKE) dev

dev-api: api ## Alias for run-api

dev-ui: run-ui ## Alias for run-ui

run-admin: ## Run the admin dashboard on :3001
	cd $(UI_DIR) && npm run dev:admin

dev-admin: ## Start API + Admin UI in parallel (API :8080, Admin :3001)
	@echo "Starting API on :8080 and Admin UI on :3001..."
	@rm -f $(UI_DIR)/.next/dev/lock 2>/dev/null || true
	@trap 'kill 0' INT TERM; \
	( go run ./cmd/api ) & \
	( cd $(UI_DIR) && npm run dev:admin ) & \
	wait

# ---- UI Shortcuts ----

ui-build: ## Build the Next.js UI for production
	cd $(UI_DIR) && npm run build

ui-lint: ## Lint the Next.js UI
	cd $(UI_DIR) && npm run lint

ui-install: ## Install UI dependencies
	cd $(UI_DIR) && npm install

# ---- Admin ----

setup:
	make seed-admin EMAIL=admin@neo.et PASSWORD=password
	make seed-rates
	make seed-funds PHONE=+251960598761 AMOUNT=12000
	make seed-loan-capital AMOUNT=1000000
	make seed-overdraft-capital AMOUNT=200000
	make seed-credit-profile USERNAME=vonmutinda TRUST_SCORE=700 LIMIT_CENTS=5000000

# make seed-admin EMAIL=admin@neo.et PASSWORD=password
seed-admin: ## Seed a super_admin staff account (EMAIL=... PASSWORD=...)
	@if [ -z "$(EMAIL)" ] || [ -z "$(PASSWORD)" ]; then echo "Usage: make seed-admin EMAIL=admin@neo.et PASSWORD=secret"; exit 1; fi
	@HASH=$$(go run -C . ./cmd/seed-admin "$(PASSWORD)"); \
	docker exec neo-postgres psql -U neobank -d neobank -c "INSERT INTO staff (email, full_name, role, department, password_hash) VALUES ('$(EMAIL)', 'Super Admin', 'super_admin', 'Executive', '$$HASH') ON CONFLICT (email) DO UPDATE SET password_hash = EXCLUDED.password_hash;"
	@echo "Super admin seeded: $(EMAIL)"

# make seed-funds PHONE=+251960598761 AMOUNT=10000
seed-funds: ## Seed ETB to a user's primary wallet (PHONE=+251... AMOUNT=100)
	@if [ -z "$(PHONE)" ] || [ -z "$(AMOUNT)" ]; then echo "Usage: make seed-funds PHONE=+251912345678 AMOUNT=100000"; exit 1; fi
	@WALLET_ID=$$(docker exec neo-postgres psql -U neobank -t -A -c "SELECT ledger_wallet_id FROM users WHERE '+' || country_code || number = '$(PHONE)';" | tr -d ' ' | grep -v '^$$'); \
	if [ -z "$$WALLET_ID" ]; then echo "User with phone $(PHONE) not found."; exit 1; fi; \
	SANITIZED_WALLET=$$(echo "$$WALLET_ID" | tr -d '-'); \
	CENTS=$$(($(AMOUNT) * 100)); \
	curl -s -X POST "$(FORMANCE_URL)/v2/neobank/transactions" -H "Content-Type: application/json" -d '{ "postings": [ { "amount": '$$CENTS', "asset": "ETB/2", "destination": "neo:wallets:'$$SANITIZED_WALLET':main", "source": "world" } ], "reference": "seed_'$$CENTS'_to_'$$SANITIZED_WALLET'" }' > /dev/null; \
	echo "Seeded $(AMOUNT) ETB to user $(PHONE) (Wallet: $$WALLET_ID)"

seed-loan-capital: ## Seed ETB into the loan capital pool (AMOUNT=1000000 default)
	@AMOUNT=$${AMOUNT:-1000000}; \
	CENTS=$$(($$AMOUNT * 100)); \
	curl -s -X POST "$(FORMANCE_URL)/v2/neobank/transactions" -H "Content-Type: application/json" \
		-d '{ "postings": [{ "amount": '$$CENTS', "asset": "ETB/2", "destination": "neo:system:loan_capital", "source": "world" }], "reference": "seed_loan_capital_'$$CENTS'" }' > /dev/null; \
	echo "Seeded $$AMOUNT ETB into neo:system:loan_capital"

seed-overdraft-capital: ## Seed ETB into the overdraft capital pool (AMOUNT=1000000 default)
	@AMOUNT=$${AMOUNT:-1000000}; \
	CENTS=$$(($$AMOUNT * 100)); \
	curl -s -X POST "$(FORMANCE_URL)/v2/neobank/transactions" -H "Content-Type: application/json" \
		-d '{ "postings": [{ "amount": '$$CENTS', "asset": "ETB/2", "destination": "neo:system:overdraft_capital", "source": "world" }], "reference": "seed_overdraft_capital_'$$CENTS'" }' > /dev/null; \
	echo "Seeded $$AMOUNT ETB into neo:system:overdraft_capital"

# make seed-credit-profile USERNAME=vonmutinda
# make seed-credit-profile USERNAME=alice TRUST_SCORE=700 LIMIT_CENTS=5000000
seed-credit-profile: ## Seed credit profile by username (USERNAME=... TRUST_SCORE=650 LIMIT_CENTS=1000000)
	@if [ -z "$(USERNAME)" ]; then echo "Usage: make seed-credit-profile USERNAME=vonmutinda [TRUST_SCORE=650] [LIMIT_CENTS=1000000]"; exit 1; fi
	@TRUST_SCORE=$${TRUST_SCORE:-650}; LIMIT_CENTS=$${LIMIT_CENTS:-1000000}; \
	USER_ID=$$(docker exec neo-postgres psql -U neobank -d neobank -t -A -c "SELECT id FROM users WHERE username = '$(USERNAME)';" | tr -d ' \n' | grep -v '^$$'); \
	if [ -z "$$USER_ID" ]; then echo "User with username $(USERNAME) not found."; exit 1; fi; \
	docker exec neo-postgres psql -U neobank -d neobank -c "INSERT INTO credit_profiles (user_id, trust_score, approved_limit_cents, last_calculated_at) VALUES ('$$USER_ID', $$TRUST_SCORE, $$LIMIT_CENTS, NOW()) ON CONFLICT (user_id) DO UPDATE SET trust_score = EXCLUDED.trust_score, approved_limit_cents = EXCLUDED.approved_limit_cents, last_calculated_at = NOW(), updated_at = NOW();" > /dev/null; \
	echo "Seeded credit profile for $(USERNAME): trust_score=$$TRUST_SCORE, approved_limit_cents=$$LIMIT_CENTS"

seed-rates: ## Seed dev FX rates for USD/ETB/EUR (6 cross-rate pairs, 1.5% spread)
	@docker exec neo-postgres psql -U neobank -c " \
		INSERT INTO fx_rates (from_currency, to_currency, mid_rate, bid_rate, ask_rate, spread_percent, source, fetched_at) VALUES \
		('USD', 'ETB', 57.50,   57.069, 57.931, 1.5, 'manual', NOW()), \
		('ETB', 'USD', 0.01739, 0.01726, 0.01752, 1.5, 'manual', NOW()), \
		('EUR', 'USD', 1.08,    1.0719, 1.0881, 1.5, 'manual', NOW()), \
		('USD', 'EUR', 0.9259,  0.9190, 0.9328, 1.5, 'manual', NOW()), \
		('EUR', 'ETB', 62.10,   61.634, 62.566, 1.5, 'manual', NOW()), \
		('ETB', 'EUR', 0.01610, 0.01598, 0.01622, 1.5, 'manual', NOW()) \
		;" > /dev/null
	@echo "Seeded 6 FX rate pairs (USD/ETB/EUR) with 1.5% spread"

# ---- Cleanup ----

clean: ## Remove build artifacts
	rm -rf bin/ coverage.out

# ---- Help ----

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
