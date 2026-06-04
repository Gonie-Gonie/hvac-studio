# Concept Map

HVAC Studio has one source-of-truth model and several surfaces that read or write
it.

## Artifact To Surface Map

| Artifact | Studio | CLI | SDK | Runtime export |
| --- | --- | --- | --- | --- |
| `project.bcsproj` | Open, copy, create, export | `--project` entry point | `RunnerClient.start(...)` project path | Copied under `project/` |
| `graph.json` | Canvas, Inspector, Parameters, public IO | Validate, run, schema export | Loaded through runner session | Copied under `project/` |
| `components/` | Code workspace and source checks | Python worker imports | Runner-backed SDK calls | Copied with project files |
| `inputs/` | Run Inputs and scenarios | `run --input` | One-shot helper inputs | Default input scripts |
| `scenarios/` | Save and reopen cases | Batch/run inputs | Scenario helper loading | Copied when present |
| `parameter_sets/` | Parameter Manager and overlays | `--parameter-set` | Parameter-set helpers | Copied when present |
| `datasets/` | Dataset preview and validation mapping | `validate-data` | Validation helpers | Copied when present |
| `validation/mappings/` | Data command setup | `validate-data --mapping` | Validation helpers | Copied when present |
| `calibration/setups/` | Calibration run setup | `calibrate --setup` | Calibration helpers | Copied when present |
| `optimization/setups/` | Optimization run setup | `optimize --setup` | Optimization helpers | Copied when present |
| `runs/`, `batches/`, workflow result folders | Structured artifact browser | Optional saved records | Record loading helpers | Optional record inclusion |
| `exports/` | Export workspace | Not a source project input | Export manifest helper | Output package |

## Flow

1. Studio edits project files.
2. CLI, Studio, SDK, and exports call the same runner/runtime path.
3. The runner validates `project.bcsproj`, `graph.json`, Python source contracts,
   public IO, and workflow artifact references.
4. Result records store provenance and checksums so later runs can be inspected
   or rejected with clear mismatch diagnostics.
5. Runtime export copies the project contract and enough runner support to use
   the model outside Studio.

## Rule Of Thumb

If a workflow matters, keep it as a project artifact rather than only as UI state.
That is what lets Studio, CLI, SDK, smoke tests, and runtime exports agree.
