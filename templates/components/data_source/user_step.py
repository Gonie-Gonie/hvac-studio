from .helpers import context_value


def step(inputs, state, params, context):
    value = context_value(context, "source_value", params.get("default_value", 0.0))
    return {"value": float(value)}, state
