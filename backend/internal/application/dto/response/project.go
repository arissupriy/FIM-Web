// Package response contains HTTP response DTOs.
package response

import (
	"ojs-monitor/backend/internal/domain/models"
)

// ProjectResponse wraps project with optional metrics
type ProjectResponse struct {
	Project  *models.Project     `json:"project"`
	Metrics *DashboardResponse `json:"metrics,omitempty"`
}

// DashboardResponse wraps dashboard metrics
type DashboardResponse struct {
	Status           string `json:"status"`
	BaselineTotal    int    `json:"baseline_total"`
	Files           *FileStats    `json:"files,omitempty"`
	OJS             *OJSStats    `json:"ojs,omitempty"`
}

// FileStats holds file statistics
type FileStats struct {
	Added    int `json:"added"`
	Modified int `json:"modified"`
	Deleted  int `json:"deleted"`
	Orphan   int `json:"orphan"`
	Total    int `json:"total"`
}

// OJSStats holds OJS-specific stats
type OJSStats struct {
	Users    int `json:"users,omitempty"`
	Submissions int    `json:"submissions,omitempty"`
	Articles int    `json:"articles,omitempty"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
