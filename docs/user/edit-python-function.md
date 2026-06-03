# Edit Python Function Body

Python source is where users define component calculation logic.

## Current Studio Behavior

Studio has a dedicated Code workspace for direct component Python source editing. It shows the selected component's source file, graph contract, nodes, parameters, and source-check issues together. The Code workspace component selector shows display names with stable IDs when they differ. The Project tree's Python Source section can open a component directly in this workspace and shows short source states such as loaded, read only, dirty, ok, or issue counts. Workspace project source can be checked, saved, reverted, and edited with snippets. Reverting source restores the saved file content and clears draft source-check state. The evaluate snippet reflects the selected component's declared input and output nodes. Saving a source file also returns the source check result so contract problems can appear immediately in both the Code workspace and the Problems panel. Bundled examples are read-only through Studio write APIs.

In workspace projects, the Code workspace contract panel can insert selected input, output, and parameter references into the editor. These insert actions use the current `graph.json` contract, so they track node and parameter edits made in the Inspector.

Before run, batch, and export actions, Studio flushes dirty workspace source drafts to the project source files. If saved source files have source-check errors, Studio stops the action and shows Problems first. The server enforces the same gate for API calls, reopened projects, and export requests. The source file remains the source of truth; the editor is only an authoring surface.

After a successful run, the Code workspace contract panel shows the selected component's latest input and output values from the runner result. If the source or model changes afterward, those values are marked stale until another successful run updates them.

The source checker validates the expected Python class name, the presence of `evaluate`, basic return-shape hints, graph input/output name references, Python syntax, and draft source load/import errors when a Python runtime is available.

Line-specific source-check rows can be clicked from the Code workspace or Problems panel to focus the editor line.

The editor supports tab indentation and outdent for selected Python lines. `Ctrl+S` saves the current source, and `Ctrl+Enter` runs the source check. The Code workspace can also save the current workspace source and run the project through the normal execution path.

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
