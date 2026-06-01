package studio

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/goniegonie/hvac-studio/tools/go/internal/model"
	"github.com/goniegonie/hvac-studio/tools/go/internal/project"
	runtimecore "github.com/goniegonie/hvac-studio/tools/go/internal/runtime"
)

func TestProjectsEndpointListsExamples(t *testing.T) {
	server := newTestServer(t)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/projects", nil)

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Projects []ProjectSummary `json:"projects"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Projects) < 2 {
		t.Fatalf("project count = %d", len(body.Projects))
	}
}

func TestStaticIndexServesWorkspace(t *testing.T) {
	server := newTestServer(t)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(body, []byte("HVAC Studio")) {
		t.Fatalf("index did not contain Studio shell")
	}
}

func TestCreateProjectEndpointCreatesWorkspaceProject(t *testing.T) {
	root := t.TempDir()
	server, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte(`{"name":"My First Project"}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Project ProjectSummary `json:"project"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Project.Source != "workspace" {
		t.Fatalf("source = %s", body.Project.Source)
	}
	if _, err := os.Stat(filepath.Join(root, "projects", "my-first-project", "components", "scalar.py")); err != nil {
		t.Fatal(err)
	}
	if _, err := project.Load(body.Project.ProjectPath); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateParametersEndpointWritesWorkspaceGraph(t *testing.T) {
	root := t.TempDir()
	server, err := New(root)
	if err != nil {
		t.Fatal(err)
	}

	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Editable Project"}`)))
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
		"parameters": map[string]any{
			"scalar": map[string]any{"gain": 3.0},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	updateResponse := httptest.NewRecorder()
	updateRequest := httptest.NewRequest(http.MethodPost, "/api/project/parameters", bytes.NewReader(payload))

	server.Handler().ServeHTTP(updateResponse, updateRequest)

	if updateResponse.Code != http.StatusOK {
		t.Fatalf("update status = %d body=%s", updateResponse.Code, updateResponse.Body.String())
	}
	loaded, err := project.Load(createBody.Project.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := loaded.Graph.Components[0].Parameters["gain"]; got != 3.0 {
		t.Fatalf("gain = %v, want 3", got)
	}
}

func TestUpdateParametersEndpointRejectsExamples(t *testing.T) {
	server := newTestServer(t)
	payload := []byte(`{
		"project_path": "examples/001_scalar_component/project.bcsproj",
		"parameters": {
			"scalar": {"gain": 3}
		}
	}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/parameters", bytes.NewReader(payload))

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

func TestRunRecordsRoundTrip(t *testing.T) {
	projectRoot := t.TempDir()
	loaded := &project.LoadedProject{
		Project: &model.Project{ProjectName: "recorded"},
		Root:    projectRoot,
	}
	input := runtimecore.RunInput{
		Inputs:  map[string]any{"value": 4.0},
		Context: map[string]any{"time": 0.0, "dt": 60.0},
	}
	result := &runtimecore.RunResult{
		OK:      true,
		Outputs: map[string]any{"result": 8.0},
	}

	summary, err := writeRunRecord(loaded, input, result)
	if err != nil {
		t.Fatal(err)
	}
	if summary.RelativePath == "" {
		t.Fatal("run summary did not include relative path")
	}
	summaries := loadRunSummaries(projectRoot)
	if len(summaries) != 1 {
		t.Fatalf("run summary count = %d", len(summaries))
	}
	if summaries[0].ID != summary.ID {
		t.Fatalf("run id = %s, want %s", summaries[0].ID, summary.ID)
	}
}

func newTestServer(t *testing.T) *Server {
	t.Helper()
	root, err := findRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	server, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	return server
}

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "examples")); err == nil {
			if _, err := os.Stat(filepath.Join(dir, "tools", "go", "go.mod")); err == nil {
				return dir, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}
