import json
from . import user_init, user_step


class CustomGainWrapper:
    """Studio-owned runtime wrapper.

    Component contract metadata is regenerated from graph.json/component.json.
    Edit user_step.py for model logic.

    Inputs: value
    Outputs: result, call_count
    Parameters: gain, offset
    State: calls
    """

    input_nodes = json.loads("{\"value\":{\"id\":\"value\",\"name\":\"Value\",\"preset\":\"scalar_input\",\"direction\":\"inlet\",\"medium\":\"signal\",\"value_type\":\"float\",\"unit\":\"\",\"required\":true,\"default\":null}}")
    output_nodes = json.loads("{\"call_count\":{\"id\":\"call_count\",\"name\":\"Call Count\",\"preset\":\"scalar_output\",\"direction\":\"outlet\",\"medium\":\"signal\",\"value_type\":\"float\",\"unit\":\"count\",\"required\":null,\"default\":null},\"result\":{\"id\":\"result\",\"name\":\"Result\",\"preset\":\"scalar_output\",\"direction\":\"outlet\",\"medium\":\"signal\",\"value_type\":\"float\",\"unit\":\"\",\"required\":null,\"default\":null}}")
    parameter_schema = json.loads("{\"gain\":{\"display_name\":\"Gain\",\"unit\":\"ratio\",\"default\":3,\"current\":3,\"bounds\":{\"min\":0,\"max\":100},\"role\":\"calibration_target\",\"group\":\"Model\",\"description\":\"Multiplier applied to the input value.\"},\"offset\":{\"display_name\":\"Offset\",\"default\":1,\"current\":1,\"bounds\":{\"min\":-100,\"max\":100},\"role\":\"fixed\",\"group\":\"Model\",\"description\":\"Value added after gain is applied.\"}}")
    state_schema = json.loads("{\"calls\":{\"display_name\":\"Calls\",\"unit\":\"count\",\"initial\":0,\"description\":\"Number of completed evaluations.\"}}")

    def initialize(self, params, context):
        state = user_init.initialize(params, context)
        if state is None:
            return {}
        return state

    def evaluate(self, inputs, state, params, context):
        return user_step.step(inputs, state, params, context)
