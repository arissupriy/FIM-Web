// Package process provides enterprise-grade process management.
package process

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// ProcessInfo stores complete process metadata
type ProcessInfo struct {
	PID       int       `json:"pid"`
	Name      string    `json:"name"`
	Binary    string    `json:"binary"`
	StartTime time.Time `json:"start_time"`
	Port      int       `json:"port,omitempty"`
	Host      string    `json:"host,omitempty"`
}

// Manager handles server and worker processes
type Manager struct {
	binDir string
}

// New creates a new process manager
func New() (*Manager, error) {
	exe, err := os.Executable()
	if err != nil {
		if len(os.Args) > 0 {
			exe = os.Args[0]
		} else {
			exe = "./manage"
		}
	}

	absExe, err := filepath.EvalSymlinks(exe)
	if err != nil {
		absExe = exe
	}

	binDir := filepath.Dir(absExe)

	return &Manager{
		binDir: binDir,
	}, nil
}

// getPIDPath returns the PID file path
func (m *Manager) getPIDPath(name string) string {
	return filepath.Join(m.binDir, "."+name+".pid")
}

// writePIDFile writes process info to PID file
func (m *Manager) writePIDFile(name string, info *ProcessInfo) error {
	pidPath := m.getPIDPath(name)

	data, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("marshal error: %w", err)
	}

	if err := os.WriteFile(pidPath, data, 0644); err != nil {
		return fmt.Errorf("write error: %w", err)
	}

	return nil
}

// readPIDFile reads process info from PID file
func (m *Manager) readPIDFile(name string) (*ProcessInfo, error) {
	pidPath := m.getPIDPath(name)

	data, err := os.ReadFile(pidPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read error: %w", err)
	}

	var info ProcessInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("parse error for %s: %w", name, err)
	}

	return &info, nil
}

// removeFiles removes PID files
func (m *Manager) removeFiles(name string) {
	os.Remove(m.getPIDPath(name))
}

// Status represents process status
type Status struct {
	Name    string `json:"name"`
	PID     int    `json:"pid"`
	Running bool   `json:"running"`
	Uptime  string `json:"uptime,omitempty"`
}

// IsRunning checks if process is running
func IsRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil
}

// IsPortInUse checks if a port is already in use
func IsPortInUse(port int) bool {
	if port <= 0 {
		return false
	}
	addr := fmt.Sprintf(":%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return true // Port is in use
	}
	ln.Close()
	return false
}

// GetStatus returns process status
func (m *Manager) GetStatus(name string) Status {
	info, err := m.readPIDFile(name)
	if err != nil || info == nil {
		return Status{Name: name, Running: false}
	}

	running := IsRunning(info.PID)

	return Status{
		Name:    name,
		PID:     info.PID,
		Running: running,
		Uptime:  time.Since(info.StartTime).Round(time.Second).String(),
	}
}

// GetAllStatus returns status of all processes
func (m *Manager) GetAllStatus() []Status {
	statuses := make([]Status, 0, 2)
	for _, name := range []string{"server", "worker"} {
		statuses = append(statuses, m.GetStatus(name))
	}
	return statuses
}

