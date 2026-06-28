package backup

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/borg-backup-manager/backend/internal/config"
	"github.com/borg-backup-manager/backend/internal/crypto"
	"github.com/borg-backup-manager/backend/internal/db"
	"github.com/borg-backup-manager/backend/internal/notify"
	"github.com/borg-backup-manager/backend/internal/stream"
)

type Engine struct {
	cfg        *config.Config
	store      *db.Store
	encryptor  *crypto.Encryptor
	hub        *stream.Hub
	notifier   *notify.Service
	mu         sync.Mutex
	running    map[int64]struct{}
	cancelled  map[int64]struct{}
	progress   map[int64]Progress
	processes  map[int64]*exec.Cmd
	restores   map[string]*RestoreState
	restoreMu  sync.RWMutex
}

type Progress struct {
	Percent   *float64 `json:"percent"`
	Message   string   `json:"message"`
	StartTime float64  `json:"start_time"`
	EstSize   *int64   `json:"est_size,omitempty"`
}

type RestoreState struct {
	ID       string
	Status   string
	Logs     string
	Progress *float64
	Done     bool
}

func NewEngine(cfg *config.Config, store *db.Store, enc *crypto.Encryptor, hub *stream.Hub, notifier *notify.Service) *Engine {
	return &Engine{
		cfg: cfg, store: store, encryptor: enc, hub: hub, notifier: notifier,
		running: map[int64]struct{}{}, cancelled: map[int64]struct{}{},
		progress: map[int64]Progress{}, processes: map[int64]*exec.Cmd{},
		restores: map[string]*RestoreState{},
	}
}

func (e *Engine) StartWorker(ctx context.Context) {
	if n, err := e.store.RecoverInterruptedRuns(ctx); err == nil && n > 0 {
		log.Printf("recovered %d interrupted runs", n)
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			run, err := e.store.GetNextQueuedRun(ctx)
			if err != nil || run == nil {
				time.Sleep(3 * time.Second)
				continue
			}
			e.mu.Lock()
			if _, ok := e.running[run.JobID]; ok {
				e.mu.Unlock()
				time.Sleep(3 * time.Second)
				continue
			}
			limit := 1
			if v := e.store.GetSetting(ctx, "concurrent_backup_limit"); v != "" {
				if n, err := strconv.Atoi(v); err == nil && n > 0 {
					limit = n
				}
			}
			if len(e.running) >= limit {
				e.mu.Unlock()
				time.Sleep(3 * time.Second)
				continue
			}
			e.running[run.JobID] = struct{}{}
			e.mu.Unlock()

			if e.isCancelled(run.JobID) {
				e.finishCancelled(ctx, run)
				continue
			}
			e.doBackup(ctx, run.JobID, run.ID)
			e.mu.Lock()
			delete(e.running, run.JobID)
			delete(e.cancelled, run.JobID)
			e.mu.Unlock()
			time.Sleep(time.Second)
		}
	}()
}

func (e *Engine) isCancelled(jobID int64) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	_, ok := e.cancelled[jobID]
	return ok
}

func (e *Engine) finishCancelled(ctx context.Context, run *db.BackupRun) {
	now := time.Now().UTC()
	run.Status = "cancelled"
	run.FinishedAt = &now
	msg := "Job was cancelled from queue."
	run.LogOutput = &msg
	_ = e.store.UpdateRun(ctx, run)
	e.mu.Lock()
	delete(e.running, run.JobID)
	delete(e.cancelled, run.JobID)
	e.mu.Unlock()
}

func (e *Engine) QueueJob(ctx context.Context, jobID int64) (int64, error) {
	if active, _ := e.store.ActiveRunForJob(ctx, jobID); active != nil {
		return 0, fmt.Errorf("job already queued or running")
	}
	run, err := e.store.CreateRun(ctx, jobID)
	if err != nil {
		return 0, err
	}
	return run.ID, nil
}

func (e *Engine) CancelJob(ctx context.Context, jobID int64) (bool, string) {
	run, err := e.store.ActiveRunForJob(ctx, jobID)
	if err != nil {
		return false, "job not running or queued"
	}
	now := time.Now().UTC()
	if run.Status == "queued" {
		e.mu.Lock()
		e.cancelled[jobID] = struct{}{}
		e.mu.Unlock()
		run.Status = "cancelled"
		run.FinishedAt = &now
		msg := "Job was cancelled while queued."
		run.LogOutput = &msg
		_ = e.store.UpdateRun(ctx, run)
		return true, "queued job cancelled"
	}
	e.mu.Lock()
	e.cancelled[jobID] = struct{}{}
	if proc, ok := e.processes[jobID]; ok {
		_ = proc.Process.Kill()
	}
	e.mu.Unlock()
	return true, "termination signal sent"
}

