class PlantAggregator:
    def initialize(self, params, context):
        return {}

    def evaluate(self, inputs, state, params, context):
        total = (
            inputs["chiller_power_kw"]
            + inputs["pump_power_kw"]
            + inputs["tower_fan_power_kw"]
            + params["aux_power_kw"]
        )
        return {
            "total_power_kw": total,
        }, state
