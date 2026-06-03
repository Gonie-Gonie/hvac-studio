package modelvalidation

import (
	"context"
	"path/filepath"
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
