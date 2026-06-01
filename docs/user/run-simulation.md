# Run Simulation

## Validate

Validation checks graph structure, public IO mappings, connection references, and execution constraints before running.

## Run One Case

Studio runs the current public inputs and context through the same runtime path as `bcs-runner`.

Workspace runs are saved as:

```text
runs/run-*.json
```

## Inspect Results

Run results include:

- public outputs
- component inputs
- component outputs
- states
- context
- execution order

Selecting a component after a run shows its latest inputs and outputs in the Inspector.

## Scenarios

Current run inputs can be saved as scenario artifacts under `scenarios/`.

Saved scenarios can be reopened from the Project tree to populate the Run Inputs panel for repeatable one-case runs.
