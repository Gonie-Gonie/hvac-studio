class PlantLoad:
    def initialize(self, params, context):
        return {}

    def evaluate(self, inputs, state, params, context):
        return {
            "plant_load_kw": inputs["building_load_kw"] * params["load_factor"],
        }, state
