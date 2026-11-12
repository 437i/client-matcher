import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import {
  baseRoutes,
  crmClientA,
  deferred,
  installFetchMock,
  jsonResponse,
  lookupSiteRecord,
  renderApp,
  searchForCrm,
  selectFirstCandidate,
  siteClientB,
  stepCard,
  waitForSiteRecord,
} from '@/test/utils'

beforeEach(() => {
  vi.unstubAllGlobals()
})

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('поиск клиента в основной базе', () => {
  it('не отправляет запрос на пустое поле и объясняет, что вводить', async () => {
    const user = userEvent.setup()
    const { calls } = installFetchMock(baseRoutes())
    renderApp()

    await user.click(screen.getByRole('button', { name: 'Найти клиента' }))

    expect(await screen.findByText('Введите имя или фамилию клиента')).toBeInTheDocument()
    expect(calls.filter((call) => call.pathname === '/api/v1/crm-clients')).toHaveLength(0)
  })

  it('показывает найденных кандидатов и не выбирает никого', async () => {
    const user = userEvent.setup()
    installFetchMock(baseRoutes())
    renderApp()

    await searchForCrm(user)

    const step = stepCard(/Выберите клиента/)
    expect(await within(step).findByText('Иванов Иван Бек кызы')).toBeInTheDocument()
    expect(within(step).getByText('Королёв Пётр Сергеевич')).toBeInTheDocument()
    expect(within(step).getByText('Наименование Организации')).toBeInTheDocument()
    expect(within(step).getByText('15.04.1982')).toBeInTheDocument()
    expect(within(step).getByText('1352')).toBeInTheDocument()
    expect(screen.getByText('Найдено записей: 2')).toBeInTheDocument()
    // Никто не выбран: действия изменения выбора ещё нет.
    expect(screen.queryByRole('button', { name: 'Изменить выбор' })).not.toBeInTheDocument()
  })

  it('объясняет, что ничего не нашлось', async () => {
    const user = userEvent.setup()
    installFetchMock(
      baseRoutes([
        { method: 'GET', path: '/api/v1/crm-clients', handler: () => jsonResponse([]) },
      ]),
    )
    renderApp()

    await searchForCrm(user)

    expect(
      await screen.findByText(
        'Никого не нашли. Проверьте написание и попробуйте ввести только фамилию или только имя.',
      ),
    ).toBeInTheDocument()
  })

  it('объясняет, что список клиентов получить не удалось', async () => {
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
  })

  it('повторяет поиск после ошибки без изменения введённого текста', async () => {
    const user = userEvent.setup()
    let attempts = 0
    const { calls } = installFetchMock(
      baseRoutes([
        {
          method: 'GET',
          path: '/api/v1/crm-clients',
          handler: () => {
            attempts += 1
            if (attempts === 1) {
              return jsonResponse({ code: 'internal_error', message: 'x' }, 500)
            }
            return jsonResponse([crmClientA])
          },
        },
      ]),
    )
    renderApp()

    await searchForCrm(user)
    expect(
      await screen.findByText('Не удалось получить список клиентов. Попробуйте ещё раз.'),
    ).toBeInTheDocument()

    // Тот же текст запроса: поиск должен повториться.
    await user.click(screen.getByRole('button', { name: 'Найти клиента' }))

    const step = stepCard(/Выберите клиента/)
    expect(await within(step).findByText('Иванов Иван Бек кызы')).toBeInTheDocument()
    expect(calls.filter((call) => call.pathname === '/api/v1/crm-clients')).toHaveLength(2)
  })

  it('повторяет поиск после сетевого сбоя без изменения введённого текста', async () => {
    const user = userEvent.setup()
    let attempts = 0
    const { calls } = installFetchMock(
      baseRoutes([
        {
          method: 'GET',
          path: '/api/v1/crm-clients',
          handler: () => {
            attempts += 1
            if (attempts === 1) {
              throw new TypeError('Failed to fetch')
            }
            return jsonResponse([crmClientA])
          },
        },
      ]),
    )
    renderApp()

    await searchForCrm(user)
    expect(
      await screen.findByText('Не удалось получить список клиентов. Попробуйте ещё раз.'),
    ).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Найти клиента' }))

    const step = stepCard(/Выберите клиента/)
    expect(await within(step).findByText('Иванов Иван Бек кызы')).toBeInTheDocument()
    expect(calls.filter((call) => call.pathname === '/api/v1/crm-clients')).toHaveLength(2)
  })

  it('не отправляет второй запрос, пока первый ещё выполняется', async () => {
    const user = userEvent.setup()
    const gate = deferred<void>()
    const { calls } = installFetchMock(
      baseRoutes([
        {
          method: 'GET',
          path: '/api/v1/crm-clients',
          handler: async () => {
            await gate.promise
            return jsonResponse([crmClientA])
          },
        },
      ]),
    )
    renderApp()

    const input = screen.getByLabelText('Имя или фамилия клиента')
    await user.type(input, 'Иванов')
    await user.keyboard('{Enter}')

    // Пока запрос выполняется, повторное нажатие не отправляет второй.
    expect(await screen.findByRole('button', { name: 'Ищем клиентов…' })).toBeInTheDocument()
    await user.keyboard('{Enter}')

    expect(calls.filter((call) => call.pathname === '/api/v1/crm-clients')).toHaveLength(1)

    gate.resolve()
    await within(stepCard(/Выберите клиента/)).findByText('Иванов Иван Бек кызы')
  })

  it('предупреждает, что показаны только первые 50 совпадений', async () => {
    const user = userEvent.setup()
    const many = Array.from({ length: 50 }, (_, index) => ({
      ...crmClientA,
      id: String(2000 + index),
    }))
    installFetchMock(
      baseRoutes([
        { method: 'GET', path: '/api/v1/crm-clients', handler: () => jsonResponse(many) },
      ]),
    )
    renderApp()

    await searchForCrm(user)

    expect(
      await screen.findByText(
        'Найдено записей: 50. Показаны первые 50 совпадений. Уточните фамилию или имя, чтобы найти нужного человека',
      ),
    ).toBeInTheDocument()
  })

  it('не предупреждает о лимите, когда найдено меньше 50 клиентов', async () => {
    const user = userEvent.setup()
    installFetchMock(baseRoutes())
    renderApp()

    await searchForCrm(user)

    expect(await screen.findByText('Найдено записей: 2')).toBeInTheDocument()
    expect(screen.queryByText(/Показаны первые 50 совпадений/)).not.toBeInTheDocument()
  })

  it('показывает выбранного клиента и позволяет изменить выбор', async () => {
    const user = userEvent.setup()
    installFetchMock(baseRoutes())
    renderApp()

    await searchForCrm(user)
    await selectFirstCandidate(user)

    const step = stepCard(/Выберите клиента/)
    expect(await within(step).findByText('Иванов Иван Бек кызы')).toBeInTheDocument()
    expect(within(step).getByText('1352')).toBeInTheDocument()
    expect(within(step).getByText('Наименование Организации')).toBeInTheDocument()

    await user.click(within(step).getByRole('button', { name: 'Изменить выбор' }))
    expect(await screen.findAllByRole('button', { name: 'Выбрать этого клиента' })).toHaveLength(2)
  })
})

