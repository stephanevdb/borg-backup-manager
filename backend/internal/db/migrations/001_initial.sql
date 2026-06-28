-- +goose Up
CREATE TABLE IF NOT EXISTS sources (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    type TEXT NOT NULL DEFAULT 'local',
    path TEXT,
    ssh_key_path TEXT,
    excludes TEXT,
    s3_endpoint TEXT,
    s3_bucket TEXT,
    s3_prefix TEXT,
    s3_access_key_env TEXT,
    s3_secret_key_env TEXT,
    s3_access_key_encrypted TEXT,
    s3_secret_key_encrypted TEXT,
    s3_region TEXT,
    s3_provider TEXT,
    notes TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    created_by TEXT,
    ssh_host_key_checking INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS destinations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    type TEXT NOT NULL DEFAULT 'local',
    repo_url TEXT,
    ssh_key_path TEXT,
    passphrase_env TEXT,
    s3_endpoint TEXT,
    s3_bucket TEXT,
    s3_access_key_env TEXT,
    s3_secret_key_env TEXT,
    s3_access_key_encrypted TEXT,
    s3_secret_key_encrypted TEXT,
    s3_region TEXT,
    s3_provider TEXT,
    notes TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    created_by TEXT,
    ssh_host_key_checking INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS notification_channels (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    enabled INTEGER DEFAULT 1,
    on_success INTEGER DEFAULT 1,
    on_failure INTEGER DEFAULT 1,
    on_storage_warning INTEGER DEFAULT 0,
    config TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS backup_jobs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    source_id INTEGER NOT NULL REFERENCES sources(id),
    destination_id INTEGER NOT NULL REFERENCES destinations(id),
    schedule TEXT DEFAULT 'manual',
    compression TEXT DEFAULT 'lz4',
    retention_daily INTEGER DEFAULT 7,
    retention_weekly INTEGER DEFAULT 4,
    retention_monthly INTEGER DEFAULT 6,
    enabled INTEGER DEFAULT 1,
    daily_verification_enabled INTEGER DEFAULT 1,
    ssh_before_command TEXT,
    ssh_after_command TEXT,
    restore_before_command TEXT,
    restore_after_command TEXT,
    last_run_at DATETIME,
    last_status TEXT DEFAULT 'never',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    created_by TEXT
);

CREATE TABLE IF NOT EXISTS job_notification_channels (
    job_id INTEGER NOT NULL REFERENCES backup_jobs(id) ON DELETE CASCADE,
    channel_id INTEGER NOT NULL REFERENCES notification_channels(id) ON DELETE CASCADE,
    PRIMARY KEY (job_id, channel_id)
);

CREATE TABLE IF NOT EXISTS backup_runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id INTEGER NOT NULL REFERENCES backup_jobs(id),
    started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    finished_at DATETIME,
    status TEXT NOT NULL DEFAULT 'running',
    size_bytes INTEGER,
    archive_name TEXT,
    archive_size_bytes INTEGER,
    duration_seconds INTEGER,
    log_output TEXT,
    acknowledged INTEGER DEFAULT 0,
    is_host_key_error INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS ssh_keys (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    private_key TEXT NOT NULL,
    public_key TEXT NOT NULL,
    key_type TEXT NOT NULL DEFAULT 'ed25519',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    created_by TEXT
);

CREATE TABLE IF NOT EXISTS repo_checks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    destination_id INTEGER NOT NULL REFERENCES destinations(id),
    job_id INTEGER,
    check_type TEXT NOT NULL DEFAULT 'full',
    status TEXT NOT NULL DEFAULT 'running',
    started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    finished_at DATETIME,
    duration_seconds INTEGER,
    log_output TEXT,
    triggered_by TEXT
);

CREATE TABLE IF NOT EXISTS audit_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    user TEXT,
    action TEXT NOT NULL,
    target_type TEXT NOT NULL,
    target_id INTEGER,
    target_name TEXT,
    details TEXT,
    ip_address TEXT
);

CREATE INDEX IF NOT EXISTS idx_audit_logs_timestamp ON audit_logs(timestamp);

CREATE TABLE IF NOT EXISTS api_keys (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    key_hash TEXT NOT NULL UNIQUE,
    prefix TEXT NOT NULL,
    user TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME,
    last_used_at DATETIME
);

CREATE TABLE IF NOT EXISTS app_settings (
    key TEXT PRIMARY KEY,
    value TEXT,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- +goose Down
DROP TABLE IF EXISTS job_notification_channels;
DROP TABLE IF EXISTS backup_runs;
DROP TABLE IF EXISTS backup_jobs;
DROP TABLE IF EXISTS repo_checks;
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS app_settings;
DROP TABLE IF EXISTS ssh_keys;
DROP TABLE IF EXISTS notification_channels;
DROP TABLE IF EXISTS destinations;
DROP TABLE IF EXISTS sources;
