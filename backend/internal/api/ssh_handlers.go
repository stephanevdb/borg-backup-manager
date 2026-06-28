package api

import (
	"crypto/ed25519"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/borg-backup-manager/backend/internal/db"
	"golang.org/x/crypto/ssh"
)

func (s *Server) sshGenerateKey(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	sshPub, _ := ssh.NewPublicKey(pub)
	pubBytes := ssh.MarshalAuthorizedKey(sshPub)
	privBlock, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	privStr := string(pem.EncodeToMemory(privBlock))
	encPriv, _ := s.encryptor.Encrypt(privStr)
	user := authUsername(r)
	key := &db.SSHKey{Name: body.Name, PrivateKey: encPriv, PublicKey: string(pubBytes), KeyType: "ed25519", CreatedBy: &user}
	if err := s.store.CreateSSHKey(r.Context(), key); err != nil {
		s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.writeJSON(w, http.StatusCreated, map[string]any{
		"success": true, "key": map[string]any{
			"id": key.ID, "name": key.Name, "public_key": key.PublicKey, "key_type": key.KeyType,
		},
		"install_command": fmt.Sprintf("mkdir -p ~/.ssh && echo %q >> ~/.ssh/authorized_keys", string(pubBytes)),
	})
}

func (s *Server) sshTest(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Host string `json:"host"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	cmd := exec.Command("ssh", "-o", "BatchMode=yes", "-o", "ConnectTimeout=10", body.Host, "echo ok")
	out, err := cmd.CombinedOutput()
	s.writeJSON(w, http.StatusOK, map[string]any{"success": err == nil, "output": string(out)})
}

func (s *Server) sshScan(w http.ResponseWriter, r *http.Request) {
	host := r.URL.Query().Get("host")
	cmd := exec.Command("ssh-keyscan", host)
	out, err := cmd.Output()
	s.writeJSON(w, http.StatusOK, map[string]any{"success": err == nil, "keys": string(out)})
}

func (s *Server) sshConfirm(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Host    string `json:"host"`
		KeyLine string `json:"key_line"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	sshDir := filepath.Join(s.cfg.DataDir, "ssh")
	_ = os.MkdirAll(sshDir, 0o700)
	f, err := os.OpenFile(filepath.Join(sshDir, "known_hosts"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer f.Close()
	_, _ = f.WriteString(body.Host + " " + body.KeyLine + "\n")
	s.writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}
