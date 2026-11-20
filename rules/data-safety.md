# Data safety rules

- Пользователь подтвердил: CSV проекта и fixtures синтетические и допустимы. Реальных клиентских данных и production DB доступа нет.
- Не добавляй реальные клиентские данные; для неизвестных экспортов уточняй происхождение до чтения.
- Не читай .env, credentials, auth.json, токены и ключи без необходимости; .env.example допускается. Не публикуй и не коммить секреты, не включай их в вывод, логи и отчёты.
- scripts/wellflow_usage.py получает ключ самостоятельно только при отдельном запуске пользователем: WELLFLOW_API_KEY, иначе OPENAI_API_KEY из CODEX_HOME/auth.json или ~/.codex/auth.json. Это не разрешает агенту исследовать реальные credentials.
- Тесты Collector используют mocks и временные synthetic fixtures без сети и настоящего auth.json. Реальные БД не подключаются.
- Snapshots остаются вне Git в ~/Library/Application Support/client-matcher/wellflow-usage/; ключ в них не сохраняется.
- Не меняй исходные пользовательские данные и миграции без требования; destructive Git/Docker, commit/push требуют отдельной авторизации.
