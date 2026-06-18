package studio

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goniegonie/hvac-studio/tools/go/internal/project"
)

func TestRunEndpointMapsPythonTracebackToSourceLine(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Traceback Line Project"}`)))
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
	sourcePath := filepath.Join(root, "projects", "traceback-line-project", "components", "scalar.py")
	source := "class ScalarComponent:\n    def evaluate(self, inputs, state, params, context):\n        scale = 1 / 0\n        return {\"result\": scale}, state\n"
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
	if len(body.Problems) != 1 {
		t.Fatalf("problems = %#v", body.Problems)
	}
	problem := body.Problems[0]
	if problem.ComponentID != "scalar" || problem.Source != "components/scalar.py" || problem.Line != 3 {
		t.Fatalf("problem location = %#v", problem)
	}
	if len(body.Error.Problems) != 1 || body.Error.Problems[0].Source != "components/scalar.py" || body.Error.Problems[0].Line != 3 {
		t.Fatalf("structured error problems = %#v", body.Error.Problems)
	}
	if !strings.Contains(problem.Message, "ZeroDivisionError") {
		t.Fatalf("problem message = %s", problem.Message)
	}
}

func TestRunEndpointMapsGeneratedWrapperTracebackToUserStep(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	projectSummary := createWorkspaceProject(t, server, "Traceback Wrapper Project")

	componentPayload, err := json.Marshal(map[string]any{
		"project_path":      projectSummary.ProjectPath,
		"name":              "Trace Gain",
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

	source := strings.Join([]string{
		"def step(inputs, state, params, context):",
		"    value = float(inputs[\"value\"])",
		"    scale = 1 / 0",
		"    return {\"result\": value * scale}, state",
		"",
	}, "\n")
	sourcePayload, err := json.Marshal(map[string]any{
		"project_path": projectSummary.ProjectPath,
		"component_id": "trace_gain",
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

	runPayload, err := json.Marshal(map[string]any{
		"project_path": projectSummary.ProjectPath,
		"inputs": map[string]any{
			"value":            1,
			"trace_gain_value": 2,
		},
		"context": map[string]any{"time": 0.0, "dt": 60.0},
	})
	if err != nil {
		t.Fatal(err)
	}
	runResponse := httptest.NewRecorder()
	runRequest := httptest.NewRequest(http.MethodPost, "/api/run", bytes.NewReader(runPayload))
	server.Handler().ServeHTTP(runResponse, runRequest)
	if runResponse.Code != http.StatusBadGateway {
		t.Fatalf("run status = %d body=%s", runResponse.Code, runResponse.Body.String())
	}
	var body apiError
	if err := json.Unmarshal(runResponse.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	problem, ok := findProblemMessageContaining(body.Problems, "ZeroDivisionError")
	if !ok {
		t.Fatalf("traceback problem missing from %#v", body.Problems)
	}
	if problem.ComponentID != "trace_gain" || problem.Source != "components/trace_gain/user_step.py" || problem.Line != 3 {
		t.Fatalf("problem location = %#v", problem)
	}
}

func TestTracebackFramesParseWindowsPythonFileLines(t *testing.T) {
	message := "Traceback (most recent call last):\n" +
		"  File \"C:\\Temp\\project\\components\\scalar.py\", line 3, in evaluate\n" +
		"    scale = 1 / 0\n" +
		"ZeroDivisionError: division by zero\n"

	frames := tracebackFrames(message)

	if len(frames) != 1 {
		t.Fatalf("frames = %#v", frames)
	}
	if frames[0].Path != `C:\Temp\project\components\scalar.py` || frames[0].Line != 3 {
		t.Fatalf("frame = %#v", frames[0])
	}
}

func TestTracebackSourceLocationFallsBackToProjectFrame(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	projectSummary := createWorkspaceProject(t, server, "Traceback Helper Project")
	loaded, err := project.Load(projectSummary.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join(root, "projects", "traceback-helper-project", "components", "scalar.py")
	message := "Traceback (most recent call last):\n" +
		"  File \"C:\\Users\\GonieGonie\\Documents\\GitHub\\hvac-studio\\python\\bcs_worker\\bcs_worker\\worker.py\", line 82, in evaluate_component\n" +
		"    result = instance.evaluate(inputs or {}, state or {}, params or {}, context or {})\n" +
		"  File \"" + sourcePath + "\", line 3, in evaluate\n" +
		"    scale = 1 / 0\n" +
		"ZeroDivisionError: division by zero\n"

	location, ok := tracebackSourceLocation(loaded, message, "scalar")

	if !ok {
		t.Fatalf("location not found for frames %#v root %s", tracebackFrames(message), loaded.Root)
	}
	if location.ComponentID != "scalar" || location.Source != "components/scalar.py" || location.Line != 3 {
		t.Fatalf("location = %#v", location)
	}
}
