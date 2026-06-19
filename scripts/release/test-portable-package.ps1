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
  $PackageOutput = & (Join-Path $RepoRoot 'scripts\release\package-portable.ps1') -Version $Version
  $PackagePath = ($PackageOutput | Select-Object -Last 1)
}

if (-not (Test-Path -LiteralPath $PackagePath)) {
  throw "package does not exist: $PackagePath"
}

function Get-FreePort {
  $Listener = [Net.Sockets.TcpListener]::new([Net.IPAddress]::Parse('127.0.0.1'), 0)
  $Listener.Start()
  try {
    return $Listener.LocalEndpoint.Port
  } finally {
    $Listener.Stop()
  }
}

function Get-ComponentSourcePaths {
  param(
    [Parameter(Mandatory = $true)][string]$ProjectPath,
    [Parameter(Mandatory = $true)]$Component
  )

  $ProjectRoot = Split-Path -Parent $ProjectPath
  $Source = $Component.source
  $Required = @()
  if ($Source -and $Source.layout -eq 'generated_wrapper') {
    foreach ($RelativePath in @($Source.step, $Source.wrapper, $Source.metadata)) {
      if ($RelativePath) {
        $Required += $RelativePath
      }
    }
  } elseif ($Source -and $Source.step) {
    $Required += $Source.step
  } else {
    $Required += "components\$($Component.id).py"
  }
  foreach ($RelativePath in $Required) {
    Join-Path $ProjectRoot (($RelativePath -as [string]) -replace '/', '\')
  }
}

function Assert-ComponentSourceWritten {
  param(
    [Parameter(Mandatory = $true)][string]$ProjectPath,
    [Parameter(Mandatory = $true)]$Component,
    [Parameter(Mandatory = $true)][string]$Label
  )

  foreach ($SourcePath in (Get-ComponentSourcePaths -ProjectPath $ProjectPath -Component $Component)) {
    if (-not (Test-Path -LiteralPath $SourcePath)) {
      throw "$Label source was not written: $SourcePath"
    }
  }
}

$TestRoot = New-PackageTestRoot -Prefix 'hvac-portable-test'

$StudioProcess = $null
$DesktopProcess = $null
$ErrLog = ''
$OriginalPath = $env:PATH

try {
  Expand-Archive -LiteralPath $PackagePath -DestinationPath $TestRoot -Force
  $PackageDir = Get-ChildItem -LiteralPath $TestRoot -Directory | Select-Object -First 1
  if ($null -eq $PackageDir) {
    throw "package did not expand to a directory: $PackagePath"
  }

  $Studio = Join-Path $PackageDir.FullName 'HVAC Studio.exe'
  $StudioServer = Join-Path $PackageDir.FullName 'bin\studio.exe'
  $Runner = Join-Path $PackageDir.FullName 'bin\bcs-runner.exe'
  $EnvTool = Join-Path $PackageDir.FullName 'bin\bcs-env.exe'
  $PackagedPython = Join-Path $PackageDir.FullName 'runtime\python\python.exe'
  foreach ($RequiredPath in @($Studio, $StudioServer, $Runner, $EnvTool, $PackagedPython)) {
    if (-not (Test-Path -LiteralPath $RequiredPath)) {
      throw "portable package is missing $RequiredPath"
    }
  }
  Assert-ReleaseProvenance -PackageRoot $PackageDir.FullName -PackageType 'studio-portable' -Version $Version

  $env:PATH = Get-MinimalPackagePath -PackageRoot $PackageDir.FullName

  $DesktopProcess = Start-Process -FilePath $Studio -PassThru -ArgumentList @(
    '--repo',
    $PackageDir.FullName
  )
  $DesktopReady = $false
  $DesktopTitle = ''
  for ($Index = 0; $Index -lt 20; $Index++) {
    Start-Sleep -Milliseconds 500
    if ($DesktopProcess.HasExited) {
      throw "Studio desktop app exited during launch smoke"
    }
    $DesktopTitle = (Get-Process -Id $DesktopProcess.Id -ErrorAction Stop).MainWindowTitle
    if ($DesktopTitle -eq 'Error') {
      throw "Studio desktop app opened an error dialog instead of the Wails window"
    }
    if ($DesktopTitle -match 'HVAC Studio') {
      $DesktopReady = $true
      break
    }
  }
  if (-not $DesktopReady) {
    throw "Studio desktop app did not open the expected Wails window; title='$DesktopTitle'"
  }
  Stop-Process -Id $DesktopProcess.Id -Force -ErrorAction SilentlyContinue
  $DesktopProcess = $null

  $Project = Join-Path $PackageDir.FullName 'examples\003_feedforward_system\project.bcsproj'
  $Input = Join-Path $PackageDir.FullName 'examples\003_feedforward_system\inputs\case01.json'
  $Expected = Join-Path $PackageDir.FullName 'examples\003_feedforward_system\expected\output.json'
  $Output = Join-Path $PackageDir.FullName 'outputs\003_feedforward_system.json'

  Invoke-Checked $PackagedPython @('--version')
  $EnvStatusRaw = & $EnvTool check --root $PackageDir.FullName --json
  if ($LASTEXITCODE -ne 0) {
    throw "bcs-env check failed: $EnvStatusRaw"
  }
  $EnvStatus = $EnvStatusRaw | ConvertFrom-Json
  if (-not $EnvStatus.ok) {
    throw "bcs-env reported package problems: $($EnvStatus.problems -join '; ')"
  }
  if ($EnvStatus.mode -ne 'portable-studio') {
    throw "bcs-env mode mismatch: $($EnvStatus.mode)"
  }
  Invoke-Checked $Runner @('validate', '--project', $Project)
  Invoke-Checked $Runner @('run', '--project', $Project, '--input', $Input, '--output', $Output)

  $ExpectedJson = Get-Content -Raw -LiteralPath $Expected | ConvertFrom-Json
  $ActualJson = Get-Content -Raw -LiteralPath $Output | ConvertFrom-Json
  Assert-JsonSubset -Expected $ExpectedJson -Actual $ActualJson -Path '$'

  $Port = Get-FreePort
  $OutLog = Join-Path $TestRoot 'studio.out.log'
  $ErrLog = Join-Path $TestRoot 'studio.err.log'
  $StudioProcess = Start-Process -FilePath $StudioServer -WindowStyle Hidden -PassThru -RedirectStandardOutput $OutLog -RedirectStandardError $ErrLog -ArgumentList @(
    '--repo',
    $PackageDir.FullName,
    '--server',
    '--addr',
    "127.0.0.1:$Port"
  )

  $ProjectsUrl = "http://127.0.0.1:$Port/api/projects"
  $Ready = $false
  for ($Index = 0; $Index -lt 40; $Index++) {
    try {
      $Response = Invoke-WebRequest -UseBasicParsing -Uri $ProjectsUrl -TimeoutSec 2
      if ($Response.StatusCode -eq 200) {
        $Ready = $true
        break
      }
    } catch {
      Start-Sleep -Milliseconds 500
    }
  }
  if (-not $Ready) {
    throw "Studio did not respond at $ProjectsUrl"
  }

  $RunBody = @{
    project_path = 'examples/003_feedforward_system/project.bcsproj'
    inputs = @{
      building_load_kw = 500
      base_chw_setpoint_c = 7
    }
    context = @{
      time = 0
      dt = 60
    }
  } | ConvertTo-Json -Depth 8

  $RunResponse = Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:$Port/api/run" -Method POST -ContentType 'application/json' -Body $RunBody -TimeoutSec 20
  $RunJson = $RunResponse.Content | ConvertFrom-Json
  if ($RunJson.result.outputs.total_power_kw -ne 122) {
    throw "Studio API smoke result mismatch: total_power_kw=$($RunJson.result.outputs.total_power_kw)"
  }

  $CopyBody = @{ project_path = $Project; name = 'Editable Feedforward Copy' } | ConvertTo-Json -Depth 4
  $CopyResponse = Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:$Port/api/projects/copy" -Method POST -ContentType 'application/json' -Body $CopyBody -TimeoutSec 20
  $CopiedProject = ($CopyResponse.Content | ConvertFrom-Json).project
  if ($CopiedProject.source -ne 'workspace') {
    throw "copied project should be a workspace project: source=$($CopiedProject.source)"
  }
  if (-not (Test-Path -LiteralPath $CopiedProject.project_path)) {
    throw "copied project file was not written: $($CopiedProject.project_path)"
  }
  $CopiedSourceResponse = Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:$Port/api/project/source?project_path=$([uri]::EscapeDataString($CopiedProject.project_path))&component_id=load_model" -TimeoutSec 20
  $CopiedSourceJson = $CopiedSourceResponse.Content | ConvertFrom-Json
  if ($CopiedSourceJson.source.read_only) {
    throw "copied example source should be editable"
  }

  $CreateBody = @{ name = 'Portable Smoke Project'; template = 'scalar' } | ConvertTo-Json -Depth 4
  $CreateResponse = Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:$Port/api/projects" -Method POST -ContentType 'application/json' -Body $CreateBody -TimeoutSec 20
  $CreatedProject = ($CreateResponse.Content | ConvertFrom-Json).project
  if (-not (Test-Path -LiteralPath $CreatedProject.project_path)) {
    throw "created project file was not written: $($CreatedProject.project_path)"
  }

  $RenameBody = @{
    project_path = $CreatedProject.project_path
    component_id = 'scalar'
    name = 'Portable Scalar Driver'
  } | ConvertTo-Json -Depth 4
  $RenameResponse = Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:$Port/api/project/components/update" -Method POST -ContentType 'application/json' -Body $RenameBody -TimeoutSec 20
  $RenameJson = $RenameResponse.Content | ConvertFrom-Json
  if (($RenameJson.project.graph.components | Where-Object { $_.id -eq 'scalar' }).name -ne 'Portable Scalar Driver') {
    throw "workspace component rename did not persist"
  }

  $NodeBody = @{
    project_path = $CreatedProject.project_path
    component_id = 'scalar'
    direction = 'input'
    id = 'bias'
    value_type = 'float'
    default = 4
  } | ConvertTo-Json -Depth 4
  $NodeResponse = Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:$Port/api/project/nodes" -Method POST -ContentType 'application/json' -Body $NodeBody -TimeoutSec 20
  $NodeJson = $NodeResponse.Content | ConvertFrom-Json
  if (-not ($NodeJson.project.graph.systems[0].public_inputs | Where-Object { $_.id -eq 'scalar_bias' })) {
    throw "created input node was not exposed as public input"
  }
  if ($NodeJson.project.default_run_input.inputs.scalar_bias -ne 4) {
    throw "created input node default mismatch: scalar_bias=$($NodeJson.project.default_run_input.inputs.scalar_bias)"
  }

  $SourceResponse = Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:$Port/api/project/source?project_path=$([uri]::EscapeDataString($CreatedProject.project_path))&component_id=scalar" -TimeoutSec 20
  $SourceJson = $SourceResponse.Content | ConvertFrom-Json
  if ($SourceJson.source.read_only) {
    throw "workspace component source should be editable"
  }
  $EditedSource = $SourceJson.source.content -replace 'return \{"result": value \* gain\}, state', "bias = float(inputs.get(`"bias`", 0.0))`n        offset = float(params.get(`"offset`", 0.0))`n        print(`"portable smoke scalar log`")`n        return {`"result`": value * gain + offset + bias}, state"
  if ($EditedSource -eq $SourceJson.source.content) {
    throw "workspace source edit pattern was not found"
  }
  $SourceBody = @{
    project_path = $CreatedProject.project_path
    component_id = 'scalar'
    content = "$EditedSource`n# portable source edit smoke`n"
  } | ConvertTo-Json -Depth 4
  $SourceUpdateResponse = Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:$Port/api/project/source" -Method POST -ContentType 'application/json' -Body $SourceBody -TimeoutSec 20
  $SourceUpdateJson = $SourceUpdateResponse.Content | ConvertFrom-Json
  if ($SourceUpdateJson.source.content -notmatch 'portable source edit smoke') {
    throw "workspace source update did not round-trip"
  }
  $SourceCheckBody = @{
    project_path = $CreatedProject.project_path
    component_id = 'scalar'
    content = $SourceUpdateJson.source.content
  } | ConvertTo-Json -Depth 4
  $SourceCheckResponse = Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:$Port/api/project/source/check" -Method POST -ContentType 'application/json' -Body $SourceCheckBody -TimeoutSec 20
  $SourceCheckJson = $SourceCheckResponse.Content | ConvertFrom-Json
  if (-not $SourceCheckJson.check.ok) {
    throw "workspace source check failed: $($SourceCheckJson.check.problems | ConvertTo-Json -Compress)"
  }

  $DuplicateBody = @{
    project_path = $CreatedProject.project_path
    source_component_id = 'scalar'
    name = 'Portable Scalar Duplicate'
  } | ConvertTo-Json -Depth 4
  $DuplicateResponse = Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:$Port/api/project/components/duplicate" -Method POST -ContentType 'application/json' -Body $DuplicateBody -TimeoutSec 20
  $DuplicateJson = $DuplicateResponse.Content | ConvertFrom-Json
  if ($DuplicateJson.component.id -ne 'portable_scalar_duplicate') {
    throw "duplicated component id mismatch: $($DuplicateJson.component.id)"
  }
  Assert-ComponentSourceWritten -ProjectPath $CreatedProject.project_path -Component $DuplicateJson.component -Label 'duplicated component'
  if ($DuplicateJson.project.graph.systems[0].components | Where-Object { $_ -eq $DuplicateJson.component.id }) {
    throw "duplicated component should not be added to the entry system"
  }

  $ComponentBody = @{ project_path = $CreatedProject.project_path; name = 'Portable Extra Component'; template = 'scalar' } | ConvertTo-Json -Depth 4
  $ComponentResponse = Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:$Port/api/project/components" -Method POST -ContentType 'application/json' -Body $ComponentBody -TimeoutSec 20
  $CreatedComponent = ($ComponentResponse.Content | ConvertFrom-Json).component
  Assert-ComponentSourceWritten -ProjectPath $CreatedProject.project_path -Component $CreatedComponent -Label 'created component'
  $IncludeBody = @{ project_path = $CreatedProject.project_path; component_id = $CreatedComponent.id } | ConvertTo-Json -Depth 4
  $IncludeResponse = Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:$Port/api/project/system/components" -Method POST -ContentType 'application/json' -Body $IncludeBody -TimeoutSec 20
  $IncludeJson = $IncludeResponse.Content | ConvertFrom-Json
  $ExtraInputId = "$($CreatedComponent.id)_value"
  if (-not ($IncludeJson.project.graph.systems[0].public_inputs | Where-Object { $_.id -eq $ExtraInputId })) {
    throw "included component public input was not created: $ExtraInputId"
  }
  $ConnectionBody = @{
    project_path = $CreatedProject.project_path
    from_component = 'scalar'
    from_node = 'result'
    to_component = $CreatedComponent.id
    to_node = 'value'
  } | ConvertTo-Json -Depth 8
  $ConnectionResponse = Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:$Port/api/project/connections" -Method POST -ContentType 'application/json' -Body $ConnectionBody -TimeoutSec 20
  $ConnectionJson = $ConnectionResponse.Content | ConvertFrom-Json
  if (-not ($ConnectionJson.project.graph.systems[0].connections | Where-Object { $_ -eq $ConnectionJson.connection.id })) {
    throw "created connection was not added to the entry system"
  }
  if ($ConnectionJson.project.graph.systems[0].public_inputs | Where-Object { $_.id -eq $ExtraInputId }) {
    throw "connected target input should no longer be public: $ExtraInputId"
  }

  $ParameterBody = @{
    project_path = $CreatedProject.project_path
    parameters = @{
      scalar = @{
        gain = 3
        offset = 2
        smoke_temp = 7
      }
    }
  } | ConvertTo-Json -Depth 8
  $ParameterResponse = Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:$Port/api/project/parameters" -Method POST -ContentType 'application/json' -Body $ParameterBody -TimeoutSec 20
  $ParameterJson = $ParameterResponse.Content | ConvertFrom-Json
  if ($ParameterJson.project.graph.components[0].parameters.gain -ne 3) {
    throw "workspace parameter update mismatch: gain=$($ParameterJson.project.graph.components[0].parameters.gain)"
  }
  if ($ParameterJson.project.graph.components[0].parameters.offset -ne 2) {
    throw "workspace parameter add mismatch: offset=$($ParameterJson.project.graph.components[0].parameters.offset)"
  }
  if ($ParameterJson.project.graph.components[0].parameters.smoke_temp -ne 7) {
    throw "workspace parameter add mismatch: smoke_temp=$($ParameterJson.project.graph.components[0].parameters.smoke_temp)"
  }
  $DeleteParameterBody = @{
    project_path = $CreatedProject.project_path
    component_id = 'scalar'
    name = 'smoke_temp'
  } | ConvertTo-Json -Depth 4
  $DeleteParameterResponse = Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:$Port/api/project/parameters/delete" -Method POST -ContentType 'application/json' -Body $DeleteParameterBody -TimeoutSec 20
  $DeleteParameterJson = $DeleteParameterResponse.Content | ConvertFrom-Json
  if ($null -ne $DeleteParameterJson.project.graph.components[0].parameters.PSObject.Properties['smoke_temp']) {
    throw "deleted parameter should be removed from the component graph"
  }
  if ($DeleteParameterJson.project.graph.components[0].parameters.gain -ne 3) {
    throw "parameter delete should keep remaining parameters"
  }
  if ($DeleteParameterJson.project.graph.components[0].parameters.offset -ne 2) {
    throw "parameter delete should keep runtime-significant parameters"
  }

  $InputValues = @{ value = 5; scalar_bias = 4 }
  $InputBody = @{
    project_path = $CreatedProject.project_path
    inputs = $InputValues
    context = @{
      time = 0
      dt = 60
    }
  } | ConvertTo-Json -Depth 8
  $InputResponse = Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:$Port/api/project/input" -Method POST -ContentType 'application/json' -Body $InputBody -TimeoutSec 20
  $InputJson = $InputResponse.Content | ConvertFrom-Json
  if ($InputJson.project.default_run_input.inputs.value -ne 5) {
    throw "workspace input update mismatch: value=$($InputJson.project.default_run_input.inputs.value)"
  }
  if ($InputJson.project.default_run_input.inputs.scalar_bias -ne 4) {
    throw "workspace input update mismatch: scalar_bias=$($InputJson.project.default_run_input.inputs.scalar_bias)"
  }
  $SeriesInputPath = Join-Path (Split-Path -Parent $CreatedProject.project_path) 'inputs\series01.json'
  $SeriesInputJson = @{
    schema_version = '0.1.0'
    context = @{
      dt = 60
    }
    steps = @(
      @{
        id = 'step-1'
        inputs = $InputValues
        context = @{
          time = 0
        }
      },
      @{
        id = 'step-2'
        inputs = @{
          value = 6
          scalar_bias = 4
        }
        context = @{
          time = 60
        }
      }
    )
  } | ConvertTo-Json -Depth 8
  [IO.File]::WriteAllText($SeriesInputPath, $SeriesInputJson + [Environment]::NewLine, [Text.UTF8Encoding]::new($false))
  if (-not (Test-Path -LiteralPath $SeriesInputPath)) {
    throw "workspace series input was not written: $SeriesInputPath"
  }
  $ProjectRoot = Split-Path -Parent $CreatedProject.project_path
  $DatasetPath = Join-Path $ProjectRoot 'datasets\portable_smoke_validation.csv'
  $ValidationMappingPath = Join-Path $ProjectRoot 'validation\mappings\portable_smoke_validation.json'
  $CalibrationSetupPath = Join-Path $ProjectRoot 'calibration\setups\portable_smoke_gain.json'
  $OptimizationSetupPath = Join-Path $ProjectRoot 'optimization\setups\portable_smoke_value_grid.json'
  New-Item -ItemType Directory -Force -Path (Split-Path -Parent $DatasetPath) | Out-Null
  New-Item -ItemType Directory -Force -Path (Split-Path -Parent $ValidationMappingPath) | Out-Null
  New-Item -ItemType Directory -Force -Path (Split-Path -Parent $CalibrationSetupPath) | Out-Null
  New-Item -ItemType Directory -Force -Path (Split-Path -Parent $OptimizationSetupPath) | Out-Null
  [IO.File]::WriteAllText($DatasetPath, "value,scalar_bias,observed_result,observed_extra_result`n5,4,21,42`n6,4,24,48`n", [Text.UTF8Encoding]::new($false))
  $ValidationMappingJson = @{
    id = 'portable_smoke_validation'
    dataset = 'datasets/portable_smoke_validation.csv'
    input_columns = @{
      value = 'value'
      scalar_bias = 'scalar_bias'
    }
    observed_output_columns = @{
      result = 'observed_result'
    }
  } | ConvertTo-Json -Depth 8
  [IO.File]::WriteAllText($ValidationMappingPath, $ValidationMappingJson + [Environment]::NewLine, [Text.UTF8Encoding]::new($false))
  $CalibrationSetupJson = @{
    id = 'portable_smoke_gain'
    algorithm = 'grid'
    mapping = 'validation/mappings/portable_smoke_validation.json'
    objective = @{
      metric = 'rmse'
      outputs = @{
        result = 1.0
      }
    }
    parameters = @(
      @{
        component = 'scalar'
        name = 'gain'
        min = 2.0
        max = 4.0
        step = 1.0
      }
    )
  } | ConvertTo-Json -Depth 8
  [IO.File]::WriteAllText($CalibrationSetupPath, $CalibrationSetupJson + [Environment]::NewLine, [Text.UTF8Encoding]::new($false))
  $OptimizationSetupJson = @{
    id = 'portable_smoke_value_grid'
    algorithm = 'grid'
    base_inputs = $InputValues
    context = @{
      time = 0
      dt = 60
    }
    objective = @{
      output = 'result'
      sense = 'max'
    }
    decision_variables = @(
      @{
        kind = 'public_input'
        name = 'value'
        min = 4.0
        max = 6.0
        step = 1.0
      }
    )
  } | ConvertTo-Json -Depth 8
  [IO.File]::WriteAllText($OptimizationSetupPath, $OptimizationSetupJson + [Environment]::NewLine, [Text.UTF8Encoding]::new($false))

  $ValidateBody = @{
    project_path = $CreatedProject.project_path
  } | ConvertTo-Json -Depth 4
  $ValidateResponse = Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:$Port/api/validate" -Method POST -ContentType 'application/json' -Body $ValidateBody -TimeoutSec 20
  $ValidateJson = $ValidateResponse.Content | ConvertFrom-Json
  if ($ValidateJson.validation.source_checks -lt 2) {
    throw "workspace validation source check count too small: $($ValidateJson.validation.source_checks)"
  }
  $ValidateErrors = @($ValidateJson.validation.problems | Where-Object { $_.severity -eq 'error' })
  if ($ValidateErrors.Count -gt 0) {
    throw "workspace validation reported source errors: $($ValidateErrors | ConvertTo-Json -Compress)"
  }

  $ScenarioBody = @{
    project_path = $CreatedProject.project_path
    name = 'Portable Scenario'
    inputs = $InputValues
    context = @{
      time = 0
      dt = 60
    }
  } | ConvertTo-Json -Depth 8
  $ScenarioResponse = Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:$Port/api/project/scenarios" -Method POST -ContentType 'application/json' -Body $ScenarioBody -TimeoutSec 20
  $ScenarioJson = $ScenarioResponse.Content | ConvertFrom-Json
  $ScenarioPath = Join-Path (Split-Path -Parent $CreatedProject.project_path) $ScenarioJson.summary.relative_path
  if (-not (Test-Path -LiteralPath $ScenarioPath)) {
    throw "workspace scenario was not written: $ScenarioPath"
  }
  $ScenarioReadResponse = Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:$Port/api/project/scenario?project_path=$([uri]::EscapeDataString($CreatedProject.project_path))&scenario_id=$([uri]::EscapeDataString($ScenarioJson.summary.id))" -TimeoutSec 20
  $ScenarioReadJson = $ScenarioReadResponse.Content | ConvertFrom-Json
  if ($ScenarioReadJson.scenario.inputs.value -ne 5) {
    throw "workspace scenario read mismatch: value=$($ScenarioReadJson.scenario.inputs.value)"
  }
  if ($ScenarioReadJson.scenario.inputs.scalar_bias -ne 4) {
    throw "workspace scenario read mismatch: scalar_bias=$($ScenarioReadJson.scenario.inputs.scalar_bias)"
  }

  $BatchBody = @{
    project_path = $CreatedProject.project_path
  } | ConvertTo-Json -Depth 4
  $BatchResponse = Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:$Port/api/batch" -Method POST -ContentType 'application/json' -Body $BatchBody -TimeoutSec 20
  $BatchJson = $BatchResponse.Content | ConvertFrom-Json
  if ($BatchJson.summary.case_count -ne 1 -or $BatchJson.summary.ok_count -ne 1) {
    throw "workspace batch counts mismatch: ok=$($BatchJson.summary.ok_count) cases=$($BatchJson.summary.case_count)"
  }
  if ($BatchJson.batch.cases[0].result.outputs.result -ne 21) {
    throw "workspace batch result mismatch: result=$($BatchJson.batch.cases[0].result.outputs.result)"
  }
  $BatchPath = Join-Path (Split-Path -Parent $CreatedProject.project_path) $BatchJson.summary.relative_path
  if (-not (Test-Path -LiteralPath $BatchPath)) {
    throw "workspace batch record was not written: $BatchPath"
  }
  $BatchReadResponse = Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:$Port/api/project/batch?project_path=$([uri]::EscapeDataString($CreatedProject.project_path))&batch_id=$([uri]::EscapeDataString($BatchJson.summary.id))" -TimeoutSec 20
  $BatchReadJson = $BatchReadResponse.Content | ConvertFrom-Json
  if ($BatchReadJson.batch_record.cases[0].result.outputs.result -ne 21) {
    throw "workspace batch record detail mismatch: result=$($BatchReadJson.batch_record.cases[0].result.outputs.result)"
  }

  $WorkspaceRunBody = @{
    project_path = $CreatedProject.project_path
    save = $true
  } | ConvertTo-Json -Depth 8
  $WorkspaceRunResponse = Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:$Port/api/run" -Method POST -ContentType 'application/json' -Body $WorkspaceRunBody -TimeoutSec 20
  $WorkspaceRunJson = $WorkspaceRunResponse.Content | ConvertFrom-Json
  if ($WorkspaceRunJson.result.outputs.result -ne 21) {
    throw "workspace run result mismatch: result=$($WorkspaceRunJson.result.outputs.result)"
  }
  $ExtraOutputId = "$($CreatedComponent.id)_result"
  $ExtraOutput = $WorkspaceRunJson.result.outputs.PSObject.Properties[$ExtraOutputId].Value
  if ($ExtraOutput -ne 42) {
    throw "workspace included component result mismatch: $ExtraOutputId=$ExtraOutput"
  }
  $RunRecordPath = Join-Path (Split-Path -Parent $CreatedProject.project_path) $WorkspaceRunJson.run_record.relative_path
  if (-not (Test-Path -LiteralPath $RunRecordPath)) {
    throw "workspace run record was not written: $RunRecordPath"
  }
  $RunRecordResponse = Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:$Port/api/project/run?project_path=$([uri]::EscapeDataString($CreatedProject.project_path))&run_id=$([uri]::EscapeDataString($WorkspaceRunJson.run_record.id))" -TimeoutSec 20
  $RunRecordJson = $RunRecordResponse.Content | ConvertFrom-Json
  if ($RunRecordJson.run_record.result.outputs.result -ne 21) {
    throw "workspace run record detail mismatch: result=$($RunRecordJson.run_record.result.outputs.result)"
  }

  $ExportBody = @{ project_path = $CreatedProject.project_path; profile = 'runtime_package' } | ConvertTo-Json -Depth 4
  $ExportResponse = Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:$Port/api/export" -Method POST -ContentType 'application/json' -Body $ExportBody -TimeoutSec 20
  $ExportJson = $ExportResponse.Content | ConvertFrom-Json
  $ExportManifestPath = Join-Path (Split-Path -Parent $CreatedProject.project_path) $ExportJson.summary.relative_path
  if (-not (Test-Path -LiteralPath $ExportManifestPath)) {
    throw "workspace export manifest was not written: $ExportManifestPath"
  }
  if ($ExportJson.export.runner -ne 'bin/bcs-runner.exe') {
    throw "workspace export manifest runner mismatch: $($ExportJson.export.runner)"
  }
  if ($ExportJson.export.project_root -ne 'project') {
    throw "workspace export project root mismatch: $($ExportJson.export.project_root)"
  }
  if ($ExportJson.export.project_path -ne 'project/project.bcsproj') {
    throw "workspace export project path mismatch: $($ExportJson.export.project_path)"
  }
  if ($ExportJson.export.interface_schema -ne 'schema/public-io.json') {
    throw "workspace export interface schema mismatch: $($ExportJson.export.interface_schema)"
  }
  foreach ($ExportFile in @('README.md', 'bin/bcs-runner.exe', 'bin/bcs-env.exe', 'project/project.bcsproj', 'project/graph.json', 'project/components/scalar.py', 'project/inputs/case01.json', 'project/inputs/series01.json', 'project/datasets/portable_smoke_validation.csv', 'project/validation/mappings/portable_smoke_validation.json', 'project/calibration/setups/portable_smoke_gain.json', 'project/optimization/setups/portable_smoke_value_grid.json', 'check-env.ps1', 'docs/CLI_Guide.md', 'run-default.ps1', 'run-scenario.ps1', 'run-series.ps1', 'validate-data.ps1', 'calibrate.ps1', 'optimize.ps1', 'optimize-sdk.py', 'sdk-example.py', 'serve.ps1', 'runtime/python/python.exe', 'schema/public-io.json')) {
    if ($ExportJson.export.files -notcontains $ExportFile) {
      throw "workspace export file missing from manifest: $ExportFile"
    }
    $ExportArtifactPath = Join-Path (Split-Path -Parent $ExportManifestPath) ($ExportFile -replace '/', [IO.Path]::DirectorySeparatorChar)
    if (-not (Test-Path -LiteralPath $ExportArtifactPath)) {
      throw "workspace export artifact was not written: $ExportArtifactPath"
    }
  }
  $ExportSchemaPath = Join-Path (Split-Path -Parent $ExportManifestPath) 'schema\public-io.json'
  $ExportSchemaJson = Get-Content -Raw -LiteralPath $ExportSchemaPath | ConvertFrom-Json
  if (@($ExportSchemaJson.inputs).Count -lt 1 -or @($ExportSchemaJson.outputs).Count -lt 1) {
    throw "workspace export schema missing public inputs or outputs"
  }
  $ExportRunnerPath = Join-Path (Split-Path -Parent $ExportManifestPath) 'bin\bcs-runner.exe'
  $ExportEnvToolPath = Join-Path (Split-Path -Parent $ExportManifestPath) 'bin\bcs-env.exe'
  $ExportEnvStatusRaw = & $ExportEnvToolPath check --root (Split-Path -Parent $ExportManifestPath) --json
  $ExportEnvStatus = $ExportEnvStatusRaw | ConvertFrom-Json
  if (-not $ExportEnvStatus.ok) {
    throw "exported runtime env check failed: $($ExportEnvStatus.problems -join '; ')"
  }
  if ($ExportEnvStatus.mode -ne 'runtime-export') {
    throw "exported runtime env mode mismatch: $($ExportEnvStatus.mode)"
  }
  $ExportProjectPath = Join-Path (Split-Path -Parent $ExportManifestPath) 'project\project.bcsproj'
  $ExportInputPath = Join-Path (Split-Path -Parent $ExportManifestPath) 'project\inputs\case01.json'
  $ExportOutputPath = Join-Path $TestRoot 'exported-runtime-output.json'
  Invoke-Checked $ExportRunnerPath @('validate', '--project', $ExportProjectPath)
  Invoke-Checked $ExportRunnerPath @('run', '--project', $ExportProjectPath, '--input', $ExportInputPath, '--output', $ExportOutputPath)
  $ExportRunJson = Get-Content -Raw -LiteralPath $ExportOutputPath | ConvertFrom-Json
  if ($ExportRunJson.outputs.result -ne 21) {
    throw "exported runtime run result mismatch: result=$($ExportRunJson.outputs.result)"
  }
  $ExportExtraOutput = $ExportRunJson.outputs.PSObject.Properties[$ExtraOutputId].Value
  if ($ExportExtraOutput -ne 42) {
    throw "exported runtime included component result mismatch: $ExtraOutputId=$ExportExtraOutput"
  }
  $ExportRunScript = Join-Path (Split-Path -Parent $ExportManifestPath) 'run-default.ps1'
  $ExportScriptOutputPath = Join-Path $TestRoot 'exported-runtime-script-output.json'
  $PowerShellExe = Join-Path $PSHOME 'powershell.exe'
  Invoke-Checked $PowerShellExe @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', $ExportRunScript, '-Output', $ExportScriptOutputPath)
  $ExportScriptRunJson = Get-Content -Raw -LiteralPath $ExportScriptOutputPath | ConvertFrom-Json
  if ($ExportScriptRunJson.outputs.result -ne 21) {
    throw "exported runtime script result mismatch: result=$($ExportScriptRunJson.outputs.result)"
  }
  $ExportScenarioScript = Join-Path (Split-Path -Parent $ExportManifestPath) 'run-scenario.ps1'
  $ExportScenarioOutputPath = Join-Path $TestRoot 'exported-runtime-scenario-output.json'
  Invoke-Checked $PowerShellExe @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', $ExportScenarioScript, '-InputFile', 'project\inputs\case01.json', '-Output', $ExportScenarioOutputPath)
  $ExportScenarioJson = Get-Content -Raw -LiteralPath $ExportScenarioOutputPath | ConvertFrom-Json
  if ($ExportScenarioJson.outputs.result -ne 21) {
    throw "exported runtime scenario script result mismatch: result=$($ExportScenarioJson.outputs.result)"
  }
  $ExportSeriesScript = Join-Path (Split-Path -Parent $ExportManifestPath) 'run-series.ps1'
  $ExportSeriesOutputPath = Join-Path $TestRoot 'exported-runtime-series-output.json'
  Invoke-Checked $PowerShellExe @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', $ExportSeriesScript, '-Output', $ExportSeriesOutputPath)
  $ExportSeriesJson = Get-Content -Raw -LiteralPath $ExportSeriesOutputPath | ConvertFrom-Json
  if ($ExportSeriesJson.step_count -ne 2) {
    throw "exported runtime series step count mismatch: steps=$($ExportSeriesJson.step_count)"
  }
  if (@($ExportSeriesJson.outputs.result)[0] -ne 21 -or @($ExportSeriesJson.outputs.result)[1] -ne 24) {
    throw "exported runtime series result mismatch: result=$($ExportSeriesJson.outputs.result -join ',')"
  }
  if (@($ExportSeriesJson.outputs.PSObject.Properties[$ExtraOutputId].Value)[0] -ne 42 -or @($ExportSeriesJson.outputs.PSObject.Properties[$ExtraOutputId].Value)[1] -ne 48) {
    throw "exported runtime series included component mismatch: $ExtraOutputId=$($ExportSeriesJson.outputs.PSObject.Properties[$ExtraOutputId].Value -join ',')"
  }
  $ExportValidationScript = Join-Path (Split-Path -Parent $ExportManifestPath) 'validate-data.ps1'
  $ExportValidationOutputPath = Join-Path $TestRoot 'exported-runtime-validation-output.json'
  Invoke-Checked $PowerShellExe @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', $ExportValidationScript, '-Output', $ExportValidationOutputPath)
  $ExportValidationJson = Get-Content -Raw -LiteralPath $ExportValidationOutputPath | ConvertFrom-Json
  if (-not $ExportValidationJson.ok -or $ExportValidationJson.row_count -ne 2 -or $ExportValidationJson.metrics.result.rmse -ne 0) {
    throw "exported runtime validation script mismatch: ok=$($ExportValidationJson.ok) rows=$($ExportValidationJson.row_count) rmse=$($ExportValidationJson.metrics.result.rmse)"
  }
  $ExportCalibrationScript = Join-Path (Split-Path -Parent $ExportManifestPath) 'calibrate.ps1'
  $ExportCalibrationOutputPath = Join-Path $TestRoot 'exported-runtime-calibration-output.json'
  Invoke-Checked $PowerShellExe @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', $ExportCalibrationScript, '-Output', $ExportCalibrationOutputPath)
  $ExportCalibrationJson = Get-Content -Raw -LiteralPath $ExportCalibrationOutputPath | ConvertFrom-Json
  if (-not $ExportCalibrationJson.ok -or @($ExportCalibrationJson.candidates).Count -ne 3 -or $ExportCalibrationJson.best_parameter_set.components.scalar.gain -ne 3) {
    throw "exported runtime calibration script mismatch: ok=$($ExportCalibrationJson.ok) candidates=$(@($ExportCalibrationJson.candidates).Count) gain=$($ExportCalibrationJson.best_parameter_set.components.scalar.gain)"
  }
  $ExportOptimizationScript = Join-Path (Split-Path -Parent $ExportManifestPath) 'optimize.ps1'
  $ExportOptimizationOutputPath = Join-Path $TestRoot 'exported-runtime-optimization-output.json'
  Invoke-Checked $PowerShellExe @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', $ExportOptimizationScript, '-Output', $ExportOptimizationOutputPath)
  $ExportOptimizationJson = Get-Content -Raw -LiteralPath $ExportOptimizationOutputPath | ConvertFrom-Json
  if (-not $ExportOptimizationJson.ok -or @($ExportOptimizationJson.candidates).Count -ne 3 -or $ExportOptimizationJson.best_inputs.value -ne 6) {
    throw "exported runtime optimization script mismatch: ok=$($ExportOptimizationJson.ok) candidates=$(@($ExportOptimizationJson.candidates).Count) value=$($ExportOptimizationJson.best_inputs.value)"
  }
  $ExportScriptLogBundlePath = Join-Path (Split-Path -Parent $ExportManifestPath) 'outputs\logs\exported-runtime-script-output-logs.json'
  if (-not (Test-Path -LiteralPath $ExportScriptLogBundlePath)) {
    throw "exported runtime script log bundle was not written: $ExportScriptLogBundlePath"
  }
  $ExportScriptLogBundle = Get-Content -Raw -LiteralPath $ExportScriptLogBundlePath | ConvertFrom-Json
  if ($ExportScriptLogBundle.schema -ne 'hvac-studio.runtime-log-bundle.v1') {
    throw "exported runtime script log bundle schema mismatch: $($ExportScriptLogBundle.schema)"
  }
  if (-not (@($ExportScriptLogBundle.component_logs) | Where-Object { $_.message -eq 'portable smoke scalar log' })) {
    throw "exported runtime script log bundle missing component stdout log"
  }

  $DeleteConnectionBody = @{
    project_path = $CreatedProject.project_path
    connection_id = $ConnectionJson.connection.id
  } | ConvertTo-Json -Depth 4
  $DeleteConnectionResponse = Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:$Port/api/project/connections/delete" -Method POST -ContentType 'application/json' -Body $DeleteConnectionBody -TimeoutSec 20
  $DeleteConnectionJson = $DeleteConnectionResponse.Content | ConvertFrom-Json
  if ($DeleteConnectionJson.project.graph.systems[0].connections | Where-Object { $_ -eq $ConnectionJson.connection.id }) {
    throw "deleted connection was still referenced by the entry system"
  }
  if (-not ($DeleteConnectionJson.project.graph.systems[0].public_inputs | Where-Object { $_.id -eq $ExtraInputId })) {
    throw "deleted connection target input was not restored as public: $ExtraInputId"
  }
  if ($null -eq $DeleteConnectionJson.project.default_run_input.inputs.PSObject.Properties[$ExtraInputId]) {
    throw "deleted connection target default input was not restored: $ExtraInputId"
  }

  $RemoveComponentBody = @{
    project_path = $CreatedProject.project_path
    component_id = $CreatedComponent.id
  } | ConvertTo-Json -Depth 4
  $RemoveComponentResponse = Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:$Port/api/project/system/components/remove" -Method POST -ContentType 'application/json' -Body $RemoveComponentBody -TimeoutSec 20
  $RemoveComponentJson = $RemoveComponentResponse.Content | ConvertFrom-Json
  if ($RemoveComponentJson.project.graph.systems[0].components | Where-Object { $_ -eq $CreatedComponent.id }) {
    throw "removed component was still in the entry system"
  }
  if (-not ($RemoveComponentJson.project.graph.components | Where-Object { $_.id -eq $CreatedComponent.id })) {
    throw "removed system component artifact should remain in the graph"
  }
  if ($RemoveComponentJson.project.graph.systems[0].public_inputs | Where-Object { $_.id -eq $ExtraInputId }) {
    throw "removed component public input should be removed: $ExtraInputId"
  }
  if ($null -ne $RemoveComponentJson.project.default_run_input.inputs.PSObject.Properties[$ExtraInputId]) {
    throw "removed component default input should be removed: $ExtraInputId"
  }

  $DeleteComponentBody = @{
    project_path = $CreatedProject.project_path
    component_id = $CreatedComponent.id
  } | ConvertTo-Json -Depth 4
  $DeleteComponentResponse = Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:$Port/api/project/components/delete" -Method POST -ContentType 'application/json' -Body $DeleteComponentBody -TimeoutSec 20
  $DeleteComponentJson = $DeleteComponentResponse.Content | ConvertFrom-Json
  if ($DeleteComponentJson.project.graph.components | Where-Object { $_.id -eq $CreatedComponent.id }) {
    throw "deleted component should be removed from the graph"
  }
  foreach ($SourcePath in (Get-ComponentSourcePaths -ProjectPath $CreatedProject.project_path -Component $CreatedComponent)) {
    if (Test-Path -LiteralPath $SourcePath) {
      throw "deleted component source should be removed: $SourcePath"
    }
  }

  $DeleteNodeBody = @{
    project_path = $CreatedProject.project_path
    component_id = 'scalar'
    node_id = 'bias'
  } | ConvertTo-Json -Depth 4
  $DeleteNodeResponse = Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:$Port/api/project/nodes/delete" -Method POST -ContentType 'application/json' -Body $DeleteNodeBody -TimeoutSec 20
  $DeleteNodeJson = $DeleteNodeResponse.Content | ConvertFrom-Json
  if (($DeleteNodeJson.project.graph.components | Where-Object { $_.id -eq 'scalar' }).nodes.inputs | Where-Object { $_.id -eq 'bias' }) {
    throw "deleted node still exists in component graph"
  }
  if ($DeleteNodeJson.project.graph.systems[0].public_inputs | Where-Object { $_.id -eq 'scalar_bias' }) {
    throw "deleted node public input should be removed"
  }
  if ($null -ne $DeleteNodeJson.project.default_run_input.inputs.PSObject.Properties['scalar_bias']) {
    throw "deleted node default input should be removed"
  }

  Write-Host "portable package smoke test ok: $PackagePath"
} finally {
  if ($null -ne $DesktopProcess -and -not $DesktopProcess.HasExited) {
    Stop-Process -Id $DesktopProcess.Id -Force -ErrorAction SilentlyContinue
  }
  if ($null -ne $StudioProcess -and -not $StudioProcess.HasExited) {
    Stop-Process -Id $StudioProcess.Id -Force -ErrorAction SilentlyContinue
  }
  if ($ErrLog -and (Test-Path -LiteralPath $ErrLog -ErrorAction SilentlyContinue)) {
    $ErrText = Get-Content -Raw -LiteralPath $ErrLog -ErrorAction SilentlyContinue
    if ($ErrText) {
      Write-Host $ErrText
    }
  }
  $env:PATH = $OriginalPath
  Remove-Item -LiteralPath $TestRoot -Recurse -Force -ErrorAction SilentlyContinue
}
