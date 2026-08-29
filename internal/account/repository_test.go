package account

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/uptrace/bun/driver/pgdriver"
)

func TestNormalizeEmail(t *testing.T) {
	cases := []struct{ in, want string }{
		{"user@example.com", "user@example.com"},
		{"  User@Example.COM ", "user@example.com"},
		{"USER@EXAMPLE.COM", "user@example.com"},
		{"\tMe@Foo.Bar\n", "me@foo.bar"},
		{"", ""},
	}
	for _, c := range cases {
		if got := normalizeEmail(c.in); got != c.want {
			t.Errorf("normalizeEmail(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestMapConflictPassthrough(t *testing.T) {
	for _, err := range []error{
		errors.New("boom"),
		sql.ErrNoRows,
		context.DeadlineExceeded,
	} {
		if got := mapConflict(err); got != err {
			t.Errorf("mapConflict(%v) = %v, want unchanged passthrough", err, got)
		}
	}

	// A driver error outside the integrity-violation class must keep its type
	// and pass through unmapped.
	pgErr := pgdriver.Error{}
	got := mapConflict(pgErr)
	if errors.Is(got, ErrConflict) {
		t.Error("mapConflict mapped a non-integrity violation to ErrConflict")
	}
	var asErr pgdriver.Error
	if !errors.As(got, &asErr) {
		t.Errorf("mapConflict altered the driver error type: %T", got)
	}
}
