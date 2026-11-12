import { describe, expect, it } from 'vitest'

import {
  canSave,
  initialWorkflowState,
  isPairAlreadySaved,
  isPairCurrent,
  savedPairMessage,
  staleNotice,
  workflowReducer,
  type WorkflowState,
} from './workflow'

const clientA = { id: '1352', fullName: 'Иванов Иван', birthday: '1982-04-15', clinic: 'Организация' }
const clientC = { id: '1354', fullName: 'Королёв Пётр', birthday: '2001-09-09', clinic: 'НИИ' }
const siteRecord = { id: '3586', shortName: 'Ваня', lastName: 'Иванов', birthday: '1982-04-15' }

function stateWithLoadedPair(): WorkflowState {
  let state = workflowReducer(initialWorkflowState, { type: 'select-crm', client: clientA })
  state = workflowReducer(state, { type: 'change-site-id', value: '3586' })
  state = workflowReducer(state, { type: 'site-lookup-started', requestId: 1 })
  state = workflowReducer(state, {
    type: 'site-lookup-succeeded',
    requestId: 1,
    crmId: '1352',
    siteId: '3586',
    record: siteRecord,
  })
  return state
}

describe('актуальность пары', () => {
  it('пара актуальна, когда клиент и запись найдены и совпадают', () => {
    const state = stateWithLoadedPair()
    expect(isPairCurrent(state)).toBe(true)
    expect(canSave(state)).toBe(true)
  })

  it('смена клиента основной базы делает запись сайта недействительной', () => {
    const state = workflowReducer(stateWithLoadedPair(), { type: 'select-crm', client: clientC })

    expect(isPairCurrent(state)).toBe(false)
    expect(canSave(state)).toBe(false)
    expect(state.site.record).toBeUndefined()
    expect(staleNotice(state)).toContain('Клиент основной базы изменён')
  })

  it('повторный выбор того же клиента тоже делает запись недействительной', () => {
    const state = workflowReducer(stateWithLoadedPair(), { type: 'select-crm', client: clientA })

    expect(isPairCurrent(state)).toBe(false)
    expect(canSave(state)).toBe(false)
  })

  it('возврат к выбору показывает, что запись нужно проверить заново', () => {
    const state = workflowReducer(stateWithLoadedPair(), { type: 'start-changing-crm' })

    expect(isPairCurrent(state)).toBe(false)
    expect(canSave(state)).toBe(false)
    expect(staleNotice(state)).toContain('Вы выбираете другого клиента основной базы')
  })

  it('изменение номера записи делает запись сайта недействительной', () => {
    const state = workflowReducer(stateWithLoadedPair(), { type: 'change-site-id', value: '3587' })

    expect(isPairCurrent(state)).toBe(false)
    expect(canSave(state)).toBe(false)
    expect(staleNotice(state)).toContain('Номер изменился')
  })

  it('введение того же номера не сбрасывает запись', () => {
    const state = workflowReducer(stateWithLoadedPair(), { type: 'change-site-id', value: ' 3586 ' })

    expect(isPairCurrent(state)).toBe(true)
    expect(canSave(state)).toBe(true)
  })

  it('запись без номера не позволяет сохранить связь', () => {
    let state = workflowReducer(initialWorkflowState, { type: 'select-crm', client: clientA })
    state = workflowReducer(state, { type: 'change-site-id', value: '3586' })
    state = workflowReducer(state, { type: 'site-lookup-started', requestId: 1 })
    state = workflowReducer(state, {
      type: 'site-lookup-succeeded',
      requestId: 1,
      crmId: '1352',
      siteId: '3586',
      record: { id: '', shortName: 'Ваня' },
    })

    expect(isPairCurrent(state)).toBe(false)
    expect(canSave(state)).toBe(false)
  })

  it('номер возвращённой записи должен совпадать с номером поиска', () => {
    let state = workflowReducer(initialWorkflowState, { type: 'select-crm', client: clientA })
    state = workflowReducer(state, { type: 'change-site-id', value: '3586' })
    state = workflowReducer(state, { type: 'site-lookup-started', requestId: 1 })
    state = workflowReducer(state, {
      type: 'site-lookup-succeeded',
      requestId: 1,
      crmId: '1352',
      siteId: '3586',
      record: { id: '9999', shortName: 'Ваня' },
    })

    expect(isPairCurrent(state)).toBe(false)
    expect(canSave(state)).toBe(false)
  })

  it('ошибка поиска записи не оставляет прежние данные действительными', () => {
    let state = stateWithLoadedPair()
    state = workflowReducer(state, { type: 'site-lookup-started', requestId: 2 })
    state = workflowReducer(state, {
      type: 'site-lookup-failed',
      requestId: 2,
      message: 'Запись не найдена',
    })

    expect(isPairCurrent(state)).toBe(false)
    expect(state.site.record).toBeUndefined()
    expect(state.site.errorMessage).toBe('Запись не найдена')
  })
})

