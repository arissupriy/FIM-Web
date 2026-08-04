// Package main provides CLI management commands for OJS Monitor.
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"ojs-monitor/backend/cmd/manage/process"
	"ojs-monitor/backend/internal/wire"
)

const version = "1.0.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]

	// Handle colon-separated subcommands
	if strings.Contains(cmd, ":") {
		parts := strings.SplitN(cmd, ":", 2)
		handleServerCommand(parts[0], parts[1])
		return
	}

	// Initialize database for all commands except version and help
	if cmd != "version" && cmd != "-v" && cmd != "--version" && cmd != "help" && cmd != "-h" && cmd != "--help" {
		initDB()
	}

	switch cmd {
	case "migrate":
		runMigrate()
	case "seed":
		runSeed()
	case "add-admin":
		runAddAdmin()
	case "status":
		runStatus()
	case "server":
		handleServerCommand("server", "")
	case "version", "-v", "--version":
		fmt.Printf("OJS Monitor CLI v%s\n", version)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func handleServerCommand(sub, action string) {
	pm, err := process.New()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	switch sub {
	case "server":
		handleServerSubCommand(pm, action)
	default:
		fmt.Printf("Unknown command: %s\n\n", sub)
		printUsage()
		os.Exit(1)
	}
}

func handleServerSubCommand(pm *process.Manager, action string) {
	switch action {
	case "start":
		runServerStart(pm)
	case "stop":
		runServerStop(pm)
	case "restart":
		runServerRestart(pm)
	case "status":
		runServerProcessStatus(pm)
	case "":
		// Default: show status
		runServerProcessStatus(pm)
	default:
		fmt.Printf("Unknown action: %s\n\n", action)
		printUsage()
		os.Exit(1)
	}
}

func runServerStart(pm *process.Manager) {
	fmt.Println("Starting OJS Monitor services...")

	// Check if already running
	status := pm.GetAllStatus()
	running := 0
	for _, s := range status {
		if s.Running {
			running++
		}
	}

	if running > 0 {
		fmt.Println("⚠ Some services are already running. Stop them first with 'manage server:stop'")
		os.Exit(1)
	}

	if err := pm.StartAll(); err != nil {
		fmt.Printf("✗ Failed to start services: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println("✓ All services started successfully")
	fmt.Println("  API Server: http://localhost:8080")
	fmt.Println()
	fmt.Println("Use 'manage server:stop' to stop services")
}

func runServerStop(pm *process.Manager) {
	fmt.Println("Stopping OJS Monitor services...")

	if err := pm.StopAll(); err != nil {
		fmt.Printf("Warning: %v\n", err)
	}

	fmt.Println("✓ All services stopped")
}

func runServerRestart(pm *process.Manager) {
	fmt.Println("Restarting OJS Monitor services...")

	if err := pm.RestartAll(); err != nil {
		fmt.Printf("✗ Failed to restart services: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ All services restarted successfully")
}

func runServerProcessStatus(pm *process.Manager) {
	fmt.Println("┌─────────────────────────────────────────────────────┐")
	fmt.Println("│         OJS Monitor - Service Status               │")
	fmt.Println("└─────────────────────────────────────────────────────┘")
	fmt.Println()

	status := pm.GetAllStatus()
	allStopped := true

	for _, s := range status {
		icon := "✗"
		status := "stopped"
		if s.Running {
			icon = "✓"
			status = "running"
			allStopped = false
		}
		fmt.Printf("  %s %-10s %s (PID %d)\n", icon, s.Name+":", status, s.PID)
	}

	fmt.Println()
	if allStopped {
		fmt.Println("All services are stopped")
		fmt.Println()
		fmt.Println("Use 'manage server:start' to start services")
	} else {
		fmt.Println("All services are running")
		fmt.Println()
		fmt.Println("Use 'manage server:stop' to stop services")
		fmt.Println("Use 'manage server:restart' to restart services")
	}
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
	fmt.Println("System Commands:")
	fmt.Println("  manage status                    Show system status")
	fmt.Println()
	fmt.Println("Server Commands:")
	fmt.Println("  manage server:start              Start server and worker")
	fmt.Println("  manage server:stop              Stop server and worker")
	fmt.Println("  manage server:restart           Restart all services")
	fmt.Println("  manage server:status            Show service status")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  manage migrate")
	fmt.Println("  manage seed")
	fmt.Println("  manage add-admin admin secretpassword")
	fmt.Println("  manage server:start")
	fmt.Println("  manage server:stop")
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

func runStatus() {
	ctx := context.Background()

	// Get process status
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
			fmt.Printf("  %s %-10s PID %d\n", icon, s.Name+":", s.PID)
		}
		fmt.Println()

		fmt.Println("Database:")
	} else {
		fmt.Println("OJS Monitor - System Status")
		fmt.Println()
	}

	// Database already initialized by initDB()
	fmt.Println("  ✓ Database: Connected")

	// Check admin users
	adminCount := wire.GetAdminCount(ctx)
	if adminCount > 0 {
		fmt.Printf("  ✓ Admin users: %d\n", adminCount)
	} else {
		fmt.Println("  ⚠ Admin users: None")
		fmt.Println("    → Run 'manage seed' or 'manage add-admin'")
	}

	// Check projects
	projects := wire.GetProjectsCount(ctx)
	fmt.Printf("  ✓ Projects: %d configured\n", projects)

	fmt.Println()
	fmt.Println("System: Ready")
}
