import { useEffect, useRef } from 'react'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { formatCalendarDate, fieldText } from '@/lib/format'
import type { SelectedCrmClient } from '@/lib/workflow'

export type CrmCandidate = {
  id?: string | undefined
  full_name?: string | undefined
  birthday?: string | undefined
  clinic?: string | undefined
}

type CrmCandidatesProps = {
  candidates: CrmCandidate[]
  /** Выбранный клиент основной базы. */
  selected?: SelectedCrmClient | undefined
  /** Открыт ли выбор другого клиента. */
  isChoosing: boolean
  /** Изменён ли текст запроса после последнего поиска. */
  isQueryStale: boolean
  /** Идёт ли сохранение связи: пока оно идёт, выбор менять нельзя. */
  isSaving: boolean
  /** Поле поиска: на него возвращается фокус, если выбирать пока нечего. */
  searchInputRef?: React.RefObject<HTMLInputElement | null> | undefined
  onSelect: (client: SelectedCrmClient) => void
  onStartChanging: () => void
}

/**
 * Шаг 2: выбор клиента основной базы.
 * Кандидат никогда не выбирается автоматически: выбор всегда делает сотрудник.
 *
 * Фокус ведётся за действиями сотрудника: после выбора он переходит на
 * выбранного клиента, после «Изменить выбор» — на список кандидатов, а если
 * выбирать пока нечего — на поле поиска. Фокус не перемещается сам по себе
 * при обновлении результатов.
 */
export function CrmCandidates({
  candidates,
  selected,
  isChoosing,
  isQueryStale,
  isSaving,
  searchInputRef,
  onSelect,
  onStartChanging,
}: CrmCandidatesProps) {
  const listRef = useRef<HTMLUListElement>(null)
  const wasChoosing = useRef(isChoosing)

  useEffect(() => {
    // Фокус переводится только в момент возврата к списку кандидатов:
    // обновление самих результатов фокус не перемещает.
    const choosingStarted = isChoosing && !wasChoosing.current
    wasChoosing.current = isChoosing

    if (!choosingStarted) {
      return
    }
    if (candidates.length > 0) {
      listRef.current?.focus()
      return
    }
    searchInputRef?.current?.focus()
  }, [isChoosing, candidates.length, searchInputRef])

  if (selected && !isChoosing) {
    return (
      <SelectedCrmCard selected={selected} isSaving={isSaving} onStartChanging={onStartChanging} />
    )
  }

  if (isQueryStale) {
    return (
      <p className="text-sm text-muted-foreground">
        Вы изменили условия поиска. Нажмите «Найти клиента», чтобы увидеть результаты по новым условиям.
      </p>
    )
  }

  return (
    <ul ref={listRef} tabIndex={-1} aria-label="Найденные клиенты" className="flex flex-col gap-3">
      {candidates.map((candidate, index) => (
        <li key={candidate.id ?? `candidate-${index}`}>
          <CandidateCard
            candidate={candidate}
            isSaving={isSaving}
            onSelect={onSelect}
          />
        </li>
      ))}
    </ul>
  )
}

function CandidateCard({
  candidate,
  isSaving,
  onSelect,
}: {
  candidate: CrmCandidate
  isSaving: boolean
  onSelect: (client: SelectedCrmClient) => void
}) {
  const id = candidate.id?.trim() ?? ''

  return (
    <Card className="gap-0 py-0">
      <CardContent className="flex flex-col gap-3 py-4">
        <p className="text-lg font-semibold text-foreground">{fieldText(candidate.full_name)}</p>
        <dl className="flex flex-col gap-1 text-sm">
          <div className="flex flex-wrap gap-x-2">
            <dt className="text-muted-foreground">Дата рождения:</dt>
            <dd className="text-foreground">{formatCalendarDate(candidate.birthday)}</dd>
          </div>
          <div className="flex flex-wrap gap-x-2">
            <dt className="text-muted-foreground">Организация:</dt>
            <dd className="text-foreground">{fieldText(candidate.clinic)}</dd>
          </div>
          <div className="flex flex-wrap gap-x-2">
            <dt className="text-muted-foreground">Номер в основной базе:</dt>
            <dd className="text-foreground">{fieldText(candidate.id)}</dd>
          </div>
        </dl>
        <div>
          <Button
            type="button"
            variant="secondary"
            disabled={id === '' || isSaving}
            onClick={() =>
              onSelect({
                id,
                fullName: candidate.full_name,
                birthday: candidate.birthday,
                clinic: candidate.clinic,
              })
            }
          >
            Выбрать этого клиента
          </Button>
          {id === '' ? (
            <p className="mt-2 text-sm text-muted-foreground">
              У этого клиента нет номера, сохранить связь с ним нельзя.
            </p>
          ) : null}
        </div>
      </CardContent>
    </Card>
  )
}

/**
 * Выбранный клиент основной базы.
 *
 * Фокус переходит на имя выбранного клиента: список кандидатов исчезает
 * вместе с активной кнопкой, и без явного перевода фокуса клавиатурный
 * сотрудник теряет место в интерфейсе.
 */
function SelectedCrmCard({
  selected,
  isSaving,
  onStartChanging,
}: {
  selected: SelectedCrmClient
  isSaving: boolean
  onStartChanging: () => void
}) {
  const nameRef = useRef<HTMLParagraphElement>(null)

  useEffect(() => {
    nameRef.current?.focus()
  }, [])

  return (
    <Card className="gap-0 border-primary/40 py-0">
      <CardContent className="flex flex-col gap-3 py-4">
        <Badge variant="secondary" className="w-fit">
          Основная база (CRM)
        </Badge>
        <p ref={nameRef} tabIndex={-1} className="text-lg font-semibold text-foreground">
          {fieldText(selected.fullName)}
        </p>
        <dl className="flex flex-col gap-1 text-sm">
          <div className="flex flex-wrap gap-x-2">
            <dt className="text-muted-foreground">Дата рождения:</dt>
            <dd className="text-foreground">{formatCalendarDate(selected.birthday)}</dd>
          </div>
          <div className="flex flex-wrap gap-x-2">
            <dt className="text-muted-foreground">Организация:</dt>
            <dd className="text-foreground">{fieldText(selected.clinic)}</dd>
          </div>
          <div className="flex flex-wrap gap-x-2">
            <dt className="text-muted-foreground">Номер в основной базе:</dt>
            <dd className="text-foreground">{fieldText(selected.id)}</dd>
          </div>
        </dl>
        <div>
          <Button type="button" variant="outline" disabled={isSaving} onClick={onStartChanging}>
            Изменить выбор
          </Button>
        </div>
      </CardContent>
    </Card>
  )
}
