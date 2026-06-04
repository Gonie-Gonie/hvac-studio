# Release Notes Policy

Every public release note should include:

- version and release date
- package artifacts and SHA-256 checksum location
- whether assets are signed or unsigned
- supported platforms and runtime requirements
- compatibility and migration notes
- new user-facing features
- changed behavior and breaking changes
- known issues and workarounds
- verification commands for portable, installer, and runtime packages
- upgrade guidance from the previous release line

Alpha, beta, release-candidate, development, and manually triggered release
builds must be marked as prereleases. Stable tags such as `v1.0.0` are the only
non-prerelease channel.

Release notes should not imply support for platforms, installers, file
associations, automatic updates, or code signing that are not present in the
package manifest and support matrix.
