# Edit Python Function Body

Python source is where users define component calculation logic.

## Current Studio Behavior

Studio can load component source into the Python panel. Workspace project source can be saved. Bundled examples are read-only through Studio write APIs.

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

