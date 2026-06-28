package api

import (
	"net/http"

	"github.com/borg-backup-manager/backend/internal/security"
	"github.com/go-chi/chi/v5"
)

func (s *Server) sourcesRoutes(r chi.Router) {
	r.Get("/", s.listSources)
	r.Post("/", s.createSource)
	r.Put("/{id}", s.updateSource)
	r.Delete("/{id}", s.deleteSource)
}

func (s *Server) listSources(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListSources(r.Context())
	if err != nil {
		s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"success": true, "sources": items})
}

func (s *Server) createSource(w http.ResponseWriter, r *http.Request) {
	m, err := decodeJSONMap(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req, err := mapToSource(m)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Path != nil && !security.IsSafePathOrURL(*req.Path) {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsafe path"})
		return
	}
	user := authUsername(r)
	req.CreatedBy = &user
	if err := s.applyS3CredsFromMap(&req.S3AccessKeyEncrypted, &req.S3SecretKeyEncrypted, m); err != nil {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := s.store.CreateSource(r.Context(), &req); err != nil {
		s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	id := req.ID
	s.audit(r.Context(), r, "create", "source", &id, req.Name, nil)
	s.writeJSON(w, http.StatusCreated, map[string]any{"success": true, "source": req})
}

func (s *Server) updateSource(w http.ResponseWriter, r *http.Request) {
	id := parseID(chi.URLParam(r, "id"))
	existing, err := s.store.GetSource(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	m, err := decodeJSONMap(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req, err := mapToSource(m)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req.ID = id
	req.CreatedBy = existing.CreatedBy
	req.S3AccessKeyEncrypted = existing.S3AccessKeyEncrypted
	req.S3SecretKeyEncrypted = existing.S3SecretKeyEncrypted
	_ = s.applyS3CredsFromMap(&req.S3AccessKeyEncrypted, &req.S3SecretKeyEncrypted, m)
	if err := s.store.UpdateSource(r.Context(), &req); err != nil {
		s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.audit(r.Context(), r, "update", "source", &id, req.Name, nil)
	s.writeJSON(w, http.StatusOK, map[string]any{"success": true, "source": req})
}

func (s *Server) deleteSource(w http.ResponseWriter, r *http.Request) {
	id := parseID(chi.URLParam(r, "id"))
	src, _ := s.store.GetSource(r.Context(), id)
	_ = s.store.DeleteSource(r.Context(), id)
	name := ""
	if src != nil {
		name = src.Name
	}
	s.audit(r.Context(), r, "delete", "source", &id, name, nil)
	s.writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}
