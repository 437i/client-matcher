import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'

import { baseRoutes, installFetchMock, renderApp, stepCard } from '@/test/utils'

/**
 * Клавиатурная навигация проверяется настоящими переходами Tab: текущий
 * элемент определяется через document.activeElement, без программной
 * установки фокуса.
 */
describe('последовательная клавиатурная навигация', () => {
  it('ведёт по шагам сценария в порядке работы сотрудника', async () => {
    const user = userEvent.setup()
    installFetchMock(baseRoutes())
    renderApp()

    const searchInput = screen.getByLabelText('Имя или фамилия клиента')
    const searchButton = screen.getByRole('button', { name: 'Найти клиента' })

    // Шаг 1: сначала поле поиска, затем кнопка поиска.
    await user.tab()
    expect(document.activeElement).toBe(searchInput)
    await user.keyboard('Иванов')

    await user.tab()
    expect(document.activeElement).toBe(searchButton)
    await user.keyboard('{Enter}')

    // Шаг 2: после появления результатов следующий элемент — выбор кандидата.
    const candidates = await screen.findAllByRole('button', { name: 'Выбрать этого клиента' })
    await user.tab()
    expect(document.activeElement).toBe(candidates[0])

    await user.keyboard('{Enter}')

    // Список кандидатов исчез вместе с активной кнопкой — фокус перешёл на
    // выбранного клиента.
    const changeChoice = await screen.findByRole('button', { name: 'Изменить выбор' })
    await waitFor(() =>
      expect(document.activeElement).toHaveTextContent('Иванов Иван Бек кызы'),
    )

    // Шаг 3: изменение выбора → поле номера записи на сайте.
    await user.tab()
    expect(document.activeElement).toBe(changeChoice)

    const siteInput = screen.getByLabelText('Номер записи на сайте')
    await user.tab()
    expect(document.activeElement).toBe(siteInput)
    await user.keyboard('3586{Enter}')

    await within(stepCard(/Найдите его запись на сайте/)).findByText('База сайта')

    // Шаги 3–4 и раздел сохранённых связей — дальше по порядку.
    await user.tab()
    expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Найти запись на сайте' }))

    await user.tab()
    expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Сохранить связь' }))

    await user.tab()
    expect(document.activeElement).toBe(screen.getByRole('link', { name: 'Скачать CSV' }))
  })

  it('позволяет пройти весь сценарий клавишей Enter', async () => {
    const user = userEvent.setup()
    installFetchMock(baseRoutes())
    renderApp()

    await user.tab()
    await user.keyboard('Иванов{Enter}')

    const candidates = await screen.findAllByRole('button', { name: 'Выбрать этого клиента' })
    await user.tab()
    await user.tab()
    expect(document.activeElement).toBe(candidates[0])
    await user.keyboard('{Enter}')

    await screen.findByRole('button', { name: 'Изменить выбор' })
    await waitFor(() =>
      expect(document.activeElement).toHaveTextContent('Иванов Иван Бек кызы'),
    )

    await user.tab()
    expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Изменить выбор' }))

    await user.tab()
    expect(document.activeElement).toBe(screen.getByLabelText('Номер записи на сайте'))
    await user.keyboard('3586{Enter}')

    await within(stepCard(/Найдите его запись на сайте/)).findByText('База сайта')

    await user.tab()
    expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Найти запись на сайте' }))

    await user.tab()
    expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Сохранить связь' }))
    await user.keyboard('{Enter}')

    expect(
      await screen.findByText(
        'Готово. Связь сохранена: клиент основной базы № 1352 и запись на сайте № 3586.',
      ),
    ).toBeInTheDocument()
  })
})

describe('управление фокусом после выбора клиента', () => {
  it('после выбора клиента фокус переходит на выбранного клиента', async () => {
    const user = userEvent.setup()
    installFetchMock(baseRoutes())
    renderApp()

    await user.type(screen.getByLabelText('Имя или фамилия клиента'), 'Иванов')
    await user.click(screen.getByRole('button', { name: 'Найти клиента' }))
    const buttons = await screen.findAllByRole('button', { name: 'Выбрать этого клиента' })
    await user.click(buttons[0] as HTMLElement)

    await waitFor(() =>
      expect(document.activeElement).toHaveTextContent('Иванов Иван Бек кызы'),
    )
    expect(document.activeElement).toHaveAttribute('tabindex', '-1')
  })

  it('после «Изменить выбор» фокус переходит к списку клиентов', async () => {
    const user = userEvent.setup()
    installFetchMock(baseRoutes())
    renderApp()

    await user.type(screen.getByLabelText('Имя или фамилия клиента'), 'Иванов')
    await user.click(screen.getByRole('button', { name: 'Найти клиента' }))
    const buttons = await screen.findAllByRole('button', { name: 'Выбрать этого клиента' })
    await user.click(buttons[0] as HTMLElement)

    await user.click(screen.getByRole('button', { name: 'Изменить выбор' }))

    await waitFor(() => expect(document.activeElement?.tagName).toBe('UL'))
    const list = document.activeElement as HTMLElement
    expect(list).toHaveAttribute('aria-label', 'Найденные клиенты')
    expect(within(list).getAllByRole('button', { name: 'Выбрать этого клиента' })).toHaveLength(2)
  })

  it('не забирает фокус, когда сотрудник сам возвращается к поиску', async () => {
    const user = userEvent.setup()
    installFetchMock(baseRoutes())
    renderApp()

    await user.type(screen.getByLabelText('Имя или фамилия клиента'), 'Иванов')
    await user.click(screen.getByRole('button', { name: 'Найти клиента' }))
    const buttons = await screen.findAllByRole('button', { name: 'Выбрать этого клиента' })
    await user.click(buttons[0] as HTMLElement)

    // Сотрудник вернулся к поиску: обновление результатов не забирает фокус.
    const input = screen.getByLabelText('Имя или фамилия клиента')
    await user.click(input)
    await user.type(input, 'Петров')

    expect(document.activeElement).toBe(input)
  })
})
