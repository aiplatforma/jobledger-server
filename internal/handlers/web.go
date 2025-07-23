package handlers

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/dusansimic/jobledger/internal/auth"
	"github.com/dusansimic/jobledger/internal/models"
	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
)

var templates = template.Must(template.ParseGlob("internal/templates/*.html"))
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

type DashboardData struct {
	Jobs           []models.Job
	NumberJobs     int
	PendingJobs    int
	InProgressJobs int
	FailedJobs     int
	CompletedJobs  int
}

func Dashboard(db *sqlx.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		jobs := []models.Job{}

		// get all jobs from the database
		err := db.Select(&jobs, "SELECT * FROM job")
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

		// count jobs by state
		numberJobs := len(jobs)
		pendingJobs := 0
		inProgressJobs := 0
		failedJobs := 0
		completedJobs := 0
		for _, job := range jobs {
			switch job.State {
			case "notstarted":
				pendingJobs++
			case "inprogress":
				inProgressJobs++
			case "fail":
				failedJobs++
			case "complete":
				completedJobs++
			}
		}

		// otherwise render the dashboard page with the jobs
		templates.ExecuteTemplate(w, "index.html", DashboardData{
			Jobs:           jobs,
			NumberJobs:     numberJobs,
			PendingJobs:    pendingJobs,
			InProgressJobs: inProgressJobs,
			FailedJobs:     failedJobs,
			CompletedJobs:  completedJobs,
		})
	}
}

type TokenData struct {
	ID              int
	Comment         string
	TimeLeft        time.Duration
	DurationExpired bool
	Token           string
}

func (td TokenData) TimeLeftFormatted() string {
	totalDays := int(td.TimeLeft.Hours() / 24)
	months := totalDays / 30
	days := totalDays % 30
	hours := int(td.TimeLeft.Hours()) % 24
	minutes := int(td.TimeLeft.Minutes()) % 60
	seconds := int(td.TimeLeft.Seconds()) % 60

	if months > 0 {
		return fmt.Sprintf("%dm %dd %02d:%02d:%02d", months, days, hours, minutes, seconds)
	} else if days > 0 {
		return fmt.Sprintf("%dd %02d:%02d:%02d", days, hours, minutes, seconds)
	} else {
		return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
	}
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
		tokensData[i] = TokenData{
			ID:              token.ID,
			Comment:         token.Comment,
			TimeLeft:        time.Until(token.CreatedAt.Add(token.Duration)),
			DurationExpired: time.Until(token.CreatedAt.Add(token.Duration)) <= 0,
			Token:           token.Token,
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
