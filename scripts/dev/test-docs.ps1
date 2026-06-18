$ErrorActionPreference = 'Stop'

. (Join-Path $PSScriptRoot 'env.ps1')
. (Join-Path $PSScriptRoot '..\release\package-common.ps1')

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$SiteRoot = Join-Path $RepoRoot 'dist\docs\ci-site'

function Test-StudioHelpLinks {
  $StaticRoot = Join-Path $RepoRoot 'tools\go\internal\studio\static'
  $Files = Get-ChildItem -LiteralPath $StaticRoot -Recurse -File |
    Where-Object { $_.Extension -in @('.html', '.js') }
  $Pattern = [regex]'/docs/user/[A-Za-z0-9._/-]+\.md'
  $Missing = New-Object System.Collections.Generic.List[string]

  foreach ($File in $Files) {
    $Text = Get-Content -Raw -Encoding UTF8 -LiteralPath $File.FullName
    foreach ($Match in $Pattern.Matches($Text)) {
      $TargetRelative = $Match.Value.TrimStart('/').Replace('/', '\')
      $Target = Join-Path $RepoRoot $TargetRelative
      if (-not (Test-Path -LiteralPath $Target)) {
        $Missing.Add("$($File.FullName): $($Match.Value)")
      }
    }
  }

  if ($Missing.Count -gt 0) {
    throw "Studio help links reference missing docs:`n$($Missing -join "`n")"
  }

  Write-Host 'studio help links ok'
}

Remove-Item -LiteralPath $SiteRoot -Recurse -Force -ErrorAction SilentlyContinue
Invoke-MkDocsBuild -RepoRoot $RepoRoot -SiteRoot $SiteRoot
Test-StudioHelpLinks
Write-Host "docs html ok: $SiteRoot"
