# Architecture

Studio, the Python SDK, and external engines use one runner and the same
persisted project artifacts:

```text
Studio / CLI / Python SDK / external engine
  -> bcs-runner
  -> Go project loader, compiler, and runtime
  -> Python worker / external executable / composite child system
  -> user-defined component logic
```

## Source of Truth

`project.bcsproj`, `graph.json`, component source and metadata, JSON schemas,
datasets, dependency locks, parameter sets, scenarios, and saved records define
the model. Studio edits and inspects those files. The SDK wraps runner commands
and persistent serve sessions.

The runtime loads the project and graph, validates contracts and references,
rejects graph cycles, compiles a topological execution plan, initializes
components, evaluates them in order, and returns structured outputs, states,
timings, and logs. Python component calls use a persistent JSONL worker process.

## Contract Decisions

- A Component is a calculation unit; its Nodes are input/output endpoints.
- Public IO maps explicit system IDs to component/node endpoints. Name guessing
  must not determine connectivity or execution.
- The outer graph is acyclic. Feedback iteration belongs inside a component
  declaring solver metadata, iteration method, and stopping criteria.
- Composite components expose node IDs matching a child system's public IO and
  carry the child's state through the wrapper state.
- Connection conversions are explicit `unit_conversion` metadata. Labels alone
  do not transform values.
- Calibration and optimization save new result artifacts. Runtime parameter
  overlays preserve the baseline graph; applying them to the graph is explicit.
- Runtime exports must run after being moved away from a source checkout when
  their selected profile includes the required support files.

These rules consolidate the repository's original architecture decisions.
Runnable examples and golden outputs are regression assets for those contracts.

## Code Boundaries

| Path | Responsibility |
| --- | --- |
| `go/cmd/` | Runner, environment checker, and Studio entrypoints |
| `go/internal/project` | Project and graph loading |
| `go/internal/graph` | Graph indexing helpers |
| `go/internal/compiler` | Contract validation and execution planning |
| `go/internal/pythonworker` | JSONL worker client |
| `go/internal/runtime` | Evaluation orchestration |
| `go/internal/studio` | Desktop web workspace, local API, persisted editing |
| `go/internal/platform` | OS-specific paths, processes, and runtime discovery |
| `python/bcs_worker` | Python component host |
| `python/bcs_sdk` | Runner client and research workflow helpers |
| `schema/` | Persisted artifact and protocol schemas |
| `scripts/release/runtime-manifest.json` | Package runtime requirements |

Keep Studio's `static/js/app.js` focused on orchestration and place workspace
behavior, shared helpers, and result rendering in focused modules. Common UI
flows should use structured controls and target errors to a component, node,
source line, or artifact; raw JSON remains available for diagnostics.

Windows is the primary distribution platform. Isolate executable naming,
process, packaging, installer, and signing behavior from portable runtime and
schema code. See [development](development.md) and [release](release.md).