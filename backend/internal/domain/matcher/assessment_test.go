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

func TestAssessAuthoritativeTitle(t *testing.T) {
	date, err := client.ParseDate("2006-04-16")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		title, last, first, patronymic, siteLast, siteFirst string
		lastCode, firstCode                                 Code
		status                                              Status
		mismatch                                            bool
	}{
		{"Иванов Иван", "", "Пётр", "", "Иванов", "Иван", CodeExact, CodeExact, StatusReady, false},
		{"Петров Иван", "Иванов", "", "", "Иванов", "Иван", CodeMissing, CodeExact, StatusRequiresConfirmation, false},
		{"Иванов Иван Петрович", "", "", "", "Иванов", "Иван", CodeExact, CodeExact, StatusReady, false},
		{"Иванов Иван Петрович", "", "", "Петрович", "Иванов", "Иван", CodeExact, CodeExact, StatusReady, false},
		{"Иванов Иван Петрович", "", "", "Иванович", "Иванов", "Иван", CodeExact, CodeExact, StatusReady, false},
		{"Иванов Иван Петров", "", "", "Иванович", "Иванов", "Иван", CodeExact, CodeExact, StatusRequiresConfirmation, true},
		{"Иванов Иван", "", "", "Петрович", "Иванов", "Иван", CodeExact, CodeExact, StatusReady, false},
		{"Иванов Иван", "Петров", "Пётр", "Иванович", "Иванов", "Иван", CodeExact, CodeExact, StatusReady, false},
		{"  ", "Иванов", "", "", "Иванов", "Иван", CodeExact, CodeMissing, StatusRequiresConfirmation, false},
		{"Иванов Александр Петрович", "", "", "", "Иванов", "Саша", CodeExact, CodeAliasMatch, StatusRequiresConfirmation, false},
		{"Иванов Александр Петров", "", "Александр", "", "Иванов", "Саша", CodeExact, CodeAliasMatch, StatusRequiresConfirmation, true},
		{"Сидоров Иван Петрович", "Сидоров", "", "Петрович", "Иванов", "Иван", CodeMismatch, CodeExact, StatusRequiresConfirmation, false},
		{"Иванов Иван Петрович Лишнее", "", "", "", "Иванов", "Иван", CodeExact, CodeExact, StatusRequiresConfirmation, true},
		{"Иванов Иван Ильич", "", "", "", "Иванов", "Иван", CodeExact, CodeExact, StatusRequiresConfirmation, true},
		{"Иванов Иван Ильич", "", "", "ильич", "Иванов", "Иван", CodeExact, CodeExact, StatusReady, false},
		{"Иванов Иван-Петрович", "", "", "", "Иванов", "Иван", CodeExact, CodeExact, StatusRequiresConfirmation, true},
		{"Иванов Иван Иван", "", "", "", "Иванов", "Иван", CodeExact, CodeMissing, StatusRequiresConfirmation, false},
		{"Иванов", "", "Иванов", "", "Иванов", "Иван", CodeExact, CodeMissing, StatusRequiresConfirmation, false},
		{"Иванов", "", "", "", "Иванов", "Иванов", CodeMissing, CodeMissing, StatusRequiresConfirmation, false},
		{"Иванов Иван", "", "", "", "", "", CodeMissing, CodeMissing, StatusRequiresConfirmation, false},
		{"Петрович Иван Иванов-Сидоров", "", "", "", "Иванов Сидоров", "Иван", CodeNormalizedMatch, CodeExact, StatusReady, false},
		{"Иванов Ивани", "", "Ивани", "", "Иванов", "Иваны", CodeExact, CodeProbableMatch, StatusRequiresConfirmation, false},
	} {
		t.Run(tc.title+"/"+tc.last+"/"+tc.first+"/"+tc.patronymic+"/"+tc.siteFirst, func(t *testing.T) {
			crm := client.CRMClient{SourceFullName: tc.title, LastName: tc.last, FirstName: tc.first, Patronymic: tc.patronymic, Birthday: &date}
			site := client.SiteClient{LastName: tc.siteLast, ShortName: tc.siteFirst, Birthday: &date}
			if tc.siteFirst == "" {
				site.UnstructuredName = "Иванов Иван"
			}
			got := Assess(crm, site)
			if got.Status != tc.status || got.Checks[0].Code != tc.lastCode || got.Checks[1].Code != tc.firstCode || got.ConfirmationRequired != (tc.status == StatusRequiresConfirmation) || (got.FullNameMismatch != nil) != tc.mismatch {
				t.Fatalf("assessment = %+v; want %s, %s, %s, full mismatch %v", got, tc.status, tc.lastCode, tc.firstCode, tc.mismatch)
			}
		})
	}
}
