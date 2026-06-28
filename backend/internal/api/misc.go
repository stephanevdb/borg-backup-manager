package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/borg-backup-manager/backend/internal/auth"
	"github.com/borg-backup-manager/backend/internal/db"
	"github.com/borg-backup-manager/backend/internal/security"
	"github.com/go-chi/chi/v5"
)

func (s *Server) sshRoutes(r chi.Router) {
	r.Post("/generate", s.sshGenerate)
	r.Get("/keys", s.sshListKeys)
	r.Delete("/keys/{id}", s.sshDeleteKey)
	r.Post("/test-connection", s.sshTestConnection)
	r.Get("/scan-host", s.sshScanHost)
	r.Post("/confirm-key", s.sshConfirmKey)
}

func (s *Server) sshGenerate(w http.ResponseWriter, r *http.Request) {
	s.sshGenerateKey(w, r)
}

func (s *Server) sshListKeys(w http.ResponseWriter, r *http.Request) {
	keys, _ := s.store.ListSSHKeys(r.Context())
	out := make([]map[string]any, 0, len(keys))
	for _, k := range keys {
		out = append(out, map[string]any{
			"id": k.ID, "name": k.Name, "public_key": k.PublicKey,
			"key_type": k.KeyType, "created_at": k.CreatedAt, "created_by": k.CreatedBy,
		})
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"success": true, "keys": out})
}

func (s *Server) sshDeleteKey(w http.ResponseWriter, r *http.Request) {
	id := parseID(chi.URLParam(r, "id"))
	_ = s.store.DeleteSSHKey(r.Context(), id)
	s.writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (s *Server) sshTestConnection(w http.ResponseWriter, r *http.Request) {
	s.sshTest(w, r)
}

func (s *Server) sshScanHost(w http.ResponseWriter, r *http.Request) {
	s.sshScan(w, r)
}

func (s *Server) sshConfirmKey(w http.ResponseWriter, r *http.Request) {
	s.sshConfirm(w, r)
}

func (s *Server) auditRoutes(r chi.Router) {
	r.Get("/", s.listAudit)
	r.Get("/users", s.auditUsers)
}

func (s *Server) listAudit(w http.ResponseWriter, r *http.Request) {
	f := db.AuditFilter{
		Action: r.URL.Query().Get("action"), TargetType: r.URL.Query().Get("target_type"),
		User: r.URL.Query().Get("user"), Search: r.URL.Query().Get("q"),
	}
	if since := r.URL.Query().Get("since"); since != "" {
		if t, err := time.Parse(time.RFC3339, since); err == nil {
			f.Since = &t
		}
	}
	if from := r.URL.Query().Get("from"); from != "" {
		if t, err := time.Parse("2006-01-02", from); err == nil {
			f.Since = &t
		}
	}
	if until := r.URL.Query().Get("until"); until != "" {
		if t, err := time.Parse(time.RFC3339, until); err == nil {
			f.Until = &t
		}
	}
	if to := r.URL.Query().Get("to"); to != "" {
		if t, err := time.Parse("2006-01-02", to); err == nil {
			end := t.Add(24*time.Hour - time.Second)
			f.Until = &end
		}
	}
	if p := r.URL.Query().Get("page"); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			f.Page = n
		}
	}
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil {
			f.PerPage = n
		}
	}
	items, total, _ := s.store.ListAuditLogs(r.Context(), f)
	s.writeJSON(w, http.StatusOK, map[string]any{"success": true, "entries": items, "total": total})
}

func (s *Server) auditUsers(w http.ResponseWriter, r *http.Request) {
	users, _ := s.store.ListAuditUsers(r.Context())
	s.writeJSON(w, http.StatusOK, map[string]any{"success": true, "users": users})
}

func (s *Server) apiKeysRoutes(r chi.Router) {
	r.Get("/", s.listAPIKeys)
	r.Post("/", s.createAPIKey)
	r.Delete("/{id}", s.deleteAPIKey)
}

func (s *Server) listAPIKeys(w http.ResponseWriter, r *http.Request) {
	keys, _ := s.store.ListAPIKeys(r.Context())
	s.writeJSON(w, http.StatusOK, map[string]any{"success": true, "keys": keys})
}

