package studio

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goniegonie/hvac-studio/tools/go/internal/compiler"
	"github.com/goniegonie/hvac-studio/tools/go/internal/model"
	"github.com/goniegonie/hvac-studio/tools/go/internal/project"
	runtimecore "github.com/goniegonie/hvac-studio/tools/go/internal/runtime"
	"github.com/goniegonie/hvac-studio/tools/go/internal/schemaexport"
)

func TestAcceptanceWalkthroughFirstProjectComponentRunExport(t *testing.T) {
	root, server := newIsolatedTestServer(t)

	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Acceptance Walkthrough"}`)))
	server.Handler().ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", createResponse.Code, createResponse.Body.String())
	}
	var createBody struct {
		Project ProjectSummary `json:"project"`
	}
	if err := json.Unmarshal(createResponse.Body.Bytes(), &createBody); err != nil {
		t.Fatal(err)
	}

	componentPayload, err := json.Marshal(map[string]any{
		"project_path":      createBody.Project.ProjectPath,
		"name":              "Walkthrough Gain",
		"template":          "scalar",
		"include_in_system": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	componentResponse := httptest.NewRecorder()
	componentRequest := httptest.NewRequest(http.MethodPost, "/api/project/components", bytes.NewReader(componentPayload))
	server.Handler().ServeHTTP(componentResponse, componentRequest)
	if componentResponse.Code != http.StatusCreated {
		t.Fatalf("component status = %d body=%s", componentResponse.Code, componentResponse.Body.String())
	}
	var componentBody struct {
		Component model.Component `json:"component"`
	}
	if err := json.Unmarshal(componentResponse.Body.Bytes(), &componentBody); err != nil {
		t.Fatal(err)
	}
	if componentBody.Component.ID != "walkthrough_gain" || componentBody.Component.Source.Layout != "generated_wrapper" {
		t.Fatalf("component = %#v", componentBody.Component)
	}

	source := `from .helpers import apply_gain


def step(inputs, state, params, context):
    value = float(inputs["value"])
    gain = float(params.get("gain", 2.0))
    return {"result": apply_gain(value, gain) + 1.0}, state
`
	sourcePayload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"component_id": "walkthrough_gain",
		"content":      source,
	})
	if err != nil {
		t.Fatal(err)
	}
	sourceResponse := httptest.NewRecorder()
	sourceRequest := httptest.NewRequest(http.MethodPost, "/api/project/source", bytes.NewReader(sourcePayload))
	server.Handler().ServeHTTP(sourceResponse, sourceRequest)
	if sourceResponse.Code != http.StatusOK {
		t.Fatalf("source status = %d body=%s", sourceResponse.Code, sourceResponse.Body.String())
	}
	var sourceBody struct {
		Check SourceCheck `json:"check"`
	}
	if err := json.Unmarshal(sourceResponse.Body.Bytes(), &sourceBody); err != nil {
		t.Fatal(err)
	}
	if !sourceBody.Check.OK {
		t.Fatalf("source check = %#v", sourceBody.Check)
	}

	runPayload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"inputs": map[string]any{
			"value":                  3.0,
			"walkthrough_gain_value": 4.0,
		},
		"context": map[string]any{"time": 0.0, "dt": 60.0},
		"save":    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	runResponse := httptest.NewRecorder()
	runRequest := httptest.NewRequest(http.MethodPost, "/api/run", bytes.NewReader(runPayload))
	server.Handler().ServeHTTP(runResponse, runRequest)
	if runResponse.Code != http.StatusOK {
		t.Fatalf("run status = %d body=%s", runResponse.Code, runResponse.Body.String())
	}
	var runBody struct {
		Result struct {
			Outputs map[string]float64 `json:"outputs"`
		} `json:"result"`
		RunRecord RunSummary `json:"run_record"`
	}
	if err := json.Unmarshal(runResponse.Body.Bytes(), &runBody); err != nil {
		t.Fatal(err)
	}
	if runBody.Result.Outputs["result"] != 6.0 || runBody.Result.Outputs["walkthrough_gain_result"] != 9.0 {
		t.Fatalf("run outputs = %#v", runBody.Result.Outputs)
	}
	if runBody.RunRecord.ID == "" {
		t.Fatalf("run record = %#v", runBody.RunRecord)
	}

	seedTestRuntimeSupport(t, root)
	exportPayload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"profile":      "runtime_package",
	})
	if err != nil {
		t.Fatal(err)
	}
	exportResponse := httptest.NewRecorder()
	exportRequest := httptest.NewRequest(http.MethodPost, "/api/export", bytes.NewReader(exportPayload))
	server.Handler().ServeHTTP(exportResponse, exportRequest)
	if exportResponse.Code != http.StatusOK {
		t.Fatalf("export status = %d body=%s", exportResponse.Code, exportResponse.Body.String())
	}
	var exportBody struct {
		Export ExportManifest `json:"export"`
	}
	if err := json.Unmarshal(exportResponse.Body.Bytes(), &exportBody); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{
		"project/components/walkthrough_gain/user_step.py",
		"run-default.ps1",
		"sdk-example.py",
		"python/bcs_sdk/bcs_sdk/client.py",
	} {
		if !containsString(exportBody.Export.Files, rel) {
			t.Fatalf("export files missing %s in %v", rel, exportBody.Export.Files)
		}
	}
	exportedProject := filepath.Join(root, "projects", "acceptance-walkthrough", "exports", "runtime_package", "project", "project.bcsproj")
	exportedLoaded, err := project.Load(exportedProject)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := compiler.Compile(exportedLoaded); err != nil {
		t.Fatal(err)
	}
}

