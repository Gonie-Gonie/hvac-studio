# ADR 0003: Explicit Public IO Mapping

## Status

Accepted

## Context

The design script shows public input/output examples as names, but the runtime needs to know which component node each public system input/output maps to. Guessing by node name would become fragile as systems grow.

## Decision

Represent public system inputs and outputs as endpoint mappings:

```json
{
  "id": "load",
  "component": "controller",
  "node": "load"
}
```

## Consequences

- The runner can validate public IO without name guessing.
- GUI can still present friendly labels.
- Future composite systems have a clear boundary contract.

