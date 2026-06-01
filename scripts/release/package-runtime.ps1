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

if (-not $SkipBuild) {
  & (Join-Path $RepoRoot 'scripts\release\build-runner.ps1')
}

$ResolvedVersion = Resolve-Version
$RuntimeId = 'windows-amd64'
$PackageName = "hvac-studio-runtime-$ResolvedVersion-$RuntimeId"
$DistRoot = Join-Path $RepoRoot 'dist'
$StageRoot = Join-Path $DistRoot $PackageName
$ZipPath = Join-Path $DistRoot "$PackageName.zip"

Remove-Item -LiteralPath $StageRoot -Recurse -Force -ErrorAction SilentlyContinue
Remove-Item -LiteralPath $ZipPath -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path $StageRoot | Out-Null

Copy-Tree -Source (Join-Path $RepoRoot 'bin\bcs-runner.exe') -Destination (Join-Path $StageRoot 'bin\bcs-runner.exe')
Copy-Tree -Source (Join-Path $RepoRoot 'python\bcs_worker') -Destination (Join-Path $StageRoot 'python\bcs_worker')
Copy-Tree -Source (Join-Path $RepoRoot 'python\bcs_sdk') -Destination (Join-Path $StageRoot 'python\bcs_sdk')
Copy-Tree -Source (Join-Path $RepoRoot 'schema') -Destination (Join-Path $StageRoot 'schema')
Copy-Tree -Source (Join-Path $RepoRoot 'runtime') -Destination (Join-Path $StageRoot 'runtime')
Copy-Tree -Source (Join-Path $RepoRoot 'docs') -Destination (Join-Path $StageRoot 'docs')
Copy-Tree -Source (Join-Path $RepoRoot 'examples\001_scalar_component') -Destination (Join-Path $StageRoot 'examples\001_scalar_component')
Copy-Tree -Source (Join-Path $RepoRoot 'README.md') -Destination (Join-Path $StageRoot 'README.md')
Copy-Tree -Source (Join-Path $RepoRoot 'CHANGELOG.md') -Destination (Join-Path $StageRoot 'CHANGELOG.md')

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
  version = $ResolvedVersion
  runtime_id = $RuntimeId
  commit = $Commit
  built_at_utc = (Get-Date).ToUniversalTime().ToString('o')
  entrypoints = [ordered]@{
    runner = 'bin/bcs-runner.exe'
    example_project = 'examples/001_scalar_component/project.bcsproj'
  }
  notes = @(
    'MVP runtime package: runner executable plus Python worker/source files.',
    'Requires Python 3.11+ on PATH until embedded runtime/python packaging is added.'
  )
}

$ReleaseManifest | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath (Join-Path $StageRoot 'release-manifest.json') -Encoding UTF8

@"
# HVAC Studio Runtime Package

Version: $ResolvedVersion
Runtime: $RuntimeId

Run the included smoke example:

```powershell
.\bin\bcs-runner.exe validate --project .\examples\001_scalar_component\project.bcsproj
.\bin\bcs-runner.exe run --project .\examples\001_scalar_component\project.bcsproj --input .\examples\001_scalar_component\inputs\case01.json --output .\outputs\001_scalar_component.json
```

This MVP package includes the runner executable and Python worker source. It still requires Python 3.11+ on PATH. A future runtime-only package will vendor `runtime/python`.
"@ | Set-Content -LiteralPath (Join-Path $StageRoot 'PACKAGE_README.md') -Encoding UTF8

Compress-Archive -LiteralPath $StageRoot -DestinationPath $ZipPath -Force

Write-Host "runtime package: $ZipPath"
Write-Output $ZipPath
