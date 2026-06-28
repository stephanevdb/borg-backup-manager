package notify

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/smtp"
	"strings"

	"github.com/borg-backup-manager/backend/internal/db"
	"github.com/borg-backup-manager/backend/internal/security"
)

type Service struct {
	store *db.Store
}

func New(store *db.Store) *Service {
	return &Service{store: store}
}

func (s *Service) NotifyRun(ctx context.Context, job *db.BackupJob, run *db.BackupRun) {
	channels, _ := s.store.ListNotificationChannels(ctx)
	for _, ch := range channels {
		if !ch.Enabled {
			continue
		}
		match := (run.Status == "success" && ch.OnSuccess) || (run.Status == "failed" && ch.OnFailure)
		if !match {
			continue
		}
		for _, cid := range job.NotificationChannelIDs {
			if cid == ch.ID {
				_ = s.send(ch, fmt.Sprintf("Backup %s: job %s run %d", run.Status, job.Name, run.ID))
			}
		}
	}
}

func (s *Service) SendTest(_ context.Context, ch db.NotificationChannel) error {
	return s.send(ch, "Test notification from Borg Backup Manager")
}

func (s *Service) NotifyStorageWarning(ctx context.Context, destinationName, message string) {
	channels, _ := s.store.ListNotificationChannels(ctx)
	for _, ch := range channels {
		if ch.Enabled && ch.OnStorageWarning {
			_ = s.send(ch, message)
		}
	}
	_ = destinationName
}

func (s *Service) send(ch db.NotificationChannel, body string) error {
	switch ch.Type {
	case "smtp":
		return s.sendSMTP(ch, body)
	case "webhook":
		return s.sendWebhook(ch, body)
	default:
		return fmt.Errorf("unknown channel type %s", ch.Type)
	}
}

func (s *Service) sendSMTP(ch db.NotificationChannel, body string) error {
	host := ch.Config["host"]
	port := ch.Config["port"]
	if port == "" {
		port = "587"
	}
	from := ch.Config["from"]
	to := ch.Config["to"]
	msg := []byte(fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: Borg Backup Manager\r\n\r\n%s", from, to, body))
	addr := host + ":" + port
	auth := smtp.PlainAuth("", ch.Config["username"], ch.Config["password"], host)
	if ch.Config["use_tls"] == "true" {
		return smtp.SendMail(addr, auth, from, []string{to}, msg)
	}
	c, err := smtp.Dial(addr)
	if err != nil {
		return err
	}
	defer c.Close()
	if ok, _ := c.Extension("STARTTLS"); ok {
		_ = c.StartTLS(&tls.Config{ServerName: host})
	}
	if ch.Config["username"] != "" {
		_ = c.Auth(auth)
	}
	_ = c.Mail(from)
	_ = c.Rcpt(to)
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err = w.Write(msg); err != nil {
		return err
	}
	return w.Close()
}

func (s *Service) sendWebhook(ch db.NotificationChannel, body string) error {
	url := ch.Config["url"]
	if !security.IsSafeURL(url) {
		return fmt.Errorf("unsafe webhook url")
	}
	payload := fmt.Sprintf(`{"text":%q}`, body)
	if ch.Config["preset"] == "discord" {
		payload = fmt.Sprintf(`{"content":%q}`, body)
	}
	resp, err := http.Post(url, "application/json", strings.NewReader(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned %d", resp.StatusCode)
	}
	return nil
}
