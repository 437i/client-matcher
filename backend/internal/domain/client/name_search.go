package client

import (
	"sort"
	"strings"
	"unicode"
)

// SearchFormKind показывает, откуда получилась поисковая форма токена.
type SearchFormKind uint8

const (
	SearchFormPrimary SearchFormKind = iota
	SearchFormAlias
	SearchFormHomoglyph
)

// SearchForm — одна форма написания слова. Forms нужны только для поиска:
// в ответ API всегда возвращается исходное ФИО из источника.
type SearchForm struct {
	Value string
	Kind  SearchFormKind
}

// SearchToken представляет одно слово ФИО и допустимые формы его написания.
type SearchToken struct {
	Forms       []SearchForm
	IsGivenName bool // true только для компонента FirstName в записи кандидата
}

// nameAliasGroups содержит только распространённые варианты имён, которые
// встречаются в тестовых данных проекта. Это словарь поиска, а не доказательство
// того, что два человека — один и тот же человек.
var nameAliasGroups = [][]string{
	{"ваня", "иван"},
	{"саша", "александр", "александра"},
	{"соня", "софья"},
	{"даша", "дарья"},
	{"настя", "анастасия"},
	{"маша", "мария"},
	{"тима", "тимофей"},
	{"лиза", "елизавета"},
	{"даня", "даниил"},
	{"варя", "варвара"},
	{"лёва", "лев"},
	{"петя", "пётр"},
}

var nameAliases = buildNameAliases()

func buildNameAliases() map[string][]string {
	aliases := make(map[string][]string)
	for _, group := range nameAliasGroups {
		normalized := make([]string, 0, len(group))
		for _, value := range group {
			value = NormalizeName(value)
			if value != "" {
				normalized = append(normalized, value)
			}
		}
		for _, value := range normalized {
			for _, alias := range normalized {
				if alias != value {
					aliases[value] = append(aliases[value], alias)
				}
			}
		}
	}
	return aliases
}

// TokenizeName разбивает строку на поисковые токены и строит формы для
// смешения латиницы с кириллицей. Варианты имён добавляются отдельно, потому
// что они применимы к имени, но не к фамилии и отчеству.
func TokenizeName(value string) []SearchToken {
	return tokenizeName(value, false, false)
}

// TokenizeSearchQuery расширяет каждое слово поискового запроса известными
// вариантами имён: пользователь может ввести «Саша», а в CRM хранится «Александр».
func TokenizeSearchQuery(value string) []SearchToken {
	return tokenizeName(value, true, false)
}

// TokenizeCRMName строит поисковое представление структурированного ФИО.
// Словарь кратких имён применяется только к FirstName, но не к фамилии или
// отчеству. Исходные поля CRM при этом не изменяются.
func TokenizeCRMName(value CRMClient) []SearchToken {
	tokens := make([]SearchToken, 0, 5)
	tokens = append(tokens, tokenizeName(value.LastName, false, false)...)
	tokens = append(tokens, tokenizeName(value.FirstName, true, true)...)
	tokens = append(tokens, tokenizeName(value.Patronymic, false, false)...)
	return tokens
}

func tokenizeName(value string, addAliases, isGivenName bool) []SearchToken {
	normalized := NormalizeName(value)
	words := make([]string, 0, 4)
	var current strings.Builder

	flush := func() {
		if current.Len() == 0 {
			return
		}
		words = append(words, current.String())
		current.Reset()
	}

	for _, r := range normalized {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			current.WriteRune(r)
		} else {
			flush()
		}
	}
	flush()

	tokens := make([]SearchToken, 0, len(words))
	for _, word := range words {
		forms := []SearchForm{{Value: word, Kind: SearchFormPrimary}}

		if folded, ok := foldMixedCyrillicLatin(word); ok && folded != word {
			forms = append(forms, SearchForm{Value: folded, Kind: SearchFormHomoglyph})
		}

		if addAliases {
			for _, alias := range nameAliases[word] {
				forms = append(forms, SearchForm{Value: alias, Kind: SearchFormAlias})
			}
		}

		tokens = append(tokens, SearchToken{Forms: forms, IsGivenName: isGivenName})
	}

	return tokens
}

// MatchNameTokens возвращает оценку совпадения или 0, если запрос не подходит.
// Каждое слово запроса должно соответствовать отдельному слову ФИО. Благодаря
// этому запрос «Иван Иванов» не совпадёт с одной фамилией «Иванова».
// Чем выше оценка, тем точнее совпадение.
func MatchNameTokens(query, candidate []SearchToken) int {
	if len(query) == 0 || len(query) > len(candidate) || len(candidate) > 63 {
		return 0
	}

	scores := make([][]int, len(query))
	order := make([]int, len(query))
	for i := range query {
		order[i] = i
		scores[i] = make([]int, len(candidate))
		for j := range candidate {
			scores[i][j] = matchToken(query[i], candidate[j])
		}
	}

	// Сначала назначаем наиболее специфичные слова запроса. Это сокращает
	// перебор и не даёт короткому слову занять единственный подходящий токен.
	sort.SliceStable(order, func(i, j int) bool {
		left, right := order[i], order[j]
		leftCount, rightCount := 0, 0
		leftBest, rightBest := 0, 0
		for k := range candidate {
			if scores[left][k] > 0 {
				leftCount++
				if scores[left][k] > leftBest {
					leftBest = scores[left][k]
				}
			}
			if scores[right][k] > 0 {
				rightCount++
				if scores[right][k] > rightBest {
					rightBest = scores[right][k]
				}
			}
		}
		if leftCount != rightCount {
			return leftCount < rightCount
		}
		if leftBest != rightBest {
			return leftBest > rightBest
		}
		return primaryValue(query[left]) > primaryValue(query[right])
	})

	type state struct {
		queryIndex int
		used       uint64
	}
	memo := make(map[state]int)
	const impossible = -1

	var assign func(int, uint64) int
	assign = func(index int, used uint64) int {
		if index == len(order) {
			return 0
		}
		key := state{queryIndex: index, used: used}
		if cached, ok := memo[key]; ok {
			return cached
		}

		queryIndex := order[index]
		best := impossible
		for candidateIndex, score := range scores[queryIndex] {
			bit := uint64(1) << candidateIndex
			if score == 0 || used&bit != 0 {
				continue
			}
			rest := assign(index+1, used|bit)
			if rest != impossible && score+rest > best {
				best = score + rest
			}
		}
		memo[key] = best
		return best
	}

	score := assign(0, 0)
	if score < 0 {
		return 0
	}
	return score
}

