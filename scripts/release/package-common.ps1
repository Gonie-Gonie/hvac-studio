$ErrorActionPreference = 'Stop'

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
  param([string]$Version)

  if ($Version) {
    return $Version
  }

  $Git = Get-Command git -ErrorAction SilentlyContinue
  if ($null -eq $Git) {
    return '0.1.0-dev'
  }

  $PreviousErrorActionPreference = $ErrorActionPreference
  $ErrorActionPreference = 'Continue'
  try {
    $ExactTag = (& git describe --tags --exact-match 2>$null)
    $ExactTagExitCode = $LASTEXITCODE
  } finally {
    $ErrorActionPreference = $PreviousErrorActionPreference
  }
  if ($ExactTagExitCode -eq 0 -and $ExactTag) {
    return $ExactTag.TrimStart('v')
  }

  $PreviousErrorActionPreference = $ErrorActionPreference
  $ErrorActionPreference = 'Continue'
  try {
    $ShortSha = (& git rev-parse --short HEAD 2>$null)
    $ShortShaExitCode = $LASTEXITCODE
  } finally {
    $ErrorActionPreference = $PreviousErrorActionPreference
  }
  if ($ShortShaExitCode -eq 0 -and $ShortSha) {
    return "0.1.0-dev-$ShortSha"
  }

  return '0.1.0-dev'
}

function Resolve-PythonRuntimeSource {
  param([Parameter(Mandatory = $true)][string]$RepoRoot)

  if ($env:HVAC_STUDIO_PYTHON -and (Test-Path -LiteralPath $env:HVAC_STUDIO_PYTHON)) {
    $BasePrefix = (& $env:HVAC_STUDIO_PYTHON -c 'import sys; print(sys.base_prefix)' 2>$null)
    if ($LASTEXITCODE -eq 0 -and $BasePrefix) {
      $BasePrefix = $BasePrefix.Trim()
      if (Test-Path -LiteralPath (Join-Path $BasePrefix 'python.exe')) {
        return (Resolve-Path -LiteralPath $BasePrefix).Path
      }
    }
  }

  $PythonInstallRoot = Join-Path $RepoRoot '.repo_tools\python'
  if (Test-Path -LiteralPath $PythonInstallRoot) {
    $Candidates = Get-ChildItem -LiteralPath $PythonInstallRoot -Directory |
      Where-Object { Test-Path -LiteralPath (Join-Path $_.FullName 'python.exe') } |
      Sort-Object Name -Descending
    if ($Candidates.Count -gt 0) {
      return $Candidates[0].FullName
    }
  }

  throw 'packaged Python runtime source was not found. Run scripts/dev/setup.ps1 first.'
}

function Copy-PackagedPythonRuntime {
  param(
    [Parameter(Mandatory = $true)][string]$RepoRoot,
    [Parameter(Mandatory = $true)][string]$Destination
  )

  $PythonSource = Resolve-PythonRuntimeSource -RepoRoot $RepoRoot
  Remove-Item -LiteralPath $Destination -Recurse -Force -ErrorAction SilentlyContinue
  Copy-Tree -Source $PythonSource -Destination $Destination

  $PythonExe = Join-Path $Destination 'python.exe'
  if (-not (Test-Path -LiteralPath $PythonExe)) {
    throw "packaged Python runtime is missing python.exe: $PythonExe"
  }
  & $PythonExe --version
  if ($LASTEXITCODE -ne 0) {
    throw "$PythonExe failed with exit code $LASTEXITCODE"
  }
}

function Remove-PythonCaches {
  param([Parameter(Mandatory = $true)][string]$Root)

  if (-not (Test-Path -LiteralPath $Root)) {
    return
  }

  Get-ChildItem -LiteralPath $Root -Recurse -Directory -Filter '__pycache__' -ErrorAction SilentlyContinue |
    Remove-Item -Recurse -Force -ErrorAction SilentlyContinue
  Get-ChildItem -LiteralPath $Root -Recurse -File -ErrorAction SilentlyContinue |
    Where-Object { $_.Extension -in @('.pyc', '.pyo') } |
    Remove-Item -Force -ErrorAction SilentlyContinue
}

