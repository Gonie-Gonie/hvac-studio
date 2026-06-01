# Build System

A system is the runnable composition of components, connections, public inputs, and public outputs.

## Add Components To A System

Creating a component only creates the component artifact. Adding it to the entry system is explicit. When Studio includes a component in the system, it currently creates public IO mappings and extends the default input file for new required public inputs.

## Connections

Connections link output nodes to input nodes. They are stored in `graph.json`, validated by the compiler, and used to compute execution order.

Current Studio builds connections from the Inspector. Select a target component that is already in the system, choose a source output endpoint, choose one of the selected component's input nodes, and connect them. If that target input was previously exposed as a public input, Studio removes the public input mapping because the value now comes from the upstream component.

## Public IO Mapping

Public input/output mapping makes a system callable from outside:

```text
public input -> component input node
component output node -> public output
```

Public IO is the stable surface for GUI, CLI, SDK, datasets, optimization, and delivery.
