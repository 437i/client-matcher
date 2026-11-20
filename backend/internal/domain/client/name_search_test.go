package client

import (
	"strings"
	"testing"
)

func tokenizedCandidate(value string) []SearchToken {
	// В тестах кандидат представлен в формате CRM: фамилия, имя, отчество.
	parts := strings.Fields(value)
	if len(parts) < 2 {
		return TokenizeName(value)
	}
	return TokenizeCRMName(CRMClient{
		LastName:   parts[0],
		FirstName:  parts[1],
		Patronymic: strings.Join(parts[2:], " "),
	})
}

func TestTokenizeNameNormalizesWordsAndHyphens(t *testing.T) {
	tokens := TokenizeName("  Магомед–Амин   ДЖАБРАИЛОВ ")
	if len(tokens) != 3 {
		t.Fatalf("got %d tokens, want 3", len(tokens))
	}
	if got := tokens[0].Forms[0].Value; got != "магомед" {
		t.Errorf("first token = %q, want магомед", got)
	}
	if got := tokens[1].Forms[0].Value; got != "амин" {
		t.Errorf("second token = %q, want амин", got)
	}
	if got := tokens[2].Forms[0].Value; got != "джабраилов" {
		t.Errorf("third token = %q, want джабраилов", got)
	}
}

func TestMatchNameTokens(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		candidate string
		wantMatch bool
	}{
		{"any name part", "артем", "Семенов Артем Ильич", true},
		{"patronymic", "ильич", "Семенов Артем Ильич", true},
		{"order does not matter", "ильич артем", "Семенов Артем Ильич", true},
		{"hyphenated name split", "амин магомед", "Джабраилов Магомед-Амин Юсупович", true},
		{"yo and ye", "артем", "Семенов Артём Ильич", true},
		{"known diminutive", "даша", "Захарова Дарья Николаевна", true},
		{"known diminutive in reverse", "дарья", "Захарова Даша Николаевна", true},
		{"mixed alphabets", "соколов", "Coколов Даниил Игоревич", true},
		{"substring", "ванов", "Иванов Иван", true},
		{"one typo", "соколоФ", "Соколов Даниил Игоревич", true},
		{"one typo next to mixed script", "соколоФ", "Coколов Даниил Игоревич", true},
		{"short typo is not fuzzy", "иван", "иват", false},
		{"one query word cannot reuse candidate token", "иван иванов", "Иванова Мария", false},
		{"all query words required", "артем иванов", "Семенов Артем Ильич", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			score := MatchNameTokens(TokenizeSearchQuery(tc.query), tokenizedCandidate(tc.candidate))
			got := score > 0
			if got != tc.wantMatch {
				t.Fatalf("MatchNameTokens(%q, %q) score=%d, match=%v; want match=%v", tc.query, tc.candidate, score, got, tc.wantMatch)
			}
		})
	}
}

func TestNameSearchFormsDoNotTransliteratePureLatin(t *testing.T) {
	tokens := TokenizeName("sokolov")
	if len(tokens) != 1 {
		t.Fatalf("got %d tokens, want 1", len(tokens))
	}
	if len(tokens[0].Forms) != 1 {
		t.Fatalf("pure Latin token got %d forms, want only primary form", len(tokens[0].Forms))
	}
}

func TestAliasesApplyOnlyToGivenName(t *testing.T) {
	if score := MatchNameTokens(TokenizeSearchQuery("саша"), TokenizeCRMName(CRMClient{LastName: "Иванов", FirstName: "Александр"})); score == 0 {
		t.Fatal("given-name alias should match")
	}
	if score := MatchNameTokens(TokenizeSearchQuery("александр"), TokenizeCRMName(CRMClient{LastName: "Саша", FirstName: "Иван"})); score != 0 {
		t.Fatalf("alias must not be applied to surname, got score %d", score)
	}
	if score := MatchNameTokens(TokenizeSearchQuery("саша"), TokenizeCRMName(CRMClient{LastName: "Саша", FirstName: "Иван"})); score == 0 {
		t.Fatal("an exact surname match must remain searchable")
	}
}

func TestMixedScriptFoldingDoesNotGuessAmbiguousLetters(t *testing.T) {
	if score := MatchNameTokens(TokenizeSearchQuery("соколов"), TokenizeCRMName(CRMClient{LastName: "Coколов", FirstName: "Даниил"})); score == 0 {
		t.Fatal("C/o homoglyphs in mixed-script token should match")
	}
	if score := MatchNameTokens(TokenizeSearchQuery("высо"), TokenizeCRMName(CRMClient{LastName: "bысо", FirstName: "Иван"})); score != 0 {
		t.Fatalf("ambiguous b-to-в folding should not match, got score %d", score)
	}
}

func TestKnownGivenNameAliases(t *testing.T) {
	pairs := [][2]string{
		{"ваня", "иван"},
		{"саша", "александр"},
		{"саша", "александра"},
		{"соня", "софья"},
		{"даша", "дарья"},
		{"настя", "анастасия"},
		{"маша", "мария"},
		{"тима", "тимофей"},
		{"лиза", "елизавета"},
		{"даня", "даниил"},
		{"варя", "варвара"},
		{"лева", "лев"},
		{"петя", "петр"},
	}

	for _, pair := range pairs {
		t.Run(pair[0]+"_"+pair[1], func(t *testing.T) {
			query := TokenizeSearchQuery(pair[0])
			candidate := TokenizeCRMName(CRMClient{LastName: "Иванов", FirstName: pair[1]})
			if score := MatchNameTokens(query, candidate); score == 0 {
				t.Fatalf("query %q should match given name %q", pair[0], pair[1])
			}
		})
	}
}
