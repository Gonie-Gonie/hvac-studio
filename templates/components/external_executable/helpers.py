def external_contract():
    return {
        "request": "stdin JSON with component_id, inputs, state, params, and context",
        "response": "stdout JSON with outputs plus optional state, logs, or ok=false error",
    }
