from . import user_init, user_step


class VectorizedGain:
    def initialize(self, params, context):
        state = user_init.initialize(params, context)
        if state is None:
            return {}
        return state

    def evaluate_batch(self, inputs, state, params, context):
        return user_step.step(inputs, state, params, context)

    def evaluate(self, inputs, state, params, context):
        return self.evaluate_batch(inputs, state, params, context)
