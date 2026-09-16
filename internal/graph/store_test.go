package graph

import (
	"testing"

	"github.com/kampong/debate/internal/models"
)

func TestStore_AddNodes(t *testing.T) {
	s := NewStore()
	s.AddNodes([]models.GraphNode{
		{ID: "n1", Label: "Node 1", Type: "claim"},
		{ID: "n2", Label: "Node 2", Type: "concept"},
	})

	if s.NodeCount() != 2 {
		t.Errorf("expected 2 nodes, got %d", s.NodeCount())
	}
}

func TestStore_AddNodes_Deduplication(t *testing.T) {
	s := NewStore()
	s.AddNodes([]models.GraphNode{
		{ID: "n1", Label: "Node 1", Type: "claim", Refs: 0},
	})
	s.AddNodes([]models.GraphNode{
		{ID: "n1", Label: "Node 1", Type: "claim", Refs: 0},
	})

	if s.NodeCount() != 1 {
		t.Errorf("expected 1 node after dedup, got %d", s.NodeCount())
	}

	nodes := s.GetAllNodes()
	if nodes[0].Refs != 2 {
		t.Errorf("expected refs=2 after two adds, got %d", nodes[0].Refs)
	}
}

func TestStore_AddEdges(t *testing.T) {
	s := NewStore()
	s.AddEdges([]models.GraphEdge{
		{From: "n1", To: "n2", Relation: "supports"},
		{From: "n2", To: "n3", Relation: "contradicts"},
	})

	if s.EdgeCount() != 2 {
		t.Errorf("expected 2 edges, got %d", s.EdgeCount())
	}
}

func TestStore_Merge(t *testing.T) {
	s := NewStore()
	s.Merge(models.GraphUpdate{
		NewNodes: []models.GraphNode{
			{ID: "a", Label: "A", Type: "claim"},
		},
		NewEdges: []models.GraphEdge{
			{From: "a", To: "b", Relation: "cites"},
		},
	})

	if s.NodeCount() != 1 {
		t.Errorf("expected 1 node, got %d", s.NodeCount())
	}
	if s.EdgeCount() != 1 {
		t.Errorf("expected 1 edge, got %d", s.EdgeCount())
	}
}

func TestStore_Snapshot(t *testing.T) {
	s := NewStore()
	s.AddNodes([]models.GraphNode{
		{ID: "x", Label: "X", Type: "evidence"},
	})
	s.AddEdges([]models.GraphEdge{
		{From: "x", To: "y", Relation: "contradicts"},
	})

	nodes, edges := s.Snapshot()
	if len(nodes) != 1 {
		t.Errorf("expected 1 node in snapshot, got %d", len(nodes))
	}
	if len(edges) != 1 {
		t.Errorf("expected 1 edge in snapshot, got %d", len(edges))
	}
}

func TestStore_Empty(t *testing.T) {
	s := NewStore()
	if s.NodeCount() != 0 {
		t.Errorf("expected 0 nodes for new store")
	}
	if s.EdgeCount() != 0 {
		t.Errorf("expected 0 edges for new store")
	}
}
