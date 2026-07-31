// Package state persists the resources of stateful mock endpoints.
//
// Resources are identified by a collection path (for example /users) and a
// resource id, and stored as JSON objects. The collection path is derived
// from the endpoint's path pattern by stripping the trailing :param segment,
// so endpoints that operate on the same resource (POST /users, GET /users/:id,
// DELETE /users/:id) share one namespace.
package state

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"

	"github.com/profause/aimocksvr/internal/models"
)

// ErrNotFound is returned when no resource matches the requested collection
// and resource id.
var ErrNotFound = errors.New("resource not found")

// ErrConflict is returned when a resource with the same collection and
// resource id already exists.
var ErrConflict = errors.New("resource already exists")

// Store persists mock resources. Implementations must be safe for concurrent
// use.
type Store interface {
	// Create stores a new resource, returning ErrConflict when the id already
	// exists in the collection.
	Create(ctx context.Context, collection, resourceID string, data map[string]any) error
	// Get returns the stored resource; found is false when it does not exist.
	Get(ctx context.Context, collection, resourceID string) (map[string]any, bool, error)
	// Update replaces the resource, returning ErrNotFound when it does not exist.
	Update(ctx context.Context, collection, resourceID string, data map[string]any) error
	// Delete removes the resource and reports whether it existed.
	Delete(ctx context.Context, collection, resourceID string) (bool, error)
}

type postgresStore struct {
	db *bun.DB
}

// NewStore creates a PostgreSQL-backed Store.
func NewStore(db *bun.DB) Store {
	return &postgresStore{db: db}
}

func (s *postgresStore) Create(ctx context.Context, collection, resourceID string, data map[string]any) error {
	resource := &models.MockResource{
		Collection: collection,
		ResourceID: resourceID,
		Data:       data,
	}
	if _, err := s.db.NewInsert().Model(resource).Exec(ctx); err != nil {
		if isUniqueViolation(err) {
			return ErrConflict
		}
		return fmt.Errorf("insert mock resource: %w", err)
	}
	return nil
}

func (s *postgresStore) Get(ctx context.Context, collection, resourceID string) (map[string]any, bool, error) {
	var resource models.MockResource
	err := s.db.NewSelect().Model(&resource).
		Where("collection = ?", collection).
		Where("resource_id = ?", resourceID).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("find mock resource: %w", err)
	}
	return resource.Data, true, nil
}

func (s *postgresStore) Update(ctx context.Context, collection, resourceID string, data map[string]any) error {
	res, err := s.db.NewUpdate().
		Model((*models.MockResource)(nil)).
		Set("data = ?", data).
		Set("updated_at = now()").
		Where("collection = ?", collection).
		Where("resource_id = ?", resourceID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("update mock resource: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update mock resource: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *postgresStore) Delete(ctx context.Context, collection, resourceID string) (bool, error) {
	res, err := s.db.NewDelete().
		Model((*models.MockResource)(nil)).
		Where("collection = ?", collection).
		Where("resource_id = ?", resourceID).
		Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("delete mock resource: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("delete mock resource: %w", err)
	}
	return affected > 0, nil
}

// isUniqueViolation reports whether err is a PostgreSQL unique-constraint
// violation.
func isUniqueViolation(err error) bool {
	var pgErr pgdriver.Error
	return errors.As(err, &pgErr) && pgErr.IntegrityViolation()
}
