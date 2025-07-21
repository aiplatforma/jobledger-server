package handlers

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/dusansimic/jobledger/internal/models"
	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
)

func GetJob(db *sqlx.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		job := []models.Job{}

		err := db.Select(&job, "SELECT * FROM job WHERE state = 'notstarted' ORDER BY id ASC LIMIT 1")
		if err != nil {
			// if no new job is found, return 204 No Content
			if err == sql.ErrNoRows {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			// if there is an error, log it and return 500 Internal Server Error
			slog.Error("Failed to query job", "handler", "GetJob", "err", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "query failed",
			})
			return
		}

		if len(job) == 0 {
			// if no job is found, return 204 No Content
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// if a job is found, return it as JSON
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(job[0])
	}
}

func CreateJob(db *sqlx.DB) http.HandlerFunc {
	type JobPayload struct {
		Name     string             `json:"name"`
		Type     string             `json:"type"`
		State    string             `json:"state"`
		Metadata models.MetadataMap `json:"metadata"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		job := JobPayload{}

		err := json.NewDecoder(r.Body).Decode(&job)
		if err != nil {
			slog.Info("Failed to parse json", "handler", "CreateJob", "err", err)
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}

		query := `
			INSERT INTO job (name, type, state, created_at, started_at, completed_at, metadata)
			VALUES (:name, :type, :state, CURRENT_TIMESTAMP, NULL, NULL, :metadata);
		`

		_, err = db.NamedExec(query, &job)
		if err != nil {
			slog.Error("Failed to run named query", "handler", "CreateJob", "err", err)
			http.Error(w, "failed to run query", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

func SetJobState(db *sqlx.DB, state string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")

		query := `
			UPDATE job
			SET state = $1, started_at = CURRENT_TIMESTAMP
			WHERE id = $2;
		`

		_, err := db.Exec(query, state, id)
		if err != nil {
			slog.Error("Failed to update job state", "handler", "SetJobState", "err", err)
			http.Error(w, "failed to update job state", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}
