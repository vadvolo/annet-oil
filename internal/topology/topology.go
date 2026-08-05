// Package topology builds and stores a network topology graph from per-device
// neighbor data (LLDP, CDP, IP/ARP) collected via the opstate module.
//
// Nodes and edges carry a status (up/down/unknown). A re-collect does NOT delete
// data: if a device is unreachable its node and its edges are marked "down"
// (topology is preserved); if it is reachable, its node is "up", its currently
// reported links are "up", and links it no longer reports are marked "down".
package topology

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"annet-oil/internal/opstate"
)

// Source is where an edge was discovered.
type Source string

const (
	SourceLLDP Source = "lldp"
	SourceCDP  Source = "cdp"
	SourceIP   Source = "ip"
)

// Status is the up/down state of a node or edge.
type Status string

const (
	StatusUp      Status = "up"
	StatusDown    Status = "down"
	StatusUnknown Status = "unknown" // discovered only as a neighbor, never polled directly
)

// Node is a device in the graph.
type Node struct {
	Host     string    `json:"host"`
	Status   Status    `json:"status"`
	MgmtIP   string    `json:"mgmt_ip,omitempty"`
	LastSeen time.Time `json:"last_seen,omitempty"`
}

// Edge is one discovered link from LocalHost to RemoteHost.
type Edge struct {
	LocalHost    string    `json:"local_host"`
	LocalPort    string    `json:"local_port,omitempty"`
	RemoteHost   string    `json:"remote_host"`
	RemotePort   string    `json:"remote_port,omitempty"`
	RemoteMgmtIP string    `json:"remote_mgmt_ip,omitempty"`
	Source       Source    `json:"source"`
	Status       Status    `json:"status"`
	LastSeen     time.Time `json:"last_seen,omitempty"`
}

func (e Edge) key() string {
	return strings.Join([]string{e.LocalHost, e.LocalPort, e.RemoteHost, e.RemotePort, string(e.Source)}, "|")
}

// Graph is a set of nodes and the edges among them.
type Graph struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

// EdgesFromState extracts topology edges from one device's collected state.
// If sources is empty, LLDP and IP are used. Status/LastSeen are set by the store.
func EdgesFromState(st *opstate.State, sources []Source) []Edge {
	want := map[Source]bool{}
	for _, s := range sources {
		want[s] = true
	}
	if len(sources) == 0 {
		want[SourceLLDP] = true
		want[SourceIP] = true
	}
	var out []Edge
	if want[SourceLLDP] {
		for _, n := range st.LLDP {
			rh := n.RemoteSystem
			if rh == "" {
				rh = n.RemoteMgmtIP
			}
			if rh == "" {
				continue
			}
			out = append(out, Edge{LocalHost: st.Host, LocalPort: n.LocalPort, RemoteHost: rh, RemotePort: n.RemotePort, RemoteMgmtIP: n.RemoteMgmtIP, Source: SourceLLDP})
		}
	}
	if want[SourceIP] {
		for _, a := range st.ARP {
			if a.IP == "" {
				continue
			}
			out = append(out, Edge{LocalHost: st.Host, LocalPort: a.Interface, RemoteHost: a.IP, RemoteMgmtIP: a.IP, Source: SourceIP})
		}
	}
	return out
}

// Store persists nodes and edges to a JSON file.
type Store struct {
	path  string
	mu    sync.Mutex
	nodes map[string]*Node
	edges map[string]*Edge
}

type persisted struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

// NewStore creates a store backed by path.
func NewStore(path string) *Store {
	return &Store{path: path, nodes: map[string]*Node{}, edges: map[string]*Edge{}}
}

// Load reads the store from disk (no error if the file does not exist yet).
func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var p persisted
	if err := json.Unmarshal(b, &p); err != nil {
		return err
	}
	s.nodes = make(map[string]*Node, len(p.Nodes))
	s.edges = make(map[string]*Edge, len(p.Edges))
	for i := range p.Nodes {
		n := p.Nodes[i]
		s.nodes[n.Host] = &n
	}
	for i := range p.Edges {
		e := p.Edges[i]
		s.edges[e.key()] = &e
	}
	return nil
}

