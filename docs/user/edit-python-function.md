# Edit Python Function Body

Python source is where users define component calculation logic.

## Current Studio Behavior

Studio has a dedicated Code workspace for direct component Python source editing. It shows the selected component's source file, graph contract, nodes, and parameters together. Workspace project source can be checked, saved, reverted, and edited with snippets. The evaluate snippet reflects the selected component's declared input and output nodes. Saving a source file also returns the source check result so contract problems can appear immediately in the Problems panel. Bundled examples are read-only through Studio write APIs.

Before run, batch, and export actions, Studio flushes dirty workspace source drafts to the project source files. If saved source files have source-check errors, Studio stops the action and shows Problems first. The server enforces the same gate for API calls, reopened projects, and export requests. The source file remains the source of truth; the editor is only an authoring surface.

The source checker validates the expected Python class name, the presence of `evaluate`, basic return-shape hints, graph input/output name references, and Python syntax when a Python runtime is available.

## Component Class Shape

```python
class MyComponent:
    input_nodes = {}
    output_nodes = {}
    parameter_schema = {}
    state_schema = {}

    def initialize(self, params, context):
        return {}

    def evaluate(self, inputs, state, params, context):
        return {}, state
```

## Inputs

`inputs` contains values for the component's input nodes.

```python
value = inputs["value"]
```

## Parameters

`params` contains component parameter values from `graph.json`.

```python
gain = params["gain"]
```

## State

`state` stores values that survive across evaluations. The one-case MVP initializes and returns state, and future time-series execution will make this more important.

## Outputs

`evaluate` must return all required output node values and the next state.

```python
return {"result": value * gain}, state
```
