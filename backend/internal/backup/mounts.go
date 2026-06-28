package backup

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/borg-backup-manager/backend/internal/db"
)

type mountCleanup struct {
	mountDir string
	confFile string
}

func (m *mountCleanup) unmount(logFn func(string)) {
	if m.mountDir == "" {
		return
	}
	if logFn != nil {
		logFn("Flushing VFS cache to S3...")
	}
	_ = exec.Command("sync").Run()
	time.Sleep(5 * time.Second)
	_ = exec.Command("fusermount3", "-u", m.mountDir).Run()
	if fi, err := os.Stat(m.mountDir); err == nil && fi.IsDir() {
		_ = os.Remove(m.mountDir)
	}
	if m.confFile != "" {
		_ = os.Remove(m.confFile)
	}
	if logFn != nil {
		logFn("Unmounted remote filesystem")
	}
}

func (e *Engine) mountS3Source(source *db.Source, logFn func(string)) (borgPath string, cleanup *mountCleanup, err error) {
	bucket := strings.Trim(strings.TrimSpace(strPtr(source.S3Bucket)), "/")
	if bucket == "" {
		return "", nil, fmt.Errorf("S3 source missing bucket")
	}
	mountDir, confFile, err := e.mountRcloneSource(source, bucket, true, "S3 source", logFn)
	if err != nil {
		return "", nil, err
	}
	cleanup = &mountCleanup{mountDir: mountDir, confFile: confFile}
	prefix := strings.Trim(strings.TrimSpace(strPtr(source.S3Prefix)), "/")
	if prefix != "" {
		full := filepath.Join(mountDir, prefix)
		if _, err := os.Stat(full); err != nil {
			return "", cleanup, fmt.Errorf("S3 prefix %q not found in bucket %q", prefix, bucket)
		}
		return full, cleanup, nil
	}
	return mountDir, cleanup, nil
}

func parseS3RepoLocation(dest *db.Destination) (bucket, path string, err error) {
	repo := resolveRepoBaseURL(dest)
	if !strings.HasPrefix(repo, "s3://") {
		return "", "", fmt.Errorf("S3 destination has no s3:// repository URL")
	}
	remainder := strings.TrimPrefix(repo, "s3://")
	parts := strings.SplitN(remainder, "/", 2)
	bucket = strings.TrimSpace(parts[0])
	if len(parts) > 1 {
		path = strings.Trim(strings.TrimSpace(parts[1]), "/")
	}
	if bucket == "" {
		return "", "", fmt.Errorf("S3 bucket missing")
	}
	return bucket, path, nil
}

func (e *Engine) mountS3Destination(dest *db.Destination, job *db.BackupJob, logFn func(string)) (repoPath string, cleanup *mountCleanup, err error) {
	bucket, path, err := parseS3RepoLocation(dest)
	if err != nil {
		return "", nil, err
	}
	mountDir, confFile, err := e.mountRcloneDest(dest, bucket, false, "S3 destination", logFn)
	if err != nil {
		return "", nil, err
	}
	cleanup = &mountCleanup{mountDir: mountDir, confFile: confFile}
	repoBase := mountDir
	if path != "" {
		repoBase = filepath.Join(mountDir, path)
	}
	_ = os.MkdirAll(repoBase, 0o755)
	if job != nil {
		repoPath = filepath.Join(repoBase, sanitizeJobName(job.Name))
	} else {
		repoPath = repoBase
	}
	_ = os.MkdirAll(repoPath, 0o755)
	return repoPath, cleanup, nil
}

func (e *Engine) mountSSHSource(source *db.Source, logFn func(string)) (borgPath string, cleanup *mountCleanup, err error) {
	if source.Path == nil || !strings.HasPrefix(*source.Path, "ssh://") {
		return "", nil, fmt.Errorf("SSH source path must start with ssh://")
	}
	parts := strings.SplitN((*source.Path)[6:], "/", 2)
	remoteHost := parts[0]
	remoteDir := "/"
	if len(parts) > 1 {
		remoteDir = "/" + parts[1]
	}
	keyFile, err := e.resolveSSHKeyFile(context.Background(), strPtr(source.SSHKeyPath))
	if err != nil {
		return "", nil, err
	}
	mountDir, err := os.MkdirTemp("", "bbm-sshfs-")
	if err != nil {
		return "", nil, err
	}
	cleanup = &mountCleanup{mountDir: mountDir}

	strict := "accept-new"
	if source.SSHHostKeyChecking {
		strict = "yes"
	}
	knownHosts := filepath.Join(e.cfg.DataDir, "ssh", "known_hosts")
	sshCmd := fmt.Sprintf("ssh -oBatchMode=yes -oUserKnownHostsFile=%s", knownHosts)
	if fam := sshAddressFamily(e.store, context.Background()); fam != "" {
		sshCmd += " -oAddressFamily=" + fam
	}
	args := []string{
		remoteHost + ":/", mountDir,
		"-o", "ssh_command=" + sshCmd,
		"-o", "IdentityFile=" + keyFile,
		"-o", "StrictHostKeyChecking=" + strict,
	}
	if logFn != nil {
		logFn("Mounting SSH source via sshfs...")
	}
	cmd := exec.Command("sshfs", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		cleanup.unmount(nil)
		return "", nil, fmt.Errorf("sshfs mount failed: %s", strings.TrimSpace(string(out)))
	}
	localPath := filepath.Join(mountDir, strings.TrimPrefix(remoteDir, "/"))
	if logFn != nil {
		logFn("SSH source mounted successfully")
	}
	return localPath, cleanup, nil
}

func (e *Engine) runSSHCommand(ctx context.Context, source *db.Source, command string, logFn func(string)) error {
	if command == "" {
		return nil
	}
	if source.Path == nil || !strings.HasPrefix(*source.Path, "ssh://") {
		return fmt.Errorf("source path is not SSH")
	}
	parts := strings.SplitN((*source.Path)[6:], "/", 2)
	remoteHost := parts[0]
	keyFile, err := e.resolveSSHKeyFile(ctx, strPtr(source.SSHKeyPath))
	if err != nil {
		return err
	}
	strict := "accept-new"
	if source.SSHHostKeyChecking {
		strict = "yes"
	}
	knownHosts := filepath.Join(e.cfg.DataDir, "ssh", "known_hosts")
	args := []string{
		"-i", keyFile,
		"-o", "StrictHostKeyChecking=" + strict,
		"-o", "UserKnownHostsFile=" + knownHosts,
		"-o", "ConnectTimeout=10",
		remoteHost, command,
	}
	if logFn != nil {
		logFn("SSH: " + command)
	}
	cmd := exec.CommandContext(ctx, "ssh", args...)
	out, err := cmd.CombinedOutput()
	if logFn != nil && len(out) > 0 {
		logFn(string(out))
	}
	if err != nil {
		return fmt.Errorf("ssh command failed: %w", err)
	}
	return nil
}
