package checkeast

import (
	"context"
	"testing"
	"time"
)

// fakeReader implements objectReader from a fixed object set for read-path tests.
type fakeReader struct {
	objs    []ObjectInfo
	bodies  map[string][]byte
	listErr error
}

func (r *fakeReader) list(_ context.Context, prefix string) ([]ObjectInfo, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	var out []ObjectInfo
	for _, o := range r.objs {
		if len(prefix) == 0 || len(o.Key) >= len(prefix) && o.Key[:len(prefix)] == prefix {
			out = append(out, o)
		}
	}
	return out, nil
}

func (r *fakeReader) get(_ context.Context, key string) ([]byte, string, error) {
	return r.bodies[key], "text/plain", nil
}

func newReadStore(r *fakeReader) *Store {
	return &Store{reader: r, bucket: "network-diffs", prefix: "diffs/annet-oil/"}
}

func TestList_ParsesAndSortsNewestFirst(t *testing.T) {
	r := &fakeReader{objs: []ObjectInfo{
		{Key: "diffs/annet-oil/2026/09/12/20260912-101500-r1/r1.diff", Size: 12},
		{Key: "diffs/annet-oil/2026/09/12/20260912-120000-r2/r2.diff", Size: 34},
		// combined archive must be skipped by List:
		{Key: "diffs/annet-oil/2026/09/12/20260912-120000-r2/diff.json.gz", Size: 99},
	}}
	s := newReadStore(r)

	got, err := s.List(context.Background(), "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 diffs (json.gz skipped), got %d", len(got))
	}
	// Newest first: r2 (12:00) before r1 (10:15).
	if got[0].Host != "r2" || got[1].Host != "r1" {
		t.Fatalf("wrong order/hosts: %+v", got)
	}
	wantTS := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	if !got[0].Timestamp.Equal(wantTS) {
		t.Fatalf("timestamp parse: got %v want %v", got[0].Timestamp, wantTS)
	}
	if got[0].RunID != "20260912-120000-r2" {
		t.Fatalf("runID: got %q", got[0].RunID)
	}
}

func TestList_HostFilter(t *testing.T) {
	r := &fakeReader{objs: []ObjectInfo{
		{Key: "diffs/annet-oil/2026/09/12/20260912-101500-a/a.diff"},
		{Key: "diffs/annet-oil/2026/09/12/20260912-101500-b/b.diff"},
	}}
	s := newReadStore(r)

	got, err := s.List(context.Background(), "A") // case-insensitive
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 || got[0].Host != "a" {
		t.Fatalf("host filter failed: %+v", got)
	}
}

func TestGet_RejectsKeyOutsidePrefix(t *testing.T) {
	s := newReadStore(&fakeReader{bodies: map[string][]byte{}})
	if _, _, err := s.Get(context.Background(), "other/secret.txt"); err == nil {
		t.Fatal("expected error for key outside prefix")
	}
}

func TestGet_ReturnsBodyUnderPrefix(t *testing.T) {
	key := "diffs/annet-oil/2026/09/12/20260912-101500-a/a.diff"
	s := newReadStore(&fakeReader{bodies: map[string][]byte{key: []byte("+ hi")}})
	body, _, err := s.Get(context.Background(), key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(body) != "+ hi" {
		t.Fatalf("body: %q", body)
	}
}

func TestList_NilStore(t *testing.T) {
	var s *Store
	got, err := s.List(context.Background(), "")
	if err != nil || got != nil {
		t.Fatalf("nil store List should be (nil,nil), got %v %v", got, err)
	}
}
