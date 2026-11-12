.DEFAULT_GOAL := help

SHELL := /bin/sh
DOCKER_COMPOSE ?= docker compose

BACKEND_DIR := backend
FRONTEND_DIR := frontend

.PHONY: help check-docker-cli check-docker doctor \
        run up start down stop restart ps logs backend-logs db-logs config reset-data \
        backend-test frontend-install frontend-typecheck frontend-lint frontend-test frontend-build frontend-check test verify

help: ## Показать список команд
	@printf '\nДоступные команды:\n\n'
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z0-9_-]+:.*##/ {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)
	@printf '\nОсновной сценарий для проверяющего: make run\n'

check-docker-cli: ## Проверить наличие Docker CLI и Compose plugin
	@command -v docker >/dev/null 2>&1 || { \
		echo 'Ошибка: Docker не найден. Установите Docker Desktop (macOS/Windows) или Docker Engine + Compose plugin (Linux).'; \
		exit 1; \
	}
	@docker compose version >/dev/null 2>&1 || { \
		echo 'Ошибка: Docker Compose plugin не найден. Нужен Docker Compose v2.'; \
		exit 1; \
	}

check-docker: check-docker-cli ## Проверить, что Docker daemon запущен
	@docker info >/dev/null 2>&1 || { \
		echo 'Ошибка: Docker установлен, но daemon не запущен. Запустите Docker Desktop или Docker Engine.'; \
		exit 1; \
	}

doctor: check-docker ## Показать версии Docker и проверить готовность окружения
	@docker --version
	@docker compose version
	@echo 'Docker daemon доступен.'

run: up ## Собрать, запустить приложение, дождаться frontend и открыть сайт в браузере
	@set -eu; \
	command -v curl >/dev/null 2>&1 || { \
		echo 'Ошибка: для проверки готовности сайта нужна команда curl.'; \
		exit 1; \
	}; \
	published="$$($(DOCKER_COMPOSE) port frontend 80 2>/dev/null | tail -n 1)"; \
	[ -n "$$published" ] || { \
		echo 'Ошибка: не удалось определить опубликованный порт frontend.'; \
		echo 'Проверьте, что в docker-compose.yml frontend публикует порт контейнера 80.'; \
		exit 1; \
	}; \
	port="$${published##*:}"; \
	url="http://localhost:$$port"; \
	echo "Ожидание готовности frontend: $$url"; \
	attempt=0; \
	until curl --silent --fail --output /dev/null --max-time 3 "$$url"; do \
		attempt=$$((attempt + 1)); \
		if [ "$$attempt" -ge 60 ]; then \
			echo 'Ошибка: frontend не ответил за 60 секунд. Последние логи:'; \
			$(DOCKER_COMPOSE) logs --tail=80 frontend; \
			exit 1; \
		fi; \
		sleep 1; \
	done; \
	echo 'Frontend готов. Открываю браузер...'; \
	os="$$(uname -s)"; \
	case "$$os" in \
		Darwin*) open "$$url" ;; \
		Linux*) \
			if command -v xdg-open >/dev/null 2>&1; then \
				xdg-open "$$url" >/dev/null 2>&1 & \
			elif command -v gio >/dev/null 2>&1; then \
				gio open "$$url" >/dev/null 2>&1 & \
			else \
				echo "Не удалось автоматически открыть браузер. Перейдите по адресу: $$url"; \
			fi ;; \
		MINGW*|MSYS*|CYGWIN*) cmd.exe /c start "" "$$url" ;; \
		*) echo "Автоматическое открытие браузера не поддерживается. Перейдите по адресу: $$url" ;; \
	esac
up: check-docker ## Собрать и запустить приложение в фоне
	@$(DOCKER_COMPOSE) up --build -d
	@printf '\nПриложение запущено (адреса ниже — стандартные, если порты не переопределены в .env).\nFrontend: http://localhost:3000\nBackend:  http://localhost:8080\nSwagger:  http://localhost:8080/docs/\n\nСтатус: make ps\nЛоги:   make logs\nОстановка: make down\n'

start: up ## Алиас для make up

down: check-docker ## Остановить приложение и удалить контейнеры (данные БД сохраняются)
	@$(DOCKER_COMPOSE) down

stop: check-docker ## Остановить контейнеры без их удаления
	@$(DOCKER_COMPOSE) stop

restart: check-docker ## Перезапустить уже созданные контейнеры без пересборки
	@$(DOCKER_COMPOSE) restart

ps: check-docker-cli ## Показать статус контейнеров
	@$(DOCKER_COMPOSE) ps

logs: check-docker-cli ## Следить за логами всех сервисов
	@$(DOCKER_COMPOSE) logs -f --tail=100

backend-logs: check-docker-cli ## Следить за логами backend
	@$(DOCKER_COMPOSE) logs -f --tail=100 backend

db-logs: check-docker-cli ## Следить за логами PostgreSQL
	@$(DOCKER_COMPOSE) logs -f --tail=100 db

config: check-docker-cli ## Проверить синтаксис docker-compose.yml и итоговую конфигурацию
	@$(DOCKER_COMPOSE) config -q
	@echo 'Docker Compose configuration is valid.'

reset-data: check-docker ## Удалить контейнеры и volume PostgreSQL (ВСЕ сохранённые связи будут удалены)
	@printf 'ВНИМАНИЕ: будут удалены все сохранённые связи в PostgreSQL.\nДля подтверждения введите DELETE: '; \
	read answer; \
	[ "$$answer" = 'DELETE' ] || { echo 'Отменено.'; exit 1; }
	@$(DOCKER_COMPOSE) down -v

backend-test: ## Запустить тесты Go backend (требуется Go)
	@command -v go >/dev/null 2>&1 || { echo 'Ошибка: Go не найден.'; exit 1; }
	@cd $(BACKEND_DIR) && go test ./...

frontend-install: ## Установить frontend-зависимости строго по package-lock.json (требуется Node.js и npm)
	@command -v npm >/dev/null 2>&1 || { echo 'Ошибка: npm не найден. Установите Node.js подходящей версии.'; exit 1; }
	@cd $(FRONTEND_DIR) && npm ci

frontend-typecheck: ## Проверить типы TypeScript (требуется npm ci)
	@cd $(FRONTEND_DIR) && npm run typecheck

frontend-lint: ## Запустить ESLint (требуется npm ci)
	@cd $(FRONTEND_DIR) && npm run lint

frontend-test: ## Запустить тесты frontend (требуется npm ci)
	@cd $(FRONTEND_DIR) && npm run test

frontend-build: ## Собрать production frontend (требуется npm ci)
	@cd $(FRONTEND_DIR) && npm run build

frontend-check: frontend-install frontend-typecheck frontend-lint frontend-test frontend-build ## Установить зависимости и выполнить все проверки frontend

test: backend-test frontend-check ## Запустить тесты backend и полный набор проверок frontend

verify: config backend-test frontend-check ## Проверить Compose-конфигурацию, backend и frontend
