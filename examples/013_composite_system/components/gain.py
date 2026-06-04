class Gain:
    input_nodes = {
        "value": {
            "medium": "signal",
            "value_type": "float",
            "required": True,
        }
    }

    output_nodes = {
        "result": {
            "medium": "signal",
            "value_type": "float",
        }
    }

    parameter_schema = {
        "gain": {
            "type": "float",
            "required": True,
        }
    }

    def initialize(self, params, context):
        return {"calls": 0}

    def evaluate(self, inputs, state, params, context):
        calls = state.get("calls", 0) + 1
        return {
            "result": float(inputs["value"]) * float(params.get("gain", 1.0)),
        }, {
            "calls": calls,
        }
