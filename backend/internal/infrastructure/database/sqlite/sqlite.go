// Package sqlite provides SQLite database implementation.
package sqlite

import (
	"database/sql"
	"fmt"
	"log"

	_ "modernc.org/sqlite"
)

// DB wraps the sql.DB connection
type DB struct {
	*sql.DB
}

// Config holds database configuration
type Config struct {
	Path string
}

// New creates a new SQLite database connection
func New(cfg Config) (*DB, error) {
	db, err := sql.Open("sqlite", cfg.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable WAL mode for better concurrency
	if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		log.Printf("Warning: failed to enable WAL mode: %v", err)
	}
	db.Exec("PRAGMA synchronous=NORMAL;")

	// Test connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{DB: db}, nil
}

// NewDB wraps an existing *sql.DB into a sqlite.DB
func NewDB(db *sql.DB) *DB {
	return &DB{DB: db}
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.DB.Close()
}

// MustClose closes the database and panics on error
func (db *DB) MustClose() {
	if err := db.Close(); err != nil {
		panic(err)
	}
}
