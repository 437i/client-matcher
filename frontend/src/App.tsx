import { useReducer, useRef, useState } from 'react'

import { useCrmClientSearch, useCreateLink, useLinks, useSiteClientLookup } from '@/api/client'
import { messageForError } from '@/api/errors'
import { StepSection } from '@/components/StepSection'
import { CrmCandidates, type CrmCandidate } from '@/features/crm-search/CrmCandidates'
import { CrmSearchForm } from '@/features/crm-search/CrmSearchForm'
import { LinkReviewStep } from '@/features/link-review/LinkReviewStep'
import { SavedLinksSection } from '@/features/saved-links/SavedLinksSection'
import { SiteLookupStep } from '@/features/site-lookup/SiteLookupStep'
import {
  canSave,
  initialWorkflowState,
  isPairAlreadySaved,
  isPairCurrent,
  savedPairMessage,
  staleNotice,
  workflowReducer,
  type SelectedCrmClient,
  type SiteRecord,
} from '@/lib/workflow'

export default function App() {
  const [state, dispatch] = useReducer(workflowReducer, initialWorkflowState)
  const [submittedQuery, setSubmittedQuery] = useState('')
  const [queryInput, setQueryInput] = useState('')
  const [saveError, setSaveError] = useState<string | undefined>(undefined)
  // Повторное нажатие не должно создавать вторую связь: пока запрос не
  // завершён, новые запросы не отправляются.
  const savingRef = useRef(false)
  // Идентификатор запроса поиска записи: он растёт при каждом поиске, поэтому
  // запоздавший ответ нельзя применить к текущему состоянию.
  const lookupRequestIdRef = useRef(0)
  // Поле поиска: на него возвращается фокус, когда выбирать клиента нечего.
  const searchInputRef = useRef<HTMLInputElement>(null)

  const search = useCrmClientSearch(submittedQuery)
  const lookup = useSiteClientLookup()
  const createLink = useCreateLink()
  const links = useLinks()

  const isSaving = createLink.isPending
  const isQueryStale = queryInput.trim() !== submittedQuery
  const hasSearchError = search.isError
  // Старые результаты нельзя предлагать как результаты изменённого запроса,
  // а после ошибки показывать прошлый список как пустой успешный поиск.
  const candidates: CrmCandidate[] =
    hasSearchError || isQueryStale ? [] : (search.data ?? [])
  const pairIsCurrent = isPairCurrent(state)
  const notice = staleNotice(state)
  const successMessage = savedPairMessage(state)
  const pairAlreadySaved = isPairAlreadySaved(state)

  const searchStatusText = getSearchStatusText({
    submittedQuery,
    isFetching: search.isFetching,
    hasSearchError,
    foundCount: candidates.length,
    isQueryStale,
  })

  function handleQueryChange(value: string) {
    setQueryInput(value)
  }

  function handleSearch(query: string) {
    const trimmed = query.trim()
    if (trimmed === '') {
      return
    }

    setQueryInput(trimmed)

    if (trimmed === submittedQuery) {
      // Тот же текст запроса: повторяем поиск, а не меняем условия. Иначе
      // после ошибки сотрудник не смог бы повторить поиск, не изменив введённое.
      if (!search.isFetching) {
        void search.refetch()
      }
      return
    }

    setSubmittedQuery(trimmed)
  }

  function handleSelectCrm(client: SelectedCrmClient) {
    // Пока идёт сохранение, менять клиента нельзя: запрос уходит с той парой,
    // которую сотрудник уже проверил.
    if (isSaving) {
      return
    }
    setSaveError(undefined)
    dispatch({ type: 'select-crm', client })
  }

  function handleStartChangingCrm() {
    if (isSaving) {
      return
    }
    setSaveError(undefined)
    dispatch({ type: 'start-changing-crm' })
  }

  function handleSiteIdChange(value: string) {
    if (isSaving) {
      return
    }
    setSaveError(undefined)
    dispatch({ type: 'change-site-id', value })
  }

  function handleSiteLookup() {
    // Пока идёт сохранение, поиск записи запускать нельзя: запрос уходит с той
    // парой, которую сотрудник уже проверил.
    if (isSaving) {
      return
    }

    const siteId = state.siteIdInput.trim()
    const crmId = state.selected?.id?.trim() ?? ''
    if (siteId === '' || crmId === '') {
      return
    }

    setSaveError(undefined)
    const requestId = lookupRequestIdRef.current + 1
    lookupRequestIdRef.current = requestId
    dispatch({ type: 'site-lookup-started', requestId })

    lookup.mutate(
      { siteId },
      {
        onSuccess: (record) => {
          if (!record) {
            dispatch({
              type: 'site-lookup-failed',
              requestId,
              message: 'Не удалось получить данные. Проверьте подключение и попробуйте ещё раз.',
            })
            return
          }

          const recordId = record.id?.trim() ?? ''
          if (recordId === '') {
            dispatch({
              type: 'site-lookup-failed',
              requestId,
              message:
                'Полученная запись без номера. Найдите запись на сайте заново и попробуйте ещё раз.',
            })
            return
          }
          if (recordId !== siteId) {
            dispatch({
              type: 'site-lookup-failed',
              requestId,
              message:
                'Полученная запись не соответствует введённому номеру. Найдите запись на сайте заново и попробуйте ещё раз.',
            })
            return
          }

          const siteRecord: SiteRecord = {
            id: recordId,
            shortName: record.short_name,
            lastName: record.last_name,
            birthday: record.birthday,
          }
          dispatch({ type: 'site-lookup-succeeded', requestId, crmId, siteId, record: siteRecord })
        },
        onError: (error: unknown) => {
          dispatch({ type: 'site-lookup-failed', requestId, message: messageForError(error) })
        },
      },
    )
  }

  function handleSave() {
    // Повторная проверка актуальности пары перед отправкой: без неё можно
    // было бы сохранить связь для другой записи или сохранить её дважды.
    if (!canSave(state)) {
      return
    }
    if (savingRef.current) {
      return
    }

    const crmId = state.selected?.id?.trim() ?? ''
    const siteId = state.site.record?.id?.trim() ?? ''
    setSaveError(undefined)
    savingRef.current = true

    createLink.mutate(
      { crm_id: crmId, site_id: siteId },
      {
        onSuccess: () => {
          dispatch({ type: 'link-saved', crmId, siteId })
        },
        onError: (error: unknown) => {
          setSaveError(messageForError(error))
        },
        onSettled: () => {
          savingRef.current = false
        },
      },
    )
  }

  return (
    <div className="min-h-svh bg-background">
      <header className="border-b border-border bg-card">
        <div className="mx-auto w-full max-w-4xl px-4 py-6">
          <h1 className="text-2xl font-semibold text-foreground">
            Связать клиента с записью на сайте
          </h1>
          <p className="mt-2 text-base text-muted-foreground">
            Найдите человека в основной базе, затем найдите его запись на сайте и сохраните связь.
          </p>
        </div>
      </header>

      <main className="mx-auto flex w-full max-w-4xl flex-col gap-6 px-4 py-8">
        <StepSection
          step={1}
          title="Найдите человека в основной базе (CRM)"
          description="Основная база — это CRM, где ведётся список клиентов фонда."
        >
          <CrmSearchForm
            statusText={searchStatusText}
            errorMessage={
              hasSearchError ? 'Не удалось получить список клиентов. Попробуйте ещё раз.' : undefined
            }
            isLoading={search.isFetching}
            inputRef={searchInputRef}
            onQueryChange={handleQueryChange}
            onSearch={handleSearch}
          />
        </StepSection>

        <StepSection step={2} title="Выберите клиента">
          <CrmCandidates
            candidates={candidates}
            selected={state.selected}
            isChoosing={state.choosingCrm}
            isQueryStale={isQueryStale}
            isSaving={isSaving}
            searchInputRef={searchInputRef}
            onSelect={handleSelectCrm}
            onStartChanging={handleStartChangingCrm}
          />
        </StepSection>

        <StepSection
          step={3}
          title="Найдите его запись на сайте"
          description="Номер записи вы получаете, когда добавляете клиента на сайт."
        >
          <SiteLookupStep
            siteIdInput={state.siteIdInput}
            onSiteIdChange={handleSiteIdChange}
            onSubmit={handleSiteLookup}
            isLoading={state.site.status === 'loading'}
            isSaving={isSaving}
            record={state.site.record}
            errorMessage={state.site.status === 'failed' ? state.site.errorMessage : undefined}
            staleNotice={notice}
            hasSelectedCrm={Boolean(state.selected)}
          />
        </StepSection>

        <StepSection step={4} title="Проверьте данные и сохраните связь">
          <LinkReviewStep
            crmClient={state.selected}
            siteRecord={pairIsCurrent ? state.site.record : undefined}
            isSaveEnabled={canSave(state)}
            isAlreadySaved={pairAlreadySaved}
            isSaving={isSaving}
            onSave={handleSave}
            saveError={saveError}
            successMessage={successMessage}
          />
        </StepSection>

        <SavedLinksSection
          links={links.data}
          isLoading={links.isFetching}
          errorMessage={
            links.isError
              ? 'Не удалось получить список сохранённых связей. Попробуйте ещё раз.'
              : undefined
          }
          onRetry={() => void links.refetch()}
        />
      </main>
    </div>
  )
}

