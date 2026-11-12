import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import createClient from 'openapi-fetch'

import { ApiError } from './errors'
import type { components, paths } from './schema'

type FetchImplementation = (input: RequestInfo | URL, init?: RequestInit) => Promise<Response>

/**
 * Реализация запроса вынесена в отдельную переменную: клиент создаётся при
 * загрузке модуля и заимствует глобальный fetch один раз, поэтому подменить
 * его позже через глобальное окружение не получится. Тесты подменяют запрос
 * через setFetchImplementation.
 */
let fetchImplementation: FetchImplementation = (input, init) => globalThis.fetch(input, init)

export function setFetchImplementation(implementation: FetchImplementation): void {
  fetchImplementation = implementation
}

/**
 * Клиент работает с относительными путями: в спецификации они уже начинаются
 * с /api/v1, поэтому префикс не дублируется. Браузер обращается к тому же
 * origin, что и само приложение, — отдельного API-origin и CORS не нужно.
 */
export const api = createClient<paths>({
  baseUrl: '/',
  fetch: (input: RequestInfo | URL, init?: RequestInit): Promise<Response> =>
    fetchImplementation(input, init),
})

export type CrmClient = components['schemas']['handlers.CRMClientResponse']
export type SiteClient = components['schemas']['handlers.SiteClientResponse']
export type Link = components['schemas']['handlers.LinkResponse']
export type ApiErrorBody = components['schemas']['handlers.ErrorResponse']

/**
 * Ответ без JSON (страница шлюза при недоступном backend) не содержит кода
 * ошибки: backend такого ответа не отдаёт, но прокси может. В этом случае
 * пользователю показывается сообщение о временной недоступности данных, а не
 * текст ответа.
 */
function isJsonResponse(response: Response): boolean {
  return (response.headers.get('content-type') ?? '').includes('application/json')
}

function toApiError(status: number, body: ApiErrorBody | undefined, response: Response): ApiError {
  if (!isJsonResponse(response)) {
    return new ApiError('service_unavailable', status)
  }
  return new ApiError(body?.code ?? 'internal_error', status)
}

export function useCrmClientSearch(query: string) {
  return useQuery({
    queryKey: ['crm-clients', query],
    enabled: query.trim().length > 0,
    queryFn: async ({ signal }) => {
      const { data, error, response } = await api.GET('/api/v1/crm-clients', {
        params: { query: { q: query.trim() } },
        signal,
      })
      if (error !== undefined) {
        throw toApiError(response.status, error, response)
      }
      return data
    },
  })
}

/**
 * Поиск записи сайта выполняется только по явному действию сотрудника,
 * поэтому это мутация, а не фоновая загрузка по изменению поля.
 */
export function useSiteClientLookup() {
  return useMutation({
    mutationFn: async ({ siteId }: { siteId: string }) => {
      const { data, error, response } = await api.GET('/api/v1/site-clients/{id}', {
        params: { path: { id: siteId } },
      })
      if (error !== undefined) {
        throw toApiError(response.status, error, response)
      }
      return data
    },
  })
}

export function useLinks() {
  return useQuery({
    queryKey: ['links'],
    queryFn: async ({ signal }) => {
      const { data, error, response } = await api.GET('/api/v1/links', { signal })
      if (error !== undefined) {
        throw toApiError(response.status, error, response)
      }
      return data
    },
  })
}

export function useCreateLink() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (pair: { crm_id: string; site_id: string }) => {
      const { data, error, response } = await api.POST('/api/v1/links', {
        body: pair,
      })
      if (error !== undefined) {
        throw toApiError(response.status, error, response)
      }
      return data
    },
    onSuccess: () => {
      // Список связей обновляется в фоне: подтверждение сохранения не должно
      // ждать перезагрузку списка, а сбой обновления не отменяет уже
      // сохранённую связь.
      void queryClient.invalidateQueries({ queryKey: ['links'] })
    },
  })
}
