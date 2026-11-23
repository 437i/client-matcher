# Spec Delta

## MODIFIED Requirements

### Requirement: Выбор источника сайта
Приложение SHALL поддерживать сочетания `direct + csv`, `direct + postgres` и `ssh + postgres`. По умолчанию `DB_TRANSPORT=direct` и `SITE_SOURCE=csv`. Режим транспорта общий для обеих БД, PostgreSQL сайта имеет отдельные настройки и пул. Ошибка настройки или подключения SHALL останавливать запуск выбранного режима.

#### Scenario: Демонстрация
- **WHEN** режим не задан или выбран CSV при локальном запуске
- **THEN** существующая демонстрация работает без настроек и соединения с PostgreSQL сайта

#### Scenario: Режим PostgreSQL
- **WHEN** явно выбран PostgreSQL сайта
- **THEN** CSV сайта не требуется, подключение сайта отдельно от собственной БД и при ошибке отсутствует автоматический переход на CSV

#### Scenario: Прямая локальная проверка PostgreSQL
- **WHEN** вне production явно задан SITE_SOURCE=postgres и настройки SITE_DB_*
- **THEN** приложение читает сайт через прямое PostgreSQL-подключение без SSH-настроек

#### Scenario: PostgreSQL в production
- **WHEN** явно выбран production-транспорт SSH и SITE_SOURCE=postgres
- **THEN** SiteReader подключается через туннель SITE с production-переменными SITE_*

#### Scenario: Недопустимая конфигурация
- **WHEN** указан неизвестный транспорт или источник сайта, либо выбран `ssh` с явным или подразумеваемым `csv`
- **THEN** backend отклоняет конфигурацию до подключения к источникам и запуска HTTP
