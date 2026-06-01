# Data Validation

Model validation compares simulated outputs against measured or reference data.

## Target Workflow

1. Import a dataset.
2. Detect columns.
3. Map dataset columns to public inputs.
4. Map observed columns to public outputs.
5. Run simulations.
6. Compute validation metrics.
7. Inspect high-error timesteps.

## Metrics

Planned metrics include:

- RMSE
- MAE
- MBE
- CVRMSE
- R2

Validation should not automatically change parameters. Calibration is the workflow that estimates parameters from observed data.

