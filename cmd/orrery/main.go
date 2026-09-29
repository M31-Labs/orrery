// Command orrery is a gosx web application that renders a repository's
// code graph as a 3D scene in the browser via WebGPU/WebGL.
//
// Usage:
//
//	go run ./cmd/orrery [--port=PORT]
//
// Then open http://localhost:PORT/?root=/path/to/repo in a browser that
// supports WebGPU or WebGL.  The WebGPU visual must be confirmed in a
// browser; this server only generates the markup and graph data.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"

	"m31labs.dev/gosx"
	"m31labs.dev/gosx/scene"
	"m31labs.dev/gosx/server"
	"m31labs.dev/orrery/internal/graph"
	"m31labs.dev/orrery/internal/layout"
)

// defaultRoot is the codebase graphed when a request has no root parameter:
// $ORRERY_ROOT when set, otherwise the current directory.
var defaultRoot = func() string {
	if root := os.Getenv("ORRERY_ROOT"); root != "" {
		return root
	}
	return "."
}()

const (
	// maxEdges caps the number of line segments sent to the browser so the
	// payload stays manageable for large graphs.
	maxEdges = 1000
	// maxLabels caps the number of text labels rendered in-scene.
	maxLabels = 50
)

func main() {
	port := flag.String("port", "9010", "HTTP port to listen on")
	flag.Parse()

	_, thisFile, _, _ := runtime.Caller(0)
	// Resolve the gosx runtime root next to the workspace root.
	// The m31labs.dev server uses the same "sibling gosx checkout" pattern.
	runtimeRoot := server.ResolveAppRoot(thisFile)

	app := server.New()

	// Scene3D feature bundles (bootstrap-feature-scene3d*.js) serve from a gosx
	// runtime root — a `gosx build` output (build.json + assets/). Without one,
	// gosx serves only a stub runtime and the 3D scene never loads. Prefer
	// GOSX_RUNTIME_ROOT; until Orrery has its own `gosx build`, point it at any
	// v0.27.x built runtime (e.g. m31labs.dev/dist) to render.
	if rr := strings.TrimSpace(os.Getenv("GOSX_RUNTIME_ROOT")); rr != "" {
		runtimeRoot = rr
	}
	if runtimeRoot != "" {
		app.SetRuntimeRoot(runtimeRoot)
		log.Printf("orrery: runtime root = %s", runtimeRoot)
	} else {
		log.Printf("orrery: WARNING no runtime root — Scene3D feature bundles will 404 and the 3D scene will not render")
	}

	app.SetLayout(func(title string, body gosx.Node) gosx.Node {
		return server.HTMLDocument("Orrery — "+title, gosx.Node{}, body)
	})

	app.Page("GET /", handleGraph)

	// JSON API to fetch raw graph data — useful for debugging.
	app.API("GET /api/graph", func(ctx *server.Context) (any, error) {
		root := ctx.Request.URL.Query().Get("root")
		if root == "" {
			root = defaultRoot
		}
		g, err := graph.Build(root)
		if err != nil {
			return nil, fmt.Errorf("build graph: %w", err)
		}
		return g, nil
	})

	addr := ":" + *port
	log.Printf("orrery listening at http://127.0.0.1%s", addr)
	log.Fatal(app.ListenAndServe(addr))
}

// handleGraph builds the code graph and renders the 3D scene page.
func handleGraph(ctx *server.Context) gosx.Node {
	r := ctx.Request
	root := r.URL.Query().Get("root")
	if root == "" {
		root = defaultRoot
	}

	g, err := graph.Build(root)
	if err != nil {
		return errorPage(root, err)
	}

	return renderScenePage(ctx, g, root)
}

