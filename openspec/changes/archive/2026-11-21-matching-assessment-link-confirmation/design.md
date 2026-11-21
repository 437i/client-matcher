# Design

## Context

Текущий ClientLinkService читает обе записи, вызывает строгий MatchClients и сохраняет ID. Domain Date не nullable; SiteClient содержит ShortName и LastName без отдельного отчества. Link port имеет только Create/List. Уникальность обеспечивают два PostgreSQL constraints. См. proposal.md и docs/roadmap.md.

## Goals / Non-Goals

Goals: объяснимая оценка, контролируемое подтверждение, актуальность, атомарный аудит и сохранение unique constraints.
Non-Goals: алгоритм поиска и ranking не менять; расширение загрузки missing данных явно входит в Stage 1, но baseline 200/55 должен сохраняться; Site PostgreSQL, SSH, Bitrix, frontend, auth, смена string IDs и внешняя схема вне scope.

## Domain Concepts

- SourceRecord: ID, структурированные доступные компоненты имени и nullable календарная дата. Отсутствие не равно некорректному значению.
- Assessment: status, ordered field checks, confirmation_required; не утверждает идентичность.
- FieldCheck: field и code exact/normalized_match/alias_match/probable_match/mismatch/missing. Значения возвращаются только текущему caller, не копируются в audit.
- Confirmation: confirmed=true и trimmed reason 1–1000 Unicode символов для requires_confirmation.
- Freshness: непросроченное подтверждение тех же ID, исходных полей и версии правил.
- LinkDecision: данные связи и минимальный audit, сохраняемые атомарно.

## Decisions

### Чистая оценка и источники

Domain matcher становится чистой функцией без I/O. Сравниваются фамилия и имя отдельно; Site patronymic в текущем порте всегда unknown: не выделять его из title и не сравнивать с CRM отчеством. Существующее многословное LastName сайта сохранять как фамилию без эвристического разбиения. NFC/регистр/пробелы/ё-е — harmless; алиас, homoglyph или fuzzy всегда требуют confirmation. Составные компоненты сравниваются без перестановки ролей фамилии/имени. Отсутствующее имя/фамилия или дата требуют confirmation. Непустая некорректная дата — 502 invalid_source_client_data без assessment/checks/token, не missing; CSV malformed rows остаются пропущенными, поэтому GetByID для них даёт not found.

Использовать *Date для отсутствующей даты в domain source models; nil сериализуется birthday:null. CSV blank birthday загружается как nil; malformed nonblank по-прежнему invalid. Отсутствующее отчество допустимо. Одночастное или пустое title хранится с явным UnstructuredName и неизвестными ролями компонентов: не назначать единственное слово фамилией или именем. Оценка имени missing требует confirmation. Для поиска UnstructuredName токенизируется как неструктурированный текст без given-name aliases; пустое title не даёт поисковых токенов, но запись доступна GetByID. Это ограниченное расширение source completeness, а не новый алгоритм. Нынешние полные fixtures сохраняют поведение. Ошибка/отсутствие источника обрабатывается application, а не чистым matcher.

### Stateless token

Application выдаёт base64url JSON claims + HMAC-SHA256: token_version=1, rule_version (константа в domain/matcher), crm_id, site_id, keyed-source fingerprint, issued_at, expires_at (5 минут). Ключ 32 случайных байта создаётся в composition root при запуске; рестарт инвалидирует tokens. Не нужны новая таблица assessments или глобальный secret. Token codec — application port с локальным crypto adapter; Domain не подписывает tokens. Fingerprint = HMAC-SHA256 отдельным подключом, полученным domain-separated HMAC от runtime key. Вход — фиксированный порядок полей с именами, length-prefixed UTF-8 значениями и отдельными nil/present markers; простая конкатенация запрещена. Подпись token использует другой domain-separated подключ. Fingerprint включает точные поля имени/даты и IDs, включая nil, а не только нормализацию: любое изменение данных требует reassessment. Token не содержит исходных ФИО/дат или открытых SHA-хешей этих полей, не логируется; keyed digest не позволяет проверять словарные догадки без runtime key. Подпись проверяется constant-time, размер token ограничен 4096 bytes, clock внедряется для тестов. Альтернатива server store отвергнута как лишнее хранение; shared key/multi-instance относится к будущей эксплуатации.

### Workflow и HTTP

POST /api/v1/link-assessments принимает crm_id/site_id (непустые строки), читает обе записи, проверяет занятость через новый port LookupBySourceIDs и возвращает 200 с status/checks/confirmation_required/token/expires_at. Некорректный ID — 400; missing source или занятый ID — 200 blocked с checks=[], confirmation_required=false, blocked_reasons (crm_not_found/site_not_found/crm_already_linked/site_already_linked), token=null и expires_at=null. Для ready/requires_confirmation blocked_reasons=[]; source timeout — 504, canceled — 499, invalid source — 502, прочие — 500.

POST /api/v1/links сохраняет crm_id/site_id; добавляет assessment_token, confirmed, reason. Token если передан проверяется всегда. Сначала token syntax/signature/expiry/IDs, затем повторное чтение и fingerprint; изменение → 409 assessment_stale. Requires_confirmation без token или confirmed → 422 confirmation_required; пустая/слишком длинная причина → 400 invalid_confirmation. Invalid token → 400 invalid_assessment_token; expired → 409 assessment_expired. Ready без token сохраняет прежнюю совместимость, но лишь после свежей оценки. Отсутствие источника при create сохраняет 404, занятый ID — 409. Успешный create сохраняет 201 и прежние LinkResponse поля; list/export не расширяются PII. Check values в assessment — только данные текущего ответа. Old frontend с mismatched pair получит безопасную ошибку, без автоматического создания.

