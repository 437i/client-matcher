import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import {
  Table,
  TableBody,
  TableCaption,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { formatLinkDateTime, fieldText } from '@/lib/format'
import type { Link } from '@/api/client'

type SavedLinksSectionProps = {
  links: Link[] | undefined
  isLoading: boolean
  errorMessage?: string | undefined
  onRetry: () => void
}

/** Адрес выгрузки: это обычная ссылка, браузер скачивает файл сам. */
export const exportCsvUrl = '/api/v1/links/export.csv'

/**
 * Раздел сохранённых связей и выгрузка CSV.
 */
export function SavedLinksSection({ links, isLoading, errorMessage, onRetry }: SavedLinksSectionProps) {
  const isEmpty = links !== undefined && links.length === 0
  const canExport = links !== undefined && !isLoading && !errorMessage

  return (
    <Card className="gap-0 py-0">
      <CardContent className="flex flex-col gap-4 py-6">
        <div className="flex flex-col gap-1">
          <h2 className="text-lg font-semibold text-foreground">Сохранённые связи</h2>
          {links && links.length > 0 ? null : (
            <p className="text-sm text-muted-foreground">
              Здесь появятся сохранённые связи. После первого сохранения они будут видны в этом списке.
            </p>
          )}
        </div>

        {isLoading ? (
          <p aria-live="polite" className="text-sm text-muted-foreground">
            Загружаем список связей…
          </p>
        ) : null}

        {errorMessage ? (
          <Alert variant="destructive" className="border-destructive">
            <AlertDescription>{errorMessage}</AlertDescription>
          </Alert>
        ) : null}

        {isEmpty ? (
          <p className="text-sm text-muted-foreground">
            Пока ни одной связи нет. Когда вы сохраните первую, она появится в этом списке.
          </p>
        ) : null}

        {links && links.length > 0 ? (
          <>
            <Table
              containerProps={{
                role: 'region',
                'aria-label': 'Сохранённые связи',
                tabIndex: 0,
                className:
                  'rounded-lg focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background',
              }}
            >
              <TableCaption className="sr-only">
                Сохранённые связи между клиентами основной базы и записями сайта
              </TableCaption>
              <TableHeader>
                <TableRow>
                  <TableHead scope="col">Номер в основной базе</TableHead>
                  <TableHead scope="col">Номер записи на сайте</TableHead>
                  <TableHead scope="col">Сохранено</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {links.map((link) => (
                  <TableRow key={link.id ?? `${link.crm_id ?? ''}-${link.site_id ?? ''}`}>
                    <TableCell>{fieldText(link.crm_id)}</TableCell>
                    <TableCell>{fieldText(link.site_id)}</TableCell>
                    <TableCell>{formatLinkDateTime(link.created_at)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </>
        ) : null}

        <div className="flex flex-col gap-2">
          <div className="flex flex-wrap items-center gap-3">
            {canExport ? (
              <Button asChild variant="outline">
                <a href={exportCsvUrl} download>
                  Скачать CSV
                </a>
              </Button>
            ) : (
              // Пока список не загружен, показываем настоящую отключённую
              // кнопку: ссылка <a> не умеет быть отключённой.
              <Button type="button" variant="outline" disabled>
                Скачать CSV
              </Button>
            )}
            {!canExport ? (
              <p className="text-sm text-muted-foreground">
                Сначала дождитесь, пока загрузится список связей, — без него файл выгрузить нельзя.
              </p>
            ) : null}
          </div>
          <p className="text-sm text-muted-foreground">
            В файле будут пары номеров: номер в основной базе и номер записи на сайте. Если файл не
            скачался, попробуйте ещё раз.
          </p>
          {errorMessage ? (
            <div>
              <Button type="button" variant="outline" onClick={onRetry}>
                Повторить
              </Button>
            </div>
          ) : null}
        </div>
      </CardContent>
    </Card>
  )
}
