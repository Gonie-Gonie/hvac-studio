# Studio GUI

The current Studio surface is a Go-hosted web UI embedded in `tools/go/cmd/studio`.

It is intentionally shaped as the full authoring workspace first:

- top-level project commands
- project explorer
- system canvas
- component inspector
- problems, logs, results, schema, and Python panels
- parameter, dataset, validation, calibration, optimization, and export workspaces

Feature implementation can then progress panel by panel without making the product concept ambiguous.

The GUI must edit and visualize `project.bcsproj`, `graph.json`, component source files, schemas, and runtime outputs. The current local web host uses the same Go compiler/runtime packages as `bcs-runner`; the installed Studio path should keep using the runner/runtime boundary rather than becoming a separate simulation engine.

Distribution direction: Windows-first portable zip, then Windows installer. macOS support should remain structurally possible but is a post-MVP experimental release target because Python runtime packaging, codesign/notarization, file permissions, and external process handling diverge by OS.
