$ErrorActionPreference = 'Stop'

. (Join-Path $PSScriptRoot 'env.ps1')
. (Join-Path $PSScriptRoot 'json-assert.ps1')

if (-not $env:HVAC_STUDIO_GO) {
  throw 'go was not found. Run scripts/dev/setup.ps1 first.'
}

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$ExamplesRoot = Join-Path $RepoRoot 'examples'

function Invoke-Example {
  param([Parameter(Mandatory = $true)][string]$ProjectPath)

  $ExampleRoot = Split-Path -Parent $ProjectPath
  $ExampleName = Split-Path -Leaf $ExampleRoot
  $InputPath = Join-Path $ExampleRoot 'inputs\case01.json'
  $ExpectedPath = Join-Path $ExampleRoot 'expected\output.json'
  $OutputPath = Join-Path ([IO.Path]::GetTempPath()) "hvac-studio-$ExampleName-output.json"

  if (-not (Test-Path -LiteralPath $InputPath)) {
    throw "$ExampleName is missing inputs/case01.json"
  }
  if (-not (Test-Path -LiteralPath $ExpectedPath)) {
    throw "$ExampleName is missing expected/output.json"
  }

  Write-Host "example: $ExampleName"
  Push-Location (Join-Path $RepoRoot 'tools\go')
  try {
    Invoke-Checked $env:HVAC_STUDIO_GO @('run', '.\cmd\bcs-runner', 'validate', '--project', $ProjectPath)
    Invoke-Checked $env:HVAC_STUDIO_GO @('run', '.\cmd\bcs-runner', 'run', '--project', $ProjectPath, '--input', $InputPath, '--output', $OutputPath)
  } finally {
    Pop-Location
  }

  $Expected = Get-Content -Raw -LiteralPath $ExpectedPath | ConvertFrom-Json
  $Actual = Get-Content -Raw -LiteralPath $OutputPath | ConvertFrom-Json
  Assert-JsonSubset -Expected $Expected -Actual $Actual -Path '$'
  Remove-Item -LiteralPath $OutputPath -Force -ErrorAction SilentlyContinue
}

$Projects = Get-ChildItem -LiteralPath $ExamplesRoot -Recurse -Filter 'project.bcsproj' |
  Sort-Object FullName

if ($Projects.Count -eq 0) {
  throw "no runnable examples found under $ExamplesRoot"
}

foreach ($Project in $Projects) {
  Invoke-Example -ProjectPath $Project.FullName
}

Write-Host "example tests ok: $($Projects.Count)"
