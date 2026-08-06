// Package http provides HTTP server infrastructure.
package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"

	"ojs-monitor/backend/internal/infrastructure/acl"
	"ojs-monitor/backend/internal/infrastructure/audit"
	"ojs-monitor/backend/internal/infrastructure/http/handlers"
	httpMiddleware "ojs-monitor/backend/internal/infrastructure/http/middleware"
	"ojs-monitor/backend/internal/templates/ojs"
)

// RouterConfig holds handler dependencies.
type RouterConfig struct {
	ProjectHandler *handlers.ProjectHandler
	ScanHandler    *handlers.ScanHandler
	FIMHandler    *handlers.FIMHandler
	AuditHandler  *audit.AuditHandler
	ACLHandler    *acl.ACLHandler
	AuthHandler   *handlers.AuthHandler
	JobHandler    *handlers.JobHandler
	FileHandler   *handlers.FileHandler
	AlertHandler  *handlers.AlertHandler

	// Template handlers
	OJSHandler *ojs.Handler

	// Auth configuration
	ValidateToken func(token string) (int, string, error)
}

// NewRouter creates a chi router with all routes configured.
func NewRouter(cfg RouterConfig) *chi.Mux {
	r := chi.NewRouter()

	// Global middleware
	r.Use(chiMiddleware.Logger)
	r.Use(chiMiddleware.Recoverer)
	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// API v1 routes
	r.Route("/api/v1", func(r chi.Router) {
		// Public routes
		r.Post("/auth/login", cfg.AuthHandler.Login)
		r.Get("/auth/logs", cfg.AuthHandler.GetAuditLogs) // Auth logs are public info

		// Protected routes
		r.Group(func(r chi.Router) {
			if cfg.ValidateToken != nil {
				r.Use(httpMiddleware.RequireAuth(cfg.ValidateToken))
			}

			// Projects
			r.Route("/projects", func(r chi.Router) {
				r.Get("/", cfg.ProjectHandler.List)
				r.Post("/", cfg.ProjectHandler.Create)
				r.Get("/{id}", cfg.ProjectHandler.Get)
				r.Put("/{id}", cfg.ProjectHandler.Update)
				r.Delete("/{id}", cfg.ProjectHandler.Delete)

				// Jobs
				r.Get("/{id}/jobs", cfg.JobHandler.List)
				r.Get("/{id}/jobs/stats", cfg.JobHandler.Stats)

				// Scan endpoints
				r.Post("/{id}/scan/baseline", cfg.ScanHandler.StartBaseline)
				r.Post("/{id}/scan/baseline/reset", cfg.ScanHandler.Reset)
				r.Post("/{id}/scan/integrity", cfg.ScanHandler.StartIntegrity)
				r.Post("/{id}/scan/cancel/{jobID}", cfg.ScanHandler.Cancel)
			})

			// Jobs management
			r.Delete("/jobs/{jobID}", cfg.JobHandler.Cancel)

			// Files
			r.Route("/projects/{id}/files", func(r chi.Router) {
				r.Get("/", cfg.FileHandler.List)
				r.Get("/orphans", cfg.FileHandler.GetOrphans)
				r.Get("/stats", cfg.FileHandler.GetStats)
				r.Get("/{fileId}", cfg.FileHandler.Get)
			})

			// FIM endpoints
			r.Route("/fim", func(r chi.Router) {
				r.Get("/projects/{id}/status", cfg.FIMHandler.GetStatus)
				r.Get("/projects/{id}/events", cfg.FIMHandler.GetEvents)
				r.Get("/projects/{id}/events/stats", cfg.FIMHandler.GetEventStats)
				r.Get("/projects/{id}/watcher/status", cfg.FIMHandler.GetWatcherStatus)
				r.Post("/projects/{id}/watcher/start", cfg.FIMHandler.StartWatcher)
				r.Post("/projects/{id}/watcher/stop", cfg.FIMHandler.StopWatcher)

				// Auditd ingestion
				if cfg.AuditHandler != nil {
					r.Post("/audit/ingest", cfg.AuditHandler.IngestEvents)
					r.Get("/audit/status", cfg.AuditHandler.GetStatus)
				}

				// ACL monitoring
				if cfg.ACLHandler != nil {
					r.Post("/acl/ingest", cfg.ACLHandler.IngestChanges)
					r.Post("/acl/scan", cfg.ACLHandler.ScanACLs)
					r.Get("/acl/status", cfg.ACLHandler.GetStatus)
				}
			})

			// Alert endpoints
			r.Route("/projects/{project_id}/alerts", func(r chi.Router) {
				r.Get("/", cfg.AlertHandler.ListAlertConfigs)
				r.Get("/history", cfg.AlertHandler.ListProjectAlertHistory)
				r.Get("/stats", cfg.AlertHandler.GetAlertStats)
			})

			r.Route("/alerts", func(r chi.Router) {
				r.Post("/", cfg.AlertHandler.CreateAlertConfig)
				r.Get("/{id}", cfg.AlertHandler.GetAlertConfig)
				r.Put("/{id}", cfg.AlertHandler.UpdateAlertConfig)
				r.Delete("/{id}", cfg.AlertHandler.DeleteAlertConfig)
				r.Post("/{id}/enable", cfg.AlertHandler.EnableAlertConfig)
				r.Post("/{id}/disable", cfg.AlertHandler.DisableAlertConfig)
				r.Get("/{id}/history", cfg.AlertHandler.GetAlertHistory)
				r.Post("/{id}/test", cfg.AlertHandler.TestAlertConfig)
			})

			// OJS template endpoints
			if cfg.OJSHandler != nil {
				r.Route("/projects/{id}/ojs", func(r chi.Router) {
					r.Get("/details", cfg.OJSHandler.GetDetails)
					r.Get("/full-details", cfg.OJSHandler.GetFullDetails)
					r.Get("/metrics", cfg.OJSHandler.GetMetrics)
					r.Get("/audit", cfg.OJSHandler.GetAudit)
					r.Get("/validate", cfg.OJSHandler.ValidateIntegrity)
					r.Post("/test-connection", cfg.OJSHandler.TestConnection)
					r.Get("/files/{fileId}/relations", cfg.OJSHandler.GetFileRelations)
				})
			}
		})
	})

	return r
}
