# Запуск Client Matcher через SSH

Production использует отдельные SSH-сессии и PostgreSQL-пулы для SITE и OD. SITE — PostgreSQL **10.23**, OD — PostgreSQL **16.15**. Stage 4 добавляет CRM-источник Bitrix24. Для будущей проверки на портале потребуется отдельная чистая OD со штатными миграциями. [Адаптер и чеклист](bitrix24-adapter.md)

## Переменные и файлы

Передайте значения через окружение запуска или через отдельный файл --env-file вне репозитория. Значения с паролями и файлы с ключами храните вне обоих build contexts.

| Переменная | Значение |
| --- | --- |
| SITE_SSH_HOST, OD_SSH_HOST | Имя или IP соответствующего SSH-сервера без порта. Подключение использует TCP 22. |
| SITE_SSH_USER, OD_SSH_USER | Пользователь соответствующего SSH-сервера. |
| SITE_SSH_PASSWORD | Пароль SSH пользователя SITE. |
| SITE_HOST, OD_HOST | Оба значения строго localhost. На каждом SSH-сервере приложение открывает TCP 5432. |
| SITE_NAME, OD_NAME | Названия баз данных PostgreSQL. |
| SITE_USER, OD_USER | Пользователи PostgreSQL. Goose работает под OD_USER. |
| SITE_PASSWORD, OD_PASSWORD | Пароли PostgreSQL. |
| OD_SSH_KEY | Абсолютный путь на хосте к незашифрованному OD SSH-ключу. В контейнере /run/client-matcher/od-key. |
| SSH_KNOWN_HOSTS_PATH | Абсолютный путь к доверенному known_hosts на хосте. В контейнере /run/client-matcher/known_hosts. |
| BITRIX24_WEBHOOK_URL | Полный HTTPS URL классического webhook. Передаётся только backend как секрет из окружения или внешнего файла. |
| FRONTEND_PORT | Порт frontend на 127.0.0.1, по умолчанию 3000. |

Compose передаёт backend режимы DB_TRANSPORT=ssh, SITE_SOURCE=postgres и CRM_SOURCE=bitrix24. Файлы OD-ключа и known_hosts монтируются только для чтения. Права на хосте должны позволять пользователю app в контейнере читать эти файлы (проверьте UID и GID командой id в контейнере). Для диагностики используйте приведённые ниже команды. Полный вывод интерполированного Compose и окружения контейнера раскрывает секреты.

Пример формата known_hosts с фиктивными ключами. Используйте соответствующие проверенные ключи команды деплоя.

~~~text
site.example.invalid ssh-ed25519 <проверенный-base64-ключ-SITE>
od.example.invalid ssh-ed25519 <проверенный-base64-ключ-OD>
~~~

Команда деплоя настраивает разрешение localhost в loopback каждого SSH-сервера, разрешает SSH-проброс к localhost:5432 и обеспечивает доступ к PostgreSQL по этому локальному TCP-соединению без SSL. SITE_USER располагает USAGE на public и SELECT на public.children(id,title,birthday). В OD нужна заранее созданная схема `apps`. При первом подключении в ней либо ещё нет объектов приложения, либо их происхождение подтверждается совместимым журналом тех же Goose-миграций. Команда деплоя проверяет происхождение объектов и настраивает доступ. Роли `OD_USER` нужны `USAGE` и `CREATE` на `apps` и доступ к `apps.goose_db_version`. Если ожидающие миграции изменяют существующие объекты, выполняющей DDL роли нужны права владельца этих объектов. Встроенный Goose применяет миграции только к OD.

## Запуск и проверка

~~~sh
docker compose --env-file /absolute/path/client-matcher-production.env -f docker-compose.prod.yml config --services
docker compose --env-file /absolute/path/client-matcher-production.env -f docker-compose.prod.yml up --build -d --wait
docker compose --env-file /absolute/path/client-matcher-production.env -f docker-compose.prod.yml exec -T backend sh -c 'id && test -r "$OD_SSH_KEY" && test -r "$SSH_KNOWN_HOSTS_PATH" && wget -qO- http://127.0.0.1:8080/healthz'
~~~

Backend проверяет соединения с обеими БД и выполняет Goose Up. Затем через тот же OD-пул под `OD_USER` он проверяет наличие `apps.client_links` и эффективные права `SELECT` на колонки чтения и `RETURNING` и `INSERT` на колонки связи и аудита. Достаточно соответствующих колоночных прав. Проверка выполняется без записи при каждом старте, в том числе с уже мигрированной схемой. При ошибке запуск завершается до открытия HTTP. После проверки хранилища backend ещё раз проверяет соединения, включает восстановление SSH-сессий и открывает HTTP.

Откройте http://127.0.0.1:3000. После запуска /healthz проверяет OD. SITE проверяется чтением карточки по известному ID. В production Compose публикуется только frontend. Доступ к развёртыванию с реальными данными ограничивает команда деплоя.

Для остановки используйте тот же файл Compose и команду stop. Backend получает до 10 секунд на завершение активных HTTP-запросов. Compose выделяет процессу 20 секунд суммарно.

Локальный запуск из корня репозитория использует docker compose up --build. Он поднимает локальную PostgreSQL, отдельный мигратор и два приложения с синтетическими CSV из testdata. Для параллельного запуска назначьте разные порты frontend.
