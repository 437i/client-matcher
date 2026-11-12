/**
 * Состояние рабочего сценария и правила актуальности пары.
 *
 * Ключевое правило: пара «клиент основной базы + запись сайта» актуальна,
 * только если совпадают обе её части. Запись сайта помнит, для какого клиента
 * и для какого номера она была загружена, поэтому смена клиента или правка
 * номера делают её недействительной — и визуально, и перед отправкой
 * запроса на сохранение.
 *
 * Асинхронный поиск защищён идентификатором запроса: результат, который
 * пришёл после смены клиента или номера, не восстанавливает актуальность
 * пары.
 */

export type SelectedCrmClient = {
  id: string
  fullName?: string | undefined
  birthday?: string | undefined
  clinic?: string | undefined
}

export type SiteRecord = {
  id: string
  shortName?: string | undefined
  lastName?: string | undefined
  birthday?: string | undefined
}

/** Почему загруженная запись сайта перестала быть действительной. */
export type StaleReason = 'site-id-changed' | 'crm-changed' | 'crm-selection-open'

/** Пара клиента и записи, которая уже сохранена. */
export type SavedPair = {
  crmId: string
  siteId: string
}

export type SiteLookupState = {
  status: 'idle' | 'loading' | 'loaded' | 'failed'
  record?: SiteRecord | undefined
  /** Клиент основной базы, для которого запись была загружена. */
  loadedForCrmId?: string | undefined
  /** Номер записи, который был введён при загрузке. */
  loadedForSiteId?: string | undefined
  /** Идентификатор запроса, результат которого сейчас в силе. */
  requestId?: number | undefined
  stale?: StaleReason | undefined
  errorMessage?: string | undefined
}

export type WorkflowState = {
  selected?: SelectedCrmClient | undefined
  /** Открыт ли выбор другого клиента основной базы. */
  choosingCrm: boolean
  siteIdInput: string
  site: SiteLookupState
  /** Пара, которую уже сохранили: повторно её сохранять нельзя. */
  savedPair?: SavedPair | undefined
}

export const initialWorkflowState: WorkflowState = {
  choosingCrm: false,
  siteIdInput: '',
  site: { status: 'idle' },
}

export type WorkflowAction =
  | { type: 'start-changing-crm' }
  | { type: 'select-crm'; client: SelectedCrmClient }
  | { type: 'change-site-id'; value: string }
  | { type: 'site-lookup-started'; requestId: number }
  | {
      type: 'site-lookup-succeeded'
      requestId: number
      crmId: string
      siteId: string
      record: SiteRecord
    }
  | { type: 'site-lookup-failed'; requestId: number; message: string }
  | { type: 'link-saved'; crmId: string; siteId: string }

export const staleMessages: Record<StaleReason, string> = {
  'site-id-changed':
    'Номер изменился. Найдите запись на сайте заново, чтобы сохранить связь.',
  'crm-changed':
    'Клиент основной базы изменён. Найдите запись на сайте заново — прежняя запись уже не подходит.',
  'crm-selection-open':
    'Вы выбираете другого клиента основной базы. Найдите запись на сайте заново — прежняя запись уже не подходит.',
}

/**
 * Загруженная запись сайта становится недействительной: прежние данные не
 * должны молча использоваться для новой связи. Вместе с записью
 * обнуляется идентификатор запроса — запоздавший ответ больше не применим.
 */
function invalidateSite(site: SiteLookupState, reason: StaleReason): SiteLookupState {
  if (site.status === 'idle' && site.record === undefined && site.requestId === undefined) {
    // Запись ещё не загружалась — сбрасывать нечего.
    return site.stale ? { ...site, stale: reason } : site
  }
  return { status: 'idle', stale: reason, requestId: undefined }
}

