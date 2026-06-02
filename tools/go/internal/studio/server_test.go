package studio

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
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
	if !bytes.Contains(body, []byte(`type="module"`)) {
		t.Fatalf("index did not load the Studio JavaScript module")
	}
}

func TestStaticModuleEntrypointServes(t *testing.T) {
	server := newTestServer(t)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/js/app.js", nil)

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(body, []byte(`from "./state.js"`)) {
		t.Fatalf("module entrypoint did not contain expected imports")
	}
}

func TestCreateProjectEndpointCreatesWorkspaceProject(t *testing.T) {
	root, server := newIsolatedTestServer(t)
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

func TestCopyProjectEndpointCreatesEditableWorkspaceCopy(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Seed Project"}`)))
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

	copyPayload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"name":         "Copied Project",
	})
	if err != nil {
		t.Fatal(err)
	}
	copyResponse := httptest.NewRecorder()
	copyRequest := httptest.NewRequest(http.MethodPost, "/api/projects/copy", bytes.NewReader(copyPayload))
	server.Handler().ServeHTTP(copyResponse, copyRequest)
	if copyResponse.Code != http.StatusCreated {
		t.Fatalf("copy status = %d body=%s", copyResponse.Code, copyResponse.Body.String())
	}
	var copyBody struct {
		Project ProjectSummary `json:"project"`
	}
	if err := json.Unmarshal(copyResponse.Body.Bytes(), &copyBody); err != nil {
		t.Fatal(err)
	}
	if copyBody.Project.Source != "workspace" {
		t.Fatalf("source = %s", copyBody.Project.Source)
	}
	if copyBody.Project.Name != "Copied Project" {
		t.Fatalf("name = %s", copyBody.Project.Name)
	}
	if _, err := os.Stat(filepath.Join(root, "projects", "copied-project", "components", "scalar.py")); err != nil {
		t.Fatal(err)
	}
	loaded, err := project.Load(copyBody.Project.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Project.ProjectName != "Copied Project" {
		t.Fatalf("project_name = %s", loaded.Project.ProjectName)
	}

	updatePayload, err := json.Marshal(map[string]any{
		"project_path": copyBody.Project.ProjectPath,
		"parameters": map[string]any{
			"scalar": map[string]any{"gain": 5.0},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	updateResponse := httptest.NewRecorder()
	updateRequest := httptest.NewRequest(http.MethodPost, "/api/project/parameters", bytes.NewReader(updatePayload))
	server.Handler().ServeHTTP(updateResponse, updateRequest)
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("update copied project status = %d body=%s", updateResponse.Code, updateResponse.Body.String())
	}
}

func TestCreateComponentEndpointCreatesWorkspaceComponent(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Component Project"}`)))
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
		"name":         "Second Gain",
		"template":     "scalar",
	})
	if err != nil {
		t.Fatal(err)
	}
	componentResponse := httptest.NewRecorder()
	componentRequest := httptest.NewRequest(http.MethodPost, "/api/project/components", bytes.NewReader(payload))

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
	if componentBody.Component.ID != "second_gain" {
		t.Fatalf("component id = %s, want second_gain", componentBody.Component.ID)
	}
	if _, err := os.Stat(filepath.Join(root, "projects", "component-project", "components", "second_gain.py")); err != nil {
		t.Fatal(err)
	}
	loaded, err := project.Load(createBody.Project.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, component := range loaded.Graph.Components {
		if component.ID == "second_gain" {
			found = true
		}
	}
	if !found {
		t.Fatal("created component was not written to graph")
	}
	for _, componentID := range loaded.Graph.Systems[0].Components {
		if componentID == "second_gain" {
			t.Fatal("new component should not be added to the runnable system yet")
		}
	}
}

func TestDuplicateComponentEndpointCopiesGraphAndSource(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Duplicate Component Project"}`)))
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
		"project_path":        createBody.Project.ProjectPath,
		"source_component_id": "scalar",
		"name":                "Scalar Copy",
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/components/duplicate", bytes.NewReader(payload))
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("duplicate status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Component model.Component `json:"component"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Component.ID != "scalar_copy" {
		t.Fatalf("component id = %s, want scalar_copy", body.Component.ID)
	}
	if body.Component.Class != "components.scalar_copy.ScalarComponent" {
		t.Fatalf("component class = %s", body.Component.Class)
	}
	if got := body.Component.Parameters["gain"]; got != 2.0 {
		t.Fatalf("copied gain = %v, want 2", got)
	}
	sourcePath := filepath.Join(root, "projects", "duplicate-component-project", "components", "scalar_copy.py")
	if _, err := os.Stat(sourcePath); err != nil {
		t.Fatal(err)
	}
	loaded, err := project.Load(createBody.Project.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, found := findComponent(loaded.Graph, "scalar_copy"); !found {
		t.Fatal("duplicated component was not written to graph")
	}
	if containsString(loaded.Graph.Systems[0].Components, "scalar_copy") {
		t.Fatal("duplicated component should not be added to the runnable system yet")
	}
}

func TestUpdateComponentEndpointRenamesWorkspaceComponent(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Rename Component Project"}`)))
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
		"component_id": "scalar",
		"name":         "Outdoor Air Signal",
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/components/update", bytes.NewReader(payload))
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("update status = %d body=%s", response.Code, response.Body.String())
	}

	loaded, err := project.Load(createBody.Project.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Graph.Components[0].Name != "Outdoor Air Signal" {
		t.Fatalf("component name = %s", loaded.Graph.Components[0].Name)
	}
}

func TestCreateComponentEndpointRejectsExamples(t *testing.T) {
	server := newTestServer(t)
	payload := []byte(`{
		"project_path": "examples/001_scalar_component/project.bcsproj",
		"name": "Example Edit"
	}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/components", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}

func TestDuplicateComponentEndpointRejectsExamples(t *testing.T) {
	server := newTestServer(t)
	payload := []byte(`{
		"project_path": "examples/001_scalar_component/project.bcsproj",
		"source_component_id": "scalar",
		"name": "Example Copy"
	}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/components/duplicate", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}

func TestUpdateComponentEndpointRejectsExamples(t *testing.T) {
	server := newTestServer(t)
	payload := []byte(`{
		"project_path": "examples/001_scalar_component/project.bcsproj",
		"component_id": "scalar",
		"name": "Example Rename"
	}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/components/update", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}

func TestDeleteComponentEndpointRemovesUnusedWorkspaceComponent(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Delete Component Project"}`)))
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
		"project_path": createBody.Project.ProjectPath,
		"name":         "Scratch Gain",
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
	sourcePath := filepath.Join(root, "projects", "delete-component-project", "components", "scratch_gain.py")
	if _, err := os.Stat(sourcePath); err != nil {
		t.Fatal(err)
	}

	deletePayload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"component_id": "scratch_gain",
	})
	if err != nil {
		t.Fatal(err)
	}
	deleteResponse := httptest.NewRecorder()
	deleteRequest := httptest.NewRequest(http.MethodPost, "/api/project/components/delete", bytes.NewReader(deletePayload))
	server.Handler().ServeHTTP(deleteResponse, deleteRequest)
	if deleteResponse.Code != http.StatusOK {
		t.Fatalf("delete status = %d body=%s", deleteResponse.Code, deleteResponse.Body.String())
	}

	loaded, err := project.Load(createBody.Project.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, found := findComponent(loaded.Graph, "scratch_gain"); found {
		t.Fatal("deleted component should be removed from graph")
	}
	if _, err := os.Stat(sourcePath); !os.IsNotExist(err) {
		t.Fatalf("component source should be removed, stat err=%v", err)
	}
}

func TestDeleteComponentEndpointRejectsSystemComponent(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Reject Delete Component Project"}`)))
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
		"component_id": "scalar",
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/components/delete", bytes.NewReader(payload))
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}

func TestDeleteComponentEndpointRejectsExamples(t *testing.T) {
	server := newTestServer(t)
	payload := []byte(`{
		"project_path": "examples/001_scalar_component/project.bcsproj",
		"component_id": "scalar"
	}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/components/delete", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}

func TestCreateNodeEndpointAddsPublicIOAndDefaultInput(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Node Project"}`)))
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

	inputPayload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"component_id": "scalar",
		"direction":    "input",
		"id":           "bias",
		"value_type":   "float",
		"default":      4.0,
	})
	if err != nil {
		t.Fatal(err)
	}
	inputResponse := httptest.NewRecorder()
	inputRequest := httptest.NewRequest(http.MethodPost, "/api/project/nodes", bytes.NewReader(inputPayload))
	server.Handler().ServeHTTP(inputResponse, inputRequest)
	if inputResponse.Code != http.StatusCreated {
		t.Fatalf("input node status = %d body=%s", inputResponse.Code, inputResponse.Body.String())
	}

	outputPayload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"component_id": "scalar",
		"direction":    "output",
		"id":           "adjusted",
		"value_type":   "float",
	})
	if err != nil {
		t.Fatal(err)
	}
	outputResponse := httptest.NewRecorder()
	outputRequest := httptest.NewRequest(http.MethodPost, "/api/project/nodes", bytes.NewReader(outputPayload))
	server.Handler().ServeHTTP(outputResponse, outputRequest)
	if outputResponse.Code != http.StatusCreated {
		t.Fatalf("output node status = %d body=%s", outputResponse.Code, outputResponse.Body.String())
	}

	loaded, err := project.Load(createBody.Project.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	component := loaded.Graph.Components[0]
	if !componentHasNode(component, "bias") {
		t.Fatal("input node was not written to graph")
	}
	if !componentHasNode(component, "adjusted") {
		t.Fatal("output node was not written to graph")
	}
	foundPublicInput := false
	for _, input := range loaded.Graph.Systems[0].PublicInputs {
		if input.ID == "scalar_bias" {
			foundPublicInput = true
			break
		}
	}
	if !foundPublicInput {
		t.Fatal("input node was not exposed as public input")
	}
	foundPublicOutput := false
	for _, output := range loaded.Graph.Systems[0].PublicOutputs {
		if output.ID == "scalar_adjusted" {
			foundPublicOutput = true
			break
		}
	}
	if !foundPublicOutput {
		t.Fatal("output node was not exposed as public output")
	}
	input, err := runtimecore.LoadInput(filepath.Join(loaded.Root, loaded.Project.DefaultInput))
	if err != nil {
		t.Fatal(err)
	}
	if got := input.Inputs["scalar_bias"]; got != 4.0 {
		t.Fatalf("scalar_bias default = %v, want 4", got)
	}
}

