$ErrorActionPreference = 'Stop'

. (Join-Path $PSScriptRoot 'env.ps1')

if (-not $env:HVAC_STUDIO_GO) {
  throw 'go was not found. Run scripts/dev/setup.ps1 first.'
}
if (-not $env:HVAC_STUDIO_PYTHON) {
  throw 'python was not found. Run scripts/dev/setup.ps1 first.'
}

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$ExamplesRoot = Join-Path $RepoRoot 'examples'
$TempRoot = [IO.Path]::GetTempPath()

function Invoke-Runner {
  param([Parameter(Mandatory = $true)][string[]]$Arguments)

  Push-Location (Join-Path $RepoRoot 'tools\go')
  try {
    Invoke-Checked $env:HVAC_STUDIO_GO (@('run', '.\cmd\bcs-runner') + $Arguments)
  } finally {
    Pop-Location
  }
}

Push-Location (Join-Path $RepoRoot 'tools\go')
try {
  Write-Host 'acceptance walkthrough A: first project component run export'
  Invoke-Checked $env:HVAC_STUDIO_GO @('test', '.\internal\studio', '-run', 'TestAcceptanceWalkthroughFirstProjectComponentRunExport', '-count=1')
  Write-Host 'acceptance walkthrough B: ANN asset export'
  Invoke-Checked $env:HVAC_STUDIO_GO @('test', '.\internal\studio', '-run', 'TestExportEndpointIncludesMLAssetsAndChecksums', '-count=1')
} finally {
  Pop-Location
}

$AnnProject = Join-Path $ExamplesRoot '014_ahu_state_ann\project.bcsproj'
$AnnInput = Join-Path $ExamplesRoot '014_ahu_state_ann\inputs\case01.json'
$AnnOutput = Join-Path $TempRoot 'hvac-studio-acceptance-ann.json'
$CompositionProject = Join-Path $ExamplesRoot '015_rc_ahu_ann_composition\project.bcsproj'
$CompositionInput = Join-Path $ExamplesRoot '015_rc_ahu_ann_composition\inputs\case01.json'
$CompositionSeriesInput = Join-Path $ExamplesRoot '015_rc_ahu_ann_composition\inputs\series01.json'
$CompositionRunOutput = Join-Path $TempRoot 'hvac-studio-acceptance-composition-run.json'
$CompositionSeriesOutput = Join-Path $TempRoot 'hvac-studio-acceptance-composition-series.json'
$CompositionValidationOutput = Join-Path $TempRoot 'hvac-studio-acceptance-composition-validation.json'
$CompositionCalibrationOutput = Join-Path $TempRoot 'hvac-studio-acceptance-composition-calibration.json'
$CompositionOptimizationOutput = Join-Path $TempRoot 'hvac-studio-acceptance-composition-optimization.json'

try {
  Write-Host 'acceptance walkthrough B: ANN inference run'
  Invoke-Runner @('validate', '--project', $AnnProject)
  Invoke-Runner @('run', '--project', $AnnProject, '--input', $AnnInput, '--output', $AnnOutput)
  $Ann = Get-Content -Raw -Encoding UTF8 -LiteralPath $AnnOutput | ConvertFrom-Json
  if (-not $Ann.ok -or $Ann.outputs.supply_air_temperature_c -ne 19.46 -or $Ann.execution_order[1] -ne 'ahu_state_ann') {
    throw "ANN acceptance output mismatch: ok=$($Ann.ok) supply=$($Ann.outputs.supply_air_temperature_c)"
  }

  Write-Host 'acceptance walkthrough C: RC ANN equipment composition'
  Invoke-Runner @('run', '--project', $CompositionProject, '--input', $CompositionInput, '--output', $CompositionRunOutput)
  $CompositionRun = Get-Content -Raw -Encoding UTF8 -LiteralPath $CompositionRunOutput | ConvertFrom-Json
  if (-not $CompositionRun.ok -or $null -eq $CompositionRun.outputs.total_power_kw -or $null -eq $CompositionRun.outputs.zone_temperature_c) {
    throw "composition run acceptance failed: ok=$($CompositionRun.ok)"
  }
  Invoke-Runner @('run-series', '--project', $CompositionProject, '--input', $CompositionSeriesInput, '--output', $CompositionSeriesOutput)
  $CompositionSeries = Get-Content -Raw -Encoding UTF8 -LiteralPath $CompositionSeriesOutput | ConvertFrom-Json
  if (-not $CompositionSeries.ok -or $CompositionSeries.step_count -ne 3 -or $null -eq $CompositionSeries.final_states.zone_rc.zone_temperature_c) {
    throw "composition series acceptance failed: ok=$($CompositionSeries.ok) steps=$($CompositionSeries.step_count)"
  }
  Invoke-Runner @('validate-data', '--project', $CompositionProject, '--mapping', 'validation/mappings/rc_ahu_validation.json', '--output', $CompositionValidationOutput)
  $CompositionValidation = Get-Content -Raw -Encoding UTF8 -LiteralPath $CompositionValidationOutput | ConvertFrom-Json
  if (-not $CompositionValidation.ok -or $CompositionValidation.row_count -ne 3 -or $null -eq $CompositionValidation.metrics.total_power_kw) {
    throw "composition validation acceptance failed: ok=$($CompositionValidation.ok) rows=$($CompositionValidation.row_count)"
  }
  Invoke-Runner @('calibrate', '--project', $CompositionProject, '--setup', 'calibration/setups/chiller_cop_grid.json', '--output', $CompositionCalibrationOutput)
  $CompositionCalibration = Get-Content -Raw -Encoding UTF8 -LiteralPath $CompositionCalibrationOutput | ConvertFrom-Json
  if (-not $CompositionCalibration.ok -or $CompositionCalibration.candidates.Count -ne 3 -or $null -eq $CompositionCalibration.best_parameter_set.components.chiller.cop) {
    throw "composition calibration acceptance failed: ok=$($CompositionCalibration.ok)"
  }
  Invoke-Runner @('optimize', '--project', $CompositionProject, '--setup', 'optimization/setups/chw_pump_grid.json', '--output', $CompositionOptimizationOutput)
  $CompositionOptimization = Get-Content -Raw -Encoding UTF8 -LiteralPath $CompositionOptimizationOutput | ConvertFrom-Json
  if (-not $CompositionOptimization.ok -or $CompositionOptimization.candidates.Count -ne 9 -or $null -eq $CompositionOptimization.best_inputs.pump_speed_fraction) {
    throw "composition optimization acceptance failed: ok=$($CompositionOptimization.ok)"
  }

  Write-Host 'acceptance walkthrough D: SDK and serve protocol'
  Invoke-Checked $env:HVAC_STUDIO_PYTHON @('-m', 'unittest', 'discover', '-s', (Join-Path $RepoRoot 'python\bcs_sdk\tests'))
  & (Join-Path $PSScriptRoot 'test-serve-protocol.ps1')
} finally {
  Remove-Item -LiteralPath $AnnOutput -Force -ErrorAction SilentlyContinue
  Remove-Item -LiteralPath $CompositionRunOutput -Force -ErrorAction SilentlyContinue
  Remove-Item -LiteralPath $CompositionSeriesOutput -Force -ErrorAction SilentlyContinue
  Remove-Item -LiteralPath $CompositionValidationOutput -Force -ErrorAction SilentlyContinue
  Remove-Item -LiteralPath $CompositionCalibrationOutput -Force -ErrorAction SilentlyContinue
  Remove-Item -LiteralPath $CompositionOptimizationOutput -Force -ErrorAction SilentlyContinue
}

Write-Host 'acceptance walkthroughs ok'
