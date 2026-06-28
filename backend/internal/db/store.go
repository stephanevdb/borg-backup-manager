package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// --- Sources ---

func (s *Store) ListSources(ctx context.Context) ([]Source, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, name, type, path, ssh_key_path, excludes, s3_endpoint, s3_bucket, s3_prefix,
		s3_access_key_env, s3_secret_key_env, s3_access_key_encrypted, s3_secret_key_encrypted, s3_region, s3_provider,
		notes, created_at, created_by, ssh_host_key_checking FROM sources ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Source
	for rows.Next() {
		src, err := scanSource(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *src)
	}
	return out, rows.Err()
}

func (s *Store) GetSource(ctx context.Context, id int64) (*Source, error) {
	row := s.DB.QueryRowContext(ctx, `SELECT id, name, type, path, ssh_key_path, excludes, s3_endpoint, s3_bucket, s3_prefix,
		s3_access_key_env, s3_secret_key_env, s3_access_key_encrypted, s3_secret_key_encrypted, s3_region, s3_provider,
		notes, created_at, created_by, ssh_host_key_checking FROM sources WHERE id = ?`, id)
	return scanSource(row)
}

func (s *Store) CreateSource(ctx context.Context, src *Source) error {
	res, err := s.DB.ExecContext(ctx, `INSERT INTO sources (name, type, path, ssh_key_path, excludes, s3_endpoint, s3_bucket, s3_prefix,
		s3_access_key_env, s3_secret_key_env, s3_access_key_encrypted, s3_secret_key_encrypted, s3_region, s3_provider, notes, created_by, ssh_host_key_checking)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		src.Name, src.Type, src.Path, src.SSHKeyPath, src.Excludes, src.S3Endpoint, src.S3Bucket, src.S3Prefix,
		src.S3AccessKeyEnv, src.S3SecretKeyEnv, src.S3AccessKeyEncrypted, src.S3SecretKeyEncrypted,
		src.S3Region, src.S3Provider, src.Notes, src.CreatedBy, boolToInt(src.SSHHostKeyChecking))
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	src.ID = id
	return nil
}

func (s *Store) UpdateSource(ctx context.Context, src *Source) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE sources SET name=?, type=?, path=?, ssh_key_path=?, excludes=?, s3_endpoint=?, s3_bucket=?, s3_prefix=?,
		s3_access_key_env=?, s3_secret_key_env=?, s3_access_key_encrypted=?, s3_secret_key_encrypted=?, s3_region=?, s3_provider=?, notes=?, ssh_host_key_checking=? WHERE id=?`,
		src.Name, src.Type, src.Path, src.SSHKeyPath, src.Excludes, src.S3Endpoint, src.S3Bucket, src.S3Prefix,
		src.S3AccessKeyEnv, src.S3SecretKeyEnv, src.S3AccessKeyEncrypted, src.S3SecretKeyEncrypted,
		src.S3Region, src.S3Provider, src.Notes, boolToInt(src.SSHHostKeyChecking), src.ID)
	return err
}

func (s *Store) DeleteSource(ctx context.Context, id int64) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM sources WHERE id = ?`, id)
	return err
}

// --- Destinations ---

func scanDestination(row interface{ Scan(...any) error }) (*Destination, error) {
	var d Destination
	var createdAt sql.NullTime
	var sshCheck int
	err := row.Scan(
		&d.ID, &d.Name, &d.Type, &d.RepoURL, &d.SSHKeyPath, &d.PassphraseEnv,
		&d.S3Endpoint, &d.S3Bucket, &d.S3AccessKeyEnv, &d.S3SecretKeyEnv,
		&d.S3AccessKeyEncrypted, &d.S3SecretKeyEncrypted, &d.S3Region, &d.S3Provider,
		&d.Notes, &createdAt, &d.CreatedBy, &sshCheck,
	)
	if err != nil {
		return nil, err
	}
	if createdAt.Valid {
		d.CreatedAt = createdAt.Time
	}
	d.SSHHostKeyChecking = intToBool(sshCheck)
	d.S3CredentialsConfigured = d.S3AccessKeyEncrypted != nil && d.S3SecretKeyEncrypted != nil &&
		*d.S3AccessKeyEncrypted != "" && *d.S3SecretKeyEncrypted != ""
	return &d, nil
}

func (s *Store) ListDestinations(ctx context.Context) ([]Destination, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, name, type, repo_url, ssh_key_path, passphrase_env, s3_endpoint, s3_bucket,
		s3_access_key_env, s3_secret_key_env, s3_access_key_encrypted, s3_secret_key_encrypted, s3_region, s3_provider,
		notes, created_at, created_by, ssh_host_key_checking FROM destinations ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Destination
	for rows.Next() {
		d, err := scanDestination(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *d)
	}
	return out, rows.Err()
}

func (s *Store) GetDestination(ctx context.Context, id int64) (*Destination, error) {
	row := s.DB.QueryRowContext(ctx, `SELECT id, name, type, repo_url, ssh_key_path, passphrase_env, s3_endpoint, s3_bucket,
		s3_access_key_env, s3_secret_key_env, s3_access_key_encrypted, s3_secret_key_encrypted, s3_region, s3_provider,
		notes, created_at, created_by, ssh_host_key_checking FROM destinations WHERE id = ?`, id)
	return scanDestination(row)
}

func (s *Store) CreateDestination(ctx context.Context, d *Destination) error {
	res, err := s.DB.ExecContext(ctx, `INSERT INTO destinations (name, type, repo_url, ssh_key_path, passphrase_env, s3_endpoint, s3_bucket,
		s3_access_key_env, s3_secret_key_env, s3_access_key_encrypted, s3_secret_key_encrypted, s3_region, s3_provider, notes, created_by, ssh_host_key_checking)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.Name, d.Type, d.RepoURL, d.SSHKeyPath, d.PassphraseEnv, d.S3Endpoint, d.S3Bucket,
		d.S3AccessKeyEnv, d.S3SecretKeyEnv, d.S3AccessKeyEncrypted, d.S3SecretKeyEncrypted,
		d.S3Region, d.S3Provider, d.Notes, d.CreatedBy, boolToInt(d.SSHHostKeyChecking))
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	d.ID = id
	return nil
}

