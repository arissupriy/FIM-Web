// Package process provides process management for managing server and worker.
package process

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// Manager handles server and worker processes.
type Manager struct {
	dir     string
	ServerPID string
	WorkerPID string
}

// New creates a new process manager.
func New() (*Manager, error) {
	// Get executable directory
	exe, err := os.Executable()
	if err != nil {
		exe = os.Args[0]
	}
	dir := filepath.Dir(exe)

	return &Manager{
		dir:     dir,
		ServerPID: filepath.Join(dir, ".server.pid"),
		WorkerPID: filepath.Join(dir, ".worker.pid"),
	}, nil
}

// PIDFile represents a PID file.
type PIDFile struct {
	path string
}

// ReadPID reads the PID from a PID file.
func ReadPID(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, err
	}

	return pid, nil
}

// IsRunning checks if a process with the given PID is running.
func IsRunning(pid int) bool {
	if pid <= 0 {
		return false
	}

	// Signal 0 doesn't send anything but checks if process exists
	err := syscall.Kill(pid, 0)
	return err == nil
}

// WritePID writes a PID to a file.
func WritePID(path string, pid int) error {
	return os.WriteFile(path, []byte(strconv.Itoa(pid)), 0644)
}

// RemovePID removes a PID file.
func RemovePID(path string) error {
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// Status represents the status of a managed process.
type Status struct {
	Name    string
	PID     int
	Running bool
}

// GetServerStatus returns the status of the server process.
func (m *Manager) GetServerStatus() Status {
	pid, _ := ReadPID(m.ServerPID)
	return Status{
		Name:    "server",
		PID:     pid,
		Running: IsRunning(pid),
	}
}

// GetWorkerStatus returns the status of the worker process.
func (m *Manager) GetWorkerStatus() Status {
	pid, _ := ReadPID(m.WorkerPID)
	return Status{
		Name:    "worker",
		PID:     pid,
		Running: IsRunning(pid),
	}
}

// GetAllStatus returns the status of all managed processes.
func (m *Manager) GetAllStatus() []Status {
	return []Status{
		m.GetServerStatus(),
		m.GetWorkerStatus(),
	}
}

// StopProcess stops a process by PID file.
func (m *Manager) StopProcess(pidPath string) error {
	pid, err := ReadPID(pidPath)
	if err != nil {
		return fmt.Errorf("failed to read PID: %w", err)
	}

	if pid == 0 {
		return nil // Not running
	}

	if !IsRunning(pid) {
		// Process not running, clean up PID file
		RemovePID(pidPath)
		return nil
	}

	// Send SIGTERM for graceful shutdown
	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
		if err == syscall.ESRCH {
			// Process doesn't exist, clean up
			RemovePID(pidPath)
			return nil
		}
		return fmt.Errorf("failed to send SIGTERM: %w", err)
	}

	// Wait for process to exit (max 10 seconds)
	for i := 0; i < 100; i++ {
		if !IsRunning(pid) {
			RemovePID(pidPath)
			return nil
		}
		// sleep 100ms
	}

	// Force kill if still running
	syscall.Kill(pid, syscall.SIGKILL)
	RemovePID(pidPath)

	return nil
}

// StopServer stops the server process.
func (m *Manager) StopServer() error {
	return m.StopProcess(m.ServerPID)
}

// StopWorker stops the worker process.
func (m *Manager) StopWorker() error {
	return m.StopProcess(m.WorkerPID)
}

// StopAll stops all managed processes.
func (m *Manager) StopAll() error {
	// Stop worker first, then server
	m.StopWorker()
	m.StopServer()
	return nil
}

// StartProcess starts a new process and writes its PID.
func (m *Manager) StartProcess(name, binary string, args []string) (int, error) {
	// Check if already running
	status := m.GetAllStatus()
	for _, s := range status {
		if s.Name == name && s.Running {
			return s.PID, fmt.Errorf("%s is already running (PID %d)", name, s.PID)
		}
	}

	// Find binary path
	binaryPath := filepath.Join(m.dir, binary)
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		// Try current directory
		binaryPath = binary
	}

	// Check if binary exists
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		return 0, fmt.Errorf("binary not found: %s", binaryPath)
	}

	// Start process
	cmd, err := os.StartProcess(binaryPath, append([]string{binaryPath}, args...), &os.ProcAttr{
		Dir: m.dir,
		Env: os.Environ(),
		Sys: &syscall.SysProcAttr{
			Setsid: true, // Create new session
		},
	})
	if err != nil {
		return 0, fmt.Errorf("failed to start %s: %w", name, err)
	}

	// Write PID file
	pidPath := m.ServerPID
	if name == "worker" {
		pidPath = m.WorkerPID
	}

	if err := WritePID(pidPath, cmd.Pid); err != nil {
		cmd.Kill()
		return 0, fmt.Errorf("failed to write PID: %w", err)
	}

	return cmd.Pid, nil
}

// StartServer starts the server process.
func (m *Manager) StartServer() (int, error) {
	return m.StartProcess("server", "fim-server", nil)
}

// StartWorker starts the worker process.
func (m *Manager) StartWorker() (int, error) {
	return m.StartProcess("worker", "worker", nil)
}

// StartAll starts both server and worker processes.
func (m *Manager) StartAll() error {
	// Start worker first
	pid, err := m.StartWorker()
	if err != nil {
		fmt.Printf("Warning: %v\n", err)
	} else {
		fmt.Printf("✓ Worker started (PID %d)\n", pid)
	}

	// Start server
	pid, err = m.StartServer()
	if err != nil {
		return err
	}

	fmt.Printf("✓ Server started (PID %d)\n", pid)
	return nil
}

// RestartProcess restarts a process.
func (m *Manager) RestartProcess(name string) error {
	pidPath := m.ServerPID
	if name == "worker" {
		pidPath = m.WorkerPID
	}

	fmt.Printf("Stopping %s...\n", name)
	if err := m.StopProcess(pidPath); err != nil {
		return err
	}
	fmt.Printf("Starting %s...\n", name)

	binary := "fim-server"
	if name == "worker" {
		binary = "worker"
	}

	pid, err := m.StartProcess(name, binary, nil)
	if err != nil {
		return err
	}

	fmt.Printf("✓ %s restarted (PID %d)\n", name, pid)
	return nil
}

// RestartAll restarts all processes.
func (m *Manager) RestartAll() error {
	fmt.Println("Stopping all processes...")
	m.StopAll()
	fmt.Println("Starting all processes...")
	return m.StartAll()
}
