class ElectricChiller:
    def initialize(self, params, context):
        return {}

    def evaluate(self, inputs, state, params, context):
        lift_penalty = max(0.0, inputs["condenser_entering_temp_c"] - params["reference_condenser_temp_c"])
        power_kw = inputs["plant_load_kw"] / params["cop"] + lift_penalty * params["lift_penalty_kw_per_k"]
        return {
            "chiller_power_kw": power_kw,
            "chw_supply_temp_c": inputs["chw_setpoint_c"] + params["approach_c"],
        }, state
