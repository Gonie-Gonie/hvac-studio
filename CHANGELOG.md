# Changelog

## Unreleased

- Initialize the monorepo structure for the Component-Node System Studio.
- Add runtime-first working memory in `agent.md`.
- Add initial Go runner, Python worker, schemas, scripts, and first scalar example.
- Add repo-local setup and minimal runtime release packaging.
- Add UX-driven development plan for component-node-system authoring, validation, calibration, optimization, SDK, and delivery.
- Add the first multi-component feed-forward runtime example and golden example test runner.
- Add typed runner error categories for documented CLI exit codes.
- Add CLI validation golden tests for invalid graph diagnostics.
- Add `bcs-runner schema` for exporting system public input/output interface metadata.
- Add component input snapshots to run output for future inspector/debug UX.
- Add the first full Studio workspace shell with project explorer, system canvas, inspector, runtime actions, bottom panels, and future workflow surfaces.
- Document Windows-first portable release strategy and add portable Studio package smoke testing.
- Bundle repo-local Python into Windows portable/runtime packages and test package execution without system Python on `PATH`.
- Add Studio workspace project creation and saved run records.
- Add Studio workspace parameter editing backed by `graph.json` and portable smoke coverage for changed run results.
- Add Studio default input loading/saving so GUI runs can use persisted project input files.
- Add Studio workspace component creation for scalar Python component templates.
- Add explicit Studio system inclusion for workspace components with generated public IO and default inputs.
- Add Studio run record detail loading from saved `runs/run-*.json` artifacts.
- Add Studio runtime export manifest generation under workspace `exports/`.
- Add validation problem metadata and clickable component-linked Problems rows in Studio.
- Add Studio component Python source loading and workspace source saving.
- Add Studio scenario artifact creation from current run inputs.
- Document the connected Studio workflow covered by portable release smoke tests.
- Add the initial User Guide Markdown scaffold under `docs/user/` and document the planned HTML/PDF help release flow.
- Add Studio node-to-node connection authoring for workspace systems.
- Add Studio scenario reopening so saved scenarios can populate run inputs.
- Save workspace parameter/source model edits before Studio run and export actions.
- Add Studio connection removal with public/default input restoration for workspace systems.
- Make release package version resolution tolerate untagged development checkouts.
