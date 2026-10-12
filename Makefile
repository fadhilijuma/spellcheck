# ==============================================================================
# Configuration
#
# Requires Go 1.27.2+, Node 22.12+ and Make. Race tests need a C compiler.
# Override BUILD to identify a binary, or COMPOSE to use Docker Compose.

BUILD   ?= develop
COMPOSE ?= podman-compose

.DEFAULT_GOAL := help
.PHONY: help setup run dev fmt lint build check test up down logs

# ==============================================================================
# Install dependencies
#
# Go tools are pinned in go.mod; npm dependencies use the lockfile.
# Browser installation is separate; see README Verification.

setup:
	go mod download
	npm --prefix web ci

# ==============================================================================
# Running the project
#
# Start the API on :3000, then the editor on :3001 in a second terminal.
# The editor's development server proxies /v1 to the API.

run:
	go run ./cmd/spellcheck

dev:
	npm --prefix web run dev

# ==============================================================================
# Building the project
#
# Outputs: ./spellcheck and web/dist/.
# Example: make build BUILD=release-1

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.tag=$(BUILD)" -o spellcheck ./cmd/spellcheck
	npm --prefix web run build

# ==============================================================================
# Checks and tests
#
# check runs lint, builds and all tests. test adds vulnerability scans.
# Browser tests start the real API/editor on :3015/:3016; keep those ports free.
# Vulnerability scans need network access.

fmt:
	gofmt -s -w cmd internal

lint:
	@out=$$(gofmt -s -l cmd internal); \
	if [ -n "$$out" ]; then printf 'Run make fmt:\n%s\n' "$$out"; exit 1; fi
	go vet ./...
	go tool staticcheck -checks=all ./...

check: lint build
	CGO_ENABLED=1 go test -race -count=1 ./...
	npm --prefix web test
	npm --prefix web run test:browser

test: check
	go tool govulncheck ./...
	npm --prefix web audit

# ==============================================================================
# Containers
#
# The editor is published on :8081; the API stays on the private network.
# Examples: WEB_PORT=8083 make up
#           make up COMPOSE="docker compose"

up:
	$(COMPOSE) up --build -d

down:
	$(COMPOSE) down

logs:
	$(COMPOSE) logs --tail 100

# ==============================================================================
# Help

help:
	@printf '%s\n' \
		'Usage: make <command>' \
		'' \
		'  setup      Install Go and npm dependencies' \
		'  run        Start the API on :3000' \
		'  dev        Start the editor on :3001' \
		'  build      Build the API and editor' \
		'  fmt        Format Go code' \
		'  lint       Check formatting, vet and staticcheck' \
		'  check      Lint, build and run all tests' \
		'  test       Run all checks and vulnerability scans' \
		'  up         Build and start containers on :8081' \
		'  down       Stop and remove containers' \
		'  logs       Show container logs' \
		'  help       List commands'
