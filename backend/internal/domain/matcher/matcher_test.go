package matcher

import (
	"errors"
	"testing"

	"github.com/437i/client-matcher/backend/internal/domain/client"
)

func TestMatchClients(t *testing.T) {
	birthday := client.Date{
		Day:   15,
		Month: 4,
		Year:  1982,
	}

	crm := client.CRMClient{
		LastName: "Королёв",
		Birthday: &birthday,
	}

	tests := []struct {
		name string
		site client.SiteClient
		crm  client.CRMClient
		want error
	}{
		{
			name: "match",
			crm:  crm,
			site: client.SiteClient{
				LastName: "Королёв",
				Birthday: &birthday,
			},
			want: nil,
		},
		{
			name: "case and spaces",
			crm:  crm,
			site: client.SiteClient{
				LastName: "  КОРОЛЁВ ",
				Birthday: &birthday,
			},
			want: nil,
		},
		{
			name: "yo vs ye",
			crm:  crm,
			site: client.SiteClient{
				LastName: "Королев",
				Birthday: &birthday,
			},
			want: nil,
		},
		{
			name: "last name mismatch",
			crm:  crm,
			site: client.SiteClient{
				LastName: "Петров",
				Birthday: &birthday,
			},
			want: ErrLastNameMismatch,
		},
		{
			name: "birthday mismatch",
			crm:  crm,
			site: client.SiteClient{
				LastName: "Королёв",
				Birthday: &client.Date{Day: 16, Month: 4, Year: 1982},
			},
			want: ErrBirthdayMismatch,
		},
		{
			name: "empty crm last name",
			crm: client.CRMClient{
				LastName: "   ",
				Birthday: &birthday,
			},
			site: client.SiteClient{
				LastName: "Королёв",
				Birthday: &birthday,
			},
			want: ErrEmptyLastName,
		},
		{
			name: "empty site last name",
			crm:  crm,
			site: client.SiteClient{
				LastName: " ",
				Birthday: &birthday,
			},
			want: ErrEmptyLastName,
		},
		{
			name: "invalid crm birthday",
			crm: client.CRMClient{
				LastName: "Королёв",
				Birthday: &client.Date{Day: 31, Month: 4, Year: 1982},
			},
			site: client.SiteClient{
				LastName: "Королёв",
				Birthday: &birthday,
			},
			want: ErrInvalidBirthday,
		},
		{
			name: "invalid site birthday",
			crm:  crm,
			site: client.SiteClient{
				LastName: "Королёв",
				Birthday: &client.Date{},
			},
			want: ErrInvalidBirthday,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := MatchClients(tc.crm, tc.site)

			if tc.want == nil {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}

			if !errors.Is(err, tc.want) {
				t.Errorf("got %v, want %v", err, tc.want)
			}
		})
	}
}