type SearchStatus = {
  submittedQuery: string
  isFetching: boolean
  hasSearchError: boolean
  foundCount: number
  isQueryStale: boolean
}

/**
 * Больше 50 совпадений backend не отдаёт (`GET /api/v1/crm-clients` в
 * `backend/openapi/swagger.yaml`: «Возвращает не более 50 записей»).
 * Об этом сотрудник должен знать: иначе он решит, что других клиентов нет.
 */
const crmSearchLimit = 50

function getSearchStatusText({
  submittedQuery,
  isFetching,
  hasSearchError,
  foundCount,
  isQueryStale,
}: SearchStatus): string {
  if (isFetching) {
    return 'Ищем клиентов…'
  }
  if (hasSearchError) {
    // Об ошибке сообщает отдельное оповещение — здесь она не выглядит как
    // пустой успешный поиск.
    return ''
  }
  if (submittedQuery === '') {
    return 'Здесь появятся найденные клиенты. Введите имя или фамилию выше и нажмите «Найти клиента».'
  }
  if (isQueryStale) {
    return 'Вы изменили условия поиска. Нажмите «Найти клиента», чтобы увидеть результаты.'
  }
  if (foundCount === 0) {
    return 'Никого не нашли. Проверьте написание и попробуйте ввести только фамилию или только имя.'
  }
  if (foundCount >= crmSearchLimit) {
    return `Найдено записей: ${foundCount}. Показаны первые ${crmSearchLimit} совпадений. Уточните фамилию или имя, чтобы найти нужного человека`
  }
  return `Найдено записей: ${foundCount}`
}
