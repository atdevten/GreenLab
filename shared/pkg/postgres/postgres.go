package postgres

import (
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"go.uber.org/zap"
)

type Config struct {
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// Connect opens and validates a PostgreSQL connection, fatal on failure.
func Connect(log *zap.Logger, cfg Config) *sqlx.DB {
	db, err := sqlx.Connect("postgres", cfg.DSN)
	if err != nil {
		log.Fatal("postgres connect failed", zap.Error(err))
	}
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	log.Info("connected to postgres")
	return db
}
