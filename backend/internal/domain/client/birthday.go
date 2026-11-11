package client

import (
	"fmt"
	"strings"
	"time"
)

type Date struct {
	Day   int
	Month int
	Year  int
}

// ParseDate разбирает дату в формате YYYY-MM-DD.
func ParseDate(value string) (Date, error) {
	parsed, err := time.Parse(time.DateOnly, strings.TrimSpace(value))
	if err != nil {
		return Date{}, fmt.Errorf("parse date %q: %w", value, err)
	}
	return Date{
		Day:   parsed.Day(),
		Month: int(parsed.Month()),
		Year:  parsed.Year(),
	}, nil
}

// String возвращает дату в формате YYYY-MM-DD.
func (d Date) String() string {
	return fmt.Sprintf("%04d-%02d-%02d", d.Year, d.Month, d.Day)
}

// IsValid проверяет корректность календарной даты.
func (d Date) IsValid() bool {
	if d.Year < 1 || d.Year > 9999 {
		return false
	}

	if d.Month < 1 || d.Month > 12 {
		return false
	}

	if d.Day < 1 {
		return false
	}

	parsed := time.Date(
		d.Year,
		time.Month(d.Month),
		d.Day,
		0, 0, 0, 0,
		time.UTC,
	)

	return parsed.Year() == d.Year &&
		int(parsed.Month()) == d.Month &&
		parsed.Day() == d.Day
}
