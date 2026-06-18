$ErrorActionPreference = 'Stop'

. (Join-Path $PSScriptRoot 'env.ps1')

$RepoRoot = Resolve-Path (Join-Path $PSScriptRoot '..\..')

& (Join-Path $RepoRoot 'scripts\dev\test-go.ps1')
& (Join-Path $RepoRoot 'scripts\dev\test-studio.ps1')
& (Join-Path $RepoRoot 'scripts\dev\test-acceptance-walkthroughs.ps1')
& (Join-Path $RepoRoot 'scripts\dev\test-python.ps1')
& (Join-Path $RepoRoot 'scripts\dev\test-examples.ps1')
& (Join-Path $RepoRoot 'scripts\dev\test-validation.ps1')
& (Join-Path $RepoRoot 'scripts\dev\test-docs.ps1')
& (Join-Path $RepoRoot 'scripts\dev\test-product-wording.ps1')