// StopProcess stops a process gracefully
func (m *Manager) StopProcess(name string, timeout time.Duration) error {
	info, err := m.readPIDFile(name)
	if err != nil {
		return err
	}
	if info == nil {
		return nil
	}

	if !IsRunning(info.PID) {
		m.removeFiles(name)
		return nil
	}

	syscall.Kill(info.PID, syscall.SIGTERM)

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !IsRunning(info.PID) {
			m.removeFiles(name)
			fmt.Printf("✓ %s stopped gracefully\n", name)
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	syscall.Kill(info.PID, syscall.SIGKILL)
	m.removeFiles(name)
	fmt.Printf("✓ %s force stopped\n", name)
	return nil
}

// StopServer stops the server
func (m *Manager) StopServer() error {
	return m.StopProcess("server", 10*time.Second)
}

// StopWorker stops the worker
func (m *Manager) StopWorker() error {
	return m.StopProcess("worker", 5*time.Second)
}

// StopAll stops all processes
func (m *Manager) StopAll() {
	m.StopWorker()
	m.StopServer()
}

// StartProcess starts a new process
func (m *Manager) StartProcess(name, binary string, host string, port int) (int, error) {
	// Check if already running
	status := m.GetStatus(name)
	if status.Running {
		return status.PID, fmt.Errorf("%s is already running (PID %d)", name, status.PID)
	}

	// Check port availability for server
	if name == "server" && port > 0 {
		if IsPortInUse(port) {
			return 0, fmt.Errorf("port %d is already in use by another process", port)
		}
	}

	// Find binary
	binaryPath := filepath.Join(m.binDir, binary)
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		return 0, fmt.Errorf("binary not found: %s", binaryPath)
	}

	// Build args
	args := []string{binaryPath}
	if name == "server" {
		if host != "" && host != "0.0.0.0" {
			args = append(args, "--host", host)
		}
		if port > 0 {
			args = append(args, "--port", fmt.Sprintf("%d", port))
		}
	}

	attr := &os.ProcAttr{
		Dir: m.binDir,
		Env: os.Environ(),
		Sys: &syscall.SysProcAttr{
			Setsid: true,
		},
	}

	cmd, err := os.StartProcess(binaryPath, args, attr)
	if err != nil {
		return 0, fmt.Errorf("start error: %w", err)
	}

	time.Sleep(300 * time.Millisecond)

	if !IsRunning(cmd.Pid) {
		return 0, fmt.Errorf("process exited immediately - check logs")
	}

	info := &ProcessInfo{
		PID:       cmd.Pid,
		Name:      name,
		Binary:    binary,
		StartTime: time.Now(),
		Port:      port,
		Host:      host,
	}

	if err := m.writePIDFile(name, info); err != nil {
		syscall.Kill(cmd.Pid, syscall.SIGKILL)
		return 0, fmt.Errorf("PID write error: %w", err)
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	if name == "worker" {
		fmt.Printf("✓ %s started (PID %d)\n", name, cmd.Pid)
	} else {
		fmt.Printf("✓ %s started (PID %d) on %s\n", name, cmd.Pid, addr)
	}
	return cmd.Pid, nil
}

// StartServer starts the server
func (m *Manager) StartServer(host string, port int) (int, error) {
	return m.StartProcess("server", "fim-server", host, port)
}

// StartWorker starts the worker
func (m *Manager) StartWorker() (int, error) {
	return m.StartProcess("worker", "worker", "", 0)
}

// StartAll starts all processes
func (m *Manager) StartAll(host string, port int) error {
	_, err := m.StartWorker()
	if err != nil {
		fmt.Printf("Warning: %v\n", err)
	}

	_, err = m.StartServer(host, port)
	return err
}

// RestartProcess restarts a process
func (m *Manager) RestartProcess(name string, host string, port int) error {
	m.StopProcess(name, 5*time.Second)
	time.Sleep(500 * time.Millisecond)

	if name == "worker" {
		_, err := m.StartWorker()
		return err
	}

	_, err := m.StartServer(host, port)
	return err
}

// RestartAll restarts all processes
func (m *Manager) RestartAll(host string, port int) error {
	m.StopAll()
	time.Sleep(500 * time.Millisecond)
	return m.StartAll(host, port)
}


// CleanupStale removes stale PID files
func (m *Manager) CleanupStale() {
	for _, name := range []string{"server", "worker"} {
		info, _ := m.readPIDFile(name)
		if info != nil && !IsRunning(info.PID) {
			fmt.Printf("Cleaning up stale PID for %s (PID %d)\n", name, info.PID)
			m.removeFiles(name)
		}
	}
}