func (e *Engine) GetProgress(jobID int64) Progress {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.progress[jobID]
}

func (e *Engine) broadcastProgress(jobID int64) {
	p := e.GetProgress(jobID)
	e.hub.Publish("dashboard", "progress", map[string]any{"job_id": jobID, "progress": p})
	e.hub.Publish(fmt.Sprintf("run:%d", jobID), "progress", p)
}

func (e *Engine) broadcastLog(runID int64, line string) {
	e.hub.Publish(fmt.Sprintf("runlog:%d", runID), "log", map[string]string{"line": line})
}

func sanitizeJobName(name string) string {
	re := regexp.MustCompile(`[^a-zA-Z0-9_-]+`)
	return re.ReplaceAllString(name, "_")
}

func (e *Engine) repoPath(dest *db.Destination, job *db.BackupJob) string {
	return repoPathForJob(dest, job)
}

func (e *Engine) borgEnv(dest *db.Destination) []string {
	return e.buildBorgEnv(dest)
}

func (e *Engine) doBackup(ctx context.Context, jobID, runID int64) {
	job, err := e.store.GetJob(ctx, jobID)
	if err != nil {
		return
	}
	run, err := e.store.GetRun(ctx, runID)
	if err != nil {
		return
	}
	source, _ := e.store.GetSource(ctx, job.SourceID)
	dest, _ := e.store.GetDestination(ctx, job.DestinationID)
	if source == nil || dest == nil {
		return
	}

	run.Status = "running"
	_ = e.store.UpdateRun(ctx, run)
	_, _ = e.store.DB.ExecContext(ctx, `UPDATE backup_jobs SET last_status='running' WHERE id=?`, jobID)

	e.mu.Lock()
	e.progress[jobID] = Progress{Message: "Starting backup...", StartTime: float64(time.Now().Unix())}
	e.mu.Unlock()
	e.broadcastProgress(jobID)

	logBuf := &strings.Builder{}
	start := time.Now().UTC()
	appendLog := func(line string) {
		logBuf.WriteString(line)
		logBuf.WriteByte('\n')
		e.broadcastLog(runID, line)
	}

	defer func() {
		fin := time.Now().UTC()
		run.FinishedAt = &fin
		dur := int(fin.Sub(start).Seconds())
		run.DurationSeconds = &dur
		out := logBuf.String()
		run.LogOutput = &out
		_ = e.store.UpdateRun(ctx, run)
		_, _ = e.store.DB.ExecContext(ctx, `UPDATE backup_jobs SET last_status=?, last_run_at=? WHERE id=?`, run.Status, fin, jobID)
		e.notifier.NotifyRun(ctx, job, run)
		e.hub.Publish("dashboard", "run_complete", map[string]any{"job_id": jobID, "run_id": runID})
	}()

	if source.Type == "ssh" && job.SSHBeforeCommand != nil {
		if err := e.runSSHCommand(ctx, source, *job.SSHBeforeCommand, appendLog); err != nil {
			run.Status = "failed"
			appendLog("ERROR: " + err.Error())
			return
		}
	}

	srcPath, srcCleanups, err := e.openSourcePath(ctx, source, appendLog)
	if err != nil {
		run.Status = "failed"
		appendLog("ERROR: " + err.Error())
		return
	}
	defer func() {
		for i := len(srcCleanups) - 1; i >= 0; i-- {
			srcCleanups[i].unmount(appendLog)
		}
	}()

	rc, err := e.openDestinationRepo(ctx, dest, job, appendLog)
	if err != nil {
		run.Status = "failed"
		appendLog("ERROR: " + err.Error())
		return
	}
	defer rc.close()

	repo := rc.repoPath
	if err := e.initRepoAt(ctx, repo, rc.env, appendLog); err != nil {
		run.Status = "failed"
		appendLog("ERROR: " + err.Error())
		return
	}

	archiveName := fmt.Sprintf("%s-%s", sanitizeJobName(job.Name), start.UTC().Format("20060102T150405"))
	args := []string{"create", "--stats", "--log-json", "--lock-wait", "600",
		fmt.Sprintf("%s::%s", repo, archiveName), srcPath}
	if source.Excludes != nil && *source.Excludes != "" {
		for _, ex := range strings.Split(*source.Excludes, "\n") {
			ex = strings.TrimSpace(ex)
			if ex != "" {
				args = append(args, "--exclude", ex)
			}
		}
	}
	if job.Compression != "" && job.Compression != "none" {
		args = []string{"create", "--compression", job.Compression, "--stats", "--log-json", "--lock-wait", "600",
			fmt.Sprintf("%s::%s", repo, archiveName), srcPath}
		if source.Excludes != nil && *source.Excludes != "" {
			for _, ex := range strings.Split(*source.Excludes, "\n") {
				ex = strings.TrimSpace(ex)
				if ex != "" {
					args = append(args, "--exclude", ex)
				}
			}
		}
	}

	cmd := runBorgCmd(ctx, args, rc.env, borgRunDir(repo))
	stderr, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		run.Status = "failed"
		appendLog("ERROR: " + err.Error())
		return
	}
	e.mu.Lock()
	e.processes[jobID] = cmd
	e.mu.Unlock()
	defer func() {
		e.mu.Lock()
		delete(e.processes, jobID)
		e.mu.Unlock()
	}()

	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		line := scanner.Text()
		appendLog(line)
		e.parseBorgJSON(jobID, line)
	}
	_ = cmd.Wait()

	if source.Type == "ssh" && job.SSHAfterCommand != nil {
		_ = e.runSSHCommand(ctx, source, *job.SSHAfterCommand, appendLog)
	}

	if e.isCancelled(jobID) {
		run.Status = "cancelled"
		return
	}
	if cmd.ProcessState != nil && !cmd.ProcessState.Success() {
		run.Status = "failed"
		return
	}

	pruneArgs := []string{"prune", "--list", repo,
		"--glob-archives", sanitizeJobName(job.Name) + "-*",
		"--keep-within", "24H",
		"--keep-daily", fmt.Sprint(job.RetentionDaily),
		"--keep-weekly", fmt.Sprint(job.RetentionWeekly),
		"--keep-monthly", fmt.Sprint(job.RetentionMonthly),
	}
	prune := runBorgCmd(ctx, pruneArgs, rc.env, borgRunDir(repo))
	out, err := prune.CombinedOutput()
	appendLog(string(out))
	if err != nil {
		appendLog("WARN: prune failed: " + err.Error())
	}

	run.ArchiveName = &archiveName
	run.Status = "success"
}

