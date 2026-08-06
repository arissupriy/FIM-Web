package siem

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ElasticsearchClient implements SIEM client for Elasticsearch.
type ElasticsearchClient struct {
	*BaseClient
	Index string
}

// NewElasticsearchClient creates a new Elasticsearch client.
func NewElasticsearchClient(config *Config) *ElasticsearchClient {
	if config.Index == "" {
		config.Index = "ojs-monitor-fim"
	}

	return &ElasticsearchClient{
		BaseClient: NewBaseClient(config),
		Index:      config.Index,
	}
}

// Name returns the client name.
func (e *ElasticsearchClient) Name() string {
	return "elasticsearch"
}

// Send sends events to Elasticsearch via bulk API.
func (e *ElasticsearchClient) Send(ctx context.Context, events []Event) error {
	if len(events) == 0 {
		return nil
	}

	// Build bulk request body
	var body bytes.Buffer
	for _, event := range events {
		// Index action
		action := map[string]interface{}{
			"index": map[string]interface{}{
				"_index": e.Index,
			},
		}
		actionBytes, _ := json.Marshal(action)
		body.Write(actionBytes)
		body.WriteByte('\n')

		// Document
		doc := map[string]interface{}{
			"@timestamp": event.Timestamp.Format(time.RFC3339Nano),
			"source":     event.Source,
			"source_type": event.SourceType,
			"event_type": event.EventType,
			"risk_level": event.RiskLevel,
		}

		if event.Actor != nil {
			doc["actor"] = map[string]interface{}{
				"user_id":      event.Actor.UserID,
				"username":     event.Actor.Username,
				"process_id":   event.Actor.ProcessID,
				"process_name": event.Actor.ProcessName,
				"session_id":  event.Actor.SessionID,
				"tty":          event.Actor.TTY,
				"hostname":     event.Actor.HostName,
				"ip_address":   event.Actor.IPAddress,
			}
		}

		if event.Target != nil {
			doc["target"] = map[string]interface{}{
				"path":     event.Target.Path,
				"file_type": event.Target.FileType,
				"old_hash": event.Target.OldHash,
				"new_hash": event.Target.NewHash,
				"old_perm": event.Target.OldPerm,
				"new_perm": event.Target.NewPerm,
				"old_owner": event.Target.OldOwner,
				"new_owner": event.Target.NewOwner,
			}
		}

		if len(event.Changes) > 0 {
			changes := make([]map[string]interface{}, 0, len(event.Changes))
			for _, c := range event.Changes {
				changes = append(changes, map[string]interface{}{
					"field":     c.Field,
					"old_value": c.OldValue,
					"new_value": c.NewValue,
				})
			}
			doc["changes"] = changes
		}

		if event.Metadata != nil {
			doc["metadata"] = event.Metadata
		}

		if event.RawData != nil {
			doc["raw_data"] = event.RawData
		}

		docBytes, _ := json.Marshal(doc)
		body.Write(docBytes)
		body.WriteByte('\n')
	}

	// Send bulk request
	url := fmt.Sprintf("%s/_bulk", e.Config.Endpoint)
	return e.SendWithRetry(ctx, &body, url)
}

// SendWithRetry sends bulk data with retry logic.
func (e *ElasticsearchClient) SendWithRetry(ctx context.Context, body *bytes.Buffer, url string) error {
	var lastErr error

	for i := 0; i <= e.Config.RetryCount; i++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
		if err != nil {
			return err
		}

		req.Header.Set("Content-Type", "application/x-ndjson")
		if e.Config.APIKey != "" {
			req.Header.Set("Authorization", "ApiKey "+e.Config.APIKey)
		}

		resp, err := e.HTTPClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		defer resp.Body.Close()

		respBody, _ := io.ReadAll(resp.Body)

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			// Check for errors in bulk response
			var bulkResp BulkResponse
			if err := json.Unmarshal(respBody, &bulkResp); err == nil {
				if bulkResp.Errors {
					return fmt.Errorf("elasticsearch bulk had errors")
				}
			}
			return nil
		}

		lastErr = fmt.Errorf("elasticsearch returned %d: %s", resp.StatusCode, string(respBody))

		// Don't retry on client errors
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return lastErr
		}
	}

	return lastErr
}

