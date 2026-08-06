package siem

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// SyslogChannel implements syslog forwarding per RFC 5424.
type SyslogChannel struct {
	*BaseClient
	Protocol string // "udp", "tcp", "tls"
	Host    string
	Port    int
}

// NewSyslogChannel creates a new syslog channel.
func NewSyslogChannel(config *Config) *SyslogChannel {
	return &SyslogChannel{
		BaseClient: NewBaseClient(config),
		Protocol:  "udp",
		Host:      config.Endpoint,
		Port:      514,
	}
}

// Name returns the channel name.
func (s *SyslogChannel) Name() string {
	return "syslog"
}

// Hostname returns the local hostname for syslog messages.
var Hostname = "ojs-monitor"

// AppName returns the application name for syslog messages.
var AppName = "ojs-monitor"

// SyslogSeverity maps risk levels to syslog severities.
var SyslogSeverity = map[string]int{
	"CRITICAL": 2, // Alert
	"HIGH":     3, // Error
	"MEDIUM":   4, // Warning
	"LOW":      6, // Informational
	"INFO":     6, // Informational
}

// SyslogFacility for local use.
const SyslogFacility = 16 // Local0

// Send sends events as syslog messages.
func (s *SyslogChannel) Send(ctx context.Context, events []Event) error {
	var conn net.Conn
	var err error

	// Create connection based on protocol
	switch s.Protocol {
	case "tls":
		tlsConfig := &tls.Config{
			InsecureSkipVerify: false,
		}
		conn, err = tls.Dial("tcp", fmt.Sprintf("%s:%d", s.Host, s.Port), tlsConfig)
	case "tcp":
		conn, err = net.Dial("tcp", fmt.Sprintf("%s:%d", s.Host, s.Port))
	default: // udp
		conn, err = net.Dial("udp", fmt.Sprintf("%s:%d", s.Host, s.Port))
	}

	if err != nil {
		return fmt.Errorf("syslog dial: %w", err)
	}
	defer conn.Close()

	// Send each event
	for _, event := range events {
		msg := s.formatSyslogMessage(event)
		_, err := conn.Write([]byte(msg))
		if err != nil {
			return fmt.Errorf("syslog write: %w", err)
		}
	}

	return nil
}

// formatSyslogMessage formats an event as RFC 5424 syslog message.
func (s *SyslogChannel) formatSyslogMessage(event Event) string {
	// Get severity
	severity := SyslogSeverity[event.RiskLevel]
	if severity == 0 {
		severity = 6 // INFO
	}

	// Calculate priority
	priority := SyslogFacility*8 + severity

	// Format timestamp (RFC 5424 format)
	timestamp := event.Timestamp.Format(time.RFC3339)

	// Build structured data
	structuredData := s.formatStructuredData(event)

	// Build message
	var msg strings.Builder
	msg.WriteString(fmt.Sprintf("<%d>", priority))
	msg.WriteString(timestamp)
	msg.WriteString(" ")
	msg.WriteString(Hostname)
	msg.WriteString(" ")
	msg.WriteString(AppName)
	msg.WriteString(" ")
	msg.WriteString(strconv.Itoa(os.Getpid()))
	msg.WriteString(" ")
	msg.WriteString(fmt.Sprintf("[%s@32473", event.SourceType))
	msg.WriteString(fmt.Sprintf(" event_type=\"%s\"", event.EventType))
	msg.WriteString(fmt.Sprintf("\" risk_level=\"%s\"", event.RiskLevel))
	msg.WriteString(fmt.Sprintf("\" source=\"%s\"", event.Source))
	msg.WriteString("]")
	msg.WriteString(structuredData)
	msg.WriteString(" ")
	msg.WriteString(s.formatMessage(event))

	return msg.String()
}

// formatStructuredData formats event data as syslog structured data.
func (s *SyslogChannel) formatStructuredData(event Event) string {
	var parts []string

	if event.Actor != nil {
		actorSD := fmt.Sprintf("[actor@32473 user_id=\"%s\" username=\"%s\"",
			event.Actor.UserID, event.Actor.Username)
		if event.Actor.ProcessID > 0 {
			actorSD += fmt.Sprintf(" process_id=\"%d\"", event.Actor.ProcessID)
		}
		if event.Actor.ProcessName != "" {
			actorSD += fmt.Sprintf(" process_name=\"%s\"", event.Actor.ProcessName)
		}
		if event.Actor.HostName != "" {
			actorSD += fmt.Sprintf(" hostname=\"%s\"", event.Actor.HostName)
		}
		actorSD += "]"
		parts = append(parts, actorSD)
	}

	if event.Target != nil && event.Target.Path != "" {
		targetSD := fmt.Sprintf("[target@32473 path=\"%s\"", event.Target.Path)
		if event.Target.FileType != "" {
			targetSD += fmt.Sprintf(" file_type=\"%s\"", event.Target.FileType)
		}
		if event.Target.NewHash != "" {
			targetSD += fmt.Sprintf(" hash=\"%s\"", event.Target.NewHash)
		}
		targetSD += "]"
		parts = append(parts, targetSD)
	}

	if len(parts) == 0 {
		return "-"
	}

	return strings.Join(parts, " ")
}

