param(
  [string]$Version = '',
  [string]$PackagePath = ''
)

$ErrorActionPreference = 'Stop'

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
. (Join-Path $RepoRoot 'scripts\dev\env.ps1')
. (Join-Path $RepoRoot 'scripts\dev\json-assert.ps1')
. (Join-Path $RepoRoot 'scripts\release\package-common.ps1')

if (-not $PackagePath) {
  $PackageOutput = & (Join-Path $RepoRoot 'scripts\release\package-runtime.ps1') -Version $Version
  $PackagePath = ($PackageOutput | Select-Object -Last 1)
}

if (-not (Test-Path -LiteralPath $PackagePath)) {
  throw "package does not exist: $PackagePath"
}

$TestRoot = New-PackageTestRoot -Prefix 'hvac-runtime-test'
$OriginalPath = $env:PATH

try {
  Expand-Archive -LiteralPath $PackagePath -DestinationPath $TestRoot -Force
  $PackageDir = Get-ChildItem -LiteralPath $TestRoot -Directory | Select-Object -First 1
  if ($null -eq $PackageDir) {
    throw "package did not expand to a directory: $PackagePath"
  }

  $Runner = Join-Path $PackageDir.FullName 'bin\bcs-runner.exe'
  $EnvTool = Join-Path $PackageDir.FullName 'bin\bcs-env.exe'
  $PackagedPython = Join-Path $PackageDir.FullName 'runtime\python\python.exe'
  foreach ($RequiredPath in @($Runner, $EnvTool, $PackagedPython)) {
    if (-not (Test-Path -LiteralPath $RequiredPath)) {
      throw "runtime package is missing $RequiredPath"
    }
  }
  $env:PATH = Get-MinimalPackagePath -PackageRoot $PackageDir.FullName
  Invoke-Checked $PackagedPython @('--version')
  $EnvStatusRaw = & $EnvTool check --root $PackageDir.FullName --json
  if ($LASTEXITCODE -ne 0) {
    throw "bcs-env check failed: $EnvStatusRaw"
  }
  $EnvStatus = $EnvStatusRaw | ConvertFrom-Json
  if (-not $EnvStatus.ok) {
    throw "bcs-env reported runtime package problems: $($EnvStatus.problems -join '; ')"
  }
  if ($EnvStatus.mode -ne 'runtime-package') {
    throw "bcs-env mode mismatch: $($EnvStatus.mode)"
  }

  $Projects = Get-ChildItem -LiteralPath (Join-Path $PackageDir.FullName 'examples') -Recurse -Filter 'project.bcsproj' |
    Sort-Object FullName
  if ($Projects.Count -eq 0) {
    throw 'runtime package contains no runnable examples'
  }

  foreach ($Project in $Projects) {
    $ExampleRoot = Split-Path -Parent $Project.FullName
    $ExampleName = Split-Path -Leaf $ExampleRoot
    $Input = Join-Path $ExampleRoot 'inputs\case01.json'
    $Expected = Join-Path $ExampleRoot 'expected\output.json'
    $Output = Join-Path $PackageDir.FullName "outputs\$ExampleName.json"

    if (-not (Test-Path -LiteralPath $Input)) {
      throw "$ExampleName is missing inputs/case01.json"
    }
    if (-not (Test-Path -LiteralPath $Expected)) {
      throw "$ExampleName is missing expected/output.json"
    }

    Invoke-Checked $Runner @('validate', '--project', $Project.FullName)
    Invoke-Checked $Runner @('run', '--project', $Project.FullName, '--input', $Input, '--output', $Output)

    $ExpectedJson = Get-Content -Raw -LiteralPath $Expected | ConvertFrom-Json
    $ActualJson = Get-Content -Raw -LiteralPath $Output | ConvertFrom-Json
    Assert-JsonSubset -Expected $ExpectedJson -Actual $ActualJson -Path '$'
  }

  Write-Host "runtime package smoke test ok: $PackagePath examples=$($Projects.Count)"
} finally {
  $env:PATH = $OriginalPath
  Remove-Item -LiteralPath $TestRoot -Recurse -Force -ErrorAction SilentlyContinue
}
