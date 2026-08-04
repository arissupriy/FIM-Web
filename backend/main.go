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

	infraauth "ojs-monitor/backend/internal/infrastructure/auth"
	"ojs-monitor/backend/internal/infrastructure/database/sqlite"
	infrahttp "ojs-monitor/backend/internal/infrastructure/http"
	"ojs-monitor/backend/internal/infrastructure/http/handlers"
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
	// Initialize Database
	initDB()

	// Initialize repositories (for wire package access)
	wire.Init(db)

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

	// Mount the new API router
	r.Mount("/", router)

	// Suppress unused variable warnings
	_ = fimUC
	_ = jobUC
	_ = fileUC
	_ = authUC

	// Start Server
	port := ":8080"
	log.Printf("Starting OJS Security Monitor Backend on %s\n", port)
	if err := http.ListenAndServe(port, r); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
