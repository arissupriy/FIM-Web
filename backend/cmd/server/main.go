// Package main is the entry point for the FIM Monitor HTTP server.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	infraauth "ojs-monitor/backend/internal/infrastructure/auth"
	"ojs-monitor/backend/internal/infrastructure/database/sqlite"
	infrahttp "ojs-monitor/backend/internal/infrastructure/http"
	"ojs-monitor/backend/internal/infrastructure/http/handlers"
	"ojs-monitor/backend/internal/infrastructure/worker"
	"ojs-monitor/backend/internal/infrastructure/watcher"
	appauth "ojs-monitor/backend/internal/application/usecase/auth"
	"ojs-monitor/backend/internal/application/usecase/project"
	"ojs-monitor/backend/internal/application/usecase/scan"
	"ojs-monitor/backend/internal/application/usecase/fim"
	"ojs-monitor/backend/internal/application/usecase/job"
	"ojs-monitor/backend/internal/application/usecase/file"
	"ojs-monitor/backend/internal/templates/ojs"
	"ojs-monitor/backend/internal/wire"
)

func main() {
	// Get port from environment or default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Check for --help flag
	for _, arg := range os.Args {
		if arg == "--help" || arg == "-h" {
			printUsage()
			return
		}
	}

	fmt.Printf("Starting OJS Security Monitor Backend on :%s\n", port)
	fmt.Println("Press Ctrl+C to stop")
	fmt.Println()

	// Initialize Database
	db := wire.InitDB()

	// Seed default admin if none exists
	if err := wire.SeedDefaultAdmin(context.Background()); err != nil {
		log.Printf("Warning: failed to seed default admin: %v", err)
	}

	// Start Async Worker
	go worker.StartWorker()

	// Restore FIM watchers for active projects
	go watcher.RestoreWatchersOnStartup()

	// Wire up clean architecture handlers
	sqliteDB := sqlite.NewDB(db)

	// Repositories
	projectRepo := sqlite.NewProjectRepository(sqliteDB)
	jobRepo := sqlite.NewJobRepository(sqliteDB)
	fileRepo := sqlite.NewFileRepository(sqliteDB)
	fimEventRepo := sqlite.NewFIMEventRepository(sqliteDB)
	authRepo := sqlite.NewAuthRepository(sqliteDB)

	// Use cases
	projectUC := project.New(projectRepo)
	scanUC := scan.New(projectRepo, jobRepo, fileRepo)
	fimUC := fim.New(fimEventRepo)
	jobUC := job.New(jobRepo)
	fileUC := file.New(fileRepo, projectRepo)
	authUC := appauth.New(authRepo)

	// Auth service
	authService := infraauth.New(infraauth.DefaultConfig())

	// Handlers
	projectHandler := handlers.NewProjectHandler(projectUC)
	scanHandler := handlers.NewScanHandler(scanUC)
	fimHandler := handlers.NewFIMHandler(projectRepo, fimEventRepo)
	jobHandler := handlers.NewJobHandler(jobUC)
	fileHandler := handlers.NewFileHandler(fileUC)
	authHandler := handlers.NewAuthHandler(authUC, authService)

	// OJS template handler
	ojsHandler := ojs.NewHandler(projectRepo, fileRepo)

	// Create router with auth
	router := infrahttp.NewRouter(infrahttp.RouterConfig{
		ProjectHandler: projectHandler,
		ScanHandler:    scanHandler,
		FIMHandler:     fimHandler,
		AuthHandler:    authHandler,
		JobHandler:     jobHandler,
		FileHandler:    fileHandler,
		OJSHandler:     ojsHandler,
		ValidateToken:  authService.ValidateTokenFunc(),
	})

	// Wrap with Chi router for additional middleware
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"http://localhost:3000"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
	}))

	// Mount the API router
	r.Mount("/", router)

	// Suppress unused variable warnings
	_ = fimUC
	_ = jobUC
	_ = fileUC
	_ = authUC
	_ = projectUC
	_ = scanUC

	// Start Server
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

func printUsage() {
	fmt.Println("OJS Monitor HTTP Server")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  ./fim-server              Start the HTTP server")
	fmt.Println()
	fmt.Println("Environment Variables:")
	fmt.Println("  PORT=8080                Set HTTP server port (default: 8080)")
	fmt.Println()
	fmt.Println("CLI Commands:")
	fmt.Println("  Use './manage' for database commands")
}
