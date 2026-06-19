# Edit Python Function Body

Python source is where users define component calculation logic.

## Current Studio Behavior

Studio has a dedicated Code workspace for direct component Python source editing. It shows the selected component's source file, graph contract, nodes, parameters, and source-check issues together. The Code workspace component selector shows display names with stable IDs when they differ. The Project tree's Python Source section can open a component directly in this workspace and shows short source states such as loaded, read only, dirty, ok, or issue counts. Workspace project source can be checked, saved, reverted, and edited with snippets. Reverting source restores the saved file content and clears draft source-check state. For generated-wrapper components, the editable source is `user_step.py`; `wrapper.py` remains the Studio-owned runtime wrapper and is not overwritten by source saves. The evaluate or step snippet reflects the selected component's declared input and output nodes. Saving a source file also returns the source check result so contract problems can appear immediately in both the Code workspace and the Problems panel. Bundled examples are read-only through Studio write APIs.

In workspace projects, the Code workspace contract panel can insert selected input, output, parameter, state, and context references into the editor. These insert actions use the current `graph.json` contract, so they track node, parameter, and state edits made in the Inspector.

Before run, batch, and export actions, Studio flushes dirty workspace source drafts to the project source files. If saved source files have source-check errors, Studio stops the action and shows Problems first. The server enforces the same gate for API calls, reopened projects, and export requests. The source file remains the source of truth; the editor is only an authoring surface.

After a successful run, the Code workspace contract panel shows the selected component's latest input and output values from the runner result. If the source or model changes afterward, those values are marked stale until another successful run updates them.

The source checker validates the expected Python class name, the presence of `evaluate`, obvious return-shape errors, graph input/output, parameter, and state name references, Python syntax, and draft source load/import errors when a Python runtime is available.

Line-specific source-check rows can be clicked from the Code workspace or Problems panel to focus the editor line. Runtime Python tracebacks are mapped back to component source files when Studio can match the traceback frame to a project component; generated-wrapper failures point at the editable `user_step.py` frame when that is where the user code failed. The editor gutter marks checked or runtime-reported source lines with warning/error dots so line metadata remains visible while editing. Common source-check hints such as missing input references, missing output entries, unknown input/output/parameter/state references with close contract matches, and missing `evaluate` or generated-wrapper `step` scaffolds expose a Fix action that inserts or applies the matching contract edit.

The editor supports lightweight Python syntax highlighting, line numbers, bracket status, Enter auto indentation, and tab indentation/outdent for selected Python lines. Format performs conservative whitespace cleanup: it normalizes line endings, removes trailing spaces, expands leading tabs to four spaces, and keeps one final newline without rewriting Python logic or generated-wrapper boundaries. `Ctrl+S` saves the current source, `Ctrl+Enter` runs the source check, `Ctrl+Shift+F` formats the draft, and `Ctrl+Space` opens contract-derived completions. The Code workspace can also save the current workspace source and run the project through the normal execution path.

The completion panel and contract rows include hover text with contract labels
such as medium, value type, unit, parameter role, current/default values, and
the inserted snippet. They can insert:

- input reads such as `inputs.get("value", 0.0)`
- output dictionary entries such as `"result": value`
- parameter reads such as `params.get("gain", 2.0)`
- state reads from `state_defs`
- context reads for `time` and `dt`
- vectorized `step` or `evaluate_batch` scaffolds
- external executable stdin/stdout adapter scaffolds

For single-file components, source checks also treat keys returned by `initialize()` as initialized state references, so model assets or cached schemas loaded into `state` can be used from `evaluate`.

Source check warnings include contract-reference hints, unknown input/output/parameter/state references, and likely undefined names. Undefined-name warnings do not block save/run by themselves; source-check errors such as missing signatures, wrong return shapes, syntax failures, import/load failures, or missing output contracts block run, batch, and export.

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

`state` stores values that survive across evaluations. One-case runs initialize state for that run. Runner serve mode keeps a live runtime session and preserves component state across repeated JSONL requests.

## Outputs

`evaluate` must return all required output node values and the next state.

```python
return {"result": value * gain}, state
```
