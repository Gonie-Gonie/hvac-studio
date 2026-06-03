from .helpers import latest_values


def step(inputs, state, params, context):
    values = latest_values(context)
    values.append(float(inputs["value"]))
    context["sink_values"] = values
    return {}, state
