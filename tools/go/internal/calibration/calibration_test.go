package calibration

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunGridCalibrationWritesParameterSetWithoutOverwritingGraph(t *testing.T) {
	tmpDir := t.TempDir()
	projectRoot := filepath.Join(tmpDir, "project")
	copyTree(t, filepath.Join("..", "..", "..", "..", "examples", "005_chiller_plant_like_system"), projectRoot)
	projectPath := filepath.Join(projectRoot, "project.bcsproj")

	setup, err := LoadSetup(projectRoot, filepath.Join("calibration", "setups", "chiller_cop_grid.json"))
	if err != nil {
		t.Fatal(err)
	}
	result, err := Run(context.Background(), projectPath, setup, Options{
		SaveParameterSet: filepath.Join("parameter_sets", "calibrated_test.json"),
	})
	if err != nil {
		t.Fatal(err)
	}

	if !result.OK || result.BestObjective <= 0 || len(result.Candidates) != 5 {
		t.Fatalf("calibration result = %#v", result)
	}
	if result.SavedParameterSet != "parameter_sets/calibrated_test.json" {
		t.Fatalf("saved parameter set = %q", result.SavedParameterSet)
	}
	if result.BestParameterSet.Components["chiller"]["cop"] == nil {
		t.Fatalf("best parameter set = %#v", result.BestParameterSet)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, "parameter_sets", "calibrated_test.json")); err != nil {
		t.Fatal(err)
	}
	graphBytes, err := os.ReadFile(filepath.Join(projectRoot, "graph.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !contains(graphBytes, `"cop": 6.0`) {
		t.Fatalf("graph should keep baseline COP, got:\n%s", string(graphBytes))
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

func contains(data []byte, text string) bool {
	return strings.Contains(string(data), text)
}