// formatMessage formats the human-readable message.
func (s *SyslogChannel) formatMessage(event Event) string {
	var parts []string

	parts = append(parts, fmt.Sprintf("%s event: %s", event.SourceType, event.EventType))

	if event.RiskLevel != "" {
		parts = append(parts, fmt.Sprintf("risk=%s", event.RiskLevel))
	}

	if event.Actor != nil {
		parts = append(parts, fmt.Sprintf("user=%s", event.Actor.Username))
	}

	if event.Target != nil && event.Target.Path != "" {
		parts = append(parts, fmt.Sprintf("path=%s", event.Target.Path))
	}

	return strings.Join(parts, " ")
}

// Test tests the syslog connection.
func (s *SyslogChannel) Test(ctx context.Context) error {
	// Try to connect
	var conn net.Conn
	var err error

	switch s.Protocol {
	case "tcp":
		conn, err = net.DialTimeout("tcp", fmt.Sprintf("%s:%d", s.Host, s.Port), 5*time.Second)
	case "tls":
		tlsConfig := &tls.Config{InsecureSkipVerify: false}
		conn, err = tls.DialWithDialer(&net.Dialer{Timeout: 5 * time.Second}, "tcp", fmt.Sprintf("%s:%d", s.Host, s.Port), tlsConfig)
	default:
		conn, err = net.DialTimeout("udp", fmt.Sprintf("%s:%d", s.Host, s.Port), 5*time.Second)
	}

	if err != nil {
		return fmt.Errorf("syslog test connect: %w", err)
	}
	defer conn.Close()

	// Send a test message
	testMsg := fmt.Sprintf("<134>%s %s %s %d [test@32473] test connection",
		time.Now().Format(time.RFC3339), Hostname, AppName, os.Getpid())
	_, err = conn.Write([]byte(testMsg))
	if err != nil {
		return fmt.Errorf("syslog test write: %w", err)
	}

	return nil
}

// ParseSyslogMessage parses a syslog message back to Event.
func ParseSyslogMessage(msg string) (*Event, error) {
	event := &Event{
		Metadata: make(map[string]interface{}),
	}

	// Remove priority if present
	if strings.HasPrefix(msg, "<") {
		idx := strings.Index(msg, ">")
		if idx > 0 {
			priStr := msg[1:idx]
			pri, _ := strconv.Atoi(priStr)
			event.RiskLevel = syslogPriorityToRiskLevel(pri)
			msg = msg[idx+1:]
		}
	}

	// Parse structured data
	sdRegex := regexp.MustCompile(`\[([^\]]+)\]`)
	matches := sdRegex.FindAllStringSubmatch(msg, -1)
	for _, match := range matches {
		parts := strings.Fields(match[1])
		if len(parts) >= 1 {
			switch parts[0] {
			case "event_type":
				if len(parts) >= 2 {
					event.EventType = parts[1]
				}
			case "risk_level":
				if len(parts) >= 2 {
					event.RiskLevel = parts[1]
				}
			case "source":
				if len(parts) >= 2 {
					event.Source = parts[1]
				}
			}
		}
	}

	// Parse timestamp
	tsRegex := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}`)
	tsMatch := tsRegex.FindString(msg)
	if tsMatch != "" {
		if ts, err := time.Parse(time.RFC3339, tsMatch); err == nil {
			event.Timestamp = ts
		}
	}

	return event, nil
}

// syslogPriorityToRiskLevel converts syslog priority to risk level.
func syslogPriorityToRiskLevel(pri int) string {
	severity := pri % 8
	switch severity {
	case 0:
		return "EMERGENCY"
	case 1:
		return "ALERT"
	case 2:
		return "CRITICAL"
	case 3:
		return "HIGH"
	case 4:
		return "MEDIUM"
	case 5:
		return "WARNING"
	case 6:
		return "INFO"
	default:
		return "LOW"
	}
}

// UDPClient is a simple UDP syslog client.
type UDPClient struct {
	Addr string
}

// NewUDPClient creates a new UDP syslog client.
func NewUDPClient(addr string) *UDPClient {
	return &UDPClient{Addr: addr}
}

// Send sends a syslog message over UDP.
func (c *UDPClient) Send(msg string) error {
	conn, err := net.Dial("udp", c.Addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	_, err = conn.Write([]byte(msg))
	return err
}
