package auth

import (
	"encoding/gob"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gorilla/sessions"
)

func init() {
	gob.Register(&User{})
}

type SessionStore struct {
	store *sessions.CookieStore
}

func (s *SessionStore) Init(dir, secret string) error {
	_ = os.MkdirAll(dir, 0o700)
	s.store = sessions.NewCookieStore([]byte(secret))
	s.store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
	_ = filepath.Join(dir, ".keep")
	return nil
}

func (s *SessionStore) GetUser(r *http.Request) *User {
	sess, err := s.store.Get(r, "session")
	if err != nil {
		return nil
	}
	if u, ok := sess.Values["user"].(*User); ok {
		return u
	}
	return nil
}

func (s *SessionStore) SetUser(w http.ResponseWriter, r *http.Request, user *User) error {
	sess, err := s.store.Get(r, "session")
	if err != nil {
		return err
	}
	sess.Values["user"] = user
	return sess.Save(r, w)
}

func (s *SessionStore) Clear(w http.ResponseWriter, r *http.Request) error {
	sess, err := s.store.Get(r, "session")
	if err != nil {
		return err
	}
	sess.Options.MaxAge = -1
	return sess.Save(r, w)
}

func (s *SessionStore) WriteProbe(dir string) error {
	path := filepath.Join(dir, ".health_probe")
	return os.WriteFile(path, []byte("ok"), 0o600)
}

func (s *SessionStore) CanWrite(dir string) bool {
	return s.WriteProbe(dir) == nil
}
