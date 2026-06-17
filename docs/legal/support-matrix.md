# Support Matrix

## Supported For Current Packages

| Surface | Status | Notes |
| --- | --- | --- |
| Windows 10/11 x64 portable Studio | Supported alpha/beta path | Includes `HVAC Studio.exe`, CLI tools, examples, docs, and bundled Python. |
| Windows installer bundle | Beta readiness path | PowerShell installer bundle wraps the portable payload and is smoke-tested with `-PlanOnly`. |
| Runtime-only Windows package | Supported automation path | Contains runner, environment checker, SDK/worker sources, schemas, examples, docs, and bundled Python. |
| Runtime exports from Studio | Supported project delivery path | Exported folders include runner tools when package support files are available. |
| Experimental macOS support package | Experimental planning path | Contains package plan, prereq checks, docs, schemas, examples, SDK/worker sources, and explicit signing/notarization caveats. It is not a signed app. |

## Not Yet Supported

| Surface | Status | Notes |
| --- | --- | --- |
| Signed/notarized macOS app | Not yet supported | Requires native macOS build, Developer ID signing, notarization, stapling, and clean-machine platform smoke tests. |
| Linux package | Outside 1.0 scope | Engine code should remain OS-independent, but packaging is Windows-first. |
| Automatic updater | Unavailable | Release notes and checksums are the update mechanism for now. |
| `.bcsproj` file association | Policy only | Disabled until Studio supports project-file launch arguments. |

## Runtime Requirements

- Windows 10/11 x64.
- Microsoft Edge WebView2 Evergreen Runtime for the desktop Studio window.
- Bundled Python runtime for packaged examples and workspace execution.
- No system Python should be required for included package smoke workflows.

## Support Boundaries

The runtime validates contracts, executes user Python components, and records
structured errors. It does not certify HVAC physics, equipment performance, or
control safety.
