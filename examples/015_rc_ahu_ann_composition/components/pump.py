class VariableSpeedPump:
    def initialize(self, params, context):
        return {}

    def evaluate(self, inputs, state, params, context):
        speed = max(0.0, min(1.0, float(inputs["pump_speed_fraction"])))
        cooling_request = max(float(inputs["cooling_request_kw"]), 0.0)
        power = float(params["standby_kw"]) + float(params["design_kw"]) * speed ** 3
        power += cooling_request * float(params["request_kw_per_cooling_kw"])
        return {"pump_power_kw": round(power, 6)}, state
