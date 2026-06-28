package backup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/borg-backup-manager/backend/internal/db"
)

func (e *Engine) mountRcloneSource(source *db.Source, bucket string, readOnly bool, label string, logFn func(string)) (mountDir, confFile string, err error) {
	return e.mountRcloneFields(source.S3Endpoint, source.S3Region, source.S3AccessKeyEncrypted, source.S3SecretKeyEncrypted, source.S3AccessKeyEnv, source.S3SecretKeyEnv, bucket, readOnly, label, logFn)
}

func (e *Engine) mountRcloneDest(dest *db.Destination, bucket string, readOnly bool, label string, logFn func(string)) (mountDir, confFile string, err error) {
	return e.mountRcloneFields(dest.S3Endpoint, dest.S3Region, dest.S3AccessKeyEncrypted, dest.S3SecretKeyEncrypted, dest.S3AccessKeyEnv, dest.S3SecretKeyEnv, bucket, readOnly, label, logFn)
}

func (e *Engine) mountRcloneFields(endpoint, region, accessEnc, secretEnc, accessEnv, secretEnv *string, bucket string, readOnly bool, label string, logFn func(string)) (mountDir, confFile string, err error) {
	access, secret := e.resolveS3CredentialFields(accessEnc, secretEnc, accessEnv, secretEnv)
	if access == "" || secret == "" {
		return "", "", fmt.Errorf("missing S3 credentials")
	}
	mountDir, err = os.MkdirTemp("", "bbm-rclone-")
	if err != nil {
		return "", "", err
	}
	conf, err := os.CreateTemp("", "bbm-rclone-conf-")
	if err != nil {
		_ = os.Remove(mountDir)
		return "", "", err
	}
	confFile = conf.Name()
	var sb strings.Builder
	sb.WriteString("[s3]\n")
	sb.WriteString("type = s3\n")
	sb.WriteString("provider = Other\n")
	sb.WriteString("access_key_id = " + access + "\n")
	sb.WriteString("secret_access_key = " + secret + "\n")
	if ep := strPtr(endpoint); ep != "" {
		sb.WriteString("endpoint = " + ep + "\n")
	}
	if reg := strPtr(region); reg != "" {
		sb.WriteString("region = " + reg + "\n")
	}
	if _, err := conf.WriteString(sb.String()); err != nil {
		conf.Close()
		return "", "", err
	}
	conf.Close()

	cacheDir := filepath.Join(e.cfg.DataDir, "rclone-cache")
	_ = os.MkdirAll(cacheDir, 0o755)

	if logFn != nil {
		logFn("Mounting " + label + " s3://" + bucket + " (rclone)...")
	}
	args := []string{
		"mount", "s3:" + bucket, mountDir,
		"--config", confFile,
		"--vfs-cache-mode", "full",
		"--vfs-write-back", "0",
		"--cache-dir", cacheDir,
		"--vfs-cache-max-size", "20G",
		"--daemon",
		"--allow-other",
	}
	if readOnly {
		args = append(args, "--read-only")
	}
	cmd := exec.Command("rclone", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		_ = os.Remove(mountDir)
		_ = os.Remove(confFile)
		return "", "", fmt.Errorf("rclone mount failed: %s", strings.TrimSpace(string(out)))
	}
	if logFn != nil {
		logFn(label + " mounted successfully via rclone")
	}
	return mountDir, confFile, nil
}
