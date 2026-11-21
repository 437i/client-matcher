# Tasks

## 1. Domain и nullable источники

- [ ] 1.1 В domain/client добавить nullable birthday и missing name semantics по design, адаптировать CSV readers без изменения demo; test-first проверить nil, malformed date, одночастное/пустое имя с UnstructuredName/unknown roles без угадывания, поиск по этим записям и неизменность полных ordered search результатов baseline 200/55.
- [ ] 1.2 В domain/matcher реализовать Assessment/FieldCheck/status с разделением ролей ФИО и дат; table-driven tests проверяют ready, aliases, probable, mismatch, missing, неизвестное отчество Site, enum/order полей, агрегацию статуса, prefix/substring mismatch и отсутствие процентов, не меняя алгоритм CRM search.

## 2. Freshness и application

- [ ] 2.1 Добавить application token port/clock и локальный HMAC adapter, случайный ключ в composition root; тесты проверяют 5 минут TTL, signature, wrong IDs, 4096 bytes, keyed HMAC fingerprints, canonical length-prefix serialization, nil/present, domain-separated subkeys, разные runtime keys, token_version/rule_version и отсутствие PII в claims.
- [ ] 2.2 Добавить assessment service и advisory LookupBySourceIDs port; mocks проверяют missing/occupied blocked с заданным приоритетом, client.ErrInvalidSourceData через mock-порт, source errors и отсутствие записи link при assessment.
- [ ] 2.3 Обновить CreateLink service для повторного чтения, token validation, stale, confirmation/reason и ready compatibility; test-first проверить каждый запрет и изменение источника, включая косметическое и requires_confirmation→ready.

## 3. Собственная БД и аудит

- [ ] 3.1 Добавить additive Goose migration client_links с audit columns/checks по design, не меняя прежние миграции; Up/Down integration test на временной synthetic PostgreSQL проверяет defaults старых строк и manual consistency и отображение last_name/first_name→name:code с дедупликацией и сортировкой codes.
- [ ] 3.2 Обновить PGLinkRepo: advisory lookup и единый INSERT link+audit с прежним LinkResponse; реальные PostgreSQL tests проверяют оба unique conflicts, concurrent insert, rollback при CHECK и отсутствие ФИО/дат/token в audit; linkColumns/RETURNING не расширяются. Двойной unique conflict допускает любой из двух кодов.

## 4. HTTP и документация контракта

- [ ] 4.1 Добавить assessment handler/router/DTO и расширить create DTO/error mapping; HTTP tests проверяют все method/path/status/JSON из design, nullable birthday, ready legacy request, blocked_reasons, invalid source 502, приоритет ошибок, body 16 KiB/413, stale, expired, invalid token, ready confirmation rejection, reason и отмену.
- [ ] 4.2 Обновить backend OpenAPI JSON/YAML и описание сценария; contract tests проверяют route/status parity, reproducible generation, прежний list/export и отсутствие исходных значений/reason/token в логах.

## 5. Интеграционная проверка и принятие

- [ ] 5.1 Запустить gofmt -d, go test -count=1 ./..., go test -race -count=1 ./..., go vet ./..., go build ./..., frontend typecheck/tests без изменения frontend, DB integration tests и git diff --check; записать точные результаты, не выдавать пропущенные DB tests за успех.
- [ ] 5.2 Выполнить headless Claude Code Review точного diff и untracked, исправить подтверждённые Critical/High/Medium, повторить затронутые проверки и review; обновить PROJECT_STATUS по критериям Stage 1.

## Workflow follow-up

- Apply только после отдельного разрешения пользователя и пройденного Plan Review.
- Sync/Archive отдельно по запросу после Verify и Code Review; не архивировать автоматически.
