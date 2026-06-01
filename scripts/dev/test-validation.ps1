$ErrorActionPreference = 'Stop'

. (Join-Path $PSScriptRoot 'env.ps1')

if (-not $env:HVAC_STUDIO_GO) {
  throw 'go was not found. Run scripts/dev/setup.ps1 first.'
}

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$ValidationRoot = Join-Path $RepoRoot 'tests\golden\validation'

function Join-ProcessArguments {
  param([Parameter(Mandatory = $true)][string[]]$Arguments)

  $Escaped = foreach ($Argument in $Arguments) {
    if ($Argument -match '[\s"]') {
      '"' + ($Argument -replace '"', '\"') + '"'
    } else {
      $Argument
    }
  }
  return ($Escaped -join ' ')
}

function Invoke-CapturedProcess {
  param(
    [Parameter(Mandatory = $true)][string]$FilePath,
    [Parameter(Mandatory = $true)][string[]]$Arguments,
    [Parameter(Mandatory = $true)][string]$WorkingDirectory
  )

  $StartInfo = New-Object System.Diagnostics.ProcessStartInfo
  $StartInfo.FileName = $FilePath
  $StartInfo.Arguments = Join-ProcessArguments -Arguments $Arguments
  $StartInfo.WorkingDirectory = $WorkingDirectory
  $StartInfo.UseShellExecute = $false
  $StartInfo.RedirectStandardOutput = $true
  $StartInfo.RedirectStandardError = $true

  $Process = [System.Diagnostics.Process]::Start($StartInfo)
  $Stdout = $Process.StandardOutput.ReadToEnd()
  $Stderr = $Process.StandardError.ReadToEnd()
  $Process.WaitForExit()

  return [pscustomobject]@{
    ExitCode = $Process.ExitCode
    Output = $Stdout + $Stderr
  }
}

$Cases = Get-ChildItem -LiteralPath $ValidationRoot -Directory | Sort-Object FullName
if ($Cases.Count -eq 0) {
  throw "no validation golden cases found under $ValidationRoot"
}

foreach ($Case in $Cases) {
  $Project = Join-Path $Case.FullName 'project.bcsproj'
  $ExpectedExitCodePath = Join-Path $Case.FullName 'expected\exit_code.txt'
  $ExpectedContainsPath = Join-Path $Case.FullName 'expected\contains.txt'

  if (-not (Test-Path -LiteralPath $Project)) {
    throw "$($Case.Name) is missing project.bcsproj"
  }
  if (-not (Test-Path -LiteralPath $ExpectedExitCodePath)) {
    throw "$($Case.Name) is missing expected/exit_code.txt"
  }
  if (-not (Test-Path -LiteralPath $ExpectedContainsPath)) {
    throw "$($Case.Name) is missing expected/contains.txt"
  }

  Write-Host "validation golden: $($Case.Name)"
  $Result = Invoke-CapturedProcess `
    -FilePath $env:HVAC_STUDIO_GO `
    -Arguments @('run', '.\cmd\bcs-runner', 'validate', '--project', $Project) `
    -WorkingDirectory (Join-Path $RepoRoot 'tools\go')

  $ExpectedExitCode = [int]((Get-Content -Raw -LiteralPath $ExpectedExitCodePath).Trim())
  if ($Result.ExitCode -ne $ExpectedExitCode) {
    $OutputText = $Result.Output.Trim()
    throw "$($Case.Name) expected exit code $ExpectedExitCode, got $($Result.ExitCode). Output: $OutputText"
  }

  $CombinedOutput = $Result.Output
  $ExpectedLines = Get-Content -LiteralPath $ExpectedContainsPath | Where-Object { $_.Trim() -ne '' }
  foreach ($ExpectedLine in $ExpectedLines) {
    if (-not $CombinedOutput.Contains($ExpectedLine)) {
      throw "$($Case.Name) expected output to contain: $ExpectedLine`nActual output:`n$CombinedOutput"
    }
  }
}

Write-Host "validation golden tests ok: $($Cases.Count)"
