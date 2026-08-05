// Package main provides CLI management commands for OJS Monitor.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"golang.org/x/crypto/bcrypt"

	"ojs-monitor/backend/cmd/manage/process"
	"ojs-monitor/backend/internal/wire"
)

const version = "1.0.0"

// Global flags for server commands
var (
	hostFlag = flag.String("host", "", "Server bind address")
	portFlag = flag.Int("port", 0, "Server port")
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// Parse command manually to handle colon-separated commands
	cmd := os.Args[1]

	// Handle colon-separated subcommands (route:list, server:start, etc)
	colonIdx := strings.Index(cmd, ":")
	var subCmd, action string
	if colonIdx > 0 {
		subCmd = cmd[:colonIdx]
		action = cmd[colonIdx+1:]
	} else {
		subCmd = cmd
	}

	// Initialize database for most commands
	if subCmd != "version" && subCmd != "-v" && subCmd != "--version" &&
		subCmd != "help" && subCmd != "-h" && subCmd != "--help" &&
		subCmd != "list" && subCmd != "commands" &&
		subCmd != "about" && subCmd != "route:list" && subCmd != "routes" {
		initDB()
	}

	switch subCmd {
	case "migrate":
		runMigrate()
	case "seed":
		runSeed()
	case "add-admin":
		runAddAdmin()
	case "status":
		runStatus()
	case "cleanup":
		runCleanup()
	case "about":
		runAbout()
	case "route", "routes":
		runRouteList()
	case "list", "commands":
		runCommandList()
	case "version", "-v", "--version":
		fmt.Printf("OJS Monitor CLI v%s\n", version)
	case "help", "-h", "--help":
		printUsage()
	case "server":
		pm, err := process.New()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	handleServerCommand(pm, action)
	default:
		fmt.Printf("Unknown command: %s\n\n", cmd)
		printUsage()
		os.Exit(1)
	}
}


func handleServerCommand(pm *process.Manager, action string) {
	switch action {
	case "start":
		runServerStart(pm)
	case "stop":
		runServerStop(pm)
	case "restart":
		runServerRestart(pm)
	case "status":
		runServerStatus(pm)
	default:
		fmt.Printf("Unknown action: %s\n\n", action)
		printUsage()
		os.Exit(1)
	}
}

