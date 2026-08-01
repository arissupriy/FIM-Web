package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	// Initialize Database
	initDB()

	// Seed default admin if none exists
	if err := SeedDefaultAdmin(); err != nil {
		log.Printf("Warning: failed to seed default admin: %v", err)
	}

	// Handle CLI commands
	if len(os.Args) > 1 {
		if os.Args[1] == "add-admin" {
			if len(os.Args) != 4 {
				fmt.Println("Usage: go run . add-admin <username> <password>")
				return
			}
			username := os.Args[2]
			password := os.Args[3]
			
			hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
			if err != nil {
				log.Fatalf("Failed to hash password: %v", err)
			}
			
			if err := CreateAdmin(username, string(hash)); err != nil {
				log.Fatalf("Failed to create admin: %v", err)
			}
			
			fmt.Printf("Admin '%s' successfully created.\n", username)
			return
		}
	}

	// Start Async Worker
	go StartWorker()

	// Restore FIM watchers for active projects (after worker is ready)
	go RestoreWatchersOnStartup()

	// Initialize Router
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"http://localhost:3000"}, // Allow Next.js frontend
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
	}))

	// API Routes
	r.Route("/api", func(r chi.Router) {
		r.Post("/login", handleLogin)
		
		// Protected Routes
		r.Group(func(r chi.Router) {
			r.Use(AuthMiddleware)
			r.Get("/projects", handleGetProjects)
			r.Get("/projects/{id}", handleGetProject)
			r.Post("/projects", handleAddProject)
			r.Put("/projects/{id}", handleUpdateProject)
			r.Post("/projects/{id}/scan", handleStartScan)
			r.Post("/projects/{id}/baseline/reset", handleResetBaseline) // Reset baseline
			r.Post("/projects/{id}/integrity-scan", handleStartIntegrityScan) // Manual integrity scan
			r.Post("/projects/{id}/test-connection", handleTestConnection)
			r.Get("/projects/{id}/jobs", handleGetProjectJobs)
			r.Get("/projects/{id}/details", handleGetProjectDetails)
			r.Delete("/jobs/{id}", handleCancelJob)
			r.Get("/audit/{id}", handleAuditProject)
			r.Get("/audit/{id}/files", handleGetProjectFiles) // old endpoint for summary
			r.Get("/projects/{id}/files", handleGetProjectFilesPaginated) // new paginated endpoint
			r.Get("/projects/{id}/files/{fileId}", handleGetFileDetail) // file detail endpoint
			r.Get("/projects/{id}/files/{fileId}/ojs-relations", handleGetFileOJSRelations) // OJS relations
			r.Get("/projects/{id}/files/stats", handleGetFIMStats) // FIM statistics
			r.Get("/projects/{id}/orphan-files", handleGetOrphanFiles) // orphan files endpoint
			r.Get("/projects/{id}/events", handleGetFIMEvents) // FIM forensic events
			r.Get("/projects/{id}/events/stats", handleGetFIMEventStats) // FIM event statistics
			r.Post("/projects/{id}/watcher/start", handleStartFIMWatcher) // start FIM watcher
			r.Post("/projects/{id}/watcher/stop", handleStopFIMWatcher) // stop FIM watcher
			r.Get("/projects/{id}/watcher/status", handleGetFIMWatcherStatus) // watcher status
			r.Get("/logs", handleGetLogs)
		})
	})

	// Start Server
	port := ":8080"
	log.Printf("Starting OJS Security Monitor Backend on %s\n", port)
	if err := http.ListenAndServe(port, r); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
