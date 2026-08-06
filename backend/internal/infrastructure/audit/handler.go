// Package audit provides HTTP handlers for auditd event ingestion.
package audit

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ojs-monitor/backend/internal/domain/models"
	"ojs-monitor/backend/internal/domain/repository"
)

// AuditHandler handles auditd event ingestion.
type AuditHandler struct {
	eventRepo   repository.FIMEventRepository
	projectRepo repository.ProjectRepository
}

// NewAuditHandler creates a new audit handler.
func NewAuditHandler(eventRepo repository.FIMEventRepository, projectRepo repository.ProjectRepository) *AuditHandler {
	return &AuditHandler{
		eventRepo:   eventRepo,
		projectRepo: projectRepo,
	}
}

// IngestEventsRequest represents the request body for ingesting audit events.
type IngestEventsRequest struct {
	ProjectID int      `json:"project_id"`
	Events    []string `json:"events"` // Raw audit log lines
}

// IngestEventsResponse represents the response for ingested events.
type IngestEventsResponse struct {
	Processed int      `json:"processed"`
	Failed    int      `json:"failed"`
	Errors    []string `json:"errors,omitempty"`
}

// IngestEvents handles POST /fim/audit/ingest
// Receives raw auditd log lines and converts them to FIMEvents
func (h *AuditHandler) IngestEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var req IngestEventsRequest
	if err := json.Unmarshal(body, &req); err != nil {
		// Try parsing as raw string array
		if lines, ok := parseStringArray(body); ok {
			req.Events = lines
		} else {
			http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}
	}

	if req.ProjectID == 0 {
		http.Error(w, "project_id is required", http.StatusBadRequest)
		return
	}

	// Verify project exists
	ctx := r.Context()
	project, err := h.projectRepo.GetByID(ctx, req.ProjectID)
	if err != nil {
		http.Error(w, "Project not found: "+err.Error(), http.StatusNotFound)
		return
	}

	response := IngestEventsResponse{}
	fimEvents := make([]*models.FIMEvent, 0, len(req.Events))

	for i, line := range req.Events {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		event, err := ParseEvent(line)
		if err != nil {
			response.Failed++
			response.Errors = append(response.Errors, "line "+strconv.Itoa(i)+": "+err.Error())
			continue
		}
		if event == nil {
			continue // Skip empty/invalid lines
		}

		fimEvent := ConvertAuditToFIMEvent(event, project.ID)
		fimEvents = append(fimEvents, fimEvent)
	}

	// Save all events to repository
	if len(fimEvents) > 0 {
		for _, event := range fimEvents {
			if err := h.eventRepo.Create(ctx, event); err != nil {
				response.Failed++
				response.Errors = append(response.Errors, "save error: "+err.Error())
			} else {
				response.Processed++
			}
		}
	}

	// Log summary
	_ = project // used for verification above
	if len(fimEvents) > 0 {
		// Dispatch to alert system if high-risk events
		for _, event := range fimEvents {
			if event.IsHighRisk() {
				dispatchAlert(event)
			}
			// Dispatch to SIEM (all events, not just high-risk)
			dispatchSIEM(event)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// parseStringArray attempts to parse body as a JSON array of strings.
func parseStringArray(body []byte) ([]string, bool) {
	var lines []string
	if err := json.Unmarshal(body, &lines); err != nil {
		return nil, false
	}
	return lines, true
}

// ConvertAuditToFIMEvent converts an audit.Event to a FIMEvent.
func ConvertAuditToFIMEvent(ae *Event, projectID int) *models.FIMEvent {
	eventType := mapAuditType(ae.Type)
	riskLevel := classifyRisk(ae)
	classification := classifySource(ae)

	return &models.FIMEvent{
		ProjectID:     projectID,
		EventType:     eventType,
		FilePath:      ae.Path,
		FileHash:      "", // auditd doesn't provide hash
		ActorType:    determineActorType(ae),
		ActorID:      strconv.FormatUint(uint64(ae.ProcessID), 10),
		ActorName:    ae.ProcessName,
		ActorDetails: buildActorDetails(ae),
		RiskLevel:    riskLevel,
		Classification: classification,
		Source:       "AUDITD",
		Details:      buildDetails(ae),
		AlertSent:    false,
		Timestamp:    ae.Timestamp.Format(time.RFC3339),
	}
}

// mapAuditType maps audit event types to FIM event types.
func mapAuditType(auditType string) string {
	switch auditType {
	case "SYSCALL":
		return "MODIFIED"
	case "EXECVE":
		return "CREATED" // Command execution creates processes
	case "PATH":
		return "MODIFIED"
	case "CREATE", "OPENAT", "OPEN":
		return "CREATED"
	case "UNLINK", "RMDIR", "DELETE":
		return "DELETED"
	case "CHMOD", "FCHMOD", "FCHMODAT":
		return "MODIFIED"
	case "CHOWN", "FCHOWN", "LCHOWN", "FCHOWNAT":
		return "MODIFIED"
	case "LINK", "SYMLINK", "RENAME":
		return "MODIFIED"
	case "SETXATTR", "REMOVEXATTR", "LSETXATTR":
		return "MODIFIED"
	default:
		return "MODIFIED"
	}
}

// classifyRisk determines risk level based on audit event.
func classifyRisk(ae *Event) string {
	// High risk: execution, privilege changes, suspicious syscalls
	switch ae.Type {
	case "EXECVE":
		return "HIGH" // Command execution
	case "USER_CMD":
		return "HIGH" // User commands
	case "ANOM_LINK":
		return "HIGH" // Suspicious link activity
	case "CHOWN", "FCHOWN", "LCHOWN", "FCHOWNAT":
		return "MEDIUM" // Ownership change
	case "CHMOD", "FCHMOD", "FCHMODAT":
		return "MEDIUM" // Permission change
	case "SETXATTR", "REMOVEXATTR":
		return "MEDIUM" // xattr change
	case "USER_AUTH":
		return "HIGH" // Authentication
	case "USER_START":
		return "MEDIUM" // Session start
	case "LOGIN":
		return "CRITICAL" // Login event
	case "USER_END":
		return "LOW" // Session end
	default:
		// Check syscall for suspicious activity
		switch ae.Syscall {
		case 59: // execve
			return "HIGH"
		case 82, 83, 84: // rename, unlink, rmdir
			return "MEDIUM"
		default:
			return "LOW"
		}
	}
}

// classifySource determines if the source is trusted.
func classifySource(ae *Event) string {
	// Check if it's a system process
	if ae.ProcessName == "systemd" || ae.ProcessName == "kernel" {
		return "TRUSTED"
	}

	// Check if it's a known OJS process
	knownProcesses := map[string]bool{
		"apache2":     true,
		"apache":      true,
		"nginx":       true,
		"php-fpm":     true,
		"php":         true,
		"mysqld":      true,
		"mariadb":     true,
		"sshd":        true,
		"cron":        true,
		"atd":         true,
		"systemd":     true,
		"kernel":      true,
	}

	if knownProcesses[ae.ProcessName] {
		return "TRUSTED"
	}

	// Check if user is root (privileged)
	if ae.UserID == 0 || ae.EffectiveUID == 0 {
		return "TRUSTED"
	}

	// Check if login UID is set (real user)
	if ae.LoginUID != 4294967295 && ae.LoginUID != 0 { // 4294967295 = unset
		return "MODIFIED" // Real user activity
	}

	return "UNKNOWN_SOURCE"
}

// determineActorType determines the actor type based on audit event.
func determineActorType(ae *Event) string {
	if ae.LoginUID != 4294967295 && ae.LoginUID != 0 {
		return "OJS_USER"
	}
	if ae.ProcessName != "" {
		return "PROCESS"
	}
	return "SYSTEM_USER"
}

// buildActorDetails creates detailed actor information string.
func buildActorDetails(ae *Event) string {
	details := make([]string, 0)

	if ae.UserID != 0 {
		details = append(details, "uid="+strconv.FormatUint(uint64(ae.UserID), 10))
	}
	if ae.LoginUID != 4294967295 && ae.LoginUID != 0 {
		details = append(details, "auid="+strconv.FormatUint(uint64(ae.LoginUID), 10))
	}
	if ae.EffectiveUID != 0 {
		details = append(details, "euid="+strconv.FormatUint(uint64(ae.EffectiveUID), 10))
	}
	if ae.SessionID != 0 {
		details = append(details, "ses="+strconv.FormatUint(ae.SessionID, 10))
	}
	if ae.Syscall != 0 {
		details = append(details, "syscall="+strconv.Itoa(ae.Syscall))
	}
	if ae.TTY != "" {
		details = append(details, "tty="+ae.TTY)
	}

	return strings.Join(details, " ")
}

// buildDetails creates detailed event information.
func buildDetails(ae *Event) string {
	details := make([]string, 0)

	if ae.Path != "" {
		details = append(details, "path="+ae.Path)
	}
	if ae.Directory != "" {
		details = append(details, "dir="+ae.Directory)
	}
	if len(ae.Command) > 0 {
		details = append(details, "cmd="+strings.Join(ae.Command, " "))
	}
	if ae.ReturnCode != 0 {
		details = append(details, "exit="+strconv.Itoa(ae.ReturnCode))
	}
	if ae.Mode != "" {
		details = append(details, "mode="+ae.Mode)
	}
	if ae.Key != "" {
		details = append(details, "key="+ae.Key)
	}

	return strings.Join(details, "; ")
}

// dispatchAlert sends high-risk events to the alert system.
func dispatchAlert(event *models.FIMEvent) {
	// Import alert dispatcher if available
	// This will be wired during INT-01 integration
	_ = event
}

// dispatchSIEM sends events to SIEM for long-term storage.
func dispatchSIEM(event *models.FIMEvent) {
	// SIEM dispatcher is optional - events are stored in FIMEvent table
	// and can be exported to SIEM via background worker
	_ = event
}

// GetStatus returns auditd ingestion status (placeholder).
func (h *AuditHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "active",
		"source":  "auditd",
		"version": "1.0",
	})
}
