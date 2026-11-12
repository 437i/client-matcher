import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'

import App from '@/App'
import { setFetchImplementation } from '@/api/client'

export type FetchCall = {
  method: string
  url: string
  pathname: string
  body: unknown
}

export type Route = {
  method: string
  path: string
  handler: () => Response | Promise<Response>
}

/** Ответ с JSON-телом. */
export function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'content-type': 'application/json' },
  })
}

/**
 * Подмена сетевого слоя: реальные файлы данных не используются, ответы
 * подставляются из теста.
 */
export function installFetchMock(routes: Route[]) {
  const calls: FetchCall[] = []

  const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const request = input instanceof Request ? input : undefined
    const rawUrl =
      typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url
    const method = (init?.method ?? request?.method ?? 'GET').toUpperCase()
    const pathname = new URL(rawUrl, 'http://localhost').pathname
    const rawBody = init?.body ?? (request ? await request.clone().text() : undefined)
    const body = typeof rawBody === 'string' && rawBody ? JSON.parse(rawBody) : undefined

    calls.push({ method, url: rawUrl, pathname, body })

    const route = routes.find((item) => item.method === method && item.path === pathname)
    if (!route) {
      throw new TypeError(`Необработанный запрос: ${method} ${pathname}`)
    }
    return route.handler()
  })

  setFetchImplementation(fetchMock)
  return { fetchMock, calls }
}

/** Рендер приложения с отдельным QueryClient: тесты не зависят друг от друга. */
export function renderApp() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  })

  return render(
    <QueryClientProvider client={queryClient}>
      <App />
    </QueryClientProvider>,
  )
}

/* ------------------------------------------------------------------ */
/* Общие фикстуры                                                       */
/* ------------------------------------------------------------------ */

export const crmClientA = {
  id: '1352',
  full_name: 'Иванов Иван Бек кызы',
  last_name: 'Иванов',
  first_name: 'Иван',
  birthday: '1982-04-15',
  clinic: 'Наименование Организации',
}

export const crmClientC = {
  id: '1354',
  full_name: 'Королёв Пётр Сергеевич',
  last_name: 'Королёв',
  first_name: 'Пётр',
  birthday: '2001-09-09',
  clinic: 'НИИ им. Денисова',
}

export const siteClientB = {
  id: '3586',
  short_name: 'Ваня',
  last_name: 'Иванов',
  birthday: '1982-04-15',
}

export const siteClientD = {
  id: '3587',
  short_name: 'Маша',
  last_name: 'Иванова',
  birthday: '1990-01-02',
}

/**
 * Базовые маршруты для сценария. Переопределение из теста заменяет базовый
 * маршрут с тем же путём.
 */
export function baseRoutes(overrides: Route[] = []): Route[] {
  const base: Route[] = [
    {
      method: 'GET',
      path: '/api/v1/crm-clients',
      handler: () => jsonResponse([crmClientA, crmClientC]),
    },
    {
      method: 'GET',
      path: '/api/v1/site-clients/3586',
      handler: () => jsonResponse(siteClientB),
    },
    {
      method: 'GET',
      path: '/api/v1/site-clients/3587',
      handler: () => jsonResponse(siteClientD),
    },
    {
      method: 'GET',
      path: '/api/v1/links',
      handler: () => jsonResponse([]),
    },
    {
      method: 'POST',
      path: '/api/v1/links',
      handler: () =>
        jsonResponse(
          {
            id: '6f1b7f0e-1b1f-4a1e-9f2a-2f3b4c5d6e7f',
            crm_id: '1352',
            site_id: '3586',
            created_at: '2026-11-12T16:50:46Z',
          },
          201,
        ),
    },
  ]

  const overridden = new Set(overrides.map((route) => `${route.method} ${route.path}`))
  return [
    ...base.filter((route) => !overridden.has(`${route.method} ${route.path}`)),
    ...overrides,
  ]
}

/** Отложенный ответ: тест сам решает, когда он придёт. */
export function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((res) => {
    resolve = res
  })
  return { promise, resolve }
}

/** Блок шага по его названию: в нём ищем содержимое шага. */
export function stepCard(title: RegExp): HTMLElement {
  const heading = screen.getByRole('heading', { name: title })
  const card = heading.closest('[data-slot="card"]')
  if (!card) {
    throw new Error('Не найден блок шага')
  }
  return card as HTMLElement
}

/** Ждёт, пока запись сайта появится в блоке шага 3. */
export async function waitForSiteRecord(): Promise<void> {
  await within(stepCard(/Найдите его запись на сайте/)).findByText('База сайта')
}

/* ------------------------------------------------------------------ */
/* Шаги сценария                                                        */
/* ------------------------------------------------------------------ */

export async function searchForCrm(
  user: ReturnType<typeof userEvent.setup>,
  query = 'Иванов',
) {
  const input = screen.getByLabelText('Имя или фамилия клиента')
  await user.clear(input)
  await user.type(input, query)
  await user.click(screen.getByRole('button', { name: 'Найти клиента' }))
}

export async function selectFirstCandidate(user: ReturnType<typeof userEvent.setup>) {
  const buttons = await screen.findAllByRole('button', { name: 'Выбрать этого клиента' })
  await user.click(buttons[0] as HTMLElement)
}

export async function lookupSiteRecord(
  user: ReturnType<typeof userEvent.setup>,
  siteId = '3586',
) {
  const input = screen.getByLabelText('Номер записи на сайте')
  await user.clear(input)
  await user.type(input, siteId)
  await user.click(screen.getByRole('button', { name: 'Найти запись на сайте' }))
}
