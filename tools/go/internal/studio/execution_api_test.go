package studio

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goniegonie/hvac-studio/tools/go/internal/project"
	runtimecore "github.com/goniegonie/hvac-studio/tools/go/internal/runtime"
)

func TestUpdateLayoutEndpointWritesWorkspaceLayout(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	project := createWorkspaceProject(t, server, "Layout Project")
	payload, err := json.Marshal(map[string]any{
		"project_path": project.ProjectPath,
		"components": map[string]CanvasPosition{
			"scalar":  {X: 132, Y: 96},
			"missing": {X: 10, Y: 20},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/layout", bytes.NewReader(payload))

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
	if got := body.Project.Layout.Components["scalar"]; got.X != 132 || got.Y != 96 {
		t.Fatalf("layout position = %#v, want 132,96", got)
	}
	if _, exists := body.Project.Layout.Components["missing"]; exists {
		t.Fatal("layout should ignore unknown components")
	}
	if _, err := os.Stat(filepath.Join(root, "projects", "layout-project", "studio", "layout.json")); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateInputEndpointWritesWorkspaceDefaultInput(t *testing.T) {
	_, server := newIsolatedTestServer(t)

	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Input Project"}`)))
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

	payload := []byte(`{
		"project_path": "` + filepath.ToSlash(createBody.Project.ProjectPath) + `",
		"inputs": {"value": 7},
		"context": {"time": 0, "dt": 30}
	}`)
	updateResponse := httptest.NewRecorder()
	updateRequest := httptest.NewRequest(http.MethodPost, "/api/project/input", bytes.NewReader(payload))
	server.Handler().ServeHTTP(updateResponse, updateRequest)
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("update status = %d body=%s", updateResponse.Code, updateResponse.Body.String())
	}

	loaded, err := project.Load(createBody.Project.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	input, err := runtimecore.LoadInput(filepath.Join(loaded.Root, loaded.Project.DefaultInput))
	if err != nil {
		t.Fatal(err)
	}
	if got := input.Inputs["value"]; got != 7.0 {
		t.Fatalf("input value = %v, want 7", got)
	}
}

func TestUpdateInputEndpointRejectsExamples(t *testing.T) {
	server := newTestServer(t)
	payload := []byte(`{
		"project_path": "examples/001_scalar_component/project.bcsproj",
		"inputs": {"value": 7}
	}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/input", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}

func TestRunEndpointRunsFeedForwardExample(t *testing.T) {
	server := newTestServer(t)
	payload := []byte(`{
		"project_path": "examples/003_feedforward_system/project.bcsproj",
		"inputs": {
			"building_load_kw": 500,
			"base_chw_setpoint_c": 7
		},
		"context": {
			"time": 0,
			"dt": 60
		}
	}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/run", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Result struct {
			Outputs map[string]float64 `json:"outputs"`
		} `json:"result"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Result.Outputs["total_power_kw"] != 122 {
		t.Fatalf("total_power_kw = %v", body.Result.Outputs["total_power_kw"])
	}
}

func TestRunEndpointCapturesComponentLogs(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	project := createWorkspaceProject(t, server, "Noisy Run Project")
	sourcePath := filepath.Join(filepath.Dir(project.ProjectPath), "components", "scalar.py")
	source := strings.TrimLeft(`
import sys

class ScalarComponent:
    def initialize(self, params, context):
        return {}

    def evaluate(self, inputs, state, params, context):
        print("stdout from scalar")
        print("stderr from scalar", file=sys.stderr)
        value = float(inputs["value"])
        gain = float(params.get("gain", 2.0))
        return {"result": value * gain}, state
`, "\n")
	if err := os.WriteFile(sourcePath, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}

	payload, err := json.Marshal(map[string]any{
		"project_path": project.ProjectPath,
		"inputs":       map[string]any{"value": 4},
		"context":      map[string]any{"time": 123, "dt": 60},
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/run", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Result runtimecore.RunResult `json:"result"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if got := body.Result.Outputs["result"]; got != 8.0 {
		t.Fatalf("result = %v, want 8", got)
	}
	if !hasComponentLog(body.Result.ComponentLogs, "scalar", "evaluate", "info", "stdout from scalar") {
		t.Fatalf("stdout log missing from %#v", body.Result.ComponentLogs)
	}
	if !hasComponentLog(body.Result.ComponentLogs, "scalar", "evaluate", "error", "stderr from scalar") {
		t.Fatalf("stderr log missing from %#v", body.Result.ComponentLogs)
	}
	stdoutLog, ok := findComponentLog(body.Result.ComponentLogs, "scalar", "evaluate", "info", "stdout from scalar")
	if !ok {
		t.Fatalf("stdout log missing from %#v", body.Result.ComponentLogs)
	}
	if stdoutLog.Time != float64(123) {
		t.Fatalf("stdout log time = %#v, want 123", stdoutLog.Time)
	}
	if !strings.Contains(stdoutLog.Source, "ScalarComponent") {
		t.Fatalf("stdout log source = %q", stdoutLog.Source)
	}
}

func TestRunEndpointCapturesExternalExecutableLogs(t *testing.T) {
	server := newTestServer(t)
	payload, err := json.Marshal(map[string]any{
		"project_path": "examples/010_external_executable_component/project.bcsproj",
		"inputs":       map[string]any{"request": 4},
		"context":      map[string]any{"time": 0, "dt": 60},
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/run", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Result runtimecore.RunResult `json:"result"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !hasComponentLog(body.Result.ComponentLogs, "external_gain", "external_executable", "error", "external gain stderr call 1") {
		t.Fatalf("external stderr log missing from %#v", body.Result.ComponentLogs)
	}
	infoLog, ok := findComponentLog(body.Result.ComponentLogs, "external_gain", "external_executable", "info", "external gain evaluated call 1")
	if !ok {
		t.Fatalf("external info log missing from %#v", body.Result.ComponentLogs)
	}
	if infoLog.Time != float64(0) {
		t.Fatalf("external log time = %#v, want 0", infoLog.Time)
	}
	if infoLog.Source != "components/external_gain/external_gain.py" {
		t.Fatalf("external log source = %q", infoLog.Source)
	}
}

func TestRunEndpointHonorsTimeout(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	project := createWorkspaceProject(t, server, "Slow Run Project")
	sourcePath := filepath.Join(filepath.Dir(project.ProjectPath), "components", "scalar.py")
	source := strings.TrimLeft(`
import time

class ScalarComponent:
    def initialize(self, params, context):
        return {}

    def evaluate(self, inputs, state, params, context):
        time.sleep(2)
        return {"result": float(inputs["value"])}, state
`, "\n")
	if err := os.WriteFile(sourcePath, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}

	payload, err := json.Marshal(map[string]any{
		"project_path": project.ProjectPath,
		"inputs":       map[string]any{"value": 4},
		"timeout_ms":   200,
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/run", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body apiError
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Kind != "runtime" || !strings.Contains(body.Message, "run timed out after 200ms") {
		t.Fatalf("timeout body = %#v", body)
	}
}

func TestRunEndpointAppliesParameterSet(t *testing.T) {
	server := newTestServer(t)
	payload := []byte(`{
		"project_path": "examples/005_chiller_plant_like_system/project.bcsproj",
		"parameter_set_path": "parameter_sets/high_efficiency.json",
		"inputs": {
			"building_load_kw": 600,
			"base_chw_setpoint_c": 7,
			"condenser_entering_temp_c": 32
		},
		"context": {
			"time": 0,
			"dt": 60
		}
	}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/run", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Result struct {
			ParameterSet string             `json:"parameter_set"`
			Outputs      map[string]float64 `json:"outputs"`
		} `json:"result"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Result.ParameterSet != "parameter_sets/high_efficiency.json" {
		t.Fatalf("parameter_set = %q", body.Result.ParameterSet)
	}
	if body.Result.Outputs["total_power_kw"] == 140.96 {
		t.Fatalf("parameter set did not change total_power_kw")
	}
}

func TestRunEndpointReturnsComponentLinkedRuntimeProblem(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Run Problem Project"}`)))
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
	sourcePath := filepath.Join(root, "projects", "run-problem-project", "components", "scalar.py")
	source := "class ScalarComponent:\n    def evaluate(self, inputs, state, params, context):\n        return {\"result\": 1, \"debug\": 2}, state\n"
	if err := os.WriteFile(sourcePath, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}

	payload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"inputs":       map[string]any{"value": 5},
		"context":      map[string]any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/run", bytes.NewReader(payload))
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusBadGateway {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body apiError
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error.Schema != "hvac-studio.error.v1" || body.Error.Kind != "python_worker" {
		t.Fatalf("error payload = %#v", body.Error)
	}
	if len(body.Problems) != 1 {
		t.Fatalf("problems = %#v", body.Problems)
	}
	if body.Problems[0].ComponentID != "scalar" {
		t.Fatalf("component id = %s, want scalar", body.Problems[0].ComponentID)
	}
	if !strings.Contains(body.Problems[0].Message, "returned undeclared output node: debug") {
		t.Fatalf("problem = %#v", body.Problems[0])
	}
}

func TestRunEndpointRejectsSavedSourceContractErrors(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	project := createWorkspaceProject(t, server, "Run Source Gate Project")
	writeBrokenScalarSource(t, project)

	payload, err := json.Marshal(map[string]any{
		"project_path": project.ProjectPath,
		"inputs":       map[string]any{"value": 4},
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/run", bytes.NewReader(payload))
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

func TestValidateEndpointReturnsLinkedProblem(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Invalid Project"}`)))
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
	loaded, err := project.Load(createBody.Project.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	loaded.Graph.Systems[0].PublicInputs[0].Node = "missing"
	if err := writeJSONFile(loaded.GraphPath, loaded.Graph); err != nil {
		t.Fatal(err)
	}

	payload, err := json.Marshal(map[string]any{"project_path": createBody.Project.ProjectPath})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/validate", bytes.NewReader(payload))
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body apiError
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Problems) != 1 {
		t.Fatalf("problem count = %d", len(body.Problems))
	}
	if body.Problems[0].ComponentID != "scalar" {
		t.Fatalf("component id = %s, want scalar", body.Problems[0].ComponentID)
	}
}

func TestValidateEndpointIncludesSourceChecks(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Validate Source Project"}`)))
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

	payload, err := json.Marshal(map[string]any{"project_path": createBody.Project.ProjectPath})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/validate", bytes.NewReader(payload))
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Validation struct {
			SourceChecks int       `json:"source_checks"`
			Problems     []Problem `json:"problems"`
		} `json:"validation"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Validation.SourceChecks != 1 {
		t.Fatalf("source checks = %d, want 1", body.Validation.SourceChecks)
	}
	if hasErrorProblems(body.Validation.Problems) {
		t.Fatalf("unexpected source validation errors = %#v", body.Validation.Problems)
	}
}

func TestValidateEndpointReportsSourceContractProblems(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Validate Broken Source Project"}`)))
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
	sourcePath := filepath.Join(root, "projects", "validate-broken-source-project", "components", "scalar.py")
	if err := os.WriteFile(sourcePath, []byte("class WrongName:\n    pass\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	payload, err := json.Marshal(map[string]any{"project_path": createBody.Project.ProjectPath})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/validate", bytes.NewReader(payload))
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body apiError
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Problems) < 2 {
		t.Fatalf("problems = %#v", body.Problems)
	}
	if body.Problems[0].ComponentID != "scalar" {
		t.Fatalf("component id = %s, want scalar", body.Problems[0].ComponentID)
	}
	if !strings.Contains(body.Message, "project source validation failed") {
		t.Fatalf("message = %s", body.Message)
	}
}

func TestCreateScenarioEndpointWritesWorkspaceScenario(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Scenario Project"}`)))
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

	payload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"name":         "Design Day",
		"inputs":       map[string]any{"value": 9},
		"context":      map[string]any{"time": 0, "dt": 60},
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/scenarios", bytes.NewReader(payload))
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Summary ScenarioSummary `json:"summary"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Summary.RelativePath != "scenarios/design-day.json" {
		t.Fatalf("relative path = %s", body.Summary.RelativePath)
	}
	if _, err := os.Stat(filepath.Join(root, "projects", "scenario-project", "scenarios", "design-day.json")); err != nil {
		t.Fatal(err)
	}
}

func TestScenarioEndpointReturnsSavedScenario(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Scenario Read Project"}`)))
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
	payload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"name":         "Design Day",
		"inputs":       map[string]any{"value": 9},
		"context":      map[string]any{"time": 0, "dt": 60},
	})
	if err != nil {
		t.Fatal(err)
	}
	createScenarioResponse := httptest.NewRecorder()
	createScenarioRequest := httptest.NewRequest(http.MethodPost, "/api/project/scenarios", bytes.NewReader(payload))
	server.Handler().ServeHTTP(createScenarioResponse, createScenarioRequest)
	if createScenarioResponse.Code != http.StatusCreated {
		t.Fatalf("scenario status = %d body=%s", createScenarioResponse.Code, createScenarioResponse.Body.String())
	}

	response := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/project/scenario?project_path="+url.QueryEscape(createBody.Project.ProjectPath)+"&scenario_id=design-day",
		nil,
	)
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Scenario ScenarioRecord `json:"scenario"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Scenario.Inputs["value"] != 9.0 {
		t.Fatalf("scenario input = %v, want 9", body.Scenario.Inputs["value"])
	}
}

func TestBatchEndpointRunsSavedScenarios(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Batch Project"}`)))
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
	parameterSetPath := filepath.Join(root, "projects", "batch-project", "parameter_sets", "triple_gain.json")
	if err := os.MkdirAll(filepath.Dir(parameterSetPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(parameterSetPath, []byte(`{
  "id": "triple_gain",
  "name": "Triple Gain",
  "components": {
    "scalar": {
      "gain": 3
    }
  }
}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, scenario := range []struct {
		name  string
		value float64
	}{
		{name: "Low", value: 2},
		{name: "High", value: 3},
	} {
		payload, err := json.Marshal(map[string]any{
			"project_path": createBody.Project.ProjectPath,
			"name":         scenario.name,
			"inputs":       map[string]any{"value": scenario.value},
			"context":      map[string]any{"time": 0, "dt": 60},
		})
		if err != nil {
			t.Fatal(err)
		}
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/project/scenarios", bytes.NewReader(payload))
		server.Handler().ServeHTTP(response, request)
		if response.Code != http.StatusCreated {
			t.Fatalf("scenario status = %d body=%s", response.Code, response.Body.String())
		}
	}

	batchPayload, err := json.Marshal(map[string]any{
		"project_path":       createBody.Project.ProjectPath,
		"parameter_set_path": filepath.Join("parameter_sets", "triple_gain.json"),
	})
	if err != nil {
		t.Fatal(err)
	}
	batchResponse := httptest.NewRecorder()
	batchRequest := httptest.NewRequest(http.MethodPost, "/api/batch", bytes.NewReader(batchPayload))
	server.Handler().ServeHTTP(batchResponse, batchRequest)
	if batchResponse.Code != http.StatusOK {
		t.Fatalf("batch status = %d body=%s", batchResponse.Code, batchResponse.Body.String())
	}
	var batchBody struct {
		Summary BatchSummary `json:"summary"`
		Batch   BatchRecord  `json:"batch"`
	}
	if err := json.Unmarshal(batchResponse.Body.Bytes(), &batchBody); err != nil {
		t.Fatal(err)
	}
	if batchBody.Summary.CaseCount != 2 || batchBody.Summary.OKCount != 2 {
		t.Fatalf("batch counts = %d/%d, want 2/2", batchBody.Summary.OKCount, batchBody.Summary.CaseCount)
	}
	if batchBody.Summary.ParameterSet != "parameter_sets/triple_gain.json" || batchBody.Batch.ParameterSet != "parameter_sets/triple_gain.json" {
		t.Fatalf("batch parameter set = summary:%q record:%q", batchBody.Summary.ParameterSet, batchBody.Batch.ParameterSet)
	}
	if len(batchBody.Batch.Cases) != 2 {
		t.Fatalf("case count = %d, want 2", len(batchBody.Batch.Cases))
	}
	if got := batchBody.Batch.Cases[0].Result.Outputs["result"]; got != 6.0 {
		t.Fatalf("first output = %v, want 6", got)
	}
	if got := batchBody.Batch.Cases[1].Result.Outputs["result"]; got != 9.0 {
		t.Fatalf("second output = %v, want 9", got)
	}
	if batchBody.Batch.Cases[0].Result.ParameterSet != "parameter_sets/triple_gain.json" {
		t.Fatalf("case parameter set = %q", batchBody.Batch.Cases[0].Result.ParameterSet)
	}
	if _, err := os.Stat(filepath.Join(root, "projects", "batch-project", batchBody.Summary.RelativePath)); err != nil {
		t.Fatal(err)
	}

	recordResponse := httptest.NewRecorder()
	recordRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/project/batch?project_path="+url.QueryEscape(createBody.Project.ProjectPath)+"&batch_id="+url.QueryEscape(batchBody.Summary.ID),
		nil,
	)
	server.Handler().ServeHTTP(recordResponse, recordRequest)
	if recordResponse.Code != http.StatusOK {
		t.Fatalf("batch record status = %d body=%s", recordResponse.Code, recordResponse.Body.String())
	}
	var recordBody struct {
		BatchRecord BatchRecord `json:"batch_record"`
	}
	if err := json.Unmarshal(recordResponse.Body.Bytes(), &recordBody); err != nil {
		t.Fatal(err)
	}
	if recordBody.BatchRecord.ParameterSet != "parameter_sets/triple_gain.json" {
		t.Fatalf("opened batch parameter set = %q", recordBody.BatchRecord.ParameterSet)
	}
}

func TestRunSeriesEndpointReturnsPlotReadyResult(t *testing.T) {
	server := newTestServer(t)
	payload, err := json.Marshal(map[string]any{
		"project_path": "examples/004_stateful_controller/project.bcsproj",
		"input_path":   "inputs/series01.json",
		"timeout_ms":   30000,
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/run-series", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Result runtimecore.SeriesResult `json:"result"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.Result.OK || body.Result.StepCount != 3 || len(body.Result.Series) != 3 {
		t.Fatalf("series result = %#v", body.Result)
	}
	if got := body.Result.Outputs["chw_setpoint_c"][1]; got != 6.4 {
		t.Fatalf("second setpoint = %v, want 6.4", got)
	}
	last := body.Result.Series[2]
	if got := last.Outputs["chw_setpoint_c"]; got != 6.55 {
		t.Fatalf("last setpoint = %v, want 6.55", got)
	}
	if got := last.ComponentOutputs["controller"]["control_effort_k"]; got != 0.45 {
		t.Fatalf("last component output = %v, want 0.45", got)
	}
	if got := body.Result.FinalStates["controller"]["calls"]; got != 3.0 {
		t.Fatalf("final calls = %v, want 3", got)
	}
	if len(last.NodeValues) == 0 {
		t.Fatalf("series point should include node traces")
	}
}

func TestBatchEndpointRecordsProblemsForFailedCases(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Batch Failure Project"}`)))
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

	scenarioPayload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"name":         "Broken",
		"inputs":       map[string]any{"value": 2},
		"context":      map[string]any{"time": 0, "dt": 60},
	})
	if err != nil {
		t.Fatal(err)
	}
	scenarioResponse := httptest.NewRecorder()
	scenarioRequest := httptest.NewRequest(http.MethodPost, "/api/project/scenarios", bytes.NewReader(scenarioPayload))
	server.Handler().ServeHTTP(scenarioResponse, scenarioRequest)
	if scenarioResponse.Code != http.StatusCreated {
		t.Fatalf("scenario status = %d body=%s", scenarioResponse.Code, scenarioResponse.Body.String())
	}

	sourcePayload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"component_id": "scalar",
		"content":      "class ScalarComponent:\n    def evaluate(self, inputs, state, params, context):\n        return {\"result\": float(inputs[\"value\"]), \"debug\": 1}, state\n",
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

	batchPayload, err := json.Marshal(map[string]any{"project_path": createBody.Project.ProjectPath})
	if err != nil {
		t.Fatal(err)
	}
	batchResponse := httptest.NewRecorder()
	batchRequest := httptest.NewRequest(http.MethodPost, "/api/batch", bytes.NewReader(batchPayload))
	server.Handler().ServeHTTP(batchResponse, batchRequest)
	if batchResponse.Code != http.StatusOK {
		t.Fatalf("batch status = %d body=%s", batchResponse.Code, batchResponse.Body.String())
	}
	var batchBody struct {
		Summary BatchSummary `json:"summary"`
		Batch   BatchRecord  `json:"batch"`
	}
	if err := json.Unmarshal(batchResponse.Body.Bytes(), &batchBody); err != nil {
		t.Fatal(err)
	}
	if batchBody.Summary.CaseCount != 1 || batchBody.Summary.OKCount != 0 {
		t.Fatalf("batch counts = %d/%d, want 0/1", batchBody.Summary.OKCount, batchBody.Summary.CaseCount)
	}
	if len(batchBody.Batch.Cases) != 1 {
		t.Fatalf("case count = %d, want 1", len(batchBody.Batch.Cases))
	}
	failed := batchBody.Batch.Cases[0]
	if failed.OK {
		t.Fatal("failed case was marked ok")
	}
	if !strings.Contains(failed.Error, "returned undeclared output node: debug") {
		t.Fatalf("case error = %s", failed.Error)
	}
	if len(failed.Problems) != 1 || failed.Problems[0].ComponentID != "scalar" {
		t.Fatalf("case problems = %#v", failed.Problems)
	}

	recordResponse := httptest.NewRecorder()
	recordRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/project/batch?project_path="+url.QueryEscape(createBody.Project.ProjectPath)+"&batch_id="+url.QueryEscape(batchBody.Summary.ID),
		nil,
	)
	server.Handler().ServeHTTP(recordResponse, recordRequest)
	if recordResponse.Code != http.StatusOK {
		t.Fatalf("batch record status = %d body=%s", recordResponse.Code, recordResponse.Body.String())
	}
	var recordBody struct {
		BatchRecord BatchRecord `json:"batch_record"`
	}
	if err := json.Unmarshal(recordResponse.Body.Bytes(), &recordBody); err != nil {
		t.Fatal(err)
	}
	if len(recordBody.BatchRecord.Cases) != 1 || len(recordBody.BatchRecord.Cases[0].Problems) != 1 {
		t.Fatalf("record problems = %#v", recordBody.BatchRecord.Cases)
	}
	if recordBody.BatchRecord.Cases[0].Problems[0].ComponentID != "scalar" {
		t.Fatalf("record problem component = %s", recordBody.BatchRecord.Cases[0].Problems[0].ComponentID)
	}
}

func TestBatchEndpointRejectsSavedSourceContractErrors(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	project := createWorkspaceProject(t, server, "Batch Source Gate Project")
	scenarioPayload, err := json.Marshal(map[string]any{
		"project_path": project.ProjectPath,
		"name":         "Gate",
		"inputs":       map[string]any{"value": 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	scenarioResponse := httptest.NewRecorder()
	scenarioRequest := httptest.NewRequest(http.MethodPost, "/api/project/scenarios", bytes.NewReader(scenarioPayload))
	server.Handler().ServeHTTP(scenarioResponse, scenarioRequest)
	if scenarioResponse.Code != http.StatusCreated {
		t.Fatalf("scenario status = %d body=%s", scenarioResponse.Code, scenarioResponse.Body.String())
	}
	writeBrokenScalarSource(t, project)

	batchPayload, err := json.Marshal(map[string]any{"project_path": project.ProjectPath})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/batch", bytes.NewReader(batchPayload))
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

func TestCreateScenarioEndpointRejectsExamples(t *testing.T) {
	server := newTestServer(t)
	payload := []byte(`{
		"project_path": "examples/001_scalar_component/project.bcsproj",
		"name": "Example Scenario",
		"inputs": {"value": 5}
	}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/scenarios", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}

func TestBatchEndpointRejectsExamples(t *testing.T) {
	server := newTestServer(t)
	payload := []byte(`{"project_path":"examples/001_scalar_component/project.bcsproj"}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/batch", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}
