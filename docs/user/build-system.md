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

The canvas shows node medium badges and labels each connection with its endpoint flow, medium, and latest value when a run result is available. Connection styling calls out medium warnings, explicit medium overrides, incompatible medium mismatches, long paths, and backtracking paths so large systems can be scanned without opening raw JSON.

Workspace component positions can be adjusted on the canvas. Studio saves those positions in `studio/layout.json`; the layout file affects the authoring view only and does not change runtime execution. Auto Layout rebuilds the saved layout from the current system connections so connected components are arranged left-to-right. The canvas surface expands to cover saved, auto-laid-out, and dragged node positions so cards and connection lines remain scrollable.

## Public IO Mapping

Public input/output mapping makes a system callable from outside:

```text
public input -> component input node
component output node -> public output
```

Public IO is the stable surface for GUI, CLI, SDK, datasets, optimization, and delivery.
