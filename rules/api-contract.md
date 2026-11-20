# HTTP API contract rules

- Источник истины для существующих маршрутов — Gin registration в `backend/internal/adapters/inbound/http/router.go`.
- Источник истины для JSON request/response — DTO в `handlers/dto.go` и фактические handlers.
- Источник истины для статусов и error codes — `handlers/response.go` и код конкретного handler.
- Не додумывай параметры, поля, статусы, ошибки или примеры. Если поведение не подтверждено кодом/тестом, укажи неопределённость.
- Не меняй контракт ради генератора OpenAPI. Аннотации должны отражать реальный код.
- CSV export — отдельный бинарный/текстовый HTTP-ответ с `Content-Type: text/csv; charset=utf-8`; CSV содержит только `crm_id` и `site_id`.
- `/healthz` расположен вне `/api/v1`.
- Не путай публичные HTTP DTO с Domain-моделями.
- Не помещай внутренние ошибки, SQL, stack trace, переменные окружения и секреты в публичные ответы.
