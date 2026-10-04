package aitools

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/warmbly/warmbly/internal/errx"
)

func TestPublicMessageWithholdsServerFailures(t *testing.T) {
	pg := &pgconn.PgError{Code: "42P01", Message: `relation "secret_table" does not exist`}
	cases := []struct {
		err  error
		want string
		ok   bool
	}{
		{errors.New("campaign not found"), "campaign not found", true},
		{errx.New(errx.BadRequest, "name is required"), "name is required", true},
		{errx.New(errx.Internal, "select failed"), "", false},
		{fmt.Errorf("list: %w", pg), "", false},
	}
	for _, tc := range cases {
		msg, ok := PublicMessage(tc.err)
		if msg != tc.want || ok != tc.ok {
			t.Errorf("PublicMessage(%v) = %q, %v; want %q, %v", tc.err, msg, ok, tc.want, tc.ok)
		}
	}
}
