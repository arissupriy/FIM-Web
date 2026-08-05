// Package sqlite provides SQLite database implementation.
package sqlite

import (
	"context"
	"database/sql"

	"ojs-monitor/backend/internal/domain/models"
	"ojs-monitor/backend/internal/domain/repository"
)

// JobRepository implements repository.JobRepository using SQLite
type JobRepository struct {
	db *DB
}

// NewJobRepository creates a new JobRepository
func NewJobRepository(db *DB) repository.JobRepository {
	return &JobRepository{db: db}
}

// Create inserts a new job and returns the ID
func (r *JobRepository) Create(ctx context.Context, projectID int, jobType string) (int, error) {
	result, err := r.db.ExecContext(ctx,
		"INSERT INTO jobs (project_id, type, status) VALUES (?, ?, 'queued')",
		projectID, jobType)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	return int(id), err
}

// GetByID retrieves a job by ID
func (r *JobRepository) GetByID(ctx context.Context, id int) (*models.Job, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, project_id, type, status, error_message, created_at,
		       COALESCE(started_at, ''), COALESCE(finished_at, '')
		FROM jobs WHERE id = ?`, id)

	var j models.Job
	var errorMsg sql.NullString
	err := row.Scan(&j.ID, &j.ProjectID, &j.Type, &j.Status, &errorMsg,
		&j.CreatedAt, &j.StartedAt, &j.FinishedAt)
	if err != nil {
		return nil, err
	}
	if errorMsg.Valid {
		j.ErrorMsg = errorMsg.String
	}
	return &j, nil
}

// GetByProjectID retrieves jobs for a project
func (r *JobRepository) GetByProjectID(ctx context.Context, projectID int, limit int) ([]*models.Job, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, project_id, type, status, error_message, created_at,
		       COALESCE(started_at, ''), COALESCE(finished_at, '')
		FROM jobs WHERE project_id = ? ORDER BY id DESC LIMIT ?`, projectID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []*models.Job
	for rows.Next() {
		var j models.Job
		var errorMsg sql.NullString
		err := rows.Scan(&j.ID, &j.ProjectID, &j.Type, &j.Status, &errorMsg,
			&j.CreatedAt, &j.StartedAt, &j.FinishedAt)
		if err != nil {
			return nil, err
		}
		if errorMsg.Valid {
			j.ErrorMsg = errorMsg.String
		}
		jobs = append(jobs, &j)
	}
	return jobs, rows.Err()
}

// ClaimNextQueued attempts to claim the next queued job
func (r *JobRepository) ClaimNextQueued(ctx context.Context) (jobID, projectID int, jobType string, success bool, err error) {
	// Get next queued job
	row := r.db.QueryRowContext(ctx, `
		SELECT id, project_id, type FROM jobs
		WHERE status = 'queued' ORDER BY id ASC LIMIT 1`)

	err = row.Scan(&jobID, &projectID, &jobType)
	if err != nil {
		return 0, 0, "", false, nil // No jobs available
	}

	// Claim it with UPDATE
	result, err := r.db.ExecContext(ctx,
		"UPDATE jobs SET status = 'running', started_at = CURRENT_TIMESTAMP WHERE id = ? AND status = 'queued'",
		jobID)
	if err != nil {
		return 0, 0, "", false, err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return 0, 0, "", false, nil // Another worker claimed it
	}

	return jobID, projectID, jobType, true, nil
}

// Complete marks a job as done with stats
func (r *JobRepository) Complete(ctx context.Context, id int, success, skipped, errors int) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE jobs SET status = 'done', finished_at = CURRENT_TIMESTAMP,
		files_success=?, files_skipped=?, files_error=? WHERE id = ?`,
		success, skipped, errors, id)
	return err
}

// Fail marks a job as failed with error message
func (r *JobRepository) Fail(ctx context.Context, id int, errMsg string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE jobs SET status = 'failed', error_message = ?, finished_at = CURRENT_TIMESTAMP
		WHERE id = ?`, errMsg, id)
	return err
}

// Delete removes a job
func (r *JobRepository) Delete(ctx context.Context, id int) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM jobs WHERE id = ? AND status = 'queued'", id)
	return err
}

// GetRunningCount returns number of running/queued jobs for a project
func (r *JobRepository) GetRunningCount(ctx context.Context, projectID int) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM jobs WHERE project_id = ? AND status IN ('queued', 'running')",
		projectID).Scan(&count)
	return count, err
}

// ResuscitateCrashedJobs marks running jobs as queued
func (r *JobRepository) ResuscitateCrashedJobs(ctx context.Context) (int, error) {
	result, err := r.db.ExecContext(ctx, "UPDATE jobs SET status = 'queued' WHERE status = 'running'")
	if err != nil {
		return 0, err
	}
	rowsAffected, _ := result.RowsAffected()
	return int(rowsAffected), nil
}

// Cancel cancels a running job
func (r *JobRepository) Cancel(ctx context.Context, id int) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE jobs SET status = 'cancelled', finished_at = CURRENT_TIMESTAMP WHERE id = ? AND status = 'running'",
		id)
	return err
}

// GetActiveJobs returns all active (running) jobs
func (r *JobRepository) GetActiveJobs(ctx context.Context) ([]*models.Job, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, project_id, type, status, error_message, created_at,
		       COALESCE(started_at, ''), COALESCE(finished_at, '')
		FROM jobs WHERE status IN ('running', 'queued') ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []*models.Job
	for rows.Next() {
		var j models.Job
		var errorMsg sql.NullString
		err := rows.Scan(&j.ID, &j.ProjectID, &j.Type, &j.Status, &errorMsg,
			&j.CreatedAt, &j.StartedAt, &j.FinishedAt)
		if err != nil {
			return nil, err
		}
		if errorMsg.Valid {
			j.ErrorMsg = errorMsg.String
		}
		jobs = append(jobs, &j)
	}
	return jobs, rows.Err()
}
