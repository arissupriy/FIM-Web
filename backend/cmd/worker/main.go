// Package main provides the background worker entry point.
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"ojs-monitor/backend/internal/infrastructure/worker"
	"ojs-monitor/backend/internal/infrastructure/watcher"
	"ojs-monitor/backend/internal/wire"
)

func main() {
	// Check for --help flag
	for _, arg := range os.Args {
		if arg == "--help" || arg == "-h" {
			printUsage()
			return
		}
	}

	fmt.Println("Starting OJS Monitor Background Worker")
	fmt.Println("Press Ctrl+C to stop")
	fmt.Println()

	// Initialize Database
	_ = wire.InitDB()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start worker goroutine
	go func() {
		worker.StartWorker()
	}()

	// Start watcher restoration
	go func() {
		watcher.RestoreWatchersOnStartup()
	}()

	// Wait for shutdown signal
	sig := <-sigCh
	fmt.Printf("\nReceived %v, shutting down...\n", sig)

	// Stop all watchers
	watcher.StopAllFIMWatchers()

	fmt.Println("Worker stopped.")
}

func printUsage() {
	fmt.Println("OJS Monitor Background Worker")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  ./worker              Start the background worker")
	fmt.Println()
	fmt.Println("The worker processes:")
	fmt.Println("  - Baseline scan jobs")
	fmt.Println("  - Integrity scan jobs")
	fmt.Println("  - File reconciliation")
	fmt.Println()
	fmt.Println("CLI Commands:")
	fmt.Println("  Use './manage' for database commands")
}
