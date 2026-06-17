class ChillerLikeModel:
    def initialize(self, params, context):
        return {}

    def evaluate(self, inputs, state, params, context):
        coil_load = max(float(inputs["coil_load_kw"]), 0.0)
        chw_setpoint = float(inputs["chw_setpoint_command_c"])
        approach = min(float(params["max_approach_c"]), coil_load * float(params["approach_c_per_kw"]))
        chiller_power = coil_load / float(params["cop"]) + float(params["parasitic_kw"])
        return {
            "chiller_power_kw": round(chiller_power, 6),
            "chw_supply_temperature_c": round(chw_setpoint + approach, 6),
        }, state
