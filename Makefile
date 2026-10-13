# ==============================================================================
# Configuration
#
# Running requires Podman/Docker, Compose and Make.
# Host checks require Go 1.27.2+, Node 22.12+ and a C compiler for race tests.
# Override BUILD to identify a binary, or COMPOSE to use Docker Compose.

BUILD           ?= develop
COMPOSE         ?= podman-compose
WEB_PORT        ?= 8081
API_URL         ?= http://localhost:$(WEB_PORT)
REQUEST_TIMEOUT ?= 600

.DEFAULT_GOAL := help
.PHONY: help setup run ready live request-us request-uk request-invalid \
	tidy list deps-upgrade fmt lint build check test up down logs

# ==============================================================================
# Install dependencies
#
# Build both images; Go/npm dependencies are installed inside their build stages.
# No host Go or Node installation is needed to run the containers.

setup:
	$(COMPOSE) build

# ==============================================================================
# Running the project
#
# make run starts both; nginx proxies /v1 to app:3000.
# Set WEB_PORT to change the published editor port. make down stops both.

run:
	$(COMPOSE) up --build

# ==============================================================================
# Health checks
#
# Probe the API through the Compose editor port, not the private app:3000 port.
# Example: make ready live WEB_PORT=8083

ready:
	curl --fail -i $(API_URL)/v1/readiness

live:
	curl --fail -i $(API_URL)/v1/liveness

# ==============================================================================
# Hitting endpoints
#
# Calls default to Compose. For IntelliJ: API_URL=http://127.0.0.1:3005
# The timeout allows pauses at breakpoints; each call prints its JSON response.

REQUEST_US := {"words":["recieve","speling","quick"],"language":"en-US"}
REQUEST_UK := {"words":["colour","centre","organise"],"language":"en-GB"}
REQUEST_INVALID := {"words":["two words"],"language":"en-US"}

request-us:
	curl --fail-with-body -sS -i --max-time $(REQUEST_TIMEOUT) \
		-H 'Content-Type: application/json' -d '$(REQUEST_US)' $(API_URL)/v1/check

request-uk:
	curl --fail-with-body -sS -i --max-time $(REQUEST_TIMEOUT) \
		-H 'Content-Type: application/json' -d '$(REQUEST_UK)' $(API_URL)/v1/check

# Intentionally returns HTTP 400; curl prints the error body and exits with code 22.
request-invalid:
	curl --fail-with-body -sS -i --max-time $(REQUEST_TIMEOUT) \
		-H 'Content-Type: application/json' -d '$(REQUEST_INVALID)' $(API_URL)/v1/check

# ==============================================================================
# Go modules
#
# These maintenance commands require Go on the host.
# tidy and deps-upgrade update module files and regenerate vendor/.
# Review dependency changes and run make check after an upgrade.

tidy:
	go mod tidy
	go mod vendor

list:
	go list -mod=mod all

deps-upgrade:
	go get -u -v ./...
	go mod tidy
	go mod vendor

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
		'  setup      Build both container images and install their dependencies' \
		'  run        Start both with Compose; nginx proxies /v1 to the API' \
		'  ready      Check readiness through the Compose port (WEB_PORT)' \
		'  live       Check liveness through the Compose port (WEB_PORT)' \
		'  request-us Replay US spelling corrections and print the JSON response' \
		'  request-uk Replay British English and print the JSON response' \
		'  request-invalid Replay invalid input; HTTP 400 and nonzero exit expected' \
		'  tidy       Tidy Go modules and regenerate vendor/' \
		'  list       List Go packages, using modules rather than vendor/' \
		'  deps-upgrade Upgrade Go dependencies, tidy and regenerate vendor/' \
		'  build      Build the API and editor' \
		'  fmt        Format Go code' \
		'  lint       Check formatting, vet and staticcheck' \
		'  check      Lint, build and run all tests' \
		'  test       Run all checks and vulnerability scans' \
		'  up         Build and start containers on :8081' \
		'  down       Stop and remove containers' \
		'  logs       Show container logs' \
		'  help       List commands'
