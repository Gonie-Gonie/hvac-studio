# Modeling

## Core Concepts

| Term | Meaning |
| --- | --- |
| Component | A calculation unit: equipment, controller, data transform, or user model |
| Node | An input/output endpoint on a component, with medium, value type, and unit |
| Connection | An explicit link from an output node to an input node |
| System | Runnable components, connections, and public IO mappings |
| Public Input / Public Output | A system's external contract, shared by Studio, CLI, SDK, and datasets |
| Parameter | A named model value consumed through `params` |
| State | Values carried between evaluations in a runtime session |
| Scenario | Saved public inputs and context for a repeatable run |

A chiller is a Component; its chilled-water inlet is a Node. The outer system
graph is acyclic. Put feedback iteration inside an explicit solver component.

## Project Artifacts

| Artifact | Purpose |
| --- | --- |
| `project.bcsproj` | Metadata, entry system, default input, environment and lockfile |
| `graph.json` | Components, systems, node contracts, connections, and parameters |
| `components/` | Python source and generated-wrapper metadata |
| `inputs/`, `scenarios/` | Default cases, time series, and reusable inputs/context |
| `parameter_sets/` | Named runtime parameter overlays |
| `datasets/`, `validation/` | Measured data, mappings, and saved validation results |
| `calibration/`, `optimization/` | Saved study setups and results |
| `runs/` | Saved run and batch records |
| `exports/` | Export manifests and generated runtime packages |
| `studio/layout.json` | Canvas positions; no effect on runtime execution |

Studio edits these files and calls the same runner used by CLI and SDK. Bundled
examples are read-only through Studio write APIs. Use **Copy** or **New** to
create an editable workspace project under `projects/`.

## Create Components

1. Open a workspace project and choose a template in the Project panel.
2. Add the component and edit its input/output nodes in the Inspector.
3. Define parameter and state metadata.
4. Edit and check the Python source in Code.
5. Explicitly include the component in the entry system.

Templates cover scalar, stateful, generated-wrapper, vectorized, external
executable, solver, composite, and ML-backed components. Creating or duplicating
a component creates an artifact; system membership is a separate action.

Generated-wrapper components use `components/<id>/component.json`, a
Studio-owned `wrapper.py`, and editable `user_step.py` plus helpers. Contract
edits synchronize metadata and the wrapper while preserving user code.

Removing a component from a system removes its related connections, public IO,
and default input entries while preserving its source. Delete its source only
after it is no longer used by a system.

## Edit Python

The Code workspace opens the selected component's editable source alongside
its contract and Problems. Single-file components follow this shape:

```python
class MyComponent:
    input_nodes = {}
    output_nodes = {}
    parameter_schema = {}
    state_schema = {}

    def initialize(self, params, context):
        return {}

    def evaluate(self, inputs, state, params, context):
        value = inputs["value"]
        return {"result": value * params["gain"]}, state
```

Match the graph's class, input, output, parameter, and state declarations.
Return all declared outputs plus the next state. `context` can provide `time`
and `dt`. A one-case run initializes state; serve sessions and native time
series preserve it across evaluations.

For generated wrappers edit `user_step.py`, using the supplied `step`
signature. Vectorized components declare `execution_mode: "vectorized"` and
use `evaluate_batch` (or the generated vectorized step) to process array inputs.

Code supports contract-derived insertions, snippets, completions, source checks,
and linked traceback locations. `Ctrl+S` saves, `Ctrl+Enter` checks,
`Ctrl+Shift+F` formats whitespace, and `Ctrl+Space` opens completions.
Revert restores the saved source.

Studio saves dirty source before Run, Batch, or Export. Syntax, load/import,
signature, and return-shape errors block those actions. Conservative reference
warnings remain visible without blocking execution. Problems and gutter markers
link to source lines; use a Fix action when a suggested edit matches the intent.
Latest component values become stale after model edits until another run.

## Build Systems

Include components in the entry system, then click a source output node and a
target input node on the canvas, or use the Inspector endpoint controls. The
connection persists to `graph.json`. Connecting a previously public input
removes its public mapping; removing the connection restores the mapping and
default input entry.

Public IO maps stable external IDs to component/node endpoints:

```text
public input -> component input node
component output node -> public output
```

Validation rejects invalid endpoints, multiple incoming connections, incompatible
contracts, and direct graph cycles. Select a connection to inspect values and
medium/unit information. Unit conversion is explicit: the Inspector offers
presets or custom `converted = source * factor + offset`. Runtime traces keep
both source and converted values. Unit labels alone do not convert data.

