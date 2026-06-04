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

function Invoke-MkDocsBuild {
  param(
    [Parameter(Mandatory = $true)][string]$RepoRoot,
    [Parameter(Mandatory = $true)][string]$SiteRoot
  )

  $MkDocs = Get-Command mkdocs -ErrorAction SilentlyContinue
  Push-Location $RepoRoot
  try {
    if ($null -ne $MkDocs) {
      & $MkDocs.Source build --strict --site-dir $SiteRoot
    } elseif ($env:HVAC_STUDIO_UV -and (Test-Path -LiteralPath $env:HVAC_STUDIO_UV)) {
      & $env:HVAC_STUDIO_UV tool run mkdocs build --strict --site-dir $SiteRoot
    } else {
      throw 'mkdocs was not found and repo-local uv is unavailable; cannot build required HTML docs'
    }
    if ($LASTEXITCODE -ne 0) {
      throw "mkdocs build failed with exit code $LASTEXITCODE"
    }
  } finally {
    Pop-Location
  }

  if (-not (Test-Path -LiteralPath (Join-Path $SiteRoot 'index.html'))) {
    throw "mkdocs build did not write index.html under $SiteRoot"
  }
}

function Copy-DocumentationAssets {
  param(
    [Parameter(Mandatory = $true)][string]$RepoRoot,
    [Parameter(Mandatory = $true)][string]$StageRoot,
    [string]$Version = ''
  )

  $DocsRoot = Join-Path $StageRoot 'docs'
  Copy-Tree -Source (Join-Path $RepoRoot 'docs') -Destination $DocsRoot
  $DocsVersion = [ordered]@{
    schema = 'hvac-studio.docs.v1'
    version = $Version
    source = 'docs'
    html = 'docs/site'
    manual = 'docs/manual/hvac-studio-manual.md'
    pdf = 'docs/manual/hvac-studio-manual.pdf'
    pdf_status = 'optional'
  }
  $DocsVersion | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath (Join-Path $DocsRoot 'version.json') -Encoding UTF8

  $SiteRoot = Join-Path $DocsRoot 'site'
  Invoke-MkDocsBuild -RepoRoot $RepoRoot -SiteRoot $SiteRoot
  $DocsVersion | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath (Join-Path $SiteRoot 'version.json') -Encoding UTF8

  return [ordered]@{
    source = 'docs'
    html = 'docs/site'
    version = 'docs/version.json'
    html_version = 'docs/site/version.json'
    html_status = 'built'
    html_reason = ''
  }
}