func TestDeleteNodeEndpointCleansPublicIOAndConnections(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Delete Node Project"}`)))
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

	nodePayload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"component_id": "scalar",
		"direction":    "input",
		"id":           "bias",
		"default":      4.0,
	})
	if err != nil {
		t.Fatal(err)
	}
	nodeResponse := httptest.NewRecorder()
	nodeRequest := httptest.NewRequest(http.MethodPost, "/api/project/nodes", bytes.NewReader(nodePayload))
	server.Handler().ServeHTTP(nodeResponse, nodeRequest)
	if nodeResponse.Code != http.StatusCreated {
		t.Fatalf("node status = %d body=%s", nodeResponse.Code, nodeResponse.Body.String())
	}

	deletePayload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"component_id": "scalar",
		"node_id":      "bias",
	})
	if err != nil {
		t.Fatal(err)
	}
	deleteResponse := httptest.NewRecorder()
	deleteRequest := httptest.NewRequest(http.MethodPost, "/api/project/nodes/delete", bytes.NewReader(deletePayload))
	server.Handler().ServeHTTP(deleteResponse, deleteRequest)
	if deleteResponse.Code != http.StatusOK {
		t.Fatalf("delete input node status = %d body=%s", deleteResponse.Code, deleteResponse.Body.String())
	}

	loaded, err := project.Load(createBody.Project.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	if componentHasNode(loaded.Graph.Components[0], "bias") {
		t.Fatal("deleted input node should be removed from component")
	}
	for _, input := range loaded.Graph.Systems[0].PublicInputs {
		if input.ID == "scalar_bias" {
			t.Fatal("deleted input node public input should be removed")
		}
	}
	runInput, err := runtimecore.LoadInput(filepath.Join(loaded.Root, loaded.Project.DefaultInput))
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := runInput.Inputs["scalar_bias"]; exists {
		t.Fatal("deleted input node default input should be removed")
	}

	componentPayload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"name":         "Second Gain",
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
	includePayload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"component_id": "second_gain",
	})
	if err != nil {
		t.Fatal(err)
	}
	includeResponse := httptest.NewRecorder()
	includeRequest := httptest.NewRequest(http.MethodPost, "/api/project/system/components", bytes.NewReader(includePayload))
	server.Handler().ServeHTTP(includeResponse, includeRequest)
	if includeResponse.Code != http.StatusOK {
		t.Fatalf("include status = %d body=%s", includeResponse.Code, includeResponse.Body.String())
	}
	connectionPayload, err := json.Marshal(map[string]any{
		"project_path":   createBody.Project.ProjectPath,
		"from_component": "scalar",
		"from_node":      "result",
		"to_component":   "second_gain",
		"to_node":        "value",
	})
	if err != nil {
		t.Fatal(err)
	}
	connectionResponse := httptest.NewRecorder()
	connectionRequest := httptest.NewRequest(http.MethodPost, "/api/project/connections", bytes.NewReader(connectionPayload))
	server.Handler().ServeHTTP(connectionResponse, connectionRequest)
	if connectionResponse.Code != http.StatusCreated {
		t.Fatalf("connection status = %d body=%s", connectionResponse.Code, connectionResponse.Body.String())
	}
	deleteOutputPayload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"component_id": "scalar",
		"node_id":      "result",
	})
	if err != nil {
		t.Fatal(err)
	}
	deleteOutputResponse := httptest.NewRecorder()
	deleteOutputRequest := httptest.NewRequest(http.MethodPost, "/api/project/nodes/delete", bytes.NewReader(deleteOutputPayload))
	server.Handler().ServeHTTP(deleteOutputResponse, deleteOutputRequest)
	if deleteOutputResponse.Code != http.StatusOK {
		t.Fatalf("delete output node status = %d body=%s", deleteOutputResponse.Code, deleteOutputResponse.Body.String())
	}
	loaded, err = project.Load(createBody.Project.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	if componentHasNode(loaded.Graph.Components[0], "result") {
		t.Fatal("deleted output node should be removed from component")
	}
	if len(loaded.Graph.Connections) != 0 || len(loaded.Graph.Systems[0].Connections) != 0 {
		t.Fatalf("connections after output delete = graph:%d system:%d", len(loaded.Graph.Connections), len(loaded.Graph.Systems[0].Connections))
	}
	foundRestoredInput := false
	for _, input := range loaded.Graph.Systems[0].PublicInputs {
		if input.ID == "second_gain_value" {
			foundRestoredInput = true
		}
	}
	if !foundRestoredInput {
		t.Fatal("target input should be restored as public after source output node deletion")
	}
}

