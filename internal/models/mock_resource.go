package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// MockResource is a resource stored by a stateful endpoint. Resources are
// keyed by the collection path (the endpoint path without the trailing
// :param segment) and the resource id taken from the path, so the same
// object is shared across POST /users and GET /users/:id.
type MockResource struct {
	bun.BaseModel `bun:"table:mock_resources,alias:r"`

	ID         uuid.UUID      `bun:",pk,type:uuid" json:"id"`
	AccountID  uuid.UUID      `bun:"account_id,type:uuid" json:"account_id"`
	Collection string         `bun:"collection,type:varchar(255),notnull" json:"collection"`
	ResourceID string         `bun:"resource_id,type:varchar(255),notnull" json:"resource_id"`
	Data       map[string]any `bun:"data,type:jsonb,notnull" json:"data"`
	CreatedAt  time.Time      `bun:"created_at,type:timestamptz,notnull,default:now()" json:"created_at"`
	UpdatedAt  time.Time      `bun:"updated_at,type:timestamptz,notnull,default:now()" json:"updated_at"`
}
