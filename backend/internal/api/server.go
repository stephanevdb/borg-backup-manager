package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/borg-backup-manager/backend/internal/auth"
	"github.com/borg-backup-manager/backend/internal/backup"
	"github.com/borg-backup-manager/backend/internal/config"
	"github.com/borg-backup-manager/backend/internal/crypto"
	"github.com/borg-backup-manager/backend/internal/db"
	"github.com/borg-backup-manager/backend/internal/health"
	"github.com/borg-backup-manager/backend/internal/notify"
	"github.com/borg-backup-manager/backend/internal/scheduler"
	"github.com/borg-backup-manager/backend/internal/stream"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

type Server struct {
	cfg       *config.Config
	store     *db.Store
	auth      *auth.Service
	engine    *backup.Engine
	scheduler *scheduler.Service
	health    *health.Service
	notifier  *notify.Service
	hub       *stream.Hub
	encryptor *crypto.Encryptor
	static    http.Handler
}

func NewServer(cfg *config.Config, store *db.Store, authSvc *auth.Service, engine *backup.Engine,
	sched *scheduler.Service, healthSvc *health.Service, notifier *notify.Service, hub *stream.Hub,
	enc *crypto.Encryptor, static http.Handler) *Server {
	return &Server{
		cfg: cfg, store: store, auth: authSvc, engine: engine, scheduler: sched,
		health: healthSvc, notifier: notifier, hub: hub, encryptor: enc, static: static,
	}
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Logger, middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173", "http://127.0.0.1:5173"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-API-Key"},
		AllowCredentials: true,
	}))

	r.Get("/health/live", s.health.Live)
	r.Get("/health/ready", func(w http.ResponseWriter, r *http.Request) { s.health.Ready(r.Context(), w, r) })
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) { s.health.Full(r.Context(), w, r) })
	r.Handle("/metrics", s.health.MetricsHandler(s.cfg.MetricsToken))

	r.Route("/auth", func(ar chi.Router) {
		ar.Get("/login", s.authLogin)
		ar.Get("/callback", s.authCallback)
		ar.Post("/logout", s.authLogout)
		ar.With(s.auth.OptionalMiddleware).Get("/me", s.authMe)
	})

	r.Route("/api/v1", func(api chi.Router) {
		api.Use(s.auth.Middleware)
		api.Get("/dashboard/stats", s.dashboardStats)
		api.Get("/dashboard/progress", s.dashboardProgressSSE)
		api.Get("/stats/storage-history", s.storageHistory)
		api.Post("/runs/acknowledge-failed", s.acknowledgeFailed)

		api.Route("/sources", s.sourcesRoutes)
		api.Route("/destinations", s.destinationsRoutes)
		api.Route("/jobs", s.jobsRoutes)
		api.Route("/archives", s.archivesRoutes)
		api.Route("/runs", s.runsRoutes)
		api.Route("/restores", s.restoresRoutes)
		api.Route("/ssh", s.sshRoutes)
		api.Get("/health/overview", s.healthOverview)
		api.Route("/audit-log", s.auditRoutes)
		api.Route("/api-keys", s.apiKeysRoutes)
		api.Route("/notification-channels", s.notificationRoutes)
		api.Route("/settings", s.settingsRoutes)
		api.Get("/browse", s.browse)
		api.Get("/config/export", s.configExport)
		api.Post("/config/import", s.configImport)
	})

	if s.static != nil {
		r.Get("/*", s.spaFallback)
	}
	return r
}

func (s *Server) spaFallback(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/health") || strings.HasPrefix(r.URL.Path, "/metrics") || strings.HasPrefix(r.URL.Path, "/auth/") {
		http.NotFound(w, r)
		return
	}
	// Try static file first
	if s.static != nil && r.URL.Path != "/" {
		r2 := r.Clone(r.Context())
		r2.URL.Path = r.URL.Path
		rec := &statusRecorder{ResponseWriter: w, status: 200}
		s.static.ServeHTTP(rec, r2)
		if rec.status != 404 {
			return
		}
	}
	if s.static != nil {
		r.URL.Path = "/"
		s.static.ServeHTTP(w, r)
	} else {
		http.NotFound(w, r)
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (s *Server) writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) audit(ctx context.Context, r *http.Request, action, targetType string, targetID *int64, targetName string, details any) {
	var detStr string
	if details != nil {
		b, _ := json.Marshal(details)
		detStr = string(b)
	}
	ip := r.RemoteAddr
	if i := strings.LastIndex(ip, ":"); i > 0 {
		ip = ip[:i]
	}
	_ = s.store.LogAction(ctx, auth.Username(ctx), action, targetType, targetID, targetName, detStr, ip)
}

func (s *Server) authLogin(w http.ResponseWriter, r *http.Request) {
	if s.cfg.AuthDisabled {
		_ = s.auth.Sessions().SetUser(w, r, auth.DevUser())
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	if !s.auth.OAuthEnabled() {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><body style="font-family:sans-serif;padding:2rem">
<h1>Authentication not configured</h1>
<p>Set <code>OIDC_CLIENT_ID</code>, <code>OIDC_CLIENT_SECRET</code>, and <code>OIDC_WELL_KNOWN_ENDPOINT</code>,
or set <code>AUTH_DISABLED=true</code> for local development.</p>
</body></html>`))
		return
	}
	state := fmt.Sprintf("%d", time.Now().UnixNano())
	redirect := schemeHost(r) + "/auth/callback"
	url := s.auth.LoginURL(redirect, state)
	http.Redirect(w, r, url, http.StatusFound)
}

func (s *Server) authCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	redirect := schemeHost(r) + "/auth/callback"
	user, err := s.auth.Exchange(r.Context(), redirect, code)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	_ = s.auth.Sessions().SetUser(w, r, user)
	http.Redirect(w, r, "/", http.StatusFound)
}

func (s *Server) authLogout(w http.ResponseWriter, r *http.Request) {
	_ = s.auth.Sessions().Clear(w, r)
	s.writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (s *Server) authMe(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	if u == nil {
		s.writeJSON(w, http.StatusOK, map[string]any{"authenticated": false})
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"authenticated": true, "user": u})
}

func schemeHost(r *http.Request) string {
	if r.TLS != nil {
		return "https://" + r.Host
	}
	if xf := r.Header.Get("X-Forwarded-Proto"); xf == "https" {
		return "https://" + r.Host
	}
	return "http://" + r.Host
}
