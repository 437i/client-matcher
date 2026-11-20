package client

import (
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

var nameCaseFolder = cases.Fold()

// NormalizeName возвращает каноническое представление строки для сравнения:
// Unicode NFC, регистронезависимое сравнение, ё = е и нормализованные пробелы.
// Исходное имя при этом не изменяется.
func NormalizeName(s string) string {
	s = norm.NFC.String(s)
	s = nameCaseFolder.String(s)
	s = norm.NFC.String(s)
	s = strings.ReplaceAll(s, "ё", "е")
	return strings.Join(strings.Fields(s), " ")
}
