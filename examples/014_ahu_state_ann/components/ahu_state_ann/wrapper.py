import json
from . import user_init, user_step


class AHUStateANNComponent:
    """Studio-owned runtime wrapper.

    Component contract metadata is regenerated from graph.json/component.json.
    Edit user_step.py for model logic.

    Inputs: features
    Outputs: supply_air_temperature_c, cooling_power_kw
    Parameters: output_bias
    State: model
    """

    input_nodes = json.loads("{\"features\":{\"id\":\"features\",\"name\":\"Features\",\"preset\":\"scalar_input\",\"direction\":\"inlet\",\"medium\":\"signal\",\"value_type\":\"object\",\"unit\":\"\",\"required\":true,\"default\":null}}")
    output_nodes = json.loads("{\"cooling_power_kw\":{\"id\":\"cooling_power_kw\",\"name\":\"Cooling Power\",\"preset\":\"electric_power_output\",\"direction\":\"outlet\",\"medium\":\"electricity\",\"value_type\":\"float\",\"unit\":\"kW\",\"required\":null,\"default\":null},\"supply_air_temperature_c\":{\"id\":\"supply_air_temperature_c\",\"name\":\"Supply Air Temperature\",\"preset\":\"air_outlet\",\"direction\":\"outlet\",\"medium\":\"air\",\"value_type\":\"float\",\"unit\":\"degC\",\"required\":null,\"default\":null}}")
    parameter_schema = json.loads("{\"output_bias\":{\"display_name\":\"Output Bias\",\"unit\":\"native\",\"default\":0,\"current\":0,\"bounds\":{\"min\":-10,\"max\":10},\"role\":\"calibration_target\",\"group\":\"ML\",\"description\":\"Bias added to each model output after inference.\"}}")
    state_schema = json.loads("{\"model\":{\"display_name\":\"Loaded Model\",\"description\":\"JSON model loaded once during initialize.\"}}")

    def initialize(self, params, context):
        state = user_init.initialize(params, context)
        if state is None:
            return {}
        return state

    def evaluate(self, inputs, state, params, context):
        return user_step.step(inputs, state, params, context)
