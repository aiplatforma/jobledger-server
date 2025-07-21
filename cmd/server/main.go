package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/dusansimic/jobledger/internal/db"
	"github.com/dusansimic/jobledger/internal/handlers"
	"github.com/dusansimic/jobledger/internal/middleware"
	"github.com/go-chi/chi/v5"
)

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.Info("Request", "method", r.Method, "url", r.URL)
		next.ServeHTTP(w, r)
	})
}

func main() {
	database := db.Init()
	defer database.Close()

	r := chi.NewRouter()

	r.Use(logMiddleware)

	// API routes
	r.Route("/api", func(r chi.Router) {
		r.Use(middleware.RequireAppAuth)
		r.Get("/job", handlers.GetJob(database))
		r.Post("/job", handlers.CreateJob(database))
		r.Post("/job/{id}/started", handlers.SetJobState(database, "inprogress"))
		r.Post("/job/{id}/complete", handlers.SetJobState(database, "complete"))
	})

	r.Get("/login", handlers.LoginPage)
	r.Post("/login", handlers.LoginForm)
	r.Get("/logout", handlers.Logout)
	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireUserAuth)
		r.Get("/", handlers.Dashboard(database))
		r.Get("/tokens", handlers.TokensPage(database))
		r.Post("/token", handlers.CreateToken(database))
		r.Delete("/token/{id}", handlers.DeleteToken(database))
	})

	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	slog.Info("Starting server", "addr", ":3000")
	err := http.ListenAndServe(":3000", r)
	if err != nil {
		slog.Error("Server failed", "error", err)
	}
}
