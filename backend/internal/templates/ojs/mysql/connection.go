// Package mysql provides OJS-specific MySQL database operations.
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

// Connection represents an OJS MySQL database connection.
type Connection struct {
	db *sql.DB
}

// Config holds OJS database configuration.
type Config struct {
	Host     string
	User     string
	Password string
	DBName   string
	Timeout  time.Duration
}

// DefaultTimeout is the default connection timeout.
const DefaultTimeout = 10 * time.Second

// Connect establishes a connection to OJS MySQL database.
func Connect(cfg Config) (*Connection, error) {
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

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping failed: %w", err)
	}

	return &Connection{db: db}, nil
}

// Close closes the database connection.
func (c *Connection) Close() error {
	return c.db.Close()
}

// DB returns the underlying sql.DB connection.
func (c *Connection) DB() *sql.DB {
	return c.db
}

// QueryRowContext executes a query that returns a single row.
func (c *Connection) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return c.db.QueryRowContext(ctx, query, args...)
}

// QueryContext executes a query that returns rows.
func (c *Connection) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return c.db.QueryContext(ctx, query, args...)
}

// ExecContext executes a query that doesn't return rows.
func (c *Connection) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return c.db.ExecContext(ctx, query, args...)
}

// parseHost parses host:port string.
func parseHost(host string) (string, string) {
	parts := strings.Split(host, ":")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return host, "3306"
}