function Copy-DocumentationAssets {
  param(
    [Parameter(Mandatory = $true)][string]$RepoRoot,
    [Parameter(Mandatory = $true)][string]$StageRoot
  )

  $DocsRoot = Join-Path $StageRoot 'docs'
  Copy-Tree -Source (Join-Path $RepoRoot 'docs') -Destination $DocsRoot

  $MkDocs = Get-Command mkdocs -ErrorAction SilentlyContinue
  if ($null -eq $MkDocs) {
    @(
      '# Offline HTML Docs'
      ''
      'MkDocs was not available when this package was built, so this package includes Markdown documentation only.'
      'Install MkDocs in the build environment to include `docs/site/` as offline HTML release assets.'
    ) | Set-Content -LiteralPath (Join-Path $DocsRoot 'HTML_BUILD_SKIPPED.md') -Encoding UTF8
    return [ordered]@{
      source = 'docs'
      html = ''
      html_status = 'skipped'
      html_reason = 'mkdocs command was not available'
    }
  }

  $SiteRoot = Join-Path $DocsRoot 'site'
  Push-Location $RepoRoot
  try {
    & $MkDocs.Source build --site-dir $SiteRoot
    if ($LASTEXITCODE -ne 0) {
      throw "mkdocs build failed with exit code $LASTEXITCODE"
    }
  } finally {
    Pop-Location
  }

  return [ordered]@{
    source = 'docs'
    html = 'docs/site'
    html_status = 'built'
    html_reason = ''
  }
}

function Get-GitOutputLine {
  param([Parameter(Mandatory = $true)][string[]]$Arguments)

  $PreviousErrorActionPreference = $ErrorActionPreference
  $ErrorActionPreference = 'Continue'
  try {
    $Output = (& git @Arguments 2>$null)
    if ($LASTEXITCODE -ne 0) {
      return ''
    }
  } finally {
    $ErrorActionPreference = $PreviousErrorActionPreference
  }
  return [string](($Output | Select-Object -First 1) -as [string])
}

function Get-GitMetadata {
  param([Parameter(Mandatory = $true)][string]$RepoRoot)

  $Git = Get-Command git -ErrorAction SilentlyContinue
  $Metadata = [ordered]@{
    commit = ''
    short_commit = ''
    branch = ''
    exact_tag = ''
    dirty = $false
  }
  if ($null -eq $Git) {
    return $Metadata
  }

  Push-Location $RepoRoot
  try {
    $Metadata.commit = Get-GitOutputLine -Arguments @('rev-parse', 'HEAD')
    $Metadata.short_commit = Get-GitOutputLine -Arguments @('rev-parse', '--short', 'HEAD')
    $Metadata.branch = Get-GitOutputLine -Arguments @('rev-parse', '--abbrev-ref', 'HEAD')
    $Metadata.exact_tag = Get-GitOutputLine -Arguments @('describe', '--tags', '--exact-match')
    $DirtyOutput = Get-GitOutputLine -Arguments @('status', '--porcelain')
    $Metadata.dirty = [bool]$DirtyOutput
  } finally {
    Pop-Location
  }

  foreach ($Key in @('commit', 'short_commit', 'branch', 'exact_tag')) {
    if ($null -eq $Metadata[$Key]) {
      $Metadata[$Key] = ''
    } else {
      $Metadata[$Key] = [string]$Metadata[$Key]
    }
  }
  return $Metadata
}

function Get-ToolVersionLine {
  param(
    [Parameter(Mandatory = $true)][string]$Executable,
    [string[]]$Arguments = @('--version')
  )

  if (-not $Executable) {
    return ''
  }
  if (-not (Test-Path -LiteralPath $Executable)) {
    $Command = Get-Command $Executable -ErrorAction SilentlyContinue
    if ($null -eq $Command) {
      return ''
    }
    $Executable = $Command.Source
  }

  $PreviousErrorActionPreference = $ErrorActionPreference
  $ErrorActionPreference = 'Continue'
  try {
    $Output = (& $Executable @Arguments 2>$null)
    if ($LASTEXITCODE -ne 0) {
      return ''
    }
  } finally {
    $ErrorActionPreference = $PreviousErrorActionPreference
  }
  return [string](($Output | Select-Object -First 1) -as [string])
}

function Get-PackageFileList {
  param([Parameter(Mandatory = $true)][string]$StageRoot)

  $StagePath = (Resolve-Path -LiteralPath $StageRoot).Path
  Push-Location $StagePath
  try {
    $Files = Get-ChildItem -Recurse -File -ErrorAction SilentlyContinue |
      ForEach-Object {
        $Rel = Resolve-Path -LiteralPath $_.FullName -Relative
        ($Rel -replace '^[./\\]+', '') -replace '\\', '/'
      } |
      Sort-Object
  } finally {
    Pop-Location
  }
  return @($Files)
}

