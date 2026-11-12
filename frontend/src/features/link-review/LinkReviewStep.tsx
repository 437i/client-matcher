import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { formatCalendarDate, fieldText } from '@/lib/format'
import type { SelectedCrmClient, SiteRecord } from '@/lib/workflow'

type LinkReviewStepProps = {
  crmClient?: SelectedCrmClient | undefined
  siteRecord?: SiteRecord | undefined
  /** Доступно ли сохранение: пара актуальна и оба номера на месте. */
  isSaveEnabled: boolean
  /** Пара уже сохранена — повторно сохранять её нельзя. */
  isAlreadySaved: boolean
  isSaving: boolean
  onSave: () => void
  /** Текст ошибки сохранения. */
  saveError?: string | undefined
  /** Текст подтверждения сохранённой пары. */
  successMessage?: string | undefined
}

/**
 * Шаг 4: сверка обеих записей и явное сохранение связи.
 * Связь создаётся только по нажатию кнопки — автоматического сохранения нет.
 */
export function LinkReviewStep({
  crmClient,
  siteRecord,
  isSaveEnabled,
  isAlreadySaved,
  isSaving,
  onSave,
  saveError,
  successMessage,
}: LinkReviewStepProps) {
  const crmId = crmClient?.id?.trim() ?? ''
  const siteId = siteRecord?.id?.trim() ?? ''

  return (
    <div className="flex flex-col gap-4">
      <Card className="gap-0 py-0">
        <CardContent className="flex flex-col gap-4 py-4">
          <h3 className="text-base font-semibold text-foreground">Проверьте данные</h3>

          <div className="grid gap-4 md:grid-cols-2">
            <div className="flex flex-col gap-2 rounded-lg border border-border p-4">
              <Badge variant="secondary" className="w-fit">
                Основная база (CRM)
              </Badge>
              <p className="text-base font-semibold text-foreground">
                {fieldText(crmClient?.fullName)}
              </p>
              <dl className="flex flex-col gap-1 text-sm">
                <div className="flex flex-wrap gap-x-2">
                  <dt className="text-muted-foreground">Дата рождения:</dt>
                  <dd className="text-foreground">{formatCalendarDate(crmClient?.birthday)}</dd>
                </div>
                <div className="flex flex-wrap gap-x-2">
                  <dt className="text-muted-foreground">Организация:</dt>
                  <dd className="text-foreground">{fieldText(crmClient?.clinic)}</dd>
                </div>
                <div className="flex flex-wrap gap-x-2">
                  <dt className="text-muted-foreground">Номер в основной базе:</dt>
                  <dd className="text-foreground">{fieldText(crmId)}</dd>
                </div>
              </dl>
            </div>

            <div className="flex flex-col gap-2 rounded-lg border border-border p-4">
              <Badge variant="secondary" className="w-fit">
                База сайта
              </Badge>
              <p className="text-base font-semibold text-foreground">
                {fieldText(siteRecord?.shortName)} {fieldText(siteRecord?.lastName)}
              </p>
              <dl className="flex flex-col gap-1 text-sm">
                <div className="flex flex-wrap gap-x-2">
                  <dt className="text-muted-foreground">Дата рождения:</dt>
                  <dd className="text-foreground">{formatCalendarDate(siteRecord?.birthday)}</dd>
                </div>
                <div className="flex flex-wrap gap-x-2">
                  <dt className="text-muted-foreground">Номер записи на сайте:</dt>
                  <dd className="text-foreground">{fieldText(siteId)}</dd>
                </div>
              </dl>
            </div>
          </div>
        </CardContent>
      </Card>

      <div className="flex flex-col gap-3">
        <div className="flex flex-wrap items-center gap-3">
          <Button type="button" disabled={!isSaveEnabled || isSaving} onClick={onSave}>
            {isSaving ? 'Сохраняем связь…' : 'Сохранить связь'}
          </Button>
          {!successMessage && !saveError ? (
            <p className="text-sm text-muted-foreground">
              Связь сохранится и появится в списке сохранённых связей ниже.
            </p>
          ) : null}
        </div>

        {!isSaveEnabled ? (
          <p className="text-sm text-muted-foreground">
            {isAlreadySaved
              ? 'Эта связь уже сохранена. Повторно сохранять её не нужно.'
              : 'Сначала найдите запись на сайте — без неё сохранить связь нельзя.'}
          </p>
        ) : null}

        {successMessage ? (
          <Alert role="status" className="border-success bg-success-surface text-success">
            <AlertTitle>Готово</AlertTitle>
            <AlertDescription>{successMessage}</AlertDescription>
          </Alert>
        ) : null}

        {saveError ? (
          <Alert variant="destructive" className="border-destructive">
            <AlertDescription>{saveError}</AlertDescription>
          </Alert>
        ) : null}
      </div>
    </div>
  )
}
