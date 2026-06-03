class ResetController:
    def initialize(self, params, context):
        return {}

    def evaluate(self, inputs, state, params, context):
        load_delta = inputs["plant_load_kw"] - params["reference_load_kw"]
        reset = (load_delta / 100.0) * params["reset_per_100kw"]
        return {
            "chw_setpoint_c": inputs["base_chw_setpoint_c"] - reset,
        }, state
