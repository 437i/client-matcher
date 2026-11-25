package client

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// NameWord retains the offsets of a Stage 0 primary token in the original title.
type NameWord struct {
	Primary    string
	Start, End int
}

type NameSpan struct{ Start, End int }

func (s NameSpan) Present() bool       { return s.End > s.Start }
func (s NameSpan) Contains(i int) bool { return s.Start <= i && i < s.End }
func (s NameSpan) Overlaps(other NameSpan) bool {
	return s.Present() && other.Present() && s.Start < other.End && other.Start < s.End
}

func TitleWords(raw string) []NameWord {
	var words []NameWord
	var primary strings.Builder
	var start, end int
	flush := func() {
		if primary.Len() == 0 {
			return
		}
		words = append(words, NameWord{Primary: primary.String(), Start: start, End: end})
		primary.Reset()
	}
	// NFC segments preserve source positions even when folding changes length.
	var iter norm.Iter
	iter.InitString(norm.NFC, raw)
	for !iter.Done() {
		pos := iter.Pos()
		segment := strings.ReplaceAll(foldNameCase(string(iter.Next())), "ё", "е")
		for _, r := range segment {
			if unicode.IsLetter(r) || unicode.IsNumber(r) {
				if primary.Len() == 0 {
					start = pos
				}
				end = iter.Pos()
				primary.WriteRune(r)
			} else {
				flush()
			}
		}
	}
	flush()
	return words
}

func UniqueNameSpan(words []NameWord, role string) NameSpan {
	tokens := TokenizeName(role)
	if len(tokens) == 0 {
		return NameSpan{}
	}
	var found NameSpan
	for i := 0; i+len(tokens) <= len(words); i++ {
		match := true
		for j, token := range tokens {
			if words[i+j].Primary != token.Forms[0].Value {
				match = false
				break
			}
		}
		if match {
			if found.Present() {
				return NameSpan{}
			}
			found = NameSpan{Start: i, End: i + len(tokens)}
		}
	}
	return found
}

func ConfirmedTitleRoles(words []NameWord, crm CRMClient) (last, first, patronymic NameSpan) {
	roles := [...]NameSpan{UniqueNameSpan(words, crm.LastName), UniqueNameSpan(words, crm.FirstName), UniqueNameSpan(words, crm.Patronymic)}
	confirmed := roles
	for i, role := range roles {
		for j := i + 1; j < len(roles); j++ {
			if role.Overlaps(roles[j]) {
				confirmed[i], confirmed[j] = NameSpan{}, NameSpan{}
			}
		}
	}
	return confirmed[0], confirmed[1], confirmed[2]
}

func (w NameWord) HasSpaceBoundaries(raw string) bool {
	before, _ := utf8.DecodeLastRuneInString(raw[:w.Start])
	after, _ := utf8.DecodeRuneInString(raw[w.End:])
	return (w.Start == 0 || unicode.IsSpace(before)) && (w.End == len(raw) || unicode.IsSpace(after))
}
