import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'

import {
  baseRoutes,
  deferred,
  installFetchMock,
  jsonResponse,
  lookupSiteRecord,
  renderApp,
  searchForCrm,
  selectFirstCandidate,
  siteClientB,
  siteClientD,
  stepCard,
  waitForSiteRecord,
} from '@/test/utils'

/** Ждёт, пока поиск записи на сайте завершится. */
async function waitForLookupFinished() {
  await waitFor(() =>
    expect(screen.getByRole('button', { name: 'Найти запись на сайте' })).toBeInTheDocument(),
  )
}

describe('актуальность поиска записи на сайте', () => {
  it('изменение номера позволяет начать новый поиск, а поздний ответ прежнего игнорируется', async () => {
    const user = userEvent.setup()
    const gateA = deferred<void>()
    const gateB = deferred<void>()
    const { calls } = installFetchMock(
      baseRoutes([
        {
          method: 'GET',
          path: '/api/v1/site-clients/3586',
          handler: async () => {
            await gateA.promise
            return jsonResponse(siteClientB)
          },
        },
        {
          method: 'GET',
          path: '/api/v1/site-clients/3587',
          handler: async () => {
            await gateB.promise
            return jsonResponse(siteClientD)
          },
        },
      ]),
    )
    renderApp()

    await searchForCrm(user)
    await selectFirstCandidate(user)
    await lookupSiteRecord(user)

    // Номер меняется, пока первый поиск ещё выполняется.
    const input = screen.getByLabelText('Номер записи на сайте')
    await user.clear(input)
    await user.type(input, '3587')

    // Поиск нового номера доступен, хотя прежний ещё выполняется.
    const lookupButton = screen.getByRole('button', { name: 'Найти запись на сайте' })
    expect(lookupButton).toBeEnabled()
    await user.click(lookupButton)

    // Ответ второго поиска применяется.
    gateB.resolve()
    await waitForSiteRecord()
    const step = stepCard(/Найдите его запись на сайте/)
    expect(within(step).getByText('Маша')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Сохранить связь' })).toBeEnabled()

    // Ответ первого поиска не меняет текущую пару.
    gateA.resolve()
    await waitFor(() =>
      expect(calls.filter((call) => call.pathname === '/api/v1/site-clients/3586')).toHaveLength(1),
    )
    expect(within(step).getByText('Маша')).toBeInTheDocument()
    expect(within(step).queryByText('Ваня')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Сохранить связь' })).toBeEnabled()
  })
})

describe('запоздавшие ответы поиска записи на сайте', () => {
  it('ответ первого поиска не восстанавливает пару после смены клиента A → C → A', async () => {
    const user = userEvent.setup()
    const gate = deferred<void>()
    const { calls } = installFetchMock(
      baseRoutes([
        {
          method: 'GET',
          path: '/api/v1/site-clients/3586',
          handler: async () => {
            await gate.promise
            return jsonResponse(siteClientB)
          },
        },
      ]),
    )
    renderApp()

    await searchForCrm(user)
    await selectFirstCandidate(user)
    await lookupSiteRecord(user)

    // Пока идёт поиск, сотрудник меняет клиента на C, потом возвращается к A.
    await user.click(screen.getByRole('button', { name: 'Изменить выбор' }))
    let buttons = await screen.findAllByRole('button', { name: 'Выбрать этого клиента' })
    await user.click(buttons[1] as HTMLElement)
    await user.click(screen.getByRole('button', { name: 'Изменить выбор' }))
    buttons = await screen.findAllByRole('button', { name: 'Выбрать этого клиента' })
    await user.click(buttons[0] as HTMLElement)

    gate.resolve()
    await waitForLookupFinished()

    expect(calls.some((call) => call.pathname === '/api/v1/site-clients/3586')).toBe(true)

    const step = stepCard(/Найдите его запись на сайте/)
    expect(within(step).queryByText('База сайта')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Сохранить связь' })).toBeDisabled()
  })

  it('ответ первого поиска не применяется после изменения номера записи', async () => {
    const user = userEvent.setup()
    const gate = deferred<void>()
    installFetchMock(
      baseRoutes([
        {
          method: 'GET',
          path: '/api/v1/site-clients/3586',
          handler: async () => {
            await gate.promise
            return jsonResponse(siteClientB)
          },
        },
      ]),
    )
    renderApp()

    await searchForCrm(user)
    await selectFirstCandidate(user)
    await lookupSiteRecord(user)

    const input = screen.getByLabelText('Номер записи на сайте')
    await user.clear(input)
    await user.type(input, '3587')

    gate.resolve()
    await waitForLookupFinished()

    const step = stepCard(/Найдите его запись на сайте/)
    expect(within(step).queryByText('База сайта')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Сохранить связь' })).toBeDisabled()
  })

  it('результат последнего запроса применяется', async () => {
    const user = userEvent.setup()
    const gate = deferred<void>()
    installFetchMock(
      baseRoutes([
        {
          method: 'GET',
          path: '/api/v1/site-clients/3587',
          handler: async () => {
            await gate.promise
            return jsonResponse({ id: '3587', short_name: 'Маша', last_name: 'Иванова' })
          },
        },
      ]),
    )
    renderApp()

    await searchForCrm(user)
    await selectFirstCandidate(user)
    await lookupSiteRecord(user, '3587')

    gate.resolve()
    await waitForLookupFinished()
    await waitForSiteRecord()

    const step = stepCard(/Найдите его запись на сайте/)
    expect(within(step).getByText('Маша')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Сохранить связь' })).toBeEnabled()
  })

  it('ошибка первого поиска не заменяет состояние после смены клиента', async () => {
    const user = userEvent.setup()
    const gate = deferred<void>()
    installFetchMock(
      baseRoutes([
        {
          method: 'GET',
          path: '/api/v1/site-clients/3586',
          handler: async () => {
            await gate.promise
            return jsonResponse({ code: 'site_client_not_found', message: 'x' }, 404)
          },
        },
      ]),
    )
    renderApp()

    await searchForCrm(user)
    await selectFirstCandidate(user)
    await lookupSiteRecord(user)

    // Пока идёт поиск, сотрудник меняет клиента основной базы.
    await user.click(screen.getByRole('button', { name: 'Изменить выбор' }))
    const buttons = await screen.findAllByRole('button', { name: 'Выбрать этого клиента' })
    await user.click(buttons[1] as HTMLElement)

    gate.resolve()
    await waitForLookupFinished()

    const step = stepCard(/Найдите его запись на сайте/)
    // Ошибка устаревшего поиска не показана и пара не восстановлена.
    expect(within(step).queryByText('Запись с таким номером не найдена.')).not.toBeInTheDocument()
    expect(within(step).queryByText('База сайта')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Сохранить связь' })).toBeDisabled()
  })
})

describe('защита после сохранения связи', () => {
  it('повторное сохранение той же пары не отправляет второй запрос', async () => {
    const user = userEvent.setup()
    const { calls } = installFetchMock(baseRoutes())
    renderApp()

    await searchForCrm(user)
    await selectFirstCandidate(user)
    await lookupSiteRecord(user)
    await waitForSiteRecord()

    await user.click(screen.getByRole('button', { name: 'Сохранить связь' }))
    await screen.findByText(
      'Готово. Связь сохранена: клиент основной базы № 1352 и запись на сайте № 3586.',
    )

    expect(calls.filter((call) => call.method === 'POST')).toHaveLength(1)

    // Повторное нажатие на заблокированную кнопку ничего не отправляет.
    const saveButton = screen.getByRole('button', { name: 'Сохранить связь' })
    expect(saveButton).toBeDisabled()
    await user.click(saveButton)

    expect(calls.filter((call) => call.method === 'POST')).toHaveLength(1)
    expect(
      screen.getByText('Эта связь уже сохранена. Повторно сохранять её не нужно.'),
    ).toBeInTheDocument()
  })

  it('подтверждение называет именно сохранённую пару и исчезает при её изменении', async () => {
    const user = userEvent.setup()
    installFetchMock(baseRoutes())
    renderApp()

    await searchForCrm(user)
    await selectFirstCandidate(user)
    await lookupSiteRecord(user)
    await waitForSiteRecord()

    await user.click(screen.getByRole('button', { name: 'Сохранить связь' }))
    const message = await screen.findByText(
      'Готово. Связь сохранена: клиент основной базы № 1352 и запись на сайте № 3586.',
    )
    expect(message).toBeInTheDocument()

    // Смена клиента убирает подтверждение: на экране уже другая пара.
    await user.click(screen.getByRole('button', { name: 'Изменить выбор' }))
    const buttons = await screen.findAllByRole('button', { name: 'Выбрать этого клиента' })
    await user.click(buttons[1] as HTMLElement)

    expect(
      screen.queryByText(
        'Готово. Связь сохранена: клиент основной базы № 1352 и запись на сайте № 3586.',
      ),
    ).not.toBeInTheDocument()
  })

  it('во время сохранения клиента и номер записи менять нельзя', async () => {
    const user = userEvent.setup()
    const gate = deferred<void>()
    const { calls } = installFetchMock(
      baseRoutes([
        {
          method: 'POST',
          path: '/api/v1/links',
          handler: async () => {
            await gate.promise
            return jsonResponse({ id: 'x', crm_id: '1352', site_id: '3586' }, 201)
          },
        },
      ]),
    )
    renderApp()

    await searchForCrm(user)
    await selectFirstCandidate(user)
    await lookupSiteRecord(user)
    await waitForSiteRecord()

    await user.click(screen.getByRole('button', { name: 'Сохранить связь' }))
    await screen.findByRole('button', { name: 'Сохраняем связь…' })

    // Пока запрос не завершён, выбор клиента и номер записи заблокированы.
    expect(screen.getByRole('button', { name: 'Изменить выбор' })).toBeDisabled()
    expect(screen.getByLabelText('Номер записи на сайте')).toBeDisabled()

    gate.resolve()
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Сохранить связь' })).toBeInTheDocument(),
    )

    expect(calls.filter((call) => call.method === 'POST')).toHaveLength(1)
    // Выбранный клиент не изменился.
    const step = stepCard(/Выберите клиента/)
    expect(within(step).getByText('Иванов Иван Бек кызы')).toBeInTheDocument()
  })

  it('поиск записи сайта во время сохранения недоступен и запрос не уходит', async () => {
    const user = userEvent.setup()
    const gate = deferred<void>()
    const { calls } = installFetchMock(
      baseRoutes([
        {
          method: 'POST',
          path: '/api/v1/links',
          handler: async () => {
            await gate.promise
            return jsonResponse(
              {
                id: '6f1b7f0e-1b1f-4a1e-9f2a-2f3b4c5d6e7f',
                crm_id: '1352',
                site_id: '3586',
                created_at: '2026-11-12T16:50:46Z',
              },
              201,
            )
          },
        },
      ]),
    )
    renderApp()

    await searchForCrm(user)
    await selectFirstCandidate(user)
    await lookupSiteRecord(user)
    await waitForSiteRecord()

    await user.click(screen.getByRole('button', { name: 'Сохранить связь' }))
    await screen.findByRole('button', { name: 'Сохраняем связь…' })

    // Поиск записи на сайте во время сохранения недоступен.
    const lookupButton = screen.getByRole('button', { name: 'Найти запись на сайте' })
    expect(lookupButton).toBeDisabled()
    await user.click(lookupButton)

    // Повторный запрос за записью не уходит.
    expect(calls.filter((call) => call.pathname.startsWith('/api/v1/site-clients'))).toHaveLength(1)

    gate.resolve()

    expect(
      await screen.findByText(
        'Готово. Связь сохранена: клиент основной базы № 1352 и запись на сайте № 3586.',
      ),
    ).toBeInTheDocument()

    // Запись сайта остаётся на экране.
    const step = stepCard(/Найдите его запись на сайте/)
    expect(within(step).getByText('База сайта')).toBeInTheDocument()
    expect(within(step).getByText('3586')).toBeInTheDocument()
  })

  it('после неудачного сохранения данные можно исправить и попробовать снова', async () => {
    const user = userEvent.setup()
    let attempts = 0
    const { calls } = installFetchMock(
      baseRoutes([
        {
          method: 'POST',
          path: '/api/v1/links',
          handler: async () => {
            attempts += 1
            if (attempts === 1) {
              return jsonResponse({ code: 'internal_error', message: 'x' }, 500)
            }
            return jsonResponse({ id: 'x', crm_id: '1352', site_id: '3587' }, 201)
          },
        },
      ]),
    )
    renderApp()

    await searchForCrm(user)
    await selectFirstCandidate(user)
    await lookupSiteRecord(user)
    await waitForSiteRecord()

    await user.click(screen.getByRole('button', { name: 'Сохранить связь' }))
    expect(
      await screen.findByText('Что-то пошло не так. Попробуйте чуть позже или сообщите администратору.'),
    ).toBeInTheDocument()
    expect(calls.filter((call) => call.method === 'POST')).toHaveLength(1)

    // Сотрудник исправляет номер, снова находит запись и сохраняет связь.
    const input = screen.getByLabelText('Номер записи на сайте')
    await user.clear(input)
    await user.type(input, '3587')
    await user.click(screen.getByRole('button', { name: 'Найти запись на сайте' }))
    await waitForSiteRecord()

    await user.click(screen.getByRole('button', { name: 'Сохранить связь' }))
    expect(
      await screen.findByText(
        'Готово. Связь сохранена: клиент основной базы № 1352 и запись на сайте № 3587.',
      ),
    ).toBeInTheDocument()
    expect(calls.filter((call) => call.method === 'POST')).toHaveLength(2)
  })
})

describe('актуальность результатов поиска клиента', () => {
  it('изменённый текст запроса не предлагает прежние результаты', async () => {
    const user = userEvent.setup()
    installFetchMock(baseRoutes())
    renderApp()

    await searchForCrm(user)
    const step = stepCard(/Выберите клиента/)
    expect(await within(step).findByText('Иванов Иван Бек кызы')).toBeInTheDocument()

    const input = screen.getByLabelText('Имя или фамилия клиента')
    await user.clear(input)
    await user.type(input, 'Петров')

    expect(within(step).queryByText('Иванов Иван Бек кызы')).not.toBeInTheDocument()
    expect(
      within(step).getByText(
        'Вы изменили условия поиска. Нажмите «Найти клиента», чтобы увидеть результаты по новым условиям.',
      ),
    ).toBeInTheDocument()
  })

  it('ошибка поиска не выглядит как пустой успешный поиск', async () => {
    const user = userEvent.setup()
    installFetchMock(
      baseRoutes([
        {
          method: 'GET',
          path: '/api/v1/crm-clients',
          handler: () => jsonResponse({ code: 'internal_error', message: 'x' }, 500),
        },
      ]),
    )
    renderApp()

    await searchForCrm(user)

    expect(
      await screen.findByText('Не удалось получить список клиентов. Попробуйте ещё раз.'),
    ).toBeInTheDocument()
    expect(
      screen.queryByText(
        'Никого не нашли. Проверьте написание и попробуйте ввести только фамилию или только имя.',
      ),
    ).not.toBeInTheDocument()
  })

  it('различает состояния: поиск не выполнялся, ничего не нашли, ошибка', async () => {
    const user = userEvent.setup()
    installFetchMock(
      baseRoutes([
        {
          method: 'GET',
          path: '/api/v1/crm-clients',
          handler: () => jsonResponse([]),
        },
      ]),
    )
    renderApp()

    // Поиск ещё не выполнялся.
    expect(
      screen.getByText(
        'Здесь появятся найденные клиенты. Введите имя или фамилию выше и нажмите «Найти клиента».',
      ),
    ).toBeInTheDocument()

    // Успешный поиск без результатов.
    await searchForCrm(user)
    expect(
      await screen.findByText(
        'Никого не нашли. Проверьте написание и попробуйте ввести только фамилию или только имя.',
      ),
    ).toBeInTheDocument()
  })
})

describe('проверка номера записи на сайте', () => {
  it('пустой номер объясняет ошибку, помечает поле и запрос не отправляет', async () => {
    const user = userEvent.setup()
    const { calls } = installFetchMock(baseRoutes())
    renderApp()

    await searchForCrm(user)
    await selectFirstCandidate(user)

    const input = screen.getByLabelText('Номер записи на сайте')
    await user.type(input, '   ')

    await user.click(screen.getByRole('button', { name: 'Найти запись на сайте' }))

    expect(await screen.findByText('Введите номер записи на сайте')).toBeInTheDocument()
    expect(input).toHaveAttribute('aria-invalid', 'true')
    const describedBy = input.getAttribute('aria-describedby') ?? ''
    expect(describedBy.split(' ')).toHaveLength(2)
    expect(
      calls.filter((call) => call.pathname.startsWith('/api/v1/site-clients')),
    ).toHaveLength(0)

    // Ошибка исчезает, как только сотрудник исправил поле.
    await user.clear(input)
    await user.type(input, '3586')
    expect(screen.queryByText('Введите номер записи на сайте')).not.toBeInTheDocument()
    expect(input).not.toHaveAttribute('aria-invalid')
  })

  it('пустой номер с клавиатуры тоже не приводит к запросу', async () => {
    const user = userEvent.setup()
    const { calls } = installFetchMock(baseRoutes())
    renderApp()

    await searchForCrm(user)
    await selectFirstCandidate(user)

    const input = screen.getByLabelText('Номер записи на сайте')
    input.focus()
    await user.keyboard('   {Enter}')

    expect(await screen.findByText('Введите номер записи на сайте')).toBeInTheDocument()
    expect(
      calls.filter((call) => call.pathname.startsWith('/api/v1/site-clients')),
    ).toHaveLength(0)
  })
})

describe('выгрузка CSV', () => {
  it('до успешной загрузки списка ссылки нет, а кнопка отключена', async () => {
    installFetchMock(
      baseRoutes([
        {
          method: 'GET',
          path: '/api/v1/links',
          handler: () => jsonResponse({ code: 'internal_error', message: 'x' }, 500),
        },
      ]),
    )
    renderApp()

    expect(
      await screen.findByText('Не удалось получить список сохранённых связей. Попробуйте ещё раз.'),
    ).toBeInTheDocument()

    expect(screen.queryByRole('link', { name: 'Скачать CSV' })).not.toBeInTheDocument()
    const button = screen.getByRole('button', { name: 'Скачать CSV' })
    expect(button).toBeDisabled()
    expect(button).not.toHaveAttribute('href')
  })

  it('после успешной загрузки доступна ссылка на выгрузку', async () => {
    installFetchMock(baseRoutes())
    renderApp()

    const link = await screen.findByRole('link', { name: 'Скачать CSV' })
    expect(link).toHaveAttribute('href', '/api/v1/links/export.csv')
  })
})

describe('подтверждение сохранения и обновление списка', () => {
  it('подтверждение появляется до обновления списка, а список обновляется после', async () => {
    const user = userEvent.setup()
    const gate = deferred<void>()
    let listCalls = 0

    installFetchMock(
      baseRoutes([
        {
          method: 'GET',
          path: '/api/v1/links',
          handler: async () => {
            listCalls += 1
            if (listCalls === 1) {
              return jsonResponse([])
            }
            await gate.promise
            return jsonResponse([
              {
                id: '6f1b7f0e-1b1f-4a1e-9f2a-2f3b4c5d6e7f',
                crm_id: '1352',
                site_id: '3586',
                created_at: '2026-11-12T16:50:46Z',
              },
            ])
          },
        },
      ]),
    )
    renderApp()

    await searchForCrm(user)
    await selectFirstCandidate(user)
    await lookupSiteRecord(user)
    await waitForSiteRecord()

    await user.click(screen.getByRole('button', { name: 'Сохранить связь' }))

    // Подтверждение не ждёт перезагрузку списка связей.
    expect(
      await screen.findByText(
        'Готово. Связь сохранена: клиент основной базы № 1352 и запись на сайте № 3586.',
      ),
    ).toBeInTheDocument()
    expect(screen.queryByRole('table')).not.toBeInTheDocument()

    // Список обновляется, когда ответ приходит.
    gate.resolve()
    await waitFor(() => expect(screen.getByRole('table')).toBeInTheDocument())
    expect(within(screen.getByRole('table')).getByText('1352')).toBeInTheDocument()
    expect(within(screen.getByRole('table')).getByText('3586')).toBeInTheDocument()
  })
})
