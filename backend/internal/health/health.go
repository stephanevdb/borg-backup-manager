package health

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/borg-backup-manager/backend/internal/backup"
	"github.com/borg-backup-manager/backend/internal/config"
	"github.com/borg-backup-manager/backend/internal/db"
	"github.com/borg-backup-manager/backend/internal/scheduler"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Service struct {
	cfg       *config.Config
	store     *db.Store
	engine    *backup.Engine
	scheduler *scheduler.Service
}

func New(cfg *config.Config, store *db.Store, engine *backup.Engine, sched *scheduler.Service) *Service {
	return &Service{cfg: cfg, store: store, engine: engine, scheduler: sched}
}

type Check struct {
	Status string         `json:"status"`
	Detail map[string]any `json:",inline"`
}

type Report struct {
	Status    string           `json:"status"`
	Timestamp string           `json:"timestamp"`
	Version   string           `json:"version"`
	Checks    map[string]Check `json:"checks"`
	Summary   map[string]int   `json:"summary"`
}

func (s *Service) Live(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "timestamp": time.Now().UTC().Format(time.RFC3339)})
}

func (s *Service) Ready(ctx context.Context, w http.ResponseWriter, _ *http.Request) {
	r := s.build(ctx, false)
	code := http.StatusOK
	if r.Status == "unhealthy" {
		code = http.StatusServiceUnavailable
	}
	writeJSON(w, code, r)
}

func (s *Service) Full(ctx context.Context, w http.ResponseWriter, _ *http.Request) {
	r := s.build(ctx, true)
	code := http.StatusOK
	if r.Status == "unhealthy" {
		code = http.StatusServiceUnavailable
	} else if r.Status == "degraded" {
		code = s.cfg.HealthDegradedHTTPCode
	}
	writeJSON(w, code, r)
}

func (s *Service) build(ctx context.Context, deep bool) Report {
	checks := map[string]Check{}
	overall := "ok"

	if err := s.store.DB.PingContext(ctx); err != nil {
		checks["database"] = Check{Status: "error"}
		overall = "unhealthy"
	} else {
		checks["database"] = Check{Status: "ok"}
	}

	schedCount := 0
	if s.scheduler != nil {
		schedCount = s.scheduler.ScheduledJobCount()
	}
	checks["scheduler"] = Check{Status: "ok", Detail: map[string]any{"scheduled_jobs": schedCount}}

	if s.engine.IsWorkerAlive() {
		checks["backup_worker"] = Check{Status: "ok"}
	} else {
		checks["backup_worker"] = Check{Status: "error"}
		overall = "unhealthy"
	}

	ver := s.engine.BorgVersion()
	if ver == "" {
		checks["borg"] = Check{Status: "error"}
		overall = "unhealthy"
	} else {
		checks["borg"] = Check{Status: "ok", Detail: map[string]any{"version": ver}}
	}

	dataPath := s.cfg.DataDir
	if st, err := os.Stat(dataPath); err != nil || !st.IsDir() {
		checks["data_disk"] = Check{Status: "error"}
	} else {
		checks["data_disk"] = Check{Status: "ok", Detail: map[string]any{"path": dataPath}}
	}

	if s.cfg.BackupMountRequired {
		ok := false
		for _, p := range []string{"/backup", "/app/backup"} {
			if st, err := os.Stat(p); err == nil && st.IsDir() {
				ok = true
			}
		}
		if !ok {
			checks["backup_mount"] = Check{Status: "error"}
			overall = "unhealthy"
		} else {
			checks["backup_mount"] = Check{Status: "ok"}
		}
	}

	if deep && s.cfg.HealthDeepBorgCheck {
		checks["borg_deep"] = Check{Status: "skipped"}
	}

	queued, running, _ := s.store.CountQueuedRunning(ctx)
	if s.cfg.HealthMaxRunningMin > 0 && running > 0 {
		checks["stuck_runs"] = Check{Status: "ok"}
	}

	return Report{
		Status:    overall,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Version:   s.cfg.AppVersion,
		Checks:    checks,
		Summary:   map[string]int{"queued_runs": queued, "running_runs": running},
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

var healthGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: "backup_tool_health_status",
	Help: "Overall health 0=ok 1=degraded 2=unhealthy",
}, []string{})

func init() {
	prometheus.MustRegister(healthGauge)
}

func (s *Service) MetricsHandler(token string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token != "" {
			auth := r.Header.Get("Authorization")
			q := r.URL.Query().Get("token")
			if auth != "Bearer "+token && q != token {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		promhttp.Handler().ServeHTTP(w, r)
	})
}

func BorgInstalled() bool {
	_, err := exec.LookPath("borg")
	return err == nil
}
