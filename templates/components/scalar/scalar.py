class ScalarComponent:
    input_nodes = {}
    output_nodes = {}
    parameter_schema = {}
    state_schema = {}

    def initialize(self, params, context):
        return {}

    def evaluate(self, inputs, state, params, context):
        value = float(inputs["value"])
        gain = float(params.get("gain", 2.0))
        return {"result": value * gain}, state
