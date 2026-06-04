class KWLoad:
    def initialize(self, params, context):
        return {}

    def evaluate(self, inputs, state, params, context):
        return {"load_kw": float(inputs["power_kw"])}, state
