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
$RunRoot = Join-Path ([IO.Path]::GetTempPath()) ("hvac-studio-acceptance-" + [Guid]::NewGuid().ToString('N'))

function Invoke-Runner {
  param([Parameter(Mandatory = $true)][string[]]$Arguments)

  Push-Location (Join-Path $RepoRoot 'tools\go')
  try {
    Invoke-Checked $env:HVAC_STUDIO_GO (@('run', '.\cmd\bcs-runner') + $Arguments)
  } finally {
    Pop-Location
  }
}

function Read-JsonFile {
  param([Parameter(Mandatory = $true)][string]$Path)

  return Get-Content -Raw -Encoding UTF8 -LiteralPath $Path | ConvertFrom-Json
}

function Invoke-RunnerJson {
  param(
    [Parameter(Mandatory = $true)][string[]]$Arguments,
    [Parameter(Mandatory = $true)][string]$OutputPath
  )

  Invoke-Runner $Arguments
  return Read-JsonFile -Path $OutputPath
}

function Assert-Acceptance {
  param(
    [Parameter(Mandatory = $true)][bool]$Condition,
    [Parameter(Mandatory = $true)][string]$Message
  )

  if (-not $Condition) {
    throw $Message
  }
}

Push-Location (Join-Path $RepoRoot 'tools\go')
try {
  Write-Host 'acceptance walkthrough A: first project component run export'
  Invoke-Checked $env:HVAC_STUDIO_GO @('test', '.\internal\studio', '-run', 'TestAcceptanceWalkthroughFirstProjectComponentRunExport', '-count=1')
  Write-Host 'acceptance walkthrough B: ANN asset export'
  Invoke-Checked $env:HVAC_STUDIO_GO @('test', '.\internal\studio', '-run', 'TestExportEndpointIncludesMLAssetsAndChecksums', '-count=1')
  Write-Host 'acceptance walkthrough C: composition runtime export'
  Invoke-Checked $env:HVAC_STUDIO_GO @('test', '.\internal\studio', '-run', 'TestExportEndpointRunsCompositionRuntimeWorkflows', '-count=1')
  Write-Host 'acceptance walkthrough E: error recovery'
  Invoke-Checked $env:HVAC_STUDIO_GO @('test', '.\internal\studio', '-run', 'TestAcceptanceWalkthroughErrorRecovery', '-count=1')
} finally {
  Pop-Location
}

New-Item -ItemType Directory -Force -Path $RunRoot | Out-Null
$AnnProject = Join-Path $ExamplesRoot '014_ahu_state_ann\project.bcsproj'
$AnnInput = Join-Path $ExamplesRoot '014_ahu_state_ann\inputs\case01.json'
$AnnOutput = Join-Path $RunRoot 'ann.json'
$CompositionProject = Join-Path $ExamplesRoot '015_rc_ahu_ann_composition\project.bcsproj'
$CompositionInput = Join-Path $ExamplesRoot '015_rc_ahu_ann_composition\inputs\case01.json'
$CompositionSeriesInput = Join-Path $ExamplesRoot '015_rc_ahu_ann_composition\inputs\series01.json'
$CompositionRunOutput = Join-Path $RunRoot 'composition-run.json'
$CompositionSeriesOutput = Join-Path $RunRoot 'composition-series.json'
$CompositionValidationOutput = Join-Path $RunRoot 'composition-validation.json'
$CompositionCalibrationOutput = Join-Path $RunRoot 'composition-calibration.json'
$CompositionOptimizationOutput = Join-Path $RunRoot 'composition-optimization.json'

try {
  Write-Host 'acceptance walkthrough B: ANN inference run'
  Invoke-Runner @('validate', '--project', $AnnProject)
  $Ann = Invoke-RunnerJson -Arguments @('run', '--project', $AnnProject, '--input', $AnnInput, '--output', $AnnOutput) -OutputPath $AnnOutput
  Assert-Acceptance `
    -Condition ($Ann.ok -and $Ann.outputs.supply_air_temperature_c -eq 19.46 -and $Ann.execution_order[1] -eq 'ahu_state_ann') `
    -Message "ANN acceptance output mismatch: ok=$($Ann.ok) supply=$($Ann.outputs.supply_air_temperature_c)"

  Write-Host 'acceptance walkthrough C: RC ANN equipment composition'
  $CompositionRun = Invoke-RunnerJson -Arguments @('run', '--project', $CompositionProject, '--input', $CompositionInput, '--output', $CompositionRunOutput) -OutputPath $CompositionRunOutput
  Assert-Acceptance `
    -Condition ($CompositionRun.ok -and $null -ne $CompositionRun.outputs.total_power_kw -and $null -ne $CompositionRun.outputs.zone_temperature_c) `
    -Message "composition run acceptance failed: ok=$($CompositionRun.ok)"

  $CompositionSeries = Invoke-RunnerJson -Arguments @('run-series', '--project', $CompositionProject, '--input', $CompositionSeriesInput, '--output', $CompositionSeriesOutput) -OutputPath $CompositionSeriesOutput
  Assert-Acceptance `
    -Condition ($CompositionSeries.ok -and $CompositionSeries.step_count -eq 3 -and $null -ne $CompositionSeries.final_states.zone_rc.zone_temperature_c) `
    -Message "composition series acceptance failed: ok=$($CompositionSeries.ok) steps=$($CompositionSeries.step_count)"

  $CompositionValidation = Invoke-RunnerJson -Arguments @('validate-data', '--project', $CompositionProject, '--mapping', 'validation/mappings/rc_ahu_validation.json', '--output', $CompositionValidationOutput) -OutputPath $CompositionValidationOutput
  Assert-Acceptance `
    -Condition ($CompositionValidation.ok -and $CompositionValidation.row_count -eq 3 -and $null -ne $CompositionValidation.metrics.total_power_kw) `
    -Message "composition validation acceptance failed: ok=$($CompositionValidation.ok) rows=$($CompositionValidation.row_count)"

  $CompositionCalibration = Invoke-RunnerJson -Arguments @('calibrate', '--project', $CompositionProject, '--setup', 'calibration/setups/chiller_cop_grid.json', '--output', $CompositionCalibrationOutput) -OutputPath $CompositionCalibrationOutput
  Assert-Acceptance `
    -Condition ($CompositionCalibration.ok -and $CompositionCalibration.candidates.Count -eq 3 -and $null -ne $CompositionCalibration.best_parameter_set.components.chiller.cop) `
    -Message "composition calibration acceptance failed: ok=$($CompositionCalibration.ok)"

  $CompositionOptimization = Invoke-RunnerJson -Arguments @('optimize', '--project', $CompositionProject, '--setup', 'optimization/setups/chw_pump_grid.json', '--output', $CompositionOptimizationOutput) -OutputPath $CompositionOptimizationOutput
  Assert-Acceptance `
    -Condition ($CompositionOptimization.ok -and $CompositionOptimization.candidates.Count -eq 9 -and $null -ne $CompositionOptimization.best_inputs.pump_speed_fraction) `
    -Message "composition optimization acceptance failed: ok=$($CompositionOptimization.ok)"

  Write-Host 'acceptance walkthrough D: SDK and serve protocol'
  Invoke-Checked $env:HVAC_STUDIO_PYTHON @('-m', 'unittest', 'discover', '-s', (Join-Path $RepoRoot 'python\bcs_sdk\tests'))
  & (Join-Path $PSScriptRoot 'test-serve-protocol.ps1')
} finally {
  Remove-Item -LiteralPath $RunRoot -Recurse -Force -ErrorAction SilentlyContinue
}

Write-Host 'acceptance walkthroughs ok'