### Атомарность и конкуренция

Расширить client_links новой Goose-миграцией: manually_confirmed boolean NOT NULL DEFAULT false, confirmation_reason nullable text, confirmation_codes text[] NOT NULL DEFAULT '{}', confirmed_at nullable timestamptz, assessment_rule_version integer. Хранить аудит в строке связи одним INSERT RETURNING: нет отдельной транзакционной абстракции. DB CHECK: manual=true требует reason с char_length(btrim(reason)) BETWEEN 1 AND 1000, confirmed_at NOT NULL, rule_version>0 и cardinality(confirmation_codes)>0. Codes — field:code из белого списка name:alias_match/name:probable_match/name:mismatch/name:missing/birthday:mismatch/birthday:missing. Manual=false требует reason/time NULL и пустые codes; старым строкам rule_version NULL допустим. Не хранить token, fingerprint, ФИО/дату. Для ready причина не сохраняется. Старые записи defaults совместимы. Employee identity откладывается до auth. Unique constraints остаются неизменны; Lookup advisory, INSERT — окончательный arbiter race. Два конкурентных insert: один success, другой конфликт. Повторное чтение непосредственно перед insert не создаёт атомарную транзакцию с внешними источниками; гарантия — последняя полученная версия, без обещания заблокировать внешний источник.

## Risks / Trade-offs

- Raw fingerprints чувствительны к косметическим изменениям → безопасный reassessment, без скрытого принятия.
- Рестарт/другая replica инвалидирует token → 400 invalid token и новая оценка; multi-instance не поддерживается этой локальной версией.
- Nullable birthday меняет response schema → OpenAPI и backend contract tests; frontend не меняется и для нынешнего полного demo не получает null.
- Reason может содержать PII → документация просит не вводить ФИО/даты; backend не логирует body/reason/token. Не обещать автоматическое распознавание PII.

## Migration Plan

Новая additive Goose migration собственной БД, старые миграции не менять. Тесты Up/Down на synthetic временной PostgreSQL. Rollback приложения возможен с extra columns; Down удаляет новый audit и требует осознанного принятия потери audit, поэтому не запускать автоматически. Сначала backup в реальной эксплуатации; сейчас production не подключается.

## Open Questions

Site ID mapping, timezone semantics public.children, publication filters, Bitrix fields и employee identity относятся к следующим этапам; текущие string IDs и CSV не требуют ответа на эти вопросы.

### Дополнительные контрактные решения после Plan Review

- Invalid ID возвращает 400 invalid_id без Assessment; invalid source возвращает 502 invalid_source_client_data без Assessment. Blocked reasons относятся только к известным бизнес-блокировкам; source outage — техническая ошибка.
- Probable использует существующие правила name_search: смешанная кириллица/латиница, edit distance 1 с соседней перестановкой при обоих словах от 5 букв; prefix/substring не являются согласованным ФИО и требуют confirmation. Алиасы переиспользуют nameAliasGroups только для given name. Не создавать второй словарь.
- Приоритет create: JSON/body size/ID → переданный token syntax/signature/expiry/IDs → повторное чтение/invalid/not-found → fingerprint/rule_version stale → occupied lookup → confirmation/reason → INSERT с unique checks. Ready с confirmed=true или непустой reason возвращает 400 invalid_confirmation; отсутствующие/false confirmed и пустая reason допустимы. HTTP body limit 16 KiB, превышение — 413 request_too_large.
- rule_version отличается от token_version: изменение правил даёт assessment_stale, неизвестный token_version — invalid_assessment_token. После рестарта invalid token сопровождается безопасным сообщением «Повторите проверку пары».
- В PGLinkRepo linkColumns/RETURNING остаются прежними; audit columns пишутся, но не попадают в прежний RowToStructByName DTO. При одновременном конфликте обоих unique constraints допускается любой соответствующий conflict code.

### Единые поля, агрегация и ошибки портов

FieldCheck.field — enum last_name, first_name, birthday; порядок checks фиксирован именно так. Site patronymic неизвестно и не создаёт отдельный check. Для каждого поля code — exact, normalized_match, alias_match, probable_match, mismatch или missing. alias_match допустим только для first_name; probable_match означает предусмотренный homoglyph/fuzzy, а prefix/substring без полного согласования компонента получает mismatch. Missing имеет приоритет над сравнением значений этого поля.

Если есть бизнес-блокировка, ответ blocked с пустыми checks. Иначе любой missing/alias_match/probable_match/mismatch даёт requires_confirmation; только exact/normalized_match по всем трём полям даёт ready. Проверки имени выполняются раздельно, но аудит хранит укрупнённые коды: last_name/first_name с code alias_match/probable_match/mismatch/missing отображаются в name:code; birthday — в birthday:code. Коды дедуплицируются и сортируются лексикографически. Exact/normalized_match в confirmation_codes не входят. Таким образом полевая детализация ответа сохраняется, whitelist DB не расходится с DTO.

Ports используют доменную sentinel client.ErrInvalidSourceData для недопустимого непустого значения источника; application проверяет errors.Is и возвращает 502 invalid_source_client_data. CSV malformed строки по-прежнему пропускает, поэтому этот 502 сценарий проверяется mock-портом, а не обещается для пропущенной CSV-строки.

Assessment: сначала validate ID, затем чтение CRM, затем Site. Техническая ошибка прекращает обработку; отсутствующие записи собираются в blocked_reasons (CRM перед Site). Если есть not-found, advisory lookup не выполняется. Если обе записи доступны, lookup возвращает все конфликты (CRM перед Site); при наличии конфликтов checks пусты и token отсутствует. Иначе выполняется domain assessment. Это задаёт приоритет missing-source над occupied-ID без лишних запросов.
