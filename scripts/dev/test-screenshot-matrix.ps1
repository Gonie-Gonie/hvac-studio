param(
  [string]$OutputRoot = ''
)

$ErrorActionPreference = 'Stop'

. (Join-Path $PSScriptRoot 'env.ps1')

if (-not $env:HVAC_STUDIO_GO) {
  throw 'go was not found. Run scripts/dev/setup.ps1 first.'
}

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
if (-not $OutputRoot) {
  $OutputRoot = Join-Path $RepoRoot 'artifacts\screenshot-matrix\latest'
}
$OutputRoot = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($OutputRoot)
$LogRoot = Join-Path $OutputRoot 'logs'
$BrowserProfile = Join-Path $OutputRoot 'browser-profile'
$FixtureRoot = Join-Path $OutputRoot 'fixture-root'

function Get-FreeTcpPort {
  $Listener = [Net.Sockets.TcpListener]::new([Net.IPAddress]::Parse('127.0.0.1'), 0)
  try {
    $Listener.Start()
    return $Listener.LocalEndpoint.Port
  } finally {
    $Listener.Stop()
  }
}

function Resolve-HeadlessBrowser {
  $Candidates = @(
    $env:HVAC_STUDIO_BROWSER,
    'C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe',
    'C:\Program Files\Microsoft\Edge\Application\msedge.exe',
    'C:\Program Files\Google\Chrome\Application\chrome.exe',
    'C:\Program Files (x86)\Google\Chrome\Application\chrome.exe'
  ) | Where-Object { $_ }

  foreach ($Candidate in $Candidates) {
    if (Test-Path -LiteralPath $Candidate) {
      return (Resolve-Path -LiteralPath $Candidate).Path
    }
  }

  foreach ($Name in @('msedge', 'chrome', 'chrome.exe', 'msedge.exe')) {
    $Command = Get-Command $Name -ErrorAction SilentlyContinue
    if ($null -ne $Command) {
      return $Command.Source
    }
  }

  throw 'No supported headless browser found. Install Microsoft Edge or Google Chrome, or set HVAC_STUDIO_BROWSER.'
}

function Wait-ForStudio {
  param([Parameter(Mandatory = $true)][string]$Url)

  $Deadline = (Get-Date).AddSeconds(45)
  do {
    try {
      $Response = Invoke-WebRequest -UseBasicParsing -Uri $Url -TimeoutSec 2
      if ($Response.StatusCode -eq 200) {
        return
      }
    } catch {
      Start-Sleep -Milliseconds 500
    }
  } while ((Get-Date) -lt $Deadline)

  throw "Studio server did not become ready: $Url"
}

function Assert-StaticToken {
  param(
    [Parameter(Mandatory = $true)][string]$Text,
    [Parameter(Mandatory = $true)][string]$Token,
    [Parameter(Mandatory = $true)][string]$Label
  )

  if (-not $Text.Contains($Token)) {
    throw "screenshot matrix static coverage missing $Label token: $Token"
  }
}

function Assert-Png {
  param(
    [Parameter(Mandatory = $true)][string]$Path,
    [Parameter(Mandatory = $true)][string]$Name
  )

  if (-not (Test-Path -LiteralPath $Path)) {
    throw "screenshot was not written for ${Name}: $Path"
  }
  $Info = Get-Item -LiteralPath $Path
  if ($Info.Length -lt 10000) {
    throw "screenshot looks too small for ${Name}: $($Info.Length) bytes"
  }
  $Bytes = [IO.File]::ReadAllBytes($Path)
  $PngMagic = @(137, 80, 78, 71, 13, 10, 26, 10)
  for ($Index = 0; $Index -lt $PngMagic.Count; $Index++) {
    if ($Bytes[$Index] -ne $PngMagic[$Index]) {
      throw "screenshot is not a PNG for ${Name}: $Path"
    }
  }
}

function Assert-ScreenshotStaticCoverage {
  $StaticFiles = @(
    'tools\go\internal\studio\static\index.html',
    'tools\go\internal\studio\static\styles.css',
    'tools\go\internal\studio\static\js\app.js',
    'tools\go\internal\studio\static\js\run-output.js',
    'tools\go\internal\studio\static\js\export-workspace.js'
  )
  $StaticText = ($StaticFiles | ForEach-Object {
    Get-Content -Raw -Encoding UTF8 -LiteralPath (Join-Path $RepoRoot $_)
  }) -join "`n"

  $Coverage = @(
    @{ Label = 'Start'; Tokens = @('id="startView"', 'renderStartWorkspace', 'startRuntimeRows') },
    @{ Label = 'System Canvas'; Tokens = @('id="canvasView"', 'systemCanvas', 'renderCanvas') },
    @{ Label = 'Inspector'; Tokens = @('right-sidebar', 'id="inspector"', 'renderInspector') },
    @{ Label = 'Code Editor'; Tokens = @('id="codeView"', 'sourceEditor', 'sourceLineProblemMap') },
    @{ Label = 'Run'; Tokens = @('id="runView"', 'renderRunWorkspace', 'runOutputRows') },
    @{ Label = 'Data'; Tokens = @('dataValidateButton', 'datasetSourcePathInput', 'runDataValidation') },
    @{ Label = 'Parameters'; Tokens = @('id="parametersView"', 'renderParameters', 'parameterRows') },
    @{ Label = 'Calibration'; Tokens = @('createCalibrationSetupButton', 'calibrationSetupEditorSection', 'runCalibrationSetup') },
    @{ Label = 'Optimization'; Tokens = @('createOptimizationSetupButton', 'optimizationSetupEditorSection', 'runOptimizationSetup') },
    @{ Label = 'Export'; Tokens = @('id="exportView"', 'renderExportWorkspace', 'exportManifest') },
    @{ Label = 'Artifacts'; Tokens = @('id="artifactsView"', 'renderArtifactWorkspace', 'artifactRows') },
    @{ Label = 'Error State'; Tokens = @('latestValidation = { error:', 'problemRow', 'case-error') },
    @{ Label = 'Empty State'; Tokens = @('empty-cell', 'emptyRow(', 'No outputs yet') },
    @{ Label = 'Busy State'; Tokens = @('activeRunAbortController', 'pendingRunSummaryRows', 'cancelRunButton') }
  )

  foreach ($Item in $Coverage) {
    foreach ($Token in $Item.Tokens) {
      Assert-StaticToken -Text $StaticText -Token $Token -Label $Item.Label
    }
  }
}

