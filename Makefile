.PHONY: setup run demo acquire test test-acquire lint build clean

# Listed explicitly: a bare ./... from the repo root also picks up Go files inside node_modules.
GO_PKGS = ./backend/...

# ARGS passes extra server flags, e.g. make run ARGS="-weather-url http://127.0.0.1:9/"
SERVE = cd backend && go run ./cmd/server -seed ../data/seed.json -db ../data/control.db -static ../frontend/dist $(ARGS)

# The local database holds only rows loaded from the seed, so setup always
# rebuilds it: one command converges whatever state it was in.
setup:
	@test -f data/seed.json || { echo "data/seed.json is missing. It is committed: restore it with 'git checkout data/seed.json' or rebuild it with 'make acquire'."; exit 1; }
	@echo "→ fetching Go dependencies"
	@go mod download
	@echo "→ installing frontend dependencies"
	@cd frontend && npm ci
	@echo "→ building frontend"
	@cd frontend && npm run build
	@echo "→ building local SQLite from the seed"
	@rm -f data/control.db data/control.db-shm data/control.db-wal
	@cd backend && go run ./cmd/server -init-only -seed ../data/seed.json -db ../data/control.db
	@echo "✓ setup complete — run 'make run'"

run:
	@$(SERVE)

# Show the action state on a dry day. Everything is real except the weather
# feed, which comes from a local simulator; the API and the UI label it.
demo:
	@python3 scripts/sim_weather.py --precip 0.6 & SIM=$$!; \
	trap 'kill $$SIM 2>/dev/null' EXIT INT TERM; \
	$(SERVE) -weather-url http://127.0.0.1:9099/

# Rebuild data/seed.json from the Chicago data portal. Maintainers only: the
# seed is committed. Resumable, so rerun it until it prints PASS.
acquire:
	python3 scripts/acquire.py

test-acquire:
	PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s scripts -p 'test_*.py'

test: test-acquire
	go test $(GO_PKGS) -count=1
	go vet $(GO_PKGS)
	cd frontend && npm run lint
	cd frontend && npm run typecheck
	cd frontend && npm test

lint:
	go vet $(GO_PKGS)
	cd frontend && npm run lint

build:
	go build $(GO_PKGS)
	cd frontend && npm run build

clean:
	rm -f data/control.db data/control.db-shm data/control.db-wal
	rm -rf frontend/dist frontend/node_modules
