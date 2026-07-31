// Package database manages the PostgreSQL connection and schema migrations.
package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/profause/aimocksvr/internal/config"

	// Registers the "pg" driver used to open the connection pool.
	_ "github.com/uptrace/bun/driver/pgdriver"
)

// Connect opens a PostgreSQL connection pool and returns a Bun ORM handle.
func Connect(cfg *config.Config) (*bun.DB, error) {
	sqlDB, err := sql.Open("pg", cfg.Database.URL)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(time.Hour)

	db := bun.NewDB(sqlDB, pgdialect.New())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return db, nil
}
