package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// EndpointVersion captures the prompt and schema of an endpoint at a point in time.
// Every create/update of an endpoint produces a new version.
type EndpointVersion struct {
	bun.BaseModel `bun:"table:endpoint_versions,alias:ev"`

	ID         uuid.UUID `bun:",pk,type:uuid" json:"id"`
	EndpointID uuid.UUID `bun:"endpoint_id,type:uuid,notnull" json:"endpoint_id"`
	Prompt     string    `bun:"prompt,type:text,notnull" json:"prompt"`
	Schema     string    `bun:"schema,type:text" json:"schema"`
	Version    int       `bun:"version,type:int,notnull" json:"version"`
	CreatedAt  time.Time `bun:"created_at,type:timestamptz,notnull,default:now()" json:"created_at"`

	Endpoint *Endpoint `bun:"rel:belongs-to,join:endpoint_id=id" json:"endpoint,omitempty"`
}
