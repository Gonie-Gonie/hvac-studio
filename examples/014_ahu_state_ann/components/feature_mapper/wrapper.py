import json
from . import user_init, user_step


class FeatureMapperComponent:
    """Studio-owned runtime wrapper.

    Component contract metadata is regenerated from graph.json/component.json.
    Edit user_step.py for model logic.

    Inputs: outdoor_temperature_c, return_air_temperature_c, chw_setpoint_c, fan_speed_fraction
    Outputs: features
    Parameters: none
    State: none
    """

    input_nodes = json.loads("{\"chw_setpoint_c\":{\"id\":\"chw_setpoint_c\",\"name\":\"Chilled Water Setpoint\",\"preset\":\"control_signal_input\",\"direction\":\"signal\",\"medium\":\"signal\",\"value_type\":\"float\",\"unit\":\"degC\",\"required\":true,\"default\":null},\"fan_speed_fraction\":{\"id\":\"fan_speed_fraction\",\"name\":\"Fan Speed Fraction\",\"preset\":\"control_signal_input\",\"direction\":\"signal\",\"medium\":\"signal\",\"value_type\":\"float\",\"unit\":\"fraction\",\"required\":true,\"default\":null},\"outdoor_temperature_c\":{\"id\":\"outdoor_temperature_c\",\"name\":\"Outdoor Temperature\",\"preset\":\"scalar_input\",\"direction\":\"inlet\",\"medium\":\"signal\",\"value_type\":\"float\",\"unit\":\"degC\",\"required\":true,\"default\":null},\"return_air_temperature_c\":{\"id\":\"return_air_temperature_c\",\"name\":\"Return Air Temperature\",\"preset\":\"air_inlet\",\"direction\":\"inlet\",\"medium\":\"air\",\"value_type\":\"float\",\"unit\":\"degC\",\"required\":true,\"default\":null}}")
    output_nodes = json.loads("{\"features\":{\"id\":\"features\",\"name\":\"Features\",\"preset\":\"scalar_output\",\"direction\":\"outlet\",\"medium\":\"signal\",\"value_type\":\"object\",\"unit\":\"\",\"required\":null,\"default\":null}}")
    parameter_schema = json.loads("{}")
    state_schema = json.loads("{}")

    def initialize(self, params, context):
        state = user_init.initialize(params, context)
        if state is None:
            return {}
        return state

    def evaluate(self, inputs, state, params, context):
        return user_step.step(inputs, state, params, context)
