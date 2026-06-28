package auth

import (
	"context"
	"net/http"
	"strings"
)

type contextKey string

const userContextKey contextKey = "user"

func WithUser(ctx context.Context, u *User) context.Context {
	return context.WithValue(ctx, userContextKey, u)
}

func UserFromContext(ctx context.Context) *User {
	u, _ := ctx.Value(userContextKey).(*User)
	return u
}

func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.AuthDisabled {
			next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), DevUser())))
			return
		}
		if u := s.sessions.GetUser(r); u != nil {
			next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), u)))
			return
		}
		if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
			token := strings.TrimPrefix(h, "Bearer ")
			if u, err := s.ValidateBearer(r.Context(), token); err == nil {
				next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), u)))
				return
			}
		}
		if key := r.Header.Get("X-API-Key"); key != "" {
			if u, err := s.ValidateAPIKey(r.Context(), key); err == nil {
				next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), u)))
				return
			}
		}
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
	})
}

func (s *Service) OptionalMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.AuthDisabled {
			next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), DevUser())))
			return
		}
		if u := s.sessions.GetUser(r); u != nil {
			next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), u)))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func Username(ctx context.Context) string {
	u := UserFromContext(ctx)
	if u == nil {
		return "system"
	}
	if u.PreferredUsername != "" {
		return u.PreferredUsername
	}
	return u.Email
}
