from .helpers import apply_gain


def step(inputs, state, params, context):
    calls = int(state.get("calls", 0)) + 1
    result = apply_gain(
        float(inputs["value"]),
        float(params.get("gain", 1.0)),
        float(params.get("offset", 0.0)),
    )
    return {"result": result, "call_count": calls}, {"calls": calls}
