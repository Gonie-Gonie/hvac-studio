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

$TestRoot = New-PackageTestRoot -Prefix 'hvac-portable-test'

$StudioProcess = $null
$ErrLog = ''
$OriginalPath = $env:PATH

try {
  Expand-Archive -LiteralPath $PackagePath -DestinationPath $TestRoot -Force
  $PackageDir = Get-ChildItem -LiteralPath $TestRoot -Directory | Select-Object -First 1
  if ($null -eq $PackageDir) {
    throw "package did not expand to a directory: $PackagePath"
  }

  $Studio = Join-Path $PackageDir.FullName 'bin\studio.exe'
  $Runner = Join-Path $PackageDir.FullName 'bin\bcs-runner.exe'
  $EnvTool = Join-Path $PackageDir.FullName 'bin\bcs-env.exe'
  $PackagedPython = Join-Path $PackageDir.FullName 'runtime\python\python.exe'
  foreach ($RequiredPath in @($Studio, $Runner, $EnvTool, $PackagedPython)) {
    if (-not (Test-Path -LiteralPath $RequiredPath)) {
      throw "portable package is missing $RequiredPath"
    }
  }

  $env:PATH = Get-MinimalPackagePath -PackageRoot $PackageDir.FullName

  $Project = Join-Path $PackageDir.FullName 'examples\003_feedforward_system\project.bcsproj'
  $Input = Join-Path $PackageDir.FullName 'examples\003_feedforward_system\inputs\case01.json'
  $Expected = Join-Path $PackageDir.FullName 'examples\003_feedforward_system\expected\output.json'
  $Output = Join-Path $PackageDir.FullName 'outputs\003_feedforward_system.json'

  Invoke-Checked $PackagedPython @('--version')
  Invoke-Checked $EnvTool @()
  Invoke-Checked $Runner @('validate', '--project', $Project)
  Invoke-Checked $Runner @('run', '--project', $Project, '--input', $Input, '--output', $Output)

  $ExpectedJson = Get-Content -Raw -LiteralPath $Expected | ConvertFrom-Json
  $ActualJson = Get-Content -Raw -LiteralPath $Output | ConvertFrom-Json
  Assert-JsonSubset -Expected $ExpectedJson -Actual $ActualJson -Path '$'

  $Port = Get-FreePort
  $OutLog = Join-Path $TestRoot 'studio.out.log'
  $ErrLog = Join-Path $TestRoot 'studio.err.log'
  $StudioProcess = Start-Process -FilePath $Studio -WindowStyle Hidden -PassThru -RedirectStandardOutput $OutLog -RedirectStandardError $ErrLog -ArgumentList @(
    '--repo',
    $PackageDir.FullName,
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

  $CreateBody = @{ name = 'Portable Smoke Project'; template = 'scalar' } | ConvertTo-Json -Depth 4
  $CreateResponse = Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:$Port/api/projects" -Method POST -ContentType 'application/json' -Body $CreateBody -TimeoutSec 20
  $CreatedProject = ($CreateResponse.Content | ConvertFrom-Json).project
  if (-not (Test-Path -LiteralPath $CreatedProject.project_path)) {
    throw "created project file was not written: $($CreatedProject.project_path)"
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
  $EditedSource = $SourceJson.source.content -replace 'return \{"result": value \* gain\}, state', "bias = float(inputs.get(`"bias`", 0.0))`n        offset = float(params.get(`"offset`", 0.0))`n        return {`"result`": value * gain + offset + bias}, state"
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

  $ComponentBody = @{ project_path = $CreatedProject.project_path; name = 'Portable Extra Component'; template = 'scalar' } | ConvertTo-Json -Depth 4
  $ComponentResponse = Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:$Port/api/project/components" -Method POST -ContentType 'application/json' -Body $ComponentBody -TimeoutSec 20
  $CreatedComponent = ($ComponentResponse.Content | ConvertFrom-Json).component
  $ComponentSourcePath = Join-Path (Split-Path -Parent $CreatedProject.project_path) "components\$($CreatedComponent.id).py"
  if (-not (Test-Path -LiteralPath $ComponentSourcePath)) {
    throw "created component source was not written: $ComponentSourcePath"
  }
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
  if ($ExtraOutput -ne 21) {
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

  Write-Host "portable package smoke test ok: $PackagePath"
} finally {
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
