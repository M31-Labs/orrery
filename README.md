# Orrery

A live code-graph explorer. Point it at a repository and Orrery renders the
**call graph in 3D** — functions as bodies, calls as the lines between them —
in your browser via WebGPU/WebGL. An orrery is a mechanical model of a system
in motion; this is one for your codebase.

Orrery is a thin visualization layer over two pieces of the stack:

- **[canopy](https://github.com/odvcencio/canopy)** — structural code intelligence
  (symbols, references, resolved call graph)
- **[gosx](https://github.com/odvcencio/gosx)** — Go-native web framework with a
  WebGPU `Scene3D` engine

which both stand on **[gotreesitter](https://github.com/odvcencio/gotreesitter)**,
a pure-Go tree-sitter runtime.

## Status

Early and moving. The graph engine and a browser renderer work today. An
**offline demo bundle** (static, self-contained) and a **single-binary desktop
build** are in progress.

## The graph engine

`internal/graph.Build(root)` turns any repository into a graph via canopy. The
`orrery-graph` CLI exposes it:

```sh
go run ./cmd/orrery-graph -stats /path/to/repo
# files=246 nodes=1458 edges=3012 unresolved=9183   (canopy itself)

go run ./cmd/orrery-graph /path/to/repo   # full graph as JSON
```

## The browser app

```sh
# Scene3D feature bundles serve from a gosx runtime root (a `gosx build`
# output). Until Orrery ships its own build, point GOSX_RUNTIME_ROOT at any
# gosx v0.27.x dist:
GOSX_RUNTIME_ROOT=/path/to/gosx/dist go run ./cmd/orrery --port=9010
# then open  http://localhost:9010/?root=/path/to/any/repo
```

## Development

Local builds use a `go.work` (gitignored) pointing at sibling `gosx`, `canopy`,
and `gotreesitter` checkouts. The published module builds against tagged
releases (`GOWORK=off go build ./...`).
