param(
  [string]$OutputRoot = ''
)

$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'env.ps1')

if (-not $env:HVAC_STUDIO_GO -or -not $env:HVAC_STUDIO_PYTHON) {
  throw 'Go and Python are required. Run scripts/dev/setup.ps1 first.'
}
& $env:HVAC_STUDIO_PYTHON -c "import importlib.util, sys; sys.exit(0 if importlib.util.find_spec('playwright') else 1)"
if ($LASTEXITCODE -ne 0) {
  throw 'Playwright is required. Run .toolchain/uv/uv.exe pip install --python .venv/Scripts/python.exe playwright. This test uses installed Edge/Chrome; no browser download is needed.'
}

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$TempRoot = [IO.Path]::GetFullPath((Join-Path $RepoRoot '.tmp'))
if (-not $OutputRoot) {
  $OutputRoot = Join-Path $TempRoot 'ui-review'
}
$OutputRoot = [IO.Path]::GetFullPath($ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($OutputRoot))
if (-not $OutputRoot.StartsWith($TempRoot + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) {
  throw "UI test output must be a subdirectory of $TempRoot"
}
if (Test-Path -LiteralPath $OutputRoot) {
  $ResolvedOutputRoot = (Resolve-Path -LiteralPath $OutputRoot).Path
  if (-not $ResolvedOutputRoot.StartsWith($TempRoot + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) {
    throw "Refusing to clear output outside $TempRoot"
  }
  Remove-Item -LiteralPath $ResolvedOutputRoot -Recurse -Force
}
New-Item -ItemType Directory -Force -Path $OutputRoot | Out-Null
$FixtureRoot = Join-Path $OutputRoot 'fixture-root'
New-Item -ItemType Directory -Force -Path $FixtureRoot | Out-Null
foreach ($Directory in @('examples', 'templates', 'docs')) {
  Copy-Item -LiteralPath (Join-Path $RepoRoot $Directory) -Destination (Join-Path $FixtureRoot $Directory) -Recurse -Force
}

$Browser = @(
  $env:HVAC_STUDIO_BROWSER,
  'C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe',
  'C:\Program Files\Microsoft\Edge\Application\msedge.exe',
  'C:\Program Files\Google\Chrome\Application\chrome.exe',
  'C:\Program Files (x86)\Google\Chrome\Application\chrome.exe'
) | Where-Object { $_ -and (Test-Path -LiteralPath $_) } | Select-Object -First 1
if (-not $Browser) {
  throw 'Install Edge/Chrome or set HVAC_STUDIO_BROWSER to the browser executable.'
}

$StudioExe = Join-Path $OutputRoot 'studio-ui-test.exe'
Push-Location (Join-Path $RepoRoot 'go')
try {
  Invoke-Checked $env:HVAC_STUDIO_GO @('build', '-o', $StudioExe, '.\cmd\studio')
} finally {
  Pop-Location
}
$Listener = [Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback, 0)
try {
  $Listener.Start()
  $Port = $Listener.LocalEndpoint.Port
} finally {
  $Listener.Stop()
}
$Addr = "127.0.0.1:$Port"
$BaseUrl = "http://$Addr"
$StdoutLog = Join-Path $OutputRoot 'studio.out.log'
$StderrLog = Join-Path $OutputRoot 'studio.err.log'
$StudioProcess = $null
try {
  $StudioProcess = Start-Process -FilePath $StudioExe -WindowStyle Hidden -PassThru `
    -WorkingDirectory $RepoRoot `
    -RedirectStandardOutput $StdoutLog -RedirectStandardError $StderrLog `
    -ArgumentList @('--server', '--repo', ('"' + $FixtureRoot + '"'), '--addr', $Addr)
  $Ready = $false
  $Deadline = (Get-Date).AddSeconds(45)
  do {
    if ($StudioProcess.HasExited) {
      throw "Studio exited before becoming ready. See $StderrLog"
    }
    try {
      $Response = Invoke-WebRequest -UseBasicParsing -Uri $BaseUrl -TimeoutSec 2
      $Ready = $Response.StatusCode -eq 200
    } catch {
      Start-Sleep -Milliseconds 250
    }
  } while (-not $Ready -and (Get-Date) -lt $Deadline)
  if (-not $Ready) {
    throw "Studio did not become ready: $BaseUrl"
  }
  Invoke-Checked $env:HVAC_STUDIO_PYTHON @(
    (Join-Path $PSScriptRoot 'test-studio-ui.py'),
    '--url', $BaseUrl, '--browser', $Browser,
    '--repo', $RepoRoot, '--fixture', $FixtureRoot, '--output', $OutputRoot
  )
} finally {
  if ($null -ne $StudioProcess -and -not $StudioProcess.HasExited) {
    Stop-Process -Id $StudioProcess.Id -Force -ErrorAction SilentlyContinue
  }
}
if ((Test-Path -LiteralPath $StderrLog) -and (Get-Item -LiteralPath $StderrLog).Length -gt 0) {
  throw "Studio wrote unexpected stderr. See $StderrLog"
}
Write-Host "Studio UI regression passed: $OutputRoot"
