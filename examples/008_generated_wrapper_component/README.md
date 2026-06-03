# 008 Generated Wrapper Component

This example verifies the Milestone 2 component authoring boundary:

```text
components/custom_gain/component.json   GUI-managed component contract
components/custom_gain/wrapper.py       generated runtime adapter
components/custom_gain/user_init.py     user-editable initialization body
components/custom_gain/user_step.py     user-editable step body
components/custom_gain/helpers.py       user-editable helper functions
```

The runner imports `components.custom_gain.wrapper.CustomGainWrapper`. The wrapper delegates initialization and evaluation to the user-editable files, so the worker can execute a component body without requiring runtime-managed code regions inside the editable source.
