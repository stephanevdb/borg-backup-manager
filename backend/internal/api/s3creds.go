package api

import (
	"encoding/json"
	"net/http"

	"github.com/borg-backup-manager/backend/internal/db"
)

type s3CredPayload struct {
	S3AccessKey  string `json:"s3_access_key"`
	S3SecretKey  string `json:"s3_secret_key"`
	ClearS3Creds bool   `json:"clear_s3_credentials"`
}

func (s *Server) applyS3CredsFromMap(accessEnc, secretEnc **string, m map[string]any) error {
	raw, _ := json.Marshal(m)
	var p s3CredPayload
	_ = json.Unmarshal(raw, &p)
	if p.ClearS3Creds {
		*accessEnc, *secretEnc = nil, nil
		return nil
	}
	if p.S3AccessKey != "" {
		enc, err := s.encryptor.Encrypt(p.S3AccessKey)
		if err != nil {
			return err
		}
		*accessEnc = &enc
	}
	if p.S3SecretKey != "" {
		enc, err := s.encryptor.Encrypt(p.S3SecretKey)
		if err != nil {
			return err
		}
		*secretEnc = &enc
	}
	return nil
}

func decodeJSONMap(r *http.Request) (map[string]any, error) {
	var m map[string]any
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		return nil, err
	}
	return m, nil
}

func mapToSource(m map[string]any) (db.Source, error) {
	raw, err := json.Marshal(m)
	if err != nil {
		return db.Source{}, err
	}
	var s db.Source
	return s, json.Unmarshal(raw, &s)
}

func mapToDestination(m map[string]any) (db.Destination, error) {
	raw, err := json.Marshal(m)
	if err != nil {
		return db.Destination{}, err
	}
	var d db.Destination
	return d, json.Unmarshal(raw, &d)
}
