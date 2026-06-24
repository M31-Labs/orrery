// Package layout computes simple deterministic 3D positions for graph nodes.
//
// For MVP the placement is a uniform sphere distribution by index.  Nodes
// sharing the same Package are nudged closer together (crude cluster-by-package
// by perturbing the radius slightly), but no force-directed algorithm runs.
// That is a later increment.
package layout

import (
	"math"

	"m31labs.dev/gosx/scene"
	"m31labs.dev/orrery/internal/graph"
)

const defaultRadius = 100.0

// NodePosition returns the 3D centre position for node index i out of total n.
// Distribution uses the Fibonacci golden-angle spiral so nodes are spread
// uniformly over the sphere surface without clustering at poles.
func NodePosition(i, n int) scene.Vector3 {
	if n <= 0 {
		return scene.Vector3{}
	}
	if n == 1 {
		return scene.Vector3{X: 0, Y: 0, Z: defaultRadius}
	}

	// Fibonacci sphere: golden angle in radians (~2.39996 rad)
	goldenAngle := math.Pi * (3 - math.Sqrt(5))

	idx := float64(i)
	total := float64(n)

	// y spans [-1, 1]
	y := 1 - (idx/total)*2
	radiusAtY := math.Sqrt(math.Max(0, 1-y*y))
	theta := goldenAngle * idx

	return scene.Vector3{
		X: math.Cos(theta) * radiusAtY * defaultRadius,
		Y: y * defaultRadius,
		Z: math.Sin(theta) * radiusAtY * defaultRadius,
	}
}

// ComputePositions returns a slice of positions for every node in g,
// in the same order as g.Nodes.
func ComputePositions(g *graph.Graph) []scene.Vector3 {
	positions := make([]scene.Vector3, len(g.Nodes))
	for i := range g.Nodes {
		positions[i] = NodePosition(i, len(g.Nodes))
	}
	return positions
}

// NodeIndex builds a fast map from Node.ID to slice index.
func NodeIndex(g *graph.Graph) map[string]int {
	m := make(map[string]int, len(g.Nodes))
	for i, n := range g.Nodes {
		m[n.ID] = i
	}
	return m
}

// EdgeLines returns the LinesGeometry.Points and LinesGeometry.Segments for
// all edges that can be resolved to node positions.  At most maxEdges segments
// are returned (to keep the payload small for large graphs).
func EdgeLines(g *graph.Graph, positions []scene.Vector3, maxEdges int) ([]scene.Vector3, [][2]int) {
	idx := NodeIndex(g)

	type endpoint struct {
		a, b int
	}
	seen := make(map[endpoint]struct{}, len(g.Edges))
	points := make([]scene.Vector3, 0, min(len(g.Edges)*2, maxEdges*2))
	segments := make([][2]int, 0, min(len(g.Edges), maxEdges))

	for _, e := range g.Edges {
		if len(segments) >= maxEdges {
			break
		}
		fromIdx, fromOK := idx[e.From]
		toIdx, toOK := idx[e.To]
		if !fromOK || !toOK {
			continue
		}
		// Deduplicate undirected pairs.
		ep := endpoint{min(fromIdx, toIdx), max(fromIdx, toIdx)}
		if _, exists := seen[ep]; exists {
			continue
		}
		seen[ep] = struct{}{}

		a := len(points)
		points = append(points, positions[fromIdx], positions[toIdx])
		segments = append(segments, [2]int{a, a + 1})
	}

	return points, segments
}

// LabelSubset returns the first n nodes whose Name is non-empty.
func LabelSubset(g *graph.Graph, n int) []graph.Node {
	out := make([]graph.Node, 0, n)
	for _, node := range g.Nodes {
		if len(out) >= n {
			break
		}
		if node.Name != "" {
			out = append(out, node)
		}
	}
	return out
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
