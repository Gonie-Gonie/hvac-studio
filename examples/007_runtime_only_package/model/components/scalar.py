class DeliveredGain:
    def initialize(self, params, context):
        return {}

    def evaluate(self, inputs, state, params, context):
        return {
            "delivered_result": inputs["value"] * params["gain"] + params["offset"],
        }, state
