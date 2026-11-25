package matcher

import (
	"strings"
	"unicode/utf8"

	"github.com/437i/client-matcher/backend/internal/domain/client"
)

type titleRole struct {
	client.NameSpan
	direct bool
}

func assessTitle(crm client.CRMClient, site client.SiteClient) (FieldCheck, FieldCheck, *FullNameMismatch) {
	raw := crm.SourceFullName
	words := client.TitleWords(raw)
	ufLast, ufFirst, ufPatronymic := client.ConfirmedTitleRoles(words, crm)
	last := resolveTitleRole(words, site.LastName, ufLast)
	first := resolveTitleRole(words, site.ShortName, ufFirst)
	hadFirst := first.Present()
	if last.Overlaps(first.NameSpan) {
		switch {
		case last.direct && !first.direct:
			first = titleRole{}
		case first.direct && !last.direct:
			last = titleRole{}
		default:
			last, first = titleRole{}, titleRole{}
		}
	}
	if last.Present() && !hadFirst && client.NormalizeName(site.ShortName) != "" {
		first.NameSpan = uniqueAlias(words, last.NameSpan, site.ShortName)
	}
	lastCheck := titleCheck(raw, words, last, site.LastName, FieldLastName)
	firstCheck := titleCheck(raw, words, first, site.ShortName, FieldFirstName)
	if agrees(lastCheck.Code) && agrees(firstCheck.Code) && !explainedRemainder(raw, words, last.NameSpan, first.NameSpan, ufPatronymic) {
		return lastCheck, firstCheck, &FullNameMismatch{CRMValue: raw, SiteValue: strings.TrimSpace(site.ShortName + " " + site.LastName)}
	}
	return lastCheck, firstCheck, nil
}

func resolveTitleRole(words []client.NameWord, site string, uf client.NameSpan) titleRole {
	direct := client.UniqueNameSpan(words, site)
	if direct.Present() {
		return titleRole{NameSpan: direct, direct: true}
	}
	if client.NormalizeName(site) == "" {
		return titleRole{}
	}
	return titleRole{NameSpan: uf}
}

func uniqueAlias(words []client.NameWord, last client.NameSpan, value string) client.NameSpan {
	normalized := client.NormalizeName(value)
	var found client.NameSpan
	// Stage 0 aliases represent single words.
	for i, word := range words {
		if last.Contains(i) || !client.GivenNameAliases(word.Primary, normalized) {
			continue
		}
		if found.Present() {
			return client.NameSpan{}
		}
		found = client.NameSpan{Start: i, End: i + 1}
	}
	return found
}

func titleCheck(raw string, words []client.NameWord, role titleRole, site string, field Field) FieldCheck {
	var part string
	if role.Present() {
		part = raw[words[role.Start].Start:words[role.End-1].End]
	}
	code := compareName(part, site, field == FieldFirstName)
	if role.direct && code != CodeExact {
		code = CodeNormalizedMatch
	}
	return FieldCheck{Field: field, Code: code, CRMValue: value(part), SiteValue: value(site)}
}

func agrees(code Code) bool {
	return code == CodeExact || code == CodeNormalizedMatch || code == CodeAliasMatch || code == CodeProbableMatch
}

func explainedRemainder(raw string, words []client.NameWord, last, first, patronymic client.NameSpan) bool {
	remaining := -1
	for i := range words {
		if last.Contains(i) || first.Contains(i) {
			continue
		}
		if remaining >= 0 {
			return false
		}
		remaining = i
	}
	if remaining < 0 {
		return true
	}
	if patronymic.Start == remaining && patronymic.End == remaining+1 {
		return true
	}
	word := words[remaining]
	if !word.HasSpaceBoundaries(raw) {
		return false
	}
	normalized := client.NormalizeName(raw[word.Start:word.End])
	for _, r := range normalized {
		if r < 'а' || r > 'я' {
			return false
		}
	}
	for _, suffix := range []string{"ович", "евич", "овна", "евна", "ична"} {
		if stem, ok := strings.CutSuffix(normalized, suffix); ok {
			return utf8.RuneCountInString(stem) >= 2
		}
	}
	return false
}
