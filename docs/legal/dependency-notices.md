# Dependency Notices

This notice summarizes the dependencies that are material to packaged releases.
It is a release-review aid, not a replacement for upstream license texts.

## Go Module

The Go module is `github.com/goniegonie/hvac-studio/tools/go`.

Direct dependency:

- `github.com/wailsapp/wails/v2`

Notable indirect dependencies include:

- `github.com/wailsapp/go-webview2`
- `github.com/gorilla/websocket`
- `github.com/labstack/echo/v4`
- `github.com/google/uuid`
- `golang.org/x/crypto`
- `golang.org/x/net`
- `golang.org/x/sys`
- `golang.org/x/text`

The authoritative list is `tools/go/go.mod` and `tools/go/go.sum`.

## Python Packages

The packaged Python worker and SDK currently use the Python standard library for
runtime execution. The authoritative package metadata is:

- `python/bcs_worker/pyproject.toml`
- `python/bcs_sdk/pyproject.toml`

## Release Requirement

Before a stable public release, generate or review a complete third-party notice
bundle for Go modules, Python packages, bundled Python, Wails/WebView2 runtime
requirements, and any installer tooling.
