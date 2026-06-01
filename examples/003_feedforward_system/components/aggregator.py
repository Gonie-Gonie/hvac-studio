class OutputAggregator:
    input_nodes = {
        "chiller_power_kw": {
            "medium": "electricity",
            "value_type": "float",
            "unit": "kW",
            "required": True,
        }
    }

    output_nodes = {
        "total_power_kw": {
            "medium": "electricity",
            "value_type": "float",
            "unit": "kW",
        }
    }

    parameter_schema = {
        "aux_power_kw": {
            "type": "float",
            "unit": "kW",
            "required": True,
        }
    }

    def initialize(self, params, context):
        return {}

    def evaluate(self, inputs, state, params, context):
        return {
            "total_power_kw": inputs["chiller_power_kw"] + params["aux_power_kw"],
        }, state

