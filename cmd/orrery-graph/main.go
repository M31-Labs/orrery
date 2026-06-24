// Command orrery-graph builds a code graph for a repository and prints it as JSON.
//
// Usage:
//
//	orrery-graph <root>          — print the full graph as indented JSON
//	orrery-graph -stats <root>   — print only the stats line
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"m31labs.dev/orrery/internal/graph"
)

func main() {
	statsOnly := flag.Bool("stats", false, "print only stats (files/nodes/edges/unresolved)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: orrery-graph [-stats] <root>\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	args := flag.Args()
	if len(args) != 1 {
		flag.Usage()
		os.Exit(1)
	}
	root := args[0]

	g, err := graph.Build(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "orrery-graph: build error: %v\n", err)
		os.Exit(1)
	}

	if *statsOnly {
		fmt.Printf("files=%d nodes=%d edges=%d unresolved=%d\n",
			g.Stats.Files, g.Stats.Nodes, g.Stats.Edges, g.Stats.Unresolved)
		return
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(g); err != nil {
		fmt.Fprintf(os.Stderr, "orrery-graph: json encode error: %v\n", err)
		os.Exit(1)
	}
}
