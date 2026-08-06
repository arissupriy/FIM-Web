// Package main is the entry point for the FIM Monitor HTTP server.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	infraalert "ojs-monitor/backend/internal/infrastructure/alert"
	infraacl "ojs-monitor/backend/internal/infrastructure/acl"
	"ojs-monitor/backend/internal/infrastructure/audit"
	"ojs-monitor/backend/internal/infrastructure/auth"
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
	"ojs-monitor/backend/internal/application/usecase/alert"
	"ojs-monitor/backend/internal/templates/ojs"
	"ojs-monitor/backend/internal/wire"
)

var (
	portFlag    = flag.String("port", "", "Server port (default: 8080)")
	hostFlag    = flag.String("host", "", "Server host (default: 0.0.0.0)")
)

func main() {
	// Parse flags
	flag.Parse()

	// Get port from flag, env, or default
	port := *portFlag
	if port == "" {
		port = os.Getenv("PORT")
		if port == "" {
			port = "8080"
		}
	}

	// Get host from flag, env, or default
	host := *hostFlag
	if host == "" {
		host = os.Getenv("HOST")
		if host == "" {
			host = "0.0.0.0"
		}
	}

	// Check for --help flag
	for _, arg := range os.Args {
		if arg == "--help" || arg == "-h" {
			printUsage()
			return
		}
	}

	addr := fmt.Sprintf("%s:%s", host, port)
	fmt.Printf("Starting OJS Security Monitor Backend on %s\n", addr)
	fmt.Println("Press Ctrl+C to stop")
	fmt.Println()

	// Initialize Database
	db, err := wire.InitDB()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

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
	alertConfigRepo := wire.AlertConfig()
	alertHistoryRepo := wire.AlertHistory()

	// Create and start alert dispatcher
	dispatcher := infraalert.NewDispatcher(alertConfigRepo, alertHistoryRepo)
	// Register alert channels
	dispatcher.RegisterChannel("email", &infraalert.EmailChannel{})
	dispatcher.RegisterChannel("slack", &infraalert.SlackChannel{})
	dispatcher.RegisterChannel("webhook", &infraalert.WebhookChannel{})
	// Connect watcher to alert dispatcher
	watcher.SetAlertDispatcher(dispatcher)
	// Start the dispatcher
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go dispatcher.Start(ctx)
	defer dispatcher.Stop()

	// Use cases
	projectUC := project.New(projectRepo)
	scanUC := scan.New(projectRepo, jobRepo, fileRepo)
	fimUC := fim.New(fimEventRepo)
	jobUC := job.New(jobRepo)
	fileUC := file.New(fileRepo, projectRepo)
	authUC := appauth.New(authRepo)
	alertUC := alert.NewUsecase(alertConfigRepo, alertHistoryRepo)

	// Auth service
	authService := auth.New(auth.DefaultConfig())

	// Handlers
	projectHandler := handlers.NewProjectHandler(projectUC)
	scanHandler := handlers.NewScanHandler(scanUC)
	fimHandler := handlers.NewFIMHandler(projectRepo, fimEventRepo)
	auditHandler := audit.NewAuditHandler(fimEventRepo, projectRepo)
	aclHandler := infraacl.NewACLHandler(fimEventRepo, projectRepo)
	jobHandler := handlers.NewJobHandler(jobUC)
	fileHandler := handlers.NewFileHandler(fileUC)
	authHandler := handlers.NewAuthHandler(authUC, authService)
	alertHandler := handlers.NewAlertHandler(alertUC)

	// OJS template handler
	ojsHandler := ojs.NewHandler(projectRepo, fileRepo)

	// Create router with auth
	router := infrahttp.NewRouter(infrahttp.RouterConfig{
		ProjectHandler: projectHandler,
		ScanHandler:    scanHandler,
		FIMHandler:     fimHandler,
		AuditHandler:   auditHandler,
		ACLHandler:    aclHandler,
		AuthHandler:    authHandler,
		JobHandler:     jobHandler,
		FileHandler:    fileHandler,
		AlertHandler:   alertHandler,
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
	_ = alertUC

	// Start Server
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

func printUsage() {
	fmt.Println("OJS Monitor HTTP Server")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  ./fim-server              Start the HTTP server")
	fmt.Println("  ./fim-server --port 9000  Start on port 9000")
	fmt.Println("  ./fim-server --host 127.0.0.1  Bind to localhost")
	fmt.Println("  ./fim-server --port 9000 --host 0.0.0.0")
	fmt.Println()
	fmt.Println("Environment Variables:")
	fmt.Println("  PORT=8080                Set HTTP server port")
	fmt.Println("  HOST=0.0.0.0              Set bind address")
	fmt.Println()
	fmt.Println("CLI Commands:")
	fmt.Println("  Use './manage' for database commands")
}
