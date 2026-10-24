.DEFAULT_GOAL := help

GO ?= go
COMPOSE ?= docker compose
BIN_DIR ?= $(CURDIR)/bin
TOOLS_DIR ?= $(CURDIR)/.tools
GOOSE ?= $(TOOLS_DIR)/bin/goose
NEWMAN ?= $(TOOLS_DIR)/node_modules/.bin/newman
SERVICE ?= gateway
BASE_URL ?= http://localhost:8080
LOG_SERVICE ?=
WAIT_TIMEOUT ?= 120

SERVICES := gateway reservation payment loyalty
GOOSE_VERSION := v3.26.0
NEWMAN_VERSION := 6.2.1
GOOSE_TAGS := no_clickhouse no_libsql no_mssql no_mysql no_sqlite3 no_vertica no_ydb

.PHONY: help fmt fmt-check vet test test-race check build clean run tools \
        compose-check images up down db logs ps health migrate migrate-status \
        test-integration test-api test-e2e test-system require-service require-db-service

help: ## Список основных команд
	@awk 'BEGIN {FS = ":.*## "; print "Команды (make <имя>):"} /^[a-zA-Z0-9_-]+:.*## / {printf "  %-19s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

fmt: ## Форматировать Go-код
	gofmt -w cmd internal tests

fmt-check: ## Проверить форматирование без изменения файлов
	@set -e; files="$$(gofmt -l cmd internal tests)"; \
	if [ -n "$$files" ]; then printf '%s\n' "$$files"; exit 1; fi

vet: ## Статический анализ
	$(GO) vet ./...

test: ## Go-тесты; PostgreSQL/E2E требуют переменных окружения
	$(GO) test -count=1 ./...

test-race: ## Go-тесты с детектором гонок
	$(GO) test -race -count=1 ./...

check: fmt-check vet test-race build ## Форматирование, vet, race-тесты и сборка

build: ## Собрать четыре исполняемых файла в bin/
	@mkdir -p "$(BIN_DIR)"
	@set -e; for service in $(SERVICES); do \
		$(GO) build -o "$(BIN_DIR)/$$service" "./cmd/$$service"; \
	done

clean: ## Удалить только собранные бинарники; базы сохраняются
	@set -e; for service in $(SERVICES); do rm -f "$(BIN_DIR)/$$service"; done

run: require-service ## Запустить один сервис: make run SERVICE=gateway
	$(GO) run ./cmd/$(SERVICE)

tools: ## Установить закрепленные Goose и Newman локально в .tools/
	@mkdir -p "$(TOOLS_DIR)/bin"
	GOBIN="$(TOOLS_DIR)/bin" $(GO) install -tags='$(GOOSE_TAGS)' github.com/pressly/goose/v3/cmd/goose@$(GOOSE_VERSION)
	npm install --prefix "$(TOOLS_DIR)" --no-save --no-package-lock newman@$(NEWMAN_VERSION)

compose-check: ## Проверить конфигурацию Docker Compose
	$(COMPOSE) config --quiet

images: ## Собрать все Docker-образы
	$(COMPOSE) build

up: ## Собрать и поднять систему, дождаться готовности
	$(COMPOSE) up -d --build --wait --wait-timeout $(WAIT_TIMEOUT)

down: ## Остановить Compose, сохранив данные PostgreSQL
	$(COMPOSE) down

db: ## Поднять только PostgreSQL из Compose
	$(COMPOSE) up -d --wait --wait-timeout $(WAIT_TIMEOUT) postgres

logs: ## Смотреть логи; опционально LOG_SERVICE=gateway
	$(COMPOSE) logs --follow $(LOG_SERVICE)

ps: ## Состояние контейнеров
	$(COMPOSE) ps

health: ## Проверить готовность четырех сервисов на localhost
	@set -e; for port in 8080 8070 8060 8050; do \
		printf 'localhost:%s ' "$$port"; \
		curl --fail --silent --show-error --max-time 8 "http://localhost:$$port/manage/health"; \
	done

migrate: require-db-service ## Применить миграции: SERVICE=payment DATABASE_URL=...
	@test -n "$$DATABASE_URL" || { echo 'Задайте DATABASE_URL собственной базы сервиса.' >&2; exit 1; }
	"$(GOOSE)" -dir migrations/$(SERVICE) postgres "$$DATABASE_URL" up

migrate-status: require-db-service ## Посмотреть версии миграций SERVICE=payment DATABASE_URL=...
	@test -n "$$DATABASE_URL" || { echo 'Задайте DATABASE_URL собственной базы сервиса.' >&2; exit 1; }
	"$(GOOSE)" -dir migrations/$(SERVICE) postgres "$$DATABASE_URL" status

test-integration: ## SQL-тесты; нужны три *_TEST_DATABASE_URL и мигрированные базы *_test
	@test -n "$$RESERVATION_TEST_DATABASE_URL" && test -n "$$PAYMENT_TEST_DATABASE_URL" && test -n "$$LOYALTY_TEST_DATABASE_URL" || \
		{ echo 'Задайте RESERVATION_TEST_DATABASE_URL, PAYMENT_TEST_DATABASE_URL и LOYALTY_TEST_DATABASE_URL.' >&2; exit 1; }
	$(GO) test -race -count=1 ./internal/reservation/adapters/postgres ./internal/payment/adapters/postgres ./internal/loyalty/adapters/postgres

test-api: ## Исходная приемочная коллекция Postman на работающей системе
	"$(NEWMAN)" run v2/postman/collection.json -e v2/postman/environment.json --env-var "baseUrl=$(BASE_URL)"

test-e2e: ## Дополнительные системные тесты на работающей тестовой системе
	SYSTEM_TEST_URL="$(BASE_URL)" $(GO) test -race -count=1 -v ./tests/system

test-system: ## Полный нативный прогон с временным PostgreSQL и четырьмя сервисами
	GOOSE_BIN="$(GOOSE)" NEWMAN_BIN="$(NEWMAN)" ./scripts/test-system.sh

require-service:
	@case "$(SERVICE)" in gateway|reservation|payment|loyalty) ;; \
		*) echo 'SERVICE должен быть gateway, reservation, payment или loyalty.' >&2; exit 1 ;; esac

require-db-service:
	@case "$(SERVICE)" in reservation|payment|loyalty) ;; \
		*) echo 'Для миграций задайте SERVICE=reservation, payment или loyalty.' >&2; exit 1 ;; esac
