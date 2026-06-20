# Release Trust

HVAC Studio release trust is built from verifiable package contents, clear
support boundaries, and explicit release notes.

Required release trust assets:

- package-level `release-manifest.json`
- package-level `release-provenance.json` when the package has expanded runtime contents
- package-level `release-checksums.json` when the package has expanded runtime contents
- workflow-level `SHA256SUMS.txt`
- `release-trust.json`
- `legal/license-notices.md`
- `legal/dependency-notices.md`
- `legal/support-matrix.md`
- `legal/release-notes-policy.md`

Code signing status is recorded in `release-trust.json`. Current development,
alpha, beta, and release-candidate packages may be unsigned. Stable public
installer packages require signing and documented verification.

The experimental macOS support package also records explicit unsigned and
not-notarized status in `release-manifest.json` and
`macos/package-plan.json`. It is a packaging contract, not a Gatekeeper-ready
macOS application.
