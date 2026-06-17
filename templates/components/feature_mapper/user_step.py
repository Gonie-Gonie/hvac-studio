from .helpers import ordered_features


def step(inputs, state, params, context):
    feature_config = params.get("feature_config", {})
    return {"features": ordered_features(inputs, feature_config)}, state
