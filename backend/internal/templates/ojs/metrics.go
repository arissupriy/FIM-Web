// Package ojs provides OJS-specific template implementation.
package ojs

import (
	"context"

	"ojs-monitor/backend/internal/domain/template"
	"ojs-monitor/backend/internal/templates/ojs/mysql"
)

// getMetrics returns OJS-specific dashboard metrics.
func getMetrics(ctx context.Context, db template.DBConnection) (*template.TemplateMetrics, error) {
	m := template.NewTemplateMetrics("ojs", "3.x")

	ojsConn, ok := db.(*mysql.Connection)
	if !ok {
		return m, nil
	}

	// Get metrics from mysql package
	metrics, err := ojsConn.GetMetrics(ctx)
	if err == nil {
		m.Specific["new_users_24h"] = metrics.NewUsers
		m.Specific["active_admins_7d"] = metrics.ActiveAdmins
		m.Specific["uploads_by_new_users"] = metrics.UploadsByNewUsers
		m.Specific["bad_self_reg"] = metrics.BadSelfReg
	}

	// Get counts
	if users, err := ojsConn.GetUserCount(ctx); err == nil {
		m.Specific["total_users"] = users
	}
	if submissions, err := ojsConn.GetSubmissionCount(ctx); err == nil {
		m.Specific["total_submissions"] = submissions
	}
	if journals, err := ojsConn.GetJournalCount(ctx); err == nil {
		m.Specific["total_journals"] = journals
	}

	return m, nil
}
