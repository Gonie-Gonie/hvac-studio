$ErrorActionPreference = 'Stop'

$RepoRoot = Resolve-Path (Join-Path $PSScriptRoot '..\..')
& (Join-Path $RepoRoot 'scripts\dev\test-runner.ps1')

