param(
  [string]$Version = '',
  [string]$PackagePath = ''
)

$ErrorActionPreference = 'Stop'

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
. (Join-Path $RepoRoot 'scripts\dev\env.ps1')
. (Join-Path $RepoRoot 'scripts\dev\json-assert.ps1')

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

$TestRoot = Join-Path ([IO.Path]::GetTempPath()) ('hvac-studio-portable-test-' + [Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Force -Path $TestRoot | Out-Null

$StudioProcess = $null
$ErrLog = ''

try {
  Expand-Archive -LiteralPath $PackagePath -DestinationPath $TestRoot -Force
  $PackageDir = Get-ChildItem -LiteralPath $TestRoot -Directory | Select-Object -First 1
  if ($null -eq $PackageDir) {
    throw "package did not expand to a directory: $PackagePath"
  }

  $Studio = Join-Path $PackageDir.FullName 'bin\studio.exe'
  $Runner = Join-Path $PackageDir.FullName 'bin\bcs-runner.exe'
  $EnvTool = Join-Path $PackageDir.FullName 'bin\bcs-env.exe'
  foreach ($RequiredPath in @($Studio, $Runner, $EnvTool)) {
    if (-not (Test-Path -LiteralPath $RequiredPath)) {
      throw "portable package is missing $RequiredPath"
    }
  }

  $Project = Join-Path $PackageDir.FullName 'examples\003_feedforward_system\project.bcsproj'
  $Input = Join-Path $PackageDir.FullName 'examples\003_feedforward_system\inputs\case01.json'
  $Expected = Join-Path $PackageDir.FullName 'examples\003_feedforward_system\expected\output.json'
  $Output = Join-Path $PackageDir.FullName 'outputs\003_feedforward_system.json'

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
  Remove-Item -LiteralPath $TestRoot -Recurse -Force -ErrorAction SilentlyContinue
}