func (s *Server) createAPIKey(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	prefix, _, full, err := auth.GenerateAPIKey()
	if err != nil {
		s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	user := authUsername(r)
	key := &db.APIKey{Name: body.Name, KeyHash: auth.HashAPIKey(full), Prefix: prefix, User: &user}
	_ = s.store.CreateAPIKey(r.Context(), key)
	s.writeJSON(w, http.StatusCreated, map[string]any{"success": true, "key": key, "secret": full})
}

func (s *Server) deleteAPIKey(w http.ResponseWriter, r *http.Request) {
	id := parseID(chi.URLParam(r, "id"))
	_ = s.store.DeleteAPIKey(r.Context(), id)
	s.writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (s *Server) notificationRoutes(r chi.Router) {
	r.Get("/", s.listNotifications)
	r.Post("/", s.createNotification)
	r.Put("/{id}", s.updateNotification)
	r.Delete("/{id}", s.deleteNotification)
	r.Post("/{id}/test", s.testNotification)
}

func (s *Server) listNotifications(w http.ResponseWriter, r *http.Request) {
	channels, _ := s.store.ListNotificationChannels(r.Context())
	s.writeJSON(w, http.StatusOK, map[string]any{"success": true, "channels": channels})
}

func (s *Server) createNotification(w http.ResponseWriter, r *http.Request) {
	var ch db.NotificationChannel
	_ = json.NewDecoder(r.Body).Decode(&ch)
	_ = s.store.CreateNotificationChannel(r.Context(), &ch)
	s.writeJSON(w, http.StatusCreated, map[string]any{"success": true, "channel": ch})
}

func (s *Server) updateNotification(w http.ResponseWriter, r *http.Request) {
	id := parseID(chi.URLParam(r, "id"))
	var ch db.NotificationChannel
	_ = json.NewDecoder(r.Body).Decode(&ch)
	ch.ID = id
	_ = s.store.UpdateNotificationChannel(r.Context(), &ch)
	s.writeJSON(w, http.StatusOK, map[string]any{"success": true, "channel": ch})
}

func (s *Server) deleteNotification(w http.ResponseWriter, r *http.Request) {
	id := parseID(chi.URLParam(r, "id"))
	_ = s.store.DeleteNotificationChannel(r.Context(), id)
	s.writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (s *Server) testNotification(w http.ResponseWriter, r *http.Request) {
	id := parseID(chi.URLParam(r, "id"))
	ch, err := s.store.GetNotificationChannel(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	err = s.notifier.SendTest(r.Context(), *ch)
	s.writeJSON(w, http.StatusOK, map[string]any{"success": err == nil, "error": errString(err)})
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (s *Server) settingsRoutes(r chi.Router) {
	r.Get("/", s.getSettings)
	r.Put("/", s.putSettings)
}

func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	settings, _ := s.store.GetAllSettings(r.Context())
	s.writeJSON(w, http.StatusOK, map[string]any{"success": true, "settings": settings})
}

func (s *Server) putSettings(w http.ResponseWriter, r *http.Request) {
	var body map[string]string
	_ = json.NewDecoder(r.Body).Decode(&body)
	for k, v := range body {
		_ = s.store.SetSetting(r.Context(), k, v)
	}
	s.writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (s *Server) browse(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		path = "/backup"
	}
	path = filepath.Clean(path)
	if !security.IsSafeBrowsePath(path) {
		s.writeJSON(w, http.StatusForbidden, map[string]string{"error": "Access to this directory is not permitted"})
		return
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			s.writeJSON(w, http.StatusNotFound, map[string]string{"error": "Directory does not exist: " + path})
			return
		}
		s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if !info.IsDir() {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Not a directory: " + path})
		return
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		s.writeJSON(w, http.StatusForbidden, map[string]string{"error": "Permission denied: " + path})
		return
	}

	type dirEntry struct {
		Name         string `json:"name"`
		Path         string `json:"path"`
		HasChildren  bool   `json:"has_children"`
	}
	var dirs []dirEntry
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		childPath := filepath.Join(path, e.Name())
		hasChildren := false
		if sub, err := os.ReadDir(childPath); err == nil {
			for _, se := range sub {
				if se.IsDir() && !strings.HasPrefix(se.Name(), ".") {
					hasChildren = true
					break
				}
			}
		}
		dirs = append(dirs, dirEntry{Name: e.Name(), Path: childPath, HasChildren: hasChildren})
	}

	parent := filepath.Dir(path)
	if parent == path || !security.IsSafeBrowsePath(parent) {
		parent = ""
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"success":     true,
		"current":     path,
		"parent":      parent,
		"directories": dirs,
	})
}

func (s *Server) configExport(w http.ResponseWriter, r *http.Request) {
	data, err := s.store.ExportConfig(r.Context())
	if err != nil {
		s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.audit(r.Context(), r, "export", "config", nil, "", nil)
	w.Header().Set("Content-Disposition", "attachment; filename=backup-config.json")
	s.writeJSON(w, http.StatusOK, data)
}

func (s *Server) configImport(w http.ResponseWriter, r *http.Request) {
	var data map[string]any
	_ = json.NewDecoder(r.Body).Decode(&data)
	stats, err := s.store.ImportConfig(r.Context(), data, authUsername(r))
	if err != nil {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	s.scheduler.SyncJobs(r.Context())
	s.audit(r.Context(), r, "import", "config", nil, "", stats)
	s.writeJSON(w, http.StatusOK, map[string]any{"success": true, "imported": stats})
}