// BulkResponse represents Elasticsearch bulk API response.
type BulkResponse struct {
	Took   int  `json:"took"`
	Errors bool `json:"errors"`
	Items  []struct {
		Index struct {
			ID     string `json:"_id"`
			Status int    `json:"status"`
			Error  struct {
				Type   string `json:"type"`
				Reason string `json:"reason"`
			} `json:"error,omitempty"`
		} `json:"index"`
	} `json:"items"`
}

// Test tests the Elasticsearch connection.
func (e *ElasticsearchClient) Test(ctx context.Context) error {
	url := fmt.Sprintf("%s/_cluster/health", e.Config.Endpoint)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	if e.Config.APIKey != "" {
		req.Header.Set("Authorization", "ApiKey "+e.Config.APIKey)
	}

	resp, err := e.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("elasticsearch test: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("elasticsearch test returned %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// CreateIndexTemplate creates an index template for FIM events.
func (e *ElasticsearchClient) CreateIndexTemplate(ctx context.Context) error {
	template := map[string]interface{}{
		"index_patterns": []string{e.Index + "*"},
		"template": map[string]interface{}{
			"settings": map[string]interface{}{
				"number_of_shards":   1,
				"number_of_replicas": 1,
				"index.lifecycle.name": "ojs-monitor-policy",
			},
			"mappings": map[string]interface{}{
				"properties": map[string]interface{}{
					"@timestamp": map[string]string{"type": "date"},
					"source":     map[string]string{"type": "keyword"},
					"source_type": map[string]string{"type": "keyword"},
					"event_type": map[string]string{"type": "keyword"},
					"risk_level": map[string]string{"type": "keyword"},
					"actor": map[string]interface{}{
						"properties": map[string]interface{}{
							"user_id":      map[string]string{"type": "keyword"},
							"username":     map[string]string{"type": "keyword"},
							"process_id":   map[string]string{"type": "long"},
							"process_name": map[string]string{"type": "keyword"},
							"session_id":  map[string]string{"type": "long"},
							"tty":          map[string]string{"type": "keyword"},
							"hostname":     map[string]string{"type": "keyword"},
							"ip_address":   map[string]string{"type": "ip"},
						},
					},
					"target": map[string]interface{}{
						"properties": map[string]interface{}{
							"path":      map[string]string{"type": "keyword"},
							"file_type": map[string]string{"type": "keyword"},
							"old_hash":  map[string]string{"type": "keyword"},
							"new_hash":  map[string]string{"type": "keyword"},
							"old_perm":  map[string]string{"type": "keyword"},
							"new_perm":  map[string]string{"type": "keyword"},
						},
					},
					"changes": map[string]interface{}{
						"type": "nested",
						"properties": map[string]interface{}{
							"field":     map[string]string{"type": "keyword"},
							"old_value": map[string]string{"type": "text"},
							"new_value": map[string]string{"type": "text"},
						},
					},
				},
			},
		},
	}

	body, _ := json.Marshal(template)
	url := fmt.Sprintf("%s/_index_template/%s-template", e.Config.Endpoint, e.Index)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	if e.Config.APIKey != "" {
		req.Header.Set("Authorization", "ApiKey "+e.Config.APIKey)
	}

	resp, err := e.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("create template: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create template returned %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// SearchEvents searches for events in Elasticsearch.
func (e *ElasticsearchClient) SearchEvents(ctx context.Context, query map[string]interface{}) ([]Event, error) {
	body, _ := json.Marshal(query)

	url := fmt.Sprintf("%s/%s/_search", e.Config.Endpoint, e.Index)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if e.Config.APIKey != "" {
		req.Header.Set("Authorization", "ApiKey "+e.Config.APIKey)
	}

	resp, err := e.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("search returned %d: %s", resp.StatusCode, string(respBody))
	}

	var searchResp struct {
		Hits struct {
			Hits []struct {
				Source json.RawMessage `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	var events []Event
	for _, hit := range searchResp.Hits.Hits {
		var event Event
		if err := json.Unmarshal(hit.Source, &event); err == nil {
			events = append(events, event)
		}
	}

	return events, nil
}

// BuildQuery builds an Elasticsearch query from parameters.
func BuildQuery(eventType, riskLevel, path, userID string, from, to time.Time) map[string]interface{} {
	must := []map[string]interface{}{}

	if eventType != "" {
		must = append(must, map[string]interface{}{
			"term": map[string]string{"event_type": eventType},
		})
	}

	if riskLevel != "" {
		must = append(must, map[string]interface{}{
			"term": map[string]string{"risk_level": riskLevel},
		})
	}

	if path != "" {
		must = append(must, map[string]interface{}{
			"wildcard": map[string]string{"target.path": path},
		})
	}

	if userID != "" {
		must = append(must, map[string]interface{}{
			"term": map[string]string{"actor.user_id": userID},
		})
	}

	rangeQuery := map[string]interface{}{}
	if !from.IsZero() {
		rangeQuery["gte"] = from.Format(time.RFC3339)
	}
	if !to.IsZero() {
		rangeQuery["lte"] = to.Format(time.RFC3339)
	}
	if len(rangeQuery) > 0 {
		must = append(must, map[string]interface{}{
			"range": map[string]interface{}{
				"@timestamp": rangeQuery,
			},
		})
	}

	query := map[string]interface{}{
		"size": 100,
		"sort": []map[string]interface{}{
			{"@timestamp": map[string]string{"order": "desc"}},
		},
	}

	if len(must) > 0 {
		query["query"] = map[string]interface{}{
			"bool": map[string]interface{}{
				"must": must,
			},
		}
	} else {
		query["query"] = map[string]interface{}{
			"match_all": map[string]interface{}{},
		}
	}

	return query
}

// SplunkHECClient implements SIEM client for Splunk HEC.
type SplunkHECClient struct {
	*BaseClient
	Channel string
}

// NewSplunkHECClient creates a new Splunk HEC client.
func NewSplunkHECClient(config *Config) *SplunkHECClient {
	if config.Channel == "" {
		config.Channel = "ojs-monitor"
	}

	return &SplunkHECClient{
		BaseClient: NewBaseClient(config),
		Channel:   config.Channel,
	}
}

// Name returns the client name.
func (s *SplunkHECClient) Name() string {
	return "splunk-hec"
}

// Send sends events to Splunk via HEC.
func (s *SplunkHECClient) Send(ctx context.Context, events []Event) error {
	for _, event := range events {
		// Build HEC event
		hecEvent := map[string]interface{}{
			"time": float64(event.Timestamp.UnixNano()) / 1e9,
			"host": Hostname,
			"source": s.Channel,
			"sourcetype": "ojs-monitor:fim",
			"event": event,
		}

		body, _ := json.Marshal(hecEvent)
		url := fmt.Sprintf("%s/services/collector", s.Config.Endpoint)

		if err := s.SendWithRetry(ctx, bytes.NewReader(body), url); err != nil {
			return fmt.Errorf("splunk send: %w", err)
		}
	}

	return nil
}

// Test tests the Splunk HEC connection.
func (s *SplunkHECClient) Test(ctx context.Context) error {
	// Send a test event
	testEvent := map[string]interface{}{
		"time": float64(time.Now().Unix()),
		"host": Hostname,
		"source": s.Channel,
		"sourcetype": "ojs-monitor:test",
		"event": map[string]string{"message": "test connection"},
	}

	body, _ := json.Marshal(testEvent)
	url := fmt.Sprintf("%s/services/collector", s.Config.Endpoint)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Splunk "+s.Config.APIKey)

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("splunk test: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("splunk test returned %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}