describe('поиск записи на сайте', () => {
  it('показывает найденную запись с именем, фамилией, датой и номером', async () => {
    const user = userEvent.setup()
    installFetchMock(baseRoutes())
    renderApp()

    await searchForCrm(user)
    await selectFirstCandidate(user)
    await lookupSiteRecord(user)

    const step = stepCard(/Найдите его запись на сайте/)
    expect(await within(step).findByText('База сайта')).toBeInTheDocument()
    expect(within(step).getByText('Имя:')).toBeInTheDocument()
    expect(within(step).getByText('Ваня')).toBeInTheDocument()
    expect(within(step).getByText('Фамилия:')).toBeInTheDocument()
    expect(within(step).getByText('Иванов')).toBeInTheDocument()
    expect(within(step).getByText('15.04.1982')).toBeInTheDocument()
    expect(within(step).getByText('3586')).toBeInTheDocument()
  })

  it('объясняет, что записи с таким номером нет, и оставляет номер в поле', async () => {
    const user = userEvent.setup()
    installFetchMock(
      baseRoutes([
        {
          method: 'GET',
          path: '/api/v1/site-clients/3586',
          handler: () => jsonResponse({ code: 'site_client_not_found', message: 'x' }, 404),
        },
      ]),
    )
    renderApp()

    await searchForCrm(user)
    await selectFirstCandidate(user)
    await lookupSiteRecord(user)

    expect(
      await screen.findByText('Запись с таким номером не найдена. Проверьте номер и попробуйте ещё раз.'),
    ).toBeInTheDocument()
    expect(screen.getByLabelText('Номер записи на сайте')).toHaveValue('3586')
  })

  it('не показывает текст шлюза, если ответ пришёл без JSON', async () => {
    const user = userEvent.setup()
    installFetchMock(
      baseRoutes([
        {
          method: 'GET',
          path: '/api/v1/site-clients/3586',
          handler: () =>
            new Response('<html><body>502 Bad Gateway</body></html>', {
              status: 502,
              headers: { 'content-type': 'text/html' },
            }),
        },
      ]),
    )
    renderApp()

    await searchForCrm(user)
    await selectFirstCandidate(user)
    await lookupSiteRecord(user)

    expect(
      await screen.findByText('Данные временно недоступны. Попробуйте повторить действие.'),
    ).toBeInTheDocument()
    expect(screen.queryByText(/Bad Gateway/)).not.toBeInTheDocument()
    expect(screen.getByLabelText('Номер записи на сайте')).toHaveValue('3586')
  })

  it('требует заново найти запись, если номер изменился', async () => {
    const user = userEvent.setup()
    installFetchMock(baseRoutes())
    renderApp()

    await searchForCrm(user)
    await selectFirstCandidate(user)
    await lookupSiteRecord(user)
    await waitForSiteRecord()

    const input = screen.getByLabelText('Номер записи на сайте')
    await user.clear(input)
    await user.type(input, '3587')

    expect(
      await screen.findByText(
        'Номер изменился. Найдите запись на сайте заново, чтобы сохранить связь.',
      ),
    ).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Сохранить связь' })).toBeDisabled()
  })
})