func TestCreateNodeEndpointRejectsExamples(t *testing.T) {
	server := newTestServer(t)
	payload := []byte(`{
		"project_path": "examples/001_scalar_component/project.bcsproj",
		"component_id": "scalar",
		"direction": "input",
		"id": "bias"
	}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/nodes", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}

func TestDeleteNodeEndpointRejectsExamples(t *testing.T) {
	server := newTestServer(t)
	payload := []byte(`{
		"project_path": "examples/001_scalar_component/project.bcsproj",
		"component_id": "scalar",
		"node_id": "value"
	}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/nodes/delete", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}

func TestIncludeComponentEndpointAddsPublicIOAndDefaultInput(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"System Project"}`)))
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
		"project_path": createBody.Project.ProjectPath,
		"name":         "Second Gain",
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

	includePayload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"component_id": "second_gain",
	})
	if err != nil {
		t.Fatal(err)
	}
	includeResponse := httptest.NewRecorder()
	includeRequest := httptest.NewRequest(http.MethodPost, "/api/project/system/components", bytes.NewReader(includePayload))
	server.Handler().ServeHTTP(includeResponse, includeRequest)
	if includeResponse.Code != http.StatusOK {
		t.Fatalf("include status = %d body=%s", includeResponse.Code, includeResponse.Body.String())
	}

	loaded, err := project.Load(createBody.Project.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	if !containsString(loaded.Graph.Systems[0].Components, "second_gain") {
		t.Fatal("component was not added to the entry system")
	}
	input, err := runtimecore.LoadInput(filepath.Join(loaded.Root, loaded.Project.DefaultInput))
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := input.Inputs["second_gain_value"]; !exists {
		t.Fatal("default input was not extended with second_gain_value")
	}

	runPayload, err := json.Marshal(map[string]any{"project_path": createBody.Project.ProjectPath})
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
	}
	if err := json.Unmarshal(runResponse.Body.Bytes(), &runBody); err != nil {
		t.Fatal(err)
	}
	if _, exists := runBody.Result.Outputs["second_gain_result"]; !exists {
		t.Fatal("run output did not include second_gain_result")
	}
}

func TestIncludeComponentEndpointRejectsExamples(t *testing.T) {
	server := newTestServer(t)
	payload := []byte(`{
		"project_path": "examples/001_scalar_component/project.bcsproj",
		"component_id": "scalar"
	}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/system/components", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}

func TestCreateConnectionEndpointConnectsComponents(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Connection Project"}`)))
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
		"project_path": createBody.Project.ProjectPath,
		"name":         "Second Gain",
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
	includePayload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"component_id": "second_gain",
	})
	if err != nil {
		t.Fatal(err)
	}
	includeResponse := httptest.NewRecorder()
	includeRequest := httptest.NewRequest(http.MethodPost, "/api/project/system/components", bytes.NewReader(includePayload))
	server.Handler().ServeHTTP(includeResponse, includeRequest)
	if includeResponse.Code != http.StatusOK {
		t.Fatalf("include status = %d body=%s", includeResponse.Code, includeResponse.Body.String())
	}

	connectionPayload, err := json.Marshal(map[string]any{
		"project_path":   createBody.Project.ProjectPath,
		"from_component": "scalar",
		"from_node":      "result",
		"to_component":   "second_gain",
		"to_node":        "value",
	})
	if err != nil {
		t.Fatal(err)
	}
	connectionResponse := httptest.NewRecorder()
	connectionRequest := httptest.NewRequest(http.MethodPost, "/api/project/connections", bytes.NewReader(connectionPayload))
	server.Handler().ServeHTTP(connectionResponse, connectionRequest)
	if connectionResponse.Code != http.StatusCreated {
		t.Fatalf("connection status = %d body=%s", connectionResponse.Code, connectionResponse.Body.String())
	}
	var connectionBody struct {
		Connection model.Connection `json:"connection"`
	}
	if err := json.Unmarshal(connectionResponse.Body.Bytes(), &connectionBody); err != nil {
		t.Fatal(err)
	}

	loaded, err := project.Load(createBody.Project.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Graph.Connections) != 1 {
		t.Fatalf("connection count = %d", len(loaded.Graph.Connections))
	}
	if len(loaded.Graph.Systems[0].Connections) != 1 {
		t.Fatalf("system connection count = %d", len(loaded.Graph.Systems[0].Connections))
	}
	for _, input := range loaded.Graph.Systems[0].PublicInputs {
		if input.ID == "second_gain_value" {
			t.Fatal("connected target input should no longer be public")
		}
	}
	input, err := runtimecore.LoadInput(filepath.Join(loaded.Root, loaded.Project.DefaultInput))
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := input.Inputs["second_gain_value"]; exists {
		t.Fatal("connected target default input should be removed")
	}

	runResponse := httptest.NewRecorder()
	runPayload, err := json.Marshal(map[string]any{"project_path": createBody.Project.ProjectPath})
	if err != nil {
		t.Fatal(err)
	}
	runRequest := httptest.NewRequest(http.MethodPost, "/api/run", bytes.NewReader(runPayload))
	server.Handler().ServeHTTP(runResponse, runRequest)
	if runResponse.Code != http.StatusOK {
		t.Fatalf("run status = %d body=%s", runResponse.Code, runResponse.Body.String())
	}
	var runBody struct {
		Result struct {
			Outputs map[string]float64 `json:"outputs"`
		} `json:"result"`
	}
	if err := json.Unmarshal(runResponse.Body.Bytes(), &runBody); err != nil {
		t.Fatal(err)
	}
	if runBody.Result.Outputs["second_gain_result"] != 8 {
		t.Fatalf("second_gain_result = %v, want 8", runBody.Result.Outputs["second_gain_result"])
	}

	deletePayload, err := json.Marshal(map[string]any{
		"project_path":  createBody.Project.ProjectPath,
		"connection_id": connectionBody.Connection.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	deleteResponse := httptest.NewRecorder()
	deleteRequest := httptest.NewRequest(http.MethodPost, "/api/project/connections/delete", bytes.NewReader(deletePayload))
	server.Handler().ServeHTTP(deleteResponse, deleteRequest)
	if deleteResponse.Code != http.StatusOK {
		t.Fatalf("delete connection status = %d body=%s", deleteResponse.Code, deleteResponse.Body.String())
	}
	loaded, err = project.Load(createBody.Project.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Graph.Connections) != 0 {
		t.Fatalf("connection count after delete = %d", len(loaded.Graph.Connections))
	}
	if len(loaded.Graph.Systems[0].Connections) != 0 {
		t.Fatalf("system connection count after delete = %d", len(loaded.Graph.Systems[0].Connections))
	}
	foundPublicInput := false
	for _, input := range loaded.Graph.Systems[0].PublicInputs {
		if input.ID == "second_gain_value" {
			foundPublicInput = true
			break
		}
	}
	if !foundPublicInput {
		t.Fatal("deleted connection target input should become public again")
	}
	input, err = runtimecore.LoadInput(filepath.Join(loaded.Root, loaded.Project.DefaultInput))
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := input.Inputs["second_gain_value"]; !exists {
		t.Fatal("deleted connection target default input should be restored")
	}
}

func TestRemoveComponentFromSystemEndpointCleansRuntimeSurface(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Removal Project"}`)))
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
		"project_path": createBody.Project.ProjectPath,
		"name":         "Second Gain",
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
	includePayload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"component_id": "second_gain",
	})
	if err != nil {
		t.Fatal(err)
	}
	includeResponse := httptest.NewRecorder()
	includeRequest := httptest.NewRequest(http.MethodPost, "/api/project/system/components", bytes.NewReader(includePayload))
	server.Handler().ServeHTTP(includeResponse, includeRequest)
	if includeResponse.Code != http.StatusOK {
		t.Fatalf("include status = %d body=%s", includeResponse.Code, includeResponse.Body.String())
	}
	connectionPayload, err := json.Marshal(map[string]any{
		"project_path":   createBody.Project.ProjectPath,
		"from_component": "scalar",
		"from_node":      "result",
		"to_component":   "second_gain",
		"to_node":        "value",
	})
	if err != nil {
		t.Fatal(err)
	}
	connectionResponse := httptest.NewRecorder()
	connectionRequest := httptest.NewRequest(http.MethodPost, "/api/project/connections", bytes.NewReader(connectionPayload))
	server.Handler().ServeHTTP(connectionResponse, connectionRequest)
	if connectionResponse.Code != http.StatusCreated {
		t.Fatalf("connection status = %d body=%s", connectionResponse.Code, connectionResponse.Body.String())
	}

	removePayload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"component_id": "second_gain",
	})
	if err != nil {
		t.Fatal(err)
	}
	removeResponse := httptest.NewRecorder()
	removeRequest := httptest.NewRequest(http.MethodPost, "/api/project/system/components/remove", bytes.NewReader(removePayload))
	server.Handler().ServeHTTP(removeResponse, removeRequest)
	if removeResponse.Code != http.StatusOK {
		t.Fatalf("remove status = %d body=%s", removeResponse.Code, removeResponse.Body.String())
	}

	loaded, err := project.Load(createBody.Project.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	if !componentHasNode(loaded.Graph.Components[1], "value") {
		t.Fatal("removed system component artifact should remain in graph")
	}
	if containsString(loaded.Graph.Systems[0].Components, "second_gain") {
		t.Fatal("removed component should not remain in system")
	}
	if len(loaded.Graph.Systems[0].Connections) != 0 {
		t.Fatalf("system connection count = %d, want 0", len(loaded.Graph.Systems[0].Connections))
	}
	if len(loaded.Graph.Connections) != 0 {
		t.Fatalf("graph connection count = %d, want 0", len(loaded.Graph.Connections))
	}
	for _, input := range loaded.Graph.Systems[0].PublicInputs {
		if input.Component == "second_gain" {
			t.Fatal("removed component public input should be removed")
		}
	}
	input, err := runtimecore.LoadInput(filepath.Join(loaded.Root, loaded.Project.DefaultInput))
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := input.Inputs["second_gain_value"]; exists {
		t.Fatal("removed component default input should be removed")
	}
}

