// Package sqlite provides SQLite database implementation.
package sqlite

import (
	"context"

	"ojs-monitor/backend/internal/domain/models"
	"ojs-monitor/backend/internal/domain/repository"
)

// FileRepository implements repository.FileRepository using SQLite
type FileRepository struct {
	db *DB
}

// NewFileRepository creates a new FileRepository
func NewFileRepository(db *DB) repository.FileRepository {
	return &FileRepository{db: db}
}

// BatchUpsert inserts or updates multiple files
func (r *FileRepository) BatchUpsert(ctx context.Context, files []*models.ProjectFile) error {
	if len(files) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Insert statement
	stmtInsert, err := tx.PrepareContext(ctx, `
		INSERT OR IGNORE INTO project_files
		(project_id, file_path, hash, file_size, mod_time, status, file_type,
		 file_mode, file_uid, file_gid, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, strftime('%s', 'now'), strftime('%s', 'now'))`)
	if err != nil {
		return err
	}
	defer stmtInsert.Close()

	// Update statement
	stmtUpdate, err := tx.PrepareContext(ctx, `
		UPDATE project_files SET hash=?, file_size=?, mod_time=?, status=?,
		file_mode=?, file_uid=?, file_gid=?, updated_at=strftime('%s', 'now')
		WHERE id=?`)
	if err != nil {
		return err
	}
	defer stmtUpdate.Close()

	for _, f := range files {
		fileType := f.FileType
		if fileType == "" {
			fileType = "project"
		}
		if f.ID == 0 {
			_, err = stmtInsert.ExecContext(ctx, f.ProjectID, f.FilePath, f.Hash,
				f.FileSize, f.ModTime, f.Status, fileType,
				f.FileMode, f.FileUID, f.FileGID)
		} else {
			_, err = stmtUpdate.ExecContext(ctx, f.Hash, f.FileSize, f.ModTime, f.Status,
				f.FileMode, f.FileUID, f.FileGID, f.ID)
		}
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

// BatchDelete removes multiple files by ID
func (r *FileRepository) BatchDelete(ctx context.Context, ids []int) error {
	if len(ids) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, "DELETE FROM project_files WHERE id=?")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, id := range ids {
		if _, err := stmt.ExecContext(ctx, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// GetByProjectID retrieves all files for a project as a map
func (r *FileRepository) GetByProjectID(ctx context.Context, projectID int) (map[string]*models.ProjectFile, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, project_id, file_path, hash, file_size, mod_time, status,
		       COALESCE(file_mode, ''), COALESCE(file_uid, 0), COALESCE(file_gid, 0),
		       COALESCE(permission_changes, 0)
		FROM project_files WHERE project_id=?`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	files := make(map[string]*models.ProjectFile)
	for rows.Next() {
		var f models.ProjectFile
		err := rows.Scan(&f.ID, &f.ProjectID, &f.FilePath, &f.Hash,
			&f.FileSize, &f.ModTime, &f.Status,
			&f.FileMode, &f.FileUID, &f.FileGID, &f.PermissionChanges)
		if err != nil {
			return nil, err
		}
		files[f.FilePath] = &f
	}
	return files, rows.Err()
}

// GetByID retrieves a single file
func (r *FileRepository) GetByID(ctx context.Context, fileID, projectID int) (*models.ProjectFile, error) {
	var f models.ProjectFile
	err := r.db.QueryRowContext(ctx, `
		SELECT id, project_id, file_path, hash, file_size, mod_time, status,
		       COALESCE(file_mode, ''), COALESCE(file_uid, 0), COALESCE(file_gid, 0),
		       COALESCE(permission_changes, 0)
		FROM project_files WHERE id = ? AND project_id = ?`,
		fileID, projectID).Scan(&f.ID, &f.ProjectID, &f.FilePath, &f.Hash,
		&f.FileSize, &f.ModTime, &f.Status,
		&f.FileMode, &f.FileUID, &f.FileGID, &f.PermissionChanges)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// DeleteByProjectID removes all files for a project
func (r *FileRepository) DeleteByProjectID(ctx context.Context, projectID int) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM project_files WHERE project_id = ?", projectID)
	return err
}

// IncrementPermissionChanges increments permission change counter
func (r *FileRepository) IncrementPermissionChanges(ctx context.Context, fileID, projectID int) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE project_files SET permission_changes = permission_changes + 1
		WHERE id = ? AND project_id = ?`, fileID, projectID)
	return err
}

// GetStats returns file statistics for a project
func (r *FileRepository) GetStats(ctx context.Context, projectID int) (added, modified, deleted, orphan int, err error) {
	err = r.db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(CASE WHEN status = 'ADDED' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'MODIFIED' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'DELETED' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'ORPHAN' THEN 1 ELSE 0 END), 0)
		FROM project_files WHERE project_id = ?`, projectID).Scan(&added, &modified, &deleted, &orphan)
	return
}

// ListByProjectID retrieves files for a project with filters
func (r *FileRepository) ListByProjectID(ctx context.Context, projectID int, filters repository.FileFilters) ([]*models.ProjectFile, int, error) {
	// Build WHERE clause
	where := "WHERE project_id = ?"
	args := []interface{}{projectID}

	if filters.Status != "" {
		where += " AND status = ?"
		args = append(args, filters.Status)
	}
	if filters.FileType != "" {
		where += " AND file_type = ?"
		args = append(args, filters.FileType)
	}
	if filters.Search != "" {
		where += " AND file_path LIKE ?"
		args = append(args, "%"+filters.Search+"%")
	}
	if filters.FileID > 0 {
		where += " AND id = ?"
		args = append(args, filters.FileID)
	}

	// Count total
	var total int
	countQuery := "SELECT COUNT(*) FROM project_files " + where
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Get paginated results
	limit := filters.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := filters.Offset
	if offset < 0 {
		offset = 0
	}

	query := "SELECT id, project_id, file_path, hash, file_size, mod_time, status, file_type, COALESCE(file_mode, ''), COALESCE(file_uid, 0), COALESCE(file_gid, 0), COALESCE(permission_changes, 0) FROM project_files " + where + " ORDER BY id DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var files []*models.ProjectFile
	for rows.Next() {
		var f models.ProjectFile
		err := rows.Scan(&f.ID, &f.ProjectID, &f.FilePath, &f.Hash,
			&f.FileSize, &f.ModTime, &f.Status, &f.FileType,
			&f.FileMode, &f.FileUID, &f.FileGID, &f.PermissionChanges)
		if err != nil {
			return nil, 0, err
		}
		files = append(files, &f)
	}

	if files == nil {
		files = []*models.ProjectFile{}
	}

	return files, total, rows.Err()
}

// GetByProjectIDPaginated retrieves files with pagination
func (r *FileRepository) GetByProjectIDPaginated(ctx context.Context, projectID, limit, offset int) ([]*models.ProjectFile, error) {
	if limit <= 0 {
		limit = 500
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, project_id, file_path, hash, file_size, mod_time, status, file_type,
		       COALESCE(file_mode, ''), COALESCE(file_uid, 0), COALESCE(file_gid, 0),
		       COALESCE(permission_changes, 0)
		FROM project_files WHERE project_id = ? ORDER BY id DESC LIMIT ? OFFSET ?`,
		projectID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []*models.ProjectFile
	for rows.Next() {
		var f models.ProjectFile
		err := rows.Scan(&f.ID, &f.ProjectID, &f.FilePath, &f.Hash,
			&f.FileSize, &f.ModTime, &f.Status, &f.FileType,
			&f.FileMode, &f.FileUID, &f.FileGID, &f.PermissionChanges)
		if err != nil {
			return nil, err
		}
		files = append(files, &f)
	}
	if files == nil {
		files = []*models.ProjectFile{}
	}
	return files, rows.Err()
}

// List retrieves files with flexible filters
func (r *FileRepository) List(ctx context.Context, params repository.FileListParams) ([]*models.ProjectFile, error) {
	// Build WHERE clause
	where := "WHERE project_id = ?"
	args := []interface{}{params.ProjectID}

	if params.Search != "" {
		where += " AND file_path LIKE ?"
		args = append(args, "%"+params.Search+"%")
	}
	if params.Status != "" && params.Status != "all" {
		where += " AND status = ?"
		args = append(args, params.Status)
	}

	// File type filtering based on FilesPaths
	if len(params.FilesPaths) > 0 && params.FilesPaths[0] != "" {
		if params.FileType == "project" {
			// Files NOT in files_path = project files
			where += " AND ("
			for i, fp := range params.FilesPaths {
				if fp != "" {
					if i > 0 {
						where += " AND "
					}
					where += "file_path NOT LIKE ?"
					args = append(args, fp+"%")
				}
			}
			where += ")"
		} else if params.FileType == "uploads" {
			// Files in files_path = uploads
			where += " AND ("
			for i, fp := range params.FilesPaths {
				if fp != "" {
					if i > 0 {
						where += " OR "
					}
					where += "file_path LIKE ?"
					args = append(args, fp+"%")
				}
			}
			where += ")"
		}
	}

	limit := params.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := params.Offset
	if offset < 0 {
		offset = 0
	}

	query := `
		SELECT id, project_id, file_path, hash, file_size, mod_time, status, file_type,
		       COALESCE(file_mode, ''), COALESCE(file_uid, 0), COALESCE(file_gid, 0),
		       COALESCE(permission_changes, 0)
		FROM project_files ` + where + ` ORDER BY id DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []*models.ProjectFile
	for rows.Next() {
		var f models.ProjectFile
		err := rows.Scan(&f.ID, &f.ProjectID, &f.FilePath, &f.Hash,
			&f.FileSize, &f.ModTime, &f.Status, &f.FileType,
			&f.FileMode, &f.FileUID, &f.FileGID, &f.PermissionChanges)
		if err != nil {
			return nil, err
		}
		files = append(files, &f)
	}
	if files == nil {
		files = []*models.ProjectFile{}
	}
	return files, rows.Err()
}

// Count returns total files count for a project
func (r *FileRepository) Count(ctx context.Context, projectID int, status string) (int, error) {
	var count int
	if status != "" && status != "all" {
		err := r.db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM project_files WHERE project_id = ? AND status = ?",
			projectID, status).Scan(&count)
		return count, err
	}
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM project_files WHERE project_id = ?",
		projectID).Scan(&count)
	return count, err
}

// GetOrphans retrieves orphan files for a project
func (r *FileRepository) GetOrphans(ctx context.Context, projectID, limit int) ([]*models.ProjectFile, error) {
	if limit <= 0 {
		limit = 500
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, project_id, file_path, hash, file_size, mod_time, status, file_type,
		       COALESCE(file_mode, ''), COALESCE(file_uid, 0), COALESCE(file_gid, 0),
		       COALESCE(permission_changes, 0)
		FROM project_files WHERE project_id = ? AND status = 'ORPHAN' ORDER BY id DESC LIMIT ?`,
		projectID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []*models.ProjectFile
	for rows.Next() {
		var f models.ProjectFile
		err := rows.Scan(&f.ID, &f.ProjectID, &f.FilePath, &f.Hash,
			&f.FileSize, &f.ModTime, &f.Status, &f.FileType,
			&f.FileMode, &f.FileUID, &f.FileGID, &f.PermissionChanges)
		if err != nil {
			return nil, err
		}
		files = append(files, &f)
	}
	if files == nil {
		files = []*models.ProjectFile{}
	}
	return files, rows.Err()
}

// GetByHash retrieves files with the same hash
func (r *FileRepository) GetByHash(ctx context.Context, hash string) ([]*models.ProjectFile, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, project_id, file_path, hash, file_size, mod_time, status, file_type,
		       COALESCE(file_mode, ''), COALESCE(file_uid, 0), COALESCE(file_gid, 0),
		       COALESCE(permission_changes, 0)
		FROM project_files WHERE hash = ? ORDER BY id DESC`,
		hash)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []*models.ProjectFile
	for rows.Next() {
		var f models.ProjectFile
		err := rows.Scan(&f.ID, &f.ProjectID, &f.FilePath, &f.Hash,
			&f.FileSize, &f.ModTime, &f.Status, &f.FileType,
			&f.FileMode, &f.FileUID, &f.FileGID, &f.PermissionChanges)
		if err != nil {
			return nil, err
		}
		files = append(files, &f)
	}
	if files == nil {
		files = []*models.ProjectFile{}
	}
	return files, rows.Err()
}

// GetBaselineFile retrieves a file by project ID and path for permission comparison
func (r *FileRepository) GetBaselineFile(ctx context.Context, projectID int, filePath string) (*models.ProjectFile, error) {
	var f models.ProjectFile
	var fileMode string
	var fileUID, fileGID uint32

	err := r.db.QueryRowContext(ctx, `
		SELECT id, project_id, file_path, hash, file_size, mod_time, status, file_type,
		       COALESCE(file_mode, ''), COALESCE(file_uid, 0), COALESCE(file_gid, 0),
		       COALESCE(permission_changes, 0)
		FROM project_files
		WHERE project_id = ? AND file_path = ?
	`, projectID, filePath).Scan(&f.ID, &f.ProjectID, &f.FilePath, &f.Hash,
		&f.FileSize, &f.ModTime, &f.Status, &f.FileType,
		&fileMode, &fileUID, &fileGID, &f.PermissionChanges)

	if err != nil {
		return nil, err
	}

	f.FileMode = fileMode
	f.FileUID = fileUID
	f.FileGID = fileGID
	return &f, nil
}
