package matcher

import (
	"slices"

	"github.com/437i/client-matcher/backend/internal/domain/client"
	"testing"
)

func TestAssess(t *testing.T) {
	d := &client.Date{Year: 2000, Month: 1, Day: 1}
	crm := client.CRMClient{LastName: "Семёнов", FirstName: "Александр", Patronymic: "Иванович", Birthday: d}
	for _, tt := range []struct {
		name, last, first, unstructured string
		birthday                        *client.Date
		codes                           [3]Code
		status                          Status
	}{
		{"exact", "Семёнов", "Александр", "", d, [3]Code{CodeExact, CodeExact, CodeExact}, StatusReady},
		{"normalized", " СЕМЕНОВ ", "александр", "", d, [3]Code{CodeNormalizedMatch, CodeNormalizedMatch, CodeExact}, StatusReady},
		{"alias", "Семёнов", "Саша", "", d, [3]Code{CodeExact, CodeAliasMatch, CodeExact}, StatusRequiresConfirmation},
		{"mixed script", "Cемёнов", "Александр", "", d, [3]Code{CodeProbableMatch, CodeExact, CodeExact}, StatusRequiresConfirmation},
		{"transpose", "Смеёнов", "Александр", "", d, [3]Code{CodeProbableMatch, CodeExact, CodeExact}, StatusRequiresConfirmation},
		{"prefix", "Семёнов", "Алексан", "", d, [3]Code{CodeExact, CodeMismatch, CodeExact}, StatusRequiresConfirmation},
		{"substring", "Семёнов", "лександр", "", d, [3]Code{CodeExact, CodeMismatch, CodeExact}, StatusRequiresConfirmation},
		{"unknown roles", "", "", "Саша", nil, [3]Code{CodeMissing, CodeMissing, CodeMissing}, StatusRequiresConfirmation},
		{"different date", "Семёнов", "Александр", "", &client.Date{Year: 2000, Month: 1, Day: 2}, [3]Code{CodeExact, CodeExact, CodeMismatch}, StatusRequiresConfirmation},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := Assess(crm, client.SiteClient{LastName: tt.last, ShortName: tt.first, UnstructuredName: tt.unstructured, Birthday: tt.birthday})
			if got.Status != tt.status || got.ConfirmationRequired != (tt.status == StatusRequiresConfirmation) || len(got.Checks) != 3 {
				t.Fatalf("assessment: %+v", got)
			}
			for i, f := range []Field{FieldLastName, FieldFirstName, FieldBirthday} {
				if got.Checks[i].Field != f || got.Checks[i].Code != tt.codes[i] {
					t.Fatalf("check %d: %+v", i, got.Checks[i])
				}
			}
		})
	}
}

func TestConfirmationCodes(t *testing.T) {
	a := Assess(client.CRMClient{}, client.SiteClient{})
	if got := a.ConfirmationCodes(); !slices.Equal(got, []string{"birthday:missing", "name:missing"}) {
		t.Fatalf("audit codes: %v", got)
	}
}
