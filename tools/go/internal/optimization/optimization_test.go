package optimization

import (
	"context"
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
