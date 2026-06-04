package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goniegonie/hvac-studio/tools/go/internal/apperror"
)

func TestRunReturnsValidationExitCodeForUsage(t *testing.T) {
	err := run([]string{"bcs-runner"})
	if got := apperror.ExitCode(err); got != int(apperror.CodeValidation) {
		t.Fatalf("exit code = %d, want %d", got, apperror.CodeValidation)
	}
}

func TestMigrateCommandWritesCompatibleReport(t *testing.T) {
	tmpDir := t.TempDir()
	projectRoot := filepath.Join(tmpDir, "project")
	copyTree(t, examplePath("001_scalar_component"), projectRoot)
	outputPath := filepath.Join(tmpDir, "migration-report.json")

	if err := run([]string{
		"bcs-runner",
		"migrate",
		"--project",
		filepath.Join(projectRoot, "project.bcsproj"),
		"--output",
		outputPath,
	}); err != nil {
		t.Fatal(err)
	}

	var report struct {
		OK        bool `json:"ok"`
		Artifacts []struct {
			Kind           string `json:"kind"`
			Compatible     bool   `json:"compatible"`
			NeedsMigration bool   `json:"needs_migration"`
		} `json:"artifacts"`
		Actions []struct {
			Kind string `json:"kind"`
		} `json:"actions"`
	}
	readJSONFile(t, outputPath, &report)
	if !report.OK || len(report.Artifacts) != 2 {
		t.Fatalf("report = %#v", report)
	}
	for _, artifact := range report.Artifacts {
		if !artifact.Compatible || artifact.NeedsMigration {
			t.Fatalf("artifact = %#v", artifact)
		}
	}
	if len(report.Actions) != 1 || report.Actions[0].Kind != "no_migration_needed" {
		t.Fatalf("actions = %#v", report.Actions)
	}
}

func TestMigrateCommandFailsForIncompatibleProject(t *testing.T) {
	tmpDir := t.TempDir()
	writeFile(t, filepath.Join(tmpDir, "project.bcsproj"), `{
  "project_name": "future",
  "schema_version": "0.2.0",
  "entry_system": "MainSystem",
  "graph": "graph.json"
}
`)
	writeFile(t, filepath.Join(tmpDir, "graph.json"), `{
  "schema_version": "0.1.0",
  "systems": [],
  "components": [],
  "connections": []
}
`)
	outputPath := filepath.Join(tmpDir, "migration-report.json")

	err := run([]string{
		"bcs-runner",
		"migrate",
		"--project",
		filepath.Join(tmpDir, "project.bcsproj"),
		"--output",
		outputPath,
	})
	if got := apperror.ExitCode(err); got != int(apperror.CodeValidation) {
		t.Fatalf("exit code = %d, want %d; error=%v", got, apperror.CodeValidation, err)
	}
	var report struct {
		OK        bool `json:"ok"`
		Artifacts []struct {
			Kind           string `json:"kind"`
			NeedsMigration bool   `json:"needs_migration"`
		} `json:"artifacts"`
	}
	readJSONFile(t, outputPath, &report)
	if report.OK || len(report.Artifacts) != 2 || !report.Artifacts[0].NeedsMigration {
		t.Fatalf("report = %#v", report)
	}
}

