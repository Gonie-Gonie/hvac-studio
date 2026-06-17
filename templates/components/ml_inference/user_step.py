from .helpers import evaluate_model


def step(inputs, state, params, context):
    bias = float(params.get("output_bias", 0.0))
    return evaluate_model(state, inputs, bias), state
