import { describe, expect, it } from 'vitest'

import { ApiError, messageForError } from './errors'

const forbiddenWords = [
  'endpoint',
  'API',
  'JSON',
  'HTTP',
  'payload',
  'backend',
  'frontend',
  'сервер',
  'сервис',
  'запрос',
  'мутация',
  'кэш',
  'статус-код',
]

describe('messageForError', () => {
  it('различает конфликт по клиенту основной базы и конфликт по записи сайта', () => {
    const crmConflict = messageForError(new ApiError('crm_already_linked', 409))
    const siteConflict = messageForError(new ApiError('site_already_linked', 409))

    expect(crmConflict).not.toBe(siteConflict)
    expect(crmConflict).toContain('Этот клиент основной базы уже связан с другой записью на сайте')
    expect(siteConflict).toContain('Эта запись на сайте уже связана с другим клиентом основной базы')
  })

  it('не утверждает, что текущая пара уже сохранена', () => {
    const crmConflict = messageForError(new ApiError('crm_already_linked', 409))
    const siteConflict = messageForError(new ApiError('site_already_linked', 409))

    expect(crmConflict).not.toContain('уже сохранена')
    expect(siteConflict).not.toContain('уже сохранена')
    expect(crmConflict).toContain('список сохранённых связей')
    expect(siteConflict).toContain('список сохранённых связей')
  })

  it('различает несовпадение фамилии и даты рождения', () => {
    const lastName = messageForError(new ApiError('last_name_mismatch', 422))
    const birthday = messageForError(new ApiError('birthday_mismatch', 422))

    expect(lastName).toContain('Фамилии')
    expect(birthday).toContain('Даты рождения')
    expect(lastName).not.toBe(birthday)
  })

  it('переводит известные коды в понятный текст', () => {
    expect(messageForError(new ApiError('site_client_not_found', 404))).toBe(
      'Запись с таким номером не найдена. Проверьте номер и попробуйте ещё раз.',
    )
    expect(messageForError(new ApiError('query_empty', 400))).toContain(
      'Введите имя или фамилию клиента',
    )
    expect(messageForError(new ApiError('invalid_body', 400))).toContain('Попробуйте ещё раз')
    expect(messageForError(new ApiError('internal_error', 500))).toContain('администратору')
  })

  it('объясняет сетевой сбой и неизвестный код', () => {
    expect(messageForError(new TypeError('Failed to fetch'))).toContain('Не удалось получить данные')
    expect(messageForError(new ApiError('что-то новое', 500))).toBe(
      'Что-то пошло не так. Попробуйте ещё раз.',
    )
  })

  it('не использует технические слова ни в одном сообщении', () => {
    const codes = [
      'invalid_query',
      'query_empty',
      'invalid_body',
      'id_empty',
      'crm_client_not_found',
      'site_client_not_found',
      'last_name_mismatch',
      'birthday_mismatch',
      'crm_already_linked',
      'site_already_linked',
      'invalid_source_client_data',
      'internal_error',
      'request_canceled',
      'timeout',
      'link_not_found',
      'service_unavailable',
    ]

    for (const code of codes) {
      const message = messageForError(new ApiError(code, 400))
      for (const word of forbiddenWords) {
        // Регистр не учитывается: запрещено и «Запрос», и «запрос».
        expect(
          message.toLowerCase(),
          `код ${code} содержит слово «${word}»`,
        ).not.toContain(word.toLowerCase())
      }
    }
  })

  it('объясняет прерванное действие без технического слова', () => {
    const message = messageForError(new ApiError('request_canceled', 499))

    expect(message).toContain('Действие было прервано')
    expect(message.toLowerCase()).not.toContain('запрос')
  })

  it('объясняет временную недоступность данных', () => {
    const message = messageForError(new ApiError('service_unavailable', 502))

    expect(message).toContain('Данные временно недоступны')
  })

  it('предлагает перевыбрать клиента, когда клиента в основной базе нет', () => {
    const message = messageForError(new ApiError('crm_client_not_found', 404))

    expect(message).toContain('Повторите поиск и выберите клиента заново')
    expect(message).not.toContain('Проверьте данные')
  })

  it('предлагает ввести имя или фамилию клиента', () => {
    expect(messageForError(new ApiError('invalid_query', 400))).toContain(
      'Введите имя или фамилию клиента',
    )
  })
})
