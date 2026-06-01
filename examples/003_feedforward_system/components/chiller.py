class SimpleChiller:
    input_nodes = {
        "cooling_load_kw": {
            "medium": "signal",
            "value_type": "float",
            "unit": "kW",
            "required": True,
        },
        "chw_setpoint_c": {
            "medium": "signal",
            "value_type": "float",
            "unit": "degC",
            "required": True,
        },
    }

    output_nodes = {
        "power_kw": {
            "medium": "electricity",
            "value_type": "float",
            "unit": "kW",
        },
        "chw_supply_temp_c": {
            "medium": "signal",
            "value_type": "float",
            "unit": "degC",
        },
    }

    parameter_schema = {
        "cop": {
            "type": "float",
            "required": True,
        },
        "approach_c": {
            "type": "float",
            "unit": "K",
            "required": True,
        },
    }

    def initialize(self, params, context):
        return {}

    def evaluate(self, inputs, state, params, context):
        power_kw = inputs["cooling_load_kw"] / params["cop"]
        supply_temp_c = inputs["chw_setpoint_c"] + params["approach_c"]
        return {
            "power_kw": power_kw,
            "chw_supply_temp_c": supply_temp_c,
        }, state

