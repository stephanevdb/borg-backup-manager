package db

import (
	"database/sql"
	"time"
)

type Source struct {
	ID                   int64     `json:"id"`
	Name                 string    `json:"name"`
	Type                 string    `json:"type"`
	Path                 *string   `json:"path"`
	SSHKeyPath           *string   `json:"ssh_key_path"`
	Excludes             *string   `json:"excludes"`
	S3Endpoint           *string   `json:"s3_endpoint"`
	S3Bucket             *string   `json:"s3_bucket"`
	S3Prefix             *string   `json:"s3_prefix"`
	S3AccessKeyEnv       *string   `json:"s3_access_key_env"`
	S3SecretKeyEnv       *string   `json:"s3_secret_key_env"`
	S3AccessKeyEncrypted *string   `json:"-"`
	S3SecretKeyEncrypted *string   `json:"-"`
	S3CredentialsConfigured bool   `json:"s3_credentials_configured"`
	S3Region             *string   `json:"s3_region"`
	S3Provider           *string   `json:"s3_provider"`
	Notes                *string   `json:"notes"`
	CreatedAt            time.Time `json:"created_at"`
	CreatedBy            *string   `json:"created_by"`
	SSHHostKeyChecking   bool      `json:"ssh_host_key_checking"`
}

type Destination struct {
	ID                   int64     `json:"id"`
	Name                 string    `json:"name"`
	Type                 string    `json:"type"`
	RepoURL              *string   `json:"repo_url"`
	SSHKeyPath           *string   `json:"ssh_key_path"`
	PassphraseEnv        *string   `json:"passphrase_env"`
	S3Endpoint           *string   `json:"s3_endpoint"`
	S3Bucket             *string   `json:"s3_bucket"`
	S3AccessKeyEnv       *string   `json:"s3_access_key_env"`
	S3SecretKeyEnv       *string   `json:"s3_secret_key_env"`
	S3AccessKeyEncrypted *string   `json:"-"`
	S3SecretKeyEncrypted *string   `json:"-"`
	S3CredentialsConfigured bool   `json:"s3_credentials_configured"`
	S3Region             *string   `json:"s3_region"`
	S3Provider           *string   `json:"s3_provider"`
	Notes                *string   `json:"notes"`
	CreatedAt            time.Time `json:"created_at"`
	CreatedBy            *string   `json:"created_by"`
	SSHHostKeyChecking   bool      `json:"ssh_host_key_checking"`
}

type BackupJob struct {
	ID                       int64     `json:"id"`
	Name                     string    `json:"name"`
	SourceID                 int64     `json:"source_id"`
	SourceName               string    `json:"source_name,omitempty"`
	DestinationID            int64     `json:"destination_id"`
	DestinationName          string    `json:"destination_name,omitempty"`
	Schedule                 string    `json:"schedule"`
	Compression              string    `json:"compression"`
	RetentionDaily           int       `json:"retention_daily"`
	RetentionWeekly          int       `json:"retention_weekly"`
	RetentionMonthly         int       `json:"retention_monthly"`
	Enabled                  bool      `json:"enabled"`
	DailyVerificationEnabled bool      `json:"daily_verification_enabled"`
	SSHBeforeCommand         *string   `json:"ssh_before_command"`
	SSHAfterCommand          *string   `json:"ssh_after_command"`
	RestoreBeforeCommand     *string   `json:"restore_before_command"`
	RestoreAfterCommand      *string   `json:"restore_after_command"`
	LastRunAt                *time.Time `json:"last_run_at"`
	LastStatus               string    `json:"last_status"`
	CreatedAt                time.Time `json:"created_at"`
	CreatedBy                *string   `json:"created_by"`
	NotificationChannelIDs   []int64   `json:"notification_channel_ids"`
}

type BackupRun struct {
	ID               int64      `json:"id"`
	JobID            int64      `json:"job_id"`
	JobName          string     `json:"job_name,omitempty"`
	StartedAt        time.Time  `json:"started_at"`
	FinishedAt       *time.Time `json:"finished_at"`
	Status           string     `json:"status"`
	SizeBytes        *int64     `json:"size_bytes"`
	ArchiveName      *string    `json:"archive_name"`
	ArchiveSizeBytes *int64     `json:"archive_size_bytes"`
	DurationSeconds  *int       `json:"duration_seconds"`
	LogOutput        *string    `json:"log_output"`
	Acknowledged     bool       `json:"acknowledged"`
	IsHostKeyError   bool       `json:"is_host_key_error"`
}

type SSHKey struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	PrivateKey string    `json:"-"`
	PublicKey  string    `json:"public_key"`
	KeyType    string    `json:"key_type"`
	CreatedAt  time.Time `json:"created_at"`
	CreatedBy  *string   `json:"created_by"`
}

type RepoCheck struct {
	ID              int64      `json:"id"`
	DestinationID   int64      `json:"destination_id"`
	DestinationName string     `json:"destination_name,omitempty"`
	JobID           *int64     `json:"job_id"`
	CheckType       string     `json:"check_type"`
	Status          string     `json:"status"`
	StartedAt       time.Time  `json:"started_at"`
	FinishedAt      *time.Time `json:"finished_at"`
	DurationSeconds *int       `json:"duration_seconds"`
	LogOutput       *string    `json:"log_output"`
	TriggeredBy     *string    `json:"triggered_by"`
}

type AuditLog struct {
	ID         int64     `json:"id"`
	Timestamp  time.Time `json:"timestamp"`
	User       *string   `json:"user"`
	Action     string    `json:"action"`
	TargetType string    `json:"target_type"`
	TargetID   *int64    `json:"target_id"`
	TargetName *string   `json:"target_name"`
	Details    *string   `json:"details"`
	IPAddress  *string   `json:"ip_address"`
}

type APIKey struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	KeyHash    string     `json:"-"`
	Prefix     string     `json:"prefix"`
	User       *string    `json:"user"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  *time.Time `json:"expires_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
}

type NotificationChannel struct {
	ID               int64             `json:"id"`
	Name             string            `json:"name"`
	Type             string            `json:"type"`
	Enabled          bool              `json:"enabled"`
	OnSuccess        bool              `json:"on_success"`
	OnFailure        bool              `json:"on_failure"`
	OnStorageWarning bool              `json:"on_storage_warning"`
	Config           map[string]string `json:"config"`
	CreatedAt        time.Time         `json:"created_at"`
}

type AppSetting struct {
	Key       string    `json:"key"`
	Value     *string   `json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func intToBool(n int) bool {
	return n != 0
}

func scanSource(row interface{ Scan(...any) error }) (*Source, error) {
	var s Source
	var createdAt sql.NullTime
	var sshCheck int
	err := row.Scan(
		&s.ID, &s.Name, &s.Type, &s.Path, &s.SSHKeyPath, &s.Excludes,
		&s.S3Endpoint, &s.S3Bucket, &s.S3Prefix, &s.S3AccessKeyEnv, &s.S3SecretKeyEnv,
		&s.S3AccessKeyEncrypted, &s.S3SecretKeyEncrypted, &s.S3Region, &s.S3Provider,
		&s.Notes, &createdAt, &s.CreatedBy, &sshCheck,
	)
	if err != nil {
		return nil, err
	}
	if createdAt.Valid {
		s.CreatedAt = createdAt.Time
	}
	s.SSHHostKeyChecking = intToBool(sshCheck)
	s.S3CredentialsConfigured = s.S3AccessKeyEncrypted != nil && s.S3SecretKeyEncrypted != nil &&
		*s.S3AccessKeyEncrypted != "" && *s.S3SecretKeyEncrypted != ""
	return &s, nil
}
