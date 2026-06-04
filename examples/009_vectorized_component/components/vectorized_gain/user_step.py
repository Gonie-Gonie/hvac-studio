from .helpers import as_sequence


def step(inputs, state, params, context):
    values = as_sequence(inputs["values"])
    gain = float(params.get("gain", 1.0))
    return {"results": [float(value) * gain for value in values]}, state
