package client

import "testing"

func TestDateIsValid(t *testing.T) {
	tests := []struct {
		name string
		date Date
		want bool
	}{
		{
			name: "valid date",
			date: Date{Day: 15, Month: 4, Year: 1982},
			want: true,
		},
		{
			name: "valid leap year",
			date: Date{Day: 29, Month: 2, Year: 2000},
			want: true,
		},
		{
			name: "invalid leap year",
			date: Date{Day: 29, Month: 2, Year: 1900},
			want: false,
		},
		{
			name: "day exceeds month length",
			date: Date{Day: 31, Month: 4, Year: 1982},
			want: false,
		},
		{
			name: "zero date",
			date: Date{},
			want: false,
		},
		{
			name: "zero day",
			date: Date{Day: 0, Month: 4, Year: 1982},
			want: false,
		},
		{
			name: "invalid month",
			date: Date{Day: 15, Month: 13, Year: 1982},
			want: false,
		},
		{
			name: "zero year",
			date: Date{Day: 15, Month: 4, Year: 0},
			want: false,
		},
		{
			name: "year exceeds supported range",
			date: Date{Day: 15, Month: 4, Year: 10000},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.date.IsValid(); got != tc.want {
				t.Errorf("IsValid() = %v, want %v", got, tc.want)
			}
		})
	}
}
