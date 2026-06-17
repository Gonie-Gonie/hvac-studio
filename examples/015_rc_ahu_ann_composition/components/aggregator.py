class OutputAggregator:
    def initialize(self, params, context):
        return {}

    def evaluate(self, inputs, state, params, context):
        zone_load = float(inputs["zone_load_kw"])
        zone_temperature = float(inputs["zone_temperature_c"])
        zone_setpoint = float(inputs["zone_setpoint_c"])
        chiller_power = float(inputs["chiller_power_kw"])
        pump_power = float(inputs["pump_power_kw"])

        auxiliary = zone_load * float(params["aux_kw_per_zone_kw"])
        unmet = max(zone_temperature - zone_setpoint - float(params["comfort_deadband_c"]), 0.0) * zone_load
        total = chiller_power + pump_power + auxiliary
        return {
            "total_power_kw": round(total, 6),
            "unmet_load_kw": round(unmet, 6),
        }, state
