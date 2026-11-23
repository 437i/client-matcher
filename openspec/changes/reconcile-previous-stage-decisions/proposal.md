# Proposal

## Why

После Close Stage 3 действующие инструкции всё ещё смешивают локальный сценарий Stage 2 и production-запуск через SSH. Сверка также выявила, что при уже применённых миграциях production backend может открыть HTTP, когда OD_USER не может читать или сохранять связи.

## What Changes

- В production-запуске через SSH проверять права OD_USER на `apps.client_links` после успешного Goose Up и до запуска HTTP, в том числе при отсутствии ожидающих миграций.
- Согласовать roadmap, README, локальный пример настроек и краткие инструкции проекта с подтверждёнными режимами Stage 3, схемой OD и двумя способами запуска Goose. Подробности SSH оставить в `docs/stage3-production.md`.
- За рамками изменения остаются поиск, assessment, запись и аудит связей, HTTP/OpenAPI, код frontend, SQL-миграции, перенос данных, конфигурация серверов, production deployment, авторизация приложения и будущие этапы.

## Capabilities

### New Capabilities

### Modified Capabilities

- `ssh-postgres-connectivity` — конкретный критерий готовности хранилища связей в уже реализованной возможности Stage 3. Её исходные требования пока находятся в активном `ssh-tunnel-infrastructure`. Дельта добавляет только проверку доступа к таблице связей и имеет отдельное имя требования.

## Impact

- Проверка готовности OD в `backend/internal/platform/db/migrate.go` и сценарии запуска в `backend/cmd/server/production_test.go`.
- Уточнения в `README.md`, `.env.example`, `docs/roadmap.md`, `docs/stage3-production.md`, `AGENTS.md`, `backend/AGENTS.md` и `openspec/config.yaml`.
- Результаты Verify в `docs/PROJECT_STATUS.md`.
