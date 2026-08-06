// Package alert provides alert channel implementations.
package alert

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/smtp"
	"strings"
	"time"

	"ojs-monitor/backend/internal/domain/models"
)

// EmailChannel implements email alert delivery via SMTP.
type EmailChannel struct{}

// Name returns the channel name.
func (c *EmailChannel) Name() string {
	return "email"
}

// Send sends an email alert via SMTP.
func (c *EmailChannel) Send(ctx context.Context, config *models.AlertConfig, event *models.FIMEvent) error {
	var emailConfig models.EmailConfig
	if err := json.Unmarshal([]byte(config.Config), &emailConfig); err != nil {
		return fmt.Errorf("invalid email config: %w", err)
	}

	// Validate config
	if len(emailConfig.Recipients) == 0 {
		return fmt.Errorf("no recipients configured")
	}
	if emailConfig.SMTPHost == "" {
		return fmt.Errorf("SMTP host not configured")
	}
	if emailConfig.SMTPPort == 0 {
		emailConfig.SMTPPort = 587 // Default to 587 (TLS)
	}
	if emailConfig.SMTPFrom == "" {
		emailConfig.SMTPFrom = "fim-monitor@localhost"
	}

	// Build email content
	subject := fmt.Sprintf("[FIM Alert] %s - %s", event.RiskLevel, event.EventType)
	if emailConfig.Subject != "" {
		subject = emailConfig.Subject
	}

	body := c.buildEmailBody(event, emailConfig)

	// Send email
	return c.sendEmail(emailConfig, emailConfig.Recipients, subject, body)
}

// buildEmailBody creates the email body from event.
func (c *EmailChannel) buildEmailBody(event *models.FIMEvent, config models.EmailConfig) string {
	if config.BodyType == "html" {
		return c.buildHTMLBody(event)
	}
	return c.buildTextBody(event)
}

// buildTextBody creates a plain text email body.
func (c *EmailChannel) buildTextBody(event *models.FIMEvent) string {
	return fmt.Sprintf(`File Integrity Alert

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
ALERT DETAILS
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Risk Level:     %s
Event Type:     %s
File Path:      %s
Project ID:     %d
Classification: %s
Source:         %s

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
TIMESTAMP
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

%s

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
DETAILS
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

%s

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
ACTOR INFORMATION
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Actor Type:     %s
Actor ID:       %s
Actor Name:     %s

This is an automated alert from FIM Monitor.
Do not reply to this email.
`,
		event.RiskLevel,
		event.EventType,
		event.FilePath,
		event.ProjectID,
		event.Classification,
		event.Source,
		time.Unix(parseTimestamp(event.Timestamp), 0).Format(time.RFC1123),
		event.Details,
		event.ActorType,
		event.ActorID,
		event.ActorName,
	)
}

