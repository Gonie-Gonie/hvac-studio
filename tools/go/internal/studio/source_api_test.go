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

	"github.com/goniegonie/hvac-studio/tools/go/internal/model"
)

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
	if body.Source.Layout != "single_file_class" || body.Source.EditableRole != "single_file" {
		t.Fatalf("source metadata = layout %q role %q", body.Source.Layout, body.Source.EditableRole)
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
	var body struct {
		Source SourceDetail `json:"source"`
		Check  SourceCheck  `json:"check"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Source.Content != content {
		t.Fatalf("response source = %q", body.Source.Content)
	}
	if body.Check.OK {
		t.Fatal("source save check should report missing evaluate")
	}
	if len(body.Check.Problems) == 0 {
		t.Fatal("source save check returned no problems")
	}
	sourceBytes, err := os.ReadFile(filepath.Join(root, "projects", "source-project", "components", "scalar.py"))
	if err != nil {
		t.Fatal(err)
	}
	if string(sourceBytes) != content {
		t.Fatalf("source = %q", string(sourceBytes))
	}
}

func TestUpdateSourceEndpointWritesGeneratedWrapperUserStepOnly(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	projectSummary := createWorkspaceProject(t, server, "Generated Wrapper Source Project")

	componentPayload, err := json.Marshal(map[string]any{
		"project_path": projectSummary.ProjectPath,
		"name":         "Second Gain",
		"template":     "scalar",
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

	componentRoot := filepath.Join(root, "projects", "generated-wrapper-source-project", "components", "second_gain")
	wrapperPath := filepath.Join(componentRoot, "wrapper.py")
	originalWrapper, err := os.ReadFile(wrapperPath)
	if err != nil {
		t.Fatal(err)
	}
	content := strings.Join([]string{
		"def step(inputs, state, params, context):",
		"    value = float(inputs.get(\"value\", 0.0))",
		"    gain = float(params.get(\"gain\", 2.0))",
		"    return {\"result\": value * gain + 1.0}, state",
		"",
	}, "\n")
	payload, err := json.Marshal(map[string]any{
		"project_path": projectSummary.ProjectPath,
		"component_id": "second_gain",
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
	var body struct {
		Source SourceDetail `json:"source"`
		Check  SourceCheck  `json:"check"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Source.RelativePath != "components/second_gain/user_step.py" ||
		body.Source.Layout != "generated_wrapper" ||
		body.Source.EditableRole != "user_step" {
		t.Fatalf("source metadata = %#v", body.Source)
	}
	if !body.Check.OK {
		t.Fatalf("source save check problems = %#v", body.Check.Problems)
	}
	userStepBytes, err := os.ReadFile(filepath.Join(componentRoot, "user_step.py"))
	if err != nil {
		t.Fatal(err)
	}
	if string(userStepBytes) != content {
		t.Fatalf("user step = %q", string(userStepBytes))
	}
	wrapperBytes, err := os.ReadFile(wrapperPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(wrapperBytes, originalWrapper) {
		t.Fatalf("generated wrapper changed during user step save")
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

func TestCheckSourceEndpointReportsEvaluateSignatureProblem(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Source Signature Project"}`)))
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
		"content":      "class ScalarComponent:\n    def evaluate(self, inputs):\n        return {\"result\": 1}, {}\n",
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
		t.Fatal("source check should fail when evaluate signature is wrong")
	}
	var signatureProblem *Problem
	for index := range body.Check.Problems {
		if strings.Contains(body.Check.Problems[index].Message, "evaluate signature") {
			signatureProblem = &body.Check.Problems[index]
			break
		}
	}
	if signatureProblem == nil {
		t.Fatalf("signature problem missing from %#v", body.Check.Problems)
	}
	if signatureProblem.Line != 2 {
		t.Fatalf("line = %d, want 2", signatureProblem.Line)
	}
}

func TestCheckSourceEndpointReportsReturnShapeProblems(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	projectSummary := createWorkspaceProject(t, server, "Source Return Shape Project")

	cases := []struct {
		name    string
		content string
		message string
		line    int
	}{
		{
			name: "missing state",
			content: strings.Join([]string{
				"class ScalarComponent:",
				"    def evaluate(self, inputs, state, params, context):",
				"        return {\"result\": 1}",
				"",
			}, "\n"),
			message: "return shape must be (outputs, state)",
			line:    3,
		},
		{
			name: "outputs not dictionary",
			content: strings.Join([]string{
				"class ScalarComponent:",
				"    def evaluate(self, inputs, state, params, context):",
				"        return 1, state",
				"",
			}, "\n"),
			message: "return outputs must be a dictionary",
			line:    3,
		},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			payload, err := json.Marshal(map[string]any{
				"project_path": projectSummary.ProjectPath,
				"component_id": "scalar",
				"content":      item.content,
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
				t.Fatalf("source check should fail for return shape: %#v", body.Check.Problems)
			}
			problem, ok := findProblemMessageContaining(body.Check.Problems, item.message)
			if !ok {
				t.Fatalf("return shape problem missing from %#v", body.Check.Problems)
			}
			if problem.Severity != "error" || problem.Source != "components/scalar.py" || problem.Line != item.line || problem.Column == 0 {
				t.Fatalf("return shape problem = %#v", problem)
			}
		})
	}
}

func TestCheckSourceEndpointAcceptsVariableOutputsReturnShape(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	projectSummary := createWorkspaceProject(t, server, "Source Variable Outputs Project")

	source := strings.Join([]string{
		"class ScalarComponent:",
		"    def evaluate(self, inputs, state, params, context):",
		"        outputs = {\"result\": 1}",
		"        return outputs, state",
		"",
	}, "\n")
	payload, err := json.Marshal(map[string]any{
		"project_path": projectSummary.ProjectPath,
		"component_id": "scalar",
		"content":      source,
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
		t.Fatalf("variable outputs return shape should pass: %#v", body.Check.Problems)
	}
}

func TestCheckSourceEndpointReportsImportProblem(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Source Import Project"}`)))
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
		"content":      "import definitely_missing_hvac_studio_package\n\nclass ScalarComponent:\n    def evaluate(self, inputs, state, params, context):\n        value = float(inputs.get(\"value\", 0.0))\n        return {\"result\": value}, state\n",
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
		t.Fatal("source check should fail when source imports a missing package")
	}
	if !hasProblemMessageContaining(body.Check.Problems, "source load failed:") {
		t.Fatalf("load problem missing from %#v", body.Check.Problems)
	}
	if !hasProblemMessageContaining(body.Check.Problems, "definitely_missing_hvac_studio_package") {
		t.Fatalf("missing import name was not reported in %#v", body.Check.Problems)
	}
}

func TestCheckSourceEndpointReportsUndefinedNameWarning(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Source Undefined Project"}`)))
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
		"content":      "class ScalarComponent:\n    def evaluate(self, inputs, state, params, context):\n        value = float(inputs.get(\"value\", 0.0)) * missing_factor\n        return {\"result\": value}, state\n",
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
		t.Fatalf("undefined-name hint should warn without blocking: %#v", body.Check.Problems)
	}
	var found *Problem
	for index := range body.Check.Problems {
		if strings.Contains(body.Check.Problems[index].Message, "missing_factor") {
			found = &body.Check.Problems[index]
			break
		}
	}
	if found == nil {
		t.Fatalf("undefined-name warning missing from %#v", body.Check.Problems)
	}
	if found.Severity != "warning" || found.Source != "components/scalar.py" || found.Line != 3 || found.Column == 0 {
		t.Fatalf("undefined-name problem = %#v", found)
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

func TestGeneratedWrapperComponentUsesUserStepSource(t *testing.T) {
	server := newTestServer(t)
	projectPath := filepath.Join("examples", "008_generated_wrapper_component", "project.bcsproj")

	sourceResponse := httptest.NewRecorder()
	sourceRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/project/source?project_path="+url.QueryEscape(projectPath)+"&component_id=wrapped_gain",
		nil,
	)
	server.Handler().ServeHTTP(sourceResponse, sourceRequest)
	if sourceResponse.Code != http.StatusOK {
		t.Fatalf("source status = %d body=%s", sourceResponse.Code, sourceResponse.Body.String())
	}
	var sourceBody struct {
		Source SourceDetail `json:"source"`
	}
	if err := json.Unmarshal(sourceResponse.Body.Bytes(), &sourceBody); err != nil {
		t.Fatal(err)
	}
	if sourceBody.Source.RelativePath != "components/custom_gain/user_step.py" {
		t.Fatalf("source relative path = %s", sourceBody.Source.RelativePath)
	}
	if sourceBody.Source.Layout != "generated_wrapper" || sourceBody.Source.EditableRole != "user_step" {
		t.Fatalf("source metadata = layout %q role %q", sourceBody.Source.Layout, sourceBody.Source.EditableRole)
	}
	if !sourceBody.Source.ReadOnly {
		t.Fatal("example source should be read only")
	}
	if !strings.Contains(sourceBody.Source.Content, "def step(inputs, state, params, context):") {
		t.Fatalf("source content = %s", sourceBody.Source.Content)
	}

	checkPayload, err := json.Marshal(map[string]any{
		"project_path": projectPath,
		"component_id": "wrapped_gain",
		"content":      sourceBody.Source.Content,
	})
	if err != nil {
		t.Fatal(err)
	}
	checkResponse := httptest.NewRecorder()
	checkRequest := httptest.NewRequest(http.MethodPost, "/api/project/source/check", bytes.NewReader(checkPayload))
	server.Handler().ServeHTTP(checkResponse, checkRequest)
	if checkResponse.Code != http.StatusOK {
		t.Fatalf("check status = %d body=%s", checkResponse.Code, checkResponse.Body.String())
	}
	var checkBody struct {
		Check SourceCheck `json:"check"`
	}
	if err := json.Unmarshal(checkResponse.Body.Bytes(), &checkBody); err != nil {
		t.Fatal(err)
	}
	if !checkBody.Check.OK {
		t.Fatalf("source check problems = %#v", checkBody.Check.Problems)
	}
	if checkBody.Check.ExpectedFunction != "step" {
		t.Fatalf("expected function = %s", checkBody.Check.ExpectedFunction)
	}

	badShapePayload, err := json.Marshal(map[string]any{
		"project_path": projectPath,
		"component_id": "wrapped_gain",
		"content":      "def step(inputs, state, params, context):\n    return {\"result\": 1}\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	badShapeResponse := httptest.NewRecorder()
	badShapeRequest := httptest.NewRequest(http.MethodPost, "/api/project/source/check", bytes.NewReader(badShapePayload))
	server.Handler().ServeHTTP(badShapeResponse, badShapeRequest)
	if badShapeResponse.Code != http.StatusOK {
		t.Fatalf("bad shape check status = %d body=%s", badShapeResponse.Code, badShapeResponse.Body.String())
	}
	var badShapeBody struct {
		Check SourceCheck `json:"check"`
	}
	if err := json.Unmarshal(badShapeResponse.Body.Bytes(), &badShapeBody); err != nil {
		t.Fatal(err)
	}
	if badShapeBody.Check.OK {
		t.Fatalf("generated wrapper source check should fail for return shape: %#v", badShapeBody.Check.Problems)
	}
	if !hasProblemMessage(badShapeBody.Check.Problems, "return shape must be (outputs, state)") {
		t.Fatalf("generated wrapper return shape problem missing from %#v", badShapeBody.Check.Problems)
	}

	validatePayload, err := json.Marshal(map[string]any{"project_path": projectPath})
	if err != nil {
		t.Fatal(err)
	}
	validateResponse := httptest.NewRecorder()
	validateRequest := httptest.NewRequest(http.MethodPost, "/api/validate", bytes.NewReader(validatePayload))
	server.Handler().ServeHTTP(validateResponse, validateRequest)
	if validateResponse.Code != http.StatusOK {
		t.Fatalf("validate status = %d body=%s", validateResponse.Code, validateResponse.Body.String())
	}

	runResponse := httptest.NewRecorder()
	runRequest := httptest.NewRequest(http.MethodPost, "/api/run", bytes.NewReader(validatePayload))
	server.Handler().ServeHTTP(runResponse, runRequest)
	if runResponse.Code != http.StatusOK {
		t.Fatalf("run status = %d body=%s", runResponse.Code, runResponse.Body.String())
	}
	var runBody struct {
		Result struct {
			Outputs map[string]float64            `json:"outputs"`
			States  map[string]map[string]float64 `json:"states"`
		} `json:"result"`
	}
	if err := json.Unmarshal(runResponse.Body.Bytes(), &runBody); err != nil {
		t.Fatal(err)
	}
	if runBody.Result.Outputs["result"] != 13 {
		t.Fatalf("result = %v, want 13", runBody.Result.Outputs["result"])
	}
	if runBody.Result.States["wrapped_gain"]["calls"] != 1 {
		t.Fatalf("calls state = %#v, want 1", runBody.Result.States["wrapped_gain"])
	}
}

func TestCheckSourceEndpointWarnsAboutUnreferencedContractNodes(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Source Contract Warning Project"}`)))
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
		"content":      "class ScalarComponent:\n    def evaluate(self, inputs, state, params, context):\n        return {\"other\": 1}, state\n",
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
		t.Fatalf("warnings should not fail source check = %#v", body.Check.Problems)
	}
	if !hasProblemMessage(body.Check.Problems, "required input node is not referenced in source: value") {
		t.Fatalf("input warning missing from %#v", body.Check.Problems)
	}
	if !hasProblemMessage(body.Check.Problems, "output node is not obviously returned by source: result") {
		t.Fatalf("output warning missing from %#v", body.Check.Problems)
	}
	outputProblem, ok := findProblemMessageContaining(body.Check.Problems, "output node is not obviously returned by source: result")
	if !ok {
		t.Fatalf("output problem missing from %#v", body.Check.Problems)
	}
	if outputProblem.Source != "components/scalar.py" || outputProblem.Line != 3 || outputProblem.NodeID != "result" {
		t.Fatalf("output problem location = %#v", outputProblem)
	}
}

func TestCheckSourceEndpointWarnsAboutUnknownParameterAndStateReferences(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	projectSummary := createWorkspaceProject(t, server, "Source Reference Warning Project")

	contractPayload, err := json.Marshal(map[string]any{
		"project_path": projectSummary.ProjectPath,
		"component_id": "scalar",
		"parameters": map[string]any{
			"gain": 2.0,
		},
		"parameter_defs": map[string]model.ParameterDefinition{
			"gain": {
				DisplayName: "Gain",
				Current:     2.0,
			},
		},
		"state_defs": map[string]model.StateDefinition{
			"calls": {
				DisplayName: "Calls",
				Initial:     0.0,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	contractResponse := httptest.NewRecorder()
	contractRequest := httptest.NewRequest(http.MethodPost, "/api/project/component-contract", bytes.NewReader(contractPayload))
	server.Handler().ServeHTTP(contractResponse, contractRequest)
	if contractResponse.Code != http.StatusOK {
		t.Fatalf("contract status = %d body=%s", contractResponse.Code, contractResponse.Body.String())
	}

	source := strings.Join([]string{
		"class ScalarComponent:",
		"    def evaluate(self, inputs, state, params, context):",
		"        value = inputs.get(\"value\", 0.0)",
		"        gain = params[\"gain\"]",
		"        scale = params.get(\"scale\", 1.0)",
		"        calls = state['calls']",
		"        skipped = state.get('skipped', 0.0)",
		"        local_params = {\"shadow\": 1.0}",
		"        shadow = local_params.get(\"shadow\", 1.0)",
		"        previous_state = {\"ignored\": 0.0}",
		"        ignored = previous_state.get(\"ignored\", 0.0)",
		"        return {\"result\": value * gain * scale}, {\"calls\": calls + skipped}",
		"",
	}, "\n")
	payload, err := json.Marshal(map[string]any{
		"project_path": projectSummary.ProjectPath,
		"component_id": "scalar",
		"content":      source,
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
		t.Fatalf("warnings should not fail source check = %#v", body.Check.Problems)
	}
	parameterProblem, ok := findProblemMessageContaining(body.Check.Problems, "parameter reference is not in component contract: scale")
	if !ok {
		t.Fatalf("parameter warning missing from %#v", body.Check.Problems)
	}
	if parameterProblem.Source != "components/scalar.py" || parameterProblem.Line != 5 || parameterProblem.Column == 0 {
		t.Fatalf("parameter problem location = %#v", parameterProblem)
	}
	stateProblem, ok := findProblemMessageContaining(body.Check.Problems, "state reference is not in component contract: skipped")
	if !ok {
		t.Fatalf("state warning missing from %#v", body.Check.Problems)
	}
	if stateProblem.Source != "components/scalar.py" || stateProblem.Line != 7 || stateProblem.Column == 0 {
		t.Fatalf("state problem location = %#v", stateProblem)
	}
	if hasProblemMessageContaining(body.Check.Problems, "parameter reference is not in component contract: gain") {
		t.Fatalf("known parameter should not warn: %#v", body.Check.Problems)
	}
	if hasProblemMessageContaining(body.Check.Problems, "state reference is not in component contract: calls") {
		t.Fatalf("known state should not warn: %#v", body.Check.Problems)
	}
	if hasProblemMessageContaining(body.Check.Problems, "parameter reference is not in component contract: shadow") {
		t.Fatalf("local params-like variable should not warn: %#v", body.Check.Problems)
	}
	if hasProblemMessageContaining(body.Check.Problems, "state reference is not in component contract: ignored") {
		t.Fatalf("local state-like variable should not warn: %#v", body.Check.Problems)
	}
}

func TestCheckSourceEndpointWarnsAboutUnknownInputAndOutputReferences(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	projectSummary := createWorkspaceProject(t, server, "Source IO Reference Warning Project")

	source := strings.Join([]string{
		"class ScalarComponent:",
		"    def evaluate(self, inputs, state, params, context):",
		"        value = inputs.get(\"valeu\", 0.0)",
		"        return {\"reslt\": value}, {\"calls\": 1.0}",
		"",
	}, "\n")
	payload, err := json.Marshal(map[string]any{
		"project_path": projectSummary.ProjectPath,
		"component_id": "scalar",
		"content":      source,
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
		t.Fatalf("warnings should not fail source check = %#v", body.Check.Problems)
	}
	inputProblem, ok := findProblemMessageContaining(body.Check.Problems, "input node reference is not in component contract: valeu")
	if !ok {
		t.Fatalf("input warning missing from %#v", body.Check.Problems)
	}
	if inputProblem.Source != "components/scalar.py" || inputProblem.Line != 3 || inputProblem.Column == 0 {
		t.Fatalf("input problem location = %#v", inputProblem)
	}
	outputProblem, ok := findProblemMessageContaining(body.Check.Problems, "output node reference is not in component contract: reslt")
	if !ok {
		t.Fatalf("output warning missing from %#v", body.Check.Problems)
	}
	if outputProblem.Source != "components/scalar.py" || outputProblem.Line != 4 || outputProblem.Column == 0 {
		t.Fatalf("output problem location = %#v", outputProblem)
	}
	if hasProblemMessageContaining(body.Check.Problems, "output node reference is not in component contract: calls") {
		t.Fatalf("state return dictionary should not be treated as output keys: %#v", body.Check.Problems)
	}
}

func TestCheckSourceEndpointAcceptsInitializedStateAndFileReference(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	projectSummary := createWorkspaceProject(t, server, "Source Initialized State Project")

	source := strings.Join([]string{
		"from pathlib import Path",
		"",
		"class ScalarComponent:",
		"    def initialize(self, params, context):",
		"        root = Path(__file__).resolve().parent",
		"        return {\"model\": str(root)}",
		"",
		"    def evaluate(self, inputs, state, params, context):",
		"        value = float(inputs.get(\"value\", 0.0))",
		"        model = state[\"model\"]",
		"        return {\"result\": value + len(model)}, state",
		"",
	}, "\n")
	payload, err := json.Marshal(map[string]any{
		"project_path": projectSummary.ProjectPath,
		"component_id": "scalar",
		"content":      source,
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
		t.Fatalf("initialized state source should pass: %#v", body.Check.Problems)
	}
	if hasProblemMessageContaining(body.Check.Problems, "output node reference is not in component contract: model") {
		t.Fatalf("initialize state return should not be treated as output keys: %#v", body.Check.Problems)
	}
	if hasProblemMessageContaining(body.Check.Problems, "state reference is not in component contract: model") {
		t.Fatalf("initialize state key should be accepted: %#v", body.Check.Problems)
	}
	if hasProblemMessageContaining(body.Check.Problems, "undefined name may fail at runtime: __file__") {
		t.Fatalf("__file__ should be allowed in source checks: %#v", body.Check.Problems)
	}
}

func TestSourceReturnOutputKeyReferencesOnlyReadsReturnedOutputDict(t *testing.T) {
	content := "def step(inputs, state, params, context):\n    return value, state\nCONFIG = {\"reslt\": 1}\n"
	start, end := sourceTargetFunctionBodyRange(content, "step", false)
	refs := sourceReturnOutputKeyReferencesInRange(content, start, end)
	if len(refs) != 0 {
		t.Fatalf("non-dict return references = %#v, want none", refs)
	}

	content = strings.Join([]string{
		"def step(inputs, state, params, context):",
		"    return ({",
		"        \"reslt\": value,",
		"    }, {\"calls\": 1.0})",
		"",
	}, "\n")
	start, end = sourceTargetFunctionBodyRange(content, "step", false)
	refs = sourceReturnOutputKeyReferencesInRange(content, start, end)
	if len(refs) != 1 || refs[0].Name != "reslt" || refs[0].Line != 3 {
		t.Fatalf("returned output dict references = %#v, want reslt on line 3", refs)
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
