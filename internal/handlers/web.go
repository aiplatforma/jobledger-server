package handlers

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/dusansimic/jobledger/internal/auth"
	"github.com/dusansimic/jobledger/internal/models"
	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
)

var templateFuncMap = template.FuncMap{
	"iterate": func(n int) []int {
		r := make([]int, n)
		for i := range r {
			r[i] = i + 1
		}
		return r
	},
}
var templates = template.Must(template.New("").Funcs(templateFuncMap).ParseGlob("internal/templates/*.html"))
var username, password string

func init() {
	username = os.Getenv("AUTH_USERNAME")
	if username == "" {
		slog.Warn("AUTH_USERNAME environment variable is not set, using default 'admin'")
		username = "admin"
	}

	password = os.Getenv("AUTH_PASSWORD")
	if password == "" {
		slog.Warn("AUTH_PASSWORD environment variable is not set, using default 'admin'")
		password = "admin"
	}
}

type Message struct {
	IsError   bool
	IsSuccess bool
	Content   string
}

type LoginPageData struct {
	Message Message
}

func LoginPage(w http.ResponseWriter, r *http.Request) {
	templates.ExecuteTemplate(w, "login.html", nil)
}

func LoginForm(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	formUsername := r.FormValue("username")
	formPassword := r.FormValue("password")

	if formUsername != username && formPassword != password {
		templates.ExecuteTemplate(w, "login.html", LoginPageData{
			Message: Message{
				IsError:   true,
				IsSuccess: false,
				Content:   "Username or password is incorrect",
			},
		})
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	token, _ := auth.GenerateUserToken(formUsername)
	http.SetCookie(w, &http.Cookie{
		Name:     "jwt",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   86400,
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "jwt",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

type DurationData struct {
	Duration *time.Duration
	Expired  bool
}

func (dd DurationData) DurationFormatted() string {
	if dd.Duration == nil {
		return "N/A"
	}
	totalDays := int(dd.Duration.Hours() / 24)
	months := totalDays / 30
	days := totalDays % 30
	hours := int(dd.Duration.Hours()) % 24
	minutes := int(dd.Duration.Minutes()) % 60
	seconds := int(dd.Duration.Seconds()) % 60

	if months > 0 {
		return fmt.Sprintf("%dm %dd %02d:%02d:%02d", months, days, hours, minutes, seconds)
	} else if days > 0 {
		return fmt.Sprintf("%dd %02d:%02d:%02d", days, hours, minutes, seconds)
	} else {
		return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
	}
}

type JobData struct {
	ID       int
	Name     string
	Type     string
	State    string
	Duration DurationData
}

type PaginationData struct {
	Page       int
	PageSize   int
	TotalPages int
}

type JobStats struct {
	NumberJobs     int
	PendingJobs    int
	InProgressJobs int
	FailedJobs     int
	CompletedJobs  int
}

type DashboardData struct {
	Jobs       []JobData
	Stats      JobStats
	Pagination PaginationData
}

func getJobStats(db *sqlx.DB) (*JobStats, error) {
	stats := JobStats{}

	// Query for getting pending, in-progress, failed, and completed jobs and total number of jobs
	err := db.Get(&stats.PendingJobs, "SELECT COUNT(*) FROM job WHERE state = 'notstarted'")
	if err != nil {
		return nil, fmt.Errorf("failed to count pending jobs: %w", err)
	}
	err = db.Get(&stats.InProgressJobs, "SELECT COUNT(*) FROM job WHERE state = 'inprogress'")
	if err != nil {
		return nil, fmt.Errorf("failed to count in-progress jobs: %w", err)
	}
	err = db.Get(&stats.FailedJobs, "SELECT COUNT(*) FROM job WHERE state = 'fail'")
	if err != nil {
		return nil, fmt.Errorf("failed to count failed jobs: %w", err)
	}
	err = db.Get(&stats.CompletedJobs, "SELECT COUNT(*) FROM job WHERE state = 'complete'")
	if err != nil {
		return nil, fmt.Errorf("failed to count completed jobs: %w", err)
	}
	err = db.Get(&stats.NumberJobs, "SELECT COUNT(*) FROM job")
	if err != nil {
		return nil, fmt.Errorf("failed to count total jobs: %w", err)
	}

	return &stats, nil
}

func Dashboard(db *sqlx.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Parse pagination parameters
		page := 1
		pageSize := 15

		if pageParam := r.URL.Query().Get("page"); pageParam != "" {
			if p, err := strconv.Atoi(pageParam); err == nil && p > 0 {
				page = p
			}
		}

		offset := (page - 1) * pageSize

		// Get total number of jobs for pagination
		var totalJobs int
		err := db.Get(&totalJobs, "SELECT COUNT(*) FROM job")
		if err != nil {
			slog.Error("Failed to count jobs", "handler", "Dashboard", "err", err)
			http.Error(w, "failed to count jobs", http.StatusInternalServerError)
			return
		}

		// Calculate total pages
		totalPages := (totalJobs + pageSize - 1) / pageSize

		jobs := []models.Job{}

		// get all jobs from the database
		err = db.Select(&jobs, "SELECT * FROM job ORDER BY created_at DESC LIMIT $1 OFFSET $2", pageSize, offset)
		if err != nil {
			// if there is an error, log it and return 500 Internal Server Error
			slog.Error("Failed to query jobs", "handler", "Dashboard", "err", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "query failed",
			})
			return
		}

		jobsData := make([]JobData, len(jobs))
		for i, job := range jobs {
			jobsData[i] = JobData{
				ID:    job.ID,
				Name:  job.Name,
				Type:  job.Type,
				State: job.State,
			}

			if job.CompletedAt == nil {
				if job.StartedAt == nil {
					jobsData[i].Duration = DurationData{
						Duration: nil,
						Expired:  false,
					}
				} else {
					duration := time.Since(*job.StartedAt)
					jobsData[i].Duration = DurationData{
						Duration: &duration,
						Expired:  false,
					}
				}
			} else {
				duration := job.CompletedAt.Sub(*job.StartedAt)
				jobsData[i].Duration = DurationData{
					Duration: &duration,
					Expired:  false,
				}
			}
		}

		// get job statistics
		stats, err := getJobStats(db)
		if err != nil {
			slog.Error("Failed to get job statistics", "handler", "Dashboard", "err", err)
			http.Error(w, "failed to get job statistics", http.StatusInternalServerError)
			return
		}

		// otherwise render the dashboard page with the jobs
		templates.ExecuteTemplate(w, "index.html", DashboardData{
			Jobs:  jobsData,
			Stats: *stats,
			Pagination: PaginationData{
				Page:       page,
				PageSize:   pageSize,
				TotalPages: totalPages,
			},
		})
	}
}

func JobPage(db *sqlx.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		jobID := chi.URLParam(r, "id")
		job := models.Job{}
		err := db.Get(&job, "SELECT * FROM job WHERE id = $1", jobID)
		if err != nil {
			slog.Error("Failed to query job", "handler", "JobPage", "id", jobID, "err", err)
			http.Error(w, "job not found", http.StatusNotFound)
			return
		}

		templates.ExecuteTemplate(w, "job.html", job)
	}
}

type TokenData struct {
	ID       int
	Comment  string
	Duration DurationData
	Token    string
}

type TokensPageData struct {
	Message Message
	Tokens  []TokenData
}

func queryTokens(db *sqlx.DB) ([]models.Token, error) {
	tokens := []models.Token{}
	err := db.Select(&tokens, "SELECT * FROM token")
	if err != nil {
		slog.Error("Failed to query tokens", "handler", "QueryTokens", "err", err)
		return nil, err
	}
	return tokens, nil
}

func renderTokensPage(w http.ResponseWriter, tokens []models.Token, message Message) {
	tokensData := make([]TokenData, len(tokens))
	for i, token := range tokens {
		duration := time.Until(token.CreatedAt.Add(token.Duration))
		tokensData[i] = TokenData{
			ID:      token.ID,
			Comment: token.Comment,
			Duration: DurationData{
				Duration: &duration,
				Expired:  duration <= 0,
			},
			Token: token.Token,
		}
	}

	templates.ExecuteTemplate(w, "tokens.html", TokensPageData{
		Message: message,
		Tokens:  tokensData,
	})
}

func TokensPage(db *sqlx.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tokens, err := queryTokens(db)
		if err != nil {
			slog.Error("Failed to query tokens", "handler", "TokensPage", "err", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "query failed",
			})
			return
		}

		renderTokensPage(w, tokens, Message{
			IsError:   false,
			IsSuccess: false,
		})
	}
}

