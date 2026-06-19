package studio

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/goniegonie/hvac-studio/tools/go/internal/calibration"
	"github.com/goniegonie/hvac-studio/tools/go/internal/optimization"
	"github.com/goniegonie/hvac-studio/tools/go/internal/project"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/transform"
)

func TestProjectEndpointIncludesDefaultRunInput(t *testing.T) {
	server := newTestServer(t)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/project?project_path=examples/001_scalar_component/project.bcsproj", nil)

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Project ProjectDetail `json:"project"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Project.DefaultRunInput == nil {
		t.Fatal("default_run_input is nil")
	}
	if got := body.Project.DefaultRunInput.Inputs["value"]; got != 4.0 {
		t.Fatalf("default value = %v, want 4", got)
	}
}

func TestProjectEndpointIncludesDatasetAndParameterSetSummaries(t *testing.T) {
	server := newTestServer(t)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/project?project_path=examples/005_chiller_plant_like_system/project.bcsproj", nil)

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Project ProjectDetail `json:"project"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Project.Datasets) != 1 {
		t.Fatalf("dataset count = %d", len(body.Project.Datasets))
	}
	dataset := body.Project.Datasets[0]
	if dataset.ID != "plant_validation" || dataset.RowCount != 3 || dataset.ColumnCount != 6 {
		t.Fatalf("dataset summary = %#v", dataset)
	}
	if len(body.Project.ParameterSets) != 2 {
		t.Fatalf("parameter set count = %d", len(body.Project.ParameterSets))
	}
	if body.Project.ParameterSets[0].ParameterCount == 0 {
		t.Fatalf("parameter set summary = %#v", body.Project.ParameterSets[0])
	}
	if len(body.Project.ValidationMappings) != 1 {
		t.Fatalf("validation mapping count = %d", len(body.Project.ValidationMappings))
	}
	mapping := body.Project.ValidationMappings[0]
	if mapping.ID != "plant_validation" || mapping.InputCount != 3 || mapping.OutputCount != 2 {
		t.Fatalf("validation mapping summary = %#v", mapping)
	}
	if len(body.Project.CalibrationSetups) != 1 {
		t.Fatalf("calibration setup count = %d", len(body.Project.CalibrationSetups))
	}
	if body.Project.CalibrationSetups[0].ID != "chiller_cop_grid" || body.Project.CalibrationSetups[0].ParameterCount != 1 {
		t.Fatalf("calibration setup summary = %#v", body.Project.CalibrationSetups[0])
	}
}

