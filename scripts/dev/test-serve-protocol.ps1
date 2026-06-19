$ErrorActionPreference = 'Stop'

. (Join-Path $PSScriptRoot 'env.ps1')

if (-not $env:HVAC_STUDIO_GO) {
  throw 'go was not found. Run scripts/dev/setup.ps1 first.'
}
if (-not $env:HVAC_STUDIO_PYTHON) {
  throw 'python was not found. Run scripts/dev/setup.ps1 first.'
}

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$RequestPath = Join-Path $RepoRoot 'examples\sdk\serve-requests.jsonl'
$ProjectPath = Join-Path $RepoRoot 'examples\001_scalar_component\project.bcsproj'
$SchemaPaths = @(
  (Join-Path $RepoRoot 'schema\serve-request.schema.json'),
  (Join-Path $RepoRoot 'schema\serve-response.schema.json')
)

foreach ($SchemaPath in $SchemaPaths) {
  if (-not (Test-Path -LiteralPath $SchemaPath)) {
    throw "serve protocol schema is missing: $SchemaPath"
  }
  $Schema = Get-Content -Raw -Encoding UTF8 -LiteralPath $SchemaPath | ConvertFrom-Json
  if (-not $Schema.'$schema' -or -not $Schema.title) {
    throw "serve protocol schema metadata is incomplete: $SchemaPath"
  }
}

function Assert-ServeResponses {
  param(
    [Parameter(Mandatory = $true)][string[]]$ResponseLines,
    [Parameter(Mandatory = $true)][string]$Label
  )

  $ResponseLines = @($ResponseLines | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })
  if ($ResponseLines.Count -ne 4) {
    throw "$Label expected 4 response lines, got $($ResponseLines.Count): $($ResponseLines -join '; ')"
  }

  $Responses = @($ResponseLines | ForEach-Object { $_ | ConvertFrom-Json })
  $Case1 = $Responses[0]
  $Case2 = $Responses[1]
  $Bad = $Responses[2]
  $Shutdown = $Responses[3]

  if (-not $Case1.ok -or $Case1.id -ne 'case-1' -or [math]::Abs([double]$Case1.result.outputs.result - 10.0) -gt 0.000001) {
    throw "$Label case-1 response mismatch: $($ResponseLines[0])"
  }
  if (-not $Case2.ok -or $Case2.id -ne 'case-2' -or [math]::Abs([double]$Case2.result.outputs.result - 12.5) -gt 0.000001) {
    throw "$Label case-2 response mismatch: $($ResponseLines[1])"
  }
  if ($Bad.ok -or $Bad.id -ne 'bad-missing-input' -or $Bad.error.kind -ne 'input' -or -not ($Bad.error.message -match 'missing required public input')) {
    throw "$Label structured error mismatch: $($ResponseLines[2])"
  }
  if (-not $Shutdown.ok -or $Shutdown.id -ne 'stop' -or $Shutdown.message -ne 'shutdown') {
    throw "$Label shutdown response mismatch: $($ResponseLines[3])"
  }
}

Push-Location (Join-Path $RepoRoot 'tools\go')
try {
  $ResponseLines = @(Get-Content -Encoding UTF8 -LiteralPath $RequestPath | & $env:HVAC_STUDIO_GO run '.\cmd\bcs-runner' serve --project $ProjectPath)
  if ($LASTEXITCODE -ne 0) {
    throw "serve protocol smoke failed with exit code $LASTEXITCODE"
  }
  Assert-ServeResponses -ResponseLines $ResponseLines -Label 'serve protocol smoke'

  $RawSubprocessExample = Join-Path $RepoRoot 'examples\sdk\raw_serve_subprocess.py'
  $RawResponseLines = @(& $env:HVAC_STUDIO_PYTHON $RawSubprocessExample $env:HVAC_STUDIO_GO run '.\cmd\bcs-runner')
  if ($LASTEXITCODE -ne 0) {
    throw "raw serve subprocess example failed with exit code $LASTEXITCODE"
  }
  Assert-ServeResponses -ResponseLines $RawResponseLines -Label 'raw serve subprocess example'
} finally {
  Pop-Location
}

Write-Host 'serve protocol smoke ok'
