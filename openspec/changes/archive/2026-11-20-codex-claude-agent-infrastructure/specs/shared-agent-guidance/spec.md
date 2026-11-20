## ADDED Requirements

### Requirement: Общие инструкции и skills
Проект SHALL использовать AGENTS.md как общий источник, минимальный CLAUDE.md импорт и .agents/skills как канонический каталог шести проектных skills. OpenSpec workflows SHALL сохраняться.

#### Scenario: Доступ трёх CLI
- **WHEN** CLI загружает safe-go-change, backend-review, openapi-swagger, frontend-implementation, frontend-review или frontend-ux
- **THEN** он получает общую процедуру и references через штатный каталог или ссылку; доступность проверяется CLI либо файловой проверкой с указанным ограничением

### Requirement: Сохранение OpenCode
Шесть commands и два reviewer agents SHALL сохранять имена и использовать общие skills. Proposal SHALL сохранять авторизацию Apply; frontend SHALL считаться существующим.

#### Scenario: Адаптеры
- **WHEN** вызывается planning, verify или review
- **THEN** процедура загружается из общего skill с соблюдением scope

### Requirement: Практическая защита данных
Инструкции и permissions SHALL допускать синтетические CSV проекта, запрещать публикацию секретов, реальные клиентские данные и destructive Git. Credentials SHALL NOT читаться без необходимости; глобальные настройки SHALL сохраняться.

#### Scenario: Synthetic fixture
- **WHEN** тест использует подтверждённые синтетические данные
- **THEN** расширение CSV не запрещает тест; секреты и реальные данные не включаются в отчёт или Git
