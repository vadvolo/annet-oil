package checkeast

import (
	"context"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"annet-oil/internal/annet"
	"annet-oil/internal/config"
	"annet-oil/internal/inventory"
	"annet-oil/internal/logging"
)

// Scheduler runs checkeast (diff + S3 archive) on a cron schedule for a
// configurable scope of devices. A nil *Scheduler means "disabled" — callers
// must null-check it, matching the Store convention.
type Scheduler struct {
	cron  *cron.Cron
	svc   *annet.Service
	store *Store
	jobs  []config.ScheduleJob
}

// NewScheduler returns nil when scheduling is disabled or no jobs are
// configured. It does not start the cron loop — call Start.
func NewScheduler(cfg config.ScheduleConfig, svc *annet.Service, store *Store) *Scheduler {
	if !cfg.Enabled || len(cfg.Jobs) == 0 {
		return nil
	}
	return &Scheduler{
		cron:  cron.New(),
		svc:   svc,
		store: store,
		jobs:  cfg.Jobs,
	}
}

// Start registers every valid job and starts the cron loop. Jobs with an
// invalid cron expression are skipped with a warning rather than aborting.
func (s *Scheduler) Start() {
	if s == nil {
		return
	}
	for _, job := range s.jobs {
		job := job
		if job.Cron == "" {
			logging.Warn("checkeast scheduler: job has no cron expression, skipping", "job", job.Name)
			continue
		}
		if _, err := s.cron.AddFunc(job.Cron, func() { s.RunJob(context.Background(), job) }); err != nil {
			logging.Warn("checkeast scheduler: invalid cron expression, skipping",
				"job", job.Name, "cron", job.Cron, "error", err)
			continue
		}
		logging.Info("checkeast scheduler: job registered",
			"job", job.Name, "cron", job.Cron,
			"vendor", job.Vendor, "platform", job.Platform, "pattern", job.Pattern)
	}
	s.cron.Start()
}

// Stop halts the cron loop, waiting for any running job to finish.
func (s *Scheduler) Stop() {
	if s == nil {
		return
	}
	ctx := s.cron.Stop()
	<-ctx.Done()
}

// RunJob resolves the job's device scope from inventory and runs a diff for each
// device (limited by Concurrency), archiving every result to S3. It is exported
// so a run can also be triggered on demand.
func (s *Scheduler) RunJob(ctx context.Context, job config.ScheduleJob) {
	devices := inventory.FilterDevices(job.Vendor, job.Platform, job.Pattern)
	if len(devices) == 0 {
		logging.Warn("checkeast scheduler: job matched no devices",
			"job", job.Name, "vendor", job.Vendor, "platform", job.Platform, "pattern", job.Pattern)
		return
	}

	conc := job.Concurrency
	if conc < 1 {
		conc = 1
	}
	logging.Info("checkeast scheduler: job started",
		"job", job.Name, "devices", len(devices), "concurrency", conc, "by_alias", job.ByAlias)

	start := time.Now()
	var stored, failed int64
	var mu sync.Mutex

	sem := make(chan struct{}, conc)
	var wg sync.WaitGroup
	for _, d := range devices {
		target := d.Hostname
		if job.ByAlias {
			target = aliasTarget(d)
		}
		if target == "" {
			continue
		}

		wg.Add(1)
		sem <- struct{}{}
		go func(target string) {
			defer wg.Done()
			defer func() { <-sem }()

			req := &annet.CommandRequest{
				Command:    "diff",
				Filters:    []string{target},
				Generators: job.Generators,
				Timeout:    job.Timeout,
				Quiet:      true,
			}
			resp, err := s.svc.ExecuteCommand(ctx, req)
			if err != nil {
				logging.Warn("checkeast scheduler: diff failed", "job", job.Name, "target", target, "error", err)
				mu.Lock()
				failed++
				mu.Unlock()
				return
			}
			if s.store != nil {
				if _, err := s.store.Archive(ctx, req, resp); err != nil {
					logging.Warn("checkeast scheduler: archive failed", "job", job.Name, "target", target, "error", err)
				}
			}
			mu.Lock()
			stored++
			mu.Unlock()
		}(target)
	}
	wg.Wait()

	logging.Info("checkeast scheduler: job finished",
		"job", job.Name, "stored", stored, "failed", failed, "duration", time.Since(start).String())
}

// aliasTarget picks the address to diff against when a job targets devices by
// alias/IP: the device IP, else its first alias, else the hostname.
func aliasTarget(d inventory.Device) string {
	if d.IP != "" {
		return d.IP
	}
	if len(d.Aliases) > 0 {
		return d.Aliases[0]
	}
	return d.Hostname
}
