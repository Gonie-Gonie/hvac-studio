from .helpers import ordered_features


def step(inputs, state, params, context):
    return {"features": ordered_features(inputs)}, state
