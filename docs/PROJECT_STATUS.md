# Фактическое состояние Client Matcher

Дата проверки: 2026-11-25. Долгосрочный план: [roadmap.md](roadmap.md).

## Stage 4 — Bitrix24 Adapter

План `bitrix24-crm-adapter` получил `READY` после трёх раундов независимого Claude Plan Review и утверждён владельцем. Apply начался от `c6a330aa4f9f5b8dd63bd453470eb75cc57b8dbf` на ветке `feat/crm-name-search`. Реализация и локальный Verify завершены. [Задачи](../openspec/changes/bitrix24-crm-adapter/tasks.md)

- Go 1.26.1. Успешны go test -count=1 ./..., go vet ./..., go build ./..., go mod verify и проверка версии B24GoSDK v0.2.0. Пройдены CSV Stage 0 (200 CRM / 55 Site), локальные Bitrix HTTP/TLS, HTTP parity и сквозные Search, Assess и CreateLink. Интеграционные тесты с отдельными PostgreSQL оставались отключёнными.
- Успешны строгая валидация change и всех 11 элементов OpenSpec, git diff --check и обе проверки Compose config --quiet с синтетическим окружением. SHA-256 HTML-макета совпал с preflight.
- Замеры fake portal и открытые проверки прав, OR, сортировки, collation, next, пустых UF, квот, задержек, NOT_FOUND и ingress описаны в техническом документе. Проверки на реальном Bitrix24, production smoke test и работа с production PostgreSQL отложены до разрешённого доступа. [Адаптер и чеклист](bitrix24-adapter.md)
- После Code Review с вердиктом READY WITH NOTES увеличен локальный бюджет до 20 страниц, сохранены ошибки контекста при сбое декодирования GetByID и защищено сопоставление токенов для серверного фильтра. Повторный Verify прошёл на локальных синтетических данных. Целевые тесты, полный Go-прогон, race-прогон для затронутых и сквозных пакетов, vet, build, mod verify и обе строгие проверки OpenSpec успешны. Тесты с PostgreSQL были отключены.
- Исправление F-1 принято. При нарушении инварианта сопоставления токенов адаптер возвращает `errTokenAlignment` до построения фильтра и вызова Bitrix24. HTTP использует существующий `500 internal_error`. Пустой запрос по-прежнему даёт пустую выдачу. После исправления прошли целевые Go- и HTTP-тесты, `go vet`, `go build ./...`, `git diff --check` и две строгие проверки OpenSpec. Общий Go-прогон прошёл с исключением четырёх интеграционных групп, требующих отдельного окружения. Финальный независимый Code Review дал `READY` без новых подтверждённых дефектов.

