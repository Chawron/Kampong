package graph

import (
	"github.com/kampong/debate/internal/models"
)

// Store is an in-memory knowledge graph.
type Store struct {
	nodes map[string]models.GraphNode
	edges []models.GraphEdge
}

// NewStore creates an empty graph store.
func NewStore() *Store {
	return &Store{
		nodes: make(map[string]models.GraphNode),
	}
}

// AddNodes adds nodes, deduplicating by ID.
func (s *Store) AddNodes(nodes []models.GraphNode) {
	for _, n := range nodes {
		if existing, ok := s.nodes[n.ID]; ok {
			existing.Refs++
			s.nodes[n.ID] = existing
		} else {
			n.Refs = 1
			s.nodes[n.ID] = n
		}
	}
}

// AddEdges appends edges.
func (s *Store) AddEdges(edges []models.GraphEdge) {
	s.edges = append(s.edges, edges...)
}

// GetAllNodes returns all nodes as a slice.
func (s *Store) GetAllNodes() []models.GraphNode {
	nodes := make([]models.GraphNode, 0, len(s.nodes))
	for _, n := range s.nodes {
		nodes = append(nodes, n)
	}
	return nodes
}

// GetAllEdges returns all edges.
func (s *Store) GetAllEdges() []models.GraphEdge {
	return s.edges
}

// Snapshot returns the full graph state.
func (s *Store) Snapshot() ([]models.GraphNode, []models.GraphEdge) {
	return s.GetAllNodes(), s.GetAllEdges()
}

// Merge applies a graph update (new nodes + edges).
func (s *Store) Merge(update models.GraphUpdate) {
	s.AddNodes(update.NewNodes)
	s.AddEdges(update.NewEdges)
}

// NodeCount returns the number of unique nodes.
func (s *Store) NodeCount() int {
	return len(s.nodes)
}

// EdgeCount returns the number of edges.
func (s *Store) EdgeCount() int {
	return len(s.edges)
}
