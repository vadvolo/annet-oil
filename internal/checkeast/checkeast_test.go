package checkeast

import (
	"context"
	"errors"
	"strings"
	"testing"

	"annet-oil/internal/annet"
)

// fakePutter records every put call and can be told to fail specific keys.
type fakePutter struct {
	calls   []putCall
	failKey string
	failErr error
}

type putCall struct {
	key         string
	body        []byte
	contentType string
	gzipped     bool
}

func (f *fakePutter) put(_ context.Context, key string, body []byte, contentType string, gzipped bool) (int, error) {
	f.calls = append(f.calls, putCall{key: key, body: body, contentType: contentType, gzipped: gzipped})
	if f.failKey != "" && strings.HasSuffix(key, f.failKey) {
		return 0, f.failErr
	}
	return len(body), nil
}

func newTestStore(f *fakePutter) *Store {
	return &Store{putter: f, bucket: "network-diffs", prefix: "diffs/annet-oil/"}
}

func resp(results map[string]string) *annet.CommandResponse {
	r := &annet.CommandResponse{Results: map[string]*annet.CommandResult{}, Success: true}
	for host, out := range results {
		r.Results[host] = &annet.CommandResult{Stdout: out, ExitCode: 0}
	}
	r.TotalHosts = len(results)
	r.SuccessHosts = len(results)
	return r
}

func TestArchive_BothArtifacts(t *testing.T) {
	f := &fakePutter{}
	s := newTestStore(f)
	req := &annet.CommandRequest{Command: "diff", Filters: []string{"r1", "r2"}}

	report, err := s.Archive(context.Background(), req, resp(map[string]string{
		"r1": "+ line added\n",
		"r2": "- line removed\n",
	}))
	if err != nil {
		t.Fatalf("Archive returned error: %v", err)
	}
	if !report.Success {
		t.Fatalf("expected success, got report: %+v", report)
	}
	if report.StoredHosts != 2 || report.FailedHosts != 0 {
		t.Fatalf("stored=%d failed=%d, want 2/0", report.StoredHosts, report.FailedHosts)
	}

	// Combined artifact: gzipped JSON under diff.json.gz.
	if report.Combined == nil || report.Combined.Error != nil {
		t.Fatalf("combined artifact missing or errored: %+v", report.Combined)
	}
	if !strings.HasSuffix(report.Combined.Key, "/diff.json.gz") {
		t.Errorf("combined key = %q, want suffix /diff.json.gz", report.Combined.Key)
	}
	if !strings.HasPrefix(report.Combined.Key, "diffs/annet-oil/") {
		t.Errorf("combined key = %q, want prefix diffs/annet-oil/", report.Combined.Key)
	}
	if report.Combined.Location != "s3://network-diffs/"+report.Combined.Key {
		t.Errorf("combined location = %q", report.Combined.Location)
	}

	// Per-host artifacts.
	for _, host := range []string{"r1", "r2"} {
		art := report.PerHost[host]
		if art == nil || art.Error != nil {
			t.Fatalf("per-host %s missing or errored: %+v", host, art)
		}
		if !strings.HasSuffix(art.Key, "/"+host+".diff") {
			t.Errorf("per-host %s key = %q", host, art.Key)
		}
		if art.ContentType != "text/plain" {
			t.Errorf("per-host %s content type = %q", host, art.ContentType)
		}
	}

	// Three puts total: 1 combined + 2 per-host, all under the same run folder.
	if len(f.calls) != 3 {
		t.Fatalf("expected 3 put calls, got %d", len(f.calls))
	}
	var combinedGzipped bool
	runFolders := map[string]struct{}{}
	for _, c := range f.calls {
		runFolders[c.key[:strings.LastIndex(c.key, "/")]] = struct{}{}
		if strings.HasSuffix(c.key, "diff.json.gz") {
			combinedGzipped = c.gzipped
			if c.contentType != "application/gzip" {
				t.Errorf("combined content type = %q", c.contentType)
			}
		}
	}
	if !combinedGzipped {
		t.Error("combined object should be gzipped")
	}
	if len(runFolders) != 1 {
		t.Errorf("expected 1 run folder, got %d: %v", len(runFolders), runFolders)
	}
}

