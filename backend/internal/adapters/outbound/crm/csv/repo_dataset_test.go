package csv

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"slices"
	"testing"

	sitecsv "github.com/437i/client-matcher/backend/internal/adapters/outbound/site/csv"
)

func TestStage0FullDataset(t *testing.T) {
	repo, err := NewCSVCRMRepo("testdata/stage0/crm.csv")
	if err != nil {
		t.Fatal(err)
	}
	if len(repo.byID) != 200 {
		t.Fatalf("loaded %d CRM records, want 200", len(repo.byID))
	}
	data, err := os.ReadFile("testdata/stage0/crm.csv")
	if err != nil {
		t.Fatal(err)
	}
	crmRows, err := csv.NewReader(bytes.NewReader(data)).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	slices.Reverse(crmRows[1:])
	var reversedData bytes.Buffer
	w := csv.NewWriter(&reversedData)
	if err := w.WriteAll(crmRows); err != nil {
		t.Fatal(err)
	}
	reversed := newRepo(t, reversedData.String())
	ordered := map[string][]string{}
	for id, source := range repo.byID {
		got, err := repo.Search(context.Background(), source.FullName(), 200)
		if err != nil {
			t.Fatal(err)
		}
		ordered["crm:"+id] = ids(got)
		seen := map[string]bool{}
		for _, c := range got {
			if seen[c.ID] {
				t.Fatalf("duplicate result ID %s", c.ID)
			}
			seen[c.ID] = true
		}
		found := false
		for _, c := range got {
			if c.ID == id {
				found = true
				if !reflect.DeepEqual(c, source) {
					t.Fatalf("record %s changed", id)
				}
			}
		}
		if !found {
			t.Errorf("exact full-name search lost record %s", id)
		}
	}
	site, err := sitecsv.NewCSVSiteRepo("testdata/stage0/site.csv")
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open("testdata/stage0/site.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows)-1 != 55 {
		t.Fatalf("site rows: %d", len(rows)-1)
	}
	known := map[string][]string{
		"1323": {"1223"}, "1138": {"7831"}, "1317": {"6878", "7229"}, "1535": {"8457"},
		"699": {"6878", "7229"}, "1066": {"8680"}, "825": {"1870", "7866"}, "806": {"7173"},
		"408": {"1866", "2614"}, "1150": {"6916"}, "1242": {"3800", "5380"}, "858": {"3800", "5380"},
		"1369": {"5636", "6145"}, "969": {"5751", "8963"},
	}
	for _, row := range rows[1:] {
		c, err := site.GetByID(context.Background(), row[0])
		if err != nil {
			t.Fatal(err)
		}
		if c.ShortName+" "+c.LastName != row[1] {
			t.Errorf("site record %s changed", row[0])
		}
		first, err := repo.Search(context.Background(), row[1], 50)
		if err != nil {
			t.Fatal(err)
		}
		second, err := reversed.Search(context.Background(), row[1], 50)
		if err != nil {
			t.Fatal(err)
		}
		if want, ok := known[row[0]]; ok && !slices.Equal(ids(first), want) {
			t.Errorf("site %s: got %v, want %v", row[0], ids(first), want)
		}
		ordered["site:"+row[0]] = ids(first)
		if len(first) > 50 || !reflect.DeepEqual(first, second) {
			t.Errorf("unstable site query %s", row[0])
		}
	}
	for _, tc := range []struct {
		query string
		want  []string
	}{
		{"Артём Семёнов", []string{"7831"}},
		{"Ваня Петров", []string{"6878", "7229"}},
		{"Даша Захарова", []string{"8457"}},
		{"Саша Волков", []string{"8680"}},
		{"Магомед-Амин Джабраилов", []string{"1223"}},
		{"Даня Соколов", []string{"6916"}},
		{"Кирилл Орлов", []string{"3800", "5380"}},
		{"Полина Белова", []string{"5751", "8963"}},
		{"Никита Сидоров", []string{"1866", "2614"}},
		{"Максим Григорьев", []string{"5636", "6145"}},
		{"Лиза Романова", []string{"1870", "7866"}},
		{"Азамат Далгатов", []string{}},
		{"Ильич Артем Семенов", []string{"7831"}},
		{"Семеноф Артем", []string{"7831"}},
		{"я", []string{}},
	} {
		t.Run(tc.query, func(t *testing.T) {
			got, err := repo.Search(context.Background(), tc.query, 50)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(ids(got), tc.want) {
				t.Errorf("got %v, want %v", ids(got), tc.want)
			}
			limited, err := repo.Search(context.Background(), tc.query, 1)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(ids(limited), tc.want[:min(1, len(tc.want))]) {
				t.Errorf("limit: %v", ids(limited))
			}
		})
	}
	encoded, err := json.Marshal(ordered)
	if err != nil {
		t.Fatal(err)
	}
	if digest := fmt.Sprintf("%x", sha256.Sum256(encoded)); digest != "ab255e5cbdf56331f7ea20fa98f76fbed1e74a9342265dc710391bc00ac4c6df" {
		t.Fatalf("ordered search snapshot: %s", digest)
	}
	// The known date discrepancy must not remove the selected search candidate.
	s, err := site.GetByID(context.Background(), "1066")
	if err != nil {
		t.Fatal(err)
	}
	c := repo.byID["8680"]
	if s.Birthday == nil || c.Birthday == nil || *s.Birthday == *c.Birthday {
		t.Fatal("fixture must retain differing birthdays")
	}
	if c.FullName() != "Волков Александр Тимурович" {
		t.Fatal("source name replaced with alias")
	}
}
