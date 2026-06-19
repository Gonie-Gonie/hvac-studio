package studio

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/goniegonie/hvac-studio/tools/go/internal/apperror"
	"github.com/goniegonie/hvac-studio/tools/go/internal/compiler"
	"github.com/goniegonie/hvac-studio/tools/go/internal/platform"
	"github.com/goniegonie/hvac-studio/tools/go/internal/project"
)

func writeRuntimeExportProject(loaded *project.LoadedProject, targetRoot string, options exportOptions) ([]string, error) {
	if err := resetGeneratedDir(filepath.Dir(targetRoot), targetRoot); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(targetRoot, 0o755); err != nil {
		return nil, err
	}

	files := []string{}
	seen := map[string]bool{}
	projectPath, _, err := projectOwnedRelativePath(loaded.Root, loaded.Path)
	if err != nil {
		return nil, err
	}
	graphPath, _, err := projectOwnedRelativePath(loaded.Root, loaded.GraphPath)
	if err != nil {
		return nil, err
	}
	for _, rel := range []string{projectPath, graphPath, loaded.Project.DefaultInput, loaded.Project.Environment.Lockfile} {
		if err := copyRuntimeExportFile(loaded.Root, targetRoot, rel, &files, seen); err != nil {
			return nil, err
		}
	}
	for _, rel := range []string{
		"components",
		"inputs",
		"scenarios",
		"parameter_sets",
	} {
		if err := copyRuntimeExportDir(loaded.Root, targetRoot, rel, &files, seen); err != nil {
			return nil, err
		}
	}
	if options.IncludeMLAssets {
		if err := copyRuntimeExportDir(loaded.Root, targetRoot, "assets", &files, seen); err != nil {
			return nil, err
		}
	}
	if options.IncludeDatasets {
		for _, rel := range []string{"datasets", "validation/mappings"} {
			if err := copyRuntimeExportDir(loaded.Root, targetRoot, rel, &files, seen); err != nil {
				return nil, err
			}
		}
	}
	if options.IncludeCalibrationSetups {
		if err := copyRuntimeExportDir(loaded.Root, targetRoot, "calibration/setups", &files, seen); err != nil {
			return nil, err
		}
	}
	if options.IncludeOptimizationSetups {
		if err := copyRuntimeExportDir(loaded.Root, targetRoot, "optimization/setups", &files, seen); err != nil {
			return nil, err
		}
	}
	if options.IncludeRecords {
		for _, rel := range []string{"runs", "batches", "validation/runs", "calibration/results", "optimization/results"} {
			if err := copyRuntimeExportDir(loaded.Root, targetRoot, rel, &files, seen); err != nil {
				return nil, err
			}
		}
	}
	sort.Strings(files)
	return files, nil
}

func writeRuntimeExportSupportFiles(projectRoot string, exportRoot string, options exportOptions) ([]string, error) {
	supportRoot := findRuntimeSupportRoot(projectRoot)
	if supportRoot == "" {
		return []string{}, nil
	}
	files := []string{}
	seen := map[string]bool{}
	for _, rel := range []string{"bin/bcs-runner.exe", "bin/bcs-env.exe", "runtime/manifest.json"} {
		if err := copyExternalExportFile(supportRoot, exportRoot, rel, &files, seen); err != nil {
			return nil, err
		}
	}
	for _, rel := range []string{"schema/serve-request.schema.json", "schema/serve-response.schema.json"} {
		if err := copyExternalExportFile(supportRoot, exportRoot, rel, &files, seen); err != nil {
			return nil, err
		}
	}
	if err := copyExternalExportDir(supportRoot, exportRoot, "runtime/python", &files, seen); err != nil {
		return nil, err
	}
	if options.IncludeSDKExamples {
		if err := copyExternalExportDir(supportRoot, exportRoot, "python/bcs_sdk", &files, seen); err != nil {
			return nil, err
		}
	}
	sort.Strings(files)
	return files, nil
}

type runtimeExportEntrypoint struct {
	Rel     string
	Content string
}

