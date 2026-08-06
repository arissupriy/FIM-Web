// Package acl provides HTTP handlers for ACL event ingestion.
package acl

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"ojs-monitor/backend/internal/domain/models"
	"ojs-monitor/backend/internal/domain/repository"
)

// ACLHandler handles ACL event ingestion.
type ACLHandler struct {
	eventRepo   repository.FIMEventRepository
	projectRepo repository.ProjectRepository
}

// NewACLHandler creates a new ACL handler.
func NewACLHandler(eventRepo repository.FIMEventRepository, projectRepo repository.ProjectRepository) *ACLHandler {
	return &ACLHandler{
		eventRepo:   eventRepo,
		projectRepo: projectRepo,
	}
}

// ChangeACLsRequest represents the request body for ACL change ingestion.
type ChangeACLsRequest struct {
	ProjectID    int            `json:"project_id"`
	Path         string         `json:"path"`
	BeforeACL    *ACL           `json:"before_acl,omitempty"`
	AfterACL     *ACL           `json:"after_acl"`
	DetectedBy   string         `json:"detected_by"` // "fsnotify", "auditd", "rescan"
	Timestamp    string         `json:"timestamp,omitempty"`
}

// ScanACLsRequest represents a request to scan ACLs.
type ScanACLsRequest struct {
	ProjectID int      `json:"project_id"`
	Paths     []string `json:"paths"`
}

// ScanACLsResponse represents the response from ACL scan.
type ScanACLsResponse struct {
	ProjectID int                `json:"project_id"`
	ScannedAt string             `json:"scanned_at"`
	Results   map[string]*ACL    `json:"results"`
	Errors    map[string]string  `json:"errors,omitempty"`
}

// GetStatus returns ACL monitoring status.
func (h *ACLHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "active",
		"source":  "acl",
		"version": "1.0",
	})
}

// IngestChanges handles POST /fim/acl/ingest
// Receives ACL changes and converts them to FIMEvents.
func (h *ACLHandler) IngestChanges(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ChangeACLsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.ProjectID == 0 {
		http.Error(w, "project_id is required", http.StatusBadRequest)
		return
	}

	if req.AfterACL == nil {
		http.Error(w, "after_acl is required", http.StatusBadRequest)
		return
	}

	// Verify project exists
	project, err := h.projectRepo.GetByID(r.Context(), req.ProjectID)
	if err != nil {
		http.Error(w, "Project not found: "+err.Error(), http.StatusNotFound)
		return
	}

	// Parse changes
	var changes []ACLChange
	if req.BeforeACL != nil {
		changes = CompareACLs(req.BeforeACL, req.AfterACL)
	} else {
		// No before ACL - all entries are "added"
		for _, entry := range req.AfterACL.Entries {
			changes = append(changes, ACLChange{
				Type:    ChangeAdded,
				Entry:   entry,
				NewPerms: &entry.Perms,
			})
		}
	}

	// Convert each change to FIMEvent
	fimEvents := make([]*models.FIMEvent, 0)
	for _, change := range changes {
		fimEvent := ConvertACLChangeToFIMEvent(&change, req.Path, project.ID, req.DetectedBy)
		fimEvents = append(fimEvents, fimEvent)
	}

	// Save events
	processed := 0
	for _, event := range fimEvents {
		if err := h.eventRepo.Create(r.Context(), event); err != nil {
			// Log but don't fail entire request
			continue
		}
		processed++
	}

	// Dispatch alerts for high-risk changes
	for _, event := range fimEvents {
		if event.IsHighRisk() {
			dispatchAlert(event)
		}
		// Dispatch to SIEM (all events)
		dispatchSIEM(event)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"processed": processed,
		"changes":   len(changes),
		"events":    len(fimEvents),
	})
}

// ScanACLs handles POST /fim/acl/scan
// Scans ACLs for specified paths.
func (h *ACLHandler) ScanACLs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ScanACLsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.ProjectID == 0 {
		http.Error(w, "project_id is required", http.StatusBadRequest)
		return
	}

	if len(req.Paths) == 0 {
		http.Error(w, "paths is required", http.StatusBadRequest)
		return
	}

	// Verify project exists
	_, err := h.projectRepo.GetByID(r.Context(), req.ProjectID)
	if err != nil {
		http.Error(w, "Project not found: "+err.Error(), http.StatusNotFound)
		return
	}

	// Scan paths
	monitor := NewMonitor(req.Paths...)
	results, err := monitor.Scan()

	response := ScanACLsResponse{
		ProjectID: req.ProjectID,
		ScannedAt: time.Now().Format(time.RFC3339),
		Results: results,
		Errors: make(map[string]string),
	}

	// Collect errors
	if err != nil {
		response.Errors["scan"] = err.Error()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ConvertACLChangeToFIMEvent converts an ACLChange to a FIMEvent.
func ConvertACLChangeToFIMEvent(change *ACLChange, path string, projectID int, detectedBy string) *models.FIMEvent {
	riskLevel := classifyRisk(change)
	classification := classifySource(change)

	// Build actor details
	var actorDetails string
	if change.Entry.Tag != "" {
		actorDetails = fmt.Sprintf("%s:%s", change.Entry.Tag, change.Entry.Qual)
	}

	// Build details
	var details string
	switch change.Type {
	case ChangeAdded:
		details = fmt.Sprintf("ACL entry added: %s %s", change.Entry.Tag, change.Entry.Perms.String())
	case ChangeRemoved:
		details = fmt.Sprintf("ACL entry removed: %s:%s", change.Entry.Tag, change.Entry.Qual)
	case ChangeModified:
		if change.OldPerms != nil && change.NewPerms != nil {
			details = fmt.Sprintf("ACL entry modified: %s:%s %s -> %s",
				change.Entry.Tag, change.Entry.Qual,
				change.OldPerms.String(), change.NewPerms.String())
		}
	}

	return &models.FIMEvent{
		ProjectID:     projectID,
		EventType:     "MODIFIED", // ACL change is always MODIFIED
		FilePath:      path,
		FileHash:      "", // ACL doesn't provide hash
		ActorType:     determineActorType(detectedBy),
		ActorID:       "",
		ActorName:     getActorName(change),
		ActorDetails:  actorDetails,
		RiskLevel:     riskLevel,
		Classification: classification,
		Source:        "ACL",
		Details:       details,
		AlertSent:     false,
		Timestamp:     time.Now().Format(time.RFC3339),
	}
}

// classifyRisk determines risk level based on ACL change.
func classifyRisk(change *ACLChange) string {
	// High risk: adding/removing entries, permission escalation
	switch change.Type {
	case ChangeAdded:
		// Adding write/execute permissions is high risk
		if change.NewPerms != nil && change.NewPerms.Write && change.NewPerms.Execute {
			return "HIGH"
		}
		return "MEDIUM"
	case ChangeRemoved:
		// Removing restrictions could be risky
		return "MEDIUM"
	case ChangeModified:
		// Permission escalation
		if change.OldPerms != nil && change.NewPerms != nil {
			// Adding execute permission
			if !change.OldPerms.Execute && change.NewPerms.Execute {
				return "HIGH"
			}
			// Adding write permission
			if !change.OldPerms.Write && change.NewPerms.Write {
				return "MEDIUM"
			}
		}
		return "MEDIUM"
	default:
		return "LOW"
	}
}

// classifySource determines if the source is trusted.
func classifySource(change *ACLChange) string {
	// ACL changes are typically trusted if done by root
	// but we flag any ACL modification as potentially significant
	if change.Entry.Tag == ACLTagUser || change.Entry.Tag == ACLTagGroup {
		return "MODIFIED" // Named user/group changes
	}
	return "TRUSTED"
}

// determineActorType determines the actor type.
func determineActorType(detectedBy string) string {
	switch detectedBy {
	case "fsnotify":
		return "PROCESS"
	case "auditd":
		return "SYSTEM_USER"
	case "rescan":
		return "SCANNER"
	default:
		return "PROCESS"
	}
}

// getActorName returns a human-readable actor name.
func getActorName(change *ACLChange) string {
	if change.Entry.Qual != "" {
		return fmt.Sprintf("%s:%s", change.Entry.Tag, change.Entry.Qual)
	}
	return string(change.Entry.Tag)
}

// dispatchAlert sends high-risk events to the alert system.
func dispatchAlert(event *models.FIMEvent) {
	// This will be wired during integration
	_ = event
}

// dispatchSIEM sends events to SIEM for long-term storage.
func dispatchSIEM(event *models.FIMEvent) {
	// SIEM dispatcher is optional - events are stored in FIMEvent table
	_ = event
}