func (s *Store) UpdateDestination(ctx context.Context, d *Destination) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE destinations SET name=?, type=?, repo_url=?, ssh_key_path=?, passphrase_env=?, s3_endpoint=?, s3_bucket=?,
		s3_access_key_env=?, s3_secret_key_env=?, s3_access_key_encrypted=?, s3_secret_key_encrypted=?, s3_region=?, s3_provider=?, notes=?, ssh_host_key_checking=? WHERE id=?`,
		d.Name, d.Type, d.RepoURL, d.SSHKeyPath, d.PassphraseEnv, d.S3Endpoint, d.S3Bucket,
		d.S3AccessKeyEnv, d.S3SecretKeyEnv, d.S3AccessKeyEncrypted, d.S3SecretKeyEncrypted,
		d.S3Region, d.S3Provider, d.Notes, boolToInt(d.SSHHostKeyChecking), d.ID)
	return err
}

func (s *Store) DeleteDestination(ctx context.Context, id int64) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM destinations WHERE id = ?`, id)
	return err
}

// --- Jobs ---

func (s *Store) scanJob(row interface{ Scan(...any) error }) (*BackupJob, error) {
	var j BackupJob
	var enabled, dailyVerify int
	var lastRun sql.NullTime
	var createdAt sql.NullTime
	err := row.Scan(
		&j.ID, &j.Name, &j.SourceID, &j.DestinationID, &j.Schedule, &j.Compression,
		&j.RetentionDaily, &j.RetentionWeekly, &j.RetentionMonthly,
		&enabled, &dailyVerify, &j.SSHBeforeCommand, &j.SSHAfterCommand,
		&j.RestoreBeforeCommand, &j.RestoreAfterCommand, &lastRun, &j.LastStatus, &createdAt, &j.CreatedBy,
	)
	if err != nil {
		return nil, err
	}
	j.Enabled = intToBool(enabled)
	j.DailyVerificationEnabled = intToBool(dailyVerify)
	j.LastRunAt = timePtr(lastRun)
	if createdAt.Valid {
		j.CreatedAt = createdAt.Time
	}
	return &j, nil
}

func (s *Store) enrichJob(ctx context.Context, j *BackupJob) error {
	if src, err := s.GetSource(ctx, j.SourceID); err == nil {
		j.SourceName = src.Name
	}
	if dst, err := s.GetDestination(ctx, j.DestinationID); err == nil {
		j.DestinationName = dst.Name
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT channel_id FROM job_notification_channels WHERE job_id = ?`, j.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int64
		if err := rows.Scan(&cid); err != nil {
			return err
		}
		j.NotificationChannelIDs = append(j.NotificationChannelIDs, cid)
	}
	return rows.Err()
}

func (s *Store) ListJobs(ctx context.Context) ([]BackupJob, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, name, source_id, destination_id, schedule, compression,
		retention_daily, retention_weekly, retention_monthly, enabled, daily_verification_enabled,
		ssh_before_command, ssh_after_command, restore_before_command, restore_after_command,
		last_run_at, last_status, created_at, created_by FROM backup_jobs ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BackupJob
	for rows.Next() {
		j, err := s.scanJob(rows)
		if err != nil {
			return nil, err
		}
		if err := s.enrichJob(ctx, j); err != nil {
			return nil, err
		}
		out = append(out, *j)
	}
	return out, rows.Err()
}

func (s *Store) GetJob(ctx context.Context, id int64) (*BackupJob, error) {
	row := s.DB.QueryRowContext(ctx, `SELECT id, name, source_id, destination_id, schedule, compression,
		retention_daily, retention_weekly, retention_monthly, enabled, daily_verification_enabled,
		ssh_before_command, ssh_after_command, restore_before_command, restore_after_command,
		last_run_at, last_status, created_at, created_by FROM backup_jobs WHERE id = ?`, id)
	j, err := s.scanJob(row)
	if err != nil {
		return nil, err
	}
	if err := s.enrichJob(ctx, j); err != nil {
		return nil, err
	}
	return j, nil
}

func (s *Store) CreateJob(ctx context.Context, j *BackupJob) error {
	res, err := s.DB.ExecContext(ctx, `INSERT INTO backup_jobs (name, source_id, destination_id, schedule, compression,
		retention_daily, retention_weekly, retention_monthly, enabled, daily_verification_enabled,
		ssh_before_command, ssh_after_command, restore_before_command, restore_after_command, created_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		j.Name, j.SourceID, j.DestinationID, j.Schedule, j.Compression,
		j.RetentionDaily, j.RetentionWeekly, j.RetentionMonthly, boolToInt(j.Enabled), boolToInt(j.DailyVerificationEnabled),
		j.SSHBeforeCommand, j.SSHAfterCommand, j.RestoreBeforeCommand, j.RestoreAfterCommand, j.CreatedBy)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	j.ID = id
	return s.setJobChannels(ctx, j.ID, j.NotificationChannelIDs)
}

func (s *Store) UpdateJob(ctx context.Context, j *BackupJob) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE backup_jobs SET name=?, source_id=?, destination_id=?, schedule=?, compression=?,
		retention_daily=?, retention_weekly=?, retention_monthly=?, enabled=?, daily_verification_enabled=?,
		ssh_before_command=?, ssh_after_command=?, restore_before_command=?, restore_after_command=? WHERE id=?`,
		j.Name, j.SourceID, j.DestinationID, j.Schedule, j.Compression,
		j.RetentionDaily, j.RetentionWeekly, j.RetentionMonthly, boolToInt(j.Enabled), boolToInt(j.DailyVerificationEnabled),
		j.SSHBeforeCommand, j.SSHAfterCommand, j.RestoreBeforeCommand, j.RestoreAfterCommand, j.ID)
	if err != nil {
		return err
	}
	return s.setJobChannels(ctx, j.ID, j.NotificationChannelIDs)
}

func (s *Store) setJobChannels(ctx context.Context, jobID int64, channelIDs []int64) error {
	if _, err := s.DB.ExecContext(ctx, `DELETE FROM job_notification_channels WHERE job_id = ?`, jobID); err != nil {
		return err
	}
	for _, cid := range channelIDs {
		if _, err := s.DB.ExecContext(ctx, `INSERT INTO job_notification_channels (job_id, channel_id) VALUES (?, ?)`, jobID, cid); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) DeleteJob(ctx context.Context, id int64) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM backup_jobs WHERE id = ?`, id)
	return err
}

