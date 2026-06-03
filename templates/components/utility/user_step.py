from .helpers import weighted_sum


def step(inputs, state, params, context):
    a = float(inputs["a"])
    b = float(inputs["b"])
    weight_a = float(params.get("weight_a", 1.0))
    weight_b = float(params.get("weight_b", 1.0))
    return {"result": weighted_sum(a, b, weight_a, weight_b)}, state
