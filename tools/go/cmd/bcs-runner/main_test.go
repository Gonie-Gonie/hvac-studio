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

func examplePath(name string) string {
	return filepath.Join("..", "..", "..", "..", "examples", name)
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
