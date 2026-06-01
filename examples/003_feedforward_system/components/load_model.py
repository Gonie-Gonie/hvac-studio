class LoadModel:
    input_nodes = {
        "building_load_kw": {
            "medium": "signal",
            "value_type": "float",
            "unit": "kW",
            "required": True,
        }
    }

    output_nodes = {
        "adjusted_load_kw": {
            "medium": "signal",
            "value_type": "float",
            "unit": "kW",
        }
    }

    parameter_schema = {
        "load_factor": {
            "type": "float",
            "required": True,
        }
    }

    def initialize(self, params, context):
        return {}

    def evaluate(self, inputs, state, params, context):
        return {
            "adjusted_load_kw": inputs["building_load_kw"] * params["load_factor"],
        }, state

