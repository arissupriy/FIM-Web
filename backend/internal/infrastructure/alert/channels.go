// Package alert provides alert channel implementations.
package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"ojs-monitor/backend/internal/domain/models"
)

// EmailChannel implements email alert delivery.
type EmailChannel struct {
	SMTPHost string
	SMTPPort int
	Username string
	Password string
	From     string
}

// Name returns the channel name.
func (c *EmailChannel) Name() string {
	return "email"
}

// Send sends an email alert.
func (c *EmailChannel) Send(ctx context.Context, config *models.AlertConfig, event *models.FIMEvent) error {
	var emailConfig models.EmailConfig
	if err := json.Unmarshal([]byte(config.Config), &emailConfig); err != nil {
		return fmt.Errorf("invalid email config: %w", err)
	}

	if len(emailConfig.Recipients) == 0 {
		return fmt.Errorf("no recipients configured")
	}

	// Build email content
	subject := fmt.Sprintf("[FIM Alert] %s - %s", event.RiskLevel, event.EventType)
	if emailConfig.Subject != "" {
		subject = emailConfig.Subject
	}

	body := fmt.Sprintf(`File Integrity Alert

Risk Level: %s
Event Type: %s
File Path: %s
Project ID: %d
Classification: %s
Timestamp: %s

Details:
%s
`,
		event.RiskLevel,
		event.EventType,
		event.FilePath,
		event.ProjectID,
		event.Classification,
		event.Timestamp,
		event.Details,
	)

	// TODO: Implement actual SMTP sending
	// For now, just log
	fmt.Printf("Email alert to %v: %s\n%s\n", emailConfig.Recipients, subject, body)

	return nil
}

// SlackChannel implements Slack webhook alert delivery.
type SlackChannel struct {
	// Uses webhook URL from config
}

// Name returns the channel name.
func (c *SlackChannel) Name() string {
	return "slack"
}

// Send sends a Slack alert.
func (c *SlackChannel) Send(ctx context.Context, config *models.AlertConfig, event *models.FIMEvent) error {
	var slackConfig models.SlackConfig
	if err := json.Unmarshal([]byte(config.Config), &slackConfig); err != nil {
		return fmt.Errorf("invalid slack config: %w", err)
	}

	if slackConfig.WebhookURL == "" {
		return fmt.Errorf("no webhook URL configured")
	}

	// Build Slack message
	color := "#36a64f" // green
	switch event.RiskLevel {
	case "HIGH", "CRITICAL":
		color = "#ff0000" // red
	case "MEDIUM":
		color = "#ff9900" // orange
	}

	payload := map[string]interface{}{
		"attachments": []map[string]interface{}{
			{
				"color": color,
				"fields": []map[string]interface{}{
					{"title": "Risk Level", "value": event.RiskLevel, "short": true},
					{"title": "Event Type", "value": event.EventType, "short": true},
					{"title": "File Path", "value": event.FilePath, "short": false},
					{"title": "Classification", "value": event.Classification, "short": true},
					{"title": "Actor", "value": fmt.Sprintf("%s (%s)", event.ActorName, event.ActorType), "short": true},
				},
			},
		},
	}

	if slackConfig.Channel != "" {
		payload["channel"] = slackConfig.Channel
	}
	if slackConfig.Username != "" {
		payload["username"] = slackConfig.Username
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal slack payload: %w", err)
	}

	// Send to webhook
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", slackConfig.WebhookURL, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send slack message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack webhook returned status %d", resp.StatusCode)
	}

	return nil
}

// WebhookChannel implements custom webhook alert delivery.
type WebhookChannel struct {
	Client *http.Client
}

// Name returns the channel name.
func (c *WebhookChannel) Name() string {
	return "webhook"
}

// Send sends a webhook alert.
func (c *WebhookChannel) Send(ctx context.Context, config *models.AlertConfig, event *models.FIMEvent) error {
	var webhookConfig models.WebhookConfig
	if err := json.Unmarshal([]byte(config.Config), &webhookConfig); err != nil {
		return fmt.Errorf("invalid webhook config: %w", err)
	}

	if webhookConfig.URL == "" {
		return fmt.Errorf("no URL configured")
	}

	method := webhookConfig.Method
	if method == "" {
		method = "POST"
	}

	// Build payload
	payload := map[string]interface{}{
		"event": event,
		"config": map[string]interface{}{
			"id":      config.ID,
			"name":    config.Name,
			"channel": config.Channel,
		},
		"timestamp": time.Now().Unix(),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, webhookConfig.URL, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range webhookConfig.Headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	if c.Client != nil {
		client = c.Client
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}
