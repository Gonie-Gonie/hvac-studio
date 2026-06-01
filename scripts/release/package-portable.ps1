param(
  [string]$Version = '',
  [switch]$SkipBuild
)

$ErrorActionPreference = 'Stop'

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
. (Join-Path $RepoRoot 'scripts\dev\env.ps1')

function Copy-Tree {
  param(
    [Parameter(Mandatory = $true)][string]$Source,
    [Parameter(Mandatory = $true)][string]$Destination
  )

  if (-not (Test-Path -LiteralPath $Source)) {
    throw "source path does not exist: $Source"
  }
  New-Item -ItemType Directory -Force -Path (Split-Path -Parent $Destination) | Out-Null
  Copy-Item -LiteralPath $Source -Destination $Destination -Recurse -Force
}

function Resolve-Version {
  if ($Version) {
    return $Version
  }

  $Git = Get-Command git -ErrorAction SilentlyContinue
  if ($null -eq $Git) {
    return '0.1.0-dev'
  }

  $ExactTag = (& git describe --tags --exact-match 2>$null)
  if ($LASTEXITCODE -eq 0 -and $ExactTag) {
    return $ExactTag.TrimStart('v')
  }

  $ShortSha = (& git rev-parse --short HEAD 2>$null)
  if ($LASTEXITCODE -eq 0 -and $ShortSha) {
    return "0.1.0-dev-$ShortSha"
  }

  return '0.1.0-dev'
}

$ResolvedVersion = Resolve-Version
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
`$Root = Split-Path -Parent `$MyInvocation.MyCommand.Path
& (Join-Path `$Root 'bin\studio.exe') --repo `$Root
"@ | Set-Content -LiteralPath (Join-Path $StageRoot 'Start-Studio.ps1') -Encoding UTF8

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
  }
  includes_embedded_python = $false
  notes = @(
    'Windows-first portable Studio package.',
    'Engine and project format are OS-independent; macOS packaging is a future experimental target.',
    'MVP package still requires Python 3.11+ on PATH until runtime/python is vendored.'
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

Run the CLI smoke example:

```powershell
.\bin\bcs-runner.exe validate --project .\examples\001_scalar_component\project.bcsproj
.\bin\bcs-runner.exe run --project .\examples\001_scalar_component\project.bcsproj --input .\examples\001_scalar_component\inputs\case01.json --output .\outputs\001_scalar_component.json
```

This MVP portable package is Windows-first and still requires Python 3.11+ on PATH. A future package will vendor `runtime/python`.
"@ | Set-Content -LiteralPath (Join-Path $StageRoot 'PACKAGE_README.md') -Encoding UTF8

Compress-Archive -LiteralPath $StageRoot -DestinationPath $ZipPath -Force

Write-Host "portable package: $ZipPath"
Write-Output $ZipPath
