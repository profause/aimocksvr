package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// EndpointVersion is a full snapshot of an endpoint at a point in time.
// Every create and every modification of an endpoint produces a new version,
// enabling history, diff and rollback.
type EndpointVersion struct {
	bun.BaseModel `bun:"table:endpoint_versions,alias:ev"`

	ID            uuid.UUID `bun:",pk,type:uuid" json:"id"`
	EndpointID    uuid.UUID `bun:"endpoint_id,type:uuid,notnull" json:"endpoint_id"`
	AccountID     uuid.UUID `bun:"account_id,type:uuid" json:"account_id"`
	Method        string    `bun:"method,type:varchar(10),notnull,default:''" json:"method"`
	Path          string    `bun:"path,type:varchar(255),notnull,default:''" json:"path"`
	Description   string    `bun:"description,type:text,notnull,default:''" json:"description"`
	Prompt        string    `bun:"prompt,type:text,notnull" json:"prompt"`
	ResponseType  string    `bun:"response_type,type:varchar(50),notnull,default:'json'" json:"response_type"`
	Stateful      bool      `bun:"stateful,notnull,default:false" json:"stateful"`
	Status        string    `bun:"status,type:varchar(20),notnull,default:'active'" json:"status"`
	RequestSchema string    `bun:"request_schema,type:text,notnull,default:''" json:"request_schema"`
	ErrorSim      string    `bun:"error_sim,type:text,notnull,default:''" json:"error_sim"`
	Public        bool      `bun:"public,notnull,default:false" json:"public"`
	Schema        string    `bun:"schema,type:text" json:"schema"`
	Version       int       `bun:"version,type:int,notnull" json:"version"`
	CreatedAt     time.Time `bun:"created_at,type:timestamptz,notnull,default:now()" json:"created_at"`

	Endpoint *Endpoint `bun:"rel:belongs-to,join:endpoint_id=id" json:"endpoint,omitempty"`
}
