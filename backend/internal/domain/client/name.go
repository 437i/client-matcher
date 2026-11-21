package client

import (
	"slices"

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

func GivenNameAliases(a, b string) bool {
	return slices.Contains(nameAliases[a], b)
}

// ProbableNameMatch compares aligned words of a normalized component.
func ProbableNameMatch(a, b string) bool {
	left, right := strings.Fields(a), strings.Fields(b)
	if len(left) != len(right) {
		return false
	}
	for i, x := range left {
		y := right[i]
		if folded, ok := foldMixedCyrillicLatin(x); ok {
			x = folded
		}
		if folded, ok := foldMixedCyrillicLatin(y); ok {
			y = folded
		}
		if x == y {
			continue
		}
		if strings.Contains(x, y) || strings.Contains(y, x) || runeCount(x) < 5 || runeCount(y) < 5 || !oneEditApart(x, y) {
			return false
		}
	}
	return true
}
