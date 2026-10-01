package job

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/NERVEbing/ikuai-aio/v4/config"
	"github.com/NERVEbing/ikuai-aio/v4/internal/schedule"
	"github.com/robfig/cron/v3"
)

// Run serializes router writes. Each task has at most one running or queued
// invocation; cancellation interrupts queued tasks, downloads and API calls.
func Run(ctx context.Context, conf config.Config, worker *Worker, logger *slog.Logger) error {
	scheduler := cron.New(cron.WithLocation(conf.Timezone))
	serial := make(chan struct{}, 1)
	var initial sync.WaitGroup
	var jobs []cron.Job
	register := func(task config.Task, kind string, execute func(context.Context) error) error {
		spec, err := schedule.Parse(task.Schedule)
		if err != nil {
			return fmt.Errorf("%s/%s: %w", kind, task.Name, err)
		}
		job := cron.FuncJob(func() {
			select {
			case serial <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-serial }()
			if ctx.Err() != nil {
				return
			}
			jobCtx, cancel := context.WithTimeout(ctx, conf.JobTimeout)
			defer cancel()
			start := time.Now()
			if err := execute(jobCtx); err != nil {
				logger.Error("task failed", "kind", kind, "name", task.Name, "error", err, "duration", time.Since(start))
			} else {
				logger.Info("task completed", "kind", kind, "name", task.Name, "duration", time.Since(start))
			}
		})
		wrapped := cron.SkipIfStillRunning(cron.DefaultLogger)(job)
		jobs = append(jobs, wrapped)
		scheduler.Schedule(spec, wrapped)
		return nil
	}
	for _, task := range conf.IPObjects {
		if err := register(task.Task, "ip-object", func(ctx context.Context) error { return worker.SyncIPObjects(ctx, task) }); err != nil {
			return err
		}
	}
	for _, task := range conf.DomainRules {
		if err := register(task.Task, "domain-rule", func(ctx context.Context) error { return worker.SyncDomainRule(ctx, task) }); err != nil {
			return err
		}
	}
	if !conf.SkipStart {
		for _, job := range jobs {
			initial.Add(1)
			go func() { defer initial.Done(); job.Run() }()
		}
	}
	scheduler.Start()
	<-ctx.Done()
	<-scheduler.Stop().Done()
	initial.Wait()
	return nil
}
