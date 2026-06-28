package backup

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/borg-backup-manager/backend/internal/db"
)

const storageWarningPercent = 90.0

type StorageResult struct {
	AvailableBytes *int64   `json:"available_bytes"`
	TotalBytes     *int64   `json:"total_bytes"`
	UsedBytes      *int64   `json:"used_bytes"`
	PercentUsed    *float64 `json:"percent_used"`
	Status         string   `json:"status"`
	Error          *string  `json:"error"`
	IsWarning      bool     `json:"is_warning"`
}

func (e *Engine) DestinationStorage(ctx context.Context, dest *db.Destination) StorageResult {
	switch dest.Type {
	case "local":
		return e.localStorage(strPtr(dest.RepoURL))
	case "ssh":
		return e.sshStorage(ctx, dest)
	default:
		msg := "Storage stats unavailable for this destination type"
		return StorageResult{Status: "unavailable", Error: &msg}
	}
}

func (e *Engine) localStorage(repoPath string) StorageResult {
	if repoPath == "" {
		msg := "No repository path configured"
		return StorageResult{Status: "unavailable", Error: &msg}
	}
	probe := repoPath
	for probe != "" && probe != "/" {
		if _, err := os.Stat(probe); err == nil {
			break
		}
		probe = filepath.Dir(probe)
	}
	usage, err := diskUsage(probe)
	if err != nil {
		msg := err.Error()
		return StorageResult{Status: "unavailable", Error: &msg}
	}
	pct := float64(usage.used) / float64(usage.total) * 100
	if usage.total == 0 {
		pct = 0
	}
	avail := usage.free
	total := usage.total
	used := usage.used
	return StorageResult{
		AvailableBytes: &avail,
		TotalBytes:     &total,
		UsedBytes:      &used,
		PercentUsed:    &pct,
		Status:         "ok",
		IsWarning:      pct >= storageWarningPercent,
	}
}

type du struct{ total, used, free int64 }

func diskUsage(path string) (du, error) {
	cmd := exec.Command("df", "-k", path)
	out, err := cmd.Output()
	if err != nil {
		return du{}, err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		return du{}, os.ErrInvalid
	}
	fields := strings.Fields(lines[len(lines)-1])
	if len(fields) < 4 {
		return du{}, os.ErrInvalid
	}
	var total, used, avail int64
	_, _ = fmt.Sscan(fields[1], &total)
	_, _ = fmt.Sscan(fields[2], &used)
	_, _ = fmt.Sscan(fields[3], &avail)
	total *= 1024
	used *= 1024
	avail *= 1024
	return du{total: total, used: used, free: avail}, nil
}

func (e *Engine) sshStorage(ctx context.Context, dest *db.Destination) StorageResult {
	repo := strPtr(dest.RepoURL)
	if !strings.HasPrefix(repo, "ssh://") {
		msg := "Invalid SSH repository URL"
		return StorageResult{Status: "unavailable", Error: &msg}
	}
	parts := strings.SplitN(repo[6:], "/", 2)
	host := parts[0]
	remotePath := "/"
	if len(parts) > 1 {
		remotePath = "/" + parts[1]
	}
	keyFile, err := e.resolveSSHKeyFile(ctx, strPtr(dest.SSHKeyPath))
	if err != nil {
		msg := err.Error()
		return StorageResult{Status: "unavailable", Error: &msg}
	}
	knownHosts := filepath.Join(e.cfg.DataDir, "ssh", "known_hosts")
	cmd := exec.CommandContext(ctx, "ssh", "-i", keyFile, "-o", "BatchMode=yes", "-o", "UserKnownHostsFile="+knownHosts, host,
		"df -k "+remotePath+" | tail -1")
	out, err := cmd.Output()
	if err != nil {
		msg := "Failed to query remote disk usage"
		return StorageResult{Status: "unavailable", Error: &msg}
	}
	fields := strings.Fields(strings.TrimSpace(string(out)))
	if len(fields) < 4 {
		msg := "Unexpected df output"
		return StorageResult{Status: "unavailable", Error: &msg}
	}
	var total, used, avail int64
	_, _ = fmt.Sscan(fields[1], &total)
	_, _ = fmt.Sscan(fields[2], &used)
	_, _ = fmt.Sscan(fields[3], &avail)
	total *= 1024
	used *= 1024
	avail *= 1024
	pct := float64(used) / float64(total) * 100
	if total == 0 {
		pct = 0
	}
	return StorageResult{
		AvailableBytes: &avail,
		TotalBytes:     &total,
		UsedBytes:      &used,
		PercentUsed:    &pct,
		Status:         "ok",
		IsWarning:      pct >= storageWarningPercent,
	}
}