func TestAcceptanceWalkthroughErrorRecovery(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	summary := createWorkspaceProject(t, server, "Acceptance Error Recovery")

	postJSON := func(path string, payload map[string]any) *httptest.ResponseRecorder {
		t.Helper()
		body, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
		server.Handler().ServeHTTP(response, request)
		return response
	}

	missingOutputSource := strings.TrimLeft(`
class ScalarComponent:
    def evaluate(self, inputs, state, params, context):
        value = float(inputs["value"])
        return {}, state
`, "\n")
	sourceResponse := postJSON("/api/project/source", map[string]any{
		"project_path": summary.ProjectPath,
		"component_id": "scalar",
		"content":      missingOutputSource,
	})
	if sourceResponse.Code != http.StatusOK {
		t.Fatalf("source status = %d body=%s", sourceResponse.Code, sourceResponse.Body.String())
	}
	var sourceBody struct {
		Check SourceCheck `json:"check"`
	}
	if err := json.Unmarshal(sourceResponse.Body.Bytes(), &sourceBody); err != nil {
		t.Fatal(err)
	}
	if !sourceBody.Check.OK {
		t.Fatalf("missing-output source check should remain fixable warning-only = %#v", sourceBody.Check.Problems)
	}
	outputProblem, ok := findProblemMessageContaining(sourceBody.Check.Problems, "output node is not obviously returned by source: result")
	if !ok {
		t.Fatalf("missing-output warning missing from %#v", sourceBody.Check.Problems)
	}
	if outputProblem.Severity != "warning" || outputProblem.Source != "components/scalar.py" || outputProblem.Line != 4 || outputProblem.NodeID != "result" {
		t.Fatalf("missing-output warning location = %#v", outputProblem)
	}

	failedRunResponse := postJSON("/api/run", map[string]any{
		"project_path": summary.ProjectPath,
		"inputs":       map[string]any{"value": 4},
	})
	if failedRunResponse.Code != http.StatusBadGateway {
		t.Fatalf("failed run status = %d body=%s", failedRunResponse.Code, failedRunResponse.Body.String())
	}
	var failedRun apiError
	if err := json.Unmarshal(failedRunResponse.Body.Bytes(), &failedRun); err != nil {
		t.Fatal(err)
	}
	runProblem, ok := findProblemMessageContaining(failedRun.Problems, "did not return declared output node: result")
	if !ok {
		t.Fatalf("missing-output run problem missing from %#v", failedRun.Problems)
	}
	if runProblem.ComponentID != "scalar" || runProblem.NodeID != "result" {
		t.Fatalf("missing-output run problem = %#v", runProblem)
	}

	fixedSource := strings.TrimLeft(`
class ScalarComponent:
    def evaluate(self, inputs, state, params, context):
        value = float(inputs["value"])
        return {"result": value * 2.0}, state
`, "\n")
	fixedSourceResponse := postJSON("/api/project/source", map[string]any{
		"project_path": summary.ProjectPath,
		"component_id": "scalar",
		"content":      fixedSource,
	})
	if fixedSourceResponse.Code != http.StatusOK {
		t.Fatalf("fixed source status = %d body=%s", fixedSourceResponse.Code, fixedSourceResponse.Body.String())
	}
	var fixedSourceBody struct {
		Check SourceCheck `json:"check"`
	}
	if err := json.Unmarshal(fixedSourceResponse.Body.Bytes(), &fixedSourceBody); err != nil {
		t.Fatal(err)
	}
	if !fixedSourceBody.Check.OK || len(fixedSourceBody.Check.Problems) != 0 {
		t.Fatalf("fixed source check = %#v", fixedSourceBody.Check)
	}
	successRunResponse := postJSON("/api/run", map[string]any{
		"project_path": summary.ProjectPath,
		"inputs":       map[string]any{"value": 4},
	})
	if successRunResponse.Code != http.StatusOK {
		t.Fatalf("success run status = %d body=%s", successRunResponse.Code, successRunResponse.Body.String())
	}
	var successRun struct {
		Result struct {
			Outputs map[string]float64 `json:"outputs"`
		} `json:"result"`
	}
	if err := json.Unmarshal(successRunResponse.Body.Bytes(), &successRun); err != nil {
		t.Fatal(err)
	}
	if successRun.Result.Outputs["result"] != 8 {
		t.Fatalf("success outputs = %#v", successRun.Result.Outputs)
	}

	componentResponse := postJSON("/api/project/components", map[string]any{
		"project_path": summary.ProjectPath,
		"name":         "Unit Sink",
	})
	if componentResponse.Code != http.StatusCreated {
		t.Fatalf("component status = %d body=%s", componentResponse.Code, componentResponse.Body.String())
	}
	includeResponse := postJSON("/api/project/system/components", map[string]any{
		"project_path": summary.ProjectPath,
		"component_id": "unit_sink",
	})
	if includeResponse.Code != http.StatusOK {
		t.Fatalf("include status = %d body=%s", includeResponse.Code, includeResponse.Body.String())
	}

	loaded, err := project.Load(summary.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	for index := range loaded.Graph.Components {
		switch loaded.Graph.Components[index].ID {
		case "scalar":
			loaded.Graph.Components[index].Nodes.Outputs[0].Unit = "W"
		case "unit_sink":
			loaded.Graph.Components[index].Nodes.Inputs[0].Unit = "kW"
		}
	}
	if err := writeJSONFile(loaded.GraphPath, loaded.Graph); err != nil {
		t.Fatal(err)
	}

	connectionResponse := postJSON("/api/project/connections", map[string]any{
		"project_path":   summary.ProjectPath,
		"from_component": "scalar",
		"from_node":      "result",
		"to_component":   "unit_sink",
		"to_node":        "value",
	})
	if connectionResponse.Code != http.StatusCreated {
		t.Fatalf("connection status = %d body=%s", connectionResponse.Code, connectionResponse.Body.String())
	}
	var connectionBody struct {
		Connection model.Connection `json:"connection"`
	}
	if err := json.Unmarshal(connectionResponse.Body.Bytes(), &connectionBody); err != nil {
		t.Fatal(err)
	}

	validateResponse := postJSON("/api/validate", map[string]any{"project_path": summary.ProjectPath})
	if validateResponse.Code != http.StatusOK {
		t.Fatalf("validate status = %d body=%s", validateResponse.Code, validateResponse.Body.String())
	}
	var validateBody struct {
		Validation struct {
			Problems []Problem `json:"problems"`
		} `json:"validation"`
	}
	if err := json.Unmarshal(validateResponse.Body.Bytes(), &validateBody); err != nil {
		t.Fatal(err)
	}
	unitProblem, ok := findProblemMessageContaining(validateBody.Validation.Problems, "unit mismatch without conversion")
	if !ok {
		t.Fatalf("unit mismatch warning missing from %#v", validateBody.Validation.Problems)
	}
	if unitProblem.Severity != "warning" || unitProblem.ComponentID != "unit_sink" || unitProblem.NodeID != "value" {
		t.Fatalf("unit mismatch problem = %#v", unitProblem)
	}

	conversionResponse := postJSON("/api/project/connections/update", map[string]any{
		"project_path":  summary.ProjectPath,
		"connection_id": connectionBody.Connection.ID,
		"unit_conversion": map[string]any{
			"factor":      0.001,
			"description": "W to kW",
		},
	})
	if conversionResponse.Code != http.StatusOK {
		t.Fatalf("conversion status = %d body=%s", conversionResponse.Code, conversionResponse.Body.String())
	}
	resolvedValidateResponse := postJSON("/api/validate", map[string]any{"project_path": summary.ProjectPath})
	if resolvedValidateResponse.Code != http.StatusOK {
		t.Fatalf("resolved validate status = %d body=%s", resolvedValidateResponse.Code, resolvedValidateResponse.Body.String())
	}
	var resolvedValidateBody struct {
		Validation struct {
			Problems []Problem `json:"problems"`
		} `json:"validation"`
	}
	if err := json.Unmarshal(resolvedValidateResponse.Body.Bytes(), &resolvedValidateBody); err != nil {
		t.Fatal(err)
	}
	if _, ok := findProblemMessageContaining(resolvedValidateBody.Validation.Problems, "unit mismatch without conversion"); ok {
		t.Fatalf("unit mismatch warning should be resolved: %#v", resolvedValidateBody.Validation.Problems)
	}
}

func TestExportEndpointWritesRuntimeArtifact(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	seedTestRuntimeSupport(t, root)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Export Project"}`)))
	server.Handler().ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", createResponse.Code, createResponse.Body.String())
	}
	var createBody struct {
		Project ProjectSummary `json:"project"`
	}
	if err := json.Unmarshal(createResponse.Body.Bytes(), &createBody); err != nil {
		t.Fatal(err)
	}
	seedExportWorkflowArtifacts(t, filepath.Join(root, "projects", "export-project"))

	payload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"profile":      "runtime_package",
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/export", bytes.NewReader(payload))
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Summary ExportSummary  `json:"summary"`
		Export  ExportManifest `json:"export"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Summary.RelativePath != "exports/runtime_package/manifest.json" {
		t.Fatalf("relative path = %s", body.Summary.RelativePath)
	}
	if body.Export.ProjectRoot != "project" {
		t.Fatalf("project root = %s", body.Export.ProjectRoot)
	}
	if body.Export.ProjectPath != "project/project.bcsproj" {
		t.Fatalf("project path = %s", body.Export.ProjectPath)
	}
	if body.Export.GraphPath != "project/graph.json" {
		t.Fatalf("graph path = %s", body.Export.GraphPath)
	}
	if body.Export.DefaultInput != "project/inputs/case01.json" {
		t.Fatalf("default input = %s", body.Export.DefaultInput)
	}
	if body.Export.EnvironmentLockfile != "project/requirements.lock.txt" {
		t.Fatalf("environment lockfile = %s", body.Export.EnvironmentLockfile)
	}
	if body.Export.InterfaceSchema != "schema/public-io.json" {
		t.Fatalf("interface schema = %s", body.Export.InterfaceSchema)
	}
	if body.Export.Runner != "bin/bcs-runner.exe" {
		t.Fatalf("runner = %s", body.Export.Runner)
	}
	expectedFiles := []string{
		"README.md",
		"bin/bcs-env.exe",
		"bin/bcs-runner.exe",
		"calibrate.ps1",
		"check-env.ps1",
		"docs/CLI_Guide.md",
		"optimize.ps1",
		"optimize-sdk.py",
		"project/project.bcsproj",
		"project/graph.json",
		"project/components/__init__.py",
		"project/components/scalar.py",
		"project/datasets/scalar_validation.csv",
		"project/parameter_sets/baseline.json",
		"project/scenarios/case01.json",
		"project/validation/mappings/scalar_validation.json",
		"project/calibration/setups/scalar_gain.json",
		"project/optimization/setups/scalar_grid.json",
		"project/inputs/case01.json",
		"project/requirements.lock.txt",
		"python/bcs_sdk/bcs_sdk/__init__.py",
		"python/bcs_sdk/bcs_sdk/client.py",
		"runtime/manifest.json",
		"runtime/python/python.exe",
		"schema/serve-request.schema.json",
		"schema/serve-response.schema.json",
		"run-batch.ps1",
		"run-default.ps1",
		"run-scenario.ps1",
		"schema/public-io.json",
		"sdk-example.py",
		"serve.ps1",
		"validate-data.ps1",
	}
	exportRoot := filepath.Join(root, "projects", "export-project", "exports", "runtime_package")
	assertRuntimeExportHasNoSourceCheckoutPaths(t, exportRoot, root, filepath.Join(root, "projects", "export-project"))
	for _, rel := range expectedFiles {
		if !containsString(body.Export.Files, rel) {
			t.Fatalf("export files missing %s in %v", rel, body.Export.Files)
		}
		if _, err := os.Stat(filepath.Join(exportRoot, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("export file %s: %v", rel, err)
		}
	}
	if !containsString(body.Export.ParameterSets, "project/parameter_sets/baseline.json") {
		t.Fatalf("export parameter sets = %v", body.Export.ParameterSets)
	}
	if !containsString(body.Export.Datasets, "project/datasets/scalar_validation.csv") {
		t.Fatalf("export datasets = %v", body.Export.Datasets)
	}
	if !containsString(body.Export.ValidationMappings, "project/validation/mappings/scalar_validation.json") {
		t.Fatalf("export validation mappings = %v", body.Export.ValidationMappings)
	}
	if !containsString(body.Export.CalibrationSetups, "project/calibration/setups/scalar_gain.json") {
		t.Fatalf("export calibration setups = %v", body.Export.CalibrationSetups)
	}
	if !containsString(body.Export.OptimizationSetups, "project/optimization/setups/scalar_grid.json") {
		t.Fatalf("export optimization setups = %v", body.Export.OptimizationSetups)
	}
	if !body.Export.IncludeDatasets || !body.Export.IncludeCalibration || !body.Export.IncludeOptimization || !body.Export.IncludeMLAssets || !body.Export.IncludeSDKExamples {
		t.Fatalf("default export options = %#v", body.Export)
	}
	for _, rel := range body.Export.Files {
		if strings.HasPrefix(rel, "project/runs/") || strings.HasPrefix(rel, "project/batches/") || strings.HasPrefix(rel, "project/validation/runs/") || strings.HasPrefix(rel, "project/calibration/results/") || strings.HasPrefix(rel, "project/optimization/results/") || strings.HasPrefix(rel, "project/exports/") {
			t.Fatalf("export should not include generated project artifact %s", rel)
		}
	}
	if body.Export.IncludeRecords {
		t.Fatal("default API export should not include generated records")
	}
	for _, command := range []string{"check-env.ps1", "run-default.ps1", "run-scenario.ps1", "run-batch.ps1", "validate-data.ps1", "calibrate.ps1", "optimize.ps1", "serve.ps1"} {
		if !containsString(body.Export.Commands, command) {
			t.Fatalf("export commands missing %s in %v", command, body.Export.Commands)
		}
	}
	runDefaultBytes, err := os.ReadFile(filepath.Join(exportRoot, "run-default.ps1"))
	if err != nil {
		t.Fatalf("run default script: %v", err)
	}
	if !bytes.Contains(runDefaultBytes, []byte("Write-RunLogBundle")) || !bytes.Contains(runDefaultBytes, []byte("LogBundle")) || !bytes.Contains(runDefaultBytes, []byte("outputs\\logs")) {
		t.Fatalf("run default script missing runtime log bundle support:\n%s", string(runDefaultBytes))
	}
	runBatchBytes, err := os.ReadFile(filepath.Join(exportRoot, "run-batch.ps1"))
	if err != nil {
		t.Fatalf("run batch script: %v", err)
	}
	if !bytes.Contains(runBatchBytes, []byte("Write-RunLogBundle -ResultPath $Output")) {
		t.Fatalf("run batch script missing per-case log bundles:\n%s", string(runBatchBytes))
	}
	optimizeBytes, err := os.ReadFile(filepath.Join(exportRoot, "optimize.ps1"))
	if err != nil {
		t.Fatalf("optimize script: %v", err)
	}
	if !bytes.Contains(optimizeBytes, []byte("SaveParameterSet")) || !bytes.Contains(optimizeBytes, []byte("--save-parameter-set")) {
		t.Fatalf("optimize script missing parameter set save option:\n%s", string(optimizeBytes))
	}
	optimizeSDKBytes, err := os.ReadFile(filepath.Join(exportRoot, "optimize-sdk.py"))
	if err != nil {
		t.Fatalf("optimization sdk script: %v", err)
	}
	if !bytes.Contains(optimizeSDKBytes, []byte("RunnerClient")) || !bytes.Contains(optimizeSDKBytes, []byte("run_optimization")) || !bytes.Contains(optimizeSDKBytes, []byte("scalar_grid.json")) {
		t.Fatalf("optimization sdk script missing SDK optimization workflow:\n%s", string(optimizeSDKBytes))
	}
	sdkExampleBytes, err := os.ReadFile(filepath.Join(exportRoot, "sdk-example.py"))
	if err != nil {
		t.Fatalf("sdk example: %v", err)
	}
	if !bytes.Contains(sdkExampleBytes, []byte("RunnerClient")) || !bytes.Contains(sdkExampleBytes, []byte("python\" / \"bcs_sdk")) {
		t.Fatalf("sdk example does not use exported SDK:\n%s", string(sdkExampleBytes))
	}
	guideBytes, err := os.ReadFile(filepath.Join(exportRoot, "docs", "CLI_Guide.md"))
	if err != nil {
		t.Fatalf("cli guide: %v", err)
	}
	for _, text := range []string{"Runtime CLI Guide", "Public Inputs", "Validation Mappings", "Calibration Setups", "Optimization Setups", "optimize-sdk.py", "outputs\\logs"} {
		if !bytes.Contains(guideBytes, []byte(text)) {
			t.Fatalf("cli guide missing %q:\n%s", text, string(guideBytes))
		}
	}
	if _, err := os.Stat(filepath.Join(exportRoot, "manifest.json")); err != nil {
		t.Fatalf("manifest: %v", err)
	}
	var exportedSchema schemaexport.InterfaceSchema
	schemaBytes, err := os.ReadFile(filepath.Join(exportRoot, "schema", "public-io.json"))
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	if err := json.Unmarshal(schemaBytes, &exportedSchema); err != nil {
		t.Fatalf("decode schema: %v", err)
	}
	if len(exportedSchema.Inputs) != 1 || len(exportedSchema.Outputs) != 1 {
		t.Fatalf("schema inputs/outputs = %d/%d", len(exportedSchema.Inputs), len(exportedSchema.Outputs))
	}
	exportedProjectPath := filepath.Join(exportRoot, "project", "project.bcsproj")
	exportedLoaded, err := project.Load(exportedProjectPath)
	if err != nil {
		t.Fatalf("load exported project: %v", err)
	}
	if _, err := compiler.Compile(exportedLoaded); err != nil {
		t.Fatalf("compile exported project: %v", err)
	}
	relocatedExportRoot := filepath.Join(root, "relocated", "runtime_package")
	if err := copyProjectTree(exportRoot, relocatedExportRoot); err != nil {
		t.Fatalf("relocate export: %v", err)
	}
	assertRuntimeExportCompiles(t, relocatedExportRoot)
	archivePath := filepath.Join(root, "runtime-export.zip")
	if err := zipDirectory(exportRoot, archivePath); err != nil {
		t.Fatalf("zip export: %v", err)
	}
	unzippedExportRoot := filepath.Join(root, "unzipped", "runtime_package")
	if err := unzipArchive(archivePath, unzippedExportRoot); err != nil {
		t.Fatalf("unzip export: %v", err)
	}
	assertRuntimeExportCompiles(t, unzippedExportRoot)

	openResponse := httptest.NewRecorder()
	openRequest := httptest.NewRequest(http.MethodGet, "/api/project/export?project_path="+url.QueryEscape(createBody.Project.ProjectPath)+"&profile=runtime_package", nil)
	server.Handler().ServeHTTP(openResponse, openRequest)
	if openResponse.Code != http.StatusOK {
		t.Fatalf("open export status = %d body=%s", openResponse.Code, openResponse.Body.String())
	}
	var openBody struct {
		Summary ExportSummary  `json:"summary"`
		Export  ExportManifest `json:"export"`
	}
	if err := json.Unmarshal(openResponse.Body.Bytes(), &openBody); err != nil {
		t.Fatal(err)
	}
	if openBody.Summary.RelativePath != body.Summary.RelativePath {
		t.Fatalf("opened export relative path = %s, want %s", openBody.Summary.RelativePath, body.Summary.RelativePath)
	}
	if len(openBody.Export.Files) != len(body.Export.Files) {
		t.Fatalf("opened export file count = %d, want %d", len(openBody.Export.Files), len(body.Export.Files))
	}
	assertRuntimeExportWorkflowsRun(t, exportedLoaded)

	slimPayload, err := json.Marshal(map[string]any{
		"project_path":                createBody.Project.ProjectPath,
		"profile":                     "runtime_package",
		"include_datasets":            false,
		"include_calibration_setups":  false,
		"include_optimization_setups": false,
		"include_ml_assets":           false,
		"include_sdk_examples":        false,
	})
	if err != nil {
		t.Fatal(err)
	}
	slimResponse := httptest.NewRecorder()
	slimRequest := httptest.NewRequest(http.MethodPost, "/api/export", bytes.NewReader(slimPayload))
	server.Handler().ServeHTTP(slimResponse, slimRequest)
	if slimResponse.Code != http.StatusOK {
		t.Fatalf("slim export status = %d body=%s", slimResponse.Code, slimResponse.Body.String())
	}
	var slimBody struct {
		Export ExportManifest `json:"export"`
	}
	if err := json.Unmarshal(slimResponse.Body.Bytes(), &slimBody); err != nil {
		t.Fatal(err)
	}
	if slimBody.Export.IncludeDatasets || slimBody.Export.IncludeCalibration || slimBody.Export.IncludeOptimization || slimBody.Export.IncludeMLAssets || slimBody.Export.IncludeSDKExamples {
		t.Fatalf("slim export options = %#v", slimBody.Export)
	}
	for _, rel := range []string{
		"project/datasets/scalar_validation.csv",
		"project/validation/mappings/scalar_validation.json",
		"project/calibration/setups/scalar_gain.json",
		"project/optimization/setups/scalar_grid.json",
		"python/bcs_sdk/bcs_sdk/client.py",
		"sdk-example.py",
		"validate-data.ps1",
		"calibrate.ps1",
		"optimize.ps1",
		"optimize-sdk.py",
	} {
		if containsString(slimBody.Export.Files, rel) {
			t.Fatalf("slim export should not include %s in %v", rel, slimBody.Export.Files)
		}
	}

	recordPayload, err := json.Marshal(map[string]any{
		"project_path":    createBody.Project.ProjectPath,
		"profile":         "runtime_package",
		"include_records": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	recordResponse := httptest.NewRecorder()
	recordRequest := httptest.NewRequest(http.MethodPost, "/api/export", bytes.NewReader(recordPayload))
	server.Handler().ServeHTTP(recordResponse, recordRequest)
	if recordResponse.Code != http.StatusOK {
		t.Fatalf("record export status = %d body=%s", recordResponse.Code, recordResponse.Body.String())
	}
	var recordBody struct {
		Export ExportManifest `json:"export"`
	}
	if err := json.Unmarshal(recordResponse.Body.Bytes(), &recordBody); err != nil {
		t.Fatal(err)
	}
	expectedRecords := []string{
		"project/runs/run-test.json",
		"project/batches/batch-test.json",
		"project/validation/runs/validation-test.json",
		"project/calibration/results/calibration-test.json",
		"project/optimization/results/optimization-test.json",
	}
	for _, rel := range expectedRecords {
		if !containsString(recordBody.Export.Files, rel) {
			t.Fatalf("record export files missing %s in %v", rel, recordBody.Export.Files)
		}
	}
	if !recordBody.Export.IncludeRecords || !containsString(recordBody.Export.RunRecords, "project/runs/run-test.json") {
		t.Fatalf("record manifest = %#v", recordBody.Export)
	}
}

func TestExportEndpointIncludesGeneratedWrapperSources(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	seedTestRuntimeSupport(t, root)
	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	projectRoot := filepath.Join(root, "projects", "generated-wrapper-export")
	if err := copyProjectTree(filepath.Join(repoRoot, "examples", "008_generated_wrapper_component"), projectRoot); err != nil {
		t.Fatal(err)
	}

	payload, err := json.Marshal(map[string]any{
		"project_path": filepath.Join(projectRoot, "project.bcsproj"),
		"profile":      "runtime_package",
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/export", bytes.NewReader(payload))
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Export ExportManifest `json:"export"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	expectedFiles := []string{
		"project/components/custom_gain/component.json",
		"project/components/custom_gain/helpers.py",
		"project/components/custom_gain/user_init.py",
		"project/components/custom_gain/user_step.py",
		"project/components/custom_gain/wrapper.py",
	}
	exportRoot := filepath.Join(projectRoot, "exports", "runtime_package")
	for _, rel := range expectedFiles {
		if !containsString(body.Export.Files, rel) {
			t.Fatalf("export files missing %s in %v", rel, body.Export.Files)
		}
		if _, err := os.Stat(filepath.Join(exportRoot, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("export file %s: %v", rel, err)
		}
	}
}

func TestExportEndpointIncludesMLAssetsAndChecksums(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	seedTestRuntimeSupport(t, root)
	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	projectRoot := filepath.Join(root, "projects", "ahu-state-ann")
	if err := copyProjectTree(filepath.Join(repoRoot, "examples", "014_ahu_state_ann"), projectRoot); err != nil {
		t.Fatal(err)
	}

	payload, err := json.Marshal(map[string]any{
		"project_path": filepath.Join(projectRoot, "project.bcsproj"),
		"profile":      "runtime_package",
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/export", bytes.NewReader(payload))
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Export ExportManifest `json:"export"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	expectedAssets := []string{
		"project/assets/ahu_state_ann/feature_schema.json",
		"project/assets/ahu_state_ann/model.json",
		"project/assets/ahu_state_ann/target_schema.json",
		"project/assets/ahu_state_ann/validation_report.json",
	}
	exportRoot := filepath.Join(projectRoot, "exports", "runtime_package")
	for _, rel := range expectedAssets {
		if !containsString(body.Export.Files, rel) {
			t.Fatalf("export files missing %s in %v", rel, body.Export.Files)
		}
		if !containsString(body.Export.ModelAssets, rel) {
			t.Fatalf("export model assets missing %s in %v", rel, body.Export.ModelAssets)
		}
		if checksum := body.Export.Checksums[rel]; len(checksum) != 64 {
			t.Fatalf("checksum for %s = %q", rel, checksum)
		}
		if _, err := os.Stat(filepath.Join(exportRoot, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("export file %s: %v", rel, err)
		}
	}
	if len(body.Export.MLValidationReports) != 1 {
		t.Fatalf("ML validation reports = %#v", body.Export.MLValidationReports)
	}
	mlReport := body.Export.MLValidationReports[0]
	if mlReport.ComponentID != "ahu_state_ann" ||
		mlReport.ReportPath != "project/assets/ahu_state_ann/validation_report.json" ||
		mlReport.Dataset != "synthetic_ahu_state_reference" ||
		len(mlReport.ModelAssetChecksum) != 64 ||
		mlReport.Metrics["supply_air_temperature_c"]["rmse"] == nil {
		t.Fatalf("ML validation report = %#v", mlReport)
	}
	var exportedSchema schemaexport.InterfaceSchema
	schemaBytes, err := os.ReadFile(filepath.Join(exportRoot, "schema", "public-io.json"))
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	if err := json.Unmarshal(schemaBytes, &exportedSchema); err != nil {
		t.Fatalf("decode schema: %v", err)
	}
	if len(exportedSchema.ModelAssets) != len(expectedAssets) {
		t.Fatalf("schema model assets = %#v", exportedSchema.ModelAssets)
	}
	exportedProject, err := project.Load(filepath.Join(exportRoot, "project", "project.bcsproj"))
	if err != nil {
		t.Fatalf("load exported project: %v", err)
	}
	if _, err := compiler.Compile(exportedProject); err != nil {
		t.Fatalf("compile exported project: %v", err)
	}
	exportedProject.Project.Environment.Python = testPythonExecutable(t)
	exportedInput, err := runtimecore.LoadInput(filepath.Join(exportedProject.Root, filepath.FromSlash(exportedProject.Project.DefaultInput)))
	if err != nil {
		t.Fatalf("load exported ANN input: %v", err)
	}
	exportedResult, err := runtimecore.Run(context.Background(), exportedProject, exportedInput)
	if err != nil {
		t.Fatalf("run exported ANN project: %v", err)
	}
	if !exportedResult.OK {
		t.Fatalf("exported ANN result was not ok: %#v", exportedResult)
	}
	if got := exportedResult.Outputs["supply_air_temperature_c"]; got != 19.46 {
		t.Fatalf("exported ANN supply_air_temperature_c = %#v, want 19.46", got)
	}
	if len(exportedResult.ExecutionOrder) < 2 || exportedResult.ExecutionOrder[1] != "ahu_state_ann" {
		t.Fatalf("exported ANN execution order = %#v", exportedResult.ExecutionOrder)
	}

	slimPayload, err := json.Marshal(map[string]any{
		"project_path":      filepath.Join(projectRoot, "project.bcsproj"),
		"profile":           "runtime_package",
		"include_ml_assets": false,
	})
	if err != nil {
		t.Fatal(err)
	}
	slimResponse := httptest.NewRecorder()
	slimRequest := httptest.NewRequest(http.MethodPost, "/api/export", bytes.NewReader(slimPayload))
	server.Handler().ServeHTTP(slimResponse, slimRequest)
	if slimResponse.Code != http.StatusOK {
		t.Fatalf("slim status = %d body=%s", slimResponse.Code, slimResponse.Body.String())
	}
	var slimBody struct {
		Export ExportManifest `json:"export"`
	}
	if err := json.Unmarshal(slimResponse.Body.Bytes(), &slimBody); err != nil {
		t.Fatal(err)
	}
	if slimBody.Export.IncludeMLAssets || len(slimBody.Export.ModelAssets) != 0 {
		t.Fatalf("slim ML asset manifest = %#v", slimBody.Export)
	}
	for _, rel := range expectedAssets {
		if containsString(slimBody.Export.Files, rel) {
			t.Fatalf("slim ML export should not include %s in %v", rel, slimBody.Export.Files)
		}
	}
}

func TestExportEndpointRejectsSavedSourceContractErrors(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	project := createWorkspaceProject(t, server, "Export Source Gate Project")
	writeBrokenScalarSource(t, project)

	payload, err := json.Marshal(map[string]any{
		"project_path": project.ProjectPath,
		"profile":      "runtime_package",
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/export", bytes.NewReader(payload))
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body apiError
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !hasProblemMessage(body.Problems, "evaluate method is missing") {
		t.Fatalf("source problem missing from %#v", body.Problems)
	}
}

func TestExportEndpointRejectsExamples(t *testing.T) {
	server := newTestServer(t)
	payload := []byte(`{
		"project_path": "examples/001_scalar_component/project.bcsproj",
		"profile": "runtime_package"
	}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/export", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}
