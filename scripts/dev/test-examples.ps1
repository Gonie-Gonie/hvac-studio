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
  $SeriesInputPath = Join-Path $ExampleRoot 'inputs\series01.json'
  $SeriesExpectedPath = Join-Path $ExampleRoot 'expected\series_output.json'
  $SeriesOutputPath = Join-Path ([IO.Path]::GetTempPath()) "hvac-studio-$ExampleName-series-output.json"

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

  if (Test-Path -LiteralPath $SeriesInputPath) {
    if (-not (Test-Path -LiteralPath $SeriesExpectedPath)) {
      throw "$ExampleName is missing expected/series_output.json"
    }
    Write-Host "example series: $ExampleName"
    Push-Location (Join-Path $RepoRoot 'tools\go')
    try {
      Invoke-Checked $env:HVAC_STUDIO_GO @('run', '.\cmd\bcs-runner', 'run-series', '--project', $ProjectPath, '--input', $SeriesInputPath, '--output', $SeriesOutputPath)
    } finally {
      Pop-Location
    }
    $SeriesExpected = Get-Content -Raw -LiteralPath $SeriesExpectedPath | ConvertFrom-Json
    $SeriesActual = Get-Content -Raw -LiteralPath $SeriesOutputPath | ConvertFrom-Json
    Assert-JsonSubset -Expected $SeriesExpected -Actual $SeriesActual -Path '$'
    Remove-Item -LiteralPath $SeriesOutputPath -Force -ErrorAction SilentlyContinue
  }
}

function Remove-EmptyDirectory {
  param([Parameter(Mandatory = $true)][string]$Path)

  if (-not (Test-Path -LiteralPath $Path -PathType Container)) {
    return
  }
  $Children = @(Get-ChildItem -LiteralPath $Path -Force -ErrorAction SilentlyContinue)
  if ($Children.Count -eq 0) {
    Remove-Item -LiteralPath $Path -Force
  }
}

