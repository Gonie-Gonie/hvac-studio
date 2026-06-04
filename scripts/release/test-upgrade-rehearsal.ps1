param(
  [string]$Version = ''
)

$ErrorActionPreference = 'Stop'

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
. (Join-Path $RepoRoot 'scripts\dev\env.ps1')
. (Join-Path $RepoRoot 'scripts\dev\json-assert.ps1')
. (Join-Path $RepoRoot 'scripts\release\package-common.ps1')

if (-not $env:HVAC_STUDIO_GO) {
  throw 'go was not found. Run scripts/dev/setup.ps1 first.'
}
if (-not $env:HVAC_STUDIO_TEST_ROOT) {
  $env:HVAC_STUDIO_TEST_ROOT = Join-Path $RepoRoot 'artifacts\package-tests'
}

function Set-SchemaVersion {
  param(
    [Parameter(Mandatory = $true)][string]$Path,
    [Parameter(Mandatory = $true)][string]$SchemaVersion
  )

  $Document = Get-Content -Raw -LiteralPath $Path | ConvertFrom-Json
  $Document.schema_version = $SchemaVersion
  $Json = ($Document | ConvertTo-Json -Depth 32) + "`n"
  $Utf8NoBom = [System.Text.UTF8Encoding]::new($false)
  [System.IO.File]::WriteAllText($Path, $Json, $Utf8NoBom)
}

function Invoke-Runner {
  param([Parameter(Mandatory = $true)][string[]]$Arguments)

  Push-Location (Join-Path $RepoRoot 'tools\go')
  try {
    Invoke-Checked -FilePath $env:HVAC_STUDIO_GO -Arguments (@('run', '.\cmd\bcs-runner') + $Arguments)
  } finally {
    Pop-Location
  }
}

$TestRoot = New-PackageTestRoot -Prefix 'hvac-upgrade-rehearsal'

try {
  $SourceExample = Join-Path $RepoRoot 'examples\001_scalar_component'
  $ProjectRoot = Join-Path $TestRoot '001_scalar_component_019'
  Copy-Tree -Source $SourceExample -Destination $ProjectRoot
  Remove-PythonCaches -Root $ProjectRoot

  $ProjectPath = Join-Path $ProjectRoot 'project.bcsproj'
  $GraphPath = Join-Path $ProjectRoot 'graph.json'
  $InputPath = Join-Path $ProjectRoot 'inputs\case01.json'
  $ExpectedPath = Join-Path $ProjectRoot 'expected\output.json'
  $MigrationReportPath = Join-Path $TestRoot 'migration-report.json'
  $RunOutputPath = Join-Path $TestRoot 'run-output.json'

  Set-SchemaVersion -Path $ProjectPath -SchemaVersion '0.1.9'
  Set-SchemaVersion -Path $GraphPath -SchemaVersion '0.1.9'

  Invoke-Runner @('migrate', '--project', $ProjectPath, '--output', $MigrationReportPath)
  $Report = Get-Content -Raw -LiteralPath $MigrationReportPath | ConvertFrom-Json
  if (-not $Report.ok) {
    throw "upgrade rehearsal migration report is not ok: $($Report | ConvertTo-Json -Depth 8 -Compress)"
  }
  if (@($Report.artifacts).Count -ne 2) {
    throw "upgrade rehearsal expected project and graph artifacts, got $(@($Report.artifacts).Count)"
  }
  foreach ($Artifact in @($Report.artifacts)) {
    if ($Artifact.version -ne '0.1.9') {
      throw "upgrade rehearsal artifact version mismatch for $($Artifact.kind): $($Artifact.version)"
    }
    if (-not $Artifact.compatible -or $Artifact.needs_migration) {
      throw "upgrade rehearsal artifact unexpectedly requires migration: $($Artifact | ConvertTo-Json -Depth 6 -Compress)"
    }
  }
  if (-not (@($Report.actions) | Where-Object { $_.kind -eq 'no_migration_needed' })) {
    throw 'upgrade rehearsal report did not record no_migration_needed action'
  }

  Invoke-Runner @('validate', '--project', $ProjectPath)
  Invoke-Runner @('run', '--project', $ProjectPath, '--input', $InputPath, '--output', $RunOutputPath)

  $Expected = Get-Content -Raw -LiteralPath $ExpectedPath | ConvertFrom-Json
  $Actual = Get-Content -Raw -LiteralPath $RunOutputPath | ConvertFrom-Json
  Assert-JsonSubset -Expected $Expected -Actual $Actual -Path '$'

  $VersionLabel = if ($Version) { $Version } else { 'dev' }
  Write-Host "upgrade rehearsal ok: version=$VersionLabel project=$ProjectPath"
} finally {
  Remove-Item -LiteralPath $TestRoot -Recurse -Force -ErrorAction SilentlyContinue
}
