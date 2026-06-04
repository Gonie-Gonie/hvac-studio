from .helpers import external_contract


def step(inputs, state, params, context):
    contract = external_contract()
    raise RuntimeError(
        "external_executable components run parameters.command/args, not this Python step. "
        + str(contract)
    )
    return {"response": inputs["request"]}, state
