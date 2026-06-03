class ConstantDeltaTPump:
    def initialize(self, params, context):
        return {}

    def evaluate(self, inputs, state, params, context):
        return {
            "pump_power_kw": inputs["plant_load_kw"] * params["pump_kw_per_cooling_kw"],
        }, state
