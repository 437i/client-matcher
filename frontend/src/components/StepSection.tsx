import type { ReactNode } from 'react'

import { Card, CardContent } from '@/components/ui/card'

type StepSectionProps = {
  /** Номер шага: 1, 2, 3, 4. */
  step: number
  title: string
  description?: string
  children: ReactNode
}

/**
 * Пронумерованный шаг рабочего сценария. Номер и название шага видны всегда,
 * чтобы сотрудник понимал, где он находится и что делать дальше.
 */
export function StepSection({ step, title, description, children }: StepSectionProps) {
  return (
    <Card className="gap-0 py-0">
      <CardContent className="flex flex-col gap-4 py-6">
        <div className="flex items-start gap-3">
          <span
            aria-hidden="true"
            className="flex size-8 shrink-0 items-center justify-center rounded-full bg-primary text-base font-semibold text-primary-foreground"
          >
            {step}
          </span>
          <div className="flex flex-col gap-1">
            <h2 className="text-lg font-semibold text-foreground">
              <span className="sr-only">Шаг {step}. </span>
              {title}
            </h2>
            {description ? (
              <p className="text-sm text-muted-foreground">{description}</p>
            ) : null}
          </div>
        </div>
        {children}
      </CardContent>
    </Card>
  )
}
