$ErrorActionPreference = 'Stop'

. (Join-Path $PSScriptRoot 'env.ps1')

if (-not $env:HVAC_STUDIO_GO) {
  throw 'go was not found. Run scripts/dev/setup.ps1 first.'
}

$RepoRoot = Resolve-Path (Join-Path $PSScriptRoot '..\..')
$Project = Join-Path $RepoRoot 'examples\001_scalar_component\project.bcsproj'
$Input = Join-Path $RepoRoot 'examples\001_scalar_component\inputs\case01.json'
$Output = Join-Path ([IO.Path]::GetTempPath()) 'bcs-runner-001-output.json'

Push-Location (Join-Path $RepoRoot 'tools\go')
try {
  Invoke-Checked $env:HVAC_STUDIO_GO @('run', '.\cmd\bcs-runner', 'validate', '--project', $Project)
  Invoke-Checked $env:HVAC_STUDIO_GO @('run', '.\cmd\bcs-runner', 'run', '--project', $Project, '--input', $Input, '--output', $Output)
  Get-Content -Raw -LiteralPath $Output
} finally {
  Pop-Location
}
