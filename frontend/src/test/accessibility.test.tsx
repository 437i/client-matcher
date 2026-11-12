import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import axe from 'axe-core'
import { describe, expect, it } from 'vitest'

import { installFetchMock, jsonResponse, renderApp } from '@/test/utils'

const crmClient = {
  id: '1352',
  full_name: 'Иванов Иван Бек кызы',
  last_name: 'Иванов',
  first_name: 'Иван',
  birthday: '1982-04-15',
  clinic: 'Наименование Организации',
}

const siteClient = {
  id: '3586',
  short_name: 'Ваня',
  last_name: 'Иванов',
  birthday: '1982-04-15',
}

/**
 * Проверка доступности: ищем серьёзные и критические нарушения правил
 * WCAG, которые axe умеет проверять автоматически. Остальные требования
 * (контраст, фокус, размеры элементов, работа с клавиатуры) проверяются
 * отдельно: в jsdom нет canvas, поэтому axe не может измерить контраст
 * и помечает правило как «непроверяемое», а не как нарушение.
 */
async function expectNoSeriousViolations(container: HTMLElement) {
  const results = await axe.run(container)
  const serious = results.violations
    .filter((violation) => violation.impact === 'serious' || violation.impact === 'critical')
    .map((violation) => `${violation.id}: ${violation.help}`)

  expect(serious).toEqual([])
}

function installRoutes() {
  return installFetchMock([
    { method: 'GET', path: '/api/v1/crm-clients', handler: () => jsonResponse([crmClient]) },
    {
      method: 'GET',
      path: '/api/v1/site-clients/3586',
      handler: () => jsonResponse(siteClient),
    },
    { method: 'GET', path: '/api/v1/links', handler: () => jsonResponse([]) },
  ])
}

describe('доступность рабочего экрана', () => {
  it('начальное состояние не содержит серьёзных нарушений', async () => {
    installRoutes()
    const { container } = renderApp()

    await expectNoSeriousViolations(container)
  })

  it('результаты поиска не содержат серьёзных нарушений', async () => {
    const user = userEvent.setup()
    installRoutes()
    const { container } = renderApp()

    await user.type(screen.getByLabelText('Имя или фамилия клиента'), 'Иванов')
    await user.click(screen.getByRole('button', { name: 'Найти клиента' }))
    await screen.findByText('Иванов Иван Бек кызы')

    await expectNoSeriousViolations(container)
  })

  it('найденная запись на сайте не содержит серьёзных нарушений', async () => {
    const user = userEvent.setup()
    installRoutes()
    const { container } = renderApp()

    await user.type(screen.getByLabelText('Имя или фамилия клиента'), 'Иванов')
    await user.click(screen.getByRole('button', { name: 'Найти клиента' }))
    const buttons = await screen.findAllByRole('button', { name: 'Выбрать этого клиента' })
    await user.click(buttons[0] as HTMLElement)
    await user.type(screen.getByLabelText('Номер записи на сайте'), '3586')
    await user.click(screen.getByRole('button', { name: 'Найти запись на сайте' }))
    await screen.findAllByText('База сайта')

    await expectNoSeriousViolations(container)
  })

  it('состояние ошибки не содержит серьёзных нарушений', async () => {
    const user = userEvent.setup()
    installFetchMock([
      {
        method: 'GET',
        path: '/api/v1/crm-clients',
        handler: () => jsonResponse({ code: 'internal_error', message: 'x' }, 500),
      },
      { method: 'GET', path: '/api/v1/links', handler: () => jsonResponse([]) },
    ])
    const { container } = renderApp()

    await user.type(screen.getByLabelText('Имя или фамилия клиента'), 'Иванов')
    await user.click(screen.getByRole('button', { name: 'Найти клиента' }))
    await screen.findByText('Не удалось получить список клиентов. Попробуйте ещё раз.')

    await expectNoSeriousViolations(container)
  })
})
