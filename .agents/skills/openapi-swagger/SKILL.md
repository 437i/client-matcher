---
name: openapi-swagger
description: Спроектировать, добавить или проверить OpenAPI документацию и Swagger UI для существующего Gin API без изменения поведения backend.
compatibility: Codex, Claude Code, OpenCode
---

# OpenAPI and Swagger documentation for client-matcher

## Главная цель

Описать текущий HTTP-контракт backend и предоставить локальный Swagger UI для ручной проверки. Документация производна от фактического кода; она не является поводом менять поведение API.

## Перед выбором инструментария

1. Изучи `backend/internal/adapters/inbound/http/router.go`, все handlers, `handlers/dto.go`, `handlers/response.go`, health handler и HTTP tests.
2. Составь точную таблицу method/path/query/path params/body/responses/errors/content types.
3. Синтетические CSV проекта допустимы. Соблюдай rules/data-safety.md; не читай .env, credentials и секреты без необходимости.
4. Проверь актуальные официальные документы, версии библиотек, поддержку Go 1.26.1, Gin и OpenAPI 3.1. Не используй устаревший пример только потому, что он популярен.
5. Не смешивай Swagger 2.0 и OpenAPI 3.x. Если выбранный генератор формирует только Swagger 2.0, явно объясни это и выбери совместимый способ получения OpenAPI 3.1.

## Что документировать

Текущие маршруты нужно сверить с кодом, но в проверенной версии проекта есть:

- `GET /healthz` — вне `/api/v1`.
- `GET /api/v1/crm-clients?q=...`.
- `GET /api/v1/site-clients/:id`.
- `POST /api/v1/links`.
- `GET /api/v1/links`.
- `GET /api/v1/links/export.csv`.

Документируй HTTP DTO, а не Domain structs. Учитывай JSON tags, `omitempty`, binding rules, реальные статусы и error mapping. CSV export описывай как `text/csv` с ровно двумя колонками `crm_id` и `site_id`.

## Ограничения реализации

- Не меняй маршруты, HTTP methods, JSON поля, response bodies, HTTP statuses, validation или бизнес-логику ради аннотаций.
- Не меняй Domain/Application/Repository, миграции, CSV input formats или frontend.
- Не создавай дублирующие DTO, если текущие DTO уже представляют контракт.
- Аннотации размещай рядом с HTTP handlers или в поддерживаемом генератором месте.
- Генерация должна быть воспроизводимой одной документированной командой.
- Сгенерированные OpenAPI JSON/YAML должны попадать в репозиторий.
- Swagger UI должен работать локально без CDN и без обязательного доступа в интернет после сборки.
- Учитывай Docker build context, `.dockerignore`, `embed` и пути к spec.
- Не запускай генерацию автоматически при каждом старте сервера.

## OpenSpec

Используй существующий change задачи. Proposal не разрешает Apply: реализация требует явного запроса пользователя. Зафиксируй scope, решение по генератору/UI, маршруты, схемы, команды генерации и проверки. Не дели работу по одному change на каждый endpoint.

## Проверки

- Генерация проходит дважды без необъяснимого diff.
- JSON и YAML синтаксически валидны.
- Указана OpenAPI version 3.1.x, если именно OpenAPI 3.1 выбран в design.
- `servers` и base path соответствуют фактическим путям; `/api/v1` не дублируется.
- Swagger UI загружает спецификацию из локального backend.
- Все реальные маршруты имеют правильные request/response schemas, статусы и ошибки.
- Try it out собирает правильные URL.
- CSV описан с правильным media type и схемой загрузки.
- `go test ./...`, `go vet ./...`, `go build ./...`, `git diff --check`.
- Docker build/run проверяется, если Docker доступен; никаких `down -v`.

Заверши отчётом о зависимостях, файлах, команде генерации, адресах Swagger/spec, тестах и любых непроверенных пунктах.
