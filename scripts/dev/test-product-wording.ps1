$ErrorActionPreference = 'Stop'

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$Forbidden = @(
  @{ Label = 'planned'; Pattern = '\bplanned\b' },
  @{ Label = 'not implemented'; Pattern = '\bnot\s+implemented\b' },
  @{ Label = 'mock'; Pattern = '\bmock\b' },
  @{ Label = 'demo'; Pattern = '\bdemo\b' },
  @{ Label = 'placeholder'; Pattern = '\bplaceholder\b' },
  @{ Label = 'future'; Pattern = '\bfuture\b' },
  @{ Label = 'dev only'; Pattern = '\bdev\s+only\b' }
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
  'scripts\release'
)

$AllowedExtensions = @('.md', '.txt', '.ps1', '.cmd', '.json')
$ScriptPath = $MyInvocation.MyCommand.Path
$Files = New-Object System.Collections.Generic.List[string]

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
      if ($Lines[$Index] -match $Rule.Pattern) {
        $RelPath = [System.IO.Path]::GetRelativePath($RepoRoot, $File)
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
