.PHONY: all build test coverage lint tidy up down deploy generate-keys mock migrate-all migrate-up migrate-down swagger

SERVICES := iam device-registry ingestion normalization query-realtime alert-notification supporting

all: tidy build

tidy:
	@cd shared && go mod tidy
	@for svc in $(SERVICES); do \
		echo "==> Tidying $$svc"; \
		(cd services/$$svc && go mod tidy); \
	done

build:
	@for svc in $(SERVICES); do \
		echo "==> Building $$svc"; \
		(cd services/$$svc && go build ./...) || exit 1; \
	done

test:
	@for svc in $(SERVICES); do \
		echo "==> Testing $$svc"; \
		(cd services/$$svc && go test ./...); \
	done

coverage:
	@mkdir -p coverage
	@total_stmts=0; total_covered=0; \
	for svc in $(SERVICES); do \
		echo "==> Coverage $$svc"; \
		(cd services/$$svc && go test ./... -coverprofile=../../coverage/$$svc.out -covermode=atomic 2>/dev/null); \
		if [ -f coverage/$$svc.out ]; then \
			pct=$$(go tool cover -func=coverage/$$svc.out | tail -1 | awk '{print $$NF}'); \
			echo "    $$svc: $$pct"; \
		fi \
	done
	@echo ""
	@echo "==> HTML reports: coverage/<service>.html"
	@for svc in $(SERVICES); do \
		if [ -f coverage/$$svc.out ]; then \
			go tool cover -html=coverage/$$svc.out -o coverage/$$svc.html; \
		fi \
	done

coverage-%:
	@mkdir -p coverage
	cd services/$* && go test ./... -coverprofile=../../coverage/$*.out -covermode=atomic
	go tool cover -func=coverage/$*.out
	go tool cover -html=coverage/$*.out -o coverage/$*.html
	@echo "==> HTML report: coverage/$*.html"

lint:
	@for svc in $(SERVICES); do \
		echo "==> Linting $$svc"; \
		(cd services/$$svc && golangci-lint run); \
	done

up:
	docker compose up -d

down:
	docker compose down

# deploy brings up device-registry first, waits for its /health endpoint to return
# HTTP 200, then starts ingestion. This ordering is required because ingestion calls
# device-registry's /internal/validate-api-key on every write request; if ingestion
# starts first it will reject all writes with 503 until device-registry is live.
#
# Usage: make deploy
# Max wait per service: 30 × 2s = 60 seconds before timing out.
deploy:
	@echo "==> Starting device-registry..."
	docker compose up -d device-registry
	@echo "==> Waiting for device-registry to be healthy (http://localhost:8002/health)..."
	@retries=30; \
	while [ $$retries -gt 0 ]; do \
		if curl -sf http://localhost:8002/health > /dev/null 2>&1; then \
			echo "    device-registry is healthy"; break; \
		fi; \
		echo "    not ready yet ($$retries retries left)..."; \
		sleep 2; \
		retries=$$((retries - 1)); \
	done; \
	if [ $$retries -eq 0 ]; then echo "ERROR: device-registry did not become healthy in time"; exit 1; fi
	@echo "==> Starting ingestion..."
	docker compose up -d ingestion
	@echo "==> Waiting for ingestion to be healthy (http://localhost:8003/health)..."
	@retries=30; \
	while [ $$retries -gt 0 ]; do \
		if curl -sf http://localhost:8003/health > /dev/null 2>&1; then \
			echo "    ingestion is healthy"; break; \
		fi; \
		echo "    not ready yet ($$retries retries left)..."; \
		sleep 2; \
		retries=$$((retries - 1)); \
	done; \
	if [ $$retries -eq 0 ]; then echo "ERROR: ingestion did not become healthy in time"; exit 1; fi
	@echo "==> Deploy complete."

logs:
	docker compose logs -f

