$ErrorActionPreference = 'Stop'

. (Join-Path $PSScriptRoot 'env.ps1')
. (Join-Path $PSScriptRoot '..\release\package-common.ps1')

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$SiteRoot = [IO.Path]::GetFullPath((Join-Path $RepoRoot '.tmp\docs\ci-site'))

function Test-StudioHelpLinks {
  $StaticRoot = Join-Path $RepoRoot 'go\internal\studio\static'
  $Files = Get-ChildItem -LiteralPath $StaticRoot -Recurse -File |
    Where-Object { $_.Extension -in @('.html', '.js') }
  $Pattern = [regex]'/docs/user/[A-Za-z0-9._/-]+\.md'
  $Missing = New-Object System.Collections.Generic.List[string]

  foreach ($File in $Files) {
    $Text = Get-Content -Raw -Encoding UTF8 -LiteralPath $File.FullName
    foreach ($Match in $Pattern.Matches($Text)) {
      $TargetRelative = $Match.Value.TrimStart('/').Replace('/', '\')
      if (-not (Test-Path -LiteralPath (Join-Path $RepoRoot $TargetRelative))) {
        $Missing.Add("$($File.FullName): $($Match.Value)")
      }
    }
  }

  if ($Missing.Count -gt 0) {
    throw "Studio help links reference missing docs:`n$($Missing -join "`n")"
  }
  Write-Host 'studio help links ok'
}

function Test-DocumentationCoverage {
  $MkDocsConfig = Get-Content -Raw -Encoding UTF8 -LiteralPath (Join-Path $RepoRoot 'mkdocs.yml')
  $ManualScript = Get-Content -Raw -Encoding UTF8 -LiteralPath (Join-Path $RepoRoot 'scripts\release\build-docs-manual.ps1')
  $Pattern = [regex]'[A-Za-z0-9._/-]+\.md'
  $NavPages = New-Object System.Collections.Generic.HashSet[string]
  $Missing = New-Object System.Collections.Generic.List[string]

  foreach ($Match in $Pattern.Matches($MkDocsConfig)) {
    $Page = $Match.Value.Replace('/', '\')
    [void]$NavPages.Add($Page)
    $Source = 'docs\' + $Page
    if (-not $ManualScript.Contains($Source)) {
      $Missing.Add("manual source: $Source")
    }
  }

  $DocsRoot = Join-Path $RepoRoot 'docs'
  Get-ChildItem -LiteralPath $DocsRoot -Recurse -File -Filter '*.md' |
    ForEach-Object {
      $Page = $_.FullName.Substring($DocsRoot.Length + 1)
      if (-not $NavPages.Contains($Page)) {
        $Missing.Add("navigation: docs\$Page")
      }
    }

  if ($Missing.Count -gt 0) {
    throw "documentation coverage is incomplete:`n$($Missing -join "`n")"
  }
  Write-Host 'documentation navigation and manual coverage ok'
}

$TemporaryRoot = [IO.Path]::GetFullPath((Join-Path $RepoRoot '.tmp')) + '\'
if (-not $SiteRoot.StartsWith($TemporaryRoot, [StringComparison]::OrdinalIgnoreCase)) {
  throw "docs output must stay inside the repository temporary directory: $SiteRoot"
}
if (Test-Path -LiteralPath $SiteRoot) {
  Remove-Item -LiteralPath $SiteRoot -Recurse -Force
}
Invoke-MkDocsBuild -RepoRoot $RepoRoot -SiteRoot $SiteRoot
Test-StudioHelpLinks
Test-DocumentationCoverage
Write-Host "docs html ok: $SiteRoot"