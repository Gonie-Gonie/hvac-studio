# Export Runtime

Runtime export delivers a project to users or external tools that do not need
Studio. In a workspace project, open **Export**, review the profile and selected
artifacts, then export to `exports/runtime_package/`.

## Select Contents

The profile controls datasets and validation mappings, calibration/optimization
setups, ML assets, SDK examples, and generated records. Default selections keep
the available workflows self-contained. Exports always include public IO and
serve schemas, a model-specific CLI guide, and scripts for selected workflows.

When Studio runs from a portable/runtime package, it copies the bundled runner
tools and Python runtime. SDK examples also include `python/bcs_sdk`. A source
checkout export depends on which runtime support files are available; verify
the exported folder before delivery.

`manifest.json` records the exported files, public IO, execution order, commands,
workflow artifacts, selected options, ML asset paths and requirements,
validation-report summaries, and SHA-256 checksums. Selecting a saved export
profile in the Project tree reopens these details.

## Folder Layout

```text
exports/runtime_package/
  bin/                    bcs-runner.exe and bcs-env.exe
  project/                project.bcsproj, graph.json, components and selected artifacts
  runtime/python/         Bundled Python when available
  python/bcs_sdk/          SDK when selected
  schema/                 Public IO and serve protocol schemas
  docs/CLI_Guide.md        Model inputs, outputs, commands, and troubleshooting
  check-env.ps1
  run-default.ps1
  run-scenario.ps1
  run-batch.ps1
  run-series.ps1
  validate-data.ps1
  calibrate.ps1
  optimize.ps1
  serve.ps1
  sdk-example.py
  optimize-sdk.py
  manifest.json
  README.md
```

Workflow scripts and SDK examples appear when their matching artifacts are
selected. The project directory preserves inputs, scenarios, parameter sets,
datasets, study setups, component sources, and selected assets. Selected
generated records are listed separately in the manifest.

## Check and Run

Run these from the exported folder; choose the commands present for the model:

```powershell
.\check-env.ps1 -Json
.\run-default.ps1
.\run-scenario.ps1 -InputFile project\inputs\case01.json
.\run-series.ps1 -InputFile project\inputs\series01.json
.\run-batch.ps1
.\validate-data.ps1
.\calibrate.ps1
.\optimize.ps1
.\serve.ps1 -RequestFile requests.jsonl -Output outputs\serve-responses.jsonl
.\runtime\python\python.exe .\sdk-example.py
.\runtime\python\python.exe .\optimize-sdk.py
```

Scripts write JSON results under `outputs/` and component diagnostic bundles
under `outputs/logs/`. Use `-LogBundle outputs\logs\my-run.json` with the
default/scenario scripts to choose a log path. Move or extract the package to
another folder and repeat the self-check and required workflows before delivery.

`examples/007_runtime_only_package` is a static delivery reference with
`model/project.bcsproj`; Studio exports use the `project/` layout above.

## Artifact Compatibility

The current project/graph schema line is `0.1.x`. Both `project.bcsproj` and
`graph.json` require `schema_version`; compatible patch versions load without
rewrites. Unsupported major/minor versions and unknown project/graph fields
are rejected.

Before upgrading or shipping a project:

```powershell
bcs-runner.exe migrate --project project.bcsproj --output migration-report.json
```

The report lists versions, compatibility, and required actions for each checked
artifact. It succeeds when no migration is needed and returns a validation error
for missing or unsupported versions. Compatible artifacts are not rewritten;
`--write` is reserved for documented migrations.

Patch additions must be optional and preserve existing compatible files.
An incompatible major/minor change needs a documented source/target version,
intentional migration implementation, before/after fixtures, and tests before
users upgrade. Studio, runner, SDK, and exports share this boundary.

| Artifact | Version behavior |
| --- | --- |
| Project and graph | Required `schema_version`, accepts `0.1.x` |
| Component metadata, mappings, parameter sets, study setups | Project-owned JSON; no separate persisted schema version |
| Workflow records | `provenance.schema` uses `hvac-studio.workflow-provenance.v1` |
| Export manifests | Export-owned JSON, verified through package provenance/checksum gates |

See [CLI reference](cli-runner.md) for commands and exit codes, and
[troubleshooting](troubleshooting.md) for structured errors.