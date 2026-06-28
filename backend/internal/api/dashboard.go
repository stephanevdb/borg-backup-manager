package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/borg-backup-manager/backend/internal/stream"
)

func (s *Server) dashboardStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage := 20
	sources, dests, jobsCount, _ := s.store.CountEntities(ctx)
	totalStorage, _ := s.store.SumLatestRunSizes(ctx)
	lastRun, _ := s.store.LastSuccessfulRun(ctx)
	runs, total, _ := s.store.ListRecentRuns(ctx, page, perPage)
	failed, _ := s.store.ListFailedUnacknowledged(ctx)
	jobs, _ := s.store.ListJobs(ctx)

	var lastBackup any
	if lastRun != nil {
		lastBackup = lastRun.FinishedAt
	}

	since := time.Now().AddDate(0, 0, -30)
	success30, _ := s.store.CountRunsByStatus(ctx, "success", since)
	failed30, _ := s.store.CountRunsByStatus(ctx, "failed", since)

	var storageWarnings []map[string]any
	destsList, _ := s.store.ListDestinations(ctx)
	for _, d := range destsList {
		st := s.engine.DestinationStorage(ctx, &d)
		if st.IsWarning {
			storageWarnings = append(storageWarnings, map[string]any{
				"destination_id": d.ID, "destination_name": d.Name, "percent_used": st.PercentUsed,
			})
		}
	}

	jobsOverview := make([]map[string]any, 0, len(jobs))
	for _, j := range jobs {
		item := map[string]any{
			"id": j.ID, "name": j.Name, "enabled": j.Enabled, "last_status": j.LastStatus,
			"last_run_at": j.LastRunAt, "schedule": j.Schedule,
		}
		item["progress"] = s.engine.GetProgress(j.ID)
		jobsOverview = append(jobsOverview, item)
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"sources": sources, "destinations": dests, "jobs": jobsCount,
		"total_storage": totalStorage, "last_backup": lastBackup,
		"recent_runs": runs, "total_runs": total, "page": page, "per_page": perPage,
		"failed_unacknowledged": failed,
		"stats_30d":             map[string]int{"success": success30, "failed": failed30},
		"storage_warnings":      storageWarnings,
		"jobs_overview":         jobsOverview,
	})
}

func (s *Server) storageHistory(w http.ResponseWriter, r *http.Request) {
	history, err := s.store.StorageHistory(r.Context())
	if err != nil {
		s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.writeJSON(w, http.StatusOK, history)
}

func (s *Server) dashboardProgressSSE(w http.ResponseWriter, r *http.Request) {
	ch := s.hub.Subscribe("dashboard")
	defer s.hub.Unsubscribe("dashboard", ch)
	stream.ServeSSE(w, r, ch)
}

func (s *Server) acknowledgeFailed(w http.ResponseWriter, r *http.Request) {
	_ = s.store.AcknowledgeFailedRuns(r.Context())
	s.writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (s *Server) healthOverview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	dests, _ := s.store.ListDestinations(ctx)
	jobs, _ := s.store.ListJobs(ctx)
	var cards []map[string]any
	for _, d := range dests {
		for _, j := range jobs {
			if j.DestinationID != d.ID {
				continue
			}
			checks, _ := s.store.ListRepoChecks(ctx, d.ID, &j.ID)
			lastStatus := "unknown"
			if len(checks) > 0 {
				lastStatus = checks[0].Status
			}
			cards = append(cards, map[string]any{
				"destination_id": d.ID, "destination_name": d.Name,
				"job_id": j.ID, "job_name": j.Name, "last_check_status": lastStatus,
			})
		}
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"success": true, "repos": cards})
}
