// Package scan contains scan job use cases.
package scan

import (
	"context"
	"errors"

	"ojs-monitor/backend/internal/domain/repository"
)

// Sentinel errors
var (
	ErrJobExists     = errors.New("scan already exists")
	ErrNoBaseline   = errors.New("baseline not established")
	ErrNotCancellable = errors.New("job not cancellable")
)

// UseCase provides scan operations
type UseCase struct {
	projectRepo repository.ProjectRepository
	jobRepo    repository.JobRepository
	fileRepo   repository.FileRepository
}

// New creates a new scan use case
func New(projectRepo repository.ProjectRepository, jobRepo repository.JobRepository, fileRepo repository.FileRepository) *UseCase {
	return &UseCase{
		projectRepo: projectRepo,
		jobRepo:    jobRepo,
		fileRepo:   fileRepo,
	}
}

// StartBaseline queues a baseline scan job
func (uc *UseCase) StartBaseline(ctx context.Context, projectID int) error {
	// Check for existing jobs
	count, err := uc.jobRepo.GetRunningCount(ctx, projectID)
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrJobExists
	}

	// Queue baseline job
	_, err = uc.jobRepo.Create(ctx, projectID, "baseline")
	return err
}

// StartIntegrity queues an integrity scan job
func (uc *UseCase) StartIntegrity(ctx context.Context, projectID int) error {
	// Check project has baseline
	project, err := uc.projectRepo.GetByID(ctx, projectID)
	if err != nil {
		return err
	}
	if project.BaselineAt == 0 {
		return ErrNoBaseline
	}

	// Check for existing jobs
	count, err := uc.jobRepo.GetRunningCount(ctx, projectID)
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrJobExists
	}

	_, err = uc.jobRepo.Create(ctx, projectID, "integrity")
	return err
}

// Cancel removes a queued job
func (uc *UseCase) Cancel(ctx context.Context, jobID int) error {
	job, err := uc.jobRepo.GetByID(ctx, jobID)
	if err != nil {
		return err
	}
	if job.Status != "queued" {
		return ErrNotCancellable
	}
	return uc.jobRepo.Delete(ctx, jobID)
}

// Reset clears baseline and queues new baseline
func (uc *UseCase) Reset(ctx context.Context, projectID int) error {
	if err := uc.fileRepo.DeleteByProjectID(ctx, projectID); err != nil {
		return err
	}
	if err := uc.projectRepo.UpdateStatus(ctx, projectID, "pending_baseline"); err != nil {
		return err
	}
	_, err := uc.jobRepo.Create(ctx, projectID, "baseline")
	return err
}
