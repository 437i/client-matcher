package csv

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/437i/client-matcher/backend/internal/domain/client"
)

func writeCSV(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "site.csv")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	return path
}

func TestSiteRepo(t *testing.T) {
	repo, err := NewCSVSiteRepo(writeCSV(t, "id,title,birthday\n"+
		"3586,Ваня Иванов,1982-04-15\n"+
		"3587,Алим Бек кызы,1972-11-04\n"+ // фамилия из двух слов
		"3588,Одиночка,\n"+
		"empty,,\n"+
		"bad,Имя Фамилия,2020-02-30\n"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	cl, err := repo.GetByID(ctx, "3586")
	if err != nil {
		t.Fatal(err)
	}
	if cl.ShortName != "Ваня" || cl.LastName != "Иванов" || (cl.Birthday == nil || *cl.Birthday != (client.Date{Day: 15, Month: 4, Year: 1982})) {
		t.Errorf("unexpected record: %+v", cl)
	}

	multi, err := repo.GetByID(ctx, "3587")
	if err != nil || multi.LastName != "Бек кызы" {
		t.Errorf("unexpected record: %+v, err %v", multi, err)
	}

	cl.Birthday.Year = 1
	again, err := repo.GetByID(ctx, "3586")
	if err != nil || again.Birthday.Year != 1982 {
		t.Fatalf("date copy: %+v, %v", again, err)
	}
	for id, name := range map[string]string{"3588": "Одиночка", "empty": ""} {
		record, err := repo.GetByID(ctx, id)
		if err != nil || record.UnstructuredName != name || record.LastName != "" || record.ShortName != "" || record.Birthday != nil {
			t.Fatalf("unknown roles: %+v, %v", record, err)
		}
	}
	for _, id := range []string{"bad", "nope"} {
		if _, err := repo.GetByID(ctx, id); !errors.Is(err, client.ErrSiteNotFound) {
			t.Errorf("GetByID(%q): got %v, want ErrSiteNotFound", id, err)
		}
	}
}

func TestSiteRepoDuplicateID(t *testing.T) {
	_, err := NewCSVSiteRepo(writeCSV(t, "id,title,birthday\n1,Ваня Иванов,1982-04-15\n1,Петя Петров,1982-04-15\n"))
	if !errors.Is(err, ErrDuplicateID) {
		t.Errorf("got %v, want ErrDuplicateID", err)
	}
}
