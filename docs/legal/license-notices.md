# License Notices

HVAC Studio packages must include this file until a root `LICENSE` file and full
third-party notice bundle are finalized.

## Project License

The repository does not currently declare a public open-source license. Treat
pre-1.0 packages as internal or explicitly authorized evaluation builds unless a
release includes a root `LICENSE` file.

Before a public stable release:

- add the project license at the repository root
- include the license in every package
- update this notice with the exact license name and obligations
- keep package checksums and release notes aligned with the licensed artifacts

## Bundled Runtime

Windows packages include a bundled Python runtime and Go-built executables.
Public releases must preserve upstream license notices for bundled runtimes and
third-party dependencies.

## Code Signing

Current development, alpha, beta, and release-candidate packages may be unsigned.
Stable public installer packages require a documented signing certificate,
signing timestamp, and verification instructions before release.
