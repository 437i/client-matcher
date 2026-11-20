package csv

import (
	"context"
	encodingcsv "encoding/csv"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestStage0RankingAndOriginalRecords(t *testing.T) {
	const header = "id,title,birthday,clinic\n"
	rows := []string{
		"f,Сооклов Иван,2000-01-01,F\n",
		"s,Высоколов Иван,2000-01-01,S\n",
		"p,Соколовский Иван,2000-01-01,P\n",
		"h,Coколов Иван,2000-01-01,H\n",
		"2,Соколов Иван,2000-01-01,B\n",
		"10,Соколов Иван,2000-01-01,A\n",
	}
	want := []string{"10", "2", "h", "p", "s", "f"}
	for _, reverse := range []bool{false, true} {
		if reverse {
			slices.Reverse(rows)
		}
		repo := newRepo(t, header+strings.Join(rows, ""))
		for _, limit := range []int{-1, 0, 1, 3, 6, 100} {
			got, err := repo.Search(context.Background(), "соколов", limit)
			if err != nil {
				t.Fatal(err)
			}
			n := min(max(limit, 0), len(want))
			if got == nil || !slices.Equal(ids(got), want[:n]) {
				t.Errorf("reverse=%v limit=%d: %v, want %v", reverse, limit, ids(got), want[:n])
			}
		}
	}
	repo := newRepo(t, header+"x,Ёлкин Пётр-Амин Бек кызы,2000-01-01,Синтетическая организация\n")
	got, err := repo.Search(context.Background(), "амин петя елкин", 10)
	if err != nil || len(got) != 1 {
		t.Fatalf("original record search: %v, %v", got, err)
	}
	if got[0].FullName() != "Ёлкин Пётр-Амин Бек кызы" || got[0].FirstName != "Пётр-Амин" || got[0].Clinic != "Синтетическая организация" {
		t.Fatalf("search changed source fields: %+v", got[0])
	}
}

func TestStage0AliasRanking(t *testing.T) {
	repo := newRepo(t, "id,title,birthday,clinic\n"+
		"p,Тестов Дашалия,2000-01-01,A\n"+
		"a,Тестов Дарья,2000-01-01,A\n"+
		"e,Тестов Даша,2000-01-01,A\n")
	got, err := repo.Search(context.Background(), "даша", 10)
	if err != nil || !slices.Equal(ids(got), []string{"e", "a", "p"}) {
		t.Fatalf("alias ranking: %v, %v", ids(got), err)
	}
}

func TestStage0EmptyAndCanceledSearch(t *testing.T) {
	repo := newRepo(t, crmData)
	for _, q := range []string{"", "  ", "—,;/", "неттакогоимени", strings.Repeat("иван ", 64)} {
		got, err := repo.Search(context.Background(), q, 10)
		if err != nil || got == nil || len(got) != 0 {
			t.Errorf("%q: %v, %v", q, got, err)
		}
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	expired, stop := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer stop()
	for _, tc := range []struct {
		ctx  context.Context
		want error
	}{{canceled, context.Canceled}, {expired, context.DeadlineExceeded}} {
		for _, q := range []string{"иван", ""} {
			if _, err := repo.Search(tc.ctx, q, 10); !errors.Is(err, tc.want) {
				t.Errorf("context search: %v, want %v", err, tc.want)
			}
		}
	}
}

func TestStage0CSVLoading(t *testing.T) {
	for _, tc := range []struct {
		name, content string
		want          []string
		fail          bool
	}{
		{"empty file", "", nil, true},
		{"header only", "id,title,birthday,clinic\n", []string{}, false},
		{"blank lines and quoted newline", "\ufeffID,TITLE,BIRTHDAY,CLINIC\n\n1,Ёлкин Пётр,2000-01-01,\"Синтетическая, \"\"организация\"\"\nстрока\"\n", []string{"1"}, false},
		{"bad quote then valid row", "id,title,birthday,clinic\n1,Плохая\" Строка,2000-01-01,A\n2,Ёлкин Пётр,2000-01-01,B\n", []string{"2"}, false},
		{"unclosed final quote", "id,title,birthday,clinic\n1,Ёлкин Пётр,2000-01-01,A\n2,\"Незакрытая строка,2000-01-01,B\n", nil, true},
		{"unclosed middle quote", "id,title,birthday,clinic\n1,Ёлкин Пётр,2000-01-01,A\n2,\"Незакрытая строка,2000-01-01,B\n3,Сидоров Иван,2000-01-01,C\n", nil, true},
		{"too many columns", "id,title,birthday,clinic\n1,Ёлкин Пётр,2000-01-01,A,extra\n2,Ёлкин Пётр,2000-01-01,B\n", []string{"2"}, false},
		{"trimmed duplicate IDs", "id,title,birthday,clinic\n1,Ёлкин Пётр,2000-01-01,A\n 1 ,Ёлкин Пётр,2000-01-01,B\n", nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, err := NewCSVCRMRepo(writeCSV(t, tc.content))
			if tc.fail {
				if err == nil || repo != nil {
					t.Fatal("expected loading error without partial repository")
				}
				if strings.HasPrefix(tc.name, "unclosed") && !errors.Is(err, encodingcsv.ErrQuote) {
					t.Fatalf("got %v, want csv.ErrQuote", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			got, err := repo.Search(context.Background(), "петр", 100)
			if err != nil || !slices.Equal(ids(got), tc.want) {
				t.Fatalf("records: %v, %v; want %v", ids(got), err, tc.want)
			}
		})
	}
}

func TestStage0ExtendedSyntheticCandidates(t *testing.T) {
	var content strings.Builder
	content.WriteString("id,title,birthday,clinic\n")
	for i := 0; i < 120; i++ {
		fmt.Fprintf(&content, "%03d,Синтетиков Александр Бек кызы,2000-01-01,Fixture\n", i)
	}
	repo := newRepo(t, content.String())
	got, err := repo.Search(context.Background(), "кызы саша синтетиков", 50)
	if err != nil || len(got) != 50 {
		t.Fatalf("extended fixture: %d, %v", len(got), err)
	}
	for i, c := range got {
		if c.ID != fmt.Sprintf("%03d", i) {
			t.Errorf("position %d: %s", i, c.ID)
		}
	}
	all, err := repo.Search(context.Background(), "александр", 200)
	if err != nil || len(all) != 120 {
		t.Fatalf("distinct IDs lost: %d, %v", len(all), err)
	}
}

type stage0CancelDuringScan struct {
	context.Context
	calls int
}

func (c *stage0CancelDuringScan) Err() error {
	c.calls++
	if c.calls >= 3 {
		return context.Canceled
	}
	return nil
}

func TestStage0CancellationDuringScan(t *testing.T) {
	repo := newRepo(t, crmData)
	ctx := &stage0CancelDuringScan{Context: context.Background()}
	got, err := repo.Search(ctx, "иванов", 10)
	if !errors.Is(err, context.Canceled) || got != nil {
		t.Fatalf("partial results after cancellation: %v, %v", got, err)
	}
}