func runtimeExportEntrypoints(files []string, plan *compiler.Plan, projectPath string, defaultInput string, lockfile string, options exportOptions) []runtimeExportEntrypoint {
	mapping := firstProjectRelativeExport(files, "project/validation/mappings/")
	calibrationSetup := firstProjectRelativeExport(files, "project/calibration/setups/")
	optimizationSetup := firstProjectRelativeExport(files, "project/optimization/setups/")
	entrypoints := []runtimeExportEntrypoint{
		{Rel: "check-env.ps1", Content: runtimeExportCheckEnvScript()},
		{Rel: "run-default.ps1", Content: runtimeExportRunScript(projectPath, defaultInput)},
		{Rel: "run-scenario.ps1", Content: runtimeExportScenarioScript(projectPath, defaultInput)},
		{Rel: "serve.ps1", Content: runtimeExportServeScript(projectPath)},
		{Rel: "docs/CLI_Guide.md", Content: runtimeExportCLIGuide(files, plan, projectPath, defaultInput, options.IncludeSDKExamples)},
	}
	if options.IncludeSDKExamples {
		entrypoints = append(entrypoints, runtimeExportEntrypoint{Rel: "sdk-example.py", Content: runtimeExportSDKExample(projectPath, defaultInput)})
	}
	if firstProjectRelativeExport(files, "project/scenarios/") != "" {
		entrypoints = append(entrypoints, runtimeExportEntrypoint{Rel: "run-batch.ps1", Content: runtimeExportBatchScript(projectPath)})
	}
	if mapping != "" {
		entrypoints = append(entrypoints, runtimeExportEntrypoint{Rel: "validate-data.ps1", Content: runtimeExportValidationScript(projectPath, mapping)})
	}
	if calibrationSetup != "" {
		entrypoints = append(entrypoints, runtimeExportEntrypoint{Rel: "calibrate.ps1", Content: runtimeExportCalibrationScript(projectPath, calibrationSetup)})
	}
	if optimizationSetup != "" {
		entrypoints = append(entrypoints, runtimeExportEntrypoint{Rel: "optimize.ps1", Content: runtimeExportOptimizationScript(projectPath, optimizationSetup)})
		if options.IncludeSDKExamples {
			entrypoints = append(entrypoints, runtimeExportEntrypoint{Rel: "optimize-sdk.py", Content: runtimeExportOptimizationSDKExample(projectPath, optimizationSetup)})
		}
	}
	entrypoints = append([]runtimeExportEntrypoint{{Rel: "README.md", Content: runtimeExportReadme(projectPath, defaultInput, lockfile, entrypoints)}}, entrypoints...)
	return entrypoints
}