func (e *Engine) initRepoAt(ctx context.Context, repo string, env []string, logFn func(string)) error {
	info := runBorgCmd(ctx, []string{"info", "--lock-wait", "600", repo}, env, borgRunDir(repo))
	if err := info.Run(); err == nil {
		return nil
	}
	logFn("Initializing repository...")
	init := runBorgCmd(ctx, []string{"init", "--encryption=none", "--lock-wait", "600", repo}, env, borgRunDir(repo))
	out, err := init.CombinedOutput()
	logFn(string(out))
	return err
}

func (e *Engine) initRepo(repo string, dest *db.Destination, logFn func(string)) error {
	return e.initRepoAt(context.Background(), repo, e.buildBorgEnv(dest), logFn)
}

func (e *Engine) parseBorgJSON(jobID int64, line string) {
	var evt map[string]any
	if err := json.Unmarshal([]byte(line), &evt); err != nil {
		return
	}
	if t, _ := evt["type"].(string); t == "progress" {
		if p, ok := evt["progress"].(float64); ok {
			e.mu.Lock()
			pr := e.progress[jobID]
			pr.Percent = &p
			if m, ok := evt["message"].(string); ok {
				pr.Message = m
			}
			e.progress[jobID] = pr
			e.mu.Unlock()
			e.broadcastProgress(jobID)
		}
	}
}

func (e *Engine) withRepo(ctx context.Context, destID, jobID int64) (*repoContext, *db.BackupJob, error) {
	job, err := e.store.GetJob(ctx, jobID)
	if err != nil {
		return nil, nil, err
	}
	dest, err := e.store.GetDestination(ctx, destID)
	if err != nil {
		return nil, nil, err
	}
	rc, err := e.openDestinationRepo(ctx, dest, job, nil)
	return rc, job, err
}

func (e *Engine) ListArchives(ctx context.Context, destID, jobID int64) ([]map[string]any, error) {
	rc, _, err := e.withRepo(ctx, destID, jobID)
	if err != nil {
		return nil, err
	}
	defer rc.close()
	cmd := runBorgCmd(ctx, []string{"list", "--format", "{archive}{TAB}{time}{TAB}{size}", rc.repoPath}, rc.env, borgRunDir(rc.repoPath))
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var archives []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		archives = append(archives, parseArchiveListLine(line))
	}
	return archives, nil
}