describe('смена выбранного клиента', () => {
  it('делает загруженную запись сайта недействительной и не даёт сохранить', async () => {
    const user = userEvent.setup()
    const { calls } = installFetchMock(baseRoutes())
    renderApp()

    await searchForCrm(user)
    await selectFirstCandidate(user)
    await lookupSiteRecord(user)
    await waitForSiteRecord()

    await user.click(screen.getByRole('button', { name: 'Изменить выбор' }))

    const buttons = await screen.findAllByRole('button', { name: 'Выбрать этого клиента' })
    await user.click(buttons[1] as HTMLElement)

    expect(
      await screen.findByText(
        'Клиент основной базы изменён. Найдите запись на сайте заново — прежняя запись уже не подходит.',
      ),
    ).toBeInTheDocument()

    const saveButton = screen.getByRole('button', { name: 'Сохранить связь' })
    expect(saveButton).toBeDisabled()

    await user.click(saveButton)
    expect(calls.filter((call) => call.method === 'POST')).toHaveLength(0)
  })
})

describe('сохранение связи', () => {
  it('показывает обе записи и оба номера перед сохранением', async () => {
    const user = userEvent.setup()
    installFetchMock(baseRoutes())
    renderApp()

    await searchForCrm(user)
    await selectFirstCandidate(user)
    await lookupSiteRecord(user)
    await waitForSiteRecord()

    const step = stepCard(/Проверьте данные и сохраните связь/)
    expect(within(step).getByText('Проверьте данные')).toBeInTheDocument()
    expect(within(step).getByText('Основная база (CRM)')).toBeInTheDocument()
    expect(within(step).getByText('База сайта')).toBeInTheDocument()
    expect(within(step).getByText('Номер в основной базе:')).toBeInTheDocument()
    expect(within(step).getByText('Номер записи на сайте:')).toBeInTheDocument()
    expect(within(step).getByText('1352')).toBeInTheDocument()
    expect(within(step).getByText('3586')).toBeInTheDocument()
  })

  it('блокирует сохранение, пока запись на сайте не найдена', async () => {
    const user = userEvent.setup()
    installFetchMock(baseRoutes())
    renderApp()

    await searchForCrm(user)
    await selectFirstCandidate(user)

    expect(screen.getByRole('button', { name: 'Сохранить связь' })).toBeDisabled()
    expect(
      screen.getByText('Сначала найдите запись на сайте — без неё сохранить связь нельзя.'),
    ).toBeInTheDocument()
  })

  it('сохраняет связь один раз и подтверждает результат', async () => {
    const user = userEvent.setup()

    let releaseSave = (): void => {}
    const saveFinished = new Promise<void>((resolve) => {
      releaseSave = resolve
    })

    const { calls } = installFetchMock(
      baseRoutes([
        {
          method: 'POST',
          path: '/api/v1/links',
          handler: async () => {
            await saveFinished
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

    const saveButton = screen.getByRole('button', { name: 'Сохранить связь' })
    await user.click(saveButton)
    await user.click(saveButton)

    // Пока запрос не завершён, кнопка недоступна и второй запрос не уходит.
    expect(await screen.findByRole('button', { name: 'Сохраняем связь…' })).toBeDisabled()
    expect(calls.filter((call) => call.method === 'POST')).toHaveLength(1)

    releaseSave()

    expect(
      await screen.findByText(
        'Готово. Связь сохранена: клиент основной базы № 1352 и запись на сайте № 3586.',
      ),
    ).toBeInTheDocument()
  })

  it('различает конфликт по клиенту и конфликт по записи сайта', async () => {
    const user = userEvent.setup()
    installFetchMock(
      baseRoutes([
        {
          method: 'POST',
          path: '/api/v1/links',
          handler: () => jsonResponse({ code: 'crm_already_linked', message: 'x' }, 409),
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
      await screen.findByText(
        'Этот клиент основной базы уже связан с другой записью на сайте. Посмотрите список сохранённых связей — нужная связь, возможно, уже есть.',
      ),
    ).toBeInTheDocument()
  })

  it('объясняет конфликт по записи сайта отдельно', async () => {
    const user = userEvent.setup()
    installFetchMock(
      baseRoutes([
        {
          method: 'POST',
          path: '/api/v1/links',
          handler: () => jsonResponse({ code: 'site_already_linked', message: 'x' }, 409),
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
      await screen.findByText(
        'Эта запись на сайте уже связана с другим клиентом основной базы. Посмотрите список сохранённых связей — нужная связь, возможно, уже есть.',
      ),
    ).toBeInTheDocument()
  })

  it('объясняет несовпадение данных и сохраняет введённое', async () => {
    const user = userEvent.setup()
    installFetchMock(
      baseRoutes([
        {
          method: 'POST',
          path: '/api/v1/links',
          handler: () => jsonResponse({ code: 'birthday_mismatch', message: 'x' }, 422),
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
      await screen.findByText(
        'Даты рождения в основной базе и на сайте не совпадают. Проверьте, что номер записи на сайте принадлежит этому человеку.',
      ),
    ).toBeInTheDocument()
    expect(screen.getByLabelText('Номер записи на сайте')).toHaveValue('3586')
  })

  it('убирает подсказку о будущем сохранении после успеха', async () => {
    const user = userEvent.setup()
    installFetchMock(baseRoutes())
    renderApp()

    await searchForCrm(user)
    await selectFirstCandidate(user)
    await lookupSiteRecord(user)
    await waitForSiteRecord()

    expect(
      screen.getByText('Связь сохранится и появится в списке сохранённых связей ниже.'),
    ).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Сохранить связь' }))

    expect(
      await screen.findByText(
        'Готово. Связь сохранена: клиент основной базы № 1352 и запись на сайте № 3586.',
      ),
    ).toBeInTheDocument()
    expect(
      screen.queryByText('Связь сохранится и появится в списке сохранённых связей ниже.'),
    ).not.toBeInTheDocument()
  })
})

describe('сохранённые связи и выгрузка', () => {
  it('показывает пустое состояние списка', async () => {
    installFetchMock(baseRoutes())
    renderApp()

    expect(
      await screen.findByText(
        'Пока ни одной связи нет. Когда вы сохраните первую, она появится в этом списке.',
      ),
    ).toBeInTheDocument()
  })

  it('показывает сохранённые связи в таблице', async () => {
    installFetchMock(
      baseRoutes([
        {
          method: 'GET',
          path: '/api/v1/links',
          handler: () =>
            jsonResponse([
              {
                id: '6f1b7f0e-1b1f-4a1e-9f2a-2f3b4c5d6e7f',
                crm_id: '1352',
                site_id: '3586',
                created_at: '2026-11-12T16:50:46Z',
              },
            ]),
        },
      ]),
    )
    renderApp()

    const table = await screen.findByRole('table')
    expect(within(table).getByRole('columnheader', { name: 'Номер в основной базе' })).toBeInTheDocument()
    expect(within(table).getByRole('columnheader', { name: 'Номер записи на сайте' })).toBeInTheDocument()
    expect(within(table).getByRole('columnheader', { name: 'Сохранено' })).toBeInTheDocument()
    expect(within(table).getByText('1352')).toBeInTheDocument()
    expect(within(table).getByText('3586')).toBeInTheDocument()
  })

  it('скрывает вводную подсказку, когда в списке уже есть связи', async () => {
    installFetchMock(
      baseRoutes([
        {
          method: 'GET',
          path: '/api/v1/links',
          handler: () =>
            jsonResponse([
              {
                id: '6f1b7f0e-1b1f-4a1e-9f2a-2f3b4c5d6e7f',
                crm_id: '1352',
                site_id: '3586',
                created_at: '2026-11-12T16:50:46Z',
              },
            ]),
        },
      ]),
    )
    renderApp()

    await screen.findByRole('table')
    expect(
      screen.queryByText('Здесь появятся сохранённые связи. После первого сохранения они будут видны в этом списке.'),
    ).not.toBeInTheDocument()
  })

  it('делает область прокрутки таблицы доступной с клавиатуры', async () => {
    installFetchMock(
      baseRoutes([
        {
          method: 'GET',
          path: '/api/v1/links',
          handler: () =>
            jsonResponse([
              {
                id: '6f1b7f0e-1b1f-4a1e-9f2a-2f3b4c5d6e7f',
                crm_id: '1352',
                site_id: '3586',
                created_at: '2026-11-12T16:50:46Z',
              },
            ]),
        },
      ]),
    )
    renderApp()

    const region = await screen.findByRole('region', { name: 'Сохранённые связи' })
    expect(region).toHaveAttribute('tabindex', '0')
    expect(within(region).getByRole('table')).toBeInTheDocument()
  })

  it('даёт ссылку на выгрузку CSV', async () => {
    installFetchMock(baseRoutes())
    renderApp()

    const link = await screen.findByRole('link', { name: 'Скачать CSV' })
    expect(link).toHaveAttribute('href', '/api/v1/links/export.csv')
  })

  it('отдельно объясняет ошибку списка и недоступность выгрузки', async () => {
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
    expect(screen.getByRole('button', { name: 'Повторить' })).toBeInTheDocument()
    expect(
      screen.getByText(
        'Сначала дождитесь, пока загрузится список связей, — без него файл выгрузить нельзя.',
      ),
    ).toBeInTheDocument()
  })
})

describe('неполный ответ сервиса', () => {
  it('показывает обозначение отсутствия данных в записи клиента', async () => {
    const user = userEvent.setup()
    installFetchMock(
      baseRoutes([
        {
          method: 'GET',
          path: '/api/v1/crm-clients',
          handler: () =>
            jsonResponse([{ id: '1352', full_name: 'Иванов Иван', birthday: '1982-04-15' }]),
        },
      ]),
    )
    renderApp()

    await searchForCrm(user)

    const step = stepCard(/Выберите клиента/)
    // Организации в ответе нет — показываем обозначение отсутствия данных.
    expect(await within(step).findByText('Организация:')).toBeInTheDocument()
    expect(within(step).getByText('—')).toBeInTheDocument()
  })

  it('не показывает запись без номера и не даёт сохранить связь', async () => {
    const user = userEvent.setup()
    const { calls } = installFetchMock(
      baseRoutes([
        {
          method: 'GET',
          path: '/api/v1/site-clients/3586',
          handler: () =>
            jsonResponse({ id: '', short_name: 'Ваня', last_name: 'Иванов', birthday: '1982-04-15' }),
        },
      ]),
    )
    renderApp()

    await searchForCrm(user)
    await selectFirstCandidate(user)
    await lookupSiteRecord(user)

    expect(
      await screen.findByText(
        'Полученная запись без номера. Найдите запись на сайте заново и попробуйте ещё раз.',
      ),
    ).toBeInTheDocument()
    const step = stepCard(/Найдите его запись на сайте/)
    expect(within(step).queryByText('База сайта')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Сохранить связь' })).toBeDisabled()
    expect(calls.filter((call) => call.method === 'POST')).toHaveLength(0)
  })

  it('не показывает запись, номер которой не совпадает с введённым', async () => {
    const user = userEvent.setup()
    const { calls } = installFetchMock(
      baseRoutes([
        {
          method: 'GET',
          path: '/api/v1/site-clients/3586',
          handler: () =>
            jsonResponse({ id: '9999', short_name: 'Ваня', last_name: 'Иванов', birthday: '1982-04-15' }),
        },
      ]),
    )
    renderApp()

    await searchForCrm(user)
    await selectFirstCandidate(user)
    await lookupSiteRecord(user)

    expect(
      await screen.findByText(
        'Полученная запись не соответствует введённому номеру. Найдите запись на сайте заново и попробуйте ещё раз.',
      ),
    ).toBeInTheDocument()
    const step = stepCard(/Найдите его запись на сайте/)
    expect(within(step).queryByText('База сайта')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Сохранить связь' })).toBeDisabled()
    expect(calls.filter((call) => call.method === 'POST')).toHaveLength(0)
  })
})

describe('работа с клавиатуры', () => {
  it('проводит сотрудника по всему сценарию без мыши', async () => {
    const user = userEvent.setup()
    installFetchMock(baseRoutes())
    renderApp()

    const searchInput = screen.getByLabelText('Имя или фамилия клиента')
    searchInput.focus()
    await user.keyboard('Иванов')
    await user.keyboard('{Enter}')

    const buttons = await screen.findAllByRole('button', { name: 'Выбрать этого клиента' })
    buttons[0]?.focus()
    await user.keyboard('{Enter}')

    const siteInput = screen.getByLabelText('Номер записи на сайте')
    siteInput.focus()
    await user.keyboard('3586')
    await user.keyboard('{Enter}')
    await waitForSiteRecord()

    const saveButton = screen.getByRole('button', { name: 'Сохранить связь' })
    saveButton.focus()
    await user.keyboard('{Enter}')

    expect(
      await screen.findByText(
        'Готово. Связь сохранена: клиент основной базы № 1352 и запись на сайте № 3586.',
      ),
    ).toBeInTheDocument()
  })
})

describe('объявления для вспомогательных технологий', () => {
  it('связывает подсказку и ошибку поля и помечает поле как ошибочное', async () => {
    const user = userEvent.setup()
    installFetchMock(baseRoutes())
    renderApp()

    const input = screen.getByLabelText('Имя или фамилия клиента')
    expect(input).toHaveAttribute('aria-describedby')
    expect(input).not.toHaveAttribute('aria-invalid')

    await user.click(screen.getByRole('button', { name: 'Найти клиента' }))

    expect(input).toHaveAttribute('aria-invalid', 'true')
    const alert = screen.getByRole('alert')
    expect(alert).toHaveTextContent('Введите имя или фамилию клиента')
    // Текст ошибки связан с полем: его идентификатор входит в aria-describedby.
    expect(input.getAttribute('aria-describedby')).toContain(alert.id)
    expect(alert.id).not.toBe('')

    // После исправления значения связь с ошибкой убирается.
    await user.type(input, 'Иванов')
    expect(input).not.toHaveAttribute('aria-invalid')
    expect(input.getAttribute('aria-describedby')).not.toContain(alert.id)
  })

  it('объявляет результат поиска в живой области', async () => {
    const user = userEvent.setup()
    installFetchMock(baseRoutes())
    renderApp()

    await searchForCrm(user)

    const status = screen.getByText('Найдено записей: 2')
    expect(status).toHaveAttribute('aria-live', 'polite')
  })

  it('объявляет загрузку и результат поиска записи на сайте', async () => {
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

    const loading = await screen.findByText('Ищем запись на сайте…')
    expect(loading).toHaveAttribute('aria-live', 'polite')

    gate.resolve()

    const success = await screen.findByText('Запись на сайте найдена.')
    expect(success).toHaveAttribute('aria-live', 'polite')
    // Одно состояние не объявляется двумя живыми областями.
    expect(screen.getAllByText('Запись на сайте найдена.')).toHaveLength(1)
  })

  it('показывает ошибку как оповещение и сопровождает её текстом состояния', async () => {
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

    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent('Не удалось получить список клиентов. Попробуйте ещё раз.')
  })
})