Project Close Stage 4 оформлен. Владелец подтвердил выполнение задачи 13.1. Все 30 из 30 задач выполнены. Change остаётся активным в OpenSpec со статусом `complete`. Sync и Archive выполняются отдельно по запросу. Следующий gate — Explore этапа 5 «Поисковый HTTP API». [Задачи](../openspec/changes/bitrix24-crm-adapter/tasks.md) · [Дорожная карта](roadmap.md#этап-5-поисковый-http-api)

## Git

На preflight Stage 2 Propose ветка `feat/crm-name-search` и локальный tracking ref `origin/feat/crm-name-search` совпали на `cab2621fa1f81bbd316f69fbdec02686818a059c` без fetch. Исходное рабочее дерево было чистым.
Дополнительная проверка Stage 0: для site ID `1326` («Азамат Далгатов») на полном синтетическом CRM fixture подтверждено отсутствие кандидатов — `CRMRepo.Search` возвращает `[]`. Сценарий добавлен в `TestStage0FullDataset`. Производственный код не изменялся. Проверки `go test -count=1 -run '^TestStage0FullDataset$' -v ./internal/adapters/outbound/crm/csv`, `go test -count=1 ./...` и `git diff --check` завершились успешно.

На preflight Stage 3 Close ветка `feat/crm-name-search` и локальный tracking ref `origin/feat/crm-name-search` совпали на `3d02baacf7b013bf1652a6e201df1088825b53e7`. Сверка выполнена без fetch. Рабочее дерево и индекс были чистыми, включая проверку untracked-файлов.

## Этапы и фаза

Stage 0 завершён по критериям roadmap после полного dataset Verify и повторного Claude Code Review READY WITH NOTES без Critical/High/Medium. Stage 1: Explore/Propose выполнены, Plan Review пройден. Stage 1 завершён после Apply, Verify и повторного Claude Code Review.
Предыдущие исправления Венгера и csv.ErrQuote закоммичены в HEAD. Прежний интерактивный review дополнен завершённым headless Code Review текущего Stage 0 scope; отчёты сохранены ниже.

Change `reconcile-previous-stage-decisions` закрыт после повторной проверки Low-2.

## Проверенные результаты

- Пользователь подтвердил: testdata/crm.csv (200) и testdata/site.csv (55) — полный расширенный синтетический набор Stage 0.
- Отдельные package fixtures сохраняют все CSV-поля и записи; нормализованы только BOM/переводы строк. Demo не перезаписан.
- Новый TestStage0FullDataset: загружены все 200 CRM/55 Site; каждая CRM-запись находится по исходному полному ФИО; все site-запросы стабильны и ограничены 50; все именованные регрессии roadmap имеют точные ожидаемые ID, включая дубликаты и несовпадающую дату Волкова. Добавлен и подтверждён полноценный no-match сценарий для site ID 1326, «Азамат Далгатов»: `CRMRepo.Search` возвращает `[]`.
- Существующие тесты покрывают NFC/ё/е, алиасы в обе стороны, разделители, короткие запросы, опечатки, назначение токенов 63×63, ошибки CSV, context и лимиты.
- При проверке Stage 1 успешно: go test -count=1 ./..., go test -race -count=1 ./..., go vet ./..., go build ./..., git diff --check.
- Headless smoke test Sonnet 5.5 Medium из Codex: повтор вне sandbox exit 0 и CLAUDE_HEADLESS_OK; первый sandbox-запуск ENOTFOUND. Подробности в agent-infrastructure-verification.md.

## Неподтверждённое и открытые вопросы

- Stage 0 Code Review завершён; отчёт docs/stage0-code-review.md. Low-1 закрыт добавлением и проверкой no-match сценария site ID 1326, «Азамат Далгатов», с фактической выдачей `[]`. Ожидания site→CRM заданы для 14 именованных регрессий из 55 site-запросов; остальные проверены на загрузку, стабильность и лимит. Полный CRM набор проверен на находимость исходных записей и уникальность ID в выдаче.
- Дополнительные заметки в конце roadmap (родители, согласия, карточки, ручное добавление) требуют уточнения и распределения по этапам; не включать автоматически в Stage 1.
- Собственная аутентификация и роли приложения не реализуются по осознанному решению владельца и не являются незавершённой задачей разработки.
- Авторизация во внешней CRM (Bitrix), структура и права её локального индекса пока неизвестны и требуют выяснения при работе над соответствующей интеграцией.
- Production-развёртывание ещё не выполнено. Выбор, настройка и проверка инфраструктурных механизмов защиты относятся исключительно к ответственности команды деплоя, а не разработчика приложения.

## Gate и следующий шаг

Stage 0 gate закрыт. Stage 1 change: openspec/changes/matching-assessment-link-confirmation, proposal/specs/design/tasks созданы, strict validation 6/6. Первый Plan Review выявил четыре Medium; исправлены в планах, финальный review подтвердил отсутствие Critical/High/Medium.
Stage 1 gate закрыт. Выполнены OpenSpec Sync и Archive. Архивированный change находится в `openspec/changes/archive/2026-11-21-matching-assessment-link-confirmation/`. Stage 2 прошёл Explore, Propose и Plan Review.

Stage 3 gate закрыт. Перед Stage 4 необходимо отдельно выполнить `reconcile-previous-stage-decisions` в собственном OpenSpec change.

## Итоговые review gates

Stage 0: docs/stage0-code-review.md — READY WITH NOTES, без Critical/High/Medium. Low-1 закрыт: для site ID 1326 («Азамат Далгатов») добавлен полноценный no-match сценарий, фактическая выдача `CRMRepo.Search` — `[]`. Первые Medium по ожидаемым кандидатам и стабильности полного набора исправлены: таблица 14 site-регрессий, уникальность result IDs, сравнение всех 55 запросов при обратном порядке CRM. Остальные site-запросы проверены на загрузку/лимит/стабильность; ожидания не выдаются за доказанные пары.

Stage 1: docs/stage1-plan-review.md — финальная узкая проверка подтвердила закрытие M1–M4 и MD1, блокирующих Critical/High/Medium нет. Первичные отчёты сохранены отдельно как история. Все процессы review завершились exit 0; stream-json показывает модель claude-sonnet-5-5 и только Read/Grep/Glob. Reviewer не запускал тесты: результаты команд выполнены Codex.

Stage 1 Code Review завершён с вердиктом READY после исправления Medium по версии правил в аудите ready-связей. Затем выполнены Sync и Archive, результат сохранён в архивном каталоге выше. Подробности проверок и ревью приведены в [отчёте Stage 1](stage1-code-review.md). Тесты покрывают приоритет invalid CRM при отсутствующем Site и обе неизвестные роли имени при UnstructuredName. Multi-instance, авторизация и готовность к production остаются вне текущего scope.

## Stage 2 Propose

Change [site-postgresql-adapter](../openspec/changes/archive/2026-11-23-site-postgresql-adapter/proposal.md) содержит proposal, design, три delta specs и tasks. OpenSpec strict validation прошла. Успешный основной Plan Review выполнен Claude Code 2.1.329 с `claude-sonnet-5-5`, effort medium, `--permission-mode plan`, `--tools Read,Grep,Glob`, запретом Bash/PowerShell/Edit/Write/NotebookEdit/Agent, `--strict-mcp-config`, пустым MCP config `{"mcpServers":{}}`, отключёнными hooks через `--settings`, `--permission-prompts none`, `--max-turns 20`, `--output-format stream-json` и `--verbose`. Все параметры явно переданы через argv и не наследовались. Fallback model не использовался. Python wrapper использовал `p.wait(timeout=1190)`, внешний Code Mode execution — `yield_time_ms=1300000`. Wrapper elapsed составил 276.32 секунды, Claude duration — 275605 мс, `exit_code=0`, `is_error=false`, `num_turns=26`, 25 tool calls (Read 23, Glob 2), событие `result` присутствует. Verdict — `READY WITH NOTES`. Первичные findings включали один High и два Medium, исправленные в planning artifacts.

Две диагностические попытки использовали wrapper timeout 595 и 1190 секунд, получили 49 и 57 tool calls и завершились без события `result` и без verdict. В сохранённых журналах API/network errors не зафиксированы. Wrapper остановил процессы по timeout. Широкий scope, вероятно, способствовал чрезмерному исследованию, но конкретная внутренняя причина не доказана.

Успешный follow-up был ограничен `design.md` и `tasks.md`, использовал `--max-turns 4`, `p.wait(timeout=180)` и внешний `yield_time_ms=650000`. Wrapper elapsed составил 75.89 секунды, Claude duration — 75401 мс, `exit_code=0`, `is_error=false`, `num_turns=3`, два Read tool calls и событие `result`. Verdict — `READY WITH NOTES`, Critical/High/Medium отсутствуют. Оставшиеся Low-заметки касались явного сопоставления `infinity`, `-infinity` и дат вне диапазона с `client.ErrInvalidSourceData`, а также default `SITE_SOURCE=csv`. Эти уточнения внесены в текущем корректирующем изменении в `design.md` и `tasks.md`. После изменений strict validation и `git diff --check` повторно прошли. Plan Review gate закрыт. Последующее состояние реализации зафиксировано в разделе Stage 2 Apply / Verify.

- Site ID — `public.children.id` (`bigserial`). Внутренний тип переводится на `int64`, хранилище связей на `BIGINT` новой Goose-миграцией, HTTP сохраняет строки.
- `birthday` преобразуется как `((birthday AT TIME ZONE 'UTC') AT TIME ZONE 'Europe/Moscow')::date`. NULL соответствует `nil`. Некорректная дата возвращает ошибку чтения источника.
- Чтение по ключу без дополнительных фильтров. CSV-демонстрация сохраняется. PostgreSQL сайта получает отдельную конфигурацию и пул. Интеграционные проверки используют временные синтетические БД.

Для следующих этапов согласован debounce CRM-поиска около 300 мс. Внутренние учётные записи и роли приложения исключены из плана. Ограничение доступа к production-развёртыванию с реальными данными обеспечивает ответственная команда деплоя. Публичное развёртывание без соответствующей инфраструктурной защиты небезопасно.


## Stage 2 Apply / Verify

Apply завершён. Коммит `478b2cf720f55f978805a16bb8205d4fd751d7dd` с сообщением `feat: add PostgreSQL site adapter` отправлен в `origin/feat/crm-name-search`. Site ID переведён на `int64` в Go и `BIGINT` в БД связей, HTTP сохранил строковый формат. Введены токены версии 2, PostgreSQL SiteReader, выбор источника CSV или PostgreSQL и отдельный пул сайта. Выполнены все 10 задач из [tasks.md](../openspec/changes/archive/2026-11-23-site-postgresql-adapter/tasks.md).

Verify завершён. По отчёту Codex за Stage 2 прошли `go test -count=1 ./...`, `go test -race -count=1 ./...`, `go vet ./...`, `go build ./...`, `git diff --check` и `openspec validate --all --strict`. На двух отдельных временных PostgreSQL БД с синтетическими данными проверены SiteReader, репозиторий связей и миграция Up/Down, включая отказ на несовместимых данных. Сквозной запуск в режимах CSV и PostgreSQL подтвердил работу проверки состояния сервера, чтения карточки, оценки и сохранения связи. При ошибке выбранного источника сервер завершал запуск без перехода на CSV, открытые пулы закрывались.

Code Review пройден с вердиктом `READY WITH NOTES`. Подтверждённых Critical, High и Medium нет. Claude `claude-sonnet-5-5` с effort medium запущен через документированный Python wrapper с `p.wait(timeout=1190)`, внешним `yield_time_ms=1300000` и `--max-turns 20`. Использованы только `Read,Grep,Glob` и пустой MCP config. Зафиксированы `exit_code=0`, `is_error=false`, событие `result` и 85 вызовов инструментов.

Low-заметки Code Review

1. Канонизация строкового Site ID во frontend относится к Stage 5.
2. Фактический тип `birthday` в схеме сайта нужно подтвердить до подключения реальной БД.
3. Тесты можно дополнить редкими SQLSTATE и отменой запроса во время его выполнения.

Sync завершён для трёх основных спецификаций. Archive завершён. Change находится в [openspec/changes/archive/2026-11-23-site-postgresql-adapter/](../openspec/changes/archive/2026-11-23-site-postgresql-adapter/). Stage 2 завершён.


### Claude Code Review connectivity note

Claude Code в этом окружении использует кастомный провайдер ANTHROPIC_BASE_URL=https://api.wellflow.dev/v1, а не публичный Anthropic endpoint. Ошибка ENOTFOUND возникала при sandbox-запуске subprocess без сетевого доступа. Рабочий способ — запускать документированный Python wrapper через внешний Code Mode execution с сетевым разрешением, сохранять p.wait(timeout=1190) и ждать terminal session через write_stdin каждые 30 секунд до exit_code. Проверка api.wellflow.dev подтвердила DNS и HTTPS доступность без передачи credentials.


## Stage 3 Apply / Verify / Code Review / Close

Apply и Verify change [ssh-tunnel-infrastructure](../openspec/changes/ssh-tunnel-infrastructure/tasks.md) завершены. Выполнены все 12 задач. На preflight перед Apply ветка `feat/crm-name-search` и локальный upstream совпали на `1e316b69b8d47a18ac86e2643b5c4e1228882a83`. Рабочее дерево было чистым, в задачах уже было уточнение о тесте отказа при зашифрованном OD SSH-ключе. Ниже зафиксированы результаты технических проверок Codex при Verify.

- Синтетическая SITE на PostgreSQL 10.23 собрана из официального исходника. Через SSH получен server_version_num=100023. SiteReader подтвердил ID, title, преобразование даты на границе суток и NULL в birthday с доступом SELECT к трём колонкам. Для OD через SSH подтверждена PostgreSQL 16.15 (160015).
- Интеграционные и race-тесты подтвердили независимое восстановление пулов, отклонение позднего CreateResource в PrepareConn, конкурентные и прерванные Goose Up, изоляцию apps от public и освобождение ресурсов при startup и SIGTERM.
- Локальный и production Compose собраны и проверены через frontend Nginx. Production восстановился после последовательных обрывов SITE и OD без перезапуска backend. Оба backend завершились с кодом 0. В production проверены три read-only mount, чтение от app, публикация только frontend и исключение runtime-файлов из обоих build contexts и итоговых образов.
- Прошли go test -count=1 ./..., go test -race -count=1 ./..., отдельные интеграционные прогоны с синтетическими БД, go vet ./..., go build ./..., git diff --check и openspec validate --all --strict (9 из 9). Граф подтвердил Goose v3.27.1, pgx v5.12.0, puddle v2.2.3 и x/crypto v0.50.0.

Независимый Code Review Qwen 4 Max завершён с вердиктом `READY WITH NOTES`. Подтверждённых Critical, High и Medium нет. Reviewer провёл статический анализ без запуска тестов, Docker и миграций.

Low L-1 принят как необязательное улучшение. В `backend/internal/platform/sshtunnel/tunnel.go` реализован лимит `maxForwards = 32`. Отдельного теста с 32 активными пробросами и отказом 33-му пока нет. Отдельный тест half-close в `relay` также оставлен как необязательное улучшение. Reviewer упомянул его без оформления finding.

Stage 3 закрыт по текущим критериям готовности. OpenSpec change имеет статус `complete` и остаётся в списке активных изменений.

## Сверка Stage 0–2 со Stage 3

Explore и Propose завершены в [reconcile-previous-stage-decisions](../openspec/changes/reconcile-previous-stage-decisions/proposal.md). Сверка выявила недостаточную проверку прав на таблицу связей при повторном SSH-запуске и устаревшие инструкции по подключению и миграциям. OpenSpec strict validation прошла для нового change и для всех 10 проверяемых элементов. Git diff check прошёл, включая отдельно проверенные новые артефакты. Выполнено 0 из 8 задач реализации.

Независимый Plan Review Qwen 4 Max дал вердикт `READY WITH NOTES` без Critical и High. По M-1 в задаче 1.1 разделены владелец таблиц и `OD_USER`, уточнены эффективные колоночные права, отрицательные сценарии, положительный контроль под той же ролью и ожидаемое падение теста на исходном коде. По L-1–L-3 уточнены границы SSH-проверки, перечень файлов при Apply и сохранение обеих активных дельт при отдельном будущем Sync.

### Apply

Пользователь разрешил Apply после Plan Review. Перед реализацией `HEAD`, `origin/feat/crm-name-search` и опубликованный SHA ветки совпали на `7b2f82ae3bdbced225f288f2b304a83c9d2b883b`. Индекс и рабочее дерево были чистыми.

- До исправления `TestProductionProcess/OD_readiness/missing_INSERT` упал с `HTTP listened during startup` после подготовки OD отдельной ролью-владельцем.
- После исправления `TestProductionPostgres` и `TestProductionProcess` прошли на синтетических SITE 10.23 и OD 16.15. Проверены четыре отказа до открытия HTTP, включая одноимённую таблицу в `public`, успешный запуск под тем же `OD_USER` с полным набором колоночных прав, освобождение подключений и неизменность связей и журнала Goose.
- `TestLoadSiteSource` прошёл. После ручного Goose Up оба direct-режима вернули HTTP 200 на `/healthz` и `/api/v1/links`. Локальная конфигурация Compose проверена командой `docker compose --env-file .env.example -f docker-compose.yml config`.

Задачи 1.1–1.3 и 2.1–2.3 завершены. Задачи 3.1 и 3.2 ожидают отдельного Verify.

### Verify

На preflight ветка `feat/crm-name-search` и локальный `origin/feat/crm-name-search` совпали с `HEAD` на `7b2f82ae3bdbced225f288f2b304a83c9d2b883b`. Все 11 файлов Apply присутствовали в рабочем дереве. Staged и untracked-файлов не было.

- Из `backend/` прошли `go test -count=1 ./...`, `go test -race -count=1 ./...`, `go vet ./...` и `go build ./...`. Регрессии CSV, Site ID, assessment и сохранения прошли.
- Последовательный прогон `CLIENT_MATCHER_STAGE3_INTEGRATION=1 go test -p 1 -race -count=1 -v -run '^(TestProductionPostgres|TestProductionProcess)$' ./internal/platform/db ./cmd/server` прошёл. Синтетические SITE 10.23 (`100023`) и OD 16.15 (`160015`) подтверждены через локальные порты и через SSH в `TestProductionPostgres`.
- После Goose Up в полностью мигрированной OD подтверждены отказы до открытия HTTP при отсутствии `SELECT`, `INSERT`, `INSERT` на `confirmation_reason` и `apps.client_links` при наличии доступного `public.client_links`. Под той же ограниченной ролью с полным набором колоночных прав сервер открыл HTTP. Тесты подтвердили закрытие туннелей и соединений runtime-роли, неизменность строк связей и журнала Goose.
- После ручного Goose Up `direct + csv` и `direct + postgres` вернули HTTP 200 на `/healthz`, `/api/v1/links` и `/api/v1/site-clients/276`. Оба процесса штатно завершились по SIGTERM. `TestPostgresLinkAudit` и `TestSiteReaderPostgresIntegration` отдельно прошли с race detector на тех же синтетических БД.
- Прошли `openspec validate reconcile-previous-stage-decisions --strict`, `openspec validate --all --strict` (10 из 10), `git diff --check` и `git diff --cached --check`. Изменения в пределах change. Все обязательные проверки Verify выполнены.

Задачи 3.1 и 3.2 завершены, OpenSpec показывает `complete` (8 из 8).

### Code Review и исправление Low-2

Независимый Code Review Qwen 4 Max завершён с вердиктом `READY WITH NOTES` без подтверждённых Critical, High или Medium. Для Low-2 `MigrateOD` возвращает `db.ErrODLinkStorageNotReady`, а `TestProductionProcess` проверяет эту ошибку через `errors.Is`. Внешний текст ошибки сохранён дословно — `OD link storage is not ready`. Low-1 и Low-3 приняты без исправлений.

После исправления прошли `go test -count=1 ./...`, `go vet ./...` и `go build ./...`. `TestProductionPostgres` и `TestProductionProcess` прошли при последовательном запуске с `CLIENT_MATCHER_STAGE3_INTEGRATION=1` и race detector на одноразовых синтетических SITE 10.23 и OD 16.15. `git diff --check` и `git diff --cached --check` прошли.

Следующий gate — короткая независимая read-only проверка Low-2. После её успешного прохождения — Close текущего OpenSpec change.

### Close

Независимый targeted Code Review Qwen по исправлению Low-2 завершён с вердиктом `PASS`. Итог полного независимого Code Review — `READY WITH NOTES`. Low-2 исправлен через `db.ErrODLinkStorageNotReady` и проверку `errors.Is` с сохранением внешнего текста ошибки. Low-1 и Low-3 приняты без исправлений.

`reconcile-previous-stage-decisions` закрыт. Apply и Verify завершены, все 8 из 8 задач OpenSpec выполнены. Обязательные проверки Verify и повторные проверки Codex после исправления Low-2 успешно пройдены. Change остаётся активным в OpenSpec со статусом `complete`. Sync и Archive выполняются отдельно.

Итог независимого Code Review и Project Close приведены в [разделе Stage 4](#stage-4--bitrix24-adapter).