func writeRuntimeExportEntrypoints(exportRoot string, files []runtimeExportEntrypoint) ([]string, error) {
	written := []string{}
	for _, file := range files {
		path := filepath.Join(exportRoot, filepath.FromSlash(file.Rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(path, []byte(file.Content), 0o644); err != nil {
			return nil, err
		}
		written = append(written, file.Rel)
	}
	return written, nil
}

func runtimeExportRunScript(projectPath string, defaultInput string) string {
	inputLiteral := powerShellSingleQuotedPath(defaultInput)
	return strings.TrimLeft(fmt.Sprintf(`
param(
  [string]$Output = "",
  [string]$LogBundle = ""
)

%s
$DefaultInput = '%s'
$RunArgs = @('run', '--project', $Project)
if ($DefaultInput) {
  $RunArgs += @('--input', (Join-Path $Root $DefaultInput))
}
if (-not $Output) {
  $Output = Join-Path $Root 'outputs\latest.json'
} elseif (-not [IO.Path]::IsPathRooted($Output)) {
  $Output = Join-Path $Root $Output
}
$OutputDir = Split-Path -Parent $Output
if ($OutputDir) {
  New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null
}
& $Runner validate --project $Project
& $Runner @RunArgs --output $Output
Write-RunLogBundle -ResultPath $Output -LogPath $LogBundle
Write-Host "wrote $Output"
`, runtimeExportScriptPreamble(projectPath), inputLiteral), "\r\n")
}

func runtimeExportCheckEnvScript() string {
	return strings.TrimLeft(`
param(
  [switch]$Json
)

$ErrorActionPreference = 'Stop'
$Root = Split-Path -Parent $MyInvocation.MyCommand.Path
$EnvTool = Join-Path $Root 'bin\bcs-env.exe'
if (-not (Test-Path -LiteralPath $EnvTool)) {
  $EnvTool = 'bcs-env.exe'
}
$PythonRoot = Join-Path $Root 'runtime\python'
if (Test-Path -LiteralPath $PythonRoot) {
  $env:PATH = (@($PythonRoot, (Join-Path $Root 'bin'), $env:PATH) | Where-Object { $_ }) -join [IO.Path]::PathSeparator
}
$Args = @('check', '--root', $Root)
if ($Json) {
  $Args += '--json'
}
& $EnvTool @Args
`, "\r\n")
}

func runtimeExportScenarioScript(projectPath string, defaultInput string) string {
	inputLiteral := powerShellSingleQuotedPath(defaultInput)
	return strings.TrimLeft(fmt.Sprintf(`
param(
  [string]$Input = '%s',
  [string]$Output = "",
  [string]$ParameterSet = "",
  [string]$LogBundle = ""
)

%s
if (-not $Input) {
  throw 'Input is required because this project has no default input.'
} elseif (-not [IO.Path]::IsPathRooted($Input)) {
  $Input = Join-Path $Root $Input
}
if (-not $Output) {
  $Output = Join-Path $Root 'outputs\scenario-result.json'
} elseif (-not [IO.Path]::IsPathRooted($Output)) {
  $Output = Join-Path $Root $Output
}
$OutputDir = Split-Path -Parent $Output
if ($OutputDir) {
  New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null
}
$RunArgs = @('run', '--project', $Project, '--input', $Input, '--output', $Output)
if ($ParameterSet) {
  $RunArgs += @('--parameter-set', $ParameterSet)
}
& $Runner validate --project $Project
& $Runner @RunArgs
Write-RunLogBundle -ResultPath $Output -LogPath $LogBundle
Write-Host "wrote $Output"
`, inputLiteral, runtimeExportScriptPreamble(projectPath)), "\r\n")
}

func runtimeExportBatchScript(projectPath string) string {
	return strings.TrimLeft(fmt.Sprintf(`
param(
  [string]$ScenarioDir = "",
  [string]$OutputDir = "",
  [string]$ParameterSet = ""
)

%s
if (-not $ScenarioDir) {
  $ScenarioDir = Join-Path $Root 'project\scenarios'
} elseif (-not [IO.Path]::IsPathRooted($ScenarioDir)) {
  $ScenarioDir = Join-Path $Root $ScenarioDir
}
if (-not $OutputDir) {
  $OutputDir = Join-Path $Root 'outputs\batch'
} elseif (-not [IO.Path]::IsPathRooted($OutputDir)) {
  $OutputDir = Join-Path $Root $OutputDir
}
New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null
$RunArgs = @('run', '--project', $Project)
if ($ParameterSet) {
  $RunArgs += @('--parameter-set', $ParameterSet)
}
Get-ChildItem -LiteralPath $ScenarioDir -Filter '*.json' | Sort-Object Name | ForEach-Object {
  $Output = Join-Path $OutputDir ($_.BaseName + '.json')
  & $Runner @RunArgs --input $_.FullName --output $Output
  Write-RunLogBundle -ResultPath $Output
  Write-Host "wrote $Output"
}
`, runtimeExportScriptPreamble(projectPath)), "\r\n")
}

func runtimeExportValidationScript(projectPath string, mapping string) string {
	return strings.TrimLeft(fmt.Sprintf(`
param(
  [string]$Mapping = '%s',
  [string]$Output = "",
  [string]$ParameterSet = "",
  [int]$HighErrorRows = 3,
  [switch]$SaveRecord
)

%s
if (-not $Output) {
  $Output = Join-Path $Root 'outputs\validation-result.json'
} elseif (-not [IO.Path]::IsPathRooted($Output)) {
  $Output = Join-Path $Root $Output
}
$OutputDir = Split-Path -Parent $Output
if ($OutputDir) {
  New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null
}
$WorkflowArgs = @('validate-data', '--project', $Project, '--mapping', $Mapping, '--high-error-rows', [string]$HighErrorRows, '--output', $Output)
if ($ParameterSet) {
  $WorkflowArgs += @('--parameter-set', $ParameterSet)
}
if ($SaveRecord) {
  $WorkflowArgs += '--save-record'
}
& $Runner @WorkflowArgs
Write-Host "wrote $Output"
`, powerShellSingleQuotedPath(mapping), runtimeExportScriptPreamble(projectPath)), "\r\n")
}

func runtimeExportCalibrationScript(projectPath string, setup string) string {
	return strings.TrimLeft(fmt.Sprintf(`
param(
  [string]$Setup = '%s',
  [string]$Output = "",
  [string]$SaveParameterSet = "",
  [switch]$SaveRecord
)

%s
if (-not $Output) {
  $Output = Join-Path $Root 'outputs\calibration-result.json'
} elseif (-not [IO.Path]::IsPathRooted($Output)) {
  $Output = Join-Path $Root $Output
}
$OutputDir = Split-Path -Parent $Output
if ($OutputDir) {
  New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null
}
$WorkflowArgs = @('calibrate', '--project', $Project, '--setup', $Setup, '--output', $Output)
if ($SaveParameterSet) {
  $WorkflowArgs += @('--save-parameter-set', $SaveParameterSet)
}
if ($SaveRecord) {
  $WorkflowArgs += '--save-record'
}
& $Runner @WorkflowArgs
Write-Host "wrote $Output"
`, powerShellSingleQuotedPath(setup), runtimeExportScriptPreamble(projectPath)), "\r\n")
}

func runtimeExportOptimizationScript(projectPath string, setup string) string {
	return strings.TrimLeft(fmt.Sprintf(`
param(
  [string]$Setup = '%s',
  [string]$Output = "",
  [string]$SaveScenario = "",
  [string]$SaveParameterSet = "",
  [switch]$SaveRecord
)

%s
if (-not $Output) {
  $Output = Join-Path $Root 'outputs\optimization-result.json'
} elseif (-not [IO.Path]::IsPathRooted($Output)) {
  $Output = Join-Path $Root $Output
}
$OutputDir = Split-Path -Parent $Output
if ($OutputDir) {
  New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null
}
$WorkflowArgs = @('optimize', '--project', $Project, '--setup', $Setup, '--output', $Output)
if ($SaveScenario) {
  $WorkflowArgs += @('--save-scenario', $SaveScenario)
}
if ($SaveParameterSet) {
  $WorkflowArgs += @('--save-parameter-set', $SaveParameterSet)
}
if ($SaveRecord) {
  $WorkflowArgs += '--save-record'
}
& $Runner @WorkflowArgs
Write-Host "wrote $Output"
`, powerShellSingleQuotedPath(setup), runtimeExportScriptPreamble(projectPath)), "\r\n")
}

func runtimeExportServeScript(projectPath string) string {
	return strings.TrimLeft(fmt.Sprintf(`
param(
  [string]$RequestFile = "",
  [string]$Output = ""
)

%s
$ServeArgs = @('serve', '--project', $Project)
if ($RequestFile) {
  if (-not [IO.Path]::IsPathRooted($RequestFile)) {
    $RequestFile = Join-Path $Root $RequestFile
  }
  if ($Output -and -not [IO.Path]::IsPathRooted($Output)) {
    $Output = Join-Path $Root $Output
  }
  if ($Output) {
    $OutputDir = Split-Path -Parent $Output
    if ($OutputDir) {
      New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null
    }
    Get-Content -LiteralPath $RequestFile | & $Runner @ServeArgs | Tee-Object -FilePath $Output
  } else {
    Get-Content -LiteralPath $RequestFile | & $Runner @ServeArgs
  }
} else {
  & $Runner @ServeArgs
}
`, runtimeExportScriptPreamble(projectPath)), "\r\n")
}

func runtimeExportSDKExample(projectPath string, defaultInput string) string {
	return fmt.Sprintf(`from pathlib import Path
import json
import sys


ROOT = Path(__file__).resolve().parent
SDK_ROOT = ROOT / "python" / "bcs_sdk"
if SDK_ROOT.exists():
    sys.path.insert(0, str(SDK_ROOT))

from bcs_sdk import RunnerClient


RUNNER = ROOT / "bin" / "bcs-runner.exe"
PROJECT = ROOT / %q
INPUT_REL = %q
if not INPUT_REL:
    raise SystemExit("This export has no default input. Pass an input file to bcs-runner directly.")
INPUT = ROOT / INPUT_REL
OUTPUT = ROOT / "outputs" / "sdk-example-output.json"

with INPUT.open("r", encoding="utf-8") as handle:
    payload = json.load(handle)

client = RunnerClient(project=PROJECT, runner=RUNNER, persistent=False)
client.validate_project()
result = client.run_once(
    dict(payload.get("inputs") or {}),
    dict(payload.get("context") or {}),
    output=OUTPUT,
)
print(json.dumps(result["outputs"], indent=2, sort_keys=True))
`, filepath.FromSlash(projectPath), filepath.FromSlash(defaultInput))
}

func runtimeExportOptimizationSDKExample(projectPath string, setup string) string {
	return fmt.Sprintf(`from pathlib import Path
import argparse
import json
import sys


ROOT = Path(__file__).resolve().parent
SDK_ROOT = ROOT / "python" / "bcs_sdk"
if SDK_ROOT.exists():
    sys.path.insert(0, str(SDK_ROOT))

from bcs_sdk import RunnerClient


RUNNER = ROOT / "bin" / "bcs-runner.exe"
PROJECT = ROOT / %q
DEFAULT_SETUP = %q


parser = argparse.ArgumentParser(description="Run an exported HVAC Studio optimization setup through bcs_sdk.")
parser.add_argument("--setup", default=DEFAULT_SETUP, help="Project-relative optimization setup path.")
parser.add_argument("--output", default=str(ROOT / "outputs" / "optimization-sdk-result.json"), help="Output JSON path.")
parser.add_argument("--save-scenario", default="", help="Project-relative scenario path for the optimized public inputs.")
parser.add_argument("--save-parameter-set", default="", help="Project-relative parameter set path for optimized component parameters.")
parser.add_argument("--save-record", action="store_true", help="Save an optimization result record under the exported project.")
args = parser.parse_args()

output = Path(args.output)
if not output.is_absolute():
    output = ROOT / output
output.parent.mkdir(parents=True, exist_ok=True)

client = RunnerClient(project=PROJECT, runner=RUNNER, persistent=False)
client.validate_project()
result = client.run_optimization(
    setup=args.setup,
    save_scenario=args.save_scenario or None,
    save_parameter_set=args.save_parameter_set or None,
    save_record=args.save_record,
    output=output,
)
print(json.dumps({
    "ok": result.get("ok"),
    "best_objective": result.get("best_objective"),
    "saved_scenario": result.get("saved_scenario", ""),
    "saved_parameter_set": result.get("saved_parameter_set", ""),
    "output": str(output),
}, indent=2, sort_keys=True))
`, filepath.FromSlash(projectPath), setup)
}

func runtimeExportScriptPreamble(projectPath string) string {
	projectLiteral := powerShellSingleQuotedPath(projectPath)
	return strings.TrimLeft(fmt.Sprintf(`
$ErrorActionPreference = 'Stop'
$Root = Split-Path -Parent $MyInvocation.MyCommand.Path
$Runner = Join-Path $Root 'bin\bcs-runner.exe'
if (-not (Test-Path -LiteralPath $Runner)) {
  $Runner = 'bcs-runner.exe'
}
$PythonRoot = Join-Path $Root 'runtime\python'
if (Test-Path -LiteralPath $PythonRoot) {
  $env:PATH = (@($PythonRoot, (Join-Path $Root 'bin'), $env:PATH) | Where-Object { $_ }) -join [IO.Path]::PathSeparator
}
$Project = Join-Path $Root '%s'

function Write-RunLogBundle {
  param(
    [Parameter(Mandatory = $true)][string]$ResultPath,
    [string]$LogPath = ""
  )

  if (-not (Test-Path -LiteralPath $ResultPath)) {
    return
  }
  if ($LogPath -and -not [IO.Path]::IsPathRooted($LogPath)) {
    $LogPath = Join-Path $Root $LogPath
  }
  if (-not $LogPath) {
    $BaseName = [IO.Path]::GetFileNameWithoutExtension($ResultPath)
    if (-not $BaseName) {
      $BaseName = 'run'
    }
    $LogPath = Join-Path (Join-Path $Root 'outputs\logs') ($BaseName + '-logs.json')
  }
  $LogDir = Split-Path -Parent $LogPath
  if ($LogDir) {
    New-Item -ItemType Directory -Force -Path $LogDir | Out-Null
  }

  $Result = Get-Content -Raw -LiteralPath $ResultPath | ConvertFrom-Json
  $Logs = @()
  if ($null -ne $Result.component_logs) {
    $Logs = @($Result.component_logs)
  }
  [ordered]@{
    schema = 'hvac-studio.runtime-log-bundle.v1'
    generated_at_utc = (Get-Date).ToUniversalTime().ToString('o')
    project = $Project
    result = $ResultPath
    component_logs = $Logs
  } | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath $LogPath -Encoding UTF8
  Write-Host "wrote $LogPath"
}
`, projectLiteral), "\r\n")
}

func powerShellSingleQuotedPath(path string) string {
	path = strings.ReplaceAll(filepath.ToSlash(path), "/", `\`)
	return strings.ReplaceAll(path, `'`, `''`)
}

func findRuntimeSupportRoot(start string) string {
	absStart, err := filepath.Abs(start)
	if err != nil {
		return ""
	}
	for {
		runner := platform.BinExecutable(absStart, "bcs-runner")
		python := platform.RuntimePythonPath(absStart)
		if _, runnerErr := os.Stat(runner); runnerErr == nil {
			if _, pythonErr := os.Stat(python); pythonErr == nil {
				return absStart
			}
		}
		parent := filepath.Dir(absStart)
		if parent == absStart {
			return ""
		}
		absStart = parent
	}
}

func copyRuntimeExportDir(projectRoot string, targetRoot string, rel string, files *[]string, seen map[string]bool) error {
	sourceRoot, err := resolveProjectOwnedFile(projectRoot, rel)
	if err != nil {
		return err
	}
	info, err := os.Stat(sourceRoot)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return apperror.Errorf(apperror.CodeValidation, "export source is not a directory: %s", rel)
	}
	return filepath.WalkDir(sourceRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		sourceRel, err := filepath.Rel(sourceRoot, path)
		if err != nil {
			return err
		}
		if sourceRel == "." {
			return nil
		}
		if entry.IsDir() && entry.Name() == "__pycache__" {
			return filepath.SkipDir
		}
		if entry.IsDir() {
			return nil
		}
		if strings.HasSuffix(entry.Name(), ".pyc") || strings.HasSuffix(entry.Name(), ".pyo") {
			return nil
		}
		return copyRuntimeExportFile(projectRoot, targetRoot, filepath.Join(rel, sourceRel), files, seen)
	})
}

func copyExternalExportDir(sourceRoot string, targetRoot string, rel string, files *[]string, seen map[string]bool) error {
	sourcePath := filepath.Join(sourceRoot, filepath.FromSlash(rel))
	info, err := os.Stat(sourcePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return apperror.Errorf(apperror.CodeValidation, "export support source is not a directory: %s", rel)
	}
	return filepath.WalkDir(sourcePath, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		sourceRel, err := filepath.Rel(sourcePath, path)
		if err != nil {
			return err
		}
		if sourceRel == "." {
			return nil
		}
		if entry.IsDir() && entry.Name() == "__pycache__" {
			return filepath.SkipDir
		}
		if entry.IsDir() {
			return nil
		}
		if strings.HasSuffix(entry.Name(), ".pyc") || strings.HasSuffix(entry.Name(), ".pyo") {
			return nil
		}
		return copyExternalExportFile(sourceRoot, targetRoot, filepath.Join(rel, sourceRel), files, seen)
	})
}

