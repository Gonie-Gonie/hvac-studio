param(
  [string]$Version = '',
  [switch]$SkipBuild,
  [switch]$KeepStage
)

$ErrorActionPreference = 'Stop'

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
. (Join-Path $RepoRoot 'scripts\dev\env.ps1')
. (Join-Path $RepoRoot 'scripts\release\package-common.ps1')

if (-not $SkipBuild) {
  & (Join-Path $RepoRoot 'scripts\release\build-runner.ps1')
}

$ResolvedVersion = Resolve-Version -Version $Version
$RuntimeId = 'windows-amd64'
$PackageName = "hvac-studio-runtime-$ResolvedVersion-$RuntimeId"
$DistRoot = Join-Path $RepoRoot 'dist'
$StageRoot = Join-Path $DistRoot $PackageName
$ZipPath = Join-Path $DistRoot "$PackageName.zip"

Remove-Item -LiteralPath $StageRoot -Recurse -Force -ErrorAction SilentlyContinue
Remove-Item -LiteralPath $ZipPath -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path $StageRoot | Out-Null

Copy-Tree -Source (Join-Path $RepoRoot 'bin\bcs-runner.exe') -Destination (Join-Path $StageRoot 'bin\bcs-runner.exe')
Copy-Tree -Source (Join-Path $RepoRoot 'bin\bcs-env.exe') -Destination (Join-Path $StageRoot 'bin\bcs-env.exe')
Copy-Tree -Source (Join-Path $RepoRoot 'python\bcs_worker') -Destination (Join-Path $StageRoot 'python\bcs_worker')
Copy-Tree -Source (Join-Path $RepoRoot 'python\bcs_sdk') -Destination (Join-Path $StageRoot 'python\bcs_sdk')
Copy-Tree -Source (Join-Path $RepoRoot 'schema') -Destination (Join-Path $StageRoot 'schema')
Copy-Tree -Source (Join-Path $RepoRoot 'runtime') -Destination (Join-Path $StageRoot 'runtime')
Copy-PackagedPythonRuntime -RepoRoot $RepoRoot -Destination (Join-Path $StageRoot 'runtime\python')
$Documentation = Copy-DocumentationAssets -RepoRoot $RepoRoot -StageRoot $StageRoot
Copy-Tree -Source (Join-Path $RepoRoot 'examples') -Destination (Join-Path $StageRoot 'examples')
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
  includes_embedded_python = $true
  provenance = 'release-provenance.json'
  documentation = $Documentation
  entrypoints = [ordered]@{
    runner = 'bin/bcs-runner.exe'
    env = 'bin/bcs-env.exe'
    example_project = 'examples/001_scalar_component/project.bcsproj'
  }
  notes = @(
    'MVP runtime package: runner executable, bundled Python runtime, and Python worker/source files.',
    'Project-specific Python lockfiles are preserved and checked when projects declare environment.lockfile.'
  )
}

$ReleaseManifest | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath (Join-Path $StageRoot 'release-manifest.json') -Encoding UTF8

$PackageReadme = @(
  '# HVAC Studio Runtime Package'
  ''
  "Version: $ResolvedVersion"
  "Runtime: $RuntimeId"
  ''
  'Run the included smoke example:'
  ''
  '```powershell'
  '.\bin\bcs-runner.exe validate --project .\examples\001_scalar_component\project.bcsproj'
  '.\bin\bcs-runner.exe run --project .\examples\001_scalar_component\project.bcsproj --input .\examples\001_scalar_component\inputs\case01.json --output .\outputs\001_scalar_component.json'
  '```'
  ''
  'This MVP package includes the runner executable, environment checker, bundled Python runtime, and Python worker source. Project-specific Python lockfiles are preserved when projects declare environment.lockfile.'
  ''
  'Check the packaged runtime:'
  ''
  '```powershell'
  '.\bin\bcs-env.exe check'
  '```'
)
$PackageReadme | Set-Content -LiteralPath (Join-Path $StageRoot 'PACKAGE_README.md') -Encoding UTF8

Remove-PythonCaches -Root $StageRoot

Write-ReleaseProvenance `
  -RepoRoot $RepoRoot `
  -StageRoot $StageRoot `
  -PackageName $PackageName `
  -PackageType 'runtime' `
  -Version $ResolvedVersion `
  -RuntimeId $RuntimeId `
  -Documentation $Documentation

Write-ReleaseChecksums -StageRoot $StageRoot

Compress-Archive -LiteralPath $StageRoot -DestinationPath $ZipPath -Force

if (-not $KeepStage) {
  Remove-Item -LiteralPath $StageRoot -Recurse -Force -ErrorAction SilentlyContinue
}

Write-Host "runtime package: $ZipPath"
Write-Output $ZipPath
