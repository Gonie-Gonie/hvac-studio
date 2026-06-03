$ErrorActionPreference = 'Stop'

$Root = Split-Path -Parent $PSScriptRoot
& (Join-Path $Root 'bin\bcs-runner.exe') run `
  --project (Join-Path $Root 'model\project.bcsproj') `
  --input (Join-Path $PSScriptRoot 'input.json') `
  --output (Join-Path $Root 'outputs\result.json')
