package api

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/borg-backup-manager/backend/internal/db"
	"github.com/borg-backup-manager/backend/internal/stream"
	"github.com/go-chi/chi/v5"
)

func (s *Server) jobsRoutes(r chi.Router) {
	r.Get("/", s.listJobs)
	r.Post("/", s.createJob)
	r.Put("/{id}", s.updateJob)
	r.Delete("/{id}", s.deleteJob)
	r.Post("/{id}/toggle", s.toggleJob)
	r.Post("/{id}/run", s.runJob)
	r.Post("/{id}/cancel", s.cancelJob)
	r.Get("/{id}/runs", s.jobRuns)
	r.Get("/{id}/status", s.jobStatus)
	r.Get("/{id}/progress", s.jobProgress)
	r.Put("/{id}/restore-commands", s.saveRestoreCommands)
}

func (s *Server) listJobs(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListJobs(r.Context())
	if err != nil {
		s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"success": true, "jobs": items})
}

func (s *Server) createJob(w http.ResponseWriter, r *http.Request) {
	var req db.BackupJob
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	user := authUsername(r)
	req.CreatedBy = &user
	if err := s.store.CreateJob(r.Context(), &req); err != nil {
		s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.scheduler.SyncJobs(r.Context())
	id := req.ID
	s.audit(r.Context(), r, "create", "job", &id, req.Name, nil)
	s.writeJSON(w, http.StatusCreated, map[string]any{"success": true, "job": req})
}

func (s *Server) updateJob(w http.ResponseWriter, r *http.Request) {
	id := parseID(chi.URLParam(r, "id"))
	var req db.BackupJob
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req.ID = id
	if err := s.store.UpdateJob(r.Context(), &req); err != nil {
		s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.scheduler.SyncJobs(r.Context())
	s.audit(r.Context(), r, "update", "job", &id, req.Name, nil)
	s.writeJSON(w, http.StatusOK, map[string]any{"success": true, "job": req})
}

func (s *Server) deleteJob(w http.ResponseWriter, r *http.Request) {
	id := parseID(chi.URLParam(r, "id"))
	_ = s.store.DeleteJob(r.Context(), id)
	s.scheduler.SyncJobs(r.Context())
	s.audit(r.Context(), r, "delete", "job", &id, "", nil)
	s.writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (s *Server) toggleJob(w http.ResponseWriter, r *http.Request) {
	id := parseID(chi.URLParam(r, "id"))
	enabled, err := s.store.ToggleJob(r.Context(), id)
	if err != nil {
		s.writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	s.scheduler.SyncJobs(r.Context())
	s.writeJSON(w, http.StatusOK, map[string]any{"success": true, "enabled": enabled})
}

func (s *Server) runJob(w http.ResponseWriter, r *http.Request) {
	id := parseID(chi.URLParam(r, "id"))
	runID, err := s.engine.QueueJob(r.Context(), id)
	if err != nil {
		s.writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	s.audit(r.Context(), r, "run", "job", &id, "", nil)
	s.writeJSON(w, http.StatusOK, map[string]any{"success": true, "run_id": runID})
}

func (s *Server) cancelJob(w http.ResponseWriter, r *http.Request) {
	id := parseID(chi.URLParam(r, "id"))
	ok, msg := s.engine.CancelJob(r.Context(), id)
	s.writeJSON(w, http.StatusOK, map[string]any{"success": ok, "message": msg})
}

func (s *Server) jobRuns(w http.ResponseWriter, r *http.Request) {
	id := parseID(chi.URLParam(r, "id"))
	runs, _ := s.store.ListRunsForJob(r.Context(), id, 50)
	s.writeJSON(w, http.StatusOK, map[string]any{"success": true, "runs": runs})
}

func (s *Server) jobStatus(w http.ResponseWriter, r *http.Request) {
	id := parseID(chi.URLParam(r, "id"))
	run, err := s.store.ActiveRunForJob(r.Context(), id)
	if err != nil {
		s.writeJSON(w, http.StatusOK, map[string]any{"success": true, "status": "idle"})
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"success": true, "status": run.Status, "run": run})
}

func (s *Server) jobProgress(w http.ResponseWriter, r *http.Request) {
	id := parseID(chi.URLParam(r, "id"))
	p := s.engine.GetProgress(id)
	s.writeJSON(w, http.StatusOK, map[string]any{"success": true, "progress": p})
}

func (s *Server) saveRestoreCommands(w http.ResponseWriter, r *http.Request) {
	id := parseID(chi.URLParam(r, "id"))
	job, err := s.store.GetJob(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	var body struct {
		RestoreBeforeCommand *string `json:"restore_before_command"`
		RestoreAfterCommand  *string `json:"restore_after_command"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	job.RestoreBeforeCommand = body.RestoreBeforeCommand
	job.RestoreAfterCommand = body.RestoreAfterCommand
	_ = s.store.UpdateJob(r.Context(), job)
	s.writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (s *Server) archivesRoutes(r chi.Router) {
	r.Get("/", s.listArchives)
	r.Get("/diff", s.archivesDiff)
	r.Post("/delete-batch", s.deleteArchivesBatch)
	r.Get("/{name}/files", s.archiveFiles)
	r.Post("/{name}/restore", s.restoreArchive)
	r.Get("/{name}/download", s.downloadArchiveFile)
	r.Get("/{name}/download-tar", s.downloadArchiveTar)
}

func (s *Server) listArchives(w http.ResponseWriter, r *http.Request) {
	destID := parseID(r.URL.Query().Get("destination_id"))
	jobID := parseID(r.URL.Query().Get("job_id"))
	archives, err := s.engine.ListArchives(r.Context(), destID, jobID)
	if err != nil {
		s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"success": true, "archives": archives})
}

func (s *Server) archiveFiles(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	destID := parseID(r.URL.Query().Get("destination_id"))
	jobID := parseID(r.URL.Query().Get("job_id"))
	path := r.URL.Query().Get("path")
	files, err := s.engine.ListArchiveFiles(r.Context(), destID, jobID, name, path)
	if err != nil {
		s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"success": true, "files": files})
}

func (s *Server) restoreArchive(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	destID := parseID(r.URL.Query().Get("destination_id"))
	jobID := parseID(r.URL.Query().Get("job_id"))
	restoreID, err := s.engine.StartRestore(r.Context(), destID, jobID, name)
	if err != nil {
		s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.audit(r.Context(), r, "restore", "archive", nil, name, nil)
	s.writeJSON(w, http.StatusOK, map[string]any{"success": true, "restore_id": restoreID})
}

func (s *Server) deleteArchivesBatch(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DestinationID int64    `json:"destination_id"`
		JobID         int64    `json:"job_id"`
		Names         []string `json:"names"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if err := s.engine.DeleteArchives(r.Context(), body.DestinationID, body.JobID, body.Names); err != nil {
		s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (s *Server) archivesDiff(w http.ResponseWriter, r *http.Request) {
	destID := parseID(r.URL.Query().Get("destination_id"))
	jobID := parseID(r.URL.Query().Get("job_id"))
	a1 := r.URL.Query().Get("archive1")
	a2 := r.URL.Query().Get("archive2")
	result, err := s.engine.DiffArchives(r.Context(), destID, jobID, a1, a2)
	if err != nil {
		s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if e, ok := result["error"]; ok && e != nil {
		s.writeJSON(w, http.StatusOK, map[string]any{"success": false, "error": e})
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"success": true, "added": result["added"], "changed": result["changed"], "deleted": result["deleted"]})
}

func (s *Server) downloadArchiveFile(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	destID := parseID(r.URL.Query().Get("destination_id"))
	jobID := parseID(r.URL.Query().Get("job_id"))
	path := r.URL.Query().Get("path")
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename="+filepath.Base(path))
	if err := s.engine.ExportFile(w, r.Context(), destID, jobID, name, path); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) downloadArchiveTar(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	destID := parseID(r.URL.Query().Get("destination_id"))
	jobID := parseID(r.URL.Query().Get("job_id"))
	path := r.URL.Query().Get("path")
	w.Header().Set("Content-Type", "application/x-tar")
	w.Header().Set("Content-Disposition", "attachment; filename="+name+".tar")
	_ = s.engine.ExportTar(w, r.Context(), destID, jobID, name, path)
}

func (s *Server) runsRoutes(r chi.Router) {
	r.Get("/{id}/stream", s.runStreamSSE)
}

func (s *Server) runStreamSSE(w http.ResponseWriter, r *http.Request) {
	runID := parseID(chi.URLParam(r, "id"))
	topic := "runlog:" + strconv.FormatInt(runID, 10)
	ch := s.hub.Subscribe(topic)
	defer s.hub.Unsubscribe(topic, ch)
	stream.ServeSSE(w, r, ch)
}

func (s *Server) restoresRoutes(r chi.Router) {
	r.Get("/{id}/stream", s.restoreStreamSSE)
	r.Get("/{id}/status", s.restoreStatus)
}

func (s *Server) restoreStreamSSE(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ch := s.hub.Subscribe("restore:" + id)
	defer s.hub.Unsubscribe("restore:"+id, ch)
	stream.ServeSSE(w, r, ch)
}

func (s *Server) restoreStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	st := s.engine.RestoreStatus(id)
	s.writeJSON(w, http.StatusOK, map[string]any{"success": true, "restore": st})
}
