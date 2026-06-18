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
  'docs\user\tutorials.md',
  'docs\user\examples.md',
  'docs\user\core-concepts.md',
  'docs\user\concept-map.md',
  'docs\user\how-it-works.md',
  'docs\user\glossary.md',
  'docs\user\create-component.md',
  'docs\user\edit-python-function.md',
  'docs\user\build-system.md',
  'docs\user\parameter-management.md',
  'docs\user\model-replacement.md',
  'docs\user\ml-ann-component.md',
  'docs\user\external-executables.md',
  'docs\user\run-simulation.md',
  'docs\user\data-validation.md',
  'docs\user\calibration.md',
  'docs\user\optimization.md',
  'docs\user\export-runtime.md',
  'docs\user\artifact-compatibility.md',
  'docs\user\troubleshooting.md',
  'docs\user\python-sdk.md',
  'docs\user\cli-runner.md',
  'docs\user\external-engine-protocol.md',
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

function Write-PlainTextPdf {
  param(
    [Parameter(Mandatory = $true)][string]$Path,
    [Parameter(Mandatory = $true)][string]$Title,
    [string[]]$Lines
  )

  $Pages = New-Object System.Collections.Generic.List[object]
  $Current = New-Object System.Collections.Generic.List[string]
  foreach ($Line in $Lines) {
    if ($Line -like '<div style="page-break-after:*') {
      if ($Current.Count -gt 0) {
        $Pages.Add(@($Current))
        $Current = New-Object System.Collections.Generic.List[string]
      }
      continue
    }
    foreach ($Wrapped in (Split-PdfLine -Line $Line -Width 92)) {
      $Current.Add($Wrapped)
      if ($Current.Count -ge 58) {
        $Pages.Add(@($Current))
        $Current = New-Object System.Collections.Generic.List[string]
      }
    }
  }
  if ($Current.Count -gt 0 -or $Pages.Count -eq 0) {
    $Pages.Add(@($Current))
  }

  $Objects = New-Object System.Collections.Generic.List[string]
  $PageObjectIDs = New-Object System.Collections.Generic.List[int]
  $Objects.Add('<< /Type /Catalog /Pages 2 0 R >>')
  $Objects.Add('__PAGES__')
  $Objects.Add('<< /Type /Font /Subtype /Type1 /BaseFont /Courier >>')

  for ($PageIndex = 0; $PageIndex -lt $Pages.Count; $PageIndex++) {
    $PageObjectID = $Objects.Count + 1
    $ContentObjectID = $PageObjectID + 1
    $PageObjectIDs.Add($PageObjectID)
    $Objects.Add("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 3 0 R >> >> /Contents $ContentObjectID 0 R >>")
    $Content = New-PdfContentStream -Title $Title -PageNumber ($PageIndex + 1) -PageCount $Pages.Count -Lines $Pages[$PageIndex]
    $Length = [Text.Encoding]::ASCII.GetByteCount($Content)
    $Objects.Add("<< /Length $Length >>`nstream`n$Content`nendstream")
  }

  $Kids = ($PageObjectIDs | ForEach-Object { "$_ 0 R" }) -join ' '
  $Objects[1] = "<< /Type /Pages /Kids [$Kids] /Count $($Pages.Count) >>"

  $Builder = [Text.StringBuilder]::new()
  [void]$Builder.AppendLine('%PDF-1.4')
  $Offsets = New-Object System.Collections.Generic.List[int]
  for ($Index = 0; $Index -lt $Objects.Count; $Index++) {
    $Offsets.Add([Text.Encoding]::ASCII.GetByteCount($Builder.ToString()))
    [void]$Builder.AppendLine("$($Index + 1) 0 obj")
    [void]$Builder.AppendLine($Objects[$Index])
    [void]$Builder.AppendLine('endobj')
  }
  $XrefOffset = [Text.Encoding]::ASCII.GetByteCount($Builder.ToString())
  [void]$Builder.AppendLine('xref')
  [void]$Builder.AppendLine("0 $($Objects.Count + 1)")
  [void]$Builder.AppendLine('0000000000 65535 f ')
  foreach ($Offset in $Offsets) {
    [void]$Builder.AppendLine(('{0:0000000000} 00000 n ' -f $Offset))
  }
  [void]$Builder.AppendLine('trailer')
  [void]$Builder.AppendLine("<< /Size $($Objects.Count + 1) /Root 1 0 R >>")
  [void]$Builder.AppendLine('startxref')
  [void]$Builder.AppendLine("$XrefOffset")
  [void]$Builder.AppendLine('%%EOF')

  [IO.File]::WriteAllBytes($Path, [Text.Encoding]::ASCII.GetBytes($Builder.ToString()))
}

function Split-PdfLine {
  param(
    [string]$Line,
    [int]$Width = 92
  )

  $Text = ($Line -replace "`t", '    ')
  if ($null -eq $Text -or $Text.Length -eq 0) {
    return @('')
  }
  $Parts = New-Object System.Collections.Generic.List[string]
  while ($Text.Length -gt $Width) {
    $Break = $Text.LastIndexOf(' ', [Math]::Min($Width, $Text.Length - 1))
    if ($Break -lt 24) {
      $Break = $Width
    }
    $Parts.Add($Text.Substring(0, $Break).TrimEnd())
    $Text = $Text.Substring($Break).TrimStart()
  }
  $Parts.Add($Text)
  return @($Parts)
}

function New-PdfContentStream {
  param(
    [Parameter(Mandatory = $true)][string]$Title,
    [Parameter(Mandatory = $true)][int]$PageNumber,
    [Parameter(Mandatory = $true)][int]$PageCount,
    [string[]]$Lines
  )

  $Content = [Text.StringBuilder]::new()
  [void]$Content.AppendLine('BT')
  [void]$Content.AppendLine('/F1 10 Tf')
  [void]$Content.AppendLine('50 760 Td')
  [void]$Content.AppendLine("($(Escape-PdfText "$Title - page $PageNumber of $PageCount")) Tj")
  [void]$Content.AppendLine('/F1 8.5 Tf')
  [void]$Content.AppendLine('0 -18 Td')
  foreach ($Line in $Lines) {
    [void]$Content.AppendLine("($(Escape-PdfText $Line)) Tj")
    [void]$Content.AppendLine('0 -12 Td')
  }
  [void]$Content.AppendLine('ET')
  return $Content.ToString().TrimEnd()
}

function Escape-PdfText {
  param([string]$Text)

  if ($null -eq $Text) {
    return ''
  }
  $AsciiChars = New-Object System.Collections.Generic.List[string]
  foreach ($Char in $Text.ToCharArray()) {
    $Code = [int][char]$Char
    if ($Code -ge 32 -and $Code -le 126) {
      $AsciiChars.Add([string]$Char)
    } else {
      $AsciiChars.Add('?')
    }
  }
  $Escaped = $AsciiChars -join ''
  $Escaped = $Escaped.Replace('\', '\\')
  $Escaped = $Escaped.Replace('(', '\(')
  $Escaped = $Escaped.Replace(')', '\)')
  return $Escaped
}

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
} else {
  Write-PlainTextPdf -Path $PdfPath -Title "HVAC Studio Manual $ResolvedVersion" -Lines $Lines
  $PdfStatus = 'built'
  $PdfReason = 'built with plain-text fallback because pandoc was not found'
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
