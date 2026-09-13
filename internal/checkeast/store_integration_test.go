package checkeast

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"annet-oil/internal/annet"
	"annet-oil/internal/config"
)

// capturedReq records one request the mock S3 server received.
type capturedReq struct {
	method string
	path   string
	query  string
	body   string
}

// mockS3 is a minimal S3-compatible server: it accepts PutObject and
// PutBucketLifecycleConfiguration and records every request.
func mockS3(t *testing.T) (*httptest.Server, *[]capturedReq, *sync.Mutex) {
	t.Helper()
	var mu sync.Mutex
	var reqs []capturedReq

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		reqs = append(reqs, capturedReq{
			method: r.Method,
			path:   r.URL.Path,
			query:  r.URL.RawQuery,
			body:   string(body),
		})
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	return srv, &reqs, &mu
}

// This exercises the real AWS S3 client (s3Putter, gzip, key layout) and the
// retention lifecycle call end-to-end over HTTP — no external infra required.
func TestNew_AppliesRetentionLifecycle_AndArchives(t *testing.T) {
	// Static credentials + disabled IMDS so the AWS default chain never hits the
	// network during LoadDefaultConfig / request signing.
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")

	srv, reqs, mu := mockS3(t)

	store, err := New(config.S3StoreConfig{
		Enabled:  true,
		Bucket:   "network-diffs",
		Prefix:   "diffs/annet-oil/",
		Region:   "us-east-1",
		Endpoint: srv.URL,
		// RetentionDays left at 0 → default 3 days.
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if store == nil {
		t.Fatal("expected a store, got nil")
	}

	// New should have applied a lifecycle rule expiring objects after 3 days.
	mu.Lock()
	var lifecycle *capturedReq
	for i := range *reqs {
		if strings.Contains((*reqs)[i].query, "lifecycle") {
			lifecycle = &(*reqs)[i]
		}
	}
	mu.Unlock()

	if lifecycle == nil {
		t.Fatal("expected a PutBucketLifecycleConfiguration request")
	}
	if lifecycle.method != http.MethodPut {
		t.Errorf("lifecycle method = %s, want PUT", lifecycle.method)
	}
	if !strings.Contains(lifecycle.body, "<Days>3</Days>") {
		t.Errorf("lifecycle body missing 3-day expiration:\n%s", lifecycle.body)
	}
	if !strings.Contains(lifecycle.body, "<Prefix>diffs/annet-oil/</Prefix>") {
		t.Errorf("lifecycle body missing prefix filter:\n%s", lifecycle.body)
	}

	// Archive a two-host run and assert the objects were PUT with the right keys.
	req := &annet.CommandRequest{Command: "diff", Filters: []string{"r1", "r2"}}
	report, err := store.Archive(context.Background(), req, resp(map[string]string{
		"r1": "+ added\n",
		"r2": "- removed\n",
	}))
	if err != nil {
		t.Fatalf("Archive: %v", err)
	}
	if !report.Success || report.StoredHosts != 2 {
		t.Fatalf("unexpected report: %+v", report)
	}

	mu.Lock()
	defer mu.Unlock()
	var sawCombined, sawR1, sawR2 bool
	for _, rq := range *reqs {
		if rq.method != http.MethodPut {
			continue
		}
		switch {
		case strings.HasSuffix(rq.path, "/diff.json.gz"):
			sawCombined = true
			// gzip magic bytes at the start of the (compressed) body.
			if len(rq.body) < 2 || rq.body[0] != 0x1f || rq.body[1] != 0x8b {
				t.Errorf("combined object is not gzip-compressed")
			}
		case strings.HasSuffix(rq.path, "/r1.diff"):
			sawR1 = true
			if !strings.Contains(rq.body, "+ added") {
				t.Errorf("r1.diff body = %q", rq.body)
			}
		case strings.HasSuffix(rq.path, "/r2.diff"):
			sawR2 = true
		}
	}
	if !sawCombined || !sawR1 || !sawR2 {
		t.Errorf("missing PUTs: combined=%v r1=%v r2=%v", sawCombined, sawR1, sawR2)
	}
}
