class CoolingTower:
    def initialize(self, params, context):
        return {}

    def evaluate(self, inputs, state, params, context):
        heat_rejection_kw = inputs["plant_load_kw"] + inputs["chiller_power_kw"]
        return {
            "tower_fan_power_kw": heat_rejection_kw * params["fan_kw_per_heat_kw"],
        }, state