func (e *Engine) BreakLock(ctx context.Context, destID, jobID int64) error {
	rc, _, err := e.withRepo(ctx, destID, jobID)
	if err != nil {
		return err
	}
	defer rc.close()
	return runBorgCmd(ctx, []string{"break-lock", rc.repoPath}, rc.env, borgRunDir(rc.repoPath)).Run()
}

func (e *Engine) Compact(ctx context.Context, destID, jobID int64) error {
	rc, _, err := e.withRepo(ctx, destID, jobID)
	if err != nil {
		return err
	}
	defer rc.close()
	return runBorgCmd(ctx, []string{"compact", rc.repoPath}, rc.env, borgRunDir(rc.repoPath)).Run()
}

func (e *Engine) Verify(ctx context.Context, destID int64, jobID *int64, user string) (*db.RepoCheck, error) {
	var job *db.BackupJob
	var err error
	if jobID != nil {
		job, err = e.store.GetJob(ctx, *jobID)
		if err != nil {
			return nil, err
		}
	} else {
		jobs, _ := e.store.ListJobs(ctx)
		for _, j := range jobs {
			if j.DestinationID == destID {
				job = &j
				break
			}
		}
	}
	if job == nil {
		return nil, fmt.Errorf("no job for destination")
	}
	dest, err := e.store.GetDestination(ctx, destID)
	if err != nil {
		return nil, err
	}
	check := &db.RepoCheck{DestinationID: destID, JobID: jobID, CheckType: "full", Status: "running", TriggeredBy: &user}
	_ = e.store.CreateRepoCheck(ctx, check)
	rc, err := e.openDestinationRepo(ctx, dest, job, nil)
	if err != nil {
		check.Status = "failed"
		msg := err.Error()
		check.LogOutput = &msg
		_ = e.store.UpdateRepoCheck(ctx, check)
		return check, err
	}
	defer rc.close()
	cmd := runBorgCmd(ctx, []string{"check", rc.repoPath}, rc.env, borgRunDir(rc.repoPath))
	out, err := cmd.CombinedOutput()
	now := time.Now().UTC()
	check.FinishedAt = &now
	log := string(out)
	check.LogOutput = &log
	dur := int(now.Sub(check.StartedAt).Seconds())
	check.DurationSeconds = &dur
	if err != nil {
		check.Status = "failed"
	} else {
		check.Status = "success"
	}
	_ = e.store.UpdateRepoCheck(ctx, check)
	return check, nil
}

func (e *Engine) StartRestore(ctx context.Context, destID, jobID int64, archiveName string) (string, error) {
	restoreID := fmt.Sprintf("restore-%d-%d", jobID, time.Now().Unix())
	state := &RestoreState{ID: restoreID, Status: "running"}
	e.restoreMu.Lock()
	e.restores[restoreID] = state
	e.restoreMu.Unlock()

	go func() {
		job, _ := e.store.GetJob(ctx, jobID)
		dest, _ := e.store.GetDestination(ctx, destID)
		source, _ := e.store.GetSource(ctx, job.SourceID)
		if job == nil || dest == nil || source == nil {
			state.Status = "failed"
			state.Logs = "job, destination or source not found"
			state.Done = true
			return
		}
		logFn := func(line string) {
			state.Logs += line + "\n"
			e.hub.Publish("restore:"+restoreID, "log", map[string]string{"line": line})
		}

		before := job.RestoreBeforeCommand
		if before == nil {
			before = job.SSHBeforeCommand
		}
		if source.Type == "ssh" && before != nil {
			_ = e.runSSHCommand(ctx, source, *before, logFn)
		}

		extractBase, srcCleanups, err := e.openSourcePath(ctx, source, logFn)
		if err != nil {
			state.Status = "failed"
			logFn("ERROR: " + err.Error())
			state.Done = true
			return
		}
		defer func() {
			for i := len(srcCleanups) - 1; i >= 0; i-- {
				srcCleanups[i].unmount(logFn)
			}
		}()

		rc, err := e.openDestinationRepo(ctx, dest, job, logFn)
		if err != nil {
			state.Status = "failed"
			logFn("ERROR: " + err.Error())
			state.Done = true
			return
		}
		defer rc.close()

		args := []string{"extract", "--progress", "-C", extractBase, rc.repoPath + "::" + archiveName}
		cmd := runBorgCmd(ctx, args, rc.env, borgRunDir(rc.repoPath))
		stderr, _ := cmd.StderrPipe()
		_ = cmd.Start()
		sc := bufio.NewScanner(stderr)
		for sc.Scan() {
			logFn(sc.Text())
		}
		err = cmd.Wait()
		after := job.RestoreAfterCommand
		if after == nil {
			after = job.SSHAfterCommand
		}
		if source.Type == "ssh" && after != nil {
			_ = e.runSSHCommand(ctx, source, *after, logFn)
		}
		if err != nil {
			state.Status = "failed"
		} else {
			state.Status = "success"
		}
		state.Done = true
	}()
	return restoreID, nil
}

