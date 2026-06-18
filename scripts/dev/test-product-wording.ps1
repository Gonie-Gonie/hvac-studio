$ErrorActionPreference = 'Stop'

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$Forbidden = @(
  @{ Label = 'planned'; Pattern = '\bplanned\b' },
  @{ Label = 'not implemented'; Pattern = '\bnot\s+implemented\b' },
  @{ Label = 'mock'; Pattern = '\bmock\b' },
  @{ Label = 'demo'; Pattern = '\bdemo\b' },
  @{ Label = 'placeholder'; Pattern = '\bplaceholder\b' },
  @{ Label = 'future'; Pattern = '\bfuture\b' },
  @{ Label = 'dev only'; Pattern = '\bdev[\s_-]+only\b' },
  @{ Label = 'legacy milestone abbreviation'; Pattern = ('\b' + 'M' + 'VP\b') },
  @{ Label = 'legacy post milestone wording'; Pattern = ('\bpost-' + 'M' + 'VP\b') },
  @{ Label = 'old alpha baseline 3'; Pattern = ('\balpha' + '\.3\b') },
  @{ Label = 'old alpha baseline 5'; Pattern = ('\balpha' + '\.5\b') },
  @{ Label = 'component-as-node wording'; Pattern = '\b(chiller|pump|ahu|zone|controller|equipment|plant|model)\s+nodes?\b|\bnodes?\s+(chiller|pump|ahu|zone|controller|equipment|plant|model)\b' }
)

$SurfaceRoots = @(
  'README.md',
  'CHANGELOG.md',
  'docs\release.md',
  'docs\release-trust.md',
  'docs\setup.md',
  'docs\user',
  'docs\legal',
  'examples',
  'templates\README.md',
  'runtime',
  'scripts\dev',
  'scripts\release',
  'tools\go\internal\studio\static'
)

$AllowedExtensions = @('.md', '.txt', '.ps1', '.cmd', '.py', '.json', '.html', '.js', '.css')
$ScriptPath = $MyInvocation.MyCommand.Path
$Files = New-Object System.Collections.Generic.List[string]

function Test-AllowedPrototypeWordContext {
  param(
    [Parameter(Mandatory = $true)][string]$Label,
    [Parameter(Mandatory = $true)][string]$Line
  )

  if ($Label -eq 'placeholder') {
    return $Line -match '\bplaceholder\s*=' -or
      $Line -match '\.placeholder\b' -or
      $Line -match '\bplaceholder\s*;' -or
      $Line -match '\bplaceholder\b\s*[,)]' -or
      $Line -match '\bfunction\s+\w+\([^)]*\bplaceholder\b'
  }

  if ($Label -eq 'future') {
    return $Line -match '^\s*from\s+__future__\s+import\b'
  }

  return $false
}

function ConvertTo-RepoRelativePath {
  param([Parameter(Mandatory = $true)][string]$Path)

  $FullPath = (Resolve-Path -LiteralPath $Path).Path
  $Prefix = $RepoRoot.TrimEnd('\') + '\'
  if ($FullPath.StartsWith($Prefix, [System.StringComparison]::OrdinalIgnoreCase)) {
    return $FullPath.Substring($Prefix.Length)
  }
  return $FullPath
}

foreach ($Rel in $SurfaceRoots) {
  $Path = Join-Path $RepoRoot $Rel
  if (-not (Test-Path -LiteralPath $Path)) {
    continue
  }
  $Item = Get-Item -LiteralPath $Path
  if (-not $Item.PSIsContainer) {
    if ($AllowedExtensions -contains $Item.Extension.ToLowerInvariant()) {
      $Files.Add($Item.FullName)
    }
    continue
  }
  Get-ChildItem -LiteralPath $Item.FullName -Recurse -File |
    Where-Object {
      ($AllowedExtensions -contains $_.Extension.ToLowerInvariant()) -and
      ($_.FullName -ne $ScriptPath)
    } |
    ForEach-Object { $Files.Add($_.FullName) }
}

$Findings = New-Object System.Collections.Generic.List[string]
foreach ($File in ($Files | Sort-Object -Unique)) {
  $Lines = Get-Content -LiteralPath $File -Encoding UTF8
  for ($Index = 0; $Index -lt $Lines.Count; $Index++) {
    foreach ($Rule in $Forbidden) {
      if (($Lines[$Index] -match $Rule.Pattern) -and -not (Test-AllowedPrototypeWordContext -Label $Rule.Label -Line $Lines[$Index])) {
        $RelPath = ConvertTo-RepoRelativePath -Path $File
        $Findings.Add(("{0}:{1}: product-facing wording '{2}' -> {3}" -f $RelPath, ($Index + 1), $Rule.Label, $Lines[$Index].Trim()))
      }
    }
  }
}

if ($Findings.Count -gt 0) {
  $Findings | ForEach-Object { Write-Error $_ }
  throw "product wording check failed with $($Findings.Count) finding(s)"
}

Write-Host 'product wording ok'
