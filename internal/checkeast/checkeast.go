package checkeast

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"annet-oil/internal/annet"
)

// Error types (mirror internal/check/check.go style).
const (
	ErrStoreDisabled = "store_disabled"
	ErrUpload        = "upload_err"
	ErrEmptyDiff     = "empty_diff"
)

// Error describes why an archival step failed.
type Error struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// Artifact is one stored object.
type Artifact struct {
	Key         string `json:"key"`
	Location    string `json:"location"` // s3://bucket/key
	ContentType string `json:"content_type"`
	Size        int    `json:"size"`
	Error       *Error `json:"error,omitempty"`
}

// Report is the outcome of archiving a whole diff run.
type Report struct {
	RunID       string               `json:"run_id"`
	Timestamp   string               `json:"timestamp"`
	Bucket      string               `json:"bucket"`
	Combined    *Artifact            `json:"combined,omitempty"`
	PerHost     map[string]*Artifact `json:"per_host,omitempty"`
	StoredHosts int                  `json:"stored_hosts"`
	FailedHosts int                  `json:"failed_hosts"`
	Success     bool                 `json:"success"`
	Error       *Error               `json:"error,omitempty"`
}

// combinedPayload is the machine-readable whole-run archive: request metadata
// plus the full diff response, so a whole invocation can be correlated later.
type combinedPayload struct {
	RunID     string                 `json:"run_id"`
	Timestamp string                 `json:"timestamp"`
	Request   *annet.CommandRequest  `json:"request"`
	Response  *annet.CommandResponse `json:"response"`
}

// Archive stores the diff result. It does NOT run the diff — the caller runs the
// diff and passes the response in, keeping this package pure and unit-testable.
//
// It writes two kinds of objects under a per-run folder:
//
//	{prefix}{YYYY/MM/DD}/{runID}/diff.json.gz   — whole run (gzip JSON)
//	{prefix}{YYYY/MM/DD}/{runID}/{host}.diff    — raw per-host diff text
//
// A single failed object never aborts the whole run; failures are recorded on
// the corresponding Artifact and aggregated into the Report.
func (s *Store) Archive(ctx context.Context, req *annet.CommandRequest, resp *annet.CommandResponse) (*Report, error) {
	if s == nil {
		return &Report{
			Success: false,
			Error:   &Error{Type: ErrStoreDisabled, Message: "checkeast store is disabled"},
		}, nil
	}

	now := time.Now().UTC()
	ts := now.Format(time.RFC3339)
	runID := now.Format("20060102-150405")
	if token := hostToken(req, resp); token != "" {
		runID = runID + "-" + token
	}
	base := fmt.Sprintf("%s%s/%s", s.prefix, now.Format("2006/01/02"), runID)

	report := &Report{
		RunID:     runID,
		Timestamp: ts,
		Bucket:    s.bucket,
		PerHost:   map[string]*Artifact{},
		Success:   true,
	}

	// 1. Combined JSON (gzip).
	key := base + "/diff.json.gz"
	combined := &Artifact{Key: key, Location: s.location(key), ContentType: "application/gzip"}
	payload := combinedPayload{RunID: runID, Timestamp: ts, Request: req, Response: resp}
	if data, err := json.Marshal(payload); err != nil {
		combined.Error = &Error{Type: ErrUpload, Message: fmt.Sprintf("marshal combined payload: %v", err)}
		report.Success = false
	} else if n, err := s.putter.put(ctx, key, data, "application/gzip", true); err != nil {
		combined.Error = &Error{Type: ErrUpload, Message: err.Error()}
		report.Success = false
	} else {
		combined.Size = n
	}
	report.Combined = combined

	// 2. Per-host raw .diff (sorted for deterministic output).
	hosts := make([]string, 0, len(resp.Results))
	for h := range resp.Results {
		hosts = append(hosts, h)
	}
	sort.Strings(hosts)

	for _, host := range hosts {
		res := resp.Results[host]
		key := fmt.Sprintf("%s/%s.diff", base, host)
		art := &Artifact{Key: key, Location: s.location(key), ContentType: "text/plain"}

		body := ""
		if res != nil {
			body = res.Stdout
		}
		// An empty diff is annotated but still uploaded; it does not fail the run.
		if strings.TrimSpace(body) == "" {
			art.Error = &Error{Type: ErrEmptyDiff, Message: "diff is empty"}
		}

		if n, err := s.putter.put(ctx, key, []byte(body), "text/plain", false); err != nil {
			art.Error = &Error{Type: ErrUpload, Message: err.Error()}
			report.FailedHosts++
			report.Success = false
		} else {
			art.Size = n
			report.StoredHosts++
		}
		report.PerHost[host] = art
	}

	return report, nil
}

func (s *Store) location(key string) string {
	return "s3://" + s.bucket + "/" + key
}

// hostToken returns a short, filesystem-safe token identifying the run: the
// single host name when the run targets exactly one host, otherwise empty.
func hostToken(req *annet.CommandRequest, resp *annet.CommandResponse) string {
	if resp != nil && len(resp.Results) == 1 {
		for h := range resp.Results {
			return sanitizeToken(h)
		}
	}
	if req != nil && len(req.Filters) == 1 {
		return sanitizeToken(req.Filters[0])
	}
	return ""
}

// sanitizeToken keeps only characters safe for an S3 key segment.
func sanitizeToken(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}
