class SupervisoryController:
    def initialize(self, params, context):
        return {}

    def evaluate(self, inputs, state, params, context):
        zone_load = float(inputs["zone_load_kw"])
        zone_temperature = float(inputs["zone_temperature_c"])
        zone_setpoint = float(inputs["zone_setpoint_c"])
        base_chw_setpoint = float(inputs["chw_setpoint_c"])
        pump_speed = float(inputs["pump_speed_fraction"])

        error_kw = max(zone_temperature - zone_setpoint, 0.0) * float(params["temperature_error_gain_kw_per_k"])
        cooling_request = max(0.0, zone_load + error_kw)
        reset = min(float(params["max_reset_c"]), cooling_request * float(params["chw_reset_c_per_kw"]))
        commanded_chw_setpoint = base_chw_setpoint - reset

        return {
            "cooling_request_kw": round(cooling_request, 6),
            "chw_setpoint_command_c": round(commanded_chw_setpoint, 6),
            "pump_speed_fraction": pump_speed,
        }, state