func TestCreateConnectionEndpointRejectsExamples(t *testing.T) {
	server := newTestServer(t)
	payload := []byte(`{
		"project_path": "examples/003_feedforward_system/project.bcsproj",
		"from_component": "load_model",
		"from_node": "adjusted_load_kw",
		"to_component": "controller",
		"to_node": "cooling_load_kw"
	}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/connections", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}

func TestRemoveComponentFromSystemEndpointRejectsExamples(t *testing.T) {
	server := newTestServer(t)
	payload := []byte(`{
		"project_path": "examples/003_feedforward_system/project.bcsproj",
		"component_id": "chiller"
	}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/system/components/remove", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}

func TestDeleteConnectionEndpointRejectsExamples(t *testing.T) {
	server := newTestServer(t)
	payload := []byte(`{
		"project_path": "examples/003_feedforward_system/project.bcsproj",
		"connection_id": "load_to_controller"
	}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/connections/delete", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}

func TestUpdateParametersEndpointWritesWorkspaceGraph(t *testing.T) {
	_, server := newIsolatedTestServer(t)

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
			"scalar": map[string]any{"gain": 3.0, "offset": 2.0, "scratch": 7.0},
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
	if got := loaded.Graph.Components[0].Parameters["offset"]; got != 2.0 {
		t.Fatalf("offset = %v, want 2", got)
	}
	if got := loaded.Graph.Components[0].Parameters["scratch"]; got != 7.0 {
		t.Fatalf("scratch = %v, want 7", got)
	}

	deletePayload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"component_id": "scalar",
		"name":         "scratch",
	})
	if err != nil {
		t.Fatal(err)
	}
	deleteResponse := httptest.NewRecorder()
	deleteRequest := httptest.NewRequest(http.MethodPost, "/api/project/parameters/delete", bytes.NewReader(deletePayload))

	server.Handler().ServeHTTP(deleteResponse, deleteRequest)

	if deleteResponse.Code != http.StatusOK {
		t.Fatalf("delete status = %d body=%s", deleteResponse.Code, deleteResponse.Body.String())
	}
	loaded, err = project.Load(createBody.Project.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := loaded.Graph.Components[0].Parameters["scratch"]; exists {
		t.Fatal("deleted parameter should be removed from component")
	}
	if got := loaded.Graph.Components[0].Parameters["gain"]; got != 3.0 {
		t.Fatalf("gain after delete = %v, want 3", got)
	}
	if got := loaded.Graph.Components[0].Parameters["offset"]; got != 2.0 {
		t.Fatalf("offset after delete = %v, want 2", got)
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

func TestDeleteParameterEndpointRejectsExamples(t *testing.T) {
	server := newTestServer(t)
	payload := []byte(`{
		"project_path": "examples/001_scalar_component/project.bcsproj",
		"component_id": "scalar",
		"name": "gain"
	}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/parameters/delete", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}

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

func TestRunRecordEndpointReturnsSavedRecord(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Run Record Project"}`)))
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

	runPayload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"save":         true,
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
		RunRecord RunSummary `json:"run_record"`
	}
	if err := json.Unmarshal(runResponse.Body.Bytes(), &runBody); err != nil {
		t.Fatal(err)
	}

	recordResponse := httptest.NewRecorder()
	recordRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/project/run?project_path="+url.QueryEscape(createBody.Project.ProjectPath)+"&run_id="+url.QueryEscape(runBody.RunRecord.ID),
		nil,
	)
	server.Handler().ServeHTTP(recordResponse, recordRequest)
	if recordResponse.Code != http.StatusOK {
		t.Fatalf("record status = %d body=%s", recordResponse.Code, recordResponse.Body.String())
	}
	var recordBody struct {
		RunRecord RunRecord `json:"run_record"`
	}
	if err := json.Unmarshal(recordResponse.Body.Bytes(), &recordBody); err != nil {
		t.Fatal(err)
	}
	if recordBody.RunRecord.ID != runBody.RunRecord.ID {
		t.Fatalf("record id = %s, want %s", recordBody.RunRecord.ID, runBody.RunRecord.ID)
	}
	if recordBody.RunRecord.Result.Outputs["result"] != 8.0 {
		t.Fatalf("record result = %v, want 8", recordBody.RunRecord.Result.Outputs["result"])
	}
}

func TestSourceEndpointReadsExampleSource(t *testing.T) {
	server := newTestServer(t)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/project/source?project_path=examples/001_scalar_component/project.bcsproj&component_id=gain",
		nil,
	)

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Source SourceDetail `json:"source"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.Source.ReadOnly {
		t.Fatal("example source should be read-only")
	}
	if body.Source.RelativePath != "components/scalar.py" {
		t.Fatalf("relative path = %s", body.Source.RelativePath)
	}
	if !strings.Contains(body.Source.Content, "class Gain") {
		t.Fatal("source did not include Gain")
	}
}

func TestUpdateSourceEndpointWritesWorkspaceSource(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Source Project"}`)))
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

	content := "class ScalarComponent:\n    pass\n"
	payload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"component_id": "scalar",
		"content":      content,
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/source", bytes.NewReader(payload))
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	sourceBytes, err := os.ReadFile(filepath.Join(root, "projects", "source-project", "components", "scalar.py"))
	if err != nil {
		t.Fatal(err)
	}
	if string(sourceBytes) != content {
		t.Fatalf("source = %q", string(sourceBytes))
	}
}

