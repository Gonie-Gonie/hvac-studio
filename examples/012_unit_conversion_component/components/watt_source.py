class WattSource:
    def initialize(self, params, context):
        return {}

    def evaluate(self, inputs, state, params, context):
        return {"power_w": float(inputs["power_w"])}, state