function Invoke-WorkflowSmoke {
  $PlantProject = Join-Path $ExamplesRoot '005_chiller_plant_like_system\project.bcsproj'
  $OptimizationProject = Join-Path $ExamplesRoot '006_optimization_case\project.bcsproj'
  $CompositionProject = Join-Path $ExamplesRoot '015_rc_ahu_ann_composition\project.bcsproj'
  $ValidationOutput = Join-Path ([IO.Path]::GetTempPath()) 'hvac-studio-plant-validation.json'
  $CalibrationOutput = Join-Path ([IO.Path]::GetTempPath()) 'hvac-studio-plant-calibration.json'
  $OptimizationOutput = Join-Path ([IO.Path]::GetTempPath()) 'hvac-studio-optimization.json'
  $ParameterOptimizationOutput = Join-Path ([IO.Path]::GetTempPath()) 'hvac-studio-parameter-optimization.json'
  $ParameterOptimizationSet = 'parameter_sets/parameter_credit_grid_smoke.json'
  $ParameterOptimizationSetPath = Join-Path (Split-Path -Parent $OptimizationProject) $ParameterOptimizationSet
  $CompositionValidationOutput = Join-Path ([IO.Path]::GetTempPath()) 'hvac-studio-composition-validation.json'
  $CompositionCalibrationOutput = Join-Path ([IO.Path]::GetTempPath()) 'hvac-studio-composition-calibration.json'
  $CompositionOptimizationOutput = Join-Path ([IO.Path]::GetTempPath()) 'hvac-studio-composition-optimization.json'

  Push-Location (Join-Path $RepoRoot 'tools\go')
  try {
    Write-Host 'example workflow: plant validation'
    Invoke-Checked $env:HVAC_STUDIO_GO @('run', '.\cmd\bcs-runner', 'validate-data', '--project', $PlantProject, '--mapping', 'validation/mappings/plant_validation.json', '--output', $ValidationOutput)
    $Validation = Get-Content -Raw -LiteralPath $ValidationOutput | ConvertFrom-Json
    if (-not $Validation.ok -or $Validation.row_count -ne 3) {
      throw "plant validation smoke failed: ok=$($Validation.ok) rows=$($Validation.row_count)"
    }
    if ($null -eq $Validation.rows[0].time) {
      throw 'plant validation smoke did not preserve time-column row values'
    }
    if ($null -eq $Validation.metrics.total_power_kw) {
      throw 'plant validation smoke did not compute total_power_kw metrics'
    }

    Write-Host 'example workflow: plant calibration'
    Invoke-Checked $env:HVAC_STUDIO_GO @('run', '.\cmd\bcs-runner', 'calibrate', '--project', $PlantProject, '--setup', 'calibration/setups/chiller_cop_grid.json', '--output', $CalibrationOutput)
    $Calibration = Get-Content -Raw -LiteralPath $CalibrationOutput | ConvertFrom-Json
    if (-not $Calibration.ok -or $Calibration.candidates.Count -lt 1) {
      throw "plant calibration smoke failed: ok=$($Calibration.ok) candidates=$($Calibration.candidates.Count)"
    }
    if ($null -eq $Calibration.best_parameter_set.components.chiller.cop) {
      throw 'plant calibration smoke did not report a best chiller COP'
    }

    Write-Host 'example workflow: optimization'
    Invoke-Checked $env:HVAC_STUDIO_GO @('run', '.\cmd\bcs-runner', 'optimize', '--project', $OptimizationProject, '--setup', 'optimization/setups/chw_setpoint_grid.json', '--output', $OptimizationOutput)
    $Optimization = Get-Content -Raw -LiteralPath $OptimizationOutput | ConvertFrom-Json
    if (-not $Optimization.ok -or $Optimization.candidates.Count -lt 1) {
      throw "optimization smoke failed: ok=$($Optimization.ok) candidates=$($Optimization.candidates.Count)"
    }
    if ($null -eq $Optimization.best_inputs.chw_setpoint_c) {
      throw 'optimization smoke did not report best chw_setpoint_c input'
    }

    Write-Host 'example workflow: parameter optimization'
    Invoke-Checked $env:HVAC_STUDIO_GO @('run', '.\cmd\bcs-runner', 'optimize', '--project', $OptimizationProject, '--setup', 'optimization/setups/parameter_credit_grid.json', '--save-parameter-set', $ParameterOptimizationSet, '--output', $ParameterOptimizationOutput)
    $ParameterOptimization = Get-Content -Raw -LiteralPath $ParameterOptimizationOutput | ConvertFrom-Json
    if (-not $ParameterOptimization.ok -or $ParameterOptimization.best_objective -ne 88) {
      throw "parameter optimization smoke failed: ok=$($ParameterOptimization.ok) best=$($ParameterOptimization.best_objective)"
    }
    if ($null -eq $ParameterOptimization.best_parameters.tradeoff.power_credit_kw_per_k) {
      throw 'parameter optimization smoke did not report best component parameter'
    }
    if (-not (Test-Path -LiteralPath $ParameterOptimizationSetPath)) {
      throw 'parameter optimization smoke did not write a parameter set'
    }

    Write-Host 'example workflow: rc ahu validation'
    Invoke-Checked $env:HVAC_STUDIO_GO @('run', '.\cmd\bcs-runner', 'validate-data', '--project', $CompositionProject, '--mapping', 'validation/mappings/rc_ahu_validation.json', '--output', $CompositionValidationOutput)
    $CompositionValidation = Get-Content -Raw -LiteralPath $CompositionValidationOutput | ConvertFrom-Json
    if (-not $CompositionValidation.ok -or $CompositionValidation.row_count -ne 3) {
      throw "rc ahu validation smoke failed: ok=$($CompositionValidation.ok) rows=$($CompositionValidation.row_count)"
    }
    if ($null -eq $CompositionValidation.metrics.total_power_kw -or $null -eq $CompositionValidation.metrics.supply_air_temperature_c) {
      throw 'rc ahu validation smoke did not compute expected metrics'
    }

    Write-Host 'example workflow: rc ahu calibration'
    Invoke-Checked $env:HVAC_STUDIO_GO @('run', '.\cmd\bcs-runner', 'calibrate', '--project', $CompositionProject, '--setup', 'calibration/setups/chiller_cop_grid.json', '--output', $CompositionCalibrationOutput)
    $CompositionCalibration = Get-Content -Raw -LiteralPath $CompositionCalibrationOutput | ConvertFrom-Json
    if (-not $CompositionCalibration.ok -or $CompositionCalibration.candidates.Count -ne 3) {
      throw "rc ahu calibration smoke failed: ok=$($CompositionCalibration.ok) candidates=$($CompositionCalibration.candidates.Count)"
    }
    if ($null -eq $CompositionCalibration.best_parameter_set.components.chiller.cop) {
      throw 'rc ahu calibration smoke did not report a best chiller COP'
    }

    Write-Host 'example workflow: rc ahu optimization'
    Invoke-Checked $env:HVAC_STUDIO_GO @('run', '.\cmd\bcs-runner', 'optimize', '--project', $CompositionProject, '--setup', 'optimization/setups/chw_pump_grid.json', '--output', $CompositionOptimizationOutput)
    $CompositionOptimization = Get-Content -Raw -LiteralPath $CompositionOptimizationOutput | ConvertFrom-Json
    if (-not $CompositionOptimization.ok -or $CompositionOptimization.candidates.Count -ne 9) {
      throw "rc ahu optimization smoke failed: ok=$($CompositionOptimization.ok) candidates=$($CompositionOptimization.candidates.Count)"
    }
    if ($null -eq $CompositionOptimization.best_inputs.chw_setpoint_c -or $null -eq $CompositionOptimization.best_inputs.pump_speed_fraction) {
      throw 'rc ahu optimization smoke did not report best control inputs'
    }
  } finally {
    Pop-Location
    Remove-Item -LiteralPath $ValidationOutput -Force -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath $CalibrationOutput -Force -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath $OptimizationOutput -Force -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath $ParameterOptimizationOutput -Force -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath $ParameterOptimizationSetPath -Force -ErrorAction SilentlyContinue
    Remove-EmptyDirectory -Path (Split-Path -Parent $ParameterOptimizationSetPath)
    Remove-Item -LiteralPath $CompositionValidationOutput -Force -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath $CompositionCalibrationOutput -Force -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath $CompositionOptimizationOutput -Force -ErrorAction SilentlyContinue
  }
}

$Projects = Get-ChildItem -LiteralPath $ExamplesRoot -Recurse -Filter 'project.bcsproj' |
  Sort-Object FullName

if ($Projects.Count -eq 0) {
  throw "no runnable examples found under $ExamplesRoot"
}

foreach ($Project in $Projects) {
  Invoke-Example -ProjectPath $Project.FullName
}

Invoke-WorkflowSmoke
& (Join-Path $PSScriptRoot 'test-serve-protocol.ps1')

Write-Host "example tests ok: $($Projects.Count)"
