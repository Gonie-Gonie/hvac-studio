# Agent Working Notes

## Product and Source of Truth

HVAC Studio is a Python-first Component-Node System authoring/runtime tool.
`project.bcsproj`, `graph.json`, components, schemas, datasets, parameter sets,
scenarios, and saved records are source artifacts. Studio edits and inspects
them; CLI, SDK, and Studio use the same runner.

Use Component for a calculation unit and Node for its input/output endpoint.
Keep public IO explicit, outer graphs acyclic, and solver iteration inside an
explicit component boundary. Preserve baseline parameters until an explicit
apply action. Common GUI workflows use structured controls; Problems should
link to the affected component, node, source line, or artifact.

Generated-wrapper source editing must preserve `user_step.py` and helpers.
Keep `go/internal/studio/static/js/app.js` focused on orchestration and extract
focused UI modules as needed. Examples and their golden outputs are regression
assets, including stateful, vectorized, external, solver, composite, and ML cases.

## Repository and Documentation

- `go/`: runner, compiler, runtime, Studio, and OS boundaries.
- `python/`: worker and SDK.
- `scripts/release/runtime-manifest.json`: packaging runtime requirements.
- [Development](docs/development.md): setup, checks, generated-file policy.
- [Architecture](docs/architecture.md): enduring design decisions.
- [Release](docs/release.md): package scope, gates, provenance, and publishing.
- [User guide](docs/index.md): modeling, workflows, CLI, SDK, and exports.

Keep documentation consolidated. Update the relevant guide instead of adding
status snapshots, roadmap archives, or duplicated tutorial pages. When pages
change, update MkDocs navigation, manual sources, and Studio help links together.

## Verification

Read existing code before refactoring and preserve user changes. Run the
narrowest meaningful checks, broadening for shared runtime/contracts/packaging.

```powershell
.\scripts\dev\test-fast.ps1
.\scripts\dev\test-docs.ps1
.\scripts\dev\test-product-wording.ps1
.\scripts\release\test-release-candidate.ps1 -Version 0.1.0-dev -SkipSetup
```

Studio UI changes also need the relevant targets and screenshot matrix.

## Generated Output

`scripts/build.ps1` builds and smoke-tests the portable application and replaces
`dist/latest/`. Its entrypoint is `dist/latest/HVAC Studio.exe`; retain its
support directories. Build/package/test intermediates belong in `.tmp/`,
regenerable caches in `.cache/`, installed tools in `.toolchain/`, and the
Python environment in `.venv/`. These paths are Git-ignored.

`scripts/dev/clean-generated.ps1` removes intermediates and versioned outputs in
`dist/releases/`, preserving the latest application and development tools.
Use `-Inventory` to inspect targets and `-Caches` to clear caches too.
Do not remove source fixtures, locks, model assets, or user project artifacts.

## Working Rules

Keep changes within the requested scope and work carefully in a dirty tree.
Keep the latest build current when requested changes affect it. Commit and push
only when the user asks for those actions.