package client

import "testing"

func TestNormalizeName(t *testing.T) {
	tests := []struct{ name, in, want string }{
		{"lowercase", "Иванов", "иванов"},
		{"spaces", "  Бек   кызы ", "бек кызы"},
		{"yo", "Королёв", "королев"},
		{"capital yo", "ЁЛКИН", "елкин"},
		{"empty", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeName(tc.in); got != tc.want {
				t.Errorf("NormalizeName(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestParseDate(t *testing.T) {
	d, err := ParseDate(" 1982-04-15 ")
	if err != nil {
		t.Fatal(err)
	}
	if d != (Date{Day: 15, Month: 4, Year: 1982}) {
		t.Errorf("unexpected date %+v", d)
	}
	if d.String() != "1982-04-15" {
		t.Errorf("String() = %q", d.String())
	}

	for _, bad := range []string{"", "15.04.1982", "1982-13-01", "1982-02-30"} {
		if _, err := ParseDate(bad); err == nil {
			t.Errorf("ParseDate(%q): expected error", bad)
		}
	}
}

func TestCRMClientFullName(t *testing.T) {
	with := CRMClient{LastName: "Иванов", FirstName: "Иван", Patronymic: "Бек кызы"}
	if got := with.FullName(); got != "Иванов Иван Бек кызы" {
		t.Errorf("got %q", got)
	}
	without := CRMClient{LastName: "Иванова", FirstName: "Мария"}
	if got := without.FullName(); got != "Иванова Мария" {
		t.Errorf("got %q", got)
	}
}