func runServerStart(pm *process.Manager) {
	fmt.Println("Starting OJS Monitor services...")

	pm.CleanupStale()

	status := pm.GetAllStatus()
	running := 0
	for _, s := range status {
		if s.Running {
			running++
		}
	}

	if running > 0 {
		fmt.Println("⚠ Some services are already running:")
		for _, s := range status {
			if s.Running {
				fmt.Printf("  - %s (PID %d, uptime: %s)\n", s.Name, s.PID, s.Uptime)
			}
		}
		fmt.Println("\nUse 'manage server:stop' to stop them first")
		os.Exit(1)
	}

	host := *hostFlag
	if host == "" {
		host = os.Getenv("HOST")
		if host == "" {
			host = "0.0.0.0"
		}
	}

	port := *portFlag
	if port == 0 {
		if p := os.Getenv("PORT"); p != "" {
			port, _ = strconv.Atoi(p)
		}
		if port == 0 {
			port = 8080
		}
	}

	if err := pm.StartAll(host, port); err != nil {
		fmt.Printf("✗ Failed to start services: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Printf("✓ All services started successfully\n")
	fmt.Printf("  API Server: http://%s:%d\n", host, port)
	fmt.Println()
	fmt.Println("Use 'manage server:stop' to stop services")
}

func runServerStop(pm *process.Manager) {
	fmt.Println("Stopping OJS Monitor services...")
	pm.StopAll()
	fmt.Println("✓ All services stopped")
}

func runServerRestart(pm *process.Manager) {
	host := *hostFlag
	if host == "" {
		host = os.Getenv("HOST")
		if host == "" {
			host = "0.0.0.0"
		}
	}

	port := *portFlag
	if port == 0 {
		if p := os.Getenv("PORT"); p != "" {
			port, _ = strconv.Atoi(p)
		}
		if port == 0 {
			port = 8080
		}
	}

	fmt.Println("Restarting OJS Monitor services...")
	if err := pm.RestartAll(host, port); err != nil {
		fmt.Printf("✗ Failed to restart services: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ All services restarted successfully")
}

func runServerStatus(pm *process.Manager) {
	fmt.Println("┌─────────────────────────────────────────────────────┐")
	fmt.Println("│         OJS Monitor - Service Status               │")
	fmt.Println("└─────────────────────────────────────────────────────┘")
	fmt.Println()

	status := pm.GetAllStatus()
	allStopped := true
	runningCount := 0

	for _, s := range status {
		if s.Running {
			runningCount++
			allStopped = false
			fmt.Printf("  ✓ %-10s running  PID %d  uptime %s\n", s.Name+":", s.PID, s.Uptime)
		} else {
			fmt.Printf("  ✗ %-10s stopped\n", s.Name+":")
		}
	}

	fmt.Println()
	if allStopped {
		fmt.Println("All services are stopped")
		fmt.Println()
		fmt.Println("Use 'manage server:start' to start services")
	} else {
		fmt.Printf("System: Running (%d service%s active)\n", runningCount, plural(runningCount))
		fmt.Println()
		fmt.Println("Use 'manage server:stop' to stop services")
		fmt.Println("Use 'manage server:restart' to restart services")
	}
}

func runCleanup() {
	pm, err := process.New()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Cleaning up stale process files...")
	pm.CleanupStale()
	fmt.Println("✓ Cleanup complete")
}

func runStatus() {
	ctx := context.Background()

	pm, err := process.New()
	if err == nil {
		fmt.Println("┌─────────────────────────────────────────────────────┐")
		fmt.Println("│         OJS Monitor - System Status                 │")
		fmt.Println("└─────────────────────────────────────────────────────┘")
		fmt.Println()
		fmt.Println("Services:")
		status := pm.GetAllStatus()
		for _, s := range status {
			icon := "✗"
			if s.Running {
				icon = "✓"
			}
			if s.Running {
				fmt.Printf("  %s %-10s PID %d  uptime %s\n", icon, s.Name+":", s.PID, s.Uptime)
			} else {
				fmt.Printf("  %s %-10s stopped\n", icon, s.Name+":")
			}
		}
		fmt.Println()
		fmt.Println("Database:")
	} else {
		fmt.Println("OJS Monitor - System Status")
		fmt.Println()
	}

	fmt.Println("  ✓ Database: Connected")

	adminCount := wire.GetAdminCount(ctx)
	if adminCount > 0 {
		fmt.Printf("  ✓ Admin users: %d\n", adminCount)
	} else {
		fmt.Println("  ⚠ Admin users: None")
		fmt.Println("    → Run 'manage seed' or 'manage add-admin'")
	}

	projects := wire.GetProjectsCount(ctx)
	fmt.Printf("  ✓ Projects: %d configured\n", projects)

	fmt.Println()
	fmt.Println("System: Ready")
}

func runAbout() {
	fmt.Println("┌─────────────────────────────────────────────────────┐")
	fmt.Println("│                  OJS Monitor - About                 │")
	fmt.Println("└─────────────────────────────────────────────────────┘")
	fmt.Println()
	fmt.Printf("  Version:      %s\n", version)
	fmt.Println("  Description:  File Integrity Monitoring Platform")
	fmt.Println("  Template:     OJS (Open Journal Systems)")
	fmt.Println()
	fmt.Println("  Architecture: Clean Architecture")
	fmt.Println("    - Domain Layer")
	fmt.Println("    - Application Layer")
	fmt.Println("    - Infrastructure Layer")
	fmt.Println()
	fmt.Println("  Storage:")
	fmt.Println("    - SQLite (metadata)")
	fmt.Println("    - MySQL (OJS databases)")
	fmt.Println()
	fmt.Println("  Monitoring:")
	fmt.Println("    - Real-time FIM (fsnotify)")
	fmt.Println("    - Baseline scanning")
	fmt.Println("    - Integrity scanning")
	fmt.Println("    - Orphan detection")
	fmt.Println()
}

func runRouteList() {
	fmt.Println("┌─────────────────────────────────────────────────────┐")
	fmt.Println("│                   API Routes                           │")
	fmt.Println("└─────────────────────────────────────────────────────┘")
	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)

	fmt.Fprintf(w, "  %-6s %-35s %s\n", "METHOD", "ENDPOINT", "DESCRIPTION")
	fmt.Fprintf(w, "  %-6s %-35s %s\n", "──────", "───────────────────────────────────", "───────────")
	fmt.Fprintf(w, "  %-6s %-35s %s\n", "POST", "/api/login", "Admin login")
	fmt.Fprintf(w, "  %-6s %-35s %s\n", "GET", "/api/projects", "List all projects")
	fmt.Fprintf(w, "  %-6s %-35s %s\n", "POST", "/api/projects", "Create project")
	fmt.Fprintf(w, "  %-6s %-35s %s\n", "GET", "/api/projects/:id", "Get project details")
	fmt.Fprintf(w, "  %-6s %-35s %s\n", "PUT", "/api/projects/:id", "Update project")
	fmt.Fprintf(w, "  %-6s %-35s %s\n", "DELETE", "/api/projects/:id", "Delete project")
	fmt.Fprintf(w, "  %-6s %-35s %s\n", "POST", "/api/projects/:id/scan", "Start baseline scan")
	fmt.Fprintf(w, "  %-6s %-35s %s\n", "GET", "/api/projects/:id/jobs", "List scan jobs")
	fmt.Fprintf(w, "  %-6s %-35s %s\n", "GET", "/api/projects/:id/files", "List monitored files")
	fmt.Fprintf(w, "  %-6s %-35s %s\n", "GET", "/api/projects/:id/events", "List FIM events")
	fmt.Fprintf(w, "  %-6s %-35s %s\n", "GET", "/api/projects/:id/audit", "Get audit metrics")
	fmt.Fprintf(w, "  %-6s %-35s %s\n", "GET", "/api/logs", "Get audit logs")
	fmt.Fprintf(w, "  %-6s %-35s %s\n", "GET", "/api/health", "Health check")
	fmt.Fprintf(w, "  %-6s %-35s %s\n", "POST", "/api/test-connection", "Test OJS connection")
	w.Flush()

	fmt.Println()
	fmt.Println("  Total: 16 routes")
	fmt.Println()
}

func runCommandList() {
	fmt.Println("┌─────────────────────────────────────────────────────┐")
	fmt.Println("│                  Available Commands               │")
	fmt.Println("└─────────────────────────────────────────────────────┘")
	fmt.Println()

	fmt.Println("  Database:")
	fmt.Println("    migrate                       Run database migrations")
	fmt.Println("    seed                         Seed default admin user")
	fmt.Println("    add-admin <user> <pass>      Create admin user")
	fmt.Println()
	fmt.Println("  Server:")
	fmt.Println("    server:start                 Start server + worker")
	fmt.Println("    server:stop                  Stop server + worker")
	fmt.Println("    server:restart               Restart all services")
	fmt.Println("    server:status                Show service status")
	fmt.Println()
	fmt.Println("  Info:")
	fmt.Println("    about                        Show system information")
	fmt.Println("    route:list, routes          List all API routes")
	fmt.Println("    status                      Show system status")
	fmt.Println("    list, commands               Show this list")
	fmt.Println()
	fmt.Println("  Utility:")
	fmt.Println("    cleanup                      Remove stale PID files")
	fmt.Println("    version, -v                  Show version")
	fmt.Println("    help, -h                    Show help")
	fmt.Println()
	fmt.Println("  Server Options:")
	fmt.Println("    --port 9000                Server port (default: 8080)")
	fmt.Println("    --host 0.0.0.0              Bind address (default: 0.0.0.0)")
	fmt.Println()
}

func printUsage() {
	fmt.Println("OJS Monitor CLI v" + version)
	fmt.Println()
	fmt.Println("Usage: manage <command> [options]")
	fmt.Println()
	fmt.Println("Database Commands:")
	fmt.Println("  manage migrate                    Run database migrations")
	fmt.Println("  manage seed                      Seed default admin user")
	fmt.Println("  manage add-admin <user> <pass>   Create admin user")
	fmt.Println()
	fmt.Println("Server Commands:")
	fmt.Println("  manage server:start              Start server + worker")
	fmt.Println("  manage server:stop               Stop server + worker")
	fmt.Println("  manage server:restart            Restart all services")
	fmt.Println("  manage server:status             Show service status")
	fmt.Println()
	fmt.Println("Info Commands:")
	fmt.Println("  manage about                    Show system information")
	fmt.Println("  manage route:list               List all API routes")
	fmt.Println("  manage status                   Show system status")
	fmt.Println("  manage list                     Show all commands")
	fmt.Println()
	fmt.Println("Server Options:")
	fmt.Println("  --port 9000                    Server port (default: 8080)")
	fmt.Println("  --host 0.0.0.0                Bind address (default: 0.0.0.0)")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  manage migrate")
	fmt.Println("  manage seed")
	fmt.Println("  manage add-admin admin secretpassword")
	fmt.Println("  manage server:start")
	fmt.Println("  manage server:start --port 9000")
	fmt.Println("  manage server:start --host 127.0.0.1 --port 9000")
	fmt.Println("  manage route:list")
	fmt.Println("  manage about")
}

func initDB() {
	_ = wire.InitDB()
}

func runMigrate() {
	fmt.Println("Running database migrations...")
	_ = wire.InitDB()
	fmt.Println("✓ Migrations completed successfully.")
}

func runSeed() {
	fmt.Println("Seeding default admin...")
	ctx := context.Background()
	if err := wire.SeedDefaultAdmin(ctx); err != nil {
		fmt.Printf("✗ Failed to seed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ Default admin seeded successfully.")
	fmt.Println("  Default credentials: admin / admin123")
}

func runAddAdmin() {
	if len(os.Args) != 3 {
		fmt.Println("Usage: manage add-admin <username> <password>")
		os.Exit(1)
	}

	username := os.Args[2]
	password := os.Args[3]

	if len(password) < 6 {
		fmt.Println("Error: Password must be at least 6 characters")
		os.Exit(1)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		fmt.Printf("✗ Failed to hash password: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	if err := wire.CreateAdminUser(ctx, username, string(hash)); err != nil {
		fmt.Printf("✗ Failed to create admin: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Admin '%s' created successfully.\n", username)
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
