param(
  [string]$Version = '',
  [string]$PackagePath = ''
)

$ErrorActionPreference = 'Stop'

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
. (Join-Path $RepoRoot 'scripts\dev\env.ps1')

if (-not $PackagePath) {
  $PackageOutput = & (Join-Path $RepoRoot 'scripts\release\package-runtime.ps1') -Version $Version
  $PackagePath = ($PackageOutput | Select-Object -Last 1)
}

if (-not (Test-Path -LiteralPath $PackagePath)) {
  throw "package does not exist: $PackagePath"
}

$TestRoot = Join-Path ([IO.Path]::GetTempPath()) ('hvac-studio-release-test-' + [Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Force -Path $TestRoot | Out-Null

try {
  Expand-Archive -LiteralPath $PackagePath -DestinationPath $TestRoot -Force
  $PackageDir = Get-ChildItem -LiteralPath $TestRoot -Directory | Select-Object -First 1
  if ($null -eq $PackageDir) {
    throw "package did not expand to a directory: $PackagePath"
  }

  $Runner = Join-Path $PackageDir.FullName 'bin\bcs-runner.exe'
  $Project = Join-Path $PackageDir.FullName 'examples\001_scalar_component\project.bcsproj'
  $Input = Join-Path $PackageDir.FullName 'examples\001_scalar_component\inputs\case01.json'
  $Output = Join-Path $PackageDir.FullName 'outputs\001_scalar_component.json'

  Invoke-Checked $Runner @('validate', '--project', $Project)
  Invoke-Checked $Runner @('run', '--project', $Project, '--input', $Input, '--output', $Output)

  $Result = Get-Content -Raw -LiteralPath $Output | ConvertFrom-Json
  if (-not $Result.ok) {
    throw 'runtime package run returned ok=false'
  }
  if ([double]$Result.outputs.result -ne 10.0) {
    throw "unexpected example result: $($Result.outputs.result)"
  }

  Write-Host "runtime package smoke test ok: $PackagePath"
} finally {
  Remove-Item -LiteralPath $TestRoot -Recurse -Force -ErrorAction SilentlyContinue
}