function Write-ReleaseProvenance {
  param(
    [Parameter(Mandatory = $true)][string]$RepoRoot,
    [Parameter(Mandatory = $true)][string]$StageRoot,
    [Parameter(Mandatory = $true)][string]$PackageName,
    [Parameter(Mandatory = $true)][string]$PackageType,
    [Parameter(Mandatory = $true)][string]$Version,
    [Parameter(Mandatory = $true)][string]$RuntimeId,
    [object]$Documentation = @{}
  )

  $PythonExe = Join-Path $StageRoot 'runtime\python\python.exe'
  $WailsVersion = Get-ToolVersionLine -Executable 'wails'
  if (-not $WailsVersion) {
    $WailsVersion = Get-ToolVersionLine -Executable 'wails.exe'
  }

  $Files = Get-PackageFileList -StageRoot $StageRoot
  if ($Files -notcontains 'release-provenance.json') {
    $Files += 'release-provenance.json'
    $Files = @($Files | Sort-Object)
  }

  $Provenance = [ordered]@{
    schema = 'hvac-studio.release-provenance.v1'
    package_name = $PackageName
    package_type = $PackageType
    version = $Version
    runtime_id = $RuntimeId
    built_at_utc = (Get-Date).ToUniversalTime().ToString('o')
    git = Get-GitMetadata -RepoRoot $RepoRoot
    tools = [ordered]@{
      go = Get-ToolVersionLine -Executable $env:HVAC_STUDIO_GO
      uv = Get-ToolVersionLine -Executable $env:HVAC_STUDIO_UV
      python = Get-ToolVersionLine -Executable $env:HVAC_STUDIO_PYTHON
      packaged_python = Get-ToolVersionLine -Executable $PythonExe
      wails = $WailsVersion
    }
    documentation = $Documentation
    files = $Files
  }

  $Provenance | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath (Join-Path $StageRoot 'release-provenance.json') -Encoding UTF8
}

function Assert-ReleaseProvenance {
  param(
    [Parameter(Mandatory = $true)][string]$PackageRoot,
    [Parameter(Mandatory = $true)][string]$PackageType,
    [string]$Version = ''
  )

  $ManifestPath = Join-Path $PackageRoot 'release-manifest.json'
  $ProvenancePath = Join-Path $PackageRoot 'release-provenance.json'
  foreach ($RequiredPath in @($ManifestPath, $ProvenancePath)) {
    if (-not (Test-Path -LiteralPath $RequiredPath)) {
      throw "package provenance file is missing: $RequiredPath"
    }
  }

  $Manifest = Get-Content -Raw -LiteralPath $ManifestPath | ConvertFrom-Json
  $Provenance = Get-Content -Raw -LiteralPath $ProvenancePath | ConvertFrom-Json
  if ($Manifest.provenance -ne 'release-provenance.json') {
    throw "release manifest provenance path mismatch: $($Manifest.provenance)"
  }
  if ($Provenance.schema -ne 'hvac-studio.release-provenance.v1') {
    throw "release provenance schema mismatch: $($Provenance.schema)"
  }
  if ($Provenance.package_type -ne $PackageType) {
    throw "release provenance package type mismatch: $($Provenance.package_type)"
  }
  if ($Version -and $Provenance.version -ne $Version) {
    throw "release provenance version mismatch: $($Provenance.version)"
  }
  if (-not $Provenance.git.commit) {
    throw 'release provenance is missing git.commit'
  }
  if (-not $Provenance.tools.packaged_python) {
    throw 'release provenance is missing tools.packaged_python'
  }
  if (-not $Provenance.documentation.source) {
    throw 'release provenance is missing documentation.source'
  }
  if ($Provenance.documentation.html_status -notin @('built', 'skipped')) {
    throw "release provenance documentation html_status mismatch: $($Provenance.documentation.html_status)"
  }
  foreach ($RequiredFile in @('release-manifest.json', 'release-provenance.json', 'PACKAGE_README.md')) {
    if ($Provenance.files -notcontains $RequiredFile) {
      throw "release provenance file list is missing $RequiredFile"
    }
  }
}

function Get-MinimalPackagePath {
  param([Parameter(Mandatory = $true)][string]$PackageRoot)

  $WindowsSystem = Join-Path $env:SystemRoot 'System32'
  $WindowsRoot = $env:SystemRoot
  $PackageBin = Join-Path $PackageRoot 'bin'
  return (@($PackageBin, $WindowsSystem, $WindowsRoot) | Where-Object { $_ -and (Test-Path -LiteralPath $_) }) -join [IO.Path]::PathSeparator
}

function New-PackageTestRoot {
  param([Parameter(Mandatory = $true)][string]$Prefix)

  $Base = 'C:\tmp'
  try {
    New-Item -ItemType Directory -Force -Path $Base | Out-Null
  } catch {
    $Base = [IO.Path]::GetTempPath()
  }

  $Root = Join-Path $Base ($Prefix + '-' + [Guid]::NewGuid().ToString('N'))
  New-Item -ItemType Directory -Force -Path $Root | Out-Null
  return $Root
}
