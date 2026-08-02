package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"golang.org/x/crypto/bcrypt"
)

// Config holds all environment configuration
type Config struct {
	Port                  string
	Host                  string
	SecretKey             string
	DBPath                string
	Locale                string
	FIMBufferSize         int
	FIMBatchIntervalMs    int
	FIMDebounceMs         int
	FIMOJSLookupTimeoutMs int
	LogLevel              string
	CORSOrigins           []string
	SessionTimeoutMinutes int
	MaxFileSizeMB         int
	OJSConnectionTimeout  int
	// Configurable timeouts
	HTTPReadTimeoutSecs   int
	HTTPWriteTimeoutSecs  int
	HTTPIdleTimeoutSecs   int
	DBQueryTimeoutSecs    int
	ScanTimeoutHours      int
}

var cfg Config

func loadConfig() {
	cfg = Config{
		Port:                  getEnv("PORT", "8080"),
		Host:                  getEnv("HOST", "0.0.0.0"),
		SecretKey:             getEnv("SECRET_KEY", "default-secret-key-change-me"),
		DBPath:                getEnv("DB_PATH", "./data/ojs_monitor.db"),
		Locale:                getEnv("LOCALE", "id-ID"),
		FIMBufferSize:         getEnvInt("FIM_BUFFER_SIZE", 1000),
		FIMBatchIntervalMs:    getEnvInt("FIM_BATCH_INTERVAL_MS", 1000),
		FIMDebounceMs:         getEnvInt("FIM_DEBOUNCE_MS", 500),
		FIMOJSLookupTimeoutMs: getEnvInt("FIM_OJS_LOOKUP_TIMEOUT_MS", 5000),
		LogLevel:              getEnv("LOG_LEVEL", "info"),
		SessionTimeoutMinutes: getEnvInt("SESSION_TIMEOUT_MINUTES", 60),
		MaxFileSizeMB:         getEnvInt("MAX_FILE_SIZE_MB", 100),
		OJSConnectionTimeout: getEnvInt("OJS_DB_TIMEOUT_SECONDS", 10),
		// Configurable timeouts
		HTTPReadTimeoutSecs:  getEnvInt("HTTP_READ_TIMEOUT_SECS", 30),
		HTTPWriteTimeoutSecs: getEnvInt("HTTP_WRITE_TIMEOUT_SECS", 30),
		HTTPIdleTimeoutSecs:  getEnvInt("HTTP_IDLE_TIMEOUT_SECS", 60),
		DBQueryTimeoutSecs:   getEnvInt("DB_QUERY_TIMEOUT_SECS", 30),
		ScanTimeoutHours:     getEnvInt("SCAN_TIMEOUT_HOURS", 2),
	}

	// Parse CORS origins
	corsEnv := getEnv("CORS_ORIGINS", "http://localhost:3000,http://127.0.0.1:3000")
	cfg.CORSOrigins = strings.Split(corsEnv, ",")
	for i, origin := range cfg.CORSOrigins {
		cfg.CORSOrigins[i] = strings.TrimSpace(origin)
	}

	log.Printf("Config loaded: Port=%s, DB=%s, Locale=%s, ScanTimeout=%dh", cfg.Port, cfg.DBPath, cfg.Locale, cfg.ScanTimeoutHours)
}

// sanitizeForLog replaces sensitive values with placeholder
func sanitizeForLog(s string) string {
	if len(s) > 4 {
		return s[:2] + "****" + s[len(s)-2:]
	}
	return "****"
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func main() {
	// Load configuration from environment
	loadConfig()

	// Initialize watcher config
	initWatcherConfig()

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
		AllowedOrigins:   cfg.CORSOrigins, // Use configured origins from CORS_ORIGINS env
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		AllowCredentials: true,
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
			r.Post("/projects/{id}/scan/force", handleForceScan) // Force scan - cancels existing jobs first
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
	addr := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  time.Duration(cfg.HTTPReadTimeoutSecs) * time.Second,
		WriteTimeout: time.Duration(cfg.HTTPWriteTimeoutSecs) * time.Second,
		IdleTimeout:  time.Duration(cfg.HTTPIdleTimeoutSecs) * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Starting OJS Security Monitor Backend on %s\n", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// Give outstanding requests 30 seconds to complete
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Stop FIM watchers
	StopAllFIMWatchers()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited gracefully")
}
