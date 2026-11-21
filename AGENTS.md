# Инструкции для AI-агентов проекта

## Главный принцип

Сначала изучай фактический код и текущее состояние Git, затем предлагай изменения. Не делай выводы только по README, старым сообщениям или предположениям. Не начинай массовое редактирование до понимания зависимостей и сценария.

Отвечай пользователю на русском языке. Комментарии в коде добавляй только там, где они объясняют нетривиальное решение; не переписывай существующий стиль без необходимости.

## Дорожная карта и статус

Перед продолжением разработки читай docs/roadmap.md и docs/PROJECT_STATUS.md. Долгосрочную последовательность и критерии готовности бери из roadmap, актуальные факты и gates — из PROJECT_STATUS. Не меняй порядок этапов без явного решения пользователя. Обновляй PROJECT_STATUS на границах этапов и после проверок/review. Наличие кода не означает завершённость этапа.

## О проекте

- Репозиторий: `github.com/437i/client-matcher`.
- Go-модуль backend: `github.com/437i/client-matcher/backend`.
- Go module находится в `backend/go.mod`; минимальная заявленная версия проекта — Go 1.26.1.
- Backend — Gin REST API.
- Входные источники на тестовом этапе — CSV. В дальнейшем CRM читается через API, база сайта — напрямую из PostgreSQL.
- Собственная база приложения хранит только связи `crm_id` ↔ `site_id`.
- Frontend находится в frontend/ (React + TypeScript + Vite); backend-задачи не должны менять frontend без прямого требования.
- Не смешивай адаптеры внешних источников с хранилищем связей приложения.

## Обязательный порядок работы

1. Выполни `git status --short` и изучи существующий diff. Не перезаписывай незакоммиченные изменения пользователя.
2. Для новой задачи изучи релевантные маршруты, handlers, DTO, services, ports, domain и adapters. Для многофайловой или архитектурной задачи сначала составь короткий план.
3. Для изменений, затрагивающих несколько слоёв или новый функциональный сценарий, используй OpenSpec и review gates по docs/agent-workflow.md: Explore → Propose → Plan Review → User Approval → Apply → Verify → Code Review → Fix/Re-verify → Close. Sync/Archive — отдельно по запросу. Не создавай OpenSpec change для тривиальной правки одного файла.
4. Сохраняй текущие соглашения и минимальный объём изменений. Не проводи несвязанный рефакторинг.
5. После изменений запусти подходящие проверки. Чётко разделяй выполненные и не выполненные проверки; не утверждай, что непроверенный Docker или тест успешно работает.
6. В конце перечисли изменённые файлы, причины, проверки и оставшиеся риски.

## Защита данных и операций

- CSV проекта и fixtures подтверждены пользователем как синтетические и допустимы. Не добавляй реальные клиентские данные; происхождение неизвестных экспортов уточняй до чтения.
- Не читай `.env`, `.env.*` (кроме явно безопасного `.env.example`), credentials, токены и секреты.
- Не отправляй исходные данные, персональные данные, секреты или содержимое пользовательских файлов во внешние сервисы.
- Не выполняй `git push`, `git reset --hard`, `git clean`, `docker compose down -v`, удаление Docker volumes или очистку Docker без отдельного прямого разрешения пользователя.
- Не выполняй commit самостоятельно, если пользователь явно не попросил.
- Не меняй исходные данные, внешние CSV-файлы и миграции без прямого требования.

## Правила и дополнительные инструкции

Перед работой с Go-кодом соблюдай `backend/AGENTS.md` и `rules/go.md`.
Для архитектуры используй `rules/architecture.md`; для HTTP-контракта — `rules/api-contract.md`; для тестов — `rules/testing.md`; для данных — `rules/data-safety.md`.
Для специальных задач загружай подходящий skill из `.agents/skills/`.

## Frontend

- Frontend исходники размещаются в `frontend/`; приложение уже собрано, его соглашения описаны в `frontend/AGENTS.md`.
- Для frontend соблюдай `rules/frontend.md` и `frontend/AGENTS.md`.
- Backend OpenAPI (`backend/openapi/swagger.yaml`) — единственный источник истины для клиентских endpoint/DTO/status types.
- Frontend не должен дублировать backend matching logic или менять существующий API contract ради удобства UI.
- Для многофайловых frontend-задач используй существующий OpenSpec change workflow.
- CSV проекта и fixtures подтверждены пользователем как синтетические и допустимы. Не добавляй реальные клиентские данные; происхождение неизвестных экспортов уточняй до чтения.
- Контроль доступа не реализован: версия предназначена для доверенного локального или изолированного окружения, публичное развёртывание с реальными данными запрещено.

## Frontend UX for fund staff

- The frontend is used by staff with mixed and often low computer confidence. Treat `rules/non-technical-ux.md` as mandatory product requirements.
- Optimise for clarity, confidence and error prevention, not for developer convenience or visual novelty.
- Use plain Russian, visible input labels, familiar action text, one clear next step, a review of both records before saving and a persistent save confirmation.
- Preserve user work on failure. Explain errors and safe next steps without technical jargon.
- Require a novice-style heuristic walkthrough and WCAG 2.2 AA-oriented accessibility checks. Never claim actual user research without representative staff sessions.

## Модели и делегирование

Основной Codex: gpt-6.2-sol, reasoning medium. Субагенты: gpt-6-luna, reasoning high; максимум 2 одновременно, вложенные субагенты запрещены. Основной агент отвечает за архитектуру, постановку задач, интеграцию, проверку результатов субагентов, тесты и итоговый результат. Контракт субагента: цель, типы данных, инварианты, ограничения, граничные значения, область изменения, критерии результата. Предположения нельзя выдавать за требования. DeepSeek/GLM и динамический маршрутизатор не добавляются.

Claude Plan review обязателен для архитектуры, business logic, API, DB, безопасности, matching и инфраструктуры; для тривиальных локальных/documentation изменений достаточно проверок Codex. Процедура и локальный config: docs/agent-workflow.md.
