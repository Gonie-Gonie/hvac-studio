# Artifact Compatibility

HVAC Studio artifacts are JSON files owned by the project. The runner treats them as the source of truth.

## Current Policy

The current artifact schema line is `0.1.x`.

- `project.bcsproj` and `graph.json` must include `schema_version`.
- `0.1.x` project and graph files are load-compatible.
- Other major or minor versions are rejected until a migration path exists.
- Unknown JSON fields are rejected for project and graph files so spelling mistakes do not silently change behavior.

## Migration Notes

There are no automatic migrations yet. During the alpha line, compatible patch-level changes may add optional fields, but they must not require rewriting existing `0.1.x` projects.

When an incompatible schema appears, migration work should produce:

- a documented source and target version
- a command or tool action that rewrites files intentionally
- before and after fixtures
- tests that prove old fixtures either load unchanged or fail with a migration message

## Artifact Status

| Artifact | Current version behavior | Notes |
| --- | --- | --- |
| `project.bcsproj` | Requires `schema_version`; accepts `0.1.x` | Entry point for project compatibility |
| `graph.json` | Requires `schema_version`; accepts `0.1.x` | Main component-node-system contract |
| component metadata | Covered by JSON schema source; no separate persisted schema version yet | Generated-wrapper metadata remains project-owned |
| validation mappings | Project-owned JSON; no separate persisted schema version yet | Saved validation records include provenance checksums |
| parameter sets | Project-owned JSON; no separate persisted schema version yet | Applied as runtime overlays |
| calibration setups | Project-owned JSON; no separate persisted schema version yet | Saved calibration records include provenance checksums |
| optimization setups | Project-owned JSON; no separate persisted schema version yet | Saved optimization records include provenance checksums |
| export manifests | Export-owned JSON; no separate compatibility gate yet | Release/export provenance is checked by package smoke tests |
| workflow records | Include `provenance.schema` | Saved validation, calibration, and optimization records use `hvac-studio.workflow-provenance.v1` |

