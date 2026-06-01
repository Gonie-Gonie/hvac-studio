param(
  [string]$Version = '',
  [switch]$SkipBuild
)

$ErrorActionPreference = 'Stop'

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
. (Join-Path $RepoRoot 'scripts\dev\env.ps1')
. (Join-Path $RepoRoot 'scripts\release\package-common.ps1')

$ResolvedVersion = Resolve-Version -Version $Version
$RuntimeId = 'windows-amd64'
$PackageName = "hvac-studio-$ResolvedVersion-$RuntimeId-portable"
$DistRoot = Join-Path $RepoRoot 'dist'
$StageRoot = Join-Path $DistRoot $PackageName
$ZipPath = Join-Path $DistRoot "$PackageName.zip"

if (-not $SkipBuild) {
  & (Join-Path $RepoRoot 'scripts\release\build-runner.ps1')
  & (Join-Path $RepoRoot 'scripts\release\build-studio.ps1') -Version $ResolvedVersion -OutputRoot $DistRoot
}

Remove-Item -LiteralPath $StageRoot -Recurse -Force -ErrorAction SilentlyContinue
Remove-Item -LiteralPath $ZipPath -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path $StageRoot | Out-Null

$BuiltStudio = Join-Path $DistRoot "hvac-studio-$ResolvedVersion\bin\hvac-studio.exe"

Copy-Tree -Source $BuiltStudio -Destination (Join-Path $StageRoot 'bin\studio.exe')
Copy-Tree -Source (Join-Path $RepoRoot 'bin\bcs-runner.exe') -Destination (Join-Path $StageRoot 'bin\bcs-runner.exe')
Copy-Tree -Source (Join-Path $RepoRoot 'bin\bcs-env.exe') -Destination (Join-Path $StageRoot 'bin\bcs-env.exe')
Copy-Tree -Source (Join-Path $RepoRoot 'python\bcs_worker') -Destination (Join-Path $StageRoot 'python\bcs_worker')
Copy-Tree -Source (Join-Path $RepoRoot 'python\bcs_sdk') -Destination (Join-Path $StageRoot 'python\bcs_sdk')
Copy-Tree -Source (Join-Path $RepoRoot 'schema') -Destination (Join-Path $StageRoot 'schema')
Copy-Tree -Source (Join-Path $RepoRoot 'runtime') -Destination (Join-Path $StageRoot 'runtime')
Copy-PackagedPythonRuntime -RepoRoot $RepoRoot -Destination (Join-Path $StageRoot 'runtime\python')
Copy-Tree -Source (Join-Path $RepoRoot 'docs') -Destination (Join-Path $StageRoot 'docs')
Copy-Tree -Source (Join-Path $RepoRoot 'examples') -Destination (Join-Path $StageRoot 'examples')
Copy-Tree -Source (Join-Path $RepoRoot 'README.md') -Destination (Join-Path $StageRoot 'README.md')
Copy-Tree -Source (Join-Path $RepoRoot 'CHANGELOG.md') -Destination (Join-Path $StageRoot 'CHANGELOG.md')

New-Item -ItemType Directory -Force -Path (Join-Path $StageRoot 'templates\components'), (Join-Path $StageRoot 'templates\systems') | Out-Null
@"
# Templates

Component and system templates will be added as the authoring model stabilizes.
"@ | Set-Content -LiteralPath (Join-Path $StageRoot 'templates\README.md') -Encoding UTF8

@"
param(
  [string]`$Addr = '127.0.0.1:5174',
  [switch]`$NoBrowser
)

