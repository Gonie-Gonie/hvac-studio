from .helpers import apply_gain


def step(inputs, state, params, context):
    value = float(inputs["value"])
    gain = float(params.get("gain", 2.0))
    return {"result": apply_gain(value, gain)}, state
