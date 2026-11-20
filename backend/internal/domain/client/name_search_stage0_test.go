package client

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestStage0Aliases(t *testing.T) {
	for _, g := range nameAliasGroups {
		for _, q := range g {
			for _, name := range g {
				t.Run(q+"/"+name, func(t *testing.T) {
					if got := MatchNameTokens(TokenizeSearchQuery(q), TokenizeCRMName(CRMClient{FirstName: name})); got < 820 {
						t.Errorf("alias score %d", got)
					}
					for _, c := range []CRMClient{{LastName: name}, {Patronymic: name}} {
						got := MatchNameTokens(TokenizeSearchQuery(q), TokenizeCRMName(c))
						literal := MatchNameTokens(TokenizeName(q), TokenizeCRMName(c))
						if got != literal {
							t.Errorf("alias outside given name: %d, literal %d", got, literal)
						}
					}
				})
			}
		}
	}
}

func TestStage0SearchScores(t *testing.T) {
	for _, tc := range []struct {
		q, c string
		want int
	}{
		{"соколов", "соколов", 1000}, {"со", "соколов", 700}, {"кол", "соколов", 500},
		{"соколоФ", "соколов", 300}, {"соХколов", "соколов", 300}, {"соолов", "соколов", 300}, {"сооклов", "соколов", 300},
		{"сакалоф", "соколов", 0}, {"жбвг", "абвгд", 0}, {"жбвгд", "абвг", 0}, {"жбвгд", "абвгд", 300},
		{"с", "соколов", 0}, {"ко", "соколов", 0},
		{"абвгдежзийклмнопрстуфхцчшщэюяъ", "абвгдежзийклмнопрстуфхцчшщэюя", 300},
		{"иван иван", "иван", 0}, {"иван", "иван иван", 1000}, {"иван иван", "иван иван", 2000},
		{"иван петр", "иван иван", 0}, {"алекс александр", "александр алексей", 1700}, {"александр алекс", "александр алексей", 1700},
		{"", "иван", 0}, {"  —,;/ ", "иван", 0},
	} {
		t.Run(tc.q+"/"+tc.c, func(t *testing.T) {
			if got := MatchNameTokens(TokenizeSearchQuery(tc.q), TokenizeName(tc.c)); got != tc.want {
				t.Errorf("score %d, want %d", got, tc.want)
			}
		})
	}
}

func TestStage0UnicodeAndSeparators(t *testing.T) {
	c := CRMClient{LastName: "Ёлкин", FirstName: "Пётр-Амин", Patronymic: "Бек кызы"}
	for _, q := range []string{"кызы;амин,бек/петр—елкин", "ЁЛКИН ПЁТР", "петр амин", "амин петр", "бек кызы"} {
		if got := MatchNameTokens(TokenizeSearchQuery(q), TokenizeCRMName(c)); got == 0 {
			t.Errorf("no match for %q", q)
		}
	}
	if got := NormalizeName("  ЁЛКИН  ПЁТР  "); got != "елкин петр" {
		t.Errorf("NFC/spaces: %q", got)
	}
	for _, pair := range [][2]string{{"a", "а"}, {"c", "с"}, {"e", "е"}, {"k", "к"}, {"m", "м"}, {"o", "о"}, {"p", "р"}, {"t", "т"}, {"x", "х"}, {"y", "у"}} {
		a, b := "я"+pair[0]+"я", "я"+pair[1]+"я"
		for _, p := range [][2]string{{a, b}, {b, a}} {
			if got := MatchNameTokens(TokenizeSearchQuery(p[0]), TokenizeName(p[1])); got != 950 {
				t.Errorf("%q: score %d", p, got)
			}
		}
	}
	for _, p := range [][2]string{{"bысо", "высо"}, {"hина", "нина"}, {"sаша", "саша"}, {"sokolov", "соколов"}} {
		if got := MatchNameTokens(TokenizeSearchQuery(p[0]), TokenizeName(p[1])); got != 0 {
			t.Errorf("ambiguous/pure Latin %q: %d", p, got)
		}
	}
}

func TestStage0TokenBoundaries(t *testing.T) {
	for _, n := range []int{0, 1, 62, 63, 64} {
		words := make([]string, n)
		for i := range words {
			words[i] = fmt.Sprintf("token%03d", i)
		}
		got := MatchNameTokens(TokenizeName("token000"), TokenizeName(strings.Join(words, " ")))
		if (got > 0) != (n > 0 && n <= 63) {
			t.Errorf("%d tokens: %d", n, got)
		}
	}
	if got := MatchNameTokens(TokenizeName(strings.Repeat("я ", 64)), TokenizeName(strings.Repeat("я ", 63))); got != 0 {
		t.Errorf("too many query tokens: %d", got)
	}
}

func TestStage0ConcurrentNormalization(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				if got := NormalizeName("Ёлкин ПЁТР"); got != "елкин петр" {
					t.Errorf("normalization: %q", got)
				}
				MatchNameTokens(TokenizeSearchQuery("петя"), TokenizeCRMName(CRMClient{FirstName: "Пётр"}))
			}
		}()
	}
	wg.Wait()
}

func TestStage0All63TokensAssigned(t *testing.T) {
	words := make([]string, 63)
	for i := range words {
		words[i] = string(rune(0x4e00 + i))
	}
	candidate := TokenizeName(strings.Join(words, " "))
	for i, j := 0, len(words)-1; i < j; i, j = i+1, j-1 {
		words[i], words[j] = words[j], words[i]
	}
	if got := MatchNameTokens(TokenizeName(strings.Join(words, " ")), candidate); got != 63000 {
		t.Errorf("63-token assignment: %d", got)
	}
}

func TestStage0All63InterchangeableTokensAssigned(t *testing.T) {
	tokens := TokenizeName(strings.Repeat("я ", 63))
	if got := MatchNameTokens(tokens, tokens); got != 63000 {
		t.Fatalf("complete interchangeable assignment: %d, want 63000", got)
	}
}

func TestStage0AssignmentMatchesExhaustiveSearch(t *testing.T) {
	// Малые графы с разными весами и отсутствующими рёбрами проверяются
	// независимым полным перебором; большие графы здесь не перебираются.
	words := []string{"я", "ю", "иван", "иванов", "ван", "соколов", "сооклов", "со"}
	for seed := 0; seed < 300; seed++ {
		n, m := 1+seed%4, 1+(seed/4)%5
		query, candidate := make([]SearchToken, n), make([]SearchToken, m)
		for i := range query {
			query[i] = TokenizeName(words[(seed+i*3)%len(words)])[0]
		}
		for j := range candidate {
			candidate[j] = TokenizeName(words[(seed/7+j*5)%len(words)])[0]
		}
		var enumerate func(int, uint) int
		enumerate = func(i int, used uint) int {
			if i == n {
				return 0
			}
			best := -1
			for j := range candidate {
				score := matchToken(query[i], candidate[j])
				if used&(1<<j) != 0 || score == 0 {
					continue
				}
				if rest := enumerate(i+1, used|(1<<j)); rest >= 0 {
					best = max(best, score+rest)
				}
			}
			return best
		}
		want := max(0, enumerate(0, 0))
		if got := MatchNameTokens(query, candidate); got != want {
			t.Fatalf("seed %d (%dx%d): got %d, want %d", seed, n, m, got, want)
		}
	}
}
