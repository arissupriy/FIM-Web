// Package mysql provides MySQL database connections for OJS.
package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// OJS represents a connection to OJS MySQL database
type OJS struct {
	db *sql.DB
}

// Config holds OJS database configuration
type Config struct {
	Host     string
	User     string
	Password string
	DBName   string
	Timeout  time.Duration
}

// DefaultTimeout is the default connection timeout
const DefaultTimeout = 10 * time.Second

// Connect establishes a connection to OJS MySQL database
func Connect(ctx context.Context, cfg Config) (*OJS, error) {
	if cfg.Timeout == 0 {
		cfg.Timeout = DefaultTimeout
	}

	// Parse host and port
	host, port := parseHost(cfg.Host)

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&timeout=%s",
		cfg.User, cfg.Password, host, port, cfg.DBName, cfg.Timeout)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open connection: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Test connection with timeout
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), cfg.Timeout)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("connection timeout: %w", err)
	}
	conn.Close()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping failed: %w", err)
	}

	return &OJS{db: db}, nil
}

// Close closes the database connection
func (o *OJS) Close() error {
	return o.db.Close()
}

// DB returns the underlying sql.DB connection
func (o *OJS) DB() *sql.DB {
	return o.db
}

// QueryRowContext executes a query that returns a single row
func (o *OJS) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return o.db.QueryRowContext(ctx, query, args...)
}

// QueryContext executes a query that returns rows
func (o *OJS) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return o.db.QueryContext(ctx, query, args...)
}

// ExecContext executes a query that doesn't return rows
func (o *OJS) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return o.db.ExecContext(ctx, query, args...)
}

// parseHost parses host:port string
func parseHost(host string) (string, string) {
	parts := strings.Split(host, ":")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return host, "3306"
}