func TestDataValidationEndpointRunsMapping(t *testing.T) {
	server := newTestServer(t)
	payload := []byte(`{
		"project_path": "examples/005_chiller_plant_like_system/project.bcsproj",
		"mapping_path": "validation/mappings/plant_validation.json",
		"parameter_set_path": "parameter_sets/high_efficiency.json",
		"high_error_rows": 1
	}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/validation/run", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		OK               bool `json:"ok"`
		ValidationResult struct {
			RowCount     int    `json:"row_count"`
			ParameterSet string `json:"parameter_set"`
			Metrics      map[string]struct {
				Count         int `json:"count"`
				HighErrorRows []struct {
					RowIndex int `json:"row_index"`
				} `json:"high_error_rows"`
			} `json:"metrics"`
		} `json:"validation_result"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.OK || body.ValidationResult.RowCount != 3 {
		t.Fatalf("validation response = %#v", body)
	}
	if body.ValidationResult.ParameterSet != "parameter_sets/high_efficiency.json" {
		t.Fatalf("parameter_set = %q", body.ValidationResult.ParameterSet)
	}
	if body.ValidationResult.Metrics["total_power_kw"].Count != 3 || len(body.ValidationResult.Metrics["total_power_kw"].HighErrorRows) != 1 {
		t.Fatalf("validation metrics = %#v", body.ValidationResult.Metrics)
	}
}

func TestDatasetPreviewEndpointSuggestsPublicIOMapping(t *testing.T) {
	server := newTestServer(t)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/project/dataset?project_path="+url.QueryEscape("examples/005_chiller_plant_like_system/project.bcsproj")+"&path="+url.QueryEscape("datasets/plant_validation.csv"),
		nil,
	)

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Dataset DatasetPreview `json:"dataset"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Dataset.Summary.RowCount != 3 || len(body.Dataset.Columns) != 6 || len(body.Dataset.PreviewRows) == 0 {
		t.Fatalf("dataset preview = %#v", body.Dataset)
	}
	if body.Dataset.SuggestedTimeColumn != "time" {
		t.Fatalf("suggested time column = %q", body.Dataset.SuggestedTimeColumn)
	}
	if !hasColumnSuggestion(body.Dataset.SuggestedInputs, "building_load_kw", "building_load_kw") {
		t.Fatalf("input suggestions = %#v", body.Dataset.SuggestedInputs)
	}
	if !hasColumnSuggestion(body.Dataset.SuggestedOutputs, "total_power_kw", "measured_total_power_kw") {
		t.Fatalf("output suggestions = %#v", body.Dataset.SuggestedOutputs)
	}
}

func TestImportDatasetEndpointCopiesCSVAndCreatesMapping(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	projectRoot := filepath.Join(root, "projects", "dataset-import-project")
	if err := copyProjectTree(filepath.Join(repoRoot, "examples", "005_chiller_plant_like_system"), projectRoot); err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join(t.TempDir(), "incoming plant.csv")
	writeTestFile(t, sourcePath, "time;building_load_kw;outdoor_temp_c;chw_setpoint_c;measured_total_power_kw;measured_chw_supply_temp_c\n0;120;32;6;42.1;6.4\n60;150;34;6;51.2;6.6\n")
	projectPath := filepath.Join(projectRoot, "project.bcsproj")
	payload, err := json.Marshal(map[string]any{
		"project_path": projectPath,
		"source_path":  sourcePath,
		"id":           "Imported Plant",
		"delimiter":    "auto",
		"encoding":     "utf-8-bom",
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/datasets/import", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Summary DatasetSummary `json:"summary"`
		Dataset DatasetPreview `json:"dataset"`
		Project ProjectDetail  `json:"project"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Summary.RelativePath != "datasets/imported_plant.csv" || body.Summary.RowCount != 2 || len(body.Summary.SHA256) != 64 {
		t.Fatalf("summary = %#v", body.Summary)
	}
	if len(body.Dataset.ColumnProfiles) != 6 || body.Dataset.ColumnProfiles[0].ValueType != "number" {
		t.Fatalf("column profiles = %#v", body.Dataset.ColumnProfiles)
	}
	if body.Dataset.SuggestedTimeColumn != "time" {
		t.Fatalf("suggested time column = %q", body.Dataset.SuggestedTimeColumn)
	}
	importedBytes, err := os.ReadFile(filepath.Join(projectRoot, "datasets", "imported_plant.csv"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(importedBytes, []byte("time,building_load_kw,outdoor_temp_c")) {
		t.Fatalf("imported CSV was not normalized: %s", importedBytes)
	}
	if !hasDatasetSummary(body.Project.Datasets, "imported_plant") {
		t.Fatalf("project datasets = %#v", body.Project.Datasets)
	}

	mappingPayload, err := json.Marshal(map[string]any{
		"project_path":         projectPath,
		"dataset_path":         body.Summary.RelativePath,
		"id":                   "imported_plant_validation",
		"missing_value_policy": "fail_fast",
	})
	if err != nil {
		t.Fatal(err)
	}
	mappingResponse := httptest.NewRecorder()
	mappingRequest := httptest.NewRequest(http.MethodPost, "/api/project/validation-mapping", bytes.NewReader(mappingPayload))

	server.Handler().ServeHTTP(mappingResponse, mappingRequest)

	if mappingResponse.Code != http.StatusCreated {
		t.Fatalf("mapping status = %d body=%s", mappingResponse.Code, mappingResponse.Body.String())
	}
	var mappingBody struct {
		Summary ValidationMappingSummary `json:"summary"`
	}
	if err := json.Unmarshal(mappingResponse.Body.Bytes(), &mappingBody); err != nil {
		t.Fatal(err)
	}
	if mappingBody.Summary.Dataset != "datasets/imported_plant.csv" || mappingBody.Summary.DatasetChecksum != body.Summary.SHA256 {
		t.Fatalf("mapping summary = %#v", mappingBody.Summary)
	}
}

func TestImportDatasetEndpointDecodesSelectedEncoding(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	projectPath := createWorkflowTestProject(t, server, "Encoded Dataset Project")
	projectRoot := filepath.Dir(projectPath)

	utf16Source := filepath.Join(root, "incoming-utf16.csv")
	writeUTF16LECSV(t, utf16Source, "time;value\n0;1\n")
	utf16Body := importDatasetForTest(t, server, projectPath, utf16Source, "utf16_dataset", "auto", "auto")
	if utf16Body.Dataset.SourceEncoding != "utf-16" || utf16Body.Dataset.DetectedDelimiter != "semicolon" {
		t.Fatalf("utf16 import detection = encoding %q delimiter %q", utf16Body.Dataset.SourceEncoding, utf16Body.Dataset.DetectedDelimiter)
	}
	utf16Bytes, err := os.ReadFile(filepath.Join(projectRoot, "datasets", "utf16_dataset.csv"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(utf16Bytes, []byte("time,value")) {
		t.Fatalf("utf16 imported CSV was not normalized to UTF-8 comma CSV: %q", string(utf16Bytes))
	}

	cp949Source := filepath.Join(root, "incoming-cp949.csv")
	writeCP949CSV(t, cp949Source, "시간,값\n0,1\n")
	cp949Body := importDatasetForTest(t, server, projectPath, cp949Source, "cp949_dataset", "comma", "cp949")
	if cp949Body.Dataset.SourceEncoding != "cp949" || cp949Body.Dataset.Columns[0] != "시간" {
		t.Fatalf("cp949 import = encoding %q columns %#v", cp949Body.Dataset.SourceEncoding, cp949Body.Dataset.Columns)
	}
	cp949Bytes, err := os.ReadFile(filepath.Join(projectRoot, "datasets", "cp949_dataset.csv"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(cp949Bytes, []byte("시간,값")) {
		t.Fatalf("cp949 imported CSV was not normalized to UTF-8: %q", string(cp949Bytes))
	}
}

func TestParameterSetDetailEndpointReturnsDiffs(t *testing.T) {
	server := newTestServer(t)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/project/parameter-set?project_path="+url.QueryEscape("examples/005_chiller_plant_like_system/project.bcsproj")+"&path="+url.QueryEscape("parameter_sets/high_efficiency.json"),
		nil,
	)

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		ParameterSet ParameterSetDetail `json:"parameter_set"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.ParameterSet.Summary.ID != "high_efficiency" || len(body.ParameterSet.Differences) == 0 {
		t.Fatalf("parameter set detail = %#v", body.ParameterSet)
	}
	if !hasParameterDiff(body.ParameterSet.Differences, "chiller", "cop") {
		t.Fatalf("parameter diffs = %#v", body.ParameterSet.Differences)
	}
}

func TestCreateValidationMappingEndpointWritesSuggestedMapping(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	projectRoot := filepath.Join(root, "projects", "mapping-project")
	if err := copyProjectTree(filepath.Join(repoRoot, "examples", "005_chiller_plant_like_system"), projectRoot); err != nil {
		t.Fatal(err)
	}
	projectPath := filepath.Join(projectRoot, "project.bcsproj")
	payload, err := json.Marshal(map[string]any{
		"project_path":  projectPath,
		"dataset_path":  filepath.Join("datasets", "plant_validation.csv"),
		"id":            "suggested_validation",
		"time_column":   "time",
		"input_columns": map[string]string{"building_load_kw": "building_load_kw"},
		"observed_output_columns": map[string]string{
			"total_power_kw":         "measured_total_power_kw",
			"chw_supply_temp_c":      "measured_chw_supply_temp_c",
			"chiller_electric_power": "",
			"pump_electric_power":    "",
			"cooling_tower_power":    "",
		},
		"unit_hints": map[string]string{
			"building_load_kw":        "kW",
			"measured_total_power_kw": "kW",
		},
		"missing_value_policy": "fail_fast",
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/validation-mapping", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Summary ValidationMappingSummary `json:"summary"`
		Mapping struct {
			InputColumns          map[string]string `json:"input_columns"`
			ObservedOutputColumns map[string]string `json:"observed_output_columns"`
			TimeColumn            string            `json:"time_column"`
			UnitHints             map[string]string `json:"unit_hints"`
			MissingValuePolicy    string            `json:"missing_value_policy"`
		} `json:"mapping"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Summary.RelativePath != "validation/mappings/suggested_validation.json" || body.Summary.MissingValuePolicy != "error" || body.Mapping.MissingValuePolicy != "error" {
		t.Fatalf("summary = %#v", body.Summary)
	}
	if body.Mapping.InputColumns["building_load_kw"] != "building_load_kw" {
		t.Fatalf("input columns = %#v", body.Mapping.InputColumns)
	}
	if body.Mapping.ObservedOutputColumns["total_power_kw"] != "measured_total_power_kw" {
		t.Fatalf("output columns = %#v", body.Mapping.ObservedOutputColumns)
	}
	if body.Mapping.TimeColumn != "time" || body.Mapping.UnitHints["building_load_kw"] != "kW" {
		t.Fatalf("mapping time/unit hints = %#v", body.Mapping)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, "validation", "mappings", "suggested_validation.json")); err != nil {
		t.Fatal(err)
	}

	detailResponse := httptest.NewRecorder()
	detailRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/project/validation-mapping?project_path="+url.QueryEscape(projectPath)+"&path="+url.QueryEscape(filepath.Join("validation", "mappings", "suggested_validation.json")),
		nil,
	)

	server.Handler().ServeHTTP(detailResponse, detailRequest)

	if detailResponse.Code != http.StatusOK {
		t.Fatalf("detail status = %d body=%s", detailResponse.Code, detailResponse.Body.String())
	}
	var detailBody struct {
		Mapping struct {
			ID                    string            `json:"id"`
			ObservedOutputColumns map[string]string `json:"observed_output_columns"`
		} `json:"mapping"`
	}
	if err := json.Unmarshal(detailResponse.Body.Bytes(), &detailBody); err != nil {
		t.Fatal(err)
	}
	if detailBody.Mapping.ID != "suggested_validation" || detailBody.Mapping.ObservedOutputColumns["total_power_kw"] != "measured_total_power_kw" {
		t.Fatalf("mapping detail = %#v", detailBody.Mapping)
	}
}

func TestValidationMappingManagementEndpointRenamesCopiesAndDeletes(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	projectRoot := filepath.Join(root, "projects", "mapping-management-project")
	if err := copyProjectTree(filepath.Join(repoRoot, "examples", "005_chiller_plant_like_system"), projectRoot); err != nil {
		t.Fatal(err)
	}
	projectPath := filepath.Join(projectRoot, "project.bcsproj")
	mappingPath := filepath.Join("validation", "mappings", "plant_validation.json")

	updatePayload, err := json.Marshal(map[string]any{
		"project_path": projectPath,
		"mapping_path": mappingPath,
		"name":         "Plant Baseline Mapping",
	})
	if err != nil {
		t.Fatal(err)
	}
	updateResponse := httptest.NewRecorder()
	updateRequest := httptest.NewRequest(http.MethodPost, "/api/project/validation-mapping/update", bytes.NewReader(updatePayload))

	server.Handler().ServeHTTP(updateResponse, updateRequest)

	if updateResponse.Code != http.StatusOK {
		t.Fatalf("update status = %d body=%s", updateResponse.Code, updateResponse.Body.String())
	}
	var updateBody struct {
		Summary ValidationMappingSummary `json:"summary"`
		Mapping struct {
			Name string `json:"name"`
		} `json:"mapping"`
	}
	if err := json.Unmarshal(updateResponse.Body.Bytes(), &updateBody); err != nil {
		t.Fatal(err)
	}
	if updateBody.Summary.Name != "Plant Baseline Mapping" || updateBody.Mapping.Name != "Plant Baseline Mapping" {
		t.Fatalf("updated mapping = %#v", updateBody)
	}
	mappingBytes, err := os.ReadFile(filepath.Join(projectRoot, "validation", "mappings", "plant_validation.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(mappingBytes, []byte(`"name": "Plant Baseline Mapping"`)) {
		t.Fatalf("updated mapping file = %s", mappingBytes)
	}

	copyPayload, err := json.Marshal(map[string]any{
		"project_path": projectPath,
		"mapping_path": mappingPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	copyResponse := httptest.NewRecorder()
	copyRequest := httptest.NewRequest(http.MethodPost, "/api/project/validation-mapping/copy", bytes.NewReader(copyPayload))

	server.Handler().ServeHTTP(copyResponse, copyRequest)

	if copyResponse.Code != http.StatusCreated {
		t.Fatalf("copy status = %d body=%s", copyResponse.Code, copyResponse.Body.String())
	}
	var copyBody struct {
		Summary ValidationMappingSummary `json:"summary"`
	}
	if err := json.Unmarshal(copyResponse.Body.Bytes(), &copyBody); err != nil {
		t.Fatal(err)
	}
	if copyBody.Summary.RelativePath == "" || copyBody.Summary.RelativePath == filepath.ToSlash(mappingPath) || copyBody.Summary.Name != "Plant Baseline Mapping Copy" {
		t.Fatalf("copied mapping summary = %#v", copyBody.Summary)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, filepath.FromSlash(copyBody.Summary.RelativePath))); err != nil {
		t.Fatal(err)
	}

	deleteCopyPayload, err := json.Marshal(map[string]any{
		"project_path": projectPath,
		"mapping_path": copyBody.Summary.RelativePath,
	})
	if err != nil {
		t.Fatal(err)
	}
	deleteCopyResponse := httptest.NewRecorder()
	deleteCopyRequest := httptest.NewRequest(http.MethodPost, "/api/project/validation-mapping/delete", bytes.NewReader(deleteCopyPayload))

	server.Handler().ServeHTTP(deleteCopyResponse, deleteCopyRequest)

	if deleteCopyResponse.Code != http.StatusOK {
		t.Fatalf("delete copy status = %d body=%s", deleteCopyResponse.Code, deleteCopyResponse.Body.String())
	}
	if _, err := os.Stat(filepath.Join(projectRoot, filepath.FromSlash(copyBody.Summary.RelativePath))); !os.IsNotExist(err) {
		t.Fatalf("copied mapping still exists or stat failed unexpectedly: %v", err)
	}

	deleteReferencedPayload, err := json.Marshal(map[string]any{
		"project_path": projectPath,
		"mapping_path": mappingPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	deleteReferencedResponse := httptest.NewRecorder()
	deleteReferencedRequest := httptest.NewRequest(http.MethodPost, "/api/project/validation-mapping/delete", bytes.NewReader(deleteReferencedPayload))

	server.Handler().ServeHTTP(deleteReferencedResponse, deleteReferencedRequest)

	if deleteReferencedResponse.Code == http.StatusOK || !strings.Contains(deleteReferencedResponse.Body.String(), "calibration setup") {
		t.Fatalf("referenced delete status = %d body=%s", deleteReferencedResponse.Code, deleteReferencedResponse.Body.String())
	}
}

func TestCreateCalibrationSetupEndpointWritesRoleBasedSetup(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	projectRoot := filepath.Join(root, "projects", "calibration-setup-project")
	if err := copyProjectTree(filepath.Join(repoRoot, "examples", "005_chiller_plant_like_system"), projectRoot); err != nil {
		t.Fatal(err)
	}
	projectPath := filepath.Join(projectRoot, "project.bcsproj")
	payload, err := json.Marshal(map[string]any{
		"project_path": projectPath,
		"mapping_path": filepath.Join("validation", "mappings", "plant_validation.json"),
		"id":           "auto_calibration",
		"algorithm":    "differential_evolution",
		"stopping_rules": map[string]any{
			"max_candidates":      3,
			"objective_tolerance": 0.01,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/calibration-setup", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Summary CalibrationSetupSummary `json:"summary"`
		Setup   calibration.Setup       `json:"setup"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Summary.RelativePath != "calibration/setups/auto_calibration.json" || body.Summary.ParameterCount == 0 {
		t.Fatalf("summary = %#v", body.Summary)
	}
	if body.Summary.Algorithm != "differential_evolution" || body.Setup.Algorithm != "differential_evolution" {
		t.Fatalf("algorithm summary=%#v setup=%#v", body.Summary, body.Setup)
	}
	if body.Setup.StoppingRules.MaxCandidates != 3 || body.Setup.StoppingRules.ObjectiveTolerance != 0.01 {
		t.Fatalf("stopping rules = %#v", body.Setup.StoppingRules)
	}
	if body.Setup.Objective.Metric != "rmse" || body.Setup.Objective.Outputs["total_power_kw"] != 1 {
		t.Fatalf("objective = %#v", body.Setup.Objective)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, "calibration", "setups", "auto_calibration.json")); err != nil {
		t.Fatal(err)
	}
}

func TestCreateOptimizationSetupEndpointWritesPublicInputSetup(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	projectRoot := filepath.Join(root, "projects", "optimization-setup-project")
	if err := copyProjectTree(filepath.Join(repoRoot, "examples", "006_optimization_case"), projectRoot); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(projectRoot, "parameter_sets", "base.json"), `{"id":"base","components":{}}`)
	projectPath := filepath.Join(projectRoot, "project.bcsproj")
	payload, err := json.Marshal(map[string]any{
		"project_path":       projectPath,
		"id":                 "auto_optimization",
		"algorithm":          "differential_evolution",
		"base_parameter_set": filepath.Join("parameter_sets", "base.json"),
		"base_inputs": map[string]any{
			"building_load_kw": 500.0,
			"chw_setpoint_c":   7.5,
		},
		"context": map[string]any{"time": 0.0, "dt": 60.0},
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/optimization-setup", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Summary OptimizationSetupSummary `json:"summary"`
		Setup   optimization.Setup       `json:"setup"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Summary.RelativePath != "optimization/setups/auto_optimization.json" || body.Summary.VariableCount != 1 {
		t.Fatalf("summary = %#v", body.Summary)
	}
	if body.Summary.BaseParameterSet != "parameter_sets/base.json" || body.Setup.BaseParameterSet != "parameter_sets/base.json" {
		t.Fatalf("base parameter set summary=%#v setup=%#v", body.Summary, body.Setup)
	}
	if body.Summary.Algorithm != "differential_evolution" || body.Setup.Algorithm != "differential_evolution" {
		t.Fatalf("algorithm summary=%#v setup=%#v", body.Summary, body.Setup)
	}
	if body.Setup.Objective.Output != "objective_kw" || body.Setup.DecisionVariables[0].Name != "chw_setpoint_c" {
		t.Fatalf("optimization setup = %#v", body.Setup)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, "optimization", "setups", "auto_optimization.json")); err != nil {
		t.Fatal(err)
	}
}

func TestCreateOptimizationSetupEndpointWritesSystemParameterSetup(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	projectRoot := filepath.Join(root, "projects", "system-parameter-optimization")
	if err := copyProjectTree(filepath.Join(repoRoot, "examples", "006_optimization_case"), projectRoot); err != nil {
		t.Fatal(err)
	}
	projectPath := filepath.Join(projectRoot, "project.bcsproj")
	payload, err := json.Marshal(map[string]any{
		"project_path": projectPath,
		"id":           "system_parameter_optimization",
		"algorithm":    "custom_sdk_script",
		"base_inputs": map[string]any{
			"building_load_kw": 500.0,
			"chw_setpoint_c":   7.0,
		},
		"context": map[string]any{"time": 0.0, "dt": 60.0},
		"objective": map[string]any{
			"output": "chiller_power_kw",
			"sense":  "min",
		},
		"decision_variables": []map[string]any{{
			"kind":      "system_parameter",
			"component": "tradeoff",
			"name":      "power_credit_kw_per_k",
			"min":       4.0,
			"max":       12.0,
			"step":      4.0,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/optimization-setup", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Summary OptimizationSetupSummary `json:"summary"`
		Setup   optimization.Setup       `json:"setup"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Summary.Algorithm != "custom_sdk_script" || body.Setup.Algorithm != "custom_sdk_script" {
		t.Fatalf("algorithm summary=%#v setup=%#v", body.Summary, body.Setup)
	}
	if body.Setup.DecisionVariables[0].Kind != "system_parameter" || body.Setup.DecisionVariables[0].Component != "tradeoff" {
		t.Fatalf("optimization setup = %#v", body.Setup)
	}
}

func TestApplyParameterSetEndpointPersistsGraphParameters(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	projectRoot := filepath.Join(root, "projects", "parameter-project")
	if err := copyProjectTree(filepath.Join(repoRoot, "examples", "005_chiller_plant_like_system"), projectRoot); err != nil {
		t.Fatal(err)
	}
	projectPath := filepath.Join(projectRoot, "project.bcsproj")
	payload, err := json.Marshal(map[string]any{
		"project_path": projectPath,
		"path":         filepath.Join("parameter_sets", "high_efficiency.json"),
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/parameter-set/apply", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	loaded, err := project.Load(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	component, ok := findComponent(loaded.Graph, "chiller")
	if !ok {
		t.Fatal("chiller component not found")
	}
	if component.Parameters["cop"] != float64(6.8) {
		t.Fatalf("chiller cop = %#v", component.Parameters["cop"])
	}
}

func TestDataValidationEndpointSavesWorkspaceRecord(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	projectRoot := filepath.Join(root, "projects", "plant-validation")
	if err := copyProjectTree(filepath.Join(repoRoot, "examples", "005_chiller_plant_like_system"), projectRoot); err != nil {
		t.Fatal(err)
	}
	projectPath := filepath.Join(projectRoot, "project.bcsproj")
	payload, err := json.Marshal(map[string]any{
		"project_path":       projectPath,
		"mapping_path":       filepath.Join("validation", "mappings", "plant_validation.json"),
		"parameter_set_path": filepath.Join("parameter_sets", "high_efficiency.json"),
		"high_error_rows":    1,
		"save":               true,
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/validation/run", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		ValidationResult struct {
			SavedRecord string `json:"saved_record"`
		} `json:"validation_result"`
		ValidationRecord struct {
			ID           string `json:"id"`
			RelativePath string `json:"relative_path"`
			RowCount     int    `json:"row_count"`
		} `json:"validation_record"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.ValidationRecord.ID == "" || body.ValidationRecord.RowCount != 3 {
		t.Fatalf("validation record = %#v", body.ValidationRecord)
	}
	if body.ValidationResult.SavedRecord != body.ValidationRecord.RelativePath {
		t.Fatalf("saved record = %q, summary path = %q", body.ValidationResult.SavedRecord, body.ValidationRecord.RelativePath)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, filepath.FromSlash(body.ValidationRecord.RelativePath))); err != nil {
		t.Fatal(err)
	}

	detailResponse := httptest.NewRecorder()
	detailRequest := httptest.NewRequest(http.MethodGet, "/api/project?project_path="+url.QueryEscape(projectPath), nil)
	server.Handler().ServeHTTP(detailResponse, detailRequest)
	if detailResponse.Code != http.StatusOK {
		t.Fatalf("detail status = %d body=%s", detailResponse.Code, detailResponse.Body.String())
	}
	var detailBody struct {
		Project ProjectDetail `json:"project"`
	}
	if err := json.Unmarshal(detailResponse.Body.Bytes(), &detailBody); err != nil {
		t.Fatal(err)
	}
	if len(detailBody.Project.ValidationRuns) != 1 || detailBody.Project.ValidationRuns[0].ID != body.ValidationRecord.ID {
		t.Fatalf("validation run summaries = %#v", detailBody.Project.ValidationRuns)
	}

	openResponse := httptest.NewRecorder()
	openRequest := httptest.NewRequest(http.MethodGet, "/api/project/validation-record?project_path="+url.QueryEscape(projectPath)+"&record_id="+url.QueryEscape(body.ValidationRecord.ID), nil)
	server.Handler().ServeHTTP(openResponse, openRequest)
	if openResponse.Code != http.StatusOK {
		t.Fatalf("open status = %d body=%s", openResponse.Code, openResponse.Body.String())
	}
	var openBody struct {
		ValidationRecord struct {
			ID     string `json:"id"`
			Result struct {
				RowCount int `json:"row_count"`
			} `json:"result"`
		} `json:"validation_record"`
	}
	if err := json.Unmarshal(openResponse.Body.Bytes(), &openBody); err != nil {
		t.Fatal(err)
	}
	if openBody.ValidationRecord.ID != body.ValidationRecord.ID || openBody.ValidationRecord.Result.RowCount != 3 {
		t.Fatalf("opened record = %#v", openBody.ValidationRecord)
	}
}

func TestWorkflowRunEndpointsRejectSavedSourceContractErrors(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	project := createWorkspaceProject(t, server, "Workflow Source Gate Project")
	projectRoot := filepath.Dir(project.ProjectPath)
	seedExportWorkflowArtifacts(t, projectRoot)
	writeBrokenScalarSource(t, project)

	tests := []struct {
		name    string
		path    string
		payload map[string]any
	}{
		{
			name: "validation",
			path: "/api/validation/run",
			payload: map[string]any{
				"project_path": project.ProjectPath,
				"mapping_path": filepath.Join("validation", "mappings", "scalar_validation.json"),
			},
		},
		{
			name: "calibration",
			path: "/api/calibration/run",
			payload: map[string]any{
				"project_path": project.ProjectPath,
				"setup_path":   filepath.Join("calibration", "setups", "scalar_gain.json"),
			},
		},
		{
			name: "optimization",
			path: "/api/optimization/run",
			payload: map[string]any{
				"project_path": project.ProjectPath,
				"setup_path":   filepath.Join("optimization", "setups", "scalar_grid.json"),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertSourceGateRejectsRequest(t, server, http.MethodPost, test.path, test.payload)
		})
	}
}

func TestCalibrationRunEndpointSavesWorkspaceRecord(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	projectRoot := filepath.Join(root, "projects", "calibration-project")
	if err := copyProjectTree(filepath.Join(repoRoot, "examples", "005_chiller_plant_like_system"), projectRoot); err != nil {
		t.Fatal(err)
	}
	projectPath := filepath.Join(projectRoot, "project.bcsproj")
	payload, err := json.Marshal(map[string]any{
		"project_path": projectPath,
		"setup_path":   filepath.Join("calibration", "setups", "chiller_cop_grid.json"),
		"save":         true,
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/calibration/run", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		CalibrationResult struct {
			OK                bool   `json:"ok"`
			SavedParameterSet string `json:"saved_parameter_set"`
			SavedRecord       string `json:"saved_record"`
		} `json:"calibration_result"`
		CalibrationRecord struct {
			ID           string `json:"id"`
			RelativePath string `json:"relative_path"`
		} `json:"calibration_record"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.CalibrationResult.OK || body.CalibrationResult.SavedParameterSet != "parameter_sets/chiller_cop_grid_calibrated.json" {
		t.Fatalf("calibration result = %#v", body.CalibrationResult)
	}
	if body.CalibrationRecord.ID == "" || body.CalibrationResult.SavedRecord != body.CalibrationRecord.RelativePath {
		t.Fatalf("calibration record = %#v result=%#v", body.CalibrationRecord, body.CalibrationResult)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, "parameter_sets", "chiller_cop_grid_calibrated.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, filepath.FromSlash(body.CalibrationRecord.RelativePath))); err != nil {
		t.Fatal(err)
	}
}

func TestOptimizationRunEndpointSavesWorkspaceRecord(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	projectRoot := filepath.Join(root, "projects", "optimization-project")
	if err := copyProjectTree(filepath.Join(repoRoot, "examples", "006_optimization_case"), projectRoot); err != nil {
		t.Fatal(err)
	}
	projectPath := filepath.Join(projectRoot, "project.bcsproj")
	payload, err := json.Marshal(map[string]any{
		"project_path": projectPath,
		"setup_path":   filepath.Join("optimization", "setups", "chw_setpoint_grid.json"),
		"save":         true,
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/optimization/run", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		OptimizationResult struct {
			OK            bool   `json:"ok"`
			SavedScenario string `json:"saved_scenario"`
			SavedRecord   string `json:"saved_record"`
		} `json:"optimization_result"`
		OptimizationRecord struct {
			ID           string `json:"id"`
			RelativePath string `json:"relative_path"`
		} `json:"optimization_record"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.OptimizationResult.OK || body.OptimizationResult.SavedScenario != "scenarios/chw_setpoint_grid_optimized.json" {
		t.Fatalf("optimization result = %#v", body.OptimizationResult)
	}
	if body.OptimizationRecord.ID == "" || body.OptimizationResult.SavedRecord != body.OptimizationRecord.RelativePath {
		t.Fatalf("optimization record = %#v result=%#v", body.OptimizationRecord, body.OptimizationResult)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, "scenarios", "chw_setpoint_grid_optimized.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, filepath.FromSlash(body.OptimizationRecord.RelativePath))); err != nil {
		t.Fatal(err)
	}
}

func TestOptimizationRunEndpointSavesParameterSetForParameterVariables(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	projectRoot := filepath.Join(root, "projects", "parameter-optimization-project")
	if err := copyProjectTree(filepath.Join(repoRoot, "examples", "006_optimization_case"), projectRoot); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(projectRoot, "optimization", "setups", "parameter_credit_grid.json"), `{
  "id": "parameter_credit_grid",
  "name": "Parameter Credit Grid",
  "algorithm": "grid",
  "base_inputs": {
    "building_load_kw": 500.0,
    "chw_setpoint_c": 7.0
  },
  "context": {
    "time": 0,
    "dt": 60
  },
  "objective": {
    "output": "chiller_power_kw",
    "sense": "min"
  },
  "decision_variables": [
    {
      "kind": "component_parameter",
      "component": "tradeoff",
      "name": "power_credit_kw_per_k",
      "min": 4.0,
      "max": 12.0,
      "step": 4.0
    }
  ],
  "constraints": [
    {
      "output": "comfort_penalty_kw",
      "operator": "<=",
      "value": 0
    }
  ]
}
`)
	projectPath := filepath.Join(projectRoot, "project.bcsproj")
	payload, err := json.Marshal(map[string]any{
		"project_path": projectPath,
		"setup_path":   filepath.Join("optimization", "setups", "parameter_credit_grid.json"),
		"save":         true,
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/optimization/run", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		OptimizationResult struct {
			OK                bool   `json:"ok"`
			SavedScenario     string `json:"saved_scenario"`
			SavedParameterSet string `json:"saved_parameter_set"`
		} `json:"optimization_result"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.OptimizationResult.OK || body.OptimizationResult.SavedParameterSet != "parameter_sets/parameter_credit_grid_optimized.json" {
		t.Fatalf("optimization result = %#v", body.OptimizationResult)
	}
	if body.OptimizationResult.SavedScenario != "scenarios/parameter_credit_grid_optimized.json" {
		t.Fatalf("saved scenario = %q", body.OptimizationResult.SavedScenario)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, "parameter_sets", "parameter_credit_grid_optimized.json")); err != nil {
		t.Fatal(err)
	}
}

func createWorkflowTestProject(t *testing.T, server *Server, name string) string {
	t.Helper()
	payload, err := json.Marshal(map[string]any{"name": name})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader(payload))
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("create project status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Project ProjectSummary `json:"project"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body.Project.ProjectPath
}

type datasetImportTestBody struct {
	Summary DatasetSummary `json:"summary"`
	Dataset DatasetPreview `json:"dataset"`
	Project ProjectDetail  `json:"project"`
}

func importDatasetForTest(t *testing.T, server *Server, projectPath string, sourcePath string, id string, delimiter string, encoding string) datasetImportTestBody {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"project_path": projectPath,
		"source_path":  sourcePath,
		"id":           id,
		"delimiter":    delimiter,
		"encoding":     encoding,
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/datasets/import", bytes.NewReader(payload))
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("import dataset status = %d body=%s", response.Code, response.Body.String())
	}
	var body datasetImportTestBody
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body
}

func writeUTF16LECSV(t *testing.T, path string, content string) {
	t.Helper()
	var buffer bytes.Buffer
	buffer.Write([]byte{0xFF, 0xFE})
	for _, value := range utf16.Encode([]rune(content)) {
		if err := binary.Write(&buffer, binary.LittleEndian, value); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(path, buffer.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeCP949CSV(t *testing.T, path string, content string) {
	t.Helper()
	encoded, _, err := transform.String(korean.EUCKR.NewEncoder(), content)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(encoded), 0o644); err != nil {
		t.Fatal(err)
	}
}
