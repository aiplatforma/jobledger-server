package db

import (
	"log/slog"
	"os"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // PostgreSQL driver
)

var connStr string

func init() {
	connStr = os.Getenv("DATABASE_URL")
	if connStr == "" {
		slog.Error("Database connection string is not set. Please set the DATABASE_URL environment variable.")
		os.Exit(1)
	}
}

func Init() *sqlx.DB {
	db, err := sqlx.Open("postgres", connStr)
	if err != nil {
		slog.Error("Could not connect to database", "err", err)
		return nil
	}

	schema := `
		CREATE TABLE IF NOT EXISTS token (
			id SERIAL PRIMARY KEY,
			comment TEXT NOT NULL,
			duration BIGINT NOT NULL,
			token TEXT NOT NULL UNIQUE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS job (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			state TEXT NOT NULL DEFAULT 'notstarted',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			started_at TIMESTAMP NULL,
			completed_at TIMESTAMP NULL,
			metadata JSONB
		);

		CREATE INDEX IF NOT EXISTS idx_job_state ON job(state);
		CREATE INDEX IF NOT EXISTS idx_token_token ON token(token);
	`

	db.MustExec(schema)

	return db
}