mock:
	@for svc in $(SERVICES); do \
		if [ -f services/$$svc/.mockery.yaml ]; then \
			echo "==> Generating mocks for $$svc"; \
			(cd services/$$svc && go run github.com/vektra/mockery/v2@latest --config=.mockery.yaml) || exit 1; \
		fi \
	done

# migrate-up/migrate-down apply or roll back migrations for a single service.
# Usage: make migrate-up service=iam
# Requires: github.com/golang-migrate/migrate/v4/cmd/migrate
#   Install: go install github.com/golang-migrate/migrate/v4/cmd/migrate@latest
migrate-all:
	@$(MAKE) migrate-up service=iam
	@$(MAKE) migrate-up service=device-registry
	@$(MAKE) migrate-up service=alert-notification
	@$(MAKE) migrate-up service=supporting

migrate-up:
	@if [ -z "$(service)" ]; then echo "Usage: make migrate-up service=<name>"; exit 1; fi; \
	case "$(service)" in \
		iam)                db=iam_db ;; \
		device-registry)    db=device_registry_db ;; \
		alert-notification) db=alert_db ;; \
		supporting)         db=supporting_db ;; \
		*) echo "No migrations for service '$(service)'. Valid: iam, device-registry, alert-notification, supporting"; exit 1 ;; \
	esac; \
	migrate -path services/$(service)/migrations \
		-database "postgres://greenlab:greenlab@localhost:5433/$$db?sslmode=disable" up

migrate-down:
	@if [ -z "$(service)" ]; then echo "Usage: make migrate-down service=<name>"; exit 1; fi; \
	case "$(service)" in \
		iam)                db=iam_db ;; \
		device-registry)    db=device_registry_db ;; \
		alert-notification) db=alert_db ;; \
		supporting)         db=supporting_db ;; \
		*) echo "No migrations for service '$(service)'. Valid: iam, device-registry, alert-notification, supporting"; exit 1 ;; \
	esac; \
	migrate -path services/$(service)/migrations \
		-database "postgres://greenlab:greenlab@localhost:5433/$$db?sslmode=disable" down 1

migrate-down-all:
	@$(MAKE) migrate-down-service service=supporting
	@$(MAKE) migrate-down-service service=alert-notification
	@$(MAKE) migrate-down-service service=device-registry
	@$(MAKE) migrate-down-service service=iam

migrate-down-service:
	@if [ -z "$(service)" ]; then echo "Usage: make migrate-down-service service=<name>"; exit 1; fi; \
	case "$(service)" in \
		iam)                db=iam_db ;; \
		device-registry)    db=device_registry_db ;; \
		alert-notification) db=alert_db ;; \
		supporting)         db=supporting_db ;; \
		*) echo "No migrations for service '$(service)'. Valid: iam, device-registry, alert-notification, supporting"; exit 1 ;; \
	esac; \
	migrate -path services/$(service)/migrations \
		-database "postgres://greenlab:greenlab@localhost:5433/$$db?sslmode=disable" down --all

# swagger regenerates Swagger docs for all services that have a docs/ directory.
# Requires: go install github.com/swaggo/swag/cmd/swag@latest
SWAGGER_SERVICES := iam device-registry ingestion query-realtime alert-notification supporting

swagger:
	@for svc in $(SWAGGER_SERVICES); do \
		echo "==> Generating swagger for $$svc"; \
		(cd services/$$svc && swag init -g cmd/server/main.go -o docs) || exit 1; \
	done

generate-keys:
	@mkdir -p keys
	openssl genpkey -algorithm RSA -out keys/private.pem -pkeyopt rsa_keygen_bits:4096
	openssl rsa -pubout -in keys/private.pem -out keys/public.pem
	@echo "Keys generated at keys/"

run-%:
	cd services/$* && go run ./cmd/server/main.go

mock-%:
	cd services/$* && go run github.com/vektra/mockery/v2@latest --config=.mockery.yaml

.PHONY: proto
proto:
	find . -name "*.proto" -exec protoc --go_out=. --go-grpc_out=. {} \;
