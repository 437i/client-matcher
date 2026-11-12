import { useId, useState } from 'react'

import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { formatCalendarDate, fieldText } from '@/lib/format'
import type { SiteRecord } from '@/lib/workflow'

type SiteLookupStepProps = {
  siteIdInput: string
  onSiteIdChange: (value: string) => void
  onSubmit: () => void
  isLoading: boolean
  /** Идёт ли сохранение связи: пока оно идёт, номер менять нельзя. */
  isSaving: boolean
  /** Найденная запись сайта. */
  record?: SiteRecord | undefined
  /** Текст ошибки поиска записи. */
  errorMessage?: string | undefined
  /** Сообщение о том, что запись нужно найти заново. */
  staleNotice?: string | undefined
  /** Выбран ли клиент основной базы. */
  hasSelectedCrm: boolean
}

/**
 * Шаг 3: поиск записи на сайте по номеру, который сотрудник получил при
 * добавлении клиента на сайт.
 */
export function SiteLookupStep({
  siteIdInput,
  onSiteIdChange,
  onSubmit,
  isLoading,
  isSaving,
  record,
  errorMessage,
  staleNotice,
  hasSelectedCrm,
}: SiteLookupStepProps) {
  const [validationMessage, setValidationMessage] = useState<string | undefined>(undefined)
  const inputId = useId()
  const hintId = useId()
  const errorId = useId()

  // Объявление результата поиска: загрузка и успех. Ошибка объявляется
  // отдельным оповещением, поэтому в этой строке её нет.
  const lookupStatusText = isLoading
    ? 'Ищем запись на сайте…'
    : record
      ? 'Запись на сайте найдена.'
      : ''

  function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (siteIdInput.trim() === '') {
      setValidationMessage('Введите номер записи на сайте')
      return
    }
    setValidationMessage(undefined)
    onSubmit()
  }

  return (
    <div className="flex flex-col gap-4">
      <form className="flex flex-col gap-3" onSubmit={handleSubmit} noValidate>
        <div className="flex flex-col gap-2">
          <Label htmlFor={inputId}>Номер записи на сайте</Label>
          <Input
            id={inputId}
            name="site-id"
            value={siteIdInput}
            inputMode="numeric"
            autoComplete="off"
            disabled={isSaving}
            onChange={(event) => {
              onSiteIdChange(event.target.value)
              if (validationMessage) {
                setValidationMessage(undefined)
              }
            }}
            aria-describedby={validationMessage ? `${hintId} ${errorId}` : hintId}
            aria-invalid={validationMessage ? true : undefined}
            className="text-base"
          />
          <p id={hintId} className="text-sm text-muted-foreground">
            Это номер, который вы получили при добавлении клиента на сайт. Например: 3586.
          </p>
          {validationMessage ? (
            <p id={errorId} role="alert" className="text-sm font-medium text-destructive">
              {validationMessage}
            </p>
          ) : null}
        </div>

        <div className="flex flex-wrap items-center gap-3">
          <Button type="submit" disabled={!hasSelectedCrm || isLoading || isSaving}>
            {isLoading ? 'Ищем запись…' : 'Найти запись на сайте'}
          </Button>
          {!hasSelectedCrm ? (
            <p className="text-sm text-muted-foreground">
              Сначала выберите человека в основной базе — без этого найти запись на сайте нельзя.
            </p>
          ) : null}
        </div>
      </form>

      {/*
        Одна живая область для результата поиска: загрузка и успех
        объявляются здесь, ошибка — отдельным оповещением ниже. Дублирующих
        объявлений нет, фокус сам по себе не перемещается.
      */}
      {lookupStatusText ? (
        <p role="status" aria-live="polite" className="text-sm text-muted-foreground">
          {lookupStatusText}
        </p>
      ) : null}

      {staleNotice ? (
        <Alert role="status" className="border-warning bg-warning-surface text-warning">
          <AlertDescription>{staleNotice}</AlertDescription>
        </Alert>
      ) : null}

      {errorMessage ? (
        <Alert variant="destructive" className="border-destructive">
          <AlertDescription>{errorMessage}</AlertDescription>
        </Alert>
      ) : null}

      {record ? (
        <Card className="gap-0 border-primary/40 py-0">
          <CardContent className="flex flex-col gap-3 py-4">
            <Badge variant="secondary" className="w-fit">
              База сайта
            </Badge>
            <dl className="flex flex-col gap-1 text-sm">
              <div className="flex flex-wrap gap-x-2">
                <dt className="text-muted-foreground">Имя:</dt>
                <dd className="text-foreground">{fieldText(record.shortName)}</dd>
              </div>
              <div className="flex flex-wrap gap-x-2">
                <dt className="text-muted-foreground">Фамилия:</dt>
                <dd className="text-foreground">{fieldText(record.lastName)}</dd>
              </div>
              <div className="flex flex-wrap gap-x-2">
                <dt className="text-muted-foreground">Дата рождения:</dt>
                <dd className="text-foreground">{formatCalendarDate(record.birthday)}</dd>
              </div>
              <div className="flex flex-wrap gap-x-2">
                <dt className="text-muted-foreground">Номер записи на сайте:</dt>
                <dd className="text-foreground">{fieldText(record.id)}</dd>
              </div>
            </dl>
          </CardContent>
        </Card>
      ) : null}
    </div>
  )
}
