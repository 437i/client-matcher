/**
 * Единый формат ошибок интерфейса.
 *
 * Коды и статусы сверены с backend:
 * backend/internal/adapters/inbound/http/handlers/response.go,
 * handlers/client_handler.go, handlers/link_handler.go и
 * backend/internal/adapters/inbound/http/router_test.go.
 *
 * Пользователю показывается только понятный текст: коды ответов, трассировки
 * стека и внутренние детали на экран не выводятся.
 */

export class ApiError extends Error {
  readonly code: string
  readonly status: number

  constructor(code: string, status: number) {
    super(code)
    this.name = 'ApiError'
    this.code = code
    this.status = status
  }
}

/**
 * Сообщения для кодов, которые реально возвращает backend, и для
 * frontend-состояния `service_unavailable`: его использует клиент, когда
 * ответ пришёл без JSON (страница шлюза при недоступном backend) и код
 * ошибки backend недоступен.
 */
const messagesByCode: Record<string, string> = {
  invalid_query: 'Введите имя или фамилию клиента и попробуйте ещё раз.',
  query_empty: 'Введите имя или фамилию клиента — были введены только пробелы.',
  invalid_body: 'Что-то пошло не так при сохранении. Попробуйте ещё раз.',
  id_empty: 'Введите номер записи без пробелов.',
  crm_client_not_found:
    'Клиент с таким номером не найден в основной базе. Повторите поиск и выберите клиента заново.',
  site_client_not_found:
    'Запись с таким номером не найдена. Проверьте номер и попробуйте ещё раз.',
  last_name_mismatch:
    'Фамилии в основной базе и на сайте не совпадают. Проверьте, что номер записи на сайте принадлежит этому человеку.',
  birthday_mismatch:
    'Даты рождения в основной базе и на сайте не совпадают. Проверьте, что номер записи на сайте принадлежит этому человеку.',
  crm_already_linked:
    'Этот клиент основной базы уже связан с другой записью на сайте. Посмотрите список сохранённых связей — нужная связь, возможно, уже есть.',
  site_already_linked:
    'Эта запись на сайте уже связана с другим клиентом основной базы. Посмотрите список сохранённых связей — нужная связь, возможно, уже есть.',
  invalid_source_client_data:
    'В основной базе или на сайте некорректные данные клиента. Сообщите об этом администратору.',
  internal_error:
    'Что-то пошло не так. Попробуйте чуть позже или сообщите администратору.',
  request_canceled: 'Действие было прервано. Попробуйте ещё раз.',
  timeout: 'Ответ не поступил вовремя. Попробуйте ещё раз.',
  link_not_found: 'Связь не найдена. Обновите список сохранённых связей.',
  service_unavailable: 'Данные временно недоступны. Попробуйте повторить действие.',
}

const networkMessage =
  'Не удалось получить данные. Проверьте подключение и попробуйте ещё раз.'

const unknownMessage = 'Что-то пошло не так. Попробуйте ещё раз.'

/**
 * Возвращает текст для сотрудника. Код ошибки и HTTP-статус наружу не
 * показываются: по ним ничего понять нельзя, а различие между ситуациями
 * уже заложено в тексты.
 */
export function messageForError(error: unknown): string {
  if (error instanceof ApiError) {
    return messagesByCode[error.code] ?? unknownMessage
  }
  if (error instanceof TypeError) {
    // fetch не смог связаться с сервисом: это самая частая причина отказа.
    return networkMessage
  }
  return unknownMessage
}
