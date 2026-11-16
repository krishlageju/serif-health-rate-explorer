.PHONY: install dev backend frontend lint format test build start clean-cache

install: ## Install Go and npm dependencies
	cd backend && go mod download
	cd frontend && npm install

dev: ## Run backend (:8080) and Vite dev server (:5173) together
	@trap 'kill 0' INT TERM EXIT; \
	(cd backend && STATIC_DIR=none go run ./cmd/server) & \
	(cd frontend && npm run dev) & \
	wait

backend: ## Run only the Go API on :8080
	cd backend && go run ./cmd/server

frontend: ## Run only the Vite dev server on :5173 (proxies /api to :8080)
	cd frontend && npm run dev

lint: ## Check formatting and lint both projects (same checks as CI)
	cd backend && test -z "$$(gofmt -l .)" || (gofmt -l . && exit 1)
	cd backend && go vet ./...
	cd frontend && npm run format:check && npm run lint

format: ## Auto-format both projects
	cd backend && gofmt -w .
	cd frontend && npm run format

test: ## Run Go and frontend tests
	cd backend && go vet ./... && go test -race ./...
	cd frontend && npm run typecheck && npm test

build: ## Build the frontend and a single Go binary
	cd frontend && npm run build
	cd backend && go build -o bin/server ./cmd/server

start: build ## Build, then serve API + UI from one process on :8080
	cd backend && ./bin/server

clean-cache: ## Delete downloaded rate files
	rm -rf backend/data/cache
