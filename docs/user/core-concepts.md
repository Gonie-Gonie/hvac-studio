# Core Concepts

## Component

A component is a calculation unit. It can represent equipment, controls, data processing, a surrogate model, or any user-defined model.

A chiller is a component. A chilled-water inlet is a node on that component. Avoid thinking of the equipment itself as a node.

## Node

A node is a component connection point or public interface point. Nodes can represent water, air, signal, electric power, scalar values, or other contract-defined media.

## Connection

A connection links one output node to one input node.

Component graphs are feed-forward at the system level. Feedback behavior should
be represented inside an explicit solver boundary component rather than by
creating a graph cycle.

```text
Controller.control -> Chiller.control
Chiller.power_kw -> OutputAggregator.chiller_power_kw
```

Connections are stored in `graph.json`. They are not just canvas lines.

## System

A system is a runnable collection of components, connections, public inputs, and public outputs.

## Public Input And Public Output

Public inputs and outputs are the external execution contract. GUI runs, CLI runs, SDK calls, dataset mapping, validation, calibration, optimization, and runtime packages should all use the same public interface.

## Parameter

A parameter is a named value used by component logic. Parameters may later belong to parameter sets for design cases, calibration results, and optimization results.

## Scenario

A scenario records inputs and context for a repeatable run. Studio currently saves scenarios under `scenarios/`.

## Runner And Worker

The runner validates and executes a project. The Python worker executes user-defined Python component code behind a stable process boundary.
