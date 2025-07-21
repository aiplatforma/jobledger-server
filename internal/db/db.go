package db

import (
	"log/slog"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // PostgreSQL driver
)

func Init() *sqlx.DB {
	// Connection string for PostgreSQL
	// Format: "postgres://username:password@localhost/dbname?sslmode=disable"
	connStr := "postgres://postgres:postgres@host.docker.internal:5432/jobledger?sslmode=disable"

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
