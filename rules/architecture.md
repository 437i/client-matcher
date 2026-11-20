# Architecture rules

Проект использует упрощённую Hexagonal Architecture / Ports & Adapters.

Направление зависимостей:

`HTTP adapter → Application service → Domain`

Application определяет необходимые ports; outbound adapters реализуют их. Domain не знает о transport и storage.

- `domain`: сущности, инварианты, чистые функции и доменные ошибки.
- `application/ports`: интерфейсы чтения CRM/site и хранения связей.
- `application/services`: orchestration use cases.
- `adapters/inbound/http`: маршруты, handlers, HTTP DTO и сериализация.
- `adapters/outbound`: CSV, CRM HTTP API, Site PostgreSQL, PostgreSQL repository связей.
- `platform`: конфигурация, DB pool и общие технические детали.

Перед добавлением пакета проверь, можно ли разместить код в существующем пакете без нарушения ответственности. Не создавай пакет на один файл автоматически.

Не заставляй Domain импортировать адаптеры. Не делай SQL-запросы в handlers/services. Не заставляй handlers самостоятельно сопоставлять клиентов.

При замене CSV на реальные источники меняются адаптеры и wiring в `cmd/server`; бизнес-логика и ports должны сохраняться, если контракты источников не изменились.