Solver components declare `category: "solver"` and `solver_boundary` metadata,
and own their iterations and stopping criteria. The outer graph still evaluates
them as ordinary acyclic components.

Composite components declare `kind: "composite"` and `composite.system`.
Their input/output node IDs must match the child system's public IO IDs.
Nested child state remains under the wrapper state across session evaluations.

Drag a component header to arrange the canvas. **Auto layout** places components
in flow order without overlapping cards; **Fit** shows the whole system. Use
the zoom buttons or `Ctrl` + scroll for a closer view. Selecting a component
highlights its connections; connection labels appear on hover or selection.
At smaller zoom levels the overview keeps component names readable and hides
port names; zoom in or select a component to inspect its individual nodes.

Workspace positions are saved in `studio/layout.json`. Bundled examples can
also be arranged: their view is remembered locally without changing example
files. Neither kind of layout changes runtime execution.

Open **Run setup** for public input values, the parameter set, and timeout.
The Inspector shows node names and units first; expand a node or a detail group
for its contract, parameters, state, or latest values. **Inspector** toggles the
side panel, and the bottom activity panel expands when results or problems need
attention. Project creation is under **New / Copy**, and less frequent runtime
commands are under **More**.

## Parameters

Use Parameter Manager to add/edit values and the Inspector to edit definitions:
display name, default, unit, role, numeric bounds, group, description, and
visibility. Bounds must satisfy `min <= max`. State definitions also support
initial value, unit, and description.

Roles are `fixed`, `scenario_input`, `calibration_target`,
`optimization_variable`, and `derived`. Roles and bounds help select candidates
in calibration and optimization setup editors.

Parameter sets are JSON overlays under `parameter_sets/`:

```json
{"id":"high_efficiency","components":{"chiller":{"cop":6.8}}}
```

Selecting a set applies it to runtime workflows without rewriting `graph.json`;
select **Baseline** or **Revert Active** to clear it. **Apply Parameter Set** is
the explicit action that writes values to the graph. Results record the selected
set. See [workflows](workflows.md) for calibration and optimization.

## Replace Models

Select a component and replacement template, inspect **Replacement Preview**,
then choose **Replace And Validate**. The preview shows matching/missing node
IDs and parameters. **Copy same-name parameters** transfers compatible values.

Studio rewires the entry system only when every referenced endpoint exists
with the same node ID and direction on the replacement. Incompatible mappings
are rejected. The original component and source remain available; the
replacement receives its own ID and files. Run again to compare public outputs.

For a complete example, copy `examples/015_rc_ahu_ann_composition`, select
**RC Zone Load**, and replace it with **Zone Load ANN Surrogate**.

## ML and ANN Assets

Use **New ML** or the **ML Inference Component** template. It creates a generated
wrapper and model, feature schema, target schema, and validation report files.
The Inspector's **ML Assets** block imports/replaces these files, scalers,
training metadata, required packages, model format, time resolution, and valid
input ranges. **Apply Schema Nodes** synchronizes the feature input and target
outputs, including related public IO for components already in a system.

Asset paths in `ml_metadata` must be project-relative and remain inside the
project root. Load assets once in `initialize` and retain them in state. Keep
feature extraction, preprocessing, inference, and output transformation in
editable helpers.

A Feature Mapper exposes a deterministic `features` object. Its optional
`feature_config` supports per-feature `source`, `scale`, `offset`, `min`,
and `max`; missing sources produce component errors. The Inspector provides
feature previews and a connection suggestion for compatible unconnected
features. See `examples/014_ahu_state_ann`.

Runtime export copies selected assets and records paths, SHA-256 checksums,
and ML validation report summaries. Schema export lists asset requirements.

## External Executables

External components declare:

```json
{
  "kind": "external_exe",
  "execution_mode": "external_executable",
  "parameters": {
    "command": "python",
    "args": ["components/external_gain/external_gain.py"],
    "timeout_ms": 5000
  }
}
```

The process runs from the project root and receives one JSON object on stdin
with `component_id`, `inputs`, `state`, `params`, and `context`. Write one
JSON response on stdout:

```json
{"ok":true,"outputs":{},"state":{},"logs":[{"severity":"info","message":"complete"}]}
```

Return every declared output. Optional state carries forward; structured logs
and stderr join the component log stream. A failed response has `ok: false`
and `error: {"type":"ExternalError","message":"explanation"}`.
The exact request/response contracts are in
`schema/external-component-request.schema.json` and
`schema/external-component-response.schema.json`.
See `examples/010_external_executable_component`.

For integrating an external engine with an entire system, use the
[runner serve protocol](external-engine-protocol.md).
