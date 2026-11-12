import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const here = dirname(fileURLToPath(import.meta.url))
const srcDir = join(here, '..')

/**
 * Слова, которые rules/non-technical-ux.md запрещает в пользовательских
 * текстах интерфейса.
 */
const forbiddenWords = [
  'endpoint',
  'API',
  'JSON',
  'HTTP',
  'запрос',
  'payload',
  'сервер',
  'сервис',
  'backend',
  'frontend',
  'мутация',
  'кэш',
  'статус-код',
  'UUID',
]

/** Файлы с пользовательскими текстами (без примитивов shadcn/ui). */
const userTextFiles = [
  'App.tsx',
  'components/StepSection.tsx',
  'features/crm-search/CrmSearchForm.tsx',
  'features/crm-search/CrmCandidates.tsx',
  'features/site-lookup/SiteLookupStep.tsx',
  'features/link-review/LinkReviewStep.tsx',
  'features/saved-links/SavedLinksSection.tsx',
  'api/errors.ts',
]

/** Строковые литералы и текстовые строки JSX. */
function extractUserStrings(source: string): string[] {
  const found: string[] = []

  for (const match of source.matchAll(/"([^"\\\n]{3,})"|'([^'\\\n]{3,})'/g)) {
    found.push(match[1] ?? match[2] ?? '')
  }

  for (const line of source.split('\n')) {
    const stripped = line.trim()
    if (/^[А-Яа-яЁё][А-Яа-яЁё ,.:;—«»()-]*$/.test(stripped) && stripped.length > 8) {
      found.push(stripped)
    }
  }

  // Пользователю показываются только русские тексты: строки без кириллицы —
  // это пути импортов, классы CSS и прочая техническая разметка, а не текст
  // интерфейса.
  return found.filter((text) => /[А-Яа-яЁё]/.test(text))
}

describe('оформление интерфейса', () => {
  it('уважает системную настройку уменьшения анимации', () => {
    const css = readFileSync(join(srcDir, 'index.css'), 'utf-8')

    expect(css).toContain('@media (prefers-reduced-motion: reduce)')
    expect(css).toMatch(/animation-duration:\s*0\.01ms\s*!important/)
    expect(css).toMatch(/transition-duration:\s*0\.01ms\s*!important/)
  })

  it('не уменьшает размер текста в полях ввода на широком экране', () => {
    const input = readFileSync(join(srcDir, 'components/ui/input.tsx'), 'utf-8')

    // Основной текст — не меньше 16 CSS px, поэтому уменьшение на md-экранах
    // в примитиве поля недопустимо.
    expect(input).not.toContain('md:text-sm')
  })

  it('не содержит запрещённых слов в пользовательских текстах', () => {
    const problems: string[] = []

    for (const file of userTextFiles) {
      const source = readFileSync(join(srcDir, file), 'utf-8')
      for (const text of extractUserStrings(source)) {
        for (const word of forbiddenWords) {
          // Регистр не учитывается: запрещено и «Запрос», и «запрос».
          if (text.toLowerCase().includes(word.toLowerCase())) {
            problems.push(`${file}: «${word}» → «${text}»`)
          }
        }
      }
    }

    expect(problems).toEqual([])
  })
})
