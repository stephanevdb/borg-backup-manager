package backup

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/borg-backup-manager/backend/internal/db"
)

type repoContext struct {
	repoPath string
	env      []string
	cleanups []*mountCleanup
	logFn    func(string)
}

func (rc *repoContext) close() {
	for i := len(rc.cleanups) - 1; i >= 0; i-- {
		rc.cleanups[i].unmount(rc.logFn)
	}
	rc.cleanups = nil
}

func (e *Engine) openDestinationRepo(ctx context.Context, dest *db.Destination, job *db.BackupJob, logFn func(string)) (*repoContext, error) {
	rc := &repoContext{env: e.buildBorgEnv(dest), logFn: logFn}
	if dest.Type == "s3" {
		repo, cleanup, err := e.mountS3Destination(dest, job, logFn)
		if err != nil {
			return nil, err
		}
		rc.repoPath = repo
		rc.cleanups = append(rc.cleanups, cleanup)
		return rc, nil
	}
	rc.repoPath = repoPathForJob(dest, job)
	if rc.repoPath == "" {
		return nil, fmt.Errorf("repository path not configured")
	}
	return rc, nil
}

func (e *Engine) openSourcePath(ctx context.Context, source *db.Source, logFn func(string)) (path string, cleanups []*mountCleanup, err error) {
	switch source.Type {
	case "local":
		if source.Path == nil || *source.Path == "" {
			return "", nil, fmt.Errorf("no source path configured")
		}
		return *source.Path, nil, nil
	case "ssh":
		p, c, err := e.mountSSHSource(source, logFn)
		if c != nil {
			return p, []*mountCleanup{c}, err
		}
		return p, nil, err
	case "s3":
		p, c, err := e.mountS3Source(source, logFn)
		if c != nil {
			return p, []*mountCleanup{c}, err
		}
		return p, nil, err
	default:
		if source.Path != nil && *source.Path != "" {
			return *source.Path, nil, nil
		}
		return "", nil, fmt.Errorf("no source path configured")
	}
}

func runBorgCmd(ctx context.Context, args []string, env []string, cwd string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "borg", args...)
	cmd.Env = env
	if cwd != "" {
		cmd.Dir = cwd
	}
	return cmd
}

func parseArchiveListLine(line string) map[string]any {
	parts := strings.Split(line, "\t")
	name := parts[0]
	result := map[string]any{"name": name, "time": "", "size_bytes": int64(0)}
	if len(parts) > 1 {
		result["time"] = parts[1]
	}
	if len(parts) > 2 {
		var size int64
		_, _ = fmt.Sscan(parts[2], &size)
		result["size_bytes"] = size
	}
	return result
}

var errNoRepoPath = os.ErrInvalid
