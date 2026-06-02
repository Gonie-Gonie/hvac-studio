# Run Simulation

## Validate

Validation checks graph structure, public IO mappings, connection references, and execution constraints before running.

## Run One Case

Studio runs the current public inputs and context through the same runtime path as `bcs-runner`.

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
- states
- context
- execution order

After a run, the system canvas shows the latest values on component input and output node endpoints. Selecting a component also shows its latest inputs and outputs in the Inspector.

## Scenarios

Current run inputs can be saved as scenario artifacts under `scenarios/`.

Saved scenarios can be reopened from the Project tree to populate the Run Inputs panel for repeatable one-case runs.

Batch runs execute saved scenarios and write `runs/batch-*.json` records. Batch records can be reopened from the Project tree. Failed cases keep their error and component-linked Problems metadata so the Problems panel can still guide editing after the record is reopened.
