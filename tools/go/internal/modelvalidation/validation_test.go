package modelvalidation

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goniegonie/hvac-studio/tools/go/internal/project"
)

func TestRunComputesMetricsAndHighErrorInspection(t *testing.T) {
	projectPath := filepath.Join("..", "..", "..", "..", "examples", "005_chiller_plant_like_system", "project.bcsproj")
	loaded, err := project.Load(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	mapping, err := LoadMapping(loaded.Root, filepath.Join("validation", "mappings", "plant_validation.json"))
	if err != nil {
		t.Fatal(err)
	}

	result, err := Run(context.Background(), loaded, mapping, Options{HighErrorRows: 2})
	if err != nil {
		t.Fatal(err)
	}

	if !result.OK || result.RowCount != 3 || len(result.Rows) != 3 {
		t.Fatalf("result summary = %#v", result)
	}
	totalPower := result.Metrics["total_power_kw"]
	if totalPower.Count != 3 || totalPower.RMSE <= 0 || totalPower.MAE <= 0 {
		t.Fatalf("total power metrics = %#v", totalPower)
	}
	if len(totalPower.HighErrorRows) != 2 {
		t.Fatalf("high-error rows = %#v", totalPower.HighErrorRows)
	}
	if totalPower.HighErrorRows[0].Inspection.ComponentOutputs["aggregator"]["total_power_kw"] == nil {
		t.Fatalf("high-error inspection = %#v", totalPower.HighErrorRows[0].Inspection)
	}
	if result.Metrics["chw_supply_temp_c"].Count != 3 {
		t.Fatalf("supply temp metrics = %#v", result.Metrics["chw_supply_temp_c"])
	}
}

func TestRunHandlesMissingValuePolicies(t *testing.T) {
	loaded := copyPlantProject(t)
	missingDataset := writeValidationDataset(t, loaded.Root, "missing_policy.csv", `time,building_load_kw,base_chw_setpoint_c,condenser_entering_temp_c,measured_total_power_kw,measured_chw_supply_temp_c
0,450,7.0,29.0,91.2,7.12
60,,7.0,32.0,142.5,7.08
120,700,6.8,34.0,,6.98
`)

	_, err := Run(context.Background(), loaded, plantMapping(missingDataset, MissingPolicyError), Options{})
	if err == nil || !strings.Contains(err.Error(), "missing value") {
		t.Fatalf("error policy err = %v", err)
	}

	dropped, err := Run(context.Background(), loaded, plantMapping(missingDataset, MissingPolicyDrop), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !dropped.OK || dropped.InputRowCount != 3 || dropped.RowCount != 1 || dropped.SkippedRowCount != 2 {
		t.Fatalf("drop result = %#v", dropped)
	}
	if !dropped.Rows[1].Skipped || !strings.Contains(dropped.Rows[1].Error, "building_load_kw") {
		t.Fatalf("drop rows = %#v", dropped.Rows)
	}
	if dropped.Metrics["total_power_kw"].Count != 1 {
		t.Fatalf("drop metrics = %#v", dropped.Metrics["total_power_kw"])
	}

	filled, err := Run(context.Background(), loaded, plantMapping(missingDataset, MissingPolicyFill), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !filled.OK || filled.RowCount != 3 || filled.SkippedRowCount != 0 || filled.FilledValueCount < 2 {
		t.Fatalf("fill result = %#v", filled)
	}
	if len(filled.Rows[1].Filled) == 0 || len(filled.Rows[2].Filled) == 0 {
		t.Fatalf("filled row markers = %#v", filled.Rows)
	}

	outputMissingDataset := writeValidationDataset(t, loaded.Root, "missing_observed.csv", `time,building_load_kw,base_chw_setpoint_c,condenser_entering_temp_c,measured_total_power_kw,measured_chw_supply_temp_c
0,450,7.0,29.0,91.2,7.12
60,600,7.0,32.0,,7.08
120,700,6.8,34.0,172.4,6.98
`)
	ignored, err := Run(context.Background(), loaded, plantMapping(outputMissingDataset, MissingPolicyIgnoreOutputRows), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !ignored.OK || ignored.InputRowCount != 3 || ignored.RowCount != 2 || ignored.SkippedRowCount != 1 {
		t.Fatalf("ignore output rows result = %#v", ignored)
	}
	if !ignored.Rows[1].Skipped || !strings.Contains(ignored.Rows[1].Error, "observed output") {
		t.Fatalf("ignored rows = %#v", ignored.Rows)
	}
	if ignored.Metrics["total_power_kw"].Count != 2 {
		t.Fatalf("ignored metrics = %#v", ignored.Metrics["total_power_kw"])
	}
}

func plantMapping(dataset string, policy string) Mapping {
	return Mapping{
		ID:                 "plant_validation",
		Name:               "Plant Validation",
		Dataset:            dataset,
		TimeColumn:         "time",
		MissingValuePolicy: policy,
		InputColumns: map[string]string{
			"building_load_kw":          "building_load_kw",
			"base_chw_setpoint_c":       "base_chw_setpoint_c",
			"condenser_entering_temp_c": "condenser_entering_temp_c",
		},
		ObservedOutputColumns: map[string]string{
			"total_power_kw":    "measured_total_power_kw",
			"chw_supply_temp_c": "measured_chw_supply_temp_c",
		},
	}
}

func copyPlantProject(t *testing.T) *project.LoadedProject {
	t.Helper()
	src := filepath.Join("..", "..", "..", "..", "examples", "005_chiller_plant_like_system")
	dst := filepath.Join(t.TempDir(), "plant")
	if err := copyDirectory(src, dst); err != nil {
		t.Fatal(err)
	}
	loaded, err := project.Load(filepath.Join(dst, "project.bcsproj"))
	if err != nil {
		t.Fatal(err)
	}
	return loaded
}

func writeValidationDataset(t *testing.T, projectRoot string, name string, content string) string {
	t.Helper()
	relativePath := filepath.ToSlash(filepath.Join("datasets", name))
	path := filepath.Join(projectRoot, "datasets", name)
	if err := os.WriteFile(path, []byte(strings.TrimSpace(content)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return relativePath
}

func copyDirectory(src string, dst string) error {
	return filepath.WalkDir(src, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		defer input.Close()
		output, err := os.Create(target)
		if err != nil {
			return err
		}
		defer output.Close()
		_, err = io.Copy(output, input)
		return err
	})
}
