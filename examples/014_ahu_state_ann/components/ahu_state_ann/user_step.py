from .helpers import evaluate_model


def step(inputs, state, params, context):
    features = inputs["features"]
    bias = float(params.get("output_bias", 0.0))
    return evaluate_model(state, features, bias), state
