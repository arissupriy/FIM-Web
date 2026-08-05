// Package sqlite provides SQLite database implementation.
package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"

	"ojs-monitor/backend/internal/domain/models"
	"ojs-monitor/backend/internal/domain/repository"
)

// ProjectRepository implements repository.ProjectRepository using SQLite
type ProjectRepository struct {
	db *DB
}

// NewProjectRepository creates a new ProjectRepository
func NewProjectRepository(db *DB) repository.ProjectRepository {
	return &ProjectRepository{db: db}
}

// Create inserts a new project and returns the ID
func (r *ProjectRepository) Create(ctx context.Context, p *models.Project) (int, error) {
	if p.Template == "" {
		p.Template = "OJS 3.x"
	}
	appPathJSON, _ := json.Marshal(p.AppPaths)
	filesPathJSON, _ := json.Marshal(p.FilesPaths)
	blacklistJSON, _ := json.Marshal(p.BlacklistExts)
	whitelistJSON, _ := json.Marshal(p.WhitelistPaths)

	result, err := r.db.ExecContext(ctx, `
		INSERT INTO projects (name, description, template, app_path, files_path, blacklist_exts,
		whitelist_paths, db_host, db_user, db_pass, db_name, status, baseline_total, baseline_processed, error_message)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'unconfigured', 0, 0, '')`,
		p.Name, p.Description, p.Template, string(appPathJSON), string(filesPathJSON),
		string(blacklistJSON), string(whitelistJSON), p.DBHost, p.DBUser, p.DBPass, p.DBName)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	return int(id), err
}

// GetByID retrieves a project by ID
func (r *ProjectRepository) GetByID(ctx context.Context, id int) (*models.Project, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, name, description, template, app_path, files_path, blacklist_exts, whitelist_paths,
		       db_host, db_user, db_pass, db_name, status,
		       COALESCE(baseline_total, 0), COALESCE(baseline_processed, 0),
		       COALESCE(error_message, ''), COALESCE(rescan_interval, 10),
		       COALESCE(baseline_at, 0), COALESCE(watcher_status, 'stopped'),
		       COALESCE(integrity_scan_enabled, 0), COALESCE(last_integrity_scan, 0)
		FROM projects WHERE id = ?`, id)

	return r.scanProject(row)
}

// List retrieves all projects
func (r *ProjectRepository) List(ctx context.Context) ([]*models.Project, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, description, template, app_path, files_path, blacklist_exts, whitelist_paths,
		       db_host, db_user, db_pass, db_name, status,
		       COALESCE(baseline_total, 0), COALESCE(baseline_processed, 0),
		       COALESCE(error_message, ''), COALESCE(rescan_interval, 10),
		       COALESCE(baseline_at, 0), COALESCE(watcher_status, 'stopped'),
		       COALESCE(integrity_scan_enabled, 0), COALESCE(last_integrity_scan, 0)
		FROM projects`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanProjects(rows)
}

// Update updates an existing project
func (r *ProjectRepository) Update(ctx context.Context, p *models.Project) error {
	appPathJSON, _ := json.Marshal(p.AppPaths)
	filesPathJSON, _ := json.Marshal(p.FilesPaths)
	blacklistJSON, _ := json.Marshal(p.BlacklistExts)
	whitelistJSON, _ := json.Marshal(p.WhitelistPaths)

	// Determine status based on configuration
	status := "unconfigured"
	if p.DBHost != "" && p.DBUser != "" && p.DBName != "" && len(p.AppPaths) > 0 && p.AppPaths[0] != "" {
		status = "pending_baseline"
	}

	_, err := r.db.ExecContext(ctx, `
		UPDATE projects SET name=?, description=?, template=?, app_path=?, files_path=?,
		blacklist_exts=?, whitelist_paths=?, db_host=?, db_user=?, db_pass=?, db_name=?,
		status=?, rescan_interval=? WHERE id=?`,
		p.Name, p.Description, p.Template, string(appPathJSON), string(filesPathJSON),
		string(blacklistJSON), string(whitelistJSON), p.DBHost, p.DBUser, p.DBPass, p.DBName,
		status, p.RescanInterval, p.ID)
	return err
}

// Delete removes a project
func (r *ProjectRepository) Delete(ctx context.Context, id int) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM projects WHERE id = ?", id)
	return err
}

// UpdateStatus updates project status
func (r *ProjectRepository) UpdateStatus(ctx context.Context, id int, status string) error {
	_, err := r.db.ExecContext(ctx, "UPDATE projects SET status = ? WHERE id = ?", status, id)
	return err
}

// UpdateWatcherStatus updates project watcher status
func (r *ProjectRepository) UpdateWatcherStatus(ctx context.Context, id int, status string) error {
	_, err := r.db.ExecContext(ctx, "UPDATE projects SET watcher_status = ? WHERE id = ?", status, id)
	return err
}

// GetActiveProjects retrieves all active projects
func (r *ProjectRepository) GetActiveProjects(ctx context.Context) ([]*models.Project, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, description, template, app_path, files_path, blacklist_exts, whitelist_paths,
		       db_host, db_user, db_pass, db_name, status,
		       COALESCE(baseline_total, 0), COALESCE(baseline_processed, 0),
		       COALESCE(error_message, ''), COALESCE(rescan_interval, 10),
		       COALESCE(baseline_at, 0), COALESCE(watcher_status, 'stopped'),
		       COALESCE(integrity_scan_enabled, 0), COALESCE(last_integrity_scan, 0)
		FROM projects WHERE status = 'active'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanProjects(rows)
}

