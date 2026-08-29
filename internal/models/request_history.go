package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// RequestHistory records a single request served by a mock endpoint.
type RequestHistory struct {
	bun.BaseModel `bun:"table:request_history,alias:rh"`

	ID         uuid.UUID `bun:",pk,type:uuid" json:"id"`
	EndpointID uuid.UUID `bun:"endpoint_id,type:uuid,notnull" json:"endpoint_id"`
	AccountID  uuid.UUID `bun:"account_id,type:uuid" json:"account_id"`
	Request    string    `bun:"request,type:text" json:"request"`
	Response   string    `bun:"response,type:text" json:"response"`
	Latency    int64     `bun:"latency,type:bigint,notnull,default:0" json:"latency"`
	CreatedAt  time.Time `bun:"created_at,type:timestamptz,notnull,default:now()" json:"created_at"`

	Endpoint *Endpoint `bun:"rel:belongs-to,join:endpoint_id=id" json:"endpoint,omitempty"`
}
