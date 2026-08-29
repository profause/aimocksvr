package endpoint

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"

	"github.com/profause/aimocksvr/internal/models"
)

// ErrNotFound is returned when the requested endpoint does not exist.
var ErrNotFound = errors.New("endpoint not found")

// ErrConflict is returned when an endpoint with the same method and path
// already exists.
var ErrConflict = errors.New("endpoint already exists")

// mapConflict translates PostgreSQL integrity violations into ErrConflict.
func mapConflict(err error) error {
	var pgErr pgdriver.Error
	if errors.As(err, &pgErr) && pgErr.IntegrityViolation() {
		return ErrConflict
	}
	return err
}

// Repository persists endpoints and their versions and request history.
//
// Ownership is enforced on every control-plane read and write: callers pass
// the owning account id and rows outside it are invisible (reads return
// ErrNotFound / empty results). accountID uuid.Nil is treated as matching only
// NULL owners, so the account service always passes a concrete owner (real or
// the legacy account).
type Repository interface {
	// WithTx runs fn inside a database transaction. Implementations must bind
	// the transaction to ctx so nested repository calls participate in it.
	WithTx(ctx context.Context, fn func(ctx context.Context) error) error

	Create(ctx context.Context, e *models.Endpoint) error
	Update(ctx context.Context, accountID uuid.UUID, e *models.Endpoint) error
	Delete(ctx context.Context, accountID, id uuid.UUID) error
	FindByID(ctx context.Context, accountID, id uuid.UUID) (*models.Endpoint, error)
	List(ctx context.Context, accountID uuid.UUID, p ListParams) ([]models.Endpoint, int, error)
	ListActiveByMethod(ctx context.Context, method string) ([]models.Endpoint, error)

	CreateVersion(ctx context.Context, v *models.EndpointVersion) error
	ListVersions(ctx context.Context, endpointID uuid.UUID) ([]models.EndpointVersion, error)
	ListHistory(ctx context.Context, endpointID uuid.UUID) ([]models.RequestHistory, error)
	CreateHistory(ctx context.Context, h *models.RequestHistory) error

	CountEndpoints(ctx context.Context, accountID uuid.UUID) (int, error)
	CountRecentRequests(ctx context.Context, accountID uuid.UUID, since time.Time) (int, error)
	AvgLatency(ctx context.Context, accountID uuid.UUID, since time.Time) (float64, error)
	ErrorRate(ctx context.Context, accountID uuid.UUID, since time.Time) (float64, error)
}

type repository struct {
	db *bun.DB
}

// NewRepository creates a PostgreSQL-backed Repository.
func NewRepository(db *bun.DB) Repository {
	return &repository{db: db}
}

func (r *repository) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return r.db.RunInTx(ctx, nil, func(ctx context.Context, _ bun.Tx) error {
		return fn(ctx)
	})
}

func (r *repository) Create(ctx context.Context, e *models.Endpoint) error {
	// The generated columns (created_at/updated_at) are not scanned back; the
	// caller owns the struct it passed in. No Returning("*") so the model does
	// not need to round-trip every column (account_id included).
	if _, err := r.db.NewInsert().Model(e).Exec(ctx); err != nil {
		return fmt.Errorf("insert endpoint: %w", mapConflict(err))
	}
	return nil
}

func (r *repository) Update(ctx context.Context, accountID uuid.UUID, e *models.Endpoint) error {
	res, err := r.db.NewUpdate().
		Model(e).
		WherePK().
		Where("account_id = ?", accountID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("update endpoint: %w", mapConflict(err))
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update endpoint: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *repository) Delete(ctx context.Context, accountID, id uuid.UUID) error {
	res, err := r.db.NewDelete().
		Model((*models.Endpoint)(nil)).
		Where("id = ?", id).
		Where("account_id = ?", accountID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete endpoint: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete endpoint: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *repository) FindByID(ctx context.Context, accountID, id uuid.UUID) (*models.Endpoint, error) {
	e := new(models.Endpoint)
	if err := r.db.NewSelect().
		Model(e).
		Where("id = ?", id).
		Where("account_id = ?", accountID).
		Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find endpoint: %w", err)
	}
	return e, nil
}

func (r *repository) List(ctx context.Context, accountID uuid.UUID, p ListParams) ([]models.Endpoint, int, error) {
	// Initialize to a non-nil empty slice so the JSON payload serializes as
	// [] rather than null when there are no endpoints.
	endpoints := make([]models.Endpoint, 0)

	count, err := r.db.NewSelect().
		Model((*models.Endpoint)(nil)).
		Where("account_id = ?", accountID).
		Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("count endpoints: %w", err)
	}

	err = r.db.NewSelect().
		Model(&endpoints).
		Where("account_id = ?", accountID).
		OrderExpr("created_at DESC").
		Limit(p.Limit).
		Offset((p.Page - 1) * p.Limit).
		Scan(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("list endpoints: %w", err)
	}

	return endpoints, count, nil
}

