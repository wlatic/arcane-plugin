package job

import (
	"context"
	"log/slog"
	"time"

	"github.com/getarcaneapp/arcane/backend/internal/services"
	"github.com/go-co-op/gocron/v2"
)

type GitPollingJob struct {
	scheduler *Scheduler
	git       *services.GitService
	settings  *services.SettingsService
}

func NewGitPollingJob(scheduler *Scheduler, git *services.GitService, settings *services.SettingsService) *GitPollingJob {
	return &GitPollingJob{
		scheduler: scheduler,
		git:       git,
		settings:  settings,
	}
}

func (j *GitPollingJob) Register(ctx context.Context) error {
	// Fixed 1 minute interval, logic is inside Execute
	interval := 1 * time.Minute

	slog.InfoContext(ctx, "registering git polling job (per-project check)", "interval", interval.String())

	j.scheduler.RemoveJobByName("git-polling")

	jobDefinition := gocron.DurationJob(interval)
	return j.scheduler.RegisterJob(
		ctx,
		"git-polling",
		jobDefinition,
		j.Execute,
		false,
	)
}

func (j *GitPollingJob) Execute(ctx context.Context) error {
	// slog.InfoContext(ctx, "git polling run started")
	// Make it silent unless error or action? Or debug.
	// Users might complain if logs are flooded every minute.

	if !j.settings.GetBoolSetting(ctx, "opsModeEnabled", false) {
		return nil
	}

	if err := j.git.CheckAndSyncAll(ctx); err != nil {
		slog.ErrorContext(ctx, "git polling run failed", "error", err)
		return err
	}

	return nil
}

func (j *GitPollingJob) Reschedule(ctx context.Context) error {
	// No-op or reset to default
	// Since it's fixed, we might not need Reschedule exposed to settings changes anymore.
	return j.Register(ctx)
}
