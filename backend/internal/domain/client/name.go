package client

import "strings"

// NormalizeName приводит имя к виду, пригодному для сравнения и поиска:
// без лишних пробелов, в нижнем регистре, «ё» приравнена к «е».
func NormalizeName(s string) string {
	s = strings.ToLower(strings.Join(strings.Fields(s), " "))
	return strings.ReplaceAll(s, "ё", "е")
}
