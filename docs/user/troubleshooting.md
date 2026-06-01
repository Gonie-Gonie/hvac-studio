# Troubleshooting

## Validation Error

Validation errors usually mean a project artifact has an invalid reference or unsupported graph shape.

Common causes:

- system references an unknown component
- public input references an unknown node
- connection references an unknown node
- input node has multiple incoming connections
- algebraic loop detected

## Python Worker Error

Python worker errors happen while loading, initializing, or evaluating user Python code.

Common causes:

- syntax error
- import error
- class name mismatch
- missing required output node
- non-JSON-serializable return value

## Missing Public Input

If a required public input is missing, add it to the run input file or save the default input from Studio.

## Bundled Python

Portable packages include `runtime/python`. Release smoke tests constrain `PATH` to prove bundled Python is used.

