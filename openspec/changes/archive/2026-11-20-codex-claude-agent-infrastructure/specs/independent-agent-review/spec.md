## ADDED Requirements

### Requirement: Модели и делегирование
Основной Codex SHALL использовать gpt-6.1-sol/medium; субагенты SHALL использовать gpt-6-luna/high, максимум 2 без вложенности. Родитель SHALL отвечать за архитектуру, задачи, интеграцию, проверки и результат. Предположения SHALL отличаться от требований.

#### Scenario: Полный контракт
- **WHEN** разрешено делегирование
- **THEN** субагент получает цель, типы, инварианты, ограничения, граничные значения, scope и критерии результата; родитель проверяет результат

### Requirement: Независимый Claude review
Claude SHALL выполнять Plan review без записи/shell/MCP и dangerously-skip-permissions; Haiku 5.5 для обычных и Sonnet 5.5 для критичных изменений SHALL использоваться после проверки точного ID. Review SHALL быть обязательным для архитектуры, business logic, API, DB, безопасности, matching и этой инфраструктуры; тривиальные локальные/documentation изменения допускают проверки Codex. Auto Memory SHALL сохраняться без Claude-Mem.

#### Scenario: Отчёт или недоступность
- **WHEN** запускается review
- **THEN** отчёт содержит findings, severity, файл/строку, сценарий, влияние, минимальное исправление, выполненные/пропущенные проверки и ограничения; недоступность CLI/model сообщается без имитации review и остановки остальных задач

### Requirement: Полная точная история Wellflow
Collector SHALL использовать Python standard library, GET https://api.wellflow.dev/v1/usage?limit=20 с cursor до null, Decimal для cost_cents, валидацию структуры, timeout, ограниченные retries 429/5xx/timeout и ошибки 401/403/JSON/schema/cursor. Ключ SHALL получаться из WELLFLOW_API_KEY, иначе OPENAI_API_KEY в CODEX_HOME/auth.json или ~/.codex/auth.json и SHALL NOT попадать в вывод, логи или snapshots.

#### Scenario: Snapshot
- **WHEN** все страницы получены
- **THEN** все записи включая дубликаты атомарно сохраняются вне Git отдельным snapshot в ~/Library/Application Support/client-matcher/wellflow-usage/; macOS каталог/файлы доступны только владельцу; неполный обход не публикуется как завершённый

### Requirement: Отчёт и тесты Collector
Collector SHALL агрегировать по модели, фильтровать [from, until), сохранять дробные cost_cents без долларов и выдуманных request_id. Тесты SHALL использовать unittest/mocks/temporary synthetic fixtures без сети/auth.

#### Scenario: Ошибки и точность
- **WHEN** проверяются пагинация, пустая история, дроби, дубликаты, повтор cursor, JSON/schema, timeout, 401/403, 429/5xx, отсутствие и утечка ключа
- **THEN** точность и дубликаты сохраняются, ошибки не публикуют snapshot и ключ не раскрывается
