# Run Simulation

## Validate

Validation checks graph structure, public IO mappings, connection references, and execution constraints before running.

## Run One Case

Studio runs the current public inputs and context through the same runtime path as `bcs-runner`.

Components that declare `execution_mode: "vectorized"` run through the same
one-case command path, but the Python worker calls `evaluate_batch` so array
inputs can produce array outputs in one component call.

Components that declare `kind: "external_exe"` and
`execution_mode: "external_executable"` run through the same one-case command
path, but the runner invokes the configured process and exchanges JSON over
stdin/stdout.

The Run Inputs toolbar shows each public input's display name, stable ID when different, value type, unit, and required/optional status. The Default control resets a field to the saved default input value or the graph node default.

The Timeout control sets the maximum wall-clock time for Run and Batch requests. The default is 30 seconds, matching the runner's previous fixed timeout. If a run exceeds the selected timeout, Studio reports a timeout failure instead of saving a partial run record. While a Run or Batch request is active, the command bar enables Cancel; canceling keeps the previous result visible and reports the canceled request in Problems.

Workspace runs are saved as:

```text
runs/run-*.json
```

Before a workspace run, Studio saves model edits that affect runtime behavior, such as component parameters and Python source. Run input fields are sent with the run request and can be saved separately as default inputs or scenarios.

## Inspect Results

Run results include:

- public outputs
- component inputs
- component outputs
- node values
- connection values
- states
- context
- execution order
- per-component timing
- run comparison deltas
- component stdout/stderr logs
- total duration

After a run, the system canvas shows the latest values on component input and output node endpoints. Connection labels also show the latest carried value when available, alongside medium and compatibility markers. Selecting a component also shows its latest inputs and outputs in the Run workspace and the Inspector. The Run workspace includes a run comparison table, execution trace, per-component timing bars, component log table, connection-value table, and node-value table so the data flow can be inspected without reading raw JSON. The comparison table uses the previous runtime result as a baseline when a user runs twice, opens another saved run, or opens a batch after a prior run. Component logs capture stdout and stderr produced during load, initialize, and evaluate calls. If a run or batch case fails, the Run workspace keeps a failure summary and shows linked problem metadata such as component, source file, and line when available. If project inputs, parameters, Python source, or graph connections change after the run, Studio keeps the previous values visible but marks them stale until the next successful run.

## Run Time Series

Native time-series runs use `bcs-runner run-series` or the Studio Series command
in the Run toolbar. The input artifact contains ordered `steps`; each step has
public inputs and per-step context. Top-level context is merged into every step
and is also used when components initialize. The runner keeps one session alive
across the series, so stateful components can carry state across timesteps.

The output includes:

- `outputs`: public output arrays in step order
- `series[]`: step id, time, inputs, context, outputs, states, timings, and logs
- `final_states`: component states after the last step

Use this for sequential stateful studies. Use Data validation when the workflow
is comparing model outputs against measured dataset rows.

In Studio, Series uses the current Run input fields as a seed and evaluates a
short timestep preview. The Run workspace plots numeric public output arrays and
uses the final step as the latest canvas/Inspector value.

## Scenarios

Current run inputs can be saved as scenario artifacts under `scenarios/`. Enter a scenario name in the Run Inputs toolbar and use Scenario; if the name is empty, Studio generates a timestamped scenario name.

Saved scenarios can be reopened from the Project tree to populate the Run Inputs panel for repeatable one-case runs. Opening a scenario returns to the System workspace where those inputs are visible. The active scenario badge can be cleared to return the fields to the project's default run input. Editing an input also clears the active scenario badge because the fields no longer exactly match the saved scenario.

Batch runs execute saved scenarios and write `runs/batch-*.json` records. The Run workspace lists batch cases with status, public output summaries, and errors. Batch records can be reopened from the Project tree. For canvas, Inspector, and Code workspace last-value feedback, Studio uses the first successful batch case. Failed cases keep their error and component-linked Problems metadata so the Problems panel can still guide editing after the record is reopened.

## Serve Mode

`bcs-runner serve` keeps the compiled graph and Python components alive for repeated evaluations. It reads one JSON request per line and writes one JSON response per line. This is the current low-level bridge for future SDK and external-engine integrations.

```json
{"id":"case-1","inputs":{"value":4},"context":{"time":0,"dt":60}}
{"id":"stop","type":"shutdown"}
```

Each successful response includes the same structured result fields used by Studio. Request errors return a JSON error object and do not stop the serve process unless initialization failed before the loop started.
