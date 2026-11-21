# Proposal

## Why

CRM-поиск уже показывает кандидатов, но CreateLink проверяет только фамилию и полную дату и отвергает любые расхождения. Stage 1 должен дать сотруднику объяснимую оценку пары и контролируемое ручное подтверждение, сохранив обязательную свежесть данных и уникальность связей.

## What Changes

- Отделить поиск, чистую доменную оценку пары и application-сценарий сохранения.
- Явно расширить загрузку missing birthday/name с неизвестными ролями без угадывания; сохранить результаты baseline 200/55 и алгоритм поиска.
- Добавить ready/requires_confirmation/blocked, признаки по ФИО и дате, явное отсутствие данных.
- Добавить POST /api/v1/link-assessments и assessment token; повторно получать обе записи перед POST /api/v1/links.
- Требовать подтверждение с причиной для расхождений; сохранять минимальный аудит атомарно со связью.
- Сохранить прежний POST с двумя ID для свежей ready-пары. **BREAKING**: прежняя семантика отказа при расхождении заменяется confirmation_required; nullable birthday отражается в HTTP/OpenAPI.
- Новая Goose-миграция только собственной БД, сохранение unique CRM/site. Обновить backend OpenAPI; frontend в этом change не изменять.

## Capabilities

### New Capabilities

- matching-assessment: объяснимая оценка пары, доступность источников и свежесть результата.
- link-confirmation: подтверждение расхождений, атомарное сохранение и аудит при конкуренции.

### Modified Capabilities

Нет. Существующая api-documentation уже требует паритет фактических маршрутов/DTO/статусов, это требование сохраняется.

## Impact

Domain client/matcher/link, application services/ports, HTTP handlers/router/DTO, CSV readers, PGLinkRepo, новая миграция, OpenAPI и тесты. Без новых внешних сервисов, Site PostgreSQL, SSH, Bitrix, поиска POST, frontend, авторизации и смены string Site ID. Дополнительные заметки roadmap о родителях/согласиях/карточках вне scope. Apply требует отдельного разрешения после Plan Review.
