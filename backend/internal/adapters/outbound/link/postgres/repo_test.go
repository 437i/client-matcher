package postgres

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/437i/client-matcher/backend/internal/domain/link"
)

func TestMapCreateError(t *testing.T) {
	unique := func(constraint string) error {
		return fmt.Errorf("wrapped: %w", &pgconn.PgError{Code: pgUniqueViolation, ConstraintName: constraint})
	}
	other := errors.New("connection reset")

	tests := []struct {
		name string
		in   error
		want error
	}{
		{"crm id taken", unique(uqCRMIDConstraint), link.ErrCRMAlreadyLinked},
		{"site id taken", unique(uqSiteIDConstraint), link.ErrSiteAlreadyLinked},
		{"other pg error", &pgconn.PgError{Code: "23514", ConstraintName: "chk"}, nil},
		{"unknown unique constraint", unique("some_other"), nil},
		{"not a pg error", other, other},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := mapCreateError(tc.in)
			if tc.want == nil {
				if errors.Is(got, link.ErrCRMAlreadyLinked) || errors.Is(got, link.ErrSiteAlreadyLinked) {
					t.Errorf("must not be mapped to a domain error: %v", got)
				}
				return
			}
			if !errors.Is(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}
