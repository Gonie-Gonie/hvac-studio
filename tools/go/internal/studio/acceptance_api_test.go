package studio

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goniegonie/hvac-studio/tools/go/internal/compiler"
	"github.com/goniegonie/hvac-studio/tools/go/internal/model"
	"github.com/goniegonie/hvac-studio/tools/go/internal/project"
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