func copyRuntimeExportFile(projectRoot string, targetRoot string, rel string, files *[]string, seen map[string]bool) error {
	if rel == "" || rel == "." {
		return nil
	}
	ownedRel, sourcePath, err := projectOwnedRelativePath(projectRoot, rel)
	if err != nil {
		return err
	}
	info, err := os.Stat(sourcePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.IsDir() {
		return nil
	}
	artifactPath := exportArtifactPath(ownedRel)
	if seen[artifactPath] {
		return nil
	}
	bytes, err := os.ReadFile(sourcePath)
	if err != nil {
		return err
	}
	targetPath := filepath.Join(targetRoot, ownedRel)
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(targetPath, bytes, info.Mode().Perm()); err != nil {
		return err
	}
	seen[artifactPath] = true
	*files = append(*files, artifactPath)
	return nil
}

func copyExternalExportFile(sourceRoot string, targetRoot string, rel string, files *[]string, seen map[string]bool) error {
	if rel == "" || rel == "." {
		return nil
	}
	artifactPath := filepath.ToSlash(rel)
	if seen[artifactPath] {
		return nil
	}
	sourcePath := filepath.Join(sourceRoot, filepath.FromSlash(rel))
	info, err := os.Stat(sourcePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.IsDir() {
		return nil
	}
	bytes, err := os.ReadFile(sourcePath)
	if err != nil {
		return err
	}
	targetPath := filepath.Join(targetRoot, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(targetPath, bytes, info.Mode().Perm()); err != nil {
		return err
	}
	seen[artifactPath] = true
	*files = append(*files, artifactPath)
	return nil
}