func TestRunReturnsInputExitCodeForMissingPublicInput(t *testing.T) {
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "missing-input.json")
	if err := os.WriteFile(inputPath, []byte(`{"inputs":{},"context":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	projectPath := filepath.Join("..", "..", "..", "..", "examples", "001_scalar_component", "project.bcsproj")
	err := run([]string{
		"bcs-runner",
		"run",
		"--project",
		projectPath,
		"--input",
		inputPath,
		"--output",
		filepath.Join(tmpDir, "output.json"),
	})
	if got := apperror.ExitCode(err); got != int(apperror.CodeInput) {
		t.Fatalf("exit code = %d, want %d; error=%v", got, apperror.CodeInput, err)
	}
	if !strings.Contains(err.Error(), "missing required public input: value") {
		t.Fatalf("missing public input message was not actionable: %v", err)
	}
}

func TestRunReturnsInputExitCodeForInvalidInputJSON(t *testing.T) {
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "invalid-input.json")
	if err := os.WriteFile(inputPath, []byte(`{"inputs":`), 0o644); err != nil {
		t.Fatal(err)
	}

	projectPath := filepath.Join("..", "..", "..", "..", "examples", "001_scalar_component", "project.bcsproj")
	err := run([]string{
		"bcs-runner",
		"run",
		"--project",
		projectPath,
		"--input",
		inputPath,
		"--output",
		filepath.Join(tmpDir, "output.json"),
	})
	if got := apperror.ExitCode(err); got != int(apperror.CodeInput) {
		t.Fatalf("exit code = %d, want %d; error=%v", got, apperror.CodeInput, err)
	}
}

func TestRunReturnsPythonWorkerExitCodeForMissingDeclaredOutput(t *testing.T) {
	tmpDir := t.TempDir()
	projectRoot := filepath.Join(tmpDir, "project")
	copyTree(t, examplePath("001_scalar_component"), projectRoot)
	writeFile(t, filepath.Join(projectRoot, "components", "scalar.py"), `class Gain:
    def initialize(self, params, context):
        return {}

    def evaluate(self, inputs, state, params, context):
        return {}, state
`)

	err := run([]string{
		"bcs-runner",
		"run",
		"--project",
		filepath.Join(projectRoot, "project.bcsproj"),
		"--input",
		filepath.Join(projectRoot, "inputs", "case01.json"),
		"--output",
		filepath.Join(tmpDir, "output.json"),
	})
	if got := apperror.ExitCode(err); got != int(apperror.CodePythonWorker) {
		t.Fatalf("exit code = %d, want %d; error=%v", got, apperror.CodePythonWorker, err)
	}
	if !strings.Contains(err.Error(), "component gain did not return declared output node: result") {
		t.Fatalf("missing declared output message was not actionable: %v", err)
	}
}

func TestRunCommandAppliesParameterSetWithoutOverwritingGraph(t *testing.T) {
	tmpDir := t.TempDir()
	projectRoot := filepath.Join(tmpDir, "project")
	copyTree(t, examplePath("001_scalar_component"), projectRoot)
	writeFile(t, filepath.Join(projectRoot, "parameter_sets", "triple.json"), `{
  "id": "triple",
  "components": {
    "gain": {
      "gain": 3
    }
  }
}`)
	outputPath := filepath.Join(tmpDir, "output.json")

	err := run([]string{
		"bcs-runner",
		"run",
		"--project",
		filepath.Join(projectRoot, "project.bcsproj"),
		"--input",
		filepath.Join(projectRoot, "inputs", "case01.json"),
		"--parameter-set",
		filepath.Join("parameter_sets", "triple.json"),
		"--output",
		outputPath,
	})
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		ParameterSet string             `json:"parameter_set"`
		Outputs      map[string]float64 `json:"outputs"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	if result.ParameterSet != "parameter_sets/triple.json" || result.Outputs["result"] != 12 {
		t.Fatalf("result = %#v", result)
	}
	graphBytes, err := os.ReadFile(filepath.Join(projectRoot, "graph.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(graphBytes, []byte(`"gain": 2.5`)) {
		t.Fatalf("graph should keep baseline parameter, got:\n%s", string(graphBytes))
	}
}

func TestRunSeriesCommandCarriesStateAcrossSteps(t *testing.T) {
	tmpDir := t.TempDir()
	projectRoot := filepath.Join(tmpDir, "project")
	copyTree(t, examplePath("004_stateful_controller"), projectRoot)
	outputPath := filepath.Join(tmpDir, "series-output.json")

	err := run([]string{
		"bcs-runner",
		"run-series",
		"--project",
		filepath.Join(projectRoot, "project.bcsproj"),
		"--input",
		filepath.Join(projectRoot, "inputs", "series01.json"),
		"--output",
		outputPath,
	})
	if err != nil {
		t.Fatal(err)
	}

	var result struct {
		OK        bool                 `json:"ok"`
		StepCount int                  `json:"step_count"`
		Outputs   map[string][]float64 `json:"outputs"`
		Series    []struct {
			ID      string                        `json:"id"`
			Time    float64                       `json:"time"`
			Outputs map[string]float64            `json:"outputs"`
			States  map[string]map[string]float64 `json:"states"`
		} `json:"series"`
		FinalStates map[string]map[string]float64 `json:"final_states"`
	}
	readJSONFile(t, outputPath, &result)
	if !result.OK || result.StepCount != 3 || len(result.Series) != 3 {
		t.Fatalf("series result = %#v", result)
	}
	if result.Outputs["chw_setpoint_c"][0] != 6.5 ||
		result.Outputs["chw_setpoint_c"][1] != 6.4 ||
		result.Outputs["chw_setpoint_c"][2] != 6.55 {
		t.Fatalf("setpoint series = %#v", result.Outputs["chw_setpoint_c"])
	}
	if result.Series[1].ID != "minute-1" || result.Series[1].Time != 60 {
		t.Fatalf("second point identity = %#v", result.Series[1])
	}
	if result.Series[1].States["controller"]["calls"] != 2 {
		t.Fatalf("second point state = %#v", result.Series[1].States)
	}
	if result.FinalStates["controller"]["calls"] != 3 ||
		result.FinalStates["controller"]["integral_error"] != 2.5 ||
		result.FinalStates["controller"]["last_error"] != 0.5 {
		t.Fatalf("final states = %#v", result.FinalStates)
	}
}

func TestSchemaCommandWritesPublicInterface(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "schema.json")
	projectPath := filepath.Join("..", "..", "..", "..", "examples", "003_feedforward_system", "project.bcsproj")

	err := run([]string{
		"bcs-runner",
		"schema",
		"--project",
		projectPath,
		"--output",
		outputPath,
	})
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		ProjectName string `json:"project_name"`
		System      string `json:"system"`
		Inputs      []struct {
			ID        string `json:"id"`
			Component string `json:"component"`
			Node      string `json:"node"`
			Required  bool   `json:"required"`
		} `json:"inputs"`
		Outputs []struct {
			ID        string `json:"id"`
			Component string `json:"component"`
			Node      string `json:"node"`
		} `json:"outputs"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	if schema.ProjectName != "003_feedforward_system" {
		t.Fatalf("project_name = %q", schema.ProjectName)
	}
	if schema.System != "MainSystem" {
		t.Fatalf("system = %q", schema.System)
	}
	if len(schema.Inputs) != 2 {
		t.Fatalf("input count = %d", len(schema.Inputs))
	}
	if schema.Inputs[0].ID != "building_load_kw" || schema.Inputs[0].Component != "load_model" || !schema.Inputs[0].Required {
		t.Fatalf("unexpected first input: %+v", schema.Inputs[0])
	}
	if len(schema.Outputs) != 3 {
		t.Fatalf("output count = %d", len(schema.Outputs))
	}
	if schema.Outputs[0].ID != "total_power_kw" || schema.Outputs[0].Component != "aggregator" {
		t.Fatalf("unexpected first output: %+v", schema.Outputs[0])
	}
}

func TestValidateDataCommandWritesMetrics(t *testing.T) {
	tmpDir := t.TempDir()
	projectRoot := filepath.Join(tmpDir, "project")
	copyTree(t, examplePath("005_chiller_plant_like_system"), projectRoot)
	outputPath := filepath.Join(tmpDir, "validation.json")

	err := run([]string{
		"bcs-runner",
		"validate-data",
		"--project",
		filepath.Join(projectRoot, "project.bcsproj"),
		"--mapping",
		filepath.Join("validation", "mappings", "plant_validation.json"),
		"--output",
		outputPath,
		"--high-error-rows",
		"1",
		"--save-record",
	})
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		OK          bool   `json:"ok"`
		RowCount    int    `json:"row_count"`
		SavedRecord string `json:"saved_record"`
		Metrics     map[string]struct {
			Count         int `json:"count"`
			HighErrorRows []struct {
				RowIndex int `json:"row_index"`
			} `json:"high_error_rows"`
		} `json:"metrics"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.RowCount != 3 {
		t.Fatalf("validation result = %#v", result)
	}
	if !strings.HasPrefix(result.SavedRecord, "validation/runs/validation-") {
		t.Fatalf("saved record = %q", result.SavedRecord)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, filepath.FromSlash(result.SavedRecord))); err != nil {
		t.Fatal(err)
	}
	provenance := readWorkflowProvenance(t, projectRoot, result.SavedRecord)
	requireArtifact(t, provenance, "validation_mapping", "validation/mappings/plant_validation.json")
	requireArtifact(t, provenance, "dataset", "datasets/plant_validation.csv")
	if provenance.Project.SHA256 == "" || provenance.Graph.SHA256 == "" || provenance.RunnerVersion == "" {
		t.Fatalf("validation provenance = %#v", provenance)
	}
	if result.Metrics["total_power_kw"].Count != 3 || len(result.Metrics["total_power_kw"].HighErrorRows) != 1 {
		t.Fatalf("metrics = %#v", result.Metrics)
	}
}

func TestCalibrateCommandWritesResultAndParameterSet(t *testing.T) {
	tmpDir := t.TempDir()
	projectRoot := filepath.Join(tmpDir, "project")
	copyTree(t, examplePath("005_chiller_plant_like_system"), projectRoot)
	outputPath := filepath.Join(tmpDir, "calibration.json")

	err := run([]string{
		"bcs-runner",
		"calibrate",
		"--project",
		filepath.Join(projectRoot, "project.bcsproj"),
		"--setup",
		filepath.Join("calibration", "setups", "chiller_cop_grid.json"),
		"--save-parameter-set",
		filepath.Join("parameter_sets", "calibrated_cli.json"),
		"--save-record",
		"--output",
		outputPath,
	})
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		OK                bool    `json:"ok"`
		BestObjective     float64 `json:"best_objective"`
		SavedParameterSet string  `json:"saved_parameter_set"`
		SavedRecord       string  `json:"saved_record"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.BestObjective <= 0 || result.SavedParameterSet != "parameter_sets/calibrated_cli.json" {
		t.Fatalf("calibration result = %#v", result)
	}
	if !strings.HasPrefix(result.SavedRecord, "calibration/results/calibration-") {
		t.Fatalf("saved record = %q", result.SavedRecord)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, "parameter_sets", "calibrated_cli.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, filepath.FromSlash(result.SavedRecord))); err != nil {
		t.Fatal(err)
	}
	provenance := readWorkflowProvenance(t, projectRoot, result.SavedRecord)
	requireArtifact(t, provenance, "calibration_setup", "calibration/setups/chiller_cop_grid.json")
	requireArtifact(t, provenance, "validation_mapping", "validation/mappings/plant_validation.json")
	requireArtifact(t, provenance, "saved_parameter_set", "parameter_sets/calibrated_cli.json")
}

func TestOptimizeCommandWritesResultAndScenario(t *testing.T) {
	tmpDir := t.TempDir()
	projectRoot := filepath.Join(tmpDir, "project")
	copyTree(t, examplePath("006_optimization_case"), projectRoot)
	outputPath := filepath.Join(tmpDir, "optimization.json")

	err := run([]string{
		"bcs-runner",
		"optimize",
		"--project",
		filepath.Join(projectRoot, "project.bcsproj"),
		"--setup",
		filepath.Join("optimization", "setups", "chw_setpoint_grid.json"),
		"--save-scenario",
		filepath.Join("scenarios", "optimized_cli.json"),
		"--save-record",
		"--output",
		outputPath,
	})
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		OK            bool               `json:"ok"`
		BestObjective float64            `json:"best_objective"`
		BestInputs    map[string]float64 `json:"best_inputs"`
		SavedScenario string             `json:"saved_scenario"`
		SavedRecord   string             `json:"saved_record"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.BestObjective != 92 || result.BestInputs["chw_setpoint_c"] != 7 || result.SavedScenario != "scenarios/optimized_cli.json" {
		t.Fatalf("optimization result = %#v", result)
	}
	if !strings.HasPrefix(result.SavedRecord, "optimization/results/optimization-") {
		t.Fatalf("saved record = %q", result.SavedRecord)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, "scenarios", "optimized_cli.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, filepath.FromSlash(result.SavedRecord))); err != nil {
		t.Fatal(err)
	}
	provenance := readWorkflowProvenance(t, projectRoot, result.SavedRecord)
	requireArtifact(t, provenance, "optimization_setup", "optimization/setups/chw_setpoint_grid.json")
	requireArtifact(t, provenance, "saved_scenario", "scenarios/optimized_cli.json")
}

func TestServeCommandReusesLoadedSessionState(t *testing.T) {
	tmpDir := t.TempDir()
	projectRoot := filepath.Join(tmpDir, "project")
	copyTree(t, examplePath("001_scalar_component"), projectRoot)
	writeFile(t, filepath.Join(projectRoot, "components", "scalar.py"), `class Gain:
    def initialize(self, params, context):
        return {"calls": 0}

    def evaluate(self, inputs, state, params, context):
        calls = state.get("calls", 0) + 1
        return {"result": calls}, {"calls": calls}
`)

	requests := strings.Join([]string{
		`{"id":"a","inputs":{"value":1},"context":{"time":0}}`,
		`{"id":"b","inputs":{"value":1},"context":{"time":60}}`,
		`{"id":"stop","type":"shutdown"}`,
		"",
	}, "\n")
	var output bytes.Buffer
	err := serveProject([]string{"--project", filepath.Join(projectRoot, "project.bcsproj")}, strings.NewReader(requests), &output)
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("response lines = %d output=%s", len(lines), output.String())
	}
	responses := make([]struct {
		ID     string `json:"id"`
		OK     bool   `json:"ok"`
		Result struct {
			Outputs          map[string]float64            `json:"outputs"`
			States           map[string]map[string]float64 `json:"states"`
			ComponentTimings []struct {
				Component  string  `json:"component"`
				Stage      string  `json:"stage"`
				DurationMS float64 `json:"duration_ms"`
			} `json:"component_timings"`
			DurationMS float64 `json:"duration_ms"`
		} `json:"result"`
		Message string `json:"message"`
	}, len(lines))
	for index, line := range lines {
		if err := json.Unmarshal([]byte(line), &responses[index]); err != nil {
			t.Fatalf("decode response %d: %v\n%s", index, err, line)
		}
	}
	if !responses[0].OK || responses[0].ID != "a" || responses[0].Result.Outputs["result"] != 1 {
		t.Fatalf("first response = %#v", responses[0])
	}
	if !responses[1].OK || responses[1].ID != "b" || responses[1].Result.Outputs["result"] != 2 {
		t.Fatalf("second response = %#v", responses[1])
	}
	if responses[1].Result.States["gain"]["calls"] != 2 {
		t.Fatalf("second state = %#v", responses[1].Result.States)
	}
	if len(responses[1].Result.ComponentTimings) != 1 || responses[1].Result.ComponentTimings[0].Component != "gain" || responses[1].Result.ComponentTimings[0].Stage != "evaluate" {
		t.Fatalf("component timings = %#v", responses[1].Result.ComponentTimings)
	}
	if !responses[2].OK || responses[2].Message != "shutdown" {
		t.Fatalf("shutdown response = %#v", responses[2])
	}
}

