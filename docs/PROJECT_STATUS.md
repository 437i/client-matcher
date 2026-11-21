# Фактическое состояние Client Matcher

Дата проверки: 2026-11-21. Долгосрочный план: [roadmap.md](roadmap.md).

## Git

Ветка feat/crm-name-search. Опубликованный tracking ref `origin/feat/crm-name-search` перед текущими изменениями и локальный HEAD имеют SHA `352a02df1e8871e0bfc191b2a429778311be5946` (проверено без fetch).
Дополнительная проверка Stage 0: для site ID `1326` («Азамат Далгатов») на полном синтетическом CRM fixture подтверждено отсутствие кандидатов — `CRMRepo.Search` возвращает `[]`. Сценарий добавлен в `TestStage0FullDataset`. Производственный код не изменялся. Проверки `go test -count=1 -run '^TestStage0FullDataset$' -v ./internal/adapters/outbound/crm/csv`, `go test -count=1 ./...` и `git diff --check` завершились успешно.

## Этапы и фаза

Stage 0 завершён по критериям roadmap после полного dataset Verify и повторного Claude Code Review READY WITH NOTES без Critical/High/Medium. Stage 1: Explore/Propose выполнены, Plan Review пройден. Stage 1 завершён после Apply, Verify и повторного Claude Code Review.
Предыдущие исправления Венгера и csv.ErrQuote закоммичены в HEAD. Прежний интерактивный review дополнен завершённым headless Code Review текущего Stage 0 scope; отчёты сохранены ниже.

## Проверенные результаты

- Пользователь подтвердил: testdata/crm.csv (200) и testdata/site.csv (55) — полный расширенный синтетический набор Stage 0.
- Отдельные package fixtures сохраняют все CSV-поля и записи; нормализованы только BOM/переводы строк. Demo не перезаписан.
- Новый TestStage0FullDataset: загружены все 200 CRM/55 Site; каждая CRM-запись находится по исходному полному ФИО; все site-запросы стабильны и ограничены 50; все именованные регрессии roadmap имеют точные ожидаемые ID, включая дубликаты и несовпадающую дату Волкова. Добавлен и подтверждён полноценный no-match сценарий для site ID 1326, «Азамат Далгатов»: `CRMRepo.Search` возвращает `[]`.
- Существующие тесты покрывают NFC/ё/е, алиасы в обе стороны, разделители, короткие запросы, опечатки, назначение токенов 63×63, ошибки CSV, context и лимиты.
- При проверке Stage 1 успешно: go test -count=1 ./..., go test -race -count=1 ./..., go vet ./..., go build ./..., git diff --check.
- Headless smoke test Sonnet 5.5 Medium из Codex: повтор вне sandbox exit 0 и CLAUDE_HEADLESS_OK; первый sandbox-запуск ENOTFOUND. Подробности в agent-infrastructure-verification.md.

## Неподтверждённое и открытые вопросы

- Stage 0 Code Review завершён; отчёт docs/stage0-code-review.md. Low-1 закрыт добавлением и проверкой no-match сценария site ID 1326, «Азамат Далгатов», с фактической выдачей `[]`. Ожидания site→CRM заданы для 14 именованных регрессий из 55 site-запросов; остальные проверены на загрузку, стабильность и лимит. Полный CRM набор проверен на находимость исходных записей и уникальность ID в выдаче.
- Site ID соответствует children.id или id_child_fond — не подтверждено; текущие string/TEXT сохраняются до отдельного решения.
- Семантика timestamp birthday как UTC и фильтры published/type/old/transfered/death_date не подтверждены.
- Структура Bitrix, права локального CRM индекса, способы авторизации неизвестны.
- Дополнительные заметки в конце roadmap (родители, согласия, карточки, ручное добавление) требуют уточнения и распределения по этапам; не включать автоматически в Stage 1.
- Авторизация и production-readiness отсутствуют; реальные источники не подключать.

## Gate и следующий шаг

Stage 0 gate закрыт. Stage 1 change: openspec/changes/matching-assessment-link-confirmation, proposal/specs/design/tasks созданы, strict validation 6/6. Первый Plan Review выявил четыре Medium; исправлены в планах, финальный review подтвердил отсутствие Critical/High/Medium.
Stage 1 gate закрыт. Change остаётся активным в OpenSpec. Stage 2–7 не начаты.

## Итоговые review gates

Stage 0: docs/stage0-code-review.md — READY WITH NOTES, без Critical/High/Medium. Low-1 закрыт: для site ID 1326 («Азамат Далгатов») добавлен полноценный no-match сценарий, фактическая выдача `CRMRepo.Search` — `[]`. Первые Medium по ожидаемым кандидатам и стабильности полного набора исправлены: таблица 14 site-регрессий, уникальность result IDs, сравнение всех 55 запросов при обратном порядке CRM. Остальные site-запросы проверены на загрузку/лимит/стабильность; ожидания не выдаются за доказанные пары.

Stage 1: docs/stage1-plan-review.md — финальная узкая проверка подтвердила закрытие M1–M4 и MD1, блокирующих Critical/High/Medium нет. Первичные отчёты сохранены отдельно как история. Все процессы review завершились exit 0; stream-json показывает модель claude-sonnet-5-5 и только Read/Grep/Glob. Reviewer не запускал тесты: результаты команд выполнены Codex.

Stage 1 Code Review завершён с вердиктом READY после исправления Medium по версии правил в аудите ready-связей. Подробности проверок и ревью приведены в [отчёте Stage 1](stage1-code-review.md). Тесты покрывают приоритет invalid CRM при отсутствующем Site и обе неизвестные роли имени при UnstructuredName. Multi-instance, авторизация и готовность к production остаются вне текущего scope.