func (s *Store) ToggleJob(ctx context.Context, id int64) (bool, error) {
	j, err := s.GetJob(ctx, id)
	if err != nil {
		return false, err
	}
	j.Enabled = !j.Enabled
	_, err = s.DB.ExecContext(ctx, `UPDATE backup_jobs SET enabled = ? WHERE id = ?`, boolToInt(j.Enabled), id)
	return j.Enabled, err
}

// --- Runs ---

func (s *Store) scanRun(row interface{ Scan(...any) error }) (*BackupRun, error) {
	var r BackupRun
	var finished sql.NullTime
	var started sql.NullTime
	var ack, hostKey int
	err := row.Scan(
		&r.ID, &r.JobID, &started, &finished, &r.Status, &r.SizeBytes, &r.ArchiveName,
		&r.ArchiveSizeBytes, &r.DurationSeconds, &r.LogOutput, &ack, &hostKey,
	)
	if err != nil {
		return nil, err
	}
	if started.Valid {
		r.StartedAt = started.Time
	}
	r.FinishedAt = timePtr(finished)
	r.Acknowledged = intToBool(ack)
	r.IsHostKeyError = intToBool(hostKey)
	return &r, nil
}

func (s *Store) CreateRun(ctx context.Context, jobID int64) (*BackupRun, error) {
	now := time.Now().UTC()
	res, err := s.DB.ExecContext(ctx, `INSERT INTO backup_runs (job_id, started_at, status) VALUES (?, ?, 'queued')`, jobID, now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	_, _ = s.DB.ExecContext(ctx, `UPDATE backup_jobs SET last_status='queued', last_run_at=? WHERE id=?`, now, jobID)
	return s.GetRun(ctx, id)
}

func (s *Store) GetRun(ctx context.Context, id int64) (*BackupRun, error) {
	row := s.DB.QueryRowContext(ctx, `SELECT id, job_id, started_at, finished_at, status, size_bytes, archive_name,
		archive_size_bytes, duration_seconds, log_output, acknowledged, is_host_key_error FROM backup_runs WHERE id = ?`, id)
	r, err := s.scanRun(row)
	if err != nil {
		return nil, err
	}
	if j, err := s.GetJob(ctx, r.JobID); err == nil {
		r.JobName = j.Name
	}
	return r, nil
}

func (s *Store) ListRunsForJob(ctx context.Context, jobID int64, limit int) ([]BackupRun, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT id, job_id, started_at, finished_at, status, size_bytes, archive_name,
		archive_size_bytes, duration_seconds, log_output, acknowledged, is_host_key_error FROM backup_runs
		WHERE job_id = ? ORDER BY started_at DESC LIMIT ?`, jobID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BackupRun
	for rows.Next() {
		r, err := s.scanRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

func (s *Store) GetNextQueuedRun(ctx context.Context) (*BackupRun, error) {
	row := s.DB.QueryRowContext(ctx, `SELECT id, job_id, started_at, finished_at, status, size_bytes, archive_name,
		archive_size_bytes, duration_seconds, log_output, acknowledged, is_host_key_error FROM backup_runs
		WHERE status = 'queued' ORDER BY started_at ASC LIMIT 1`)
	return s.scanRun(row)
}

func (s *Store) UpdateRun(ctx context.Context, r *BackupRun) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE backup_runs SET finished_at=?, status=?, size_bytes=?, archive_name=?,
		archive_size_bytes=?, duration_seconds=?, log_output=?, acknowledged=?, is_host_key_error=? WHERE id=?`,
		nullTime(r.FinishedAt), r.Status, r.SizeBytes, r.ArchiveName, r.ArchiveSizeBytes,
		r.DurationSeconds, r.LogOutput, boolToInt(r.Acknowledged), boolToInt(r.IsHostKeyError), r.ID)
	return err
}

