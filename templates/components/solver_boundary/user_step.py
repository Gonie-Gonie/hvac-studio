from .helpers import positive_int


def step(inputs, state, params, context):
    target = float(inputs["target"])
    guess = float(inputs.get("initial", state.get("last_solution", target)))
    relaxation = float(params.get("relaxation", 0.5))
    max_iterations = positive_int(params.get("max_iterations", 10), 10)
    tolerance = float(params.get("tolerance", 0.001))

    converged = False
    iteration = 0
    for iteration in range(1, max_iterations + 1):
        next_guess = guess + relaxation * (target - guess)
        if abs(next_guess - guess) <= tolerance:
            guess = next_guess
            converged = True
            break
        guess = next_guess

    return {
        "solution": guess,
        "iterations": iteration,
        "converged": converged,
    }, {"last_solution": guess}
