# Tasks

## 1. Сквозной Site ID и HTTP

- [ ] 1.1 В internal/domain, internal/application/ports и services перевести модели, SiteReader, поиск по ID, оценку, создание и lookup связей на int64. Обновить моки и тесты. Обновить сигнатуры `ports.AssessmentToken.Verify` и `HMACToken.Verify` до `Verify(ctx context.Context, token, crmID string, siteID int64)`. Убедиться, что значения вне точного диапазона JavaScript проходят без потерь, а идентификаторы CRM и сценарии ready/blocked/confirmation сохраняются.
- [ ] 1.2 В internal/adapters/outbound/token_hmac.go и порте токена ввести версию 2, проверку числовой пары и канонический fingerprint. В `sourceBytes` преобразовать Site ID через `strconv.FormatInt(site.ID, 10)` до формирования fingerprint. Проверить прежнюю версию, эквивалентные записи ключа, другой ключ, изменение снимка, TTL и отсутствие исходных ФИО и дат в claims.
- [ ] 1.3 Во входящих HTTP handlers, DTO, CSV export и backend/openapi сохранить строки и применить единое строгое преобразование. Проверить GET карточки, assessment, create, list и export, включая 0, границы int64, переполнение, знак, пробелы, ведущие нули, дробный и экспоненциальный ввод, JSON-число, ошибки до вызова сервисов, точность ответа и parity OpenAPI.

## 2. Хранилище связей

- [ ] 2.1 В backend/migrations добавить новую транзакционную Goose-миграцию с блокировкой таблицы до проверок. На временной собственной PostgreSQL протестировать Up и Down, включая границы int64, нечисловое значение, переполнение, коллизию после нормализации, точное сохранение старых строк и схемы при отказе, канонический TEXT после Down и сохранение аудита и ограничений.
- [ ] 2.2 В internal/adapters/outbound/link/postgres обновить запись, сканирование и lookup для BIGINT. Интеграционно проверить оба unique-конфликта, конкурентное создание, атомарность аудита и неизменный порядок выдачи.

## 3. Источники сайта и сервер

- [ ] 3.1 В internal/adapters/outbound/site вынести существующий разбор title для CSV и PostgreSQL. В CSV перевести ключи на int64, проверить числовые дубликаты, ошибки и отмену. Подтвердить baseline демонстрации и регрессии Stage 0 на неизменных синтетических fixtures.
- [ ] 3.2 В domain client определить `client.ErrInvalidSourceData` в `backend/internal/domain/client/errors.go` по образцу существующих ошибок источника. В site/postgres реализовать параметризованное чтение через SiteReader. На отдельной временной PostgreSQL с синтетической children проверить PK против id_child_fond, отсутствие фильтрации, SELECT минимальных колонок под read-only ролью, NULL title/birthday, разбор имён, смену календарного дня при разных session TimeZone, `infinity`, `-infinity` и конечные даты за пределами поддерживаемого диапазона `client.Date` (годы 1–9999). Все эти неподдерживаемые значения SHALL возвращать `client.ErrInvalidSourceData`, а не `nil` и не техническую ошибку. Также проверить not found, отмену и дедлайн.
- [ ] 3.3 В internal/platform/config, db и cmd/server подключить явный выбор режима и отдельный pool. В `config.Load()` отсутствующая `SITE_SOURCE` SHALL иметь значение `csv`, после чего выполняется условная двухфазная валидация обязательных переменных: для CSV обязателен `SITE_PATH`, для PostgreSQL — группа `SITE_DB_*`; перестроить `envReader` под такую валидацию. Обязательно покрыть default `csv` тестом `config.Load()`. Проверить раздельную валидацию, неверный режим, недоступность сайта, отсутствие fallback, закрытие pool и запуск CSV с прежней конфигурацией. Описать безопасные синтетические примеры и порядок запуска в README и .env.example.

## 4. Интеграционная проверка

- [ ] 4.1 Проверить чтение, оценку и сохранение на обеих конфигурациях. Выполнить в backend gofmt -d для изменённых Go-файлов, go test -count=1 ./..., go test -race -count=1 ./..., go vet ./... и go build ./.... Явно выполнить интеграционные тесты в двух временных синтетических БД и зафиксировать результаты, затем выполнить git diff --check.
- [ ] 4.2 После Verify выполнить независимый Code Review по docs/agent-workflow.md. Исправить подтверждённые блокирующие findings, повторить затронутые проверки и обновить PROJECT_STATUS.
