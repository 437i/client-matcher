# Spec Delta

## ADDED Requirements

### Requirement: Отдельный production Compose

Production Compose SHALL содержать ровно backend и frontend. Backend SHALL запускать SSH-подключения и миграции внутри своего процесса. Frontend SHALL использовать существующий прокси API по сети Compose. Backend SHALL получать файл закрытого ключа OD, known_hosts и CSV CRM через read-only mounts во время работы контейнера.

#### Scenario: Production-конфигурация

- **WHEN** запускается отдельный production Compose
- **THEN** frontend обслуживает UI и проксирует API на backend, который работает с удалёнными БД; сервисы PostgreSQL и мигратора отсутствуют, порт backend доступен только в сети Compose, а host-порт frontend по умолчанию привязан к 127.0.0.1

#### Scenario: Доступ к SSH-файлам в контейнере

- **WHEN** backend запущен с read-only mounts для синтетического закрытого ключа OD и known_hosts
- **THEN** пользователь `app` открывает оба файла на чтение по контейнерным путям из `OD_SSH_KEY` и `SSH_KNOWN_HOSTS_PATH`

#### Scenario: Сборка образов

- **WHEN** собираются образы production Compose
- **THEN** backend image содержит SQL-миграции, а образы и их build context исключают runtime-файлы с закрытыми ключами, секретами и CSV CRM из production

#### Scenario: Обычный локальный запуск

- **WHEN** выполняется docker compose up --build с локальной конфигурацией по умолчанию
- **THEN** локальная БД, существующий сервис миграций, backend с CSV-источниками и frontend запускаются без production-переменных и SSH-файлов
