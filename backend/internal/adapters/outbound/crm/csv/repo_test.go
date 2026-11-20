package csv

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/437i/client-matcher/backend/internal/domain/client"
)

const crmData = `id,title,birthday,clinic
1352,Иванов Иван Бек кызы,1982-04-15,Наименование Организации
1353,Иванова Мария,1990-01-02,"ООО ""Рога, и копыта"""
1354,Королёв Пётр Сергеевич,2001-09-09,НИИ им. Денисова
`

func writeCSV(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "crm.csv")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	return path
}

func newRepo(t *testing.T, content string) *CSVCRMRepo {
	t.Helper()
	repo, err := NewCSVCRMRepo(writeCSV(t, content))
	if err != nil {
		t.Fatalf("NewCSVCRMRepo: %v", err)
	}
	return repo
}

func ids(list []client.CRMClient) []string {
	out := make([]string, 0, len(list))
	for _, c := range list {
		out = append(out, c.ID)
	}
	return out
}

func TestGetByID(t *testing.T) {
	repo := newRepo(t, crmData)
	ctx := context.Background()

	cl, err := repo.GetByID(ctx, "1352")
	if err != nil {
		t.Fatal(err)
	}
	if cl.LastName != "Иванов" || cl.FirstName != "Иван" || cl.Patronymic != "Бек кызы" {
		t.Errorf("unexpected name: %+v", cl)
	}
	if cl.Birthday != (client.Date{Day: 15, Month: 4, Year: 1982}) {
		t.Errorf("unexpected birthday: %+v", cl.Birthday)
	}

	quoted, _ := repo.GetByID(ctx, "1353")
	if quoted.Clinic != `ООО "Рога, и копыта"` || quoted.Patronymic != "" {
		t.Errorf("unexpected record: %+v", quoted)
	}

	if _, err := repo.GetByID(ctx, "nope"); !errors.Is(err, client.ErrCRMNotFound) {
		t.Errorf("got %v, want ErrCRMNotFound", err)
	}

	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := repo.GetByID(canceled, "1352"); !errors.Is(err, context.Canceled) {
		t.Errorf("got %v, want context.Canceled", err)
	}
}

func TestSearch(t *testing.T) {
	repo := newRepo(t, crmData)

	tests := []struct {
		name  string
		query string
		limit int
		want  []string
	}{
		{"last name", "Иванов", 10, []string{"1352", "1353"}},
		{"case insensitive", "иванов", 10, []string{"1352", "1353"}},
		{"prefix", "кор", 10, []string{"1354"}},
		{"ye matches yo", "королев", 10, []string{"1354"}},
		{"first name first", "Иван Иванов", 10, []string{"1352"}},
		{"last and first name", "Иванова Мария", 10, []string{"1353"}},
		{"patronymic word", "бек", 10, []string{"1352"}},
		{"not found", "Сидоров", 10, []string{}},
		{"blank", "   ", 10, []string{}},
		{"limit", "Иванов", 1, []string{"1352"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := repo.Search(context.Background(), tc.query, tc.limit)
			if err != nil {
				t.Fatal(err)
			}
			if got == nil {
				t.Fatal("Search must return a non-nil slice")
			}
			if !slices.Equal(ids(got), tc.want) {
				t.Errorf("Search(%q) = %v, want %v", tc.query, ids(got), tc.want)
			}
		})
	}
}