func (r *repository) CreateVersion(ctx context.Context, v *models.EndpointVersion) error {
	if _, err := r.db.NewInsert().Model(v).Exec(ctx); err != nil {
		return fmt.Errorf("insert endpoint version: %w", err)
	}
	return nil
}

func (r *repository) ListVersions(ctx context.Context, endpointID uuid.UUID) ([]models.EndpointVersion, error) {
	var versions []models.EndpointVersion
	if err := r.db.NewSelect().
		Model(&versions).
		Where("endpoint_id = ?", endpointID).
		OrderExpr("version DESC").
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("list endpoint versions: %w", err)
	}
	return versions, nil
}

func (r *repository) ListActiveByMethod(ctx context.Context, method string) ([]models.Endpoint, error) {
	var endpoints []models.Endpoint
	if err := r.db.NewSelect().
		Model(&endpoints).
		Where("method = ?", method).
		Where("status = ?", models.StatusActive).
		OrderExpr("created_at ASC").
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("list active endpoints: %w", err)
	}
	return endpoints, nil
}

func (r *repository) ListHistory(ctx context.Context, endpointID uuid.UUID) ([]models.RequestHistory, error) {
	var history []models.RequestHistory
	if err := r.db.NewSelect().
		Model(&history).
		Where("endpoint_id = ?", endpointID).
		OrderExpr("created_at DESC").
		Limit(100).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("list request history: %w", err)
	}
	return history, nil
}

func (r *repository) CreateHistory(ctx context.Context, h *models.RequestHistory) error {
	if _, err := r.db.NewInsert().Model(h).Exec(ctx); err != nil {
		return fmt.Errorf("insert request history: %w", err)
	}
	return nil
}

func (r *repository) CountEndpoints(ctx context.Context, accountID uuid.UUID) (int, error) {
	count, err := r.db.NewSelect().
		Model((*models.Endpoint)(nil)).
		Where("account_id = ?", accountID).
		Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("count endpoints: %w", err)
	}
	return count, nil
}

func (r *repository) CountRecentRequests(ctx context.Context, accountID uuid.UUID, since time.Time) (int, error) {
	count, err := r.db.NewSelect().
		Model((*models.RequestHistory)(nil)).
		Where("account_id = ?", accountID).
		Where("created_at >= ?", since).
		Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("count recent requests: %w", err)
	}
	return count, nil
}

func (r *repository) AvgLatency(ctx context.Context, accountID uuid.UUID, since time.Time) (float64, error) {
	var result struct {
		AvgLatency sql.NullFloat64 `bun:"avg_latency"`
	}
	err := r.db.NewSelect().
		ColumnExpr("AVG(latency) AS avg_latency").
		Table("request_history").
		Where("account_id = ?", accountID).
		Where("created_at >= ?", since).
		Scan(ctx, &result)
	if err != nil {
		return 0, fmt.Errorf("avg latency: %w", err)
	}
	if !result.AvgLatency.Valid {
		return 0, nil
	}
	return result.AvgLatency.Float64, nil
}

func (r *repository) ErrorRate(ctx context.Context, accountID uuid.UUID, since time.Time) (float64, error) {
	total, err := r.db.NewSelect().
		Model((*models.RequestHistory)(nil)).
		Where("account_id = ?", accountID).
		Where("created_at >= ?", since).
		Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("count total requests: %w", err)
	}
	if total == 0 {
		return 0, nil
	}

	errorCount, err := r.db.NewSelect().
		Model((*models.RequestHistory)(nil)).
		Where("account_id = ?", accountID).
		Where("created_at >= ?", since).
		Where("response LIKE '%\"error\"%'").
		Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("count error requests: %w", err)
	}

	return float64(errorCount) / float64(total) * 100, nil
}