func (e *Engine) RestoreStatus(restoreID string) *RestoreState {
	e.restoreMu.RLock()
	defer e.restoreMu.RUnlock()
	return e.restores[restoreID]
}

func (e *Engine) ListArchiveFiles(ctx context.Context, destID, jobID int64, archive, path string) ([]map[string]string, error) {
	rc, _, err := e.withRepo(ctx, destID, jobID)
	if err != nil {
		return nil, err
	}
	defer rc.close()
	target := archive
	if path != "" {
		target = archive + "::" + strings.TrimPrefix(path, "/")
	}
	cmd := runBorgCmd(ctx, []string{"list", rc.repoPath + "::" + target}, rc.env, borgRunDir(rc.repoPath))
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var files []map[string]string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 1 {
			files = append(files, map[string]string{"name": fields[len(fields)-1], "type": fields[0]})
		}
	}
	return files, nil
}

func (e *Engine) ExportTar(w io.Writer, ctx context.Context, destID, jobID int64, archive, path string) error {
	rc, _, err := e.withRepo(ctx, destID, jobID)
	if err != nil {
		return err
	}
	defer rc.close()
	args := []string{"export-tar", rc.repoPath + "::" + archive}
	if path != "" {
		args = append(args, path)
	}
	cmd := runBorgCmd(ctx, args, rc.env, borgRunDir(rc.repoPath))
	cmd.Stdout = w
	return cmd.Run()
}

func (e *Engine) ExportFile(w io.Writer, ctx context.Context, destID, jobID int64, archive, path string) error {
	rc, _, err := e.withRepo(ctx, destID, jobID)
	if err != nil {
		return err
	}
	defer rc.close()
	target := archive + "::" + strings.TrimPrefix(path, "/")
	cmd := runBorgCmd(ctx, []string{"extract", "--stdout", rc.repoPath + "::" + target}, rc.env, borgRunDir(rc.repoPath))
	cmd.Stdout = w
	return cmd.Run()
}

func (e *Engine) DiffArchives(ctx context.Context, destID, jobID int64, archive1, archive2 string) (map[string]any, error) {
	rc, _, err := e.withRepo(ctx, destID, jobID)
	if err != nil {
		return nil, err
	}
	defer rc.close()
	cmd := runBorgCmd(ctx, []string{"diff", rc.repoPath + "::" + archive1, archive2}, rc.env, borgRunDir(rc.repoPath))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return map[string]any{"error": string(out)}, nil
	}
	added, changed, deleted := []string{}, []string{}, []string{}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		path := extractDiffPath(line)
		switch {
		case strings.HasPrefix(lower, "added") || strings.HasPrefix(line, "+"):
			added = append(added, path)
		case strings.HasPrefix(lower, "removed") || strings.HasPrefix(line, "-"):
			deleted = append(deleted, path)
		default:
			changed = append(changed, path)
		}
	}
	return map[string]any{"added": added, "changed": changed, "deleted": deleted, "raw_output": string(out)}, nil
}

func extractDiffPath(line string) string {
	if idx := strings.Index(line, " "); idx >= 0 {
		return strings.TrimSpace(line[idx+1:])
	}
	return line
}

func (e *Engine) DeleteArchives(ctx context.Context, destID, jobID int64, names []string) error {
	rc, _, err := e.withRepo(ctx, destID, jobID)
	if err != nil {
		return err
	}
	defer rc.close()
	args := append([]string{"delete", rc.repoPath}, names...)
	return runBorgCmd(ctx, args, rc.env, borgRunDir(rc.repoPath)).Run()
}

func (e *Engine) BorgVersion() string {
	out, err := exec.Command("borg", "--version").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func (e *Engine) IsWorkerAlive() bool { return true }
