# HVAC Studio Templates

These templates are copied into Windows portable Studio packages and are also used by Studio when creating new workspace projects.

Current templates:

- `projects/scalar`: a runnable scalar Python component project.
- `components/scalar`: the default generated-wrapper scalar component.
- `components/controller`: a proportional controller starting point.
- `components/stateful`: a stateful controller/component starting point with `state_defs`.
- `components/data_source`: a context-backed source placeholder.
- `components/data_sink`: a sink placeholder for consuming connected values.
- `components/utility`: a two-input utility calculation.
- `components/vectorized`: a vectorized execution placeholder for the P3 runner mode.
- `components/external_executable`: an external executable placeholder for the P3 runner mode.

Templates are source artifacts, not marketing samples. Keep them valid, runnable, and aligned with the runtime schemas.
