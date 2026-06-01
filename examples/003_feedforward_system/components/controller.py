class SetpointController:
    input_nodes = {
        "cooling_load_kw": {
            "medium": "signal",
            "value_type": "float",
            "unit": "kW",
            "required": True,
        },
        "base_chw_setpoint_c": {
            "medium": "signal",
            "value_type": "float",
            "unit": "degC",
            "required": True,
        },
    }

    output_nodes = {
        "chw_setpoint_c": {
            "medium": "signal",
            "value_type": "float",
            "unit": "degC",
        }
    }

    parameter_schema = {
        "reference_load_kw": {
            "type": "float",
            "unit": "kW",
            "required": True,
        },
        "reset_per_100kw": {
            "type": "float",
            "unit": "K/100kW",
            "required": True,
        },
    }

    def initialize(self, params, context):
        return {}

    def evaluate(self, inputs, state, params, context):
        load_delta = inputs["cooling_load_kw"] - params["reference_load_kw"]
        reset = (load_delta / 100.0) * params["reset_per_100kw"]
        return {
            "chw_setpoint_c": inputs["base_chw_setpoint_c"] - reset,
        }, state

