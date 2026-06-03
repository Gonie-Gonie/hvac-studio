$ErrorActionPreference = 'Stop'

. (Join-Path $PSScriptRoot 'env.ps1')
. (Join-Path $PSScriptRoot '..\release\package-common.ps1')

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$SiteRoot = Join-Path $RepoRoot 'dist\docs\ci-site'

Remove-Item -LiteralPath $SiteRoot -Recurse -Force -ErrorAction SilentlyContinue
Invoke-MkDocsBuild -RepoRoot $RepoRoot -SiteRoot $SiteRoot
Write-Host "docs html ok: $SiteRoot"
