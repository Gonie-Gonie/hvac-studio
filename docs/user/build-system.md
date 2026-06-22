# Build System

A system is the runnable composition of components, connections, public inputs, and public outputs.

## Add Components To A System

Creating a component only creates the component artifact. Adding it to the entry system is explicit. When Studio includes a component in the system, it currently creates public IO mappings and extends the default input file for new required public inputs.

Components can be removed from the runnable system without deleting their source artifact. Studio removes system membership, connections touching that component, public IO mappings, and default input values that belonged to that system membership.

## Connections

Connections link output nodes to input nodes. They are stored in `graph.json`, validated by the compiler, and used to compute execution order.

Studio can build connections directly on the canvas. Click a source output node, then click an input node on another component. Studio persists the connection to `graph.json` through the same project API used by the Inspector. If that target input was previously exposed as a public input, Studio removes the public input mapping because the value now comes from the upstream component.

The Inspector also supports the same operation with explicit source and target endpoint controls.

Existing connections can also be selected from the canvas line or the Inspector. After a run, Inspector connection rows show the latest value carried by that connection. Removing a connection from the Inspector restores the target input as a public input and adds it back to the default input file, so the value can be edited again in Run Inputs.

## Feedback Loops

The runner executes the project graph as an acyclic component order. Direct
component-to-component feedback cycles are rejected during validation. Put
iterative feedback behavior inside a solver boundary component instead; the
outer graph should still expose normal inputs, outputs, and feed-forward
connections.

The canvas shows node medium badges and labels each connection with its endpoint flow, medium, and latest value when a run result is available. Connection styling calls out medium warnings, explicit medium overrides, incompatible medium mismatches, long paths, and backtracking paths so large systems can be scanned without opening raw JSON.
Selecting the connected component shows the same medium mismatch, warning, or override badge in the Inspector connection rows.

## Unit Conversion

Connections do not infer unit conversion from labels. If a source output and
target input use different units, select the connection and use the Inspector
Unit Conversion editor. Studio provides common linear presets for W to kW, kW
to W, degC to K, degF to degC, degC to degF, Btu/h to kW, kW to Btu/h,
refrigeration tons to kW, kW to refrigeration tons, kg/s to kg/h, and fraction
to percent. Preset detection accepts common aliases such as `Celsius`, `deg F`,
`BTU/hr`, `kg per second`, `kg/hr`, `pct`, and `%`. Custom conversions use
`converted = source * factor + offset`, with an immediate sample preview before
saving.

Without a conversion, a unit mismatch remains a warning. With an explicit
conversion, the canvas and Inspector mark the connection as converted. Runtime
trace records include both the source value and the converted target value.

## Composite Systems

A composite component wraps another system behind an explicit public IO
boundary. Declare the wrapper component with `kind: "composite"` and
`composite.system`. The wrapper input node IDs must exactly match the child
system `public_inputs[].id`, and wrapper output node IDs must exactly match the
child system `public_outputs[].id`.

At runtime, the outer system still sees one normal DAG component. The runner
evaluates the child system through its public inputs and returns the child
public outputs as wrapper outputs. Nested child states are stored under the
wrapper component state, so repeated session and `run-series` evaluations can
carry child state forward.

Workspace component positions can be adjusted on the canvas. Studio saves those positions in `studio/layout.json`; the layout file affects the authoring view only and does not change runtime execution. Auto Layout rebuilds the saved layout from the current system connections so connected components are arranged left-to-right. The canvas surface expands to cover saved, auto-laid-out, and dragged node positions so cards and connection lines remain scrollable.

## Public IO Mapping

Public input/output mapping makes a system callable from outside:

```text
public input -> component input node
component output node -> public output
```

Public IO is the stable surface for GUI, CLI, SDK, datasets, optimization, and delivery.
