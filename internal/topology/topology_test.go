package topology

import (
	"path/filepath"
	"testing"
	"time"
)

func nodeStatus(g Graph, host string) Status {
	for _, n := range g.Nodes {
		if n.Host == host {
			return n.Status
		}
	}
	return ""
}

func edgeStatus(g Graph, local, remote string) Status {
	for _, e := range g.Edges {
		if e.LocalHost == local && e.RemoteHost == remote {
			return e.Status
		}
	}
	return ""
}

func TestApplyStatusSubgraphAndDown(t *testing.T) {
	now := time.Now()
	path := filepath.Join(t.TempDir(), "topo.json")
	s := NewStore(path)

	// A up with links to B and C; B up with link to D.
	if err := s.Apply("A", true, []Edge{
		{LocalHost: "A", LocalPort: "e1", RemoteHost: "B", RemotePort: "e1", Source: SourceLLDP},
		{LocalHost: "A", LocalPort: "e2", RemoteHost: "C", RemotePort: "e1", Source: SourceLLDP},
	}, now); err != nil {
		t.Fatal(err)
	}
	if err := s.Apply("B", true, []Edge{
		{LocalHost: "B", LocalPort: "e2", RemoteHost: "D", RemotePort: "e1", Source: SourceLLDP},
	}, now); err != nil {
		t.Fatal(err)
	}

	full := s.Graph()
	if got := nodeStatus(full, "A"); got != StatusUp {
		t.Fatalf("A status = %q, want up", got)
	}
	if got := nodeStatus(full, "C"); got != StatusUnknown {
		t.Fatalf("C status = %q, want unknown (neighbor-only)", got)
	}
	if got := edgeStatus(full, "A", "B"); got != StatusUp {
		t.Fatalf("edge A->B = %q, want up", got)
	}

	// Depth filtering.
	if g := s.Subgraph("A", 1); len(g.Nodes) != 3 { // A,B,C
		t.Fatalf("depth 1 nodes = %d, want 3 (%v)", len(g.Nodes), g.Nodes)
	}
	if g := s.Subgraph("A", 2); len(g.Nodes) != 4 { // A,B,C,D
		t.Fatalf("depth 2 nodes = %d, want 4 (%v)", len(g.Nodes), g.Nodes)
	}
	if g := s.Subgraph("A", 0); len(g.Nodes) != 1 { // just A
		t.Fatalf("depth 0 nodes = %d, want 1", len(g.Nodes))
	}

	// A goes DOWN: node + its edges down, nothing deleted; B stays up.
	if err := s.Apply("A", false, nil, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	g := s.Graph()
	if got := nodeStatus(g, "A"); got != StatusDown {
		t.Fatalf("A after down = %q, want down", got)
	}
	if got := edgeStatus(g, "A", "B"); got != StatusDown {
		t.Fatalf("edge A->B after A down = %q, want down", got)
	}
	if got := nodeStatus(g, "B"); got != StatusUp {
		t.Fatalf("B should stay up, got %q", got)
	}
	if len(g.Edges) != 3 { // A->B, A->C, B->D all preserved
		t.Fatalf("edges after down = %d, want 3 (not deleted)", len(g.Edges))
	}

	// A back UP but link to C is gone -> A->B up, A->C down (stale, not deleted).
	if err := s.Apply("A", true, []Edge{
		{LocalHost: "A", LocalPort: "e1", RemoteHost: "B", RemotePort: "e1", Source: SourceLLDP},
	}, now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	g = s.Graph()
	if got := edgeStatus(g, "A", "B"); got != StatusUp {
		t.Fatalf("edge A->B re-up = %q, want up", got)
	}
	if got := edgeStatus(g, "A", "C"); got != StatusDown {
		t.Fatalf("stale edge A->C = %q, want down (kept)", got)
	}

	// Persistence round-trip.
	s2 := NewStore(path)
	if err := s2.Load(); err != nil {
		t.Fatal(err)
	}
	if got := edgeStatus(s2.Graph(), "A", "C"); got != StatusDown {
		t.Fatalf("after reload edge A->C = %q, want down", got)
	}
}
