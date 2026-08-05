// Package ojs provides OJS-specific template implementation.
package ojs

import (
	"ojs-monitor/backend/internal/domain/template"
)

func init() {
	// Register OJS template with the global registry
	template.Register(New())
}
