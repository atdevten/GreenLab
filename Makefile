.PHONY: all build test lint tidy up down generate-keys mock migrate-all migrate-up migrate-down swagger

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

lint:
	@for svc in $(SERVICES); do \
		echo "==> Linting $$svc"; \
		(cd services/$$svc && golangci-lint run); \
	done

up:
	docker compose up -d

down:
	docker compose down

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
		-database "postgres://greenlab:greenlab@localhost:5432/$$db?sslmode=disable" up

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
		-database "postgres://greenlab:greenlab@localhost:5432/$$db?sslmode=disable" down 1

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
