import '@testing-library/jest-dom/vitest'

/**
 * В браузере относительный адрес запроса разрешается относительно страницы,
 * а Node-реализация Request требует абсолютный. Приложение обращается к
 * относительным путям `/api/v1/...`, поэтому в тестах адрес дополняется
 * тестовым хостом — так проверяется ровно тот путь, который уйдёт на сервер.
 */
const NativeRequest = globalThis.Request

class TestRequest extends NativeRequest {
  constructor(input: RequestInfo | URL, init?: RequestInit) {
    super(typeof input === 'string' ? new URL(input, 'http://localhost').toString() : input, init)
  }
}

Object.defineProperty(globalThis, 'Request', {
  value: TestRequest,
  configurable: true,
  writable: true,
})