`$ErrorActionPreference = 'Stop'
`$Root = Split-Path -Parent `$MyInvocation.MyCommand.Path
`$PythonRoot = Join-Path `$Root 'runtime\python'
`$env:PATH = (@(`$PythonRoot, (Join-Path `$Root 'bin'), `$env:PATH) | Where-Object { `$_ }) -join [IO.Path]::PathSeparator
`$env:PYTHONPATH = (@(
  (Join-Path `$Root 'python\bcs_worker'),
  (Join-Path `$Root 'python\bcs_sdk'),
  `$env:PYTHONPATH
) | Where-Object { `$_ }) -join [IO.Path]::PathSeparator

`$Studio = Join-Path `$Root 'bin\studio.exe'
`$Url = "http://`$Addr"
`$LogRoot = Join-Path `$Root 'logs'
New-Item -ItemType Directory -Force -Path `$LogRoot | Out-Null
`$OutLog = Join-Path `$LogRoot 'studio.out.log'
`$ErrLog = Join-Path `$LogRoot 'studio.err.log'

Write-Host "Starting HVAC Studio at `$Url"
`$StudioProcess = Start-Process -FilePath `$Studio -WindowStyle Hidden -PassThru -RedirectStandardOutput `$OutLog -RedirectStandardError `$ErrLog -ArgumentList @(
  '--repo',
  `$Root,
  '--addr',
  `$Addr
)

`$Ready = `$false
for (`$Index = 0; `$Index -lt 40; `$Index++) {
  try {
    `$Response = Invoke-WebRequest -UseBasicParsing -Uri "`$Url/api/projects" -TimeoutSec 2
    if (`$Response.StatusCode -eq 200) {
      `$Ready = `$true
      break
    }
  } catch {
    Start-Sleep -Milliseconds 500
  }
}

if (-not `$Ready) {
  Write-Host "Studio failed to start. stdout: `$OutLog stderr: `$ErrLog"
  if (Test-Path -LiteralPath `$ErrLog) {
    Get-Content -LiteralPath `$ErrLog -Tail 40
  }
  throw "Studio did not respond at `$Url"
}

if (-not `$NoBrowser) {
  Start-Process `$Url
}

Write-Host "HVAC Studio is running. Close this window or press Ctrl+C to stop it."
try {
  Wait-Process -Id `$StudioProcess.Id
} finally {
  if (`$null -ne `$StudioProcess -and -not `$StudioProcess.HasExited) {
    Stop-Process -Id `$StudioProcess.Id -Force -ErrorAction SilentlyContinue
  }
}
"@ | Set-Content -LiteralPath (Join-Path $StageRoot 'Start-Studio.ps1') -Encoding UTF8

@"
@echo off
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0Start-Studio.ps1" %*
"@ | Set-Content -LiteralPath (Join-Path $StageRoot 'Start-Studio.cmd') -Encoding ASCII

@"
`$ErrorActionPreference = 'Stop'
`$Root = Split-Path -Parent `$MyInvocation.MyCommand.Path
`$PythonRoot = Join-Path `$Root 'runtime\python'
`$env:PATH = (@(`$PythonRoot, (Join-Path `$Root 'bin'), `$env:PATH) | Where-Object { `$_ }) -join [IO.Path]::PathSeparator

`$Project = Join-Path `$Root 'examples\003_feedforward_system\project.bcsproj'
`$Input = Join-Path `$Root 'examples\003_feedforward_system\inputs\case01.json'
`$Output = Join-Path `$Root 'outputs\003_feedforward_system.json'

New-Item -ItemType Directory -Force -Path (Split-Path -Parent `$Output) | Out-Null
& (Join-Path `$Root 'bin\bcs-runner.exe') validate --project `$Project
if (`$LASTEXITCODE -ne 0) { exit `$LASTEXITCODE }
& (Join-Path `$Root 'bin\bcs-runner.exe') run --project `$Project --input `$Input --output `$Output
if (`$LASTEXITCODE -ne 0) { exit `$LASTEXITCODE }
Write-Host "Example result written to `$Output"
"@ | Set-Content -LiteralPath (Join-Path $StageRoot 'Run-Smoke-Example.ps1') -Encoding UTF8

$Commit = ''
$GitCommand = Get-Command git -ErrorAction SilentlyContinue
if ($null -ne $GitCommand) {
  $Commit = (& git rev-parse HEAD 2>$null)
  if ($LASTEXITCODE -ne 0) {
    $Commit = ''
  }
}

$ReleaseManifest = [ordered]@{
  package_name = $PackageName
  package_type = 'studio-portable'
  version = $ResolvedVersion
  runtime_id = $RuntimeId
  primary_platform = 'Windows 10/11 x64'
  future_platforms = @('macOS experimental after MVP')
  commit = $Commit
  built_at_utc = (Get-Date).ToUniversalTime().ToString('o')
  entrypoints = [ordered]@{
    studio = 'bin/studio.exe'
    runner = 'bin/bcs-runner.exe'
    env = 'bin/bcs-env.exe'
    start_script = 'Start-Studio.ps1'
    start_cmd = 'Start-Studio.cmd'
    smoke_example = 'Run-Smoke-Example.ps1'
  }
  includes_embedded_python = $true
  notes = @(
    'Windows-first portable Studio package.',
    'Engine and project format are OS-independent; macOS packaging is a future experimental target.',
    'Includes a bundled Python runtime for included examples.',
    'Project-specific third-party Python package locking is still a later milestone.'
  )
}

$ReleaseManifest | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath (Join-Path $StageRoot 'release-manifest.json') -Encoding UTF8

@"
# HVAC Studio Portable Package

Version: $ResolvedVersion
Runtime: $RuntimeId

Launch Studio:

```powershell
.\Start-Studio.ps1
```

The launch script starts the local Studio server, waits until it is ready, and opens the browser.

Run the CLI smoke example:

```powershell
.\bin\bcs-runner.exe validate --project .\examples\001_scalar_component\project.bcsproj
.\bin\bcs-runner.exe run --project .\examples\001_scalar_component\project.bcsproj --input .\examples\001_scalar_component\inputs\case01.json --output .\outputs\001_scalar_component.json
```

This MVP portable package is Windows-first and includes a bundled Python runtime for included examples. Project-specific third-party Python package locking is still a later milestone.
"@ | Set-Content -LiteralPath (Join-Path $StageRoot 'PACKAGE_README.md') -Encoding UTF8

Remove-PythonCaches -Root $StageRoot
Compress-Archive -LiteralPath $StageRoot -DestinationPath $ZipPath -Force

Write-Host "portable package: $ZipPath"
Write-Output $ZipPath
