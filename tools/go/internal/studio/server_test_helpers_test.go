package studio

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	runtimecore "github.com/goniegonie/hvac-studio/tools/go/internal/runtime"
)

func hasProblemMessage(problems []Problem, message string) bool {
	for _, problem := range problems {
		if problem.Message == message {
			return true
		}
	}
	return false
}

func hasProblemMessageContaining(problems []Problem, text string) bool {
	_, ok := findProblemMessageContaining(problems, text)
	return ok
}

func findProblemMessageContaining(problems []Problem, text string) (Problem, bool) {
	for _, problem := range problems {
		if strings.Contains(problem.Message, text) {
			return problem, true
		}
	}
	return Problem{}, false
}

func hasComponentLog(logs []runtimecore.ComponentLog, component string, stage string, severity string, message string) bool {
	_, ok := findComponentLog(logs, component, stage, severity, message)
	return ok
}

func findComponentLog(logs []runtimecore.ComponentLog, component string, stage string, severity string, message string) (runtimecore.ComponentLog, bool) {
	for _, log := range logs {
		if log.Component == component && log.Stage == stage && log.Severity == severity && log.Message == message {
			return log, true
		}
	}
	return runtimecore.ComponentLog{}, false
}

func createWorkspaceProject(t *testing.T, server *Server, name string) ProjectSummary {
	t.Helper()
	payload, err := json.Marshal(map[string]any{"name": name})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader(payload))
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Project ProjectSummary `json:"project"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body.Project
}

func writeBrokenScalarSource(t *testing.T, project ProjectSummary) {
	t.Helper()
	sourcePath := filepath.Join(filepath.Dir(project.ProjectPath), "components", "scalar.py")
	if err := os.WriteFile(sourcePath, []byte("class ScalarComponent:\n    pass\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertSourceGateRejectsRequest(t *testing.T, server *Server, method string, path string, payload any) {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, bytes.NewReader(data))
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("%s status = %d body=%s", path, response.Code, response.Body.String())
	}
	var body apiError
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body.Message, "project source validation failed") {
		t.Fatalf("message = %s", body.Message)
	}
	if !hasProblemMessage(body.Problems, "evaluate method is missing") {
		t.Fatalf("source problem missing from %#v", body.Problems)
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

func getRouteBody(t *testing.T, server *Server, path string) []byte {
	t.Helper()
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("%s status = %d body=%s", path, response.Code, response.Body.String())
	}
	return response.Body.Bytes()
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

func seedTestRuntimeSupport(t *testing.T, root string) {
	t.Helper()
	for rel, content := range map[string]string{
		"bin/bcs-runner.exe":                 "runner",
		"bin/bcs-env.exe":                    "env",
		"runtime/manifest.json":              `{"runtime":"test"}`,
		"runtime/python/python.exe":          "python",
		"python/bcs_sdk/bcs_sdk/__init__.py": "from .client import RunnerClient\n",
		"python/bcs_sdk/bcs_sdk/client.py":   "class RunnerClient:\n    pass\n",
		"schema/serve-request.schema.json":   `{"$schema":"https://json-schema.org/draft/2020-12/schema","title":"Serve Request"}`,
		"schema/serve-response.schema.json":  `{"$schema":"https://json-schema.org/draft/2020-12/schema","title":"Serve Response"}`,
	} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func testPythonExecutable(t *testing.T) string {
	t.Helper()
	if path := strings.TrimSpace(os.Getenv("HVAC_STUDIO_PYTHON")); path != "" {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	path, err := exec.LookPath("python")
	if err != nil {
		t.Skip("python executable is not available for exported project runtime smoke")
	}
	return path
}

func hasColumnSuggestion(values []ColumnSuggestion, publicID string, column string) bool {
	for _, item := range values {
		if item.PublicID == publicID && item.Column == column {
			return true
		}
	}
	return false
}

func hasDatasetSummary(values []DatasetSummary, id string) bool {
	for _, item := range values {
		if item.ID == id {
			return true
		}
	}
	return false
}

func hasComponentTemplate(values []ComponentTemplateSummary, id string) bool {
	for _, item := range values {
		if item.ID == id {
			return true
		}
	}
	return false
}

func hasParameterDiff(values []ParameterDiff, component string, parameter string) bool {
	for _, item := range values {
		if item.Component == component && item.Parameter == parameter && item.Exists {
			return true
		}
	}
	return false
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

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func seedExportWorkflowArtifacts(t *testing.T, projectRoot string) {
	t.Helper()
	writeTestFile(t, filepath.Join(projectRoot, "parameter_sets", "baseline.json"), `{
  "id": "baseline",
  "components": {
    "scalar": {
      "gain": 2
    }
  }
}
`)
	writeTestFile(t, filepath.Join(projectRoot, "datasets", "scalar_validation.csv"), "value,observed_result\n4,8\n")
	writeTestFile(t, filepath.Join(projectRoot, "scenarios", "case01.json"), `{
  "id": "case01",
  "name": "Case 01",
  "inputs": {
    "value": 4
  },
  "context": {
    "time": 0
  }
}
`)
	writeTestFile(t, filepath.Join(projectRoot, "validation", "mappings", "scalar_validation.json"), `{
  "id": "scalar_validation",
  "dataset": "datasets/scalar_validation.csv",
  "input_columns": {
    "value": "value"
  },
  "observed_output_columns": {
    "result": "observed_result"
  }
}
`)
	writeTestFile(t, filepath.Join(projectRoot, "calibration", "setups", "scalar_gain.json"), `{
  "id": "scalar_gain",
  "algorithm": "grid",
  "mapping": "validation/mappings/scalar_validation.json",
  "objective": {
    "metric": "rmse"
  },
  "parameters": [
    {
      "component": "scalar",
      "name": "gain",
      "min": 1,
      "max": 3,
      "step": 1
    }
  ]
}
`)
	writeTestFile(t, filepath.Join(projectRoot, "optimization", "setups", "scalar_grid.json"), `{
  "id": "scalar_grid",
  "algorithm": "grid",
  "base_inputs": {
    "value": 4
  },
  "objective": {
    "output": "result",
    "sense": "max"
  },
  "decision_variables": [
    {
      "kind": "public_input",
      "name": "value",
      "min": 2,
      "max": 4,
      "step": 1
    }
  ]
}
`)
	writeTestFile(t, filepath.Join(projectRoot, "runs", "run-test.json"), `{"id":"run-test","result":{"outputs":{"result":8}}}`)
	writeTestFile(t, filepath.Join(projectRoot, "batches", "batch-test.json"), `{"id":"batch-test","cases":[]}`)
	writeTestFile(t, filepath.Join(projectRoot, "validation", "runs", "validation-test.json"), `{"id":"validation-test","result":{"row_count":1}}`)
	writeTestFile(t, filepath.Join(projectRoot, "calibration", "results", "calibration-test.json"), `{"id":"calibration-test","result":{"best_objective":0}}`)
	writeTestFile(t, filepath.Join(projectRoot, "optimization", "results", "optimization-test.json"), `{"id":"optimization-test","result":{"best_objective":0}}`)
}
