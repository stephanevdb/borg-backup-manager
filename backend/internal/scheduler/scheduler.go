package scheduler

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"sync"

	"github.com/borg-backup-manager/backend/internal/backup"
	"github.com/borg-backup-manager/backend/internal/config"
	"github.com/borg-backup-manager/backend/internal/db"
	"github.com/borg-backup-manager/backend/internal/notify"
	"github.com/robfig/cron/v3"
)

type Service struct {
	cfg       *config.Config
	store     *db.Store
	engine    *backup.Engine
	notifier  *notify.Service
	cron      *cron.Cron
	mu        sync.Mutex
	jobEntries map[int64]cron.EntryID
}

func New(cfg *config.Config, store *db.Store, engine *backup.Engine) *Service {
	return &Service{
		cfg: cfg, store: store, engine: engine,
		cron: cron.New(), jobEntries: map[int64]cron.EntryID{},
	}
}

func (s *Service) SetNotifier(n *notify.Service) {
	s.notifier = n
}

func (s *Service) Start(ctx context.Context) {
	s.SyncJobs(ctx)
	if s.cfg.DailyVerification {
		cronExpr := s.store.GetSetting(ctx, "daily_verification_cron")
		if cronExpr == "" {
			cronExpr = s.cfg.DailyVerificationCron
		}
		_, _ = s.cron.AddFunc(cronExpr, func() { s.runDailyVerifications(ctx) })
	}
	_, _ = s.cron.AddFunc("@every 1h", func() { s.storageWarningCheck(ctx) })
	s.cron.Start()
}

func (s *Service) Stop() {
	s.cron.Stop()
}

func (s *Service) SyncJobs(ctx context.Context) {
	jobs, err := s.store.ListJobs(ctx)
	if err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	active := map[int64]struct{}{}
	for _, j := range jobs {
		if !j.Enabled || j.Schedule == "" || j.Schedule == "manual" {
			continue
		}
		active[j.ID] = struct{}{}
		if _, ok := s.jobEntries[j.ID]; ok {
			continue
		}
		jobID := j.ID
		expr := j.Schedule
		eid, err := s.cron.AddFunc(expr, func() {
			if _, err := s.engine.QueueJob(ctx, jobID); err != nil {
				log.Printf("scheduled job %d: %v", jobID, err)
			}
		})
		if err != nil {
			log.Printf("invalid cron for job %d: %v", jobID, err)
			continue
		}
		s.jobEntries[jobID] = eid
	}
	for id, eid := range s.jobEntries {
		if _, ok := active[id]; !ok {
			s.cron.Remove(eid)
			delete(s.jobEntries, id)
		}
	}
}

func (s *Service) runDailyVerifications(ctx context.Context) {
	enabled := s.store.GetSetting(ctx, "daily_verification_enabled")
	if enabled == "false" {
		return
	}
	jobs, _ := s.store.ListJobs(ctx)
	for _, j := range jobs {
		if !j.Enabled || !j.DailyVerificationEnabled {
			continue
		}
		jid := j.ID
		destID := j.DestinationID
		go func() {
			_, _ = s.engine.Verify(ctx, destID, &jid, "scheduler")
		}()
	}
}

func (s *Service) storageWarningCheck(ctx context.Context) {
	dests, _ := s.store.ListDestinations(ctx)
	channels, _ := s.store.ListNotificationChannels(ctx)
	for _, d := range dests {
		st := s.engine.DestinationStorage(ctx, &d)
		if !st.IsWarning {
			continue
		}
		msg := fmt.Sprintf("Storage warning for destination %s: %.1f%% used", d.Name, 0.0)
		if st.PercentUsed != nil {
			msg = fmt.Sprintf("Storage warning for destination %s: %.1f%% used", d.Name, *st.PercentUsed)
		}
		for _, ch := range channels {
			if ch.Enabled && ch.OnStorageWarning {
				if s.notifier != nil {
					s.notifier.NotifyStorageWarning(ctx, d.Name, msg)
				}
			}
		}
	}
	_, _ = strconv.Atoi(s.store.GetSetting(ctx, "storage_warning_interval"))
}

func (s *Service) ScheduledJobCount() int {
	return len(s.cron.Entries())
}
