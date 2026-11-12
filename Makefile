.PHONY: setup run acquire acquire-monthly bootstrap-dev-seed test-acquire test lint build clean

setup:
	@echo "→ fetching Go dependencies"
	@cd backend && go mod download
	@echo "→ installing frontend dependencies"
	@cd frontend && npm ci
	@echo "→ building frontend"
	@cd frontend && npm run build
	@echo "→ ensuring data/ seed exists"
	@test -f data/seed.json || $(MAKE) bootstrap-dev-seed
	@echo "→ initializing local SQLite"
	@cd backend && go run ./cmd/server -init-only -seed ../data/seed.json -db ../data/control.db
	@echo "✓ setup complete — run 'make run'"

run:
	@cd backend && go run ./cmd/server -seed ../data/seed.json -db ../data/control.db -static ../frontend/dist

# Full real-source acquisition (bounded 60s/10MB). Requires Chicago data portal
# to respond to grouped queries. See scripts/acquire.py.
acquire:
	python3 scripts/acquire.py

acquire-monthly:
	python3 scripts/acquire.py --monthly

# Deterministic development baseline (labelled synthetic in UI). Used when
# `make acquire` cannot reach the Chicago data portal.
bootstrap-dev-seed:
	python3 scripts/bootstrap_dev_seed.py

test-acquire:
	PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s scripts -p 'test_*.py' -v

test:
	cd backend && go test ./... -count=1
	cd backend && go vet ./...
	cd frontend && npm run lint

lint:
	cd backend && go vet ./...
	cd frontend && npm run lint

build:
	cd backend && go build ./...
	cd frontend && npm run build

clean:
	rm -f data/control.db
	rm -rf frontend/dist frontend/node_modules
