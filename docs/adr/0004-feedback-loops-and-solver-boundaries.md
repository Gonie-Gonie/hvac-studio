# ADR 0004: Feedback Loops And Solver Boundaries

## Status

Accepted

## Context

HVAC and control models often contain feedback behavior. The runner, however,
executes component graphs in a deterministic order and exposes component/node
values for inspection. Allowing arbitrary graph cycles would make ordering,
state carryover, diagnostics, and GUI inspection ambiguous unless a solver
contract is defined first.

## Decision

The project graph remains acyclic. The compiler rejects algebraic loops between
components.

Feedback behavior is allowed only inside an explicit solver boundary component.
A solver boundary component:

- declares `category: "solver"`
- declares `solver_boundary` metadata
- owns its internal iteration method, stopping criteria, and diagnostics
- presents normal input nodes and output nodes to the outer graph

The runner treats the solver boundary as one normal component in the DAG. It
does not infer hidden feedback edges or iterate arbitrary graph cycles.

## Consequences

Users can model iterative behavior without weakening the source-of-truth graph
contract. The GUI and CLI can keep using one execution order, one set of public
IO mappings, and inspectable component traces.

Future work may add richer solver components, pooled solver state, or specialized
loop diagnostics, but those features should remain behind explicit component or
system boundary contracts.