// GetProjectsForIntegrityScan retrieves projects with integrity scan enabled
func (r *ProjectRepository) GetProjectsForIntegrityScan(ctx context.Context) ([]*models.Project, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, description, template, app_path, files_path, blacklist_exts, whitelist_paths,
		       db_host, db_user, db_pass, db_name, status,
		       COALESCE(baseline_total, 0), COALESCE(baseline_processed, 0),
		       COALESCE(error_message, ''), COALESCE(rescan_interval, 10),
		       COALESCE(baseline_at, 0), COALESCE(watcher_status, 'stopped'),
		       COALESCE(integrity_scan_enabled, 0), COALESCE(last_integrity_scan, 0)
		FROM projects WHERE status = 'active' AND integrity_scan_enabled = 1 AND baseline_at > 0`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanProjects(rows)
}

// UpdateBaseline updates baseline progress and status
func (r *ProjectRepository) UpdateBaseline(ctx context.Context, id int, status string, total, processed int) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE projects SET status = ?, baseline_total = ?, baseline_processed = ?,
		baseline_at = strftime('%s', 'now') WHERE id = ?`,
		status, total, processed, id)
	return err
}

// UpdateIntegrityScan updates last integrity scan timestamp and status
func (r *ProjectRepository) UpdateIntegrityScan(ctx context.Context, id int, status string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE projects SET status = ?, last_integrity_scan = strftime('%s', 'now') WHERE id = ?`,
		status, id)
	return err
}

// scanProject scans a single project from a row
func (r *ProjectRepository) scanProject(row *sql.Row) (*models.Project, error) {
	var p models.Project
	var appPathJSON, filesPathJSON, blacklistJSON, whitelistJSON string

	err := row.Scan(
		&p.ID, &p.Name, &p.Description, &p.Template,
		&appPathJSON, &filesPathJSON, &blacklistJSON, &whitelistJSON,
		&p.DBHost, &p.DBUser, &p.DBPass, &p.DBName, &p.Status,
		&p.BaselineTotal, &p.BaselineProcessed,
		&p.ErrorMessage, &p.RescanInterval,
		&p.BaselineAt, &p.WatcherStatus,
		&p.IntegrityScanEnabled, &p.LastIntegrityScan,
	)
	if err != nil {
		return nil, err
	}

	json.Unmarshal([]byte(appPathJSON), &p.AppPaths)
	json.Unmarshal([]byte(filesPathJSON), &p.FilesPaths)
	json.Unmarshal([]byte(blacklistJSON), &p.BlacklistExts)
	json.Unmarshal([]byte(whitelistJSON), &p.WhitelistPaths)

	if p.AppPaths == nil {
		p.AppPaths = []string{}
	}
	if p.FilesPaths == nil {
		p.FilesPaths = []string{}
	}
	if p.BlacklistExts == nil {
		p.BlacklistExts = []string{"php", "phtml", "sh"}
	}
	if p.WhitelistPaths == nil {
		p.WhitelistPaths = []string{}
	}
	if p.RescanInterval == 0 {
		p.RescanInterval = 10
	}
	if p.WatcherStatus == "" {
		p.WatcherStatus = "stopped"
	}

	p.Configured = p.IsConfigured()
	return &p, nil
}

// scanProjects scans multiple projects from rows
func (r *ProjectRepository) scanProjects(rows *sql.Rows) ([]*models.Project, error) {
	var projects []*models.Project
	for rows.Next() {
		var p models.Project
		var appPathJSON, filesPathJSON, blacklistJSON, whitelistJSON string

		err := rows.Scan(
			&p.ID, &p.Name, &p.Description, &p.Template,
			&appPathJSON, &filesPathJSON, &blacklistJSON, &whitelistJSON,
			&p.DBHost, &p.DBUser, &p.DBPass, &p.DBName, &p.Status,
			&p.BaselineTotal, &p.BaselineProcessed,
			&p.ErrorMessage, &p.RescanInterval,
			&p.BaselineAt, &p.WatcherStatus,
			&p.IntegrityScanEnabled, &p.LastIntegrityScan,
		)
		if err != nil {
			return nil, err
		}

		json.Unmarshal([]byte(appPathJSON), &p.AppPaths)
		json.Unmarshal([]byte(filesPathJSON), &p.FilesPaths)
		json.Unmarshal([]byte(blacklistJSON), &p.BlacklistExts)
		json.Unmarshal([]byte(whitelistJSON), &p.WhitelistPaths)

		if p.AppPaths == nil {
			p.AppPaths = []string{}
		}
		if p.FilesPaths == nil {
			p.FilesPaths = []string{}
		}
		if p.BlacklistExts == nil {
			p.BlacklistExts = []string{"php", "phtml", "sh"}
		}
		if p.WhitelistPaths == nil {
			p.WhitelistPaths = []string{}
		}
		if p.RescanInterval == 0 {
			p.RescanInterval = 10
		}
		if p.WatcherStatus == "" {
			p.WatcherStatus = "stopped"
		}

		p.Configured = p.IsConfigured()
		projects = append(projects, &p)
	}
	return projects, rows.Err()
}

// Count returns the number of projects
func (r *ProjectRepository) Count(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM projects").Scan(&count)
	return count, err
}