func (s *Store) RecoverInterruptedRuns(ctx context.Context) (int64, error) {
	res, err := s.DB.ExecContext(ctx, `UPDATE backup_runs SET status='failed', finished_at=datetime('now'),
		log_output=COALESCE(log_output,'') || char(10) || 'Interrupted by server restart' WHERE status IN ('running','queued')`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *Store) AcknowledgeFailedRuns(ctx context.Context) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE backup_runs SET acknowledged=1 WHERE status='failed' AND acknowledged=0`)
	return err
}

func (s *Store) ActiveRunForJob(ctx context.Context, jobID int64) (*BackupRun, error) {
	row := s.DB.QueryRowContext(ctx, `SELECT id, job_id, started_at, finished_at, status, size_bytes, archive_name,
		archive_size_bytes, duration_seconds, log_output, acknowledged, is_host_key_error FROM backup_runs
		WHERE job_id = ? AND status IN ('queued','running') ORDER BY started_at DESC LIMIT 1`, jobID)
	return s.scanRun(row)
}

func (s *Store) CountRunsByStatus(ctx context.Context, status string, since time.Time) (int, error) {
	var n int
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM backup_runs WHERE status=? AND started_at >= ?`, status, since).Scan(&n)
	return n, err
}

func (s *Store) ListRecentRuns(ctx context.Context, page, perPage int) ([]BackupRun, int, error) {
	if perPage <= 0 {
		perPage = 20
	}
	if page < 1 {
		page = 1
	}
	var total int
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM backup_runs`).Scan(&total); err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * perPage
	rows, err := s.DB.QueryContext(ctx, `SELECT id, job_id, started_at, finished_at, status, size_bytes, archive_name,
		archive_size_bytes, duration_seconds, log_output, acknowledged, is_host_key_error FROM backup_runs
		ORDER BY started_at DESC LIMIT ? OFFSET ?`, perPage, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []BackupRun
	for rows.Next() {
		r, err := s.scanRun(rows)
		if err != nil {
			return nil, 0, err
		}
		if j, err := s.GetJob(ctx, r.JobID); err == nil {
			r.JobName = j.Name
		}
		out = append(out, *r)
	}
	return out, total, rows.Err()
}

func (s *Store) ListFailedUnacknowledged(ctx context.Context) ([]BackupRun, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, job_id, started_at, finished_at, status, size_bytes, archive_name,
		archive_size_bytes, duration_seconds, log_output, acknowledged, is_host_key_error FROM backup_runs
		WHERE status='failed' AND acknowledged=0 ORDER BY started_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BackupRun
	for rows.Next() {
		r, err := s.scanRun(rows)
		if err != nil {
			return nil, err
		}
		if j, err := s.GetJob(ctx, r.JobID); err == nil {
			r.JobName = j.Name
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

// --- SSH Keys ---

func (s *Store) ListSSHKeys(ctx context.Context) ([]SSHKey, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, name, private_key, public_key, key_type, created_at, created_by FROM ssh_keys ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SSHKey
	for rows.Next() {
		var k SSHKey
		var created sql.NullTime
		if err := rows.Scan(&k.ID, &k.Name, &k.PrivateKey, &k.PublicKey, &k.KeyType, &created, &k.CreatedBy); err != nil {
			return nil, err
		}
		if created.Valid {
			k.CreatedAt = created.Time
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

func (s *Store) GetSSHKey(ctx context.Context, id int64) (*SSHKey, error) {
	var k SSHKey
	var created sql.NullTime
	err := s.DB.QueryRowContext(ctx, `SELECT id, name, private_key, public_key, key_type, created_at, created_by FROM ssh_keys WHERE id=?`, id).
		Scan(&k.ID, &k.Name, &k.PrivateKey, &k.PublicKey, &k.KeyType, &created, &k.CreatedBy)
	if err != nil {
		return nil, err
	}
	if created.Valid {
		k.CreatedAt = created.Time
	}
	return &k, nil
}

func (s *Store) GetSSHKeyByName(ctx context.Context, name string) (*SSHKey, error) {
	var k SSHKey
	var created sql.NullTime
	err := s.DB.QueryRowContext(ctx, `SELECT id, name, private_key, public_key, key_type, created_at, created_by FROM ssh_keys WHERE name=?`, name).
		Scan(&k.ID, &k.Name, &k.PrivateKey, &k.PublicKey, &k.KeyType, &created, &k.CreatedBy)
	if err != nil {
		return nil, err
	}
	if created.Valid {
		k.CreatedAt = created.Time
	}
	return &k, nil
}

func (s *Store) CreateSSHKey(ctx context.Context, k *SSHKey) error {
	res, err := s.DB.ExecContext(ctx, `INSERT INTO ssh_keys (name, private_key, public_key, key_type, created_by) VALUES (?,?,?,?,?)`,
		k.Name, k.PrivateKey, k.PublicKey, k.KeyType, k.CreatedBy)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	k.ID = id
	return nil
}

func (s *Store) DeleteSSHKey(ctx context.Context, id int64) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM ssh_keys WHERE id=?`, id)
	return err
}

