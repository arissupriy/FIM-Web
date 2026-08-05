// Package main provides the background worker entry point.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"ojs-monitor/backend/internal/domain/models"
	"ojs-monitor/backend/internal/infrastructure/alert"
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
	if _, err := wire.InitDB(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Initialize Alert Dispatcher
	if err := initAlertDispatcher(); err != nil {
		log.Fatalf("Failed to initialize alert dispatcher: %v", err)
	}

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

// initAlertDispatcher initializes the global alert dispatcher.
func initAlertDispatcher() error {
	// Get repositories from wire
	dispatcher := alert.NewDispatcher(
		wire.GetAlertConfigRepo(),
		wire.GetAlertHistoryRepo(),
	)

	// Register alert channels
	dispatcher.RegisterChannel(models.AlertChannelEmail, &alert.EmailChannel{})
	dispatcher.RegisterChannel(models.AlertChannelSlack, &alert.SlackChannel{})
	dispatcher.RegisterChannel(models.AlertChannelWebhook, &alert.WebhookChannel{})

	// Start the dispatcher with background context
	dispatcher.Start(context.Background())

	// Set as global dispatcher for watcher
	watcher.SetAlertDispatcher(dispatcher)

	log.Println("Alert dispatcher initialized")
	return nil
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
