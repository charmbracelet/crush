package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"
)

// SyslogExporter sends audit events to a remote syslog server.
type SyslogExporter struct {
	Network string // "udp" or "tcp"
	Addr    string // "siem.example.com:514"
}

func (s *SyslogExporter) Name() string { return "syslog" }

func (s *SyslogExporter) Export(_ context.Context, events []Event) error {
	conn, err := net.DialTimeout(s.Network, s.Addr, 5*time.Second)
	if err != nil {
		return fmt.Errorf("syslog connect failed: %w", err)
	}
	defer conn.Close()

	for _, ev := range events {
		msg := fmt.Sprintf("<%d>%s crush-secops[%s]: action=%s actor=%s resource=%s result=%s risk=%d",
			14*8+6, // facility=local0, severity=info
			ev.Timestamp.Format(time.RFC3339),
			ev.SessionID,
			ev.Action,
			ev.Actor,
			ev.Resource.Name,
			ev.Result.Status,
			ev.RiskScore,
		)
		if _, err := fmt.Fprintln(conn, msg); err != nil {
			return err
		}
	}
	return nil
}

// SplunkHECExporter sends audit events to Splunk via HTTP Event Collector.
type SplunkHECExporter struct {
	Endpoint string // "https://splunk.example.com:8088/services/collector/event"
	Token    string
	Index    string
	Source   string
	client   *http.Client
}

func NewSplunkHECExporter(endpoint, token, index string) *SplunkHECExporter {
	return &SplunkHECExporter{
		Endpoint: endpoint,
		Token:    token,
		Index:    index,
		Source:   "crush-secops",
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (s *SplunkHECExporter) Name() string { return "splunk_hec" }

func (s *SplunkHECExporter) Export(ctx context.Context, events []Event) error {
	for _, ev := range events {
		payload := map[string]interface{}{
			"time":       ev.Timestamp.Unix(),
			"sourcetype": "crush:secops:audit",
			"source":     s.Source,
			"index":      s.Index,
			"event":      ev,
		}
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.Endpoint, bytes.NewReader(data))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Splunk "+s.Token)
		req.Header.Set("Content-Type", "application/json")

		resp, err := s.client.Do(req)
		if err != nil {
			return fmt.Errorf("splunk HEC request failed: %w", err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("splunk HEC returned status %d", resp.StatusCode)
		}
	}
	return nil
}

// ELKExporter sends audit events to Elasticsearch/OpenSearch.
type ELKExporter struct {
	Endpoint string // "https://elasticsearch.example.com:9200"
	Index    string // "crush-secops-audit"
	Username string
	Password string
	client   *http.Client
}

func NewELKExporter(endpoint, index, username, password string) *ELKExporter {
	return &ELKExporter{
		Endpoint: endpoint,
		Index:    index,
		Username: username,
		Password: password,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (e *ELKExporter) Name() string { return "elasticsearch" }

func (e *ELKExporter) Export(ctx context.Context, events []Event) error {
	var buf bytes.Buffer
	for _, ev := range events {
		// NDJSON bulk format
		meta := fmt.Sprintf(`{"index":{"_index":"%s"}}`, e.Index)
		buf.WriteString(meta)
		buf.WriteByte('\n')
		data, err := json.Marshal(ev)
		if err != nil {
			return err
		}
		buf.Write(data)
		buf.WriteByte('\n')
	}

	url := fmt.Sprintf("%s/_bulk", e.Endpoint)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-ndjson")
	if e.Username != "" {
		req.SetBasicAuth(e.Username, e.Password)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("elasticsearch bulk request failed: %w", err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("elasticsearch returned status %d", resp.StatusCode)
	}
	return nil
}

// JSONFileExporter writes events to a JSON lines file for offline analysis.
type JSONFileExporter struct {
	FilePath string
}

func (j *JSONFileExporter) Name() string { return "json_file" }

func (j *JSONFileExporter) Export(_ context.Context, events []Event) error {
	data, err := json.MarshalIndent(events, "", "  ")
	if err != nil {
		return err
	}
	tmpPath := j.FilePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmpPath, j.FilePath)
}