Remove-Item -LiteralPath $OutputRoot -Recurse -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path $OutputRoot, $LogRoot, $BrowserProfile | Out-Null
New-Item -ItemType Directory -Force -Path $FixtureRoot | Out-Null
foreach ($Directory in @('examples', 'templates', 'docs')) {
  Copy-Item -LiteralPath (Join-Path $RepoRoot $Directory) -Destination (Join-Path $FixtureRoot $Directory) -Recurse -Force
}

Assert-ScreenshotStaticCoverage

$Browser = Resolve-HeadlessBrowser
$Port = Get-FreeTcpPort
$Addr = "127.0.0.1:$Port"
$BaseUrl = "http://$Addr"
$StdoutLog = Join-Path $LogRoot 'studio.out.log'
$StderrLog = Join-Path $LogRoot 'studio.err.log'
$StudioProcess = $null

try {
  $StudioProcess = Start-Process `
    -FilePath $env:HVAC_STUDIO_GO `
    -WindowStyle Hidden `
    -WorkingDirectory (Join-Path $RepoRoot 'tools\go') `
    -PassThru `
    -RedirectStandardOutput $StdoutLog `
    -RedirectStandardError $StderrLog `
    -ArgumentList @('run', '.\cmd\studio', '--server', '--repo', $FixtureRoot, '--addr', $Addr)

  Wait-ForStudio -Url $BaseUrl

  $Screenshots = @(
    @{ Name = 'start'; Mode = 'start' },
    @{ Name = 'system-canvas'; Mode = 'canvas' },
    @{ Name = 'inspector'; Mode = 'canvas' },
    @{ Name = 'code-editor'; Mode = 'code' },
    @{ Name = 'run'; Mode = 'run' },
    @{ Name = 'data'; Mode = 'artifacts' },
    @{ Name = 'parameters'; Mode = 'parameters' },
    @{ Name = 'calibration'; Mode = 'artifacts' },
    @{ Name = 'optimization'; Mode = 'artifacts' },
    @{ Name = 'export'; Mode = 'export' },
    @{ Name = 'artifacts'; Mode = 'artifacts' },
    @{ Name = 'error-state'; Mode = 'code' },
    @{ Name = 'empty-state'; Mode = 'run' },
    @{ Name = 'busy-state'; Mode = 'run' }
  )

  foreach ($Shot in $Screenshots) {
    $Path = Join-Path $OutputRoot ($Shot.Name + '.png')
    $BrowserOutLog = Join-Path $LogRoot ($Shot.Name + '.browser.out.log')
    $BrowserErrLog = Join-Path $LogRoot ($Shot.Name + '.browser.err.log')
    $Url = "$BaseUrl/#$($Shot.Mode)"
    $Arguments = @(
      '--headless',
      '--disable-gpu',
      '--no-first-run',
      '--no-default-browser-check',
      "--user-data-dir=$BrowserProfile",
      '--window-size=1440,920',
      '--virtual-time-budget=3000',
      "--screenshot=$Path",
      $Url
    )
    $BrowserProcess = Start-Process `
      -FilePath $Browser `
      -WindowStyle Hidden `
      -PassThru `
      -Wait `
      -RedirectStandardOutput $BrowserOutLog `
      -RedirectStandardError $BrowserErrLog `
      -ArgumentList $Arguments
    if ($BrowserProcess.ExitCode -ne 0) {
      throw "headless browser screenshot failed for $($Shot.Name) with exit code $($BrowserProcess.ExitCode)"
    }
    Assert-Png -Path $Path -Name $Shot.Name
  }

  $UniqueHashes = @(Get-ChildItem -LiteralPath $OutputRoot -Filter '*.png' |
    Get-FileHash -Algorithm SHA256 |
    Select-Object -ExpandProperty Hash -Unique)
  if ($UniqueHashes.Count -lt 6) {
    throw "screenshot matrix did not capture distinct workspace states; unique screenshots=$($UniqueHashes.Count)"
  }
} finally {
  if ($null -ne $StudioProcess -and -not $StudioProcess.HasExited) {
    Stop-Process -Id $StudioProcess.Id -Force -ErrorAction SilentlyContinue
  }
}

Write-Host "screenshot matrix ok: $OutputRoot"
