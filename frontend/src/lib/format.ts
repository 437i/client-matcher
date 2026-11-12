/**
 * Показ дат и значений, которых может не быть в ответе.
 *
 * В спецификации backend у схем ответов нет списка `required`, поэтому
 * сгенерированные типы считают поля необязательными. Отсутствующее значение
 * показывается обозначением «—», а не подавляется приведением типов.
 */

/** Обозначение отсутствующего значения. */
export const missingValue = '—'

/**
 * Дата рождения приходит календарной датой в формате YYYY-MM-DD.
 * Это не метка времени, поэтому строка разбирается на компоненты без
 * обращения к Date: `new Date("1982-04-15")` по стандарту разбирается как
 * полночь по UTC и в часовых поясах со смещением назад сдвинул бы дату
 * на сутки.
 */
export function formatCalendarDate(value: string | undefined): string {
  if (!value) {
    return missingValue
  }

  const parts = value.trim().split('-')
  if (parts.length !== 3) {
    return missingValue
  }

  const [year, month, day] = parts
  if (year === undefined || month === undefined || day === undefined) {
    return missingValue
  }
  if (!/^\d{4}$/.test(year) || !/^\d{2}$/.test(month) || !/^\d{2}$/.test(day)) {
    return missingValue
  }

  return `${day}.${month}.${year}`
}

/**
 * Дата и время сохранения связи приходят в формате RFC 3339 с указанием
 * смещения, поэтому разбираются обычным Date и показываются в локальном
 * часовом поясе браузера.
 */
export function formatLinkDateTime(value: string | undefined): string {
  if (!value) {
    return missingValue
  }

  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) {
    return missingValue
  }

  const day = String(parsed.getDate()).padStart(2, '0')
  const month = String(parsed.getMonth() + 1).padStart(2, '0')
  const year = parsed.getFullYear()
  const hours = String(parsed.getHours()).padStart(2, '0')
  const minutes = String(parsed.getMinutes()).padStart(2, '0')

  return `${day}.${month}.${year}, ${hours}:${minutes}`
}

/** Значение поля для показа: само значение или обозначение его отсутствия. */
export function fieldText(value: string | undefined): string {
  const trimmed = value?.trim()
  return trimmed ? trimmed : missingValue
}
