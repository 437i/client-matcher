import { describe, expect, it } from 'vitest'

import { fieldText, formatCalendarDate, formatLinkDateTime, missingValue } from './format'

describe('formatCalendarDate', () => {
  it('преобразует календарную дату в привычный формат', () => {
    expect(formatCalendarDate('1982-04-15')).toBe('15.04.1982')
    expect(formatCalendarDate('1990-01-02')).toBe('02.01.1990')
  })

  it('не смещает дату в часовом поясе, отличном от UTC', () => {
    const original = process.env['TZ']
    try {
      // В поясах со смещением назад разбор "1982-04-15" как полуночи UTC
      // сдвинул бы дату на сутки назад.
      process.env['TZ'] = 'America/Los_Angeles'
      expect(formatCalendarDate('1982-04-15')).toBe('15.04.1982')

      process.env['TZ'] = 'Pacific/Kiritimati'
      expect(formatCalendarDate('1982-04-15')).toBe('15.04.1982')

      process.env['TZ'] = 'Asia/Almaty'
      expect(formatCalendarDate('1982-04-15')).toBe('15.04.1982')
    } finally {
      process.env['TZ'] = original
    }
  })

  it('показывает обозначение отсутствия данных вместо пустого места', () => {
    expect(formatCalendarDate(undefined)).toBe(missingValue)
    expect(formatCalendarDate('')).toBe(missingValue)
    expect(formatCalendarDate('не дата')).toBe(missingValue)
    expect(formatCalendarDate('1982-4-5')).toBe(missingValue)
  })
})

describe('formatLinkDateTime', () => {
  it('показывает дату и время в локальном часовом поясе', () => {
    const value = formatLinkDateTime('2026-11-12T16:50:46Z')
    expect(value).toMatch(/^\d{2}\.\d{2}\.\d{4}, \d{2}:\d{2}$/)
  })

  it('показывает обозначение отсутствия данных вместо пустого места', () => {
    expect(formatLinkDateTime(undefined)).toBe(missingValue)
    expect(formatLinkDateTime('не дата')).toBe(missingValue)
  })
})

describe('fieldText', () => {
  it('возвращает значение, если оно есть', () => {
    expect(fieldText('1352')).toBe('1352')
  })

  it('возвращает обозначение отсутствия данных, если значения нет', () => {
    expect(fieldText(undefined)).toBe(missingValue)
    expect(fieldText('')).toBe(missingValue)
    expect(fieldText('   ')).toBe(missingValue)
  })
})