func primaryValue(token SearchToken) string {
	if len(token.Forms) == 0 {
		return ""
	}
	return token.Forms[0].Value
}

func matchToken(query, candidate SearchToken) int {
	best := 0
	for _, q := range query.Forms {
		// Краткие формы имён разрешены только для компонента FirstName.
		// Иначе, например, запрос «Александр» мог бы находить фамилию «Саша».
		if q.Kind == SearchFormAlias && !candidate.IsGivenName {
			continue
		}
		for _, c := range candidate.Forms {
			if q.Value == c.Value {
				score := 1000
				switch {
				case q.Kind == SearchFormAlias || c.Kind == SearchFormAlias:
					score = 820
				case q.Kind == SearchFormHomoglyph || c.Kind == SearchFormHomoglyph:
					score = 950
				}
				if score > best {
					best = score
				}
				continue
			}

			if runeCount(q.Value) >= 2 && strings.HasPrefix(c.Value, q.Value) {
				score := 700
				if q.Kind == SearchFormAlias || c.Kind == SearchFormAlias {
					score = 620
				} else if q.Kind == SearchFormHomoglyph || c.Kind == SearchFormHomoglyph {
					score = 680
				}
				if score > best {
					best = score
				}
			}

			if runeCount(q.Value) >= 3 && strings.Contains(c.Value, q.Value) {
				score := 500
				if q.Kind == SearchFormAlias || c.Kind == SearchFormAlias {
					score = 450
				} else if q.Kind == SearchFormHomoglyph || c.Kind == SearchFormHomoglyph {
					score = 480
				}
				if score > best {
					best = score
				}
			}
		}
	}

	// Нечёткое сравнение ограничено одной опечаткой и длинными словами.
	// Проверяем также альтернативные формы смешанной кириллицы/латиницы,
	// иначе опечатка рядом с подменённым символом не будет найдена.
	// Варианты имён для fuzzy matching не используются.
	if best < 300 {
		for _, q := range query.Forms {
			if q.Kind == SearchFormAlias || runeCount(q.Value) < 5 {
				continue
			}
			for _, c := range candidate.Forms {
				if c.Kind == SearchFormAlias || runeCount(c.Value) < 5 {
					continue
				}
				if oneEditApart(q.Value, c.Value) {
					best = 300
					break
				}
			}
			if best == 300 {
				break
			}
		}
	}

	return best
}

func runeCount(value string) int {
	return len([]rune(value))
}

// oneEditApart допускает одну замену, вставку, удаление или перестановку
// соседних символов (частую опечатку при ручном вводе).
func oneEditApart(a, b string) bool {
	left, right := []rune(a), []rune(b)
	if abs(len(left)-len(right)) > 1 {
		return false
	}

	if len(left) == len(right) {
		mismatches := make([]int, 0, 2)
		for i := range left {
			if left[i] != right[i] {
				mismatches = append(mismatches, i)
				if len(mismatches) > 2 {
					return false
				}
			}
		}
		if len(mismatches) <= 1 {
			return true
		}
		first, second := mismatches[0], mismatches[1]
		return second == first+1 && left[first] == right[second] && left[second] == right[first]
	}

	if len(left) > len(right) {
		left, right = right, left
	}

	i, j, edits := 0, 0, 0
	for i < len(left) && j < len(right) {
		if left[i] == right[j] {
			i++
			j++
			continue
		}
		edits++
		if edits > 1 {
			return false
		}
		j++
	}
	return true
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

// foldMixedCyrillicLatin добавляет альтернативную форму только для токена,
// в котором одновременно обнаружены кириллица и латиница. Это помогает
// исправить визуальную подмену символов, не транслитерируя обычные имена.
func foldMixedCyrillicLatin(value string) (string, bool) {
	hasCyrillic, hasLatin := false, false
	for _, r := range value {
		switch {
		case unicode.In(r, unicode.Cyrillic):
			hasCyrillic = true
		case unicode.In(r, unicode.Latin):
			hasLatin = true
		}
	}
	if !hasCyrillic || !hasLatin {
		return "", false
	}

	var folded strings.Builder
	folded.Grow(len(value))
	for _, r := range value {
		switch r {
		case 'a':
			r = 'а'
		case 'c':
			r = 'с'
		case 'e':
			r = 'е'
		case 'k':
			r = 'к'
		case 'm':
			r = 'м'
		case 'o':
			r = 'о'
		case 'p':
			r = 'р'
		case 't':
			r = 'т'
		case 'x':
			r = 'х'
		case 'y':
			r = 'у'
		}
		folded.WriteRune(r)
	}
	return folded.String(), true
}
