import { useId, useState } from 'react'

import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

type CrmSearchFormProps = {
  /** Текст о состоянии поиска: что происходит сейчас и что делать дальше. */
  statusText: string
  /** Текст ошибки загрузки списка клиентов. */
  errorMessage?: string | undefined
  /** Идёт ли загрузка списка клиентов. */
  isLoading: boolean
  /** Текущий текст в поле поиска: нужен, чтобы понять, что запрос изменён. */
  onQueryChange: (query: string) => void
  onSearch: (query: string) => void
  /** Ссылка на поле поиска: на него возвращается фокус. */
  inputRef?: React.Ref<HTMLInputElement>
}

/**
 * Шаг 1: поиск человека в основной базе.
 * Запрос отправляется только по явному действию — кнопкой или Enter.
 */
export function CrmSearchForm({
  statusText,
  errorMessage,
  isLoading,
  onQueryChange,
  onSearch,
  inputRef,
}: CrmSearchFormProps) {
  const [query, setQuery] = useState('')
  const [validationMessage, setValidationMessage] = useState<string | undefined>(undefined)
  const inputId = useId()
  const hintId = useId()
  const errorId = useId()

  function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const trimmed = query.trim()
    if (trimmed === '') {
      setValidationMessage('Введите имя или фамилию клиента')
      return
    }
    setValidationMessage(undefined)
    onSearch(trimmed)
  }

  return (
    <form className="flex flex-col gap-3" onSubmit={handleSubmit} noValidate>
      <div className="flex flex-col gap-2">
        <Label htmlFor={inputId}>Имя или фамилия клиента</Label>
        <Input
          id={inputId}
          ref={inputRef}
          name="crm-query"
          value={query}
          autoComplete="off"
          onChange={(event) => {
            setQuery(event.target.value)
            onQueryChange(event.target.value)
            if (validationMessage) {
              setValidationMessage(undefined)
            }
          }}
          aria-describedby={validationMessage ? `${hintId} ${errorId}` : hintId}
          aria-invalid={validationMessage ? true : undefined}
          className="text-base"
        />
        <p id={hintId} className="text-sm text-muted-foreground">
          Введите фамилию или часть имени — например, Иванов или Иван. Поиск ищет по началу слова,
          «ё» и «е» считаются одинаковыми.
        </p>
      </div>

      {validationMessage ? (
        <p id={errorId} role="alert" className="text-sm font-medium text-destructive">
          {validationMessage}
        </p>
      ) : null}

      <div className="flex flex-wrap items-center gap-3">
        <Button type="submit">
          {isLoading ? 'Ищем клиентов…' : 'Найти клиента'}
        </Button>
        {statusText ? (
          <p aria-live="polite" className="text-sm text-muted-foreground">
            {statusText}
          </p>
        ) : null}
      </div>

      {errorMessage ? (
        <Alert variant="destructive" className="border-destructive">
          <AlertDescription>{errorMessage}</AlertDescription>
        </Alert>
      ) : null}
    </form>
  )
}
