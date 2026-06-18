# bcs-sdk

`bcs-sdk` is the Python convenience layer for HVAC Studio. It wraps
`bcs-runner serve` and keeps simulation, validation, calibration, optimization,
batch execution, schema export, and runtime export inspection on the same runner
path used by the CLI and Studio.

The package does not reimplement component execution logic. It starts or calls
`bcs-runner`, preserves structured runner errors, and provides pooling helpers
for external optimization or design-space scripts.