func TestSearchWithRealisticNameAnomalies(t *testing.T) {
	content := `id,title,birthday,clinic
1,Семенов Артем Ильич,2015-01-23,A
2,Захарова Дарья Николаевна,2021-05-16,A
3,Джабраилов Магомед-Амин Юсупович,2017-03-05,A
4,Coколов Даниил Игоревич,2016-06-30,A
8,Волков Александр Тимурович,2020-12-15,A
5,Белова Полина Сергеевна,2019-07-11,A
6,Белова Полина Сергеевна,2019-07-11,B
7,Иванов Иван,2015-01-01,A
`
	repo := newRepo(t, content)

	tests := []struct {
		name  string
		query string
		want  []string
	}{
		{"first name", "артем", []string{"1"}},
		{"patronymic", "ильич", []string{"1"}},
		{"words in any order", "ильич семен", []string{"1"}},
		{"diminutive", "даша", []string{"2"}},
		{"given name alias from site form", "саша волков", []string{"8"}},
		{"hyphenated first name", "амин магомед", []string{"3"}},
		{"mixed Latin and Cyrillic letters", "соколов", []string{"4"}},
		{"single typo", "соколоФ", []string{"4"}},
		{"substring inside word", "ванов", []string{"7"}},
		{"duplicate source records are both returned", "полина белова", []string{"5", "6"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := repo.Search(context.Background(), tc.query, 50)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(ids(got), tc.want) {
				t.Errorf("Search(%q) = %v, want %v", tc.query, ids(got), tc.want)
			}
		})
	}
}

func TestSearchRanksExactMatchesBeforeBroaderMatches(t *testing.T) {
	repo := newRepo(t, `id,title,birthday,clinic
1,Иванова Мария,1990-01-02,A
2,Иванов Иван,1982-04-15,A
`)

	got, err := repo.Search(context.Background(), "иванов", 10)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"2", "1"}
	if !slices.Equal(ids(got), want) {
		t.Errorf("ranked IDs = %v, want %v", ids(got), want)
	}
}

func TestSearchResultIsIndependentCopy(t *testing.T) {
	repo := newRepo(t, crmData)
	first, _ := repo.Search(context.Background(), "Иванов", 10)
	first[0].Clinic = "changed"
	first[0].Patronymic = "changed"

	again, _ := repo.GetByID(context.Background(), "1352")
	if again.Clinic == "changed" || again.Patronymic == "changed" {
		t.Error("mutating a result must not affect the repository")
	}
}

func TestLoadSkipsBadRowsAndHandlesBOM(t *testing.T) {
	content := "\ufeffid,title,birthday,clinic\n" +
		"1,Иванов Иван,1982-04-15,A\n" +
		"2,Только,1982-04-15,B\n" + // нет имени
		"3,Петров Пётр,not-a-date,C\n" + // битая дата
		"4,Сидоров Сидор,1982-04-15\n" + // не хватает колонки
		",Безымянный Ид,1982-04-15,D\n" + // пустой id
		"5,Орлов Олег,1982-04-15,E\n"
	repo := newRepo(t, content)

	for _, id := range []string{"1", "5"} {
		if _, err := repo.GetByID(context.Background(), id); err != nil {
			t.Errorf("record %s must be loaded: %v", id, err)
		}
	}
	for _, id := range []string{"2", "3", "4"} {
		if _, err := repo.GetByID(context.Background(), id); !errors.Is(err, client.ErrCRMNotFound) {
			t.Errorf("record %s must be skipped, got %v", id, err)
		}
	}
}

func TestLoadFileLevelErrors(t *testing.T) {
	t.Run("duplicate id", func(t *testing.T) {
		_, err := NewCSVCRMRepo(writeCSV(t, "id,title,birthday,clinic\n1,Иванов Иван,1982-04-15,A\n1,Петров Пётр,1982-04-15,B\n"))
		if !errors.Is(err, ErrDuplicateID) {
			t.Errorf("got %v, want ErrDuplicateID", err)
		}
	})
	t.Run("wrong header", func(t *testing.T) {
		if _, err := NewCSVCRMRepo(writeCSV(t, "id,name,birthday,clinic\n")); err == nil {
			t.Error("expected error")
		}
	})
	t.Run("missing file", func(t *testing.T) {
		if _, err := NewCSVCRMRepo(filepath.Join(t.TempDir(), "missing.csv")); err == nil {
			t.Error("expected error")
		}
	})
}
