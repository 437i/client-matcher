# client-matcher

Сопоставление ID клиента в CRM (источник 1) и в базе сайта (источник 2).
Пары ID хранятся в PostgreSQL и выгружаются в CSV (`crm_id,site_id`).

## Запуск

```bash
cp .env.example .env          # задайте DB_PASSWORD
docker compose up --build
```

Положите CSV-источники в `data/` как `crm.csv` (`id,title,birthday,clinic`) и
`site.csv` (`id,title,birthday`). Приложение: http://localhost:8080,
миграции применяет сервис `migrate`. Данные в БД переживают перезапуск (volume `pgdata`).

Без Docker: поднимите PostgreSQL, примените `backend/migrations` (goose), задайте
`DB_HOST DB_PORT DB_USER DB_PASSWORD DB_NAME [DB_SSLMODE] CRM_PATH SITE_PATH [PORT]`
и выполните `go run ./cmd/server` в `backend/`.

## API (`/api/v1`)

| Метод | Путь | Назначение |
|---|---|---|
| GET | `/crm-clients?q=иванов ив` | поиск в CRM по имени (слова в любом порядке, по началу слова, ё=е), до 50 записей |
| GET | `/site-clients/:id` | клиент сайта по ID (показать редактору до сохранения) |
| POST | `/links` `{"crm_id","site_id"}` | сохранить связь: 201 / 404 / 409 / 422 |
| GET | `/links` | все связи |
| GET | `/links/export.csv` | CSV с колонками `crm_id,site_id` |
| GET | `/healthz` | проверка живости и БД |

Ошибки: `{"code": "...", "message": "..."}`; `code` стабилен и предназначен для фронтенда.

## Допущения

- Связь 1:1 (`UNIQUE` на `crm_id` и на `site_id`).
- Связь допускается, только если совпали фамилия (без учёта регистра и ё/е) и дата рождения.
- Битые строки CSV пропускаются с предупреждением в логе; дубликат ID в источнике останавливает запуск.
