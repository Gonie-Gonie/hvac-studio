param(
  [string]$Version = '',
  [string]$OutputRoot = ''
)

$ErrorActionPreference = 'Stop'

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
. (Join-Path $RepoRoot 'scripts\release\package-common.ps1')

$ResolvedVersion = Resolve-Version -Version $Version
if (-not $OutputRoot) {
  $OutputRoot = Join-Path $RepoRoot 'dist\docs\manual'
}

New-Item -ItemType Directory -Force -Path $OutputRoot | Out-Null
$ManualPath = Join-Path $OutputRoot 'hvac-studio-manual.md'
$PdfPath = Join-Path $OutputRoot 'hvac-studio-manual.pdf'
$StatusPath = Join-Path $OutputRoot 'manual-build.json'

$Sources = @(
  'docs\index.md',
  'docs\user\index.md',
  'docs\user\quick-start.md',
  'docs\user\concept-map.md',
  'docs\user\core-concepts.md',
  'docs\user\how-it-works.md',
  'docs\user\tutorials.md',
  'docs\user\examples.md',
  'docs\user\model-replacement.md',
  'docs\user\cli-runner.md',
  'docs\user\artifact-compatibility.md',
  'docs\user\export-runtime.md',
  'docs\user\troubleshooting.md',
  'docs\release-trust.md',
  'docs\legal\support-matrix.md',
  'docs\legal\release-notes-policy.md',
  'docs\legal\license-notices.md',
  'docs\legal\dependency-notices.md'
)

$Lines = @(
  '# HVAC Studio Manual'
  ''
  "Version: $ResolvedVersion"
  "Generated: $((Get-Date).ToUniversalTime().ToString('o'))"
  ''
)
foreach ($Rel in $Sources) {
  $Path = Join-Path $RepoRoot $Rel
  if (-not (Test-Path -LiteralPath $Path)) {
    throw "manual source is missing: $Rel"
  }
  $Lines += ''
  $Lines += '<div style="page-break-after: always;"></div>'
  $Lines += ''
  $Lines += Get-Content -LiteralPath $Path -Encoding UTF8
}
$Lines | Set-Content -LiteralPath $ManualPath -Encoding UTF8

$PdfStatus = 'skipped'
$PdfReason = 'pandoc was not found'
$Pandoc = Get-Command pandoc -ErrorAction SilentlyContinue
if ($null -ne $Pandoc) {
  & $Pandoc.Source $ManualPath -o $PdfPath
  if ($LASTEXITCODE -ne 0) {
    throw "pandoc failed with exit code $LASTEXITCODE"
  }
  $PdfStatus = 'built'
  $PdfReason = ''
}

[ordered]@{
  schema = 'hvac-studio.manual-build.v1'
  version = $ResolvedVersion
  markdown = $ManualPath
  pdf = $PdfPath
  pdf_status = $PdfStatus
  pdf_reason = $PdfReason
} | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath $StatusPath -Encoding UTF8

Write-Host "manual markdown: $ManualPath"
if ($PdfStatus -eq 'built') {
  Write-Host "manual pdf: $PdfPath"
} else {
  Write-Host "manual pdf skipped: $PdfReason"
}
Write-Output $ManualPath
