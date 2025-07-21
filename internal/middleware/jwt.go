package middleware

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/dusansimic/jobledger/internal/auth"
)

func RequireUserAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("jwt")
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		claims, err := auth.ValidateToken(cookie.Value)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		slog.Info("Authenticated user", "user", claims["username"])

		next.ServeHTTP(w, r)
	})
}

func RequireAppAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")

		claims, err := auth.ValidateToken(header)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "unauthorized",
			})
			return
		}

		slog.Info("Authenticated app", "comment", claims["comment"])

		next.ServeHTTP(w, r)
	})
}