func TestCheckSourceEndpointReportsContractProblems(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Source Check Project"}`)))
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
		"component_id": "scalar",
		"content":      "class WrongName:\n    pass\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/source/check", bytes.NewReader(payload))
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Check SourceCheck `json:"check"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Check.OK {
		t.Fatal("source check should fail when class and evaluate are missing")
	}
	if body.Check.ExpectedClass != "ScalarComponent" {
		t.Fatalf("expected class = %s", body.Check.ExpectedClass)
	}
	if len(body.Check.Problems) < 2 {
		t.Fatalf("problems = %#v", body.Check.Problems)
	}
}

func TestCheckSourceEndpointAcceptsWorkspaceSource(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Source Check Valid Project"}`)))
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
	sourceBytes, err := os.ReadFile(filepath.Join(root, "projects", "source-check-valid-project", "components", "scalar.py"))
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"component_id": "scalar",
		"content":      string(sourceBytes),
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/source/check", bytes.NewReader(payload))
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Check SourceCheck `json:"check"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.Check.OK {
		t.Fatalf("source check problems = %#v", body.Check.Problems)
	}
}

func TestUpdateSourceEndpointRejectsExamples(t *testing.T) {
	server := newTestServer(t)
	payload := []byte(`{
		"project_path": "examples/001_scalar_component/project.bcsproj",
		"component_id": "scalar",
		"content": "class ScalarComponent:\n    pass\n"
	}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/source", bytes.NewReader(payload))

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
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
	if batchBody.Summary.CaseCount != 2 || batchBody.Summary.OKCount != 2 {
		t.Fatalf("batch counts = %d/%d, want 2/2", batchBody.Summary.OKCount, batchBody.Summary.CaseCount)
	}
	if len(batchBody.Batch.Cases) != 2 {
		t.Fatalf("case count = %d, want 2", len(batchBody.Batch.Cases))
	}
	if got := batchBody.Batch.Cases[0].Result.Outputs["result"]; got != 4.0 {
		t.Fatalf("first output = %v, want 4", got)
	}
	if got := batchBody.Batch.Cases[1].Result.Outputs["result"]; got != 6.0 {
		t.Fatalf("second output = %v, want 6", got)
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

func TestExportEndpointWritesRuntimeArtifact(t *testing.T) {
	root, server := newIsolatedTestServer(t)
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
	if body.Export.Runner != "bin/bcs-runner.exe" {
		t.Fatalf("runner = %s", body.Export.Runner)
	}
	expectedFiles := []string{
		"project/project.bcsproj",
		"project/graph.json",
		"project/components/__init__.py",
		"project/components/scalar.py",
		"project/inputs/case01.json",
	}
	exportRoot := filepath.Join(root, "projects", "export-project", "exports", "runtime_package")
	for _, rel := range expectedFiles {
		if !containsString(body.Export.Files, rel) {
			t.Fatalf("export files missing %s in %v", rel, body.Export.Files)
		}
		if _, err := os.Stat(filepath.Join(exportRoot, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("export file %s: %v", rel, err)
		}
	}
	for _, rel := range body.Export.Files {
		if strings.HasPrefix(rel, "project/runs/") || strings.HasPrefix(rel, "project/exports/") {
			t.Fatalf("export should not include generated project artifact %s", rel)
		}
	}
	if _, err := os.Stat(filepath.Join(exportRoot, "manifest.json")); err != nil {
		t.Fatalf("manifest: %v", err)
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

func newIsolatedTestServer(t *testing.T) (string, *Server) {
	t.Helper()
	root := t.TempDir()
	seedTestTemplates(t, root)
	server, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	return root, server
}

func seedTestTemplates(t *testing.T, root string) {
	t.Helper()
	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	if err := copyProjectTree(filepath.Join(repoRoot, "templates"), filepath.Join(root, "templates")); err != nil {
		t.Fatal(err)
	}
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