describe('устаревшие ответы поиска', () => {
  it('успешный ответ старого запроса не восстанавливает актуальность пары', () => {
    let state = workflowReducer(initialWorkflowState, { type: 'select-crm', client: clientA })
    state = workflowReducer(state, { type: 'change-site-id', value: '3586' })
    state = workflowReducer(state, { type: 'site-lookup-started', requestId: 1 })

    // Пока идёт поиск, сотрудник меняет клиента, потом возвращается к первому.
    state = workflowReducer(state, { type: 'select-crm', client: clientC })
    state = workflowReducer(state, { type: 'select-crm', client: clientA })

    // Приходит ответ первого поиска — он уже неактуален.
    const afterStale = workflowReducer(state, {
      type: 'site-lookup-succeeded',
      requestId: 1,
      crmId: '1352',
      siteId: '3586',
      record: siteRecord,
    })

    expect(afterStale).toBe(state)
    expect(isPairCurrent(afterStale)).toBe(false)
    expect(canSave(afterStale)).toBe(false)
    expect(afterStale.site.record).toBeUndefined()
  })

  it('успешный ответ старого запроса не применяется после изменения номера', () => {
    let state = workflowReducer(initialWorkflowState, { type: 'select-crm', client: clientA })
    state = workflowReducer(state, { type: 'change-site-id', value: '3586' })
    state = workflowReducer(state, { type: 'site-lookup-started', requestId: 1 })
    state = workflowReducer(state, { type: 'change-site-id', value: '3587' })

    const afterStale = workflowReducer(state, {
      type: 'site-lookup-succeeded',
      requestId: 1,
      crmId: '1352',
      siteId: '3586',
      record: siteRecord,
    })

    expect(afterStale).toBe(state)
    expect(isPairCurrent(afterStale)).toBe(false)
    expect(afterStale.site.record).toBeUndefined()
  })

  it('ошибка старого запроса не заменяет состояние нового поиска', () => {
    let state = workflowReducer(initialWorkflowState, { type: 'select-crm', client: clientA })
    state = workflowReducer(state, { type: 'change-site-id', value: '3586' })
    state = workflowReducer(state, { type: 'site-lookup-started', requestId: 1 })
    state = workflowReducer(state, { type: 'change-site-id', value: '3587' })
    state = workflowReducer(state, { type: 'site-lookup-started', requestId: 2 })
    state = workflowReducer(state, {
      type: 'site-lookup-succeeded',
      requestId: 2,
      crmId: '1352',
      siteId: '3587',
      record: { id: '3587', shortName: 'Маша' },
    })

    const afterStaleError = workflowReducer(state, {
      type: 'site-lookup-failed',
      requestId: 1,
      message: 'Старая ошибка',
    })

    expect(afterStaleError).toBe(state)
    expect(afterStaleError.site.status).toBe('loaded')
    expect(afterStaleError.site.errorMessage).toBeUndefined()
  })

  it('результат последнего запроса принимается', () => {
    let state = workflowReducer(initialWorkflowState, { type: 'select-crm', client: clientA })
    state = workflowReducer(state, { type: 'change-site-id', value: '3587' })
    state = workflowReducer(state, { type: 'site-lookup-started', requestId: 2 })
    state = workflowReducer(state, {
      type: 'site-lookup-succeeded',
      requestId: 2,
      crmId: '1352',
      siteId: '3587',
      record: { id: '3587', shortName: 'Маша' },
    })

    expect(state.site.status).toBe('loaded')
    expect(isPairCurrent(state)).toBe(true)
    expect(canSave(state)).toBe(true)
  })
})

describe('повторное сохранение той же пары', () => {
  it('после сохранения ту же пару сохранить повторно нельзя', () => {
    let state = stateWithLoadedPair()
    state = workflowReducer(state, { type: 'link-saved', crmId: '1352', siteId: '3586' })

    expect(isPairAlreadySaved(state)).toBe(true)
    expect(canSave(state)).toBe(false)
  })

  it('подтверждение показывается только для сохранённой пары', () => {
    let state = stateWithLoadedPair()
    state = workflowReducer(state, { type: 'link-saved', crmId: '1352', siteId: '3586' })

    expect(savedPairMessage(state)).toBe(
      'Готово. Связь сохранена: клиент основной базы № 1352 и запись на сайте № 3586.',
    )
  })

  it('подтверждение исчезает, когда пара изменилась', () => {
    let state = stateWithLoadedPair()
    state = workflowReducer(state, { type: 'link-saved', crmId: '1352', siteId: '3586' })
    state = workflowReducer(state, { type: 'change-site-id', value: '3587' })

    expect(savedPairMessage(state)).toBeUndefined()
  })

  it('другая пара после сохранения первой сохраняется', () => {
    let state = stateWithLoadedPair()
    state = workflowReducer(state, { type: 'link-saved', crmId: '1352', siteId: '3586' })
    state = workflowReducer(state, { type: 'change-site-id', value: '3587' })
    state = workflowReducer(state, { type: 'site-lookup-started', requestId: 2 })
    state = workflowReducer(state, {
      type: 'site-lookup-succeeded',
      requestId: 2,
      crmId: '1352',
      siteId: '3587',
      record: { id: '3587', shortName: 'Маша' },
    })

    expect(canSave(state)).toBe(true)
  })
})
