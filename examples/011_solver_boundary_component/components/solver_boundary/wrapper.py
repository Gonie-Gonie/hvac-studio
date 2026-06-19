import json
from . import user_init, user_step


class SolverBoundaryComponent:
    """Studio-owned runtime wrapper.

    Component contract metadata is regenerated from graph.json/component.json.
    Edit user_step.py for model logic.

    Inputs: target, initial
    Outputs: solution, iterations, converged
    Parameters: max_iterations, relaxation, tolerance
    State: last_solution
    """

    input_nodes = json.loads("{\"initial\":{\"id\":\"initial\",\"name\":\"Initial Guess\",\"preset\":\"scalar_input\",\"direction\":\"inlet\",\"medium\":\"signal\",\"value_type\":\"float\",\"unit\":\"\",\"required\":false,\"default\":0},\"target\":{\"id\":\"target\",\"name\":\"Target\",\"preset\":\"scalar_input\",\"direction\":\"inlet\",\"medium\":\"signal\",\"value_type\":\"float\",\"unit\":\"\",\"required\":true,\"default\":null}}")
    output_nodes = json.loads("{\"converged\":{\"id\":\"converged\",\"name\":\"Converged\",\"preset\":\"scalar_output\",\"direction\":\"outlet\",\"medium\":\"signal\",\"value_type\":\"boolean\",\"unit\":\"\",\"required\":null,\"default\":null},\"iterations\":{\"id\":\"iterations\",\"name\":\"Iterations\",\"preset\":\"scalar_output\",\"direction\":\"outlet\",\"medium\":\"signal\",\"value_type\":\"integer\",\"unit\":\"count\",\"required\":null,\"default\":null},\"solution\":{\"id\":\"solution\",\"name\":\"Solution\",\"preset\":\"scalar_output\",\"direction\":\"outlet\",\"medium\":\"signal\",\"value_type\":\"float\",\"unit\":\"\",\"required\":null,\"default\":null}}")
    parameter_schema = json.loads("{\"max_iterations\":{\"display_name\":\"Max Iterations\",\"unit\":\"count\",\"default\":4,\"current\":4,\"role\":\"fixed\",\"group\":\"Solver\",\"description\":\"Maximum internal iterations before returning the latest estimate.\"},\"relaxation\":{\"display_name\":\"Relaxation\",\"unit\":\"ratio\",\"default\":0.5,\"current\":0.5,\"bounds\":{\"min\":0,\"max\":1},\"role\":\"fixed\",\"group\":\"Solver\",\"description\":\"Fraction of the target residual applied each iteration.\"},\"tolerance\":{\"display_name\":\"Tolerance\",\"default\":0,\"current\":0,\"role\":\"fixed\",\"group\":\"Solver\",\"description\":\"Convergence threshold for the change in solution.\"}}")
    state_schema = json.loads("{\"last_solution\":{\"display_name\":\"Last Solution\",\"initial\":0,\"description\":\"Previous solver estimate used as a warm start.\"}}")

    def initialize(self, params, context):
        state = user_init.initialize(params, context)
        if state is None:
            return {}
        return state

    def evaluate(self, inputs, state, params, context):
        return user_step.step(inputs, state, params, context)