func TestArchive_SingleHostRunIDToken(t *testing.T) {
	f := &fakePutter{}
	s := newTestStore(f)
	req := &annet.CommandRequest{Command: "diff", Filters: []string{"core-sw1"}}

	report, _ := s.Archive(context.Background(), req, resp(map[string]string{"core-sw1": "+ x\n"}))
	if !strings.HasSuffix(report.RunID, "-core-sw1") {
		t.Errorf("runID = %q, want single-host token suffix", report.RunID)
	}
}

func TestArchive_EmptyDiffAnnotatedNotFailed(t *testing.T) {
	f := &fakePutter{}
	s := newTestStore(f)
	req := &annet.CommandRequest{Command: "diff", Filters: []string{"r1"}}

	report, _ := s.Archive(context.Background(), req, resp(map[string]string{"r1": "   \n"}))
	if !report.Success {
		t.Errorf("empty diff must not fail the run: %+v", report)
	}
	art := report.PerHost["r1"]
	if art == nil || art.Error == nil || art.Error.Type != ErrEmptyDiff {
		t.Errorf("empty diff should be annotated with %q, got %+v", ErrEmptyDiff, art)
	}
	if report.StoredHosts != 1 {
		t.Errorf("empty diff still stored: stored=%d", report.StoredHosts)
	}
}

func TestArchive_PerHostUploadFailureAggregated(t *testing.T) {
	f := &fakePutter{failKey: "r2.diff", failErr: errors.New("network down")}
	s := newTestStore(f)
	req := &annet.CommandRequest{Command: "diff", Filters: []string{"r1", "r2"}}

	report, _ := s.Archive(context.Background(), req, resp(map[string]string{
		"r1": "+ a\n",
		"r2": "+ b\n",
	}))
	if report.Success {
		t.Error("run with a failed upload should not be Success")
	}
	if report.StoredHosts != 1 || report.FailedHosts != 1 {
		t.Errorf("stored=%d failed=%d, want 1/1", report.StoredHosts, report.FailedHosts)
	}
	if report.PerHost["r2"].Error == nil || report.PerHost["r2"].Error.Type != ErrUpload {
		t.Errorf("r2 should carry an upload error, got %+v", report.PerHost["r2"])
	}
	if report.PerHost["r1"].Error != nil {
		t.Errorf("r1 should have succeeded, got %+v", report.PerHost["r1"])
	}
}

func TestArchive_CombinedUploadFailure(t *testing.T) {
	f := &fakePutter{failKey: "diff.json.gz", failErr: errors.New("boom")}
	s := newTestStore(f)
	req := &annet.CommandRequest{Command: "diff", Filters: []string{"r1"}}

	report, _ := s.Archive(context.Background(), req, resp(map[string]string{"r1": "+ a\n"}))
	if report.Success {
		t.Error("combined upload failure should fail the run")
	}
	if report.Combined.Error == nil || report.Combined.Error.Type != ErrUpload {
		t.Errorf("combined should carry upload error, got %+v", report.Combined)
	}
	// Per-host upload still proceeds despite combined failure.
	if report.StoredHosts != 1 {
		t.Errorf("per-host upload should still run, stored=%d", report.StoredHosts)
	}
}

func TestArchive_NilStoreDisabled(t *testing.T) {
	var s *Store
	report, err := s.Archive(context.Background(), &annet.CommandRequest{Command: "diff"}, resp(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Success || report.Error == nil || report.Error.Type != ErrStoreDisabled {
		t.Errorf("nil store should report %q, got %+v", ErrStoreDisabled, report)
	}
}
