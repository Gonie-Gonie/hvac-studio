# Build System

A system is the runnable composition of components, connections, public inputs, and public outputs.

## Add Components To A System

Creating a component only creates the component artifact. Adding it to the entry system is explicit. When Studio includes a component in the system, it currently creates public IO mappings and extends the default input file for new required public inputs.

Components can be removed from the runnable system without deleting their source artifact. Studio removes system membership, connections touching that component, public IO mappings, and default input values that belonged to that system membership.

## Connections

Connections link output nodes to input nodes. They are stored in `graph.json`, validated by the compiler, and used to compute execution order.

Current Studio builds connections from the Inspector. Select a target component that is already in the system, choose a source output endpoint, choose one of the selected component's input nodes, and connect them. If that target input was previously exposed as a public input, Studio removes the public input mapping because the value now comes from the upstream component.

Existing connections can also be removed from the Inspector. Removing a connection restores the target input as a public input and adds it back to the default input file, so the value can be edited again in Run Inputs.

## Public IO Mapping

Public input/output mapping makes a system callable from outside:

```text
public input -> component input node
component output node -> public output
```

Public IO is the stable surface for GUI, CLI, SDK, datasets, optimization, and delivery.
