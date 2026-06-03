from .helpers import unsupported_external_mode


def step(inputs, state, params, context):
    unsupported_external_mode(params.get("command", ""))
    return {"response": inputs["request"]}, state