function Copy-ReleaseTrustAssets {
  param(
    [Parameter(Mandatory = $true)][string]$RepoRoot,
    [Parameter(Mandatory = $true)][string]$StageRoot,
    [Parameter(Mandatory = $true)][string]$PackageType,
    [Parameter(Mandatory = $true)][string]$Version
  )

  $LegalSource = Join-Path $RepoRoot 'docs\legal'
  $LegalRoot = Join-Path $StageRoot 'legal'
  New-Item -ItemType Directory -Force -Path $LegalRoot | Out-Null
  foreach ($Name in @('license-notices.md', 'dependency-notices.md', 'support-matrix.md', 'release-notes-policy.md')) {
    Copy-Tree -Source (Join-Path $LegalSource $Name) -Destination (Join-Path $LegalRoot $Name)
  }

  $SigningStatus = if ($Version -match '-(alpha|beta|rc|dev)([.-]|$)') { 'unsigned-prerelease' } else { 'unsigned-stable-blocker' }
  $Trust = [ordered]@{
    schema = 'hvac-studio.release-trust.v1'
    package_type = $PackageType
    version = $Version
    code_signing = [ordered]@{
      status = $SigningStatus
      stable_public_release_requires_signature = $true
      verification = 'No Authenticode signature is promised unless release notes and this file say signed.'
    }
    checksums = [ordered]@{
      package_internal = 'release-checksums.json when present'
      workflow_artifact = 'SHA256SUMS.txt'
      algorithm = 'SHA-256'
    }
    notices = [ordered]@{
      license = 'legal/license-notices.md'
      dependencies = 'legal/dependency-notices.md'
      support_matrix = 'legal/support-matrix.md'
      release_notes_policy = 'legal/release-notes-policy.md'
    }
  }
  $Trust | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath (Join-Path $StageRoot 'release-trust.json') -Encoding UTF8
  return $Trust
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

function Write-ReleaseChecksums {
  param([Parameter(Mandatory = $true)][string]$StageRoot)

  $ChecksumRel = 'release-checksums.json'
  $ChecksumPath = Join-Path $StageRoot $ChecksumRel
  Remove-Item -LiteralPath $ChecksumPath -Force -ErrorAction SilentlyContinue

  $ProvenancePath = Join-Path $StageRoot 'release-provenance.json'
  if (-not (Test-Path -LiteralPath $ProvenancePath)) {
    throw 'release provenance must be written before release checksums'
  }
  $Provenance = Get-Content -Raw -LiteralPath $ProvenancePath | ConvertFrom-Json
  $Files = @($Provenance.files)
  if ($Files -notcontains $ChecksumRel) {
    $Files += $ChecksumRel
    $Provenance.files = @($Files | Sort-Object)
    $Provenance | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $ProvenancePath -Encoding UTF8
  }

  $Entries = @()
  foreach ($Rel in (Get-PackageFileList -StageRoot $StageRoot)) {
    if ($Rel -eq $ChecksumRel) {
      continue
    }
    $Path = Join-Path $StageRoot ($Rel -replace '/', '\')
    $Item = Get-Item -LiteralPath $Path
    $Entries += [ordered]@{
      path = $Rel
      sha256 = (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant()
      bytes = $Item.Length
    }
  }

  [ordered]@{
    schema = 'hvac-studio.release-checksums.v1'
    generated_at_utc = (Get-Date).ToUniversalTime().ToString('o')
    files = @($Entries | Sort-Object path)
  } | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $ChecksumPath -Encoding UTF8
}

function Assert-ReleaseProvenance {
  param(
    [Parameter(Mandatory = $true)][string]$PackageRoot,
    [Parameter(Mandatory = $true)][string]$PackageType,
    [string]$Version = ''
  )

  $ManifestPath = Join-Path $PackageRoot 'release-manifest.json'
  $ProvenancePath = Join-Path $PackageRoot 'release-provenance.json'
  $ChecksumsPath = Join-Path $PackageRoot 'release-checksums.json'
  foreach ($RequiredPath in @($ManifestPath, $ProvenancePath, $ChecksumsPath)) {
    if (-not (Test-Path -LiteralPath $RequiredPath)) {
      throw "package provenance file is missing: $RequiredPath"
    }
  }

  $Manifest = Get-Content -Raw -LiteralPath $ManifestPath | ConvertFrom-Json
  $Provenance = Get-Content -Raw -LiteralPath $ProvenancePath | ConvertFrom-Json
  $Checksums = Get-Content -Raw -LiteralPath $ChecksumsPath | ConvertFrom-Json
  $TrustPath = Join-Path $PackageRoot 'release-trust.json'
  if (-not (Test-Path -LiteralPath $TrustPath)) {
    throw 'release package is missing release-trust.json'
  }
  $Trust = Get-Content -Raw -LiteralPath $TrustPath | ConvertFrom-Json
  if ($Trust.schema -ne 'hvac-studio.release-trust.v1') {
    throw "release trust schema mismatch: $($Trust.schema)"
  }
  if ($Trust.package_type -ne $PackageType) {
    throw "release trust package type mismatch: $($Trust.package_type)"
  }
  if ($Manifest.provenance -ne 'release-provenance.json') {
    throw "release manifest provenance path mismatch: $($Manifest.provenance)"
  }
  if (-not $Manifest.trust -or $Manifest.trust.schema -ne 'hvac-studio.release-trust.v1') {
    throw 'release manifest is missing trust metadata'
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
  $RequiresPackagedPython = $PackageType -notin @('studio-macos-experimental')
  if ($RequiresPackagedPython -and -not $Provenance.tools.packaged_python) {
    throw 'release provenance is missing tools.packaged_python'
  }
  if (-not $Provenance.documentation.source) {
    throw 'release provenance is missing documentation.source'
  }
  if ($Provenance.documentation.html_status -ne 'built') {
    throw "release provenance documentation html_status mismatch: $($Provenance.documentation.html_status)"
  }
  foreach ($RequiredDocPath in @('docs\site\index.html', 'docs\version.json', 'docs\site\version.json')) {
    if (-not (Test-Path -LiteralPath (Join-Path $PackageRoot $RequiredDocPath))) {
      throw "release package is missing required $RequiredDocPath"
    }
  }
  foreach ($RequiredFile in @('release-manifest.json', 'release-provenance.json', 'release-checksums.json', 'release-trust.json', 'PACKAGE_README.md', 'docs/version.json', 'docs/site/version.json', 'legal/license-notices.md', 'legal/dependency-notices.md', 'legal/support-matrix.md', 'legal/release-notes-policy.md')) {
    if ($Provenance.files -notcontains $RequiredFile) {
      throw "release provenance file list is missing $RequiredFile"
    }
  }
  if ($Checksums.schema -ne 'hvac-studio.release-checksums.v1') {
    throw "release checksums schema mismatch: $($Checksums.schema)"
  }
  $ChecksumPaths = @($Checksums.files | ForEach-Object { $_.path })
  foreach ($RequiredFile in @('release-manifest.json', 'release-provenance.json', 'release-trust.json', 'PACKAGE_README.md', 'docs/site/index.html', 'docs/version.json', 'docs/site/version.json', 'legal/license-notices.md', 'legal/dependency-notices.md', 'legal/support-matrix.md', 'legal/release-notes-policy.md')) {
    if ($ChecksumPaths -notcontains $RequiredFile) {
      throw "release checksums are missing $RequiredFile"
    }
  }
  foreach ($Entry in @($Checksums.files)) {
    $Path = Join-Path $PackageRoot (($Entry.path -as [string]) -replace '/', '\')
    if (-not (Test-Path -LiteralPath $Path)) {
      throw "release checksum path is missing: $($Entry.path)"
    }
    $Actual = (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($Actual -ne $Entry.sha256) {
      throw "release checksum mismatch for $($Entry.path)"
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

  $Candidates = @()
  if ($env:HVAC_STUDIO_TEST_ROOT) {
    $Candidates += $env:HVAC_STUDIO_TEST_ROOT
  }
  $Candidates += 'C:\tmp'
  $Candidates += [IO.Path]::GetTempPath()
  try {
    $Candidates += (Join-Path (Resolve-Path -LiteralPath '.').Path 'artifacts\package-tests')
  } catch {
  }

  foreach ($Base in @($Candidates | Where-Object { $_ } | Select-Object -Unique)) {
    try {
      New-Item -ItemType Directory -Force -Path $Base | Out-Null
      $Root = Join-Path $Base ($Prefix + '-' + [Guid]::NewGuid().ToString('N'))
      New-Item -ItemType Directory -Force -Path $Root | Out-Null
      return $Root
    } catch {
      continue
    }
  }

  throw 'could not create a package test root'
}
