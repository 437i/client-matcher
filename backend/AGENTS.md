# Backend — инструкции Go-агенту

## Технологии и структура

- Module: `github.com/437i/client-matcher/backend`.
- Go: `1.26.1` в `backend/go.mod`; не меняй версию без отдельной причины.
- HTTP: Gin.
- PostgreSQL: pgx/v5; миграции — Goose SQL.
- CSV: стандартный `encoding/csv` и общий адаптер `internal/adapters/outbound/csvutil`.
- UUID: `github.com/google/uuid`.

Основные пакеты:

- `cmd/server` — composition root, конфигурация зависимостей и запуск.
- `internal/domain` — сущности, ошибки предметной области и правила сопоставления.
- `internal/application/ports` — интерфейсы исходящих зависимостей.
- `internal/application/services` — сценарии приложения.
- `internal/adapters/inbound/http` — Gin router и handlers.
- `internal/adapters/outbound` — CSV/PostgreSQL и будущие внешние реализации.
- `internal/platform` — конфигурация и инфраструктурные подключения.
- `migrations` — только схема собственной БД приложения.

CRM читается из CSV локально или Bitrix24 в production. Оба outbound-адаптера работают через CRMReader и отделены от OD. Сайт читается из CSV или отдельной PostgreSQL. Доступны direct + csv, direct + postgres и ssh + postgres. [Адаптер Bitrix24](../docs/bitrix24-adapter.md) · [Руководство Stage 3](../docs/stage3-production.md)

## Фактические HTTP-маршруты на момент подготовки этих инструкций

Сверяй их с `internal/adapters/inbound/http/router.go`: код является источником истины.

- `GET /healthz`
- `GET /api/v1/crm-clients?q=...`
- `GET /api/v1/site-clients/:id`
- `POST /api/v1/link-assessments`
- `POST /api/v1/links`
- `GET /api/v1/links`
- `GET /api/v1/links/export.csv`

Не добавляй и не переименовывай маршруты в рамках задач документирования.

## Архитектурные правила

- Domain не импортирует Gin, pgx, CSV-парсеры, HTTP-клиенты и инфраструктуру.
- Application зависит от интерфейсов `ports`, а не от конкретных адаптеров.
- HTTP handlers отвечают за binding, вызов application service и HTTP-представление результата; бизнес-правила не размещай в handlers.
- Внешние источники CRM и сайта используются только для чтения.
- Собственная PostgreSQL хранит связи идентификаторов и аудит ручного подтверждения в той же строке.
- Не объединяй CRM/site repositories и repository связей в одну реализацию.
- Сохраняй DTO как отдельный HTTP-контракт; не отдавай Domain-сущности напрямую без явного решения.
- Не вводи ORM, Redis, Kafka, микросервисы или DI-контейнер без прямой необходимости.

## Ошибки и контракты

- Используй существующие sentinel errors и `errors.Is` там, где это принято в проекте.
- Оборачивай ошибки с контекстом через `%w`.
- HTTP-формат ошибок централизован в `internal/adapters/inbound/http/handlers/response.go`; не дублируй mapping в handlers.
- Не меняй JSON-поля, статусы, маршруты и Content-Type без явного требования.

## Проверки

Запускай из `backend/`:

```bash
go test ./...
go vet ./...
go build ./...
gofmt -w <изменённые .go файлы>
```

Из корня репозитория перед/после изменений полезны:

```bash
git status --short
git diff --check
docker compose config
```

Не запускай разрушающие Docker-команды и не удаляй volumes.
