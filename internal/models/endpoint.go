package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

const (
	StatusActive   = "active"
	StatusInactive = "inactive"

	ResponseTypeJSON = "json"
)

// Endpoint represents a mock API endpoint registered in the registry.
type Endpoint struct {
	bun.BaseModel `bun:"table:endpoints,alias:e"`

	ID            uuid.UUID `bun:",pk,type:uuid" json:"id"`
	Method        string    `bun:"method,type:varchar(10),notnull" json:"method"`
	Path          string    `bun:"path,type:varchar(255),notnull" json:"path"`
	Description   string    `bun:"description,type:text,notnull" json:"description"`
	Prompt        string    `bun:"prompt,type:text,notnull" json:"prompt"`
	ResponseType  string    `bun:"response_type,type:varchar(50),notnull,default:'json'" json:"response_type"`
	Stateful      bool      `bun:"stateful,notnull,default:false" json:"stateful"`
	Status        string    `bun:"status,type:varchar(20),notnull,default:'active'" json:"status"`
	RequestSchema string    `bun:"request_schema,type:text,notnull,default:''" json:"request_schema"`
	ErrorSim      string    `bun:"error_sim,type:text,notnull,default:''" json:"error_sim"`
	Public        bool      `bun:"public,notnull,default:true" json:"public"`
	CreatedAt     time.Time `bun:"created_at,type:timestamptz,notnull,default:now()" json:"created_at"`
	UpdatedAt     time.Time `bun:"updated_at,type:timestamptz,notnull,default:now()" json:"updated_at"`
}
