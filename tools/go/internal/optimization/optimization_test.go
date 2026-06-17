package optimization

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRunGridOptimizationWritesScenario(t *testing.T) {
	tmpDir := t.TempDir()
	projectRoot := filepath.Join(tmpDir, "project")
	copyTree(t, filepath.Join("..", "..", "..", "..", "examples", "006_optimization_case"), projectRoot)
	projectPath := filepath.Join(projectRoot, "project.bcsproj")

	setup, err := LoadSetup(projectRoot, filepath.Join("optimization", "setups", "chw_setpoint_grid.json"))
	if err != nil {
		t.Fatal(err)
	}
	result, err := Run(context.Background(), projectPath, setup, Options{
		SaveScenario: filepath.Join("scenarios", "optimized_setpoint.json"),
	})
	if err != nil {
		t.Fatal(err)
	}

	if !result.OK || result.BestObjective != 92 || result.BestInputs["chw_setpoint_c"] != 7.0 {
		t.Fatalf("optimization result = %#v", result)
	}
	if len(result.Candidates) != 9 {
		t.Fatalf("candidate count = %d", len(result.Candidates))
	}
	if _, err := os.Stat(filepath.Join(projectRoot, "scenarios", "optimized_setpoint.json")); err != nil {
		t.Fatal(err)
	}
}

func TestRunGridOptimizationAppliesConstraints(t *testing.T) {
	tmpDir := t.TempDir()
	projectRoot := filepath.Join(tmpDir, "project")
	copyTree(t, filepath.Join("..", "..", "..", "..", "examples", "006_optimization_case"), projectRoot)
	projectPath := filepath.Join(projectRoot, "project.bcsproj")

	setup, err := LoadSetup(projectRoot, filepath.Join("optimization", "setups", "chw_setpoint_grid.json"))
	if err != nil {
		t.Fatal(err)
	}
	setup.Constraints = []Constraint{{
		Output:   "comfort_penalty_kw",
		Operator: "<=",
		Value:    0,
	}}

	result, err := Run(context.Background(), projectPath, setup, Options{})
	if err != nil {
		t.Fatal(err)
	}

	if !result.OK || result.BestInputs["chw_setpoint_c"] != 7.0 {
		t.Fatalf("optimization result = %#v", result)
	}
	foundViolation := false
	for _, candidate := range result.Candidates {
		if candidate.Feasible == false && len(candidate.ConstraintViolations) > 0 {
			foundViolation = true
		}
	}
	if !foundViolation {
		t.Fatalf("constraint violations were not recorded: %#v", result.Candidates)
	}
}

func TestRunGridOptimizationAppliesComponentParameterVariables(t *testing.T) {
	tmpDir := t.TempDir()
	projectRoot := filepath.Join(tmpDir, "project")
	copyTree(t, filepath.Join("..", "..", "..", "..", "examples", "006_optimization_case"), projectRoot)
	projectPath := filepath.Join(projectRoot, "project.bcsproj")

	setup := Setup{
		ID:        "parameter_credit_grid",
		Name:      "Parameter Credit Grid",
		Algorithm: "grid",
		BaseInputs: map[string]any{
			"building_load_kw": 500.0,
			"chw_setpoint_c":   7.0,
		},
		Context: map[string]any{"time": 0.0, "dt": 60.0},
		Objective: Objective{
			Output: "chiller_power_kw",
			Sense:  "min",
		},
		DecisionVariables: []DecisionVariable{{
			Kind:      "component_parameter",
			Component: "tradeoff",
			Name:      "power_credit_kw_per_k",
			Min:       4,
			Max:       12,
			Step:      4,
		}},
	}

	result, err := Run(context.Background(), projectPath, setup, Options{
		SaveParameterSet: filepath.Join("parameter_sets", "optimized_credit.json"),
	})
	if err != nil {
		t.Fatal(err)
	}

	if !result.OK || result.BestObjective != 88 || result.BestParameters["tradeoff"]["power_credit_kw_per_k"] != 12.0 {
		t.Fatalf("optimization result = %#v", result)
	}
	if result.SavedParameterSet != "parameter_sets/optimized_credit.json" {
		t.Fatalf("saved parameter set = %q", result.SavedParameterSet)
	}
	data, err := os.ReadFile(filepath.Join(projectRoot, "parameter_sets", "optimized_credit.json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved struct {
		Components map[string]map[string]any `json:"components"`
	}
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Components["tradeoff"]["power_credit_kw_per_k"] != 12.0 {
		t.Fatalf("saved parameter set = %#v", saved.Components)
	}
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
