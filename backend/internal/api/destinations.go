package api

import (
	"net/http"
	"strconv"

	"github.com/borg-backup-manager/backend/internal/auth"
	"github.com/borg-backup-manager/backend/internal/security"
	"github.com/go-chi/chi/v5"
)

func authUsername(r *http.Request) string {
	return auth.Username(r.Context())
}

func parseID(s string) int64 {
	id, _ := strconv.ParseInt(s, 10, 64)
	return id
}

func (s *Server) applyS3Creds(accessEnc, secretEnc **string, r *http.Request) error {
	return nil
}

func (s *Server) applyS3CredsUpdate(accessEnc, secretEnc **string, r *http.Request) error {
	return nil
}

func (s *Server) encryptField(val string) (*string, error) {
	if val == "" {
		return nil, nil
	}
	enc, err := s.encryptor.Encrypt(val)
	if err != nil {
		return nil, err
	}
	return &enc, nil
}

func (s *Server) destinationsRoutes(r chi.Router) {
	r.Get("/", s.listDestinations)
	r.Post("/", s.createDestination)
	r.Put("/{id}", s.updateDestination)
	r.Delete("/{id}", s.deleteDestination)
	r.Get("/storage", s.destinationsStorage)
	r.Post("/{id}/break-lock", s.breakLock)
	r.Post("/{id}/compact", s.compactRepo)
	r.Post("/{id}/verify", s.verifyRepo)
	r.Get("/{id}/health", s.destHealth)
	r.Get("/{id}/checks", s.destChecks)
}

func (s *Server) listDestinations(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListDestinations(r.Context())
	if err != nil {
		s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"success": true, "destinations": items})
}

func (s *Server) createDestination(w http.ResponseWriter, r *http.Request) {
	m, err := decodeJSONMap(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req, err := mapToDestination(m)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.RepoURL != nil && !security.IsSafePathOrURL(*req.RepoURL) {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsafe repo url"})
		return
	}
	user := authUsername(r)
	req.CreatedBy = &user
	_ = s.applyS3CredsFromMap(&req.S3AccessKeyEncrypted, &req.S3SecretKeyEncrypted, m)
	if err := s.store.CreateDestination(r.Context(), &req); err != nil {
		s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	id := req.ID
	s.audit(r.Context(), r, "create", "destination", &id, req.Name, nil)
	s.writeJSON(w, http.StatusCreated, map[string]any{"success": true, "destination": req})
}

func (s *Server) updateDestination(w http.ResponseWriter, r *http.Request) {
	id := parseID(chi.URLParam(r, "id"))
	existing, err := s.store.GetDestination(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	m, err := decodeJSONMap(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req, err := mapToDestination(m)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req.ID = id
	req.CreatedBy = existing.CreatedBy
	req.S3AccessKeyEncrypted = existing.S3AccessKeyEncrypted
	req.S3SecretKeyEncrypted = existing.S3SecretKeyEncrypted
	_ = s.applyS3CredsFromMap(&req.S3AccessKeyEncrypted, &req.S3SecretKeyEncrypted, m)
	if err := s.store.UpdateDestination(r.Context(), &req); err != nil {
		s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.audit(r.Context(), r, "update", "destination", &id, req.Name, nil)
	s.writeJSON(w, http.StatusOK, map[string]any{"success": true, "destination": req})
}

func (s *Server) deleteDestination(w http.ResponseWriter, r *http.Request) {
	id := parseID(chi.URLParam(r, "id"))
	_ = s.store.DeleteDestination(r.Context(), id)
	s.audit(r.Context(), r, "delete", "destination", &id, "", nil)
	s.writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (s *Server) destinationsStorage(w http.ResponseWriter, r *http.Request) {
	dests, _ := s.store.ListDestinations(r.Context())
	out := map[string]any{}
	for _, d := range dests {
		st := s.engine.DestinationStorage(r.Context(), &d)
		out[strconv.FormatInt(d.ID, 10)] = st
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"success": true, "storage": out})
}

func (s *Server) breakLock(w http.ResponseWriter, r *http.Request) {
	destID := parseID(chi.URLParam(r, "id"))
	jobID := parseID(r.URL.Query().Get("job_id"))
	if err := s.engine.BreakLock(r.Context(), destID, jobID); err != nil {
		s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (s *Server) compactRepo(w http.ResponseWriter, r *http.Request) {
	destID := parseID(chi.URLParam(r, "id"))
	jobID := parseID(r.URL.Query().Get("job_id"))
	if err := s.engine.Compact(r.Context(), destID, jobID); err != nil {
		s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (s *Server) verifyRepo(w http.ResponseWriter, r *http.Request) {
	destID := parseID(chi.URLParam(r, "id"))
	var jobID *int64
	if j := r.URL.Query().Get("job_id"); j != "" {
		id := parseID(j)
		jobID = &id
	}
	check, err := s.engine.Verify(r.Context(), destID, jobID, authUsername(r))
	if err != nil {
		s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"success": true, "check": check})
}

func (s *Server) destHealth(w http.ResponseWriter, r *http.Request) {
	destID := parseID(chi.URLParam(r, "id"))
	checks, _ := s.store.ListRepoChecks(r.Context(), destID, nil)
	s.writeJSON(w, http.StatusOK, map[string]any{"success": true, "checks": checks})
}

func (s *Server) destChecks(w http.ResponseWriter, r *http.Request) {
	destID := parseID(chi.URLParam(r, "id"))
	var jobID *int64
	if j := r.URL.Query().Get("job_id"); j != "" {
		id := parseID(j)
		jobID = &id
	}
	checks, _ := s.store.ListRepoChecks(r.Context(), destID, jobID)
	s.writeJSON(w, http.StatusOK, map[string]any{"success": true, "checks": checks})
}