// renderScenePage emits the Scene3D mount plus a stats HUD and the embedded
// graph data (counts).
func renderScenePage(ctx *server.Context, g *graph.Graph, root string) gosx.Node {
	positions := layout.ComputePositions(g)
	nodeCount := len(g.Nodes)
	edgeCount := len(g.Edges)

	// Build node instanced mesh (spheres, one per node).
	nodeColors := nodeColorSlice(g)
	nodeScales := uniformScale(nodeCount, 2.0) // 2-unit radius spheres
	nodeMesh := scene.InstancedMesh{
		ID:        "orrery-nodes",
		Count:     nodeCount,
		Geometry:  scene.SphereGeometry{Radius: 1, Segments: 8},
		Material:  scene.FlatMaterial{Color: "#44aaff", Opacity: scene.Float(0.85)},
		Colors:    nodeColors,
		Positions: positions,
		Scales:    nodeScales,
	}

	// Build edge line geometry.
	linePoints, lineSegs := layout.EdgeLines(g, positions, maxEdges)
	edgeMeshNode := gosx.Node{} // empty if no edges
	if len(lineSegs) > 0 {
		edgeMesh := scene.Mesh{
			ID: "orrery-edges",
			Geometry: scene.LinesGeometry{
				Points:   linePoints,
				Segments: lineSegs,
				Width:    0.5,
			},
			Material: scene.FlatMaterial{Color: "#334455", Opacity: scene.Float(0.5)},
		}
		edgeMeshNode = sceneEngineFromMesh(ctx, edgeMesh)
	}

	// Stats HUD.
	statsJSON, _ := json.MarshalIndent(g.Stats, "", "  ")
	hud := gosx.El("pre",
		gosx.Attrs(
			gosx.Attr("id", "orrery-hud"),
			gosx.Attr("style", hudStyle),
		),
		gosx.Text(fmt.Sprintf(
			"root: %s\nfiles: %d  nodes: %d  edges: %d  unresolved: %d\nstats: %s",
			root,
			g.Stats.Files, nodeCount, edgeCount, g.Stats.Unresolved,
			string(statsJSON),
		)),
	)

	// Embedded counts for test verification.
	embedComment := gosx.RawHTML(fmt.Sprintf(
		`<!-- orrery-graph-data: {"nodes":%d,"edges":%d,"files":%d} -->`,
		nodeCount, edgeCount, g.Stats.Files,
	))

	// Camera: pull back far enough to see the full sphere.
	sceneProps := scene.Props{
		Background: "#050a14",
		Controls:   scene.ControlOrbit,
		Camera: scene.PerspectiveCamera{
			Position: scene.Vector3{X: 0, Y: 0, Z: 280},
			FOV:      55,
			Near:     1,
			Far:      2000,
		},
	}

	// Build the node instanced mesh via engine config.
	nodeEngineNode := renderInstancedMeshEngine(ctx, nodeMesh)

	return gosx.El("div",
		gosx.Attrs(gosx.Attr("style", "position:relative;width:100%;height:100vh;background:#050a14;")),
		// Scene3D mount for the full scene (camera + background).
		sceneMount(ctx, sceneProps),
		// Overlay: node instanced mesh engine.
		nodeEngineNode,
		// Overlay: edge lines.
		edgeMeshNode,
		// HUD overlay.
		hud,
		// Embedded counts comment.
		embedComment,
	)
}

// sceneMount renders the Scene3D container that carries camera and background.
func sceneMount(ctx *server.Context, props scene.Props) gosx.Node {
	cfg := props.EngineConfig()
	fallback := gosx.El("div",
		gosx.Attrs(gosx.Attr("style", "color:#8af;font-family:monospace;padding:2rem;")),
		gosx.Text("WebGPU/WebGL scene loading…"),
	)
	return gosx.El("div",
		gosx.Attrs(
			gosx.Attr("id", "orrery-scene"),
			gosx.Attr("style", "position:absolute;inset:0;"),
		),
		ctx.Engine(cfg, fallback),
	)
}

// renderInstancedMeshEngine builds a separate Scene3D engine config for the
// node spheres and returns its mount node.
func renderInstancedMeshEngine(ctx *server.Context, im scene.InstancedMesh) gosx.Node {
	// Embed the instanced mesh as the scene prop.
	p := scene.Props{}
	p.Graph = scene.Graph{Nodes: []scene.Node{im}}

	cfg := p.EngineConfig()
	cfg.Name = "orrery-nodes-engine"
	fallback := gosx.Node{}
	return ctx.Engine(cfg, fallback)
}

// sceneEngineFromMesh builds a Scene3D engine for a single Mesh node (edges).
func sceneEngineFromMesh(ctx *server.Context, m scene.Mesh) gosx.Node {
	p := scene.Props{}
	p.Graph = scene.Graph{Nodes: []scene.Node{m}}
	cfg := p.EngineConfig()
	cfg.Name = "orrery-edges-engine"
	return ctx.Engine(cfg, gosx.Node{})
}

// nodeColorSlice assigns a color per node based on its Kind.
func nodeColorSlice(g *graph.Graph) []string {
	colors := make([]string, len(g.Nodes))
	for i, n := range g.Nodes {
		switch n.Kind {
		case "function", "method":
			colors[i] = "#44aaff"
		case "type", "struct", "interface":
			colors[i] = "#ff9944"
		case "variable", "const":
			colors[i] = "#66cc88"
		default:
			colors[i] = "#aaaacc"
		}
	}
	return colors
}

// uniformScale returns n scale vectors all set to s.
func uniformScale(n int, s float64) []scene.Vector3 {
	scales := make([]scene.Vector3, n)
	for i := range scales {
		scales[i] = scene.Vector3{X: s, Y: s, Z: s}
	}
	return scales
}

const hudStyle = `
position:absolute;
bottom:16px;left:16px;
margin:0;padding:10px 14px;
background:rgba(5,10,20,0.88);
color:#8dccff;
font:11px/1.6 ui-monospace,Menlo,monospace;
border:1px solid rgba(68,170,255,0.22);
border-radius:6px;
pointer-events:none;
white-space:pre;
z-index:99;
max-width:420px;
`

func errorPage(root string, err error) gosx.Node {
	return gosx.El("div",
		gosx.Attrs(gosx.Attr("style", "padding:2rem;font-family:monospace;color:#ff6666;background:#050a14;min-height:100vh;")),
		gosx.El("h2", gosx.Text("Orrery — graph build failed")),
		gosx.El("p", gosx.Text("root: "+root)),
		gosx.El("pre", gosx.Text(err.Error())),
	)
}
