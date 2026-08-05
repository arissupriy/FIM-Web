// Package sqlite provides SQLite database implementation.
package sqlite

import (
	"context"
	"database/sql"
	"time"

	"ojs-monitor/backend/internal/domain/models"
	"ojs-monitor/backend/internal/domain/repository"
)

// FIMEventRepository implements repository.FIMEventRepository using SQLite
type FIMEventRepository struct {
	db *DB
}

// NewFIMEventRepository creates a new FIMEventRepository
func NewFIMEventRepository(db *DB) repository.FIMEventRepository {
	return &FIMEventRepository{db: db}
}

// Create inserts a new FIM event
func (r *FIMEventRepository) Create(ctx context.Context, event *models.FIMEvent) error {
	ts := time.Now().Unix()
	if event.Timestamp != "" {
		if t, err := time.Parse(time.RFC3339, event.Timestamp); err == nil {
			ts = t.Unix()
		}
	}

	details := event.Details
	if details == "" {
		details = "{}"
	}

	alertSent := 0
	if event.AlertSent {
		alertSent = 1
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO fim_events
		(project_id, event_type, file_path, file_hash, actor_type, actor_id, actor_name,
		 actor_details, risk_level, classification, source, details, timestamp, alert_sent)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event.ProjectID, event.EventType, event.FilePath, event.FileHash,
		event.ActorType, event.ActorID, event.ActorName, event.ActorDetails,
		event.RiskLevel, event.Classification, event.Source, details, ts, alertSent)

	return err
}

// GetByProjectID retrieves FIM events with filters
func (r *FIMEventRepository) GetByProjectID(ctx context.Context, projectID int, filters repository.FIMEventFilters) ([]*models.FIMEvent, int, error) {
	filters.Validate()

	// Build conditions
	conditions := "WHERE project_id = ?"
	args := []interface{}{projectID}

	if filters.EventType != "" && filters.EventType != "all" {
		conditions += " AND event_type = ?"
		args = append(args, filters.EventType)
	}
	if filters.RiskLevel != "" && filters.RiskLevel != "all" {
		conditions += " AND risk_level = ?"
		args = append(args, filters.RiskLevel)
	}
	if filters.Classification != "" && filters.Classification != "all" {
		conditions += " AND classification = ?"
		args = append(args, filters.Classification)
	}
	if filters.Search != "" {
		conditions += " AND (file_path LIKE ? OR actor_name LIKE ?)"
		pattern := "%" + filters.Search + "%"
		args = append(args, pattern, pattern)
	}

	// Count total
	var total int
	countQuery := "SELECT COUNT(*) FROM fim_events " + conditions
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Get paginated results
	query := `
		SELECT id, project_id, event_type, file_path, file_hash, actor_type,
		       COALESCE(actor_id, ''), COALESCE(actor_name, ''), COALESCE(actor_details, ''),
		       risk_level, classification, source, COALESCE(details, ''),
		       alert_sent, datetime(timestamp, 'unixepoch'), datetime(created_at, 'unixepoch')
		FROM fim_events ` + conditions + " ORDER BY timestamp DESC LIMIT ? OFFSET ?"

	args = append(args, filters.Limit, filters.Offset())
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var events []*models.FIMEvent
	for rows.Next() {
		var e models.FIMEvent
		var actorID, actorName, actorDetails, details string
		var alertSentInt int
		var timestamp, createdAt sql.NullString

		err := rows.Scan(
			&e.ID, &e.ProjectID, &e.EventType, &e.FilePath, &e.FileHash,
			&e.ActorType, &actorID, &actorName, &actorDetails,
			&e.RiskLevel, &e.Classification, &e.Source, &details,
			&alertSentInt, &timestamp, &createdAt)
		if err != nil {
			return nil, 0, err
		}

		e.ActorID = actorID
		e.ActorName = actorName
		e.ActorDetails = actorDetails
		e.Details = details
		e.AlertSent = alertSentInt == 1
		if timestamp.Valid {
			e.Timestamp = timestamp.String
		}
		if createdAt.Valid {
			e.CreatedAt = createdAt.String
		}
		events = append(events, &e)
	}

	if events == nil {
		events = []*models.FIMEvent{}
	}
	return events, total, rows.Err()
}

// GetStats returns FIM event statistics
func (r *FIMEventRepository) GetStats(ctx context.Context, projectID int) (*repository.FIMStats, error) {
	stats := &repository.FIMStats{}

	// All time
	r.db.QueryRowContext(ctx, `
		SELECT COUNT(*),
		       COALESCE(SUM(CASE WHEN risk_level IN ('HIGH', 'CRITICAL') THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN risk_level = 'CRITICAL' THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN classification = 'UNKNOWN_SOURCE' THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN alert_sent = 1 THEN 1 ELSE 0 END), 0)
		FROM fim_events WHERE project_id = ?`, projectID).Scan(
		&stats.Events, &stats.HighRisk, &stats.CriticalRisk, &stats.UnknownSrc, &stats.AlertsSent)

	return stats, nil
}
