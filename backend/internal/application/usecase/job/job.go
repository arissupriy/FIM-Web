// Package job provides job-related use cases.
package job

import (
	"context"

	"ojs-monitor/backend/internal/domain/models"
	"ojs-monitor/backend/internal/domain/repository"
)

// UseCase handles job-related business logic.
type UseCase struct {
	jobRepo repository.JobRepository
}

// New creates a new job use case.
func New(jobRepo repository.JobRepository) *UseCase {
	return &UseCase{jobRepo: jobRepo}
}

// GetByProjectID returns all jobs for a project.
func (uc *UseCase) GetByProjectID(ctx context.Context, projectID int, limit int) ([]*models.Job, error) {
	return uc.jobRepo.GetByProjectID(ctx, projectID, limit)
}

// GetByID returns a job by ID.
func (uc *UseCase) GetByID(ctx context.Context, id int) (*models.Job, error) {
	return uc.jobRepo.GetByID(ctx, id)
}

// Cancel cancels a running job.
func (uc *UseCase) Cancel(ctx context.Context, id int) error {
	return uc.jobRepo.Cancel(ctx, id)
}

// GetActiveJobs returns all active (running) jobs.
func (uc *UseCase) GetActiveJobs(ctx context.Context) ([]*models.Job, error) {
	return uc.jobRepo.GetActiveJobs(ctx)
}

// GetStats returns job statistics for a project.
func (uc *UseCase) GetStats(ctx context.Context, projectID int) (*JobStats, error) {
	// Get all jobs (limit=0 means no limit)
	jobs, err := uc.jobRepo.GetByProjectID(ctx, projectID, 0)
	if err != nil {
		return nil, err
	}

	stats := &JobStats{}
	for _, job := range jobs {
		switch job.Status {
		case "queued":
			stats.Queued++
		case "running":
			stats.Running++
		case "done":
			stats.Completed++
		case "failed":
			stats.Failed++
		}
	}
	return stats, nil
}

// JobStats holds job statistics.
type JobStats struct {
	Queued    int `json:"queued"`
	Running   int `json:"running"`
	Completed int `json:"completed"`
	Failed    int `json:"failed"`
}