export function workflowReducer(state: WorkflowState, action: WorkflowAction): WorkflowState {
  switch (action.type) {
    case 'start-changing-crm':
      return {
        ...state,
        choosingCrm: true,
        site: invalidateSite(state.site, 'crm-selection-open'),
      }

    case 'select-crm':
      return {
        ...state,
        selected: action.client,
        choosingCrm: false,
        site: invalidateSite(state.site, 'crm-changed'),
      }

    case 'change-site-id': {
      const trimmed = action.value.trim()
      // Номер изменился — прежний ответ (и текущий запрос) больше не подходят.
      const site =
        state.site.loadedForSiteId !== trimmed
          ? invalidateSite(state.site, 'site-id-changed')
          : state.site
      return { ...state, siteIdInput: action.value, site }
    }

    case 'site-lookup-started':
      return { ...state, site: { status: 'loading', requestId: action.requestId } }

    case 'site-lookup-succeeded':
      // Ответ устаревшего запроса не должен восстанавливать актуальность пары.
      if (state.site.requestId !== action.requestId) {
        return state
      }
      return {
        ...state,
        site: {
          status: 'loaded',
          record: action.record,
          loadedForCrmId: action.crmId,
          loadedForSiteId: action.siteId,
          requestId: action.requestId,
        },
      }

    case 'site-lookup-failed':
      // Ошибка устаревшего запроса не должна заменять состояние нового поиска.
      if (state.site.requestId !== action.requestId) {
        return state
      }
      return {
        ...state,
        site: {
          status: 'failed',
          errorMessage: action.message,
          requestId: action.requestId,
        },
      }

    case 'link-saved':
      return {
        ...state,
        savedPair: { crmId: action.crmId, siteId: action.siteId },
      }
  }
}

/** Пара актуальна: выбранный клиент и загруженная запись относятся друг к другу. */
export function isPairCurrent(state: WorkflowState): boolean {
  const { selected, site, siteIdInput } = state

  if (!selected || selected.id.trim() === '') {
    return false
  }
  if (state.choosingCrm) {
    return false
  }
  if (site.status !== 'loaded' || !site.record) {
    return false
  }
  if (site.loadedForCrmId !== selected.id) {
    return false
  }
  if (site.loadedForSiteId !== siteIdInput.trim()) {
    return false
  }
  // Запись должна быть той же, по номеру которой искали.
  if (site.record.id.trim() !== site.loadedForSiteId) {
    return false
  }
  return true
}

/** Пара уже сохранена — повторно сохранять её нельзя. */
export function isPairAlreadySaved(state: WorkflowState): boolean {
  const saved = state.savedPair
  if (!saved) {
    return false
  }
  const crmId = state.selected?.id?.trim() ?? ''
  const siteId = state.site.record?.id?.trim() ?? ''
  return saved.crmId === crmId && saved.siteId === siteId && crmId !== '' && siteId !== ''
}

/** Сохранение доступно только для актуальной пары с обоими номерами. */
export function canSave(state: WorkflowState): boolean {
  if (!isPairCurrent(state)) {
    return false
  }
  const siteId = state.site.record?.id?.trim() ?? ''
  if (siteId === '') {
    return false
  }
  return !isPairAlreadySaved(state)
}

/**
 * Подтверждение сохранения показывается только для той пары, которая сейчас
 * на экране и действительно сохранена.
 */
export function savedPairMessage(state: WorkflowState): string | undefined {
  const saved = state.savedPair
  if (!saved) {
    return undefined
  }
  if (!isPairCurrent(state)) {
    return undefined
  }
  if (saved.crmId !== state.selected?.id?.trim()) {
    return undefined
  }
  if (saved.siteId !== state.siteIdInput.trim()) {
    return undefined
  }
  return `Готово. Связь сохранена: клиент основной базы № ${saved.crmId} и запись на сайте № ${saved.siteId}.`
}

/** Причина, по которой запись сайта требует повторной проверки. */
export function staleNotice(state: WorkflowState): string | undefined {
  const reason = state.site.stale
  return reason ? staleMessages[reason] : undefined
}