func TestServeCommandReturnsStructuredError(t *testing.T) {
	tmpDir := t.TempDir()
	projectRoot := filepath.Join(tmpDir, "project")
	copyTree(t, examplePath("001_scalar_component"), projectRoot)

	requests := strings.Join([]string{
		`{"id":"bad","inputs":{},"context":{}}`,
		`{"id":"stop","type":"shutdown"}`,
		"",
	}, "\n")
	var output bytes.Buffer
	err := serveProject([]string{"--project", filepath.Join(projectRoot, "project.bcsproj")}, strings.NewReader(requests), &output)
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("response lines = %d output=%s", len(lines), output.String())
	}
	var response struct {
		ID    string `json:"id"`
		OK    bool   `json:"ok"`
		Error struct {
			Schema  string `json:"schema"`
			Code    int    `json:"code"`
			Kind    string `json:"kind"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &response); err != nil {
		t.Fatal(err)
	}
	if response.OK || response.Error.Schema != "hvac-studio.error.v1" || response.Error.Kind != "input" {
		t.Fatalf("response = %#v", response)
	}
	if !strings.Contains(response.Error.Message, "missing required public input") {
		t.Fatalf("message = %s", response.Error.Message)
	}
}

func TestSplitGlobalErrorFormat(t *testing.T) {
	args, format := splitGlobalErrorFormat([]string{"bcs-runner", "--error-format", "json", "validate"})
	if format != "json" || strings.Join(args, " ") != "bcs-runner validate" {
		t.Fatalf("args=%v format=%s", args, format)
	}
	args, format = splitGlobalErrorFormat([]string{"bcs-runner", "--error-format=yaml", "validate"})
	if format != "text" || strings.Join(args, " ") != "bcs-runner validate" {
		t.Fatalf("args=%v format=%s", args, format)
	}
}

func examplePath(name string) string {
	return filepath.Join("..", "..", "..", "..", "examples", name)
}

type workflowProvenance struct {
	Schema        string `json:"schema"`
	RunnerVersion string `json:"runner_version"`
	Project       struct {
		Path   string `json:"path"`
		SHA256 string `json:"sha256"`
	} `json:"project"`
	Graph struct {
		Path   string `json:"path"`
		SHA256 string `json:"sha256"`
	} `json:"graph"`
	Artifacts []struct {
		Role   string `json:"role"`
		Path   string `json:"path"`
		SHA256 string `json:"sha256"`
	} `json:"artifacts"`
}

func readWorkflowProvenance(t *testing.T, projectRoot string, recordPath string) workflowProvenance {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(projectRoot, filepath.FromSlash(recordPath)))
	if err != nil {
		t.Fatal(err)
	}
	var record struct {
		Provenance workflowProvenance `json:"provenance"`
	}
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	if record.Provenance.Schema != "hvac-studio.workflow-provenance.v1" {
		t.Fatalf("provenance schema = %q", record.Provenance.Schema)
	}
	if len(record.Provenance.Project.SHA256) != 64 || len(record.Provenance.Graph.SHA256) != 64 {
		t.Fatalf("provenance checksums = %#v", record.Provenance)
	}
	return record.Provenance
}

func requireArtifact(t *testing.T, provenance workflowProvenance, role string, path string) {
	t.Helper()
	for _, artifact := range provenance.Artifacts {
		if artifact.Role == role && artifact.Path == path && len(artifact.SHA256) == 64 {
			return
		}
	}
	t.Fatalf("provenance missing artifact role=%s path=%s: %#v", role, path, provenance.Artifacts)
}

func copyTree(t *testing.T, sourceRoot string, targetRoot string) {
	t.Helper()
	err := filepath.WalkDir(sourceRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(sourceRoot, path)
		if err != nil {
			return err
		}
		target := filepath.Join(targetRoot, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readJSONFile(t *testing.T, path string, target any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		t.Fatal(err)
	}
}