func CreateToken(db *sqlx.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		comment := r.FormValue("comment")
		durationString := r.FormValue("duration")

		var duration time.Duration

		switch durationString {
		case "day":
			duration = auth.DURATION_DAY
		case "week":
			duration = auth.DURATION_WEEK
		case "month":
			duration = auth.DURATION_MONTH
		case "three months":
			duration = auth.DURATION_THREE_MONTHS
		case "six months":
			duration = auth.DURATION_SIX_MONTHS
		default:
			slog.Error("Invalid duration specified", "handler", "CreateToken", "duration", durationString)
			tokens, _ := queryTokens(db)
			renderTokensPage(w, tokens, Message{
				IsError:   true,
				IsSuccess: false,
				Content:   "Duration not specified",
			})
			http.Redirect(w, r, "/tokens", http.StatusSeeOther)
			return
		}

		token, _ := auth.GenerateAppToken(comment, duration)

		// insert token to db
		_, err := db.Exec("INSERT INTO token (comment, duration, token) VALUES ($1, $2, $3)", comment, duration, token)
		if err != nil {
			slog.Error("Failed to insert token", "handler", "CreateToken", "err", err)
			tokens, _ := queryTokens(db)
			renderTokensPage(w, tokens, Message{
				IsError:   true,
				IsSuccess: false,
				Content:   "Failed to create token",
			})
			http.Redirect(w, r, "/tokens", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/tokens", http.StatusSeeOther)
	}
}

func DeleteToken(db *sqlx.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")

		// delete token from db
		_, err := db.Exec("DELETE FROM token WHERE id = $1", id)
		if err != nil {
			slog.Error("Failed to delete token", "handler", "DeleteToken", "err", err)
			tokens, _ := queryTokens(db)
			renderTokensPage(w, tokens, Message{
				IsError:   true,
				IsSuccess: false,
				Content:   "Failed to delete token",
			})
			http.Redirect(w, r, "/tokens", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/tokens", http.StatusSeeOther)
	}
}
