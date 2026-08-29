package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// Account represents an authenticated user of the mock server.
type Account struct {
	bun.BaseModel `bun:"table:accounts,alias:a"`

	ID           uuid.UUID `bun:",pk,type:uuid" json:"id"`
	Email        string    `bun:"email,type:varchar(320),notnull,unique" json:"email"`
	PasswordHash string    `bun:"password_hash,type:varchar(255),notnull" json:"-"`
	CreatedAt    time.Time `bun:"created_at,type:timestamptz,notnull,default:now()" json:"created_at"`
	UpdatedAt    time.Time `bun:"updated_at,type:timestamptz,notnull,default:now()" json:"updated_at"`
}

// LegacyAccountID owns every resource that predates account-based auth (and,
// when auth is disabled, every new resource). It corresponds to the synthetic
// legacy account seeded by migration 000011 with the reserved email
// legacy@local; it is never used to log in.
var LegacyAccountID = uuid.MustParse("00000000-0000-0000-0000-000000000001")