func (s *Store) save() error {
	p := persisted{Nodes: s.nodesLocked(), Edges: s.edgesLocked()}
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	if dir := filepath.Dir(s.path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// Apply records the result of collecting one device.
//   - reachable:  node -> up; reported edges -> up; edges from this host no longer
//     reported -> down (link gone, not deleted).
//   - unreachable: node -> down; all edges from this host -> down. Nothing deleted.
func (s *Store) Apply(host string, reachable bool, fresh []Edge, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ln := s.nodes[host]
	if ln == nil {
		ln = &Node{Host: host}
		s.nodes[host] = ln
	}
	if !reachable {
		ln.Status = StatusDown
		for _, e := range s.edges {
			if e.LocalHost == host {
				e.Status = StatusDown
			}
		}
		return s.save()
	}
	ln.Status = StatusUp
	ln.LastSeen = now
	freshKeys := make(map[string]bool, len(fresh))
	for i := range fresh {
		e := fresh[i]
		e.Status = StatusUp
		e.LastSeen = now
		k := e.key()
		s.edges[k] = &e
		freshKeys[k] = true
		s.ensureRemoteLocked(e, now)
	}
	for k, e := range s.edges {
		if e.LocalHost == host && !freshKeys[k] {
			e.Status = StatusDown
		}
	}
	return s.save()
}

// ensureRemoteLocked makes sure a neighbor node exists (status unknown until it
// is itself polled). Records its mgmt IP when learned.
func (s *Store) ensureRemoteLocked(e Edge, now time.Time) {
	if e.RemoteHost == "" {
		return
	}
	n := s.nodes[e.RemoteHost]
	if n == nil {
		s.nodes[e.RemoteHost] = &Node{Host: e.RemoteHost, Status: StatusUnknown}
		n = s.nodes[e.RemoteHost]
	}
	if e.RemoteMgmtIP != "" && n.MgmtIP == "" {
		n.MgmtIP = e.RemoteMgmtIP
	}
}

func (s *Store) nodesLocked() []Node {
	out := make([]Node, 0, len(s.nodes))
	for _, n := range s.nodes {
		out = append(out, *n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Host < out[j].Host })
	return out
}

func (s *Store) edgesLocked() []Edge {
	out := make([]Edge, 0, len(s.edges))
	for _, e := range s.edges {
		out = append(out, *e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].key() < out[j].key() })
	return out
}

// Graph returns the full stored graph.
func (s *Store) Graph() Graph {
	s.mu.Lock()
	defer s.mu.Unlock()
	return Graph{Nodes: s.nodesLocked(), Edges: s.edgesLocked()}
}

// Subgraph returns the induced subgraph of nodes within `depth` hops of host
// (depth 0 = just host, 1 = host + direct neighbors, ...). Undirected reachability.
func (s *Store) Subgraph(host string, depth int) Graph {
	s.mu.Lock()
	defer s.mu.Unlock()
	edges := s.edgesLocked()
	adj := map[string]map[string]bool{}
	link := func(a, b string) {
		if a == "" || b == "" || a == b {
			return
		}
		if adj[a] == nil {
			adj[a] = map[string]bool{}
		}
		adj[a][b] = true
	}
	for _, e := range edges {
		link(e.LocalHost, e.RemoteHost)
		link(e.RemoteHost, e.LocalHost)
		if e.RemoteMgmtIP != "" && e.RemoteMgmtIP != e.RemoteHost {
			link(e.LocalHost, e.RemoteMgmtIP)
			link(e.RemoteMgmtIP, e.LocalHost)
		}
	}
	within := map[string]bool{host: true}
	frontier := []string{host}
	for d := 0; d < depth; d++ {
		var next []string
		for _, n := range frontier {
			for m := range adj[n] {
				if !within[m] {
					within[m] = true
					next = append(next, m)
				}
			}
		}
		if len(next) == 0 {
			break
		}
		frontier = next
	}
	var subEdges []Edge
	for _, e := range edges {
		if within[e.LocalHost] && (within[e.RemoteHost] || (e.RemoteMgmtIP != "" && within[e.RemoteMgmtIP])) {
			subEdges = append(subEdges, e)
		}
	}
	if subEdges == nil {
		subEdges = []Edge{}
	}
	var nodes []Node
	for h := range within {
		if n := s.nodes[h]; n != nil {
			nodes = append(nodes, *n)
		} else {
			nodes = append(nodes, Node{Host: h, Status: StatusUnknown})
		}
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].Host < nodes[j].Host })
	return Graph{Nodes: nodes, Edges: subEdges}
}
