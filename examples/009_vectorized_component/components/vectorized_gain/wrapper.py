import json
from . import user_init, user_step


class VectorizedGain:
    """Studio-owned runtime wrapper.

    Component contract metadata is regenerated from graph.json/component.json.
    Edit user_step.py for model logic.

    Inputs: values
    Outputs: results
    Parameters: gain
    State: none
    """

    input_nodes = json.loads("{\"values\":{\"id\":\"values\",\"name\":\"Values\",\"preset\":\"time_series_input\",\"direction\":\"inlet\",\"medium\":\"signal\",\"value_type\":\"array\",\"unit\":\"\",\"required\":true,\"default\":null}}")
    output_nodes = json.loads("{\"results\":{\"id\":\"results\",\"name\":\"Results\",\"preset\":\"scalar_output\",\"direction\":\"outlet\",\"medium\":\"signal\",\"value_type\":\"array\",\"unit\":\"\",\"required\":null,\"default\":null}}")
    parameter_schema = json.loads("{\"gain\":{\"display_name\":\"Gain\",\"unit\":\"ratio\",\"default\":2,\"current\":2,\"bounds\":{\"min\":0,\"max\":100},\"role\":\"calibration_target\",\"group\":\"Vectorized\",\"description\":\"Multiplier applied to each value.\"}}")
    state_schema = json.loads("{}")

    def initialize(self, params, context):
        state = user_init.initialize(params, context)
        if state is None:
            return {}
        return state

    def evaluate(self, inputs, state, params, context):
        return user_step.step(inputs, state, params, context)
