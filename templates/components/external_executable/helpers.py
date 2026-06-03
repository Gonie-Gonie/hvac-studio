def unsupported_external_mode(command):
    raise RuntimeError(
        "external_executable components need the P3 external process runner before execution: "
        + str(command)
    )
