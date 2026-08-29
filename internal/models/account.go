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
