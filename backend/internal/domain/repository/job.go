// Package repository defines interfaces for data access.
package repository

import (
	"context"

	"ojs-monitor/backend/internal/domain/models"
)

// JobRepository defines the interface for job data access
type JobRepository interface {
	// Create inserts a new job and returns the ID
	Create(ctx context.Context, projectID int, jobType string) (int, error)

	// GetByID retrieves a job by ID
	GetByID(ctx context.Context, id int) (*models.Job, error)

	// GetByProjectID retrieves jobs for a project (limit=0 means no limit)
	GetByProjectID(ctx context.Context, projectID int, limit int) ([]*models.Job, error)

	// Cancel cancels a running job
	Cancel(ctx context.Context, id int) error

	// GetActiveJobs returns all active (running) jobs
	GetActiveJobs(ctx context.Context) ([]*models.Job, error)

	// ClaimNextQueued attempts to claim the next queued job
	// Returns job ID, project ID, job type, and success boolean
	ClaimNextQueued(ctx context.Context) (jobID, projectID int, jobType string, success bool, err error)

	// Complete marks a job as done with stats
	Complete(ctx context.Context, id int, success, skipped, errors int) error

	// Fail marks a job as failed with error message
	Fail(ctx context.Context, id int, errMsg string) error

	// Delete removes a job
	Delete(ctx context.Context, id int) error

	// GetRunningCount returns number of running/queued jobs for a project
	GetRunningCount(ctx context.Context, projectID int) (int, error)

	// ResuscitateCrashedJobs marks running jobs as queued
	ResuscitateCrashedJobs(ctx context.Context) (int, error)
}