// --- Audit ---

func (s *Store) LogAction(ctx context.Context, user, action, targetType string, targetID *int64, targetName, details, ip string) error {
	_, err := s.DB.ExecContext(ctx, `INSERT INTO audit_logs (user, action, target_type, target_id, target_name, details, ip_address)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, user, action, targetType, targetID, targetName, details, ip)
	return err
}

type AuditFilter struct {
	Action     string
	TargetType string
	User       string
	Search     string
	Since      *time.Time
	Until      *time.Time
	Page       int
	PerPage    int
}

func (s *Store) ListAuditLogs(ctx context.Context, f AuditFilter) ([]AuditLog, int, error) {
	if f.PerPage <= 0 {
		f.PerPage = 50
	}
	if f.Page < 1 {
		f.Page = 1
	}
	where := []string{"1=1"}
	args := []any{}
	if f.Action != "" {
		where = append(where, "action = ?")
		args = append(args, f.Action)
	}
	if f.TargetType != "" {
		where = append(where, "target_type = ?")
		args = append(args, f.TargetType)
	}
	if f.User != "" {
		where = append(where, "user = ?")
		args = append(args, f.User)
	}
	if f.Search != "" {
		where = append(where, "(target_name LIKE ? OR details LIKE ?)")
		q := "%" + f.Search + "%"
		args = append(args, q, q)
	}
	if f.Since != nil {
		where = append(where, "timestamp >= ?")
		args = append(args, f.Since)
	}
	if f.Until != nil {
		where = append(where, "timestamp <= ?")
		args = append(args, f.Until)
	}
	w := strings.Join(where, " AND ")
	var total int
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_logs WHERE `+w, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	offset := (f.Page - 1) * f.PerPage
	args = append(args, f.PerPage, offset)
	rows, err := s.DB.QueryContext(ctx, `SELECT id, timestamp, user, action, target_type, target_id, target_name, details, ip_address
		FROM audit_logs WHERE `+w+` ORDER BY timestamp DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []AuditLog
	for rows.Next() {
		var a AuditLog
		var ts sql.NullTime
		if err := rows.Scan(&a.ID, &ts, &a.User, &a.Action, &a.TargetType, &a.TargetID, &a.TargetName, &a.Details, &a.IPAddress); err != nil {
			return nil, 0, err
		}
		if ts.Valid {
			a.Timestamp = ts.Time
		}
		out = append(out, a)
	}
	return out, total, rows.Err()
}

func (s *Store) ListAuditUsers(ctx context.Context) ([]string, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT DISTINCT user FROM audit_logs WHERE user IS NOT NULL ORDER BY user`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// --- API Keys ---

func (s *Store) ListAPIKeys(ctx context.Context) ([]APIKey, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, name, key_hash, prefix, user, created_at, expires_at, last_used_at FROM api_keys ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []APIKey
	for rows.Next() {
		var k APIKey
		var created, expires, lastUsed sql.NullTime
		if err := rows.Scan(&k.ID, &k.Name, &k.KeyHash, &k.Prefix, &k.User, &created, &expires, &lastUsed); err != nil {
			return nil, err
		}
		if created.Valid {
			k.CreatedAt = created.Time
		}
		k.ExpiresAt = timePtr(expires)
		k.LastUsedAt = timePtr(lastUsed)
		out = append(out, k)
	}
	return out, rows.Err()
}

func (s *Store) CreateAPIKey(ctx context.Context, k *APIKey) error {
	res, err := s.DB.ExecContext(ctx, `INSERT INTO api_keys (name, key_hash, prefix, user, expires_at) VALUES (?,?,?,?,?)`,
		k.Name, k.KeyHash, k.Prefix, k.User, nullTime(k.ExpiresAt))
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	k.ID = id
	return nil
}

func (s *Store) GetAPIKeyByHash(ctx context.Context, hash string) (*APIKey, error) {
	var k APIKey
	var created, expires, lastUsed sql.NullTime
	err := s.DB.QueryRowContext(ctx, `SELECT id, name, key_hash, prefix, user, created_at, expires_at, last_used_at FROM api_keys WHERE key_hash=?`, hash).
		Scan(&k.ID, &k.Name, &k.KeyHash, &k.Prefix, &k.User, &created, &expires, &lastUsed)
	if err != nil {
		return nil, err
	}
	if created.Valid {
		k.CreatedAt = created.Time
	}
	k.ExpiresAt = timePtr(expires)
	k.LastUsedAt = timePtr(lastUsed)
	return &k, nil
}

func (s *Store) DeleteAPIKey(ctx context.Context, id int64) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM api_keys WHERE id=?`, id)
	return err
}

func (s *Store) TouchAPIKey(ctx context.Context, id int64) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE api_keys SET last_used_at=datetime('now') WHERE id=?`, id)
	return err
}

// --- Notifications ---

func (s *Store) ListNotificationChannels(ctx context.Context) ([]NotificationChannel, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, name, type, enabled, on_success, on_failure, on_storage_warning, config, created_at FROM notification_channels ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NotificationChannel
	for rows.Next() {
		var c NotificationChannel
		var enabled, onSuccess, onFailure, onWarn int
		var configJSON sql.NullString
		var created sql.NullTime
		if err := rows.Scan(&c.ID, &c.Name, &c.Type, &enabled, &onSuccess, &onFailure, &onWarn, &configJSON, &created); err != nil {
			return nil, err
		}
		c.Enabled = intToBool(enabled)
		c.OnSuccess = intToBool(onSuccess)
		c.OnFailure = intToBool(onFailure)
		c.OnStorageWarning = intToBool(onWarn)
		c.Config = map[string]string{}
		if configJSON.Valid && configJSON.String != "" {
			_ = json.Unmarshal([]byte(configJSON.String), &c.Config)
		}
		if created.Valid {
			c.CreatedAt = created.Time
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) GetNotificationChannel(ctx context.Context, id int64) (*NotificationChannel, error) {
	channels, err := s.ListNotificationChannels(ctx)
	if err != nil {
		return nil, err
	}
	for _, c := range channels {
		if c.ID == id {
			return &c, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (s *Store) CreateNotificationChannel(ctx context.Context, c *NotificationChannel) error {
	cfg, _ := json.Marshal(c.Config)
	res, err := s.DB.ExecContext(ctx, `INSERT INTO notification_channels (name, type, enabled, on_success, on_failure, on_storage_warning, config)
		VALUES (?,?,?,?,?,?,?)`, c.Name, c.Type, boolToInt(c.Enabled), boolToInt(c.OnSuccess), boolToInt(c.OnFailure), boolToInt(c.OnStorageWarning), string(cfg))
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	c.ID = id
	return nil
}

func (s *Store) UpdateNotificationChannel(ctx context.Context, c *NotificationChannel) error {
	cfg, _ := json.Marshal(c.Config)
	_, err := s.DB.ExecContext(ctx, `UPDATE notification_channels SET name=?, type=?, enabled=?, on_success=?, on_failure=?, on_storage_warning=?, config=? WHERE id=?`,
		c.Name, c.Type, boolToInt(c.Enabled), boolToInt(c.OnSuccess), boolToInt(c.OnFailure), boolToInt(c.OnStorageWarning), string(cfg), c.ID)
	return err
}

func (s *Store) DeleteNotificationChannel(ctx context.Context, id int64) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM notification_channels WHERE id=?`, id)
	return err
}

// --- Settings ---

var DefaultSettings = map[string]string{
	"ssh_address_family":         "",
	"default_compression":        "lz4",
	"default_retention_daily":    "7",
	"default_retention_weekly":   "4",
	"default_retention_monthly":  "6",
	"daily_verification_enabled": "true",
	"daily_verification_cron":    "0 3 * * *",
	"storage_warning_interval":   "1",
	"backup_log_retention_days":  "0",
	"audit_log_retention_days":   "0",
	"concurrent_backup_limit":    "1",
}

func (s *Store) GetAllSettings(ctx context.Context) (map[string]string, error) {
	out := map[string]string{}
	for k, v := range DefaultSettings {
		out[k] = v
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT key, value FROM app_settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var k string
		var v sql.NullString
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		if v.Valid {
			out[k] = v.String
		}
	}
	return out, nil
}

func (s *Store) GetSetting(ctx context.Context, key string) string {
	settings, err := s.GetAllSettings(ctx)
	if err != nil {
		if def, ok := DefaultSettings[key]; ok {
			return def
		}
		return ""
	}
	if v, ok := settings[key]; ok {
		return v
	}
	return DefaultSettings[key]
}

func (s *Store) SetSetting(ctx context.Context, key, value string) error {
	_, err := s.DB.ExecContext(ctx, `INSERT INTO app_settings (key, value, updated_at) VALUES (?, ?, datetime('now'))
		ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=datetime('now')`, key, value)
	return err
}

// --- Repo checks ---

func (s *Store) CreateRepoCheck(ctx context.Context, c *RepoCheck) error {
	res, err := s.DB.ExecContext(ctx, `INSERT INTO repo_checks (destination_id, job_id, check_type, status, triggered_by) VALUES (?,?,?,?,?)`,
		c.DestinationID, c.JobID, c.CheckType, c.Status, c.TriggeredBy)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	c.ID = id
	return nil
}

func (s *Store) UpdateRepoCheck(ctx context.Context, c *RepoCheck) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE repo_checks SET status=?, finished_at=?, duration_seconds=?, log_output=? WHERE id=?`,
		c.Status, nullTime(c.FinishedAt), c.DurationSeconds, c.LogOutput, c.ID)
	return err
}

func (s *Store) ListRepoChecks(ctx context.Context, destID int64, jobID *int64) ([]RepoCheck, error) {
	q := `SELECT rc.id, rc.destination_id, rc.job_id, rc.check_type, rc.status, rc.started_at, rc.finished_at,
		rc.duration_seconds, rc.log_output, rc.triggered_by, d.name FROM repo_checks rc
		LEFT JOIN destinations d ON d.id = rc.destination_id WHERE rc.destination_id = ?`
	args := []any{destID}
	if jobID != nil {
		q += ` AND rc.job_id = ?`
		args = append(args, *jobID)
	}
	q += ` ORDER BY rc.started_at DESC LIMIT 100`
	rows, err := s.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RepoCheck
	for rows.Next() {
		var c RepoCheck
		var started, finished sql.NullTime
		if err := rows.Scan(&c.ID, &c.DestinationID, &c.JobID, &c.CheckType, &c.Status, &started, &finished,
			&c.DurationSeconds, &c.LogOutput, &c.TriggeredBy, &c.DestinationName); err != nil {
			return nil, err
		}
		if started.Valid {
			c.StartedAt = started.Time
		}
		c.FinishedAt = timePtr(finished)
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) CountEntities(ctx context.Context) (sources, destinations, jobs int, err error) {
	if err = s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM sources`).Scan(&sources); err != nil {
		return
	}
	if err = s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM destinations`).Scan(&destinations); err != nil {
		return
	}
	err = s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM backup_jobs`).Scan(&jobs)
	return
}

func (s *Store) SumLatestRunSizes(ctx context.Context) (int64, error) {
	var total sql.NullInt64
	err := s.DB.QueryRowContext(ctx, `
		SELECT SUM(size_bytes) FROM (
			SELECT job_id, MAX(size_bytes) as size_bytes FROM backup_runs
			WHERE status='success' AND size_bytes IS NOT NULL GROUP BY job_id
		)`).Scan(&total)
	if err != nil || !total.Valid {
		return 0, err
	}
	return total.Int64, nil
}

func (s *Store) LastSuccessfulRun(ctx context.Context) (*BackupRun, error) {
	row := s.DB.QueryRowContext(ctx, `SELECT id, job_id, started_at, finished_at, status, size_bytes, archive_name,
		archive_size_bytes, duration_seconds, log_output, acknowledged, is_host_key_error FROM backup_runs
		WHERE status='success' ORDER BY finished_at DESC LIMIT 1`)
	return s.scanRun(row)
}

func (s *Store) CountQueuedRunning(ctx context.Context) (queued, running int, err error) {
	err = s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM backup_runs WHERE status='queued'`).Scan(&queued)
	if err != nil {
		return
	}
	err = s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM backup_runs WHERE status='running'`).Scan(&running)
	return
}

func (s *Store) StorageHistory(ctx context.Context) (map[string][]map[string]any, error) {
	dests, err := s.ListDestinations(ctx)
	if err != nil {
		return nil, err
	}
	history := map[string][]map[string]any{}
	for _, d := range dests {
		rows, err := s.DB.QueryContext(ctx, `
			SELECT date(started_at) as d, MAX(size_bytes) as size FROM backup_runs r
			JOIN backup_jobs j ON r.job_id = j.id
			WHERE j.destination_id = ? AND r.status = 'success' AND r.size_bytes IS NOT NULL
			GROUP BY date(started_at) ORDER BY d ASC`, d.ID)
		if err != nil {
			continue
		}
		var points []map[string]any
		for rows.Next() {
			var dateStr string
			var size int64
			if err := rows.Scan(&dateStr, &size); err == nil {
				points = append(points, map[string]any{"date": dateStr, "size": size})
			}
		}
		rows.Close()
		history[d.Name] = points
	}
	return history, nil
}

func (s *Store) ExportConfig(ctx context.Context) (map[string]any, error) {
	sources, err := s.ListSources(ctx)
	if err != nil {
		return nil, err
	}
	dests, err := s.ListDestinations(ctx)
	if err != nil {
		return nil, err
	}
	jobs, err := s.ListJobs(ctx)
	if err != nil {
		return nil, err
	}
	channels, err := s.ListNotificationChannels(ctx)
	if err != nil {
		return nil, err
	}
	keys, err := s.ListSSHKeys(ctx)
	if err != nil {
		return nil, err
	}
	pubKeys := make([]map[string]any, 0, len(keys))
	for _, k := range keys {
		pubKeys = append(pubKeys, map[string]any{
			"name": k.Name, "public_key": k.PublicKey, "key_type": k.KeyType,
		})
	}
	return map[string]any{
		"version":                2,
		"exported_at":            time.Now().UTC().Format(time.RFC3339),
		"sources":                sources,
		"destinations":           dests,
		"jobs":                   jobs,
		"notification_channels":  channels,
		"ssh_keys_public":        pubKeys,
	}, nil
}

func (s *Store) ImportConfig(ctx context.Context, data map[string]any, user string) (map[string]int, error) {
	stats := map[string]int{"sources": 0, "destinations": 0, "jobs": 0, "channels": 0}
	nameToSourceID := map[string]int64{}
	nameToDestID := map[string]int64{}
	nameToChannelID := map[string]int64{}

	if raw, ok := data["sources"].([]any); ok {
		for _, item := range raw {
			m, _ := item.(map[string]any)
			src := mapToSource(m, user)
			if err := s.CreateSource(ctx, src); err != nil {
				return stats, fmt.Errorf("source %s: %w", src.Name, err)
			}
			nameToSourceID[src.Name] = src.ID
			stats["sources"]++
		}
	}
	if raw, ok := data["destinations"].([]any); ok {
		for _, item := range raw {
			m, _ := item.(map[string]any)
			d := mapToDestination(m, user)
			if err := s.CreateDestination(ctx, d); err != nil {
				return stats, fmt.Errorf("destination %s: %w", d.Name, err)
			}
			nameToDestID[d.Name] = d.ID
			stats["destinations"]++
		}
	}
	if raw, ok := data["notification_channels"].([]any); ok {
		for _, item := range raw {
			m, _ := item.(map[string]any)
			c := mapToChannel(m)
			if err := s.CreateNotificationChannel(ctx, c); err != nil {
				return stats, fmt.Errorf("channel %s: %w", c.Name, err)
			}
			nameToChannelID[c.Name] = c.ID
			stats["channels"]++
		}
	}
	if raw, ok := data["jobs"].([]any); ok {
		for _, item := range raw {
			m, _ := item.(map[string]any)
			j := mapToJob(m, nameToSourceID, nameToDestID, nameToChannelID, user)
			if err := s.CreateJob(ctx, j); err != nil {
				return stats, fmt.Errorf("job %s: %w", j.Name, err)
			}
			stats["jobs"]++
		}
	}
	return stats, nil
}

func strVal(m map[string]any, key string) *string {
	if v, ok := m[key]; ok && v != nil {
		s := fmt.Sprint(v)
		return &s
	}
	return nil
}

func intVal(m map[string]any, key string, def int) int {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case float64:
			return int(n)
		case int:
			return n
		}
	}
	return def
}

func boolVal(m map[string]any, key string, def bool) bool {
	if v, ok := m[key]; ok {
		switch b := v.(type) {
		case bool:
			return b
		case string:
			return b == "true" || b == "1"
		}
	}
	return def
}

func mapToSource(m map[string]any, user string) *Source {
	return &Source{
		Name: m["name"].(string), Type: fmt.Sprint(m["type"]),
		Path: strVal(m, "path"), SSHKeyPath: strVal(m, "ssh_key_path"),
		Excludes: strVal(m, "excludes"), S3Endpoint: strVal(m, "s3_endpoint"),
		S3Bucket: strVal(m, "s3_bucket"), S3Prefix: strVal(m, "s3_prefix"),
		S3AccessKeyEnv: strVal(m, "s3_access_key_env"), S3SecretKeyEnv: strVal(m, "s3_secret_key_env"),
		S3Region: strVal(m, "s3_region"), S3Provider: strVal(m, "s3_provider"),
		Notes: strVal(m, "notes"), CreatedBy: &user,
		SSHHostKeyChecking: boolVal(m, "ssh_host_key_checking", false),
	}
}

func mapToDestination(m map[string]any, user string) *Destination {
	return &Destination{
		Name: m["name"].(string), Type: fmt.Sprint(m["type"]),
		RepoURL: strVal(m, "repo_url"), SSHKeyPath: strVal(m, "ssh_key_path"),
		PassphraseEnv: strVal(m, "passphrase_env"), S3Endpoint: strVal(m, "s3_endpoint"),
		S3Bucket: strVal(m, "s3_bucket"), S3AccessKeyEnv: strVal(m, "s3_access_key_env"),
		S3SecretKeyEnv: strVal(m, "s3_secret_key_env"), S3Region: strVal(m, "s3_region"),
		S3Provider: strVal(m, "s3_provider"), Notes: strVal(m, "notes"), CreatedBy: &user,
		SSHHostKeyChecking: boolVal(m, "ssh_host_key_checking", false),
	}
}

func mapToChannel(m map[string]any) *NotificationChannel {
	cfg := map[string]string{}
	if raw, ok := m["config"].(map[string]any); ok {
		for k, v := range raw {
			cfg[k] = fmt.Sprint(v)
		}
	}
	return &NotificationChannel{
		Name: m["name"].(string), Type: fmt.Sprint(m["type"]),
		Enabled: boolVal(m, "enabled", true), OnSuccess: boolVal(m, "on_success", true),
		OnFailure: boolVal(m, "on_failure", true), OnStorageWarning: boolVal(m, "on_storage_warning", false),
		Config: cfg,
	}
}

func mapToJob(m map[string]any, srcMap, destMap, chMap map[string]int64, user string) *BackupJob {
	j := &BackupJob{
		Name: m["name"].(string),
		SourceID: srcMap[fmt.Sprint(m["source_name"])],
		DestinationID: destMap[fmt.Sprint(m["destination_name"])],
		Schedule: fmt.Sprint(m["schedule"]), Compression: fmt.Sprint(m["compression"]),
		RetentionDaily: intVal(m, "retention_daily", 7),
		RetentionWeekly: intVal(m, "retention_weekly", 4),
		RetentionMonthly: intVal(m, "retention_monthly", 6),
		Enabled: boolVal(m, "enabled", true),
		DailyVerificationEnabled: boolVal(m, "daily_verification_enabled", true),
		SSHBeforeCommand: strVal(m, "ssh_before_command"),
		SSHAfterCommand: strVal(m, "ssh_after_command"),
		RestoreBeforeCommand: strVal(m, "restore_before_command"),
		RestoreAfterCommand: strVal(m, "restore_after_command"),
		CreatedBy: &user,
	}
	if sid, ok := m["source_id"].(float64); ok && j.SourceID == 0 {
		j.SourceID = int64(sid)
	}
	if did, ok := m["destination_id"].(float64); ok && j.DestinationID == 0 {
		j.DestinationID = int64(did)
	}
	if ids, ok := m["notification_channel_ids"].([]any); ok {
		for _, id := range ids {
			if n, ok := id.(float64); ok {
				j.NotificationChannelIDs = append(j.NotificationChannelIDs, int64(n))
			}
		}
	}
	return j
}
