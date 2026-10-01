GO := $(shell command -v go)

BINARY         := cmd/gophermart/gophermart
ACCRUAL_BINARY := cmd/accrual/accrual_linux_amd64

# Not 8080: that port is too often taken on a developer machine.
RUN_PORT     ?= 8090
ACCRUAL_PORT ?= 8091
COVER_FILE   ?= coverage.out

PG_CONTAINER ?= gophermart-pg
PG_IMAGE     ?= postgres:16-alpine
PG_PORT      ?= 5434
DATABASE_URI ?= postgres://postgres:postgres@localhost:$(PG_PORT)/praktikum?sslmode=disable

.DEFAULT_GOAL := help
.PHONY: help build run accrual test test-db cover vet fmt autotest db-up db-down clean

help: ## Show this help
	@grep -E '^[a-z-]+:.*?## ' $(MAKEFILE_LIST) | awk -F':.*?## ' '{printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'

build: ## Build the gophermart binary
	$(GO) build -o $(BINARY) ./cmd/gophermart

run: build db-up ## Run gophermart on RUN_PORT against the local postgres and `make accrual`
	$(BINARY) -a localhost:$(RUN_PORT) -d '$(DATABASE_URI)' -r http://localhost:$(ACCRUAL_PORT)

accrual: ## Run the accrual system from the template on ACCRUAL_PORT
	$(ACCRUAL_BINARY) -a localhost:$(ACCRUAL_PORT)

test: ## Run unit tests under the race detector (database tests are skipped)
	$(GO) test -race ./...

# Packages with database tests empty the same tables, so they run one by one.
test-db: db-up ## Run all tests under the race detector, including the ones that need postgres
	TEST_DATABASE_DSN='$(DATABASE_URI)' $(GO) test -race -p 1 ./...

cover: db-up ## Print the total coverage of all packages, database tests included
	TEST_DATABASE_DSN='$(DATABASE_URI)' $(GO) test -p 1 -coverpkg=./... -coverprofile=$(COVER_FILE) ./... >/dev/null
	$(GO) tool cover -func=$(COVER_FILE) | tail -1

vet: ## Run go vet
	$(GO) vet ./...

fmt: ## Format the source tree
	$(GO) fmt ./...

autotest: build db-up ## Run the gophermarttest autotests, as CI does
	gophermarttest -test.v -test.run='^TestGophermart$$' \
		-gophermart-binary-path=$(BINARY) \
		-gophermart-host=localhost \
		-gophermart-port=$(RUN_PORT) \
		-gophermart-database-uri='$(DATABASE_URI)' \
		-accrual-binary-path=$(ACCRUAL_BINARY) \
		-accrual-host=localhost \
		-accrual-port=$(ACCRUAL_PORT) \
		-accrual-database-uri='$(DATABASE_URI)'

# pg_isready goes over TCP: while the image initialises the database, it runs
# a temporary server that listens on the unix socket only.
db-up: ## Start the local postgres
	@docker inspect -f '{{.State.Running}}' $(PG_CONTAINER) 2>/dev/null | grep -q true \
		|| docker run -d --rm --name $(PG_CONTAINER) \
			-e POSTGRES_USER=postgres \
			-e POSTGRES_PASSWORD=postgres \
			-e POSTGRES_DB=praktikum \
			-p $(PG_PORT):5432 $(PG_IMAGE)
	@until docker exec $(PG_CONTAINER) pg_isready -h localhost -U postgres >/dev/null 2>&1; do sleep 1; done
	@echo "postgres ready on localhost:$(PG_PORT)"

db-down: ## Stop the local postgres
	-docker rm -f $(PG_CONTAINER)

clean: ## Remove the binary and the coverage profile
	rm -f $(BINARY) $(COVER_FILE)
