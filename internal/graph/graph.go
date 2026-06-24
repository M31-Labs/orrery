// Package graph builds a serializable code graph from a repository path using
// canopy's index and xref packages.
package graph

import (
	"m31labs.dev/canopy/pkg/index"
	"m31labs.dev/canopy/pkg/xref"
)

// Node is a serializable code definition (function, method, etc.).
type Node struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	File     string `json:"file"`
	Package  string `json:"package"`
	Callable bool   `json:"callable"`
	Line     int    `json:"line"`
}

// Edge is a serializable call relationship between two nodes.
type Edge struct {
	From       string `json:"from"`
	To         string `json:"to"`
	Kind       string `json:"kind"`
	Count      int    `json:"count"`
	Resolution string `json:"resolution"`
}

// Stats holds summary counts for the graph.
type Stats struct {
	Files      int `json:"files"`
	Nodes      int `json:"nodes"`
	Edges      int `json:"edges"`
	Unresolved int `json:"unresolved"`
}

// Graph is the top-level serializable code graph for a repository.
type Graph struct {
	Root  string `json:"root"`
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
	Stats Stats  `json:"stats"`
}

// Build indexes the repository at root and returns a Graph.
// It uses NewBuilderWithWorkspaceIgnores so all registered language parsers
// (Go, TypeScript, Python, etc.) are lazily loaded and available.
func Build(root string) (*Graph, error) {
	builder, err := index.NewBuilderWithWorkspaceIgnores(root)
	if err != nil {
		return nil, err
	}

	idx, err := builder.BuildPath(root)
	if err != nil {
		return nil, err
	}

	g, err := xref.Build(idx)
	if err != nil {
		return nil, err
	}

	nodes := make([]Node, 0, len(g.Definitions))
	for _, d := range g.Definitions {
		nodes = append(nodes, Node{
			ID:       d.ID,
			Name:     d.Name,
			Kind:     d.Kind,
			File:     d.File,
			Package:  d.Package,
			Callable: d.Callable,
			Line:     d.StartLine,
		})
	}

	matEdges := g.MaterializeEdges(g.Edges)
	edges := make([]Edge, 0, len(matEdges))
	for _, me := range matEdges {
		edges = append(edges, Edge{
			From:       me.Caller.ID,
			To:         me.Callee.ID,
			Kind:       "call",
			Count:      me.Count,
			Resolution: me.Resolution,
		})
	}

	stats := Stats{
		Files:      idx.FileCount(),
		Nodes:      len(nodes),
		Edges:      len(edges),
		Unresolved: len(g.Unresolved),
	}

	return &Graph{
		Root:  root,
		Nodes: nodes,
		Edges: edges,
		Stats: stats,
	}, nil
}
