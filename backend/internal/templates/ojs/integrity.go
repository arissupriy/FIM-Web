// Package ojs provides OJS-specific template implementation.
package ojs

import (
	"context"

	"ojs-monitor/backend/internal/domain/models"
	"ojs-monitor/backend/internal/domain/template"
	"ojs-monitor/backend/internal/templates/ojs/mysql"
)

// validateIntegrity checks OJS-specific security policies.
func validateIntegrity(ctx context.Context, db template.DBConnection, p *models.Project) ([]template.IntegrityWarning, error) {
	var warnings []template.IntegrityWarning

	ojsConn, ok := db.(*mysql.Connection)
	if !ok {
		return warnings, nil
	}

	w, err := ojsConn.GetIntegrityWarnings(ctx)
	if err != nil {
		return warnings, nil
	}

	if w.SelfRegistration > 0 {
		warnings = append(warnings, template.IntegrityWarning{
			Level:   template.WarningMedium,
			Code:    "OJS_SELF_REGISTRATION",
			Message: "Self-registration enabled for non-default roles",
			Details: "Users can self-register as reviewer/editor",
		})
	}

	if w.UnvalidatedUsers > 0 {
		warnings = append(warnings, template.IntegrityWarning{
			Level:   template.WarningLow,
			Code:    "OJS_UNVALIDATED_USERS",
			Message: "Users without email validation",
			Details: "Some users have not validated their email",
		})
	}

	return warnings, nil
}
