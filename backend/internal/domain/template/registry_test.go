// Package template provides tests for template registry.
package template

import (
	"context"
	"testing"

	"ojs-monitor/backend/internal/domain/models"
)

// mockTemplate is a mock template for testing.
type mockTemplate struct {
	name       string
	version    string
	priority   int
	compatible bool
}

func (m *mockTemplate) Name() string { return m.name }
func (m *mockTemplate) Version() string { return m.version }
func (m *mockTemplate) Priority() int { return m.priority }
func (m *mockTemplate) DefaultConfig() *TemplateConfig {
	return &TemplateConfig{Template: m.name}
}
func (m *mockTemplate) CreateDBConnection(ctx context.Context, config DBConnectionConfig) (DBConnection, error) {
	return nil, nil
}
func (m *mockTemplate) DetectOrphans(ctx context.Context, db DBConnection, files []*models.ProjectFile) ([]*models.ProjectFile, error) {
	return nil, nil
}
func (m *mockTemplate) GetMetrics(ctx context.Context, db DBConnection) (*TemplateMetrics, error) { return nil, nil }
func (m *mockTemplate) ValidateIntegrity(ctx context.Context, db DBConnection, project *models.Project) ([]IntegrityWarning, error) {
	return nil, nil
}
func (m *mockTemplate) CorrelateFile(ctx context.Context, db DBConnection, filePath string, eventType string) (*CorrelationResult, error) {
	return nil, nil
}
func (m *mockTemplate) RequiredDBConfig() []string { return nil }
func (m *mockTemplate) Compatible(ctx context.Context, db DBConnection) (bool, error) {
	return m.compatible, nil
}

func TestRegistry_Register(t *testing.T) {
	r := NewRegistry()

	tpl := &mockTemplate{name: "test", priority: 100}
	r.Register(tpl)

	got, ok := r.Get("test")
	if !ok {
		t.Error("expected to find registered template")
	}
	if got.Name() != "test" {
		t.Errorf("Name() = %q, want %q", got.Name(), "test")
	}
}

func TestRegistry_Register_Panic(t *testing.T) {
	r := NewRegistry()

	tpl := &mockTemplate{name: "test", priority: 100}
	r.Register(tpl)

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on duplicate registration")
		}
	}()

	r.Register(&mockTemplate{name: "test", priority: 50})
}

func TestRegistry_Get_NotFound(t *testing.T) {
	r := NewRegistry()

	_, ok := r.Get("nonexistent")
	if ok {
		t.Error("expected not found")
	}
}

func TestRegistry_List_SortedByPriority(t *testing.T) {
	r := NewRegistry()

	r.Register(&mockTemplate{name: "low", priority: 10})
	r.Register(&mockTemplate{name: "high", priority: 100})
	r.Register(&mockTemplate{name: "medium", priority: 50})

	templates := r.List()

	if len(templates) != 3 {
		t.Fatalf("len(templates) = %d, want 3", len(templates))
	}

	if templates[0].Name() != "high" {
		t.Errorf("templates[0].Name() = %q, want %q", templates[0].Name(), "high")
	}
	if templates[1].Name() != "medium" {
		t.Errorf("templates[1].Name() = %q, want %q", templates[1].Name(), "medium")
	}
	if templates[2].Name() != "low" {
		t.Errorf("templates[2].Name() = %q, want %q", templates[2].Name(), "low")
	}
}

func TestRegistry_MustGet(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockTemplate{name: "test", priority: 100})

	tpl := r.MustGet("test")
	if tpl.Name() != "test" {
		t.Errorf("Name() = %q, want %q", tpl.Name(), "test")
	}
}

func TestRegistry_MustGet_Panic(t *testing.T) {
	r := NewRegistry()

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on MustGet of nonexistent template")
		}
	}()

	r.MustGet("nonexistent")
}

func TestRegistry_DetectBest_FirstCompatible(t *testing.T) {
	r := NewRegistry()

	r.Register(&mockTemplate{name: "high", priority: 100, compatible: false})
	r.Register(&mockTemplate{name: "low", priority: 10, compatible: true})

	tpl, err := r.DetectBest(context.Background(), nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if tpl == nil {
		t.Fatal("expected template, got nil")
	}
	if tpl.Name() != "low" {
		t.Errorf("Name() = %q, want %q", tpl.Name(), "low")
	}
}

func TestRegistry_DetectBest_NoCompatible(t *testing.T) {
	r := NewRegistry()

	r.Register(&mockTemplate{name: "first", priority: 100, compatible: false})
	r.Register(&mockTemplate{name: "second", priority: 50, compatible: false})

	tpl, err := r.DetectBest(context.Background(), nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if tpl != nil {
		t.Error("expected nil template when no compatible")
	}
}

func TestGlobalRegistry(t *testing.T) {
	Reset()
	defer Reset()

	Register(&mockTemplate{name: "global", priority: 100})

	tpl, ok := Get("global")
	if !ok {
		t.Error("expected to find globally registered template")
	}
	if tpl.Name() != "global" {
		t.Errorf("Name() = %q, want %q", tpl.Name(), "global")
	}
}

func TestGlobalList(t *testing.T) {
	Reset()
	defer Reset()

	Register(&mockTemplate{name: "one", priority: 100})
	Register(&mockTemplate{name: "two", priority: 50})

	templates := List()
	if len(templates) != 2 {
		t.Errorf("len(templates) = %d, want 2", len(templates))
	}
}