// buildHTMLBody creates an HTML email body.
func (c *EmailChannel) buildHTMLBody(event *models.FIMEvent) string {
	riskColor := getRiskLevelColor(event.RiskLevel)
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; margin: 0; padding: 20px; background: #f5f5f5; }
        .container { max-width: 600px; margin: 0 auto; background: white; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .header { padding: 20px; border-bottom: 1px solid #eee; }
        .risk-badge { display: inline-block; padding: 4px 12px; border-radius: 4px; color: white; font-weight: bold; }
        .content { padding: 20px; }
        .detail-row { display: flex; padding: 8px 0; border-bottom: 1px solid #f0f0f0; }
        .detail-label { width: 140px; color: #666; font-weight: 500; }
        .detail-value { flex: 1; color: #333; }
        .footer { padding: 15px 20px; background: #f9f9f9; border-top: 1px solid #eee; border-radius: 0 0 8px 8px; font-size: 12px; color: #999; }
        .monospace { font-family: 'SF Mono', Monaco, monospace; background: #f5f5f5; padding: 2px 6px; border-radius: 3px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h2 style="margin: 0 0 10px 0; color: #333;">🔒 File Integrity Alert</h2>
            <span class="risk-badge" style="background: %s;">%s - %s</span>
        </div>
        <div class="content">
            <div class="detail-row">
                <div class="detail-label">File Path</div>
                <div class="detail-value"><code class="monospace">%s</code></div>
            </div>
            <div class="detail-row">
                <div class="detail-label">Project ID</div>
                <div class="detail-value">%d</div>
            </div>
            <div class="detail-row">
                <div class="detail-label">Classification</div>
                <div class="detail-value">%s</div>
            </div>
            <div class="detail-row">
                <div class="detail-label">Source</div>
                <div class="detail-value">%s</div>
            </div>
            <div class="detail-row">
                <div class="detail-label">Timestamp</div>
                <div class="detail-value">%s</div>
            </div>
            <div class="detail-row">
                <div class="detail-label">Actor</div>
                <div class="detail-value">%s (%s)</div>
            </div>
            %s
        </div>
        <div class="footer">
            This is an automated alert from FIM Monitor. Do not reply to this email.
        </div>
    </div>
</body>
</html>`,
		riskColor,
		event.RiskLevel,
		event.EventType,
		event.FilePath,
		event.ProjectID,
		event.Classification,
		event.Source,
		time.Unix(parseTimestamp(event.Timestamp), 0).Format(time.RFC1123),
		event.ActorName,
		event.ActorID,
		formatDetailsHTML(event.Details),
	)
}

// sendEmail sends an email via SMTP.
func (c *EmailChannel) sendEmail(config models.EmailConfig, recipients []string, subject, body string) error {
	// Build email message
	headers := make(map[string]string)
	headers["From"] = config.SMTPFrom
	headers["To"] = strings.Join(recipients, ", ")
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/plain; charset=UTF-8"

	var msg bytes.Buffer
	for k, v := range headers {
		msg.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	msg.WriteString("\r\n")
	msg.WriteString(body)

	// SMTP authentication
	auth := smtp.PlainAuth("", config.SMTPUsername, config.SMTPPassword, config.SMTPHost)

	// Connect to SMTP server
	addr := fmt.Sprintf("%s:%d", config.SMTPHost, config.SMTPPort)

	if config.UseTLS {
		return c.sendWithTLS(addr, config, recipients, msg.Bytes(), auth)
	}

	return smtp.SendMail(addr, auth, config.SMTPFrom, recipients, msg.Bytes())
}

// sendWithTLS sends email with TLS encryption.
func (c *EmailChannel) sendWithTLS(addr string, config models.EmailConfig, recipients []string, msg []byte, auth smtp.Auth) error {
	tlsConfig := &tls.Config{
		ServerName: config.SMTPHost,
	}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to connect via TLS: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, config.SMTPHost)
	if err != nil {
		return fmt.Errorf("failed to create SMTP client: %w", err)
	}
	defer client.Close()

	// Authenticate
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("SMTP authentication failed: %w", err)
	}

	// Set sender and recipients
	if err := client.Mail(config.SMTPFrom); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}
	for _, r := range recipients {
		if err := client.Rcpt(r); err != nil {
			return fmt.Errorf("failed to set recipient %s: %w", r, err)
		}
	}

	// Send message body
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to send data: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("failed to close message: %w", err)
	}

	return client.Quit()
}

// getRiskLevelColor returns HTML color for risk level.
func getRiskLevelColor(level string) string {
	switch level {
	case "CRITICAL":
		return "#dc3545"
	case "HIGH":
		return "#fd7e14"
	case "MEDIUM":
		return "#ffc107"
	default:
		return "#28a745"
	}
}

// parseTimestamp parses timestamp string to int64.
func parseTimestamp(ts string) int64 {
	if ts == "" {
		return time.Now().Unix()
	}
	// Try parsing as Unix timestamp
	var i int64
	fmt.Sscanf(ts, "%d", &i)
	if i > 0 {
		return i
	}
	// Try parsing as time string
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return time.Now().Unix()
	}
	return t.Unix()
}

// formatDetailsHTML formats event details as HTML.
func formatDetailsHTML(details string) string {
	if details == "" || details == "{}" {
		return ""
	}
	return fmt.Sprintf(`<div class="detail-row"><div class="detail-label">Details</div><div class="detail-value"><pre class="monospace" style="white-space: pre-wrap;">%s</pre></div></div>`, details)
}

// parseAddress parses host:port string.
func parseAddress(addr string) (host string, port int) {
	parts := strings.Split(addr, ":")
	host = parts[0]
	port = 587
	if len(parts) > 1 {
		fmt.Sscanf(parts[1], "%d", &port)
	}
	return
}

// dialTimeout is a helper for connecting with timeout.
func dialTimeout(network, addr string, timeout time.Duration) (net.Conn, error) {
	return net.DialTimeout(network, addr, timeout)
}

// ─────────────────────────────────────────────────────────────────────────────
// Slack Channel
// ─────────────────────────────────────────────────────────────────────────────

// SlackChannel implements Slack webhook alert delivery.
type SlackChannel struct{}

// Name returns the channel name.
func (c *SlackChannel) Name() string {
	return "slack"
}

// Send sends a Slack alert via webhook.
func (c *SlackChannel) Send(ctx context.Context, config *models.AlertConfig, event *models.FIMEvent) error {
	var slackConfig models.SlackConfig
	if err := json.Unmarshal([]byte(config.Config), &slackConfig); err != nil {
		return fmt.Errorf("invalid slack config: %w", err)
	}

	if slackConfig.WebhookURL == "" {
		return fmt.Errorf("webhook URL not configured")
	}

	// Build Slack message
	payload := c.buildSlackPayload(slackConfig, event)

	// Send webhook request
	return c.sendWebhook(slackConfig.WebhookURL, payload)
}

// buildSlackPayload creates the Slack message payload.
func (c *SlackChannel) buildSlackPayload(config models.SlackConfig, event *models.FIMEvent) map[string]interface{} {
	// Determine emoji based on risk level
	emoji := "🔔"
	switch event.RiskLevel {
	case "CRITICAL":
		emoji = "🚨"
	case "HIGH":
		emoji = "🔴"
	case "MEDIUM":
		emoji = "🟡"
	case "LOW":
		emoji = "🟢"
	}

	payload := map[string]interface{}{
		"username": config.Username,
		"icon_emoji": emoji,
	}

	if config.Channel != "" {
		payload["channel"] = config.Channel
	}

	// Build attachments
	attachment := map[string]interface{}{
		"color": getSlackColor(event.RiskLevel),
		"title": fmt.Sprintf("%s FIM Alert: %s", emoji, event.EventType),
		"fields": []map[string]interface{}{
			{
				"title": "Risk Level",
				"value": event.RiskLevel,
				"short": true,
			},
			{
				"title": "Classification",
				"value": event.Classification,
				"short": true,
			},
			{
				"title": "File Path",
				"value": event.FilePath,
				"short": false,
			},
			{
				"title": "Project ID",
				"value": fmt.Sprintf("%d", event.ProjectID),
				"short": true,
			},
			{
				"title": "Actor",
				"value": fmt.Sprintf("%s (%s)", event.ActorName, event.ActorID),
				"short": true,
			},
		},
		"footer": "FIM Monitor",
		"ts":     parseTimestamp(event.Timestamp),
	}

	payload["attachments"] = []map[string]interface{}{attachment}

	return payload
}

// getSlackColor returns Slack attachment color for risk level.
func getSlackColor(level string) string {
	switch level {
	case "CRITICAL":
		return "#dc3545"
	case "HIGH":
		return "#fd7e14"
	case "MEDIUM":
		return "#ffc107"
	default:
		return "#28a745"
	}
}

// sendWebhook sends HTTP POST to webhook URL.
func (c *SlackChannel) sendWebhook(url string, payload map[string]interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to send webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Webhook Channel
// ─────────────────────────────────────────────────────────────────────────────

// WebhookChannel implements custom webhook alert delivery.
type WebhookChannel struct{}

// Name returns the channel name.
func (c *WebhookChannel) Name() string {
	return "webhook"
}

// Send sends an alert to a custom webhook URL.
func (c *WebhookChannel) Send(ctx context.Context, config *models.AlertConfig, event *models.FIMEvent) error {
	var webhookConfig models.WebhookConfig
	if err := json.Unmarshal([]byte(config.Config), &webhookConfig); err != nil {
		return fmt.Errorf("invalid webhook config: %w", err)
	}

	if webhookConfig.URL == "" {
		return fmt.Errorf("webhook URL not configured")
	}

	// Build payload
	payload := c.buildWebhookPayload(event)

	// Determine method
	method := webhookConfig.Method
	if method == "" {
		method = "POST"
	}

	return c.sendRequest(webhookConfig.URL, method, webhookConfig.Headers, payload)
}

// buildWebhookPayload creates the webhook payload.
func (c *WebhookChannel) buildWebhookPayload(event *models.FIMEvent) map[string]interface{} {
	return map[string]interface{}{
		"event": map[string]interface{}{
			"id":             event.ID,
			"project_id":     event.ProjectID,
			"event_type":     event.EventType,
			"file_path":      event.FilePath,
			"file_hash":      event.FileHash,
			"actor_type":     event.ActorType,
			"actor_id":       event.ActorID,
			"actor_name":     event.ActorName,
			"risk_level":     event.RiskLevel,
			"classification": event.Classification,
			"source":         event.Source,
			"details":        event.Details,
			"timestamp":       event.Timestamp,
		},
		"meta": map[string]interface{}{
			"alerted_at": time.Now().Unix(),
			"channel":    "webhook",
		},
	}
}

// sendRequest sends HTTP request to webhook URL.
func (c *WebhookChannel) sendRequest(url, method string, headers map[string]string, payload map[string]interface{}) error {
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(payload); err != nil {
		return fmt.Errorf("failed to encode payload: %w", err)
	}

	req, err := http.NewRequest(method, url, &body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set default headers
	req.Header.Set("Content-Type", "application/json")

	// Set custom headers
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}
