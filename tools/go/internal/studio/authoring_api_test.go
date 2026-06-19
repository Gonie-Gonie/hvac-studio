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

	"github.com/goniegonie/hvac-studio/tools/go/internal/model"
	"github.com/goniegonie/hvac-studio/tools/go/internal/project"
	runtimecore "github.com/goniegonie/hvac-studio/tools/go/internal/runtime"
)

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
	if componentBody.Component.Category != "utility" || componentBody.Component.ExecutionMode != "step" {
		t.Fatalf("component authoring metadata = %#v", componentBody.Component)
	}
	if componentBody.Component.Source.Layout != "generated_wrapper" ||
		componentBody.Component.Source.Metadata != "components/second_gain/component.json" ||
		componentBody.Component.Source.Step != "components/second_gain/user_step.py" ||
		componentBody.Component.Source.Wrapper != "components/second_gain/wrapper.py" {
		t.Fatalf("component source metadata = %#v", componentBody.Component.Source)
	}
	if componentBody.Component.Class != "components.second_gain.wrapper.SecondGainComponent" {
		t.Fatalf("component class = %s", componentBody.Component.Class)
	}
	if len(componentBody.Component.Nodes.Inputs) != 1 || componentBody.Component.Nodes.Inputs[0].Preset != "scalar_input" {
		t.Fatalf("input node metadata = %#v", componentBody.Component.Nodes.Inputs)
	}
	if len(componentBody.Component.Nodes.Outputs) != 1 || componentBody.Component.Nodes.Outputs[0].Preset != "scalar_output" {
		t.Fatalf("output node metadata = %#v", componentBody.Component.Nodes.Outputs)
	}
	gainDefinition, ok := componentBody.Component.ParameterDefinitions["gain"]
	if !ok {
		t.Fatalf("parameter definitions = %#v", componentBody.Component.ParameterDefinitions)
	}
	if gainDefinition.DisplayName != "Gain" || gainDefinition.Unit != "ratio" || gainDefinition.Role != "calibration_target" || gainDefinition.Group != "Model" {
		t.Fatalf("gain definition = %#v", gainDefinition)
	}
	if gainDefinition.Default != 2.0 || gainDefinition.Current != 2.0 {
		t.Fatalf("gain values = default %v current %v", gainDefinition.Default, gainDefinition.Current)
	}
	if gainDefinition.Bounds == nil || gainDefinition.Bounds.Min != 0.0 || gainDefinition.Bounds.Max != 100.0 {
		t.Fatalf("gain bounds = %#v", gainDefinition.Bounds)
	}
	sourcePath := filepath.Join(root, "projects", "component-project", "components", "second_gain", "user_step.py")
	sourceBytes, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(sourceBytes), `inputs["value"]`) {
		t.Fatalf("component source did not read the scalar input:\n%s", string(sourceBytes))
	}
	if !strings.Contains(string(sourceBytes), `params.get("gain", 2.0)`) {
		t.Fatalf("component source did not come from scalar template:\n%s", string(sourceBytes))
	}
	wrapperBytes, err := os.ReadFile(filepath.Join(root, "projects", "component-project", "components", "second_gain", "wrapper.py"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(wrapperBytes), "class SecondGainComponent:") {
		t.Fatalf("component wrapper did not use generated class name:\n%s", string(wrapperBytes))
	}
	wrapperContent := string(wrapperBytes)
	for _, want := range []string{
		"input_nodes = json.loads",
		`\"value\"`,
		"output_nodes = json.loads",
		`\"result\"`,
		"parameter_schema = json.loads",
		`\"gain\"`,
		"Studio-owned runtime wrapper",
	} {
		if !strings.Contains(wrapperContent, want) {
			t.Fatalf("component wrapper did not include regenerated contract %q:\n%s", want, wrapperContent)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "projects", "component-project", "components", "second_gain", "component.json")); err != nil {
		t.Fatal(err)
	}
	loaded, err := project.Load(createBody.Project.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	var persisted model.Component
	found := false
	for _, component := range loaded.Graph.Components {
		if component.ID == "second_gain" {
			persisted = component
			found = true
		}
	}
	if !found {
		t.Fatal("created component was not written to graph")
	}
	if persisted.Category != "utility" || persisted.ExecutionMode != "step" {
		t.Fatalf("persisted component authoring metadata = %#v", persisted)
	}
	if persisted.Source.Step != "components/second_gain/user_step.py" || persisted.Source.Wrapper != "components/second_gain/wrapper.py" {
		t.Fatalf("persisted source metadata = %#v", persisted.Source)
	}
	if _, ok := persisted.ParameterDefinitions["gain"]; !ok {
		t.Fatalf("persisted parameter definitions = %#v", persisted.ParameterDefinitions)
	}
	if got := componentBody.Component.Parameters["gain"]; got != 2.0 {
		t.Fatalf("created gain = %v, want 2", got)
	}
	for _, componentID := range loaded.Graph.Systems[0].Components {
		if componentID == "second_gain" {
			t.Fatal("new component should not be added to the runnable system yet")
		}
	}
}

func TestCreateComponentEndpointSuffixesDuplicateComponentID(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Duplicate Component ID Project"}`)))
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

	createComponentNamed := func(name string) model.Component {
		t.Helper()
		payload, err := json.Marshal(map[string]any{
			"project_path": createBody.Project.ProjectPath,
			"name":         name,
			"template":     "scalar",
		})
		if err != nil {
			t.Fatal(err)
		}
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/project/components", bytes.NewReader(payload))
		server.Handler().ServeHTTP(response, request)
		if response.Code != http.StatusCreated {
			t.Fatalf("component status = %d body=%s", response.Code, response.Body.String())
		}
		var body struct {
			Component model.Component `json:"component"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		return body.Component
	}

	first := createComponentNamed("Second Gain")
	second := createComponentNamed("Second Gain")

	if first.ID != "second_gain" {
		t.Fatalf("first component id = %s, want second_gain", first.ID)
	}
	if second.ID != "second_gain_2" {
		t.Fatalf("second component id = %s, want second_gain_2", second.ID)
	}
	if second.Name != "Second Gain" {
		t.Fatalf("second component name = %s, want Second Gain", second.Name)
	}
	if second.Source.Metadata != "components/second_gain_2/component.json" ||
		second.Source.Step != "components/second_gain_2/user_step.py" ||
		second.Source.Wrapper != "components/second_gain_2/wrapper.py" {
		t.Fatalf("second component source metadata = %#v", second.Source)
	}
	if second.Class != "components.second_gain_2.wrapper.SecondGain2Component" {
		t.Fatalf("second component class = %s", second.Class)
	}
	for _, rel := range []string{
		"components/second_gain/user_step.py",
		"components/second_gain/component.json",
		"components/second_gain_2/user_step.py",
		"components/second_gain_2/component.json",
	} {
		if _, err := os.Stat(filepath.Join(root, "projects", "duplicate-component-id-project", filepath.FromSlash(rel))); err != nil {
			t.Fatalf("expected component artifact %s: %v", rel, err)
		}
	}
	loaded, err := project.Load(createBody.Project.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, found := findComponent(loaded.Graph, "second_gain"); !found {
		t.Fatalf("first component missing from graph: %#v", loaded.Graph.Components)
	}
	if _, found := findComponent(loaded.Graph, "second_gain_2"); !found {
		t.Fatalf("suffixed component missing from graph: %#v", loaded.Graph.Components)
	}
}

func TestCreateComponentEndpointCreatesMLComponentAssets(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"ML Component Project"}`)))
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
		"name":         "ML Inference",
		"template":     "ml_inference",
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
	component := componentBody.Component
	if component.ID != "ml_inference" {
		t.Fatalf("component id = %s, want ml_inference", component.ID)
	}
	if component.Category != "physical_component" || component.ExecutionMode != "step" {
		t.Fatalf("component authoring metadata = %#v", component)
	}
	if component.Source.Layout != "generated_wrapper" ||
		component.Source.Metadata != "components/ml_inference/component.json" ||
		component.Source.Init != "components/ml_inference/user_init.py" ||
		component.Source.Step != "components/ml_inference/user_step.py" ||
		component.Source.Helpers != "components/ml_inference/helpers.py" ||
		component.Source.Wrapper != "components/ml_inference/wrapper.py" {
		t.Fatalf("component source metadata = %#v", component.Source)
	}
	if component.Class != "components.ml_inference.wrapper.MlInferenceComponent" {
		t.Fatalf("component class = %s", component.Class)
	}
	if len(component.Nodes.Inputs) != 1 || component.Nodes.Inputs[0].ID != "features" || component.Nodes.Inputs[0].ValueType != "object" {
		t.Fatalf("input nodes = %#v", component.Nodes.Inputs)
	}
	if len(component.Nodes.Outputs) != 2 || component.Nodes.Outputs[0].ID != "supply_air_temperature_c" || component.Nodes.Outputs[1].ID != "cooling_power_kw" {
		t.Fatalf("output nodes = %#v", component.Nodes.Outputs)
	}
	if component.MLMetadata == nil {
		t.Fatal("ML metadata was not created")
	}
	if component.MLMetadata.ModelFormat != "custom" ||
		component.MLMetadata.ModelFile != "components/ml_inference/model.json" ||
		component.MLMetadata.FeatureSchemaFile != "components/ml_inference/feature_schema.json" ||
		component.MLMetadata.TargetSchemaFile != "components/ml_inference/target_schema.json" ||
		component.MLMetadata.ValidationReportFile != "components/ml_inference/validation_report.json" ||
		component.MLMetadata.ValidTimeResolution != "step" {
		t.Fatalf("ML metadata = %#v", component.MLMetadata)
	}
	if component.MLMetadata.ValidInputRanges["outdoor_temperature_c"].Min != -20.0 ||
		component.MLMetadata.ValidInputRanges["fan_speed_fraction"].Max != 1.0 {
		t.Fatalf("ML valid input ranges = %#v", component.MLMetadata.ValidInputRanges)
	}
	componentRoot := filepath.Join(root, "projects", "ml-component-project", "components", "ml_inference")
	for _, name := range []string{
		"component.json",
		"wrapper.py",
		"user_init.py",
		"user_step.py",
		"helpers.py",
		"model.json",
		"feature_schema.json",
		"target_schema.json",
		"validation_report.json",
	} {
		if _, err := os.Stat(filepath.Join(componentRoot, name)); err != nil {
			t.Fatalf("expected ML component file %s: %v", name, err)
		}
	}
	userStepBytes, err := os.ReadFile(filepath.Join(componentRoot, "user_step.py"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(userStepBytes, []byte("evaluate_model(state, inputs, bias)")) {
		t.Fatalf("ML user step did not delegate to the cached model pipeline:\n%s", string(userStepBytes))
	}
	helperBytes, err := os.ReadFile(filepath.Join(componentRoot, "helpers.py"))
	if err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{
		"def load_model_assets",
		"def extract_features",
		"def preprocess_features",
		"def run_inference",
		"def postprocess_outputs",
		"missing ML feature",
	} {
		if !bytes.Contains(helperBytes, []byte(token)) {
			t.Fatalf("ML helper did not include %s:\n%s", token, string(helperBytes))
		}
	}
	metadataBytes, err := os.ReadFile(filepath.Join(componentRoot, "component.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(metadataBytes, []byte(`"ml_metadata"`)) ||
		!bytes.Contains(metadataBytes, []byte(`"components/ml_inference/model.json"`)) {
		t.Fatalf("component metadata did not include rebased ML asset paths:\n%s", string(metadataBytes))
	}
	loaded, err := project.Load(createBody.Project.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	var persisted model.Component
	found := false
	for _, item := range loaded.Graph.Components {
		if item.ID == "ml_inference" {
			persisted = item
			found = true
		}
	}
	if !found {
		t.Fatal("created ML component was not written to graph")
	}
	if persisted.MLMetadata == nil || persisted.MLMetadata.ModelFile != "components/ml_inference/model.json" {
		t.Fatalf("persisted ML metadata = %#v", persisted.MLMetadata)
	}
}

func TestUpdateComponentMLAssetsEndpointImportsFilesAndMetadata(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"ML Asset Import Project"}`)))
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

	createComponentPayload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"name":         "ML Inference",
		"template":     "ml_inference",
	})
	if err != nil {
		t.Fatal(err)
	}
	componentResponse := httptest.NewRecorder()
	componentRequest := httptest.NewRequest(http.MethodPost, "/api/project/components", bytes.NewReader(createComponentPayload))
	server.Handler().ServeHTTP(componentResponse, componentRequest)
	if componentResponse.Code != http.StatusCreated {
		t.Fatalf("component status = %d body=%s", componentResponse.Code, componentResponse.Body.String())
	}

	importPayload, err := json.Marshal(map[string]any{
		"project_path":          createBody.Project.ProjectPath,
		"component_id":          "ml_inference",
		"model_format":          "onnx",
		"required_packages":     []string{"onnxruntime", "numpy", "onnxruntime", " "},
		"valid_time_resolution": "step",
		"valid_input_ranges": map[string]map[string]float64{
			"temperature_c": {"min": -10, "max": 50},
		},
		"assets": []map[string]any{
			{"field": "model_file", "file_name": "uploaded_model.onnx", "content_base64": "AQIDBA=="},
			{"field": "input_scaler_file", "file_name": "input_scaler.json", "content": `{"mean":[1.0],"scale":[2.0]}`},
			{"field": "feature_schema_file", "file_name": "uploaded_features.json", "content": `{"features":["temperature_c"]}`},
			{"field": "target_schema_file", "file_name": "uploaded_targets.json", "content": `{"targets":[{"id":"fan_power_kw","name":"Fan Power","unit":"kW"}]}`},
			{"field": "training_metadata_file", "file_name": "training_metadata.json", "content": `{"dataset":"train.csv"}`},
			{"field": "validation_report_file", "file_name": "uploaded_validation.json", "content": `{"metrics":{"rmse":1.25}}`},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	importResponse := httptest.NewRecorder()
	importRequest := httptest.NewRequest(http.MethodPost, "/api/project/components/ml-assets", bytes.NewReader(importPayload))

	server.Handler().ServeHTTP(importResponse, importRequest)

	if importResponse.Code != http.StatusOK {
		t.Fatalf("import status = %d body=%s", importResponse.Code, importResponse.Body.String())
	}
	var importBody struct {
		Component     model.Component `json:"component"`
		ImportedFiles []string        `json:"imported_files"`
	}
	if err := json.Unmarshal(importResponse.Body.Bytes(), &importBody); err != nil {
		t.Fatal(err)
	}
	metadata := importBody.Component.MLMetadata
	if metadata == nil {
		t.Fatal("ML metadata was not returned")
	}
	if metadata.ModelFormat != "onnx" ||
		metadata.ModelFile != "components/ml_inference/uploaded_model.onnx" ||
		metadata.InputScalerFile != "components/ml_inference/input_scaler.json" ||
		metadata.FeatureSchemaFile != "components/ml_inference/uploaded_features.json" ||
		metadata.TargetSchemaFile != "components/ml_inference/uploaded_targets.json" ||
		metadata.TrainingMetadataFile != "components/ml_inference/training_metadata.json" ||
		metadata.ValidationReportFile != "components/ml_inference/uploaded_validation.json" {
		t.Fatalf("ML metadata = %#v", metadata)
	}
	if len(metadata.RequiredPackages) != 2 || metadata.RequiredPackages[0] != "onnxruntime" || metadata.RequiredPackages[1] != "numpy" {
		t.Fatalf("required packages = %#v", metadata.RequiredPackages)
	}
	bounds, ok := metadata.ValidInputRanges["temperature_c"]
	if !ok || bounds.Min != float64(-10) || bounds.Max != float64(50) {
		t.Fatalf("valid input ranges = %#v", metadata.ValidInputRanges)
	}
	for _, rel := range []string{
		"components/ml_inference/uploaded_model.onnx",
		"components/ml_inference/input_scaler.json",
		"components/ml_inference/uploaded_features.json",
		"components/ml_inference/uploaded_targets.json",
		"components/ml_inference/training_metadata.json",
		"components/ml_inference/uploaded_validation.json",
	} {
		if !containsString(importBody.ImportedFiles, rel) {
			t.Fatalf("imported files missing %s in %v", rel, importBody.ImportedFiles)
		}
		if _, err := os.Stat(filepath.Join(root, "projects", "ml-asset-import-project", filepath.FromSlash(rel))); err != nil {
			t.Fatalf("imported file %s: %v", rel, err)
		}
	}
	modelBytes, err := os.ReadFile(filepath.Join(root, "projects", "ml-asset-import-project", "components", "ml_inference", "uploaded_model.onnx"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(modelBytes, []byte{1, 2, 3, 4}) {
		t.Fatalf("model bytes = %v", modelBytes)
	}
	metadataBytes, err := os.ReadFile(filepath.Join(root, "projects", "ml-asset-import-project", "components", "ml_inference", "component.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(metadataBytes, []byte(`"input_scaler_file": "components/ml_inference/input_scaler.json"`)) ||
		!bytes.Contains(metadataBytes, []byte(`"required_packages": [`)) {
		t.Fatalf("component metadata was not synced:\n%s", string(metadataBytes))
	}
	loaded, err := project.Load(createBody.Project.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	persisted, found := findComponent(loaded.Graph, "ml_inference")
	if !found || persisted.MLMetadata == nil || persisted.MLMetadata.ModelFile != "components/ml_inference/uploaded_model.onnx" {
		t.Fatalf("persisted ML metadata = %#v found=%v", persisted.MLMetadata, found)
	}

	badFormatPayload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"component_id": "ml_inference",
		"model_format": "custom-linear",
		"assets":       []map[string]any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	badFormatResponse := httptest.NewRecorder()
	badFormatRequest := httptest.NewRequest(http.MethodPost, "/api/project/components/ml-assets", bytes.NewReader(badFormatPayload))
	server.Handler().ServeHTTP(badFormatResponse, badFormatRequest)
	if badFormatResponse.Code == http.StatusOK || !strings.Contains(badFormatResponse.Body.String(), "ML model_format is unsupported: custom-linear") {
		t.Fatalf("bad format status = %d body=%s", badFormatResponse.Code, badFormatResponse.Body.String())
	}

	badRangePayload, err := json.Marshal(map[string]any{
		"project_path":          createBody.Project.ProjectPath,
		"component_id":          "ml_inference",
		"valid_time_resolution": "step",
		"valid_input_ranges": map[string]map[string]float64{
			"temperature_c": {"min": 50, "max": -10},
		},
		"assets": []map[string]any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	badRangeResponse := httptest.NewRecorder()
	badRangeRequest := httptest.NewRequest(http.MethodPost, "/api/project/components/ml-assets", bytes.NewReader(badRangePayload))
	server.Handler().ServeHTTP(badRangeResponse, badRangeRequest)
	if badRangeResponse.Code == http.StatusOK ||
		!strings.Contains(badRangeResponse.Body.String(), "ML valid input range min must be") ||
		!strings.Contains(badRangeResponse.Body.String(), "temperature_c") {
		t.Fatalf("bad range status = %d body=%s", badRangeResponse.Code, badRangeResponse.Body.String())
	}
	loadedAfterBadRange, err := project.Load(createBody.Project.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	afterBadRange, found := findComponent(loadedAfterBadRange.Graph, "ml_inference")
	if !found || afterBadRange.MLMetadata == nil {
		t.Fatalf("component after bad range = %#v found=%v", afterBadRange, found)
	}
	if afterBadRange.MLMetadata.ModelFormat != "onnx" {
		t.Fatalf("bad requests mutated model format = %#v", afterBadRange.MLMetadata)
	}
	if bounds := afterBadRange.MLMetadata.ValidInputRanges["temperature_c"]; bounds.Min != float64(-10) || bounds.Max != float64(50) {
		t.Fatalf("bad range request mutated persisted metadata = %#v", afterBadRange.MLMetadata.ValidInputRanges)
	}

	applyPayload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"component_id": "ml_inference",
	})
	if err != nil {
		t.Fatal(err)
	}
	applyResponse := httptest.NewRecorder()
	applyRequest := httptest.NewRequest(http.MethodPost, "/api/project/components/ml-schema-nodes", bytes.NewReader(applyPayload))
	server.Handler().ServeHTTP(applyResponse, applyRequest)
	if applyResponse.Code != http.StatusOK {
		t.Fatalf("schema node status = %d body=%s", applyResponse.Code, applyResponse.Body.String())
	}
	var applyBody struct {
		Component model.Component          `json:"component"`
		Summary   MLSchemaNodeApplySummary `json:"summary"`
	}
	if err := json.Unmarshal(applyResponse.Body.Bytes(), &applyBody); err != nil {
		t.Fatal(err)
	}
	if applyBody.Summary.FeatureCount != 1 ||
		applyBody.Summary.TargetCount != 1 ||
		!containsString(applyBody.Summary.ExistingInputs, "features") ||
		!containsString(applyBody.Summary.AddedOutputs, "fan_power_kw") {
		t.Fatalf("schema node summary = %#v", applyBody.Summary)
	}
	if !componentHasOutputNode(applyBody.Component, "fan_power_kw") {
		t.Fatalf("schema output node was not created: %#v", applyBody.Component.Nodes.Outputs)
	}

	badPayload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"component_id": "ml_inference",
		"assets": []map[string]any{
			{"field": "model_file", "file_name": "../escape.onnx", "content": "bad"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	badResponse := httptest.NewRecorder()
	badRequest := httptest.NewRequest(http.MethodPost, "/api/project/components/ml-assets", bytes.NewReader(badPayload))
	server.Handler().ServeHTTP(badResponse, badRequest)
	if badResponse.Code == http.StatusOK {
		t.Fatalf("escaping file name should be rejected: %s", badResponse.Body.String())
	}
	if _, err := os.Stat(filepath.Join(root, "projects", "ml-asset-import-project", "escape.onnx")); !os.IsNotExist(err) {
		t.Fatalf("escaping asset write state = %v", err)
	}
}

func TestCreateComponentEndpointCanIncludeComponentInSystem(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Included Component Project"}`)))
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
		"project_path":      createBody.Project.ProjectPath,
		"name":              "Second Gain",
		"template":          "scalar",
		"include_in_system": true,
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
	loaded, err := project.Load(createBody.Project.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	system := loaded.Graph.Systems[0]
	if !containsString(system.Components, "second_gain") {
		t.Fatalf("system components = %v", system.Components)
	}
	if !hasPublicInputFor(system, "second_gain", "value") {
		t.Fatalf("system public inputs = %#v", system.PublicInputs)
	}
	if !hasPublicOutputFor(system, "second_gain", "result") {
		t.Fatalf("system public outputs = %#v", system.PublicOutputs)
	}
	input, err := runtimecore.LoadInput(filepath.Join(root, "projects", "included-component-project", filepath.FromSlash(loaded.Project.DefaultInput)))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := input.Inputs["second_gain_value"]; !ok {
		t.Fatalf("default inputs = %#v", input.Inputs)
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

func TestReplaceComponentEndpointCreatesReplacementAndRewiresSystem(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Replacement Component Project"}`)))
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
	for index := range loaded.Graph.Components {
		if loaded.Graph.Components[index].ID == "scalar" {
			loaded.Graph.Components[index].Parameters["gain"] = 4.25
		}
	}
	if err := writeJSONFile(loaded.GraphPath, loaded.Graph); err != nil {
		t.Fatal(err)
	}

	payload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"component_id": "scalar",
		"name":         "Scalar Replacement",
		"template":     "scalar",
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/components/replace", bytes.NewReader(payload))
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("replace status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Component   model.Component             `json:"component"`
		Replacement ComponentReplacementSummary `json:"replacement"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Component.ID != "scalar_replacement" {
		t.Fatalf("replacement id = %s", body.Component.ID)
	}
	if !body.Replacement.SystemReplaced || body.Replacement.RewiredPublicInputs != 1 || body.Replacement.RewiredPublicOutputs != 1 {
		t.Fatalf("replacement summary = %#v", body.Replacement)
	}
	if !body.Replacement.MapParameters || body.Replacement.MappedParameters != 1 ||
		len(body.Replacement.NodeMappings) < 2 || len(body.Replacement.ParameterMappings) != 1 ||
		len(body.Replacement.Diff.MatchedInputs) != 1 || len(body.Replacement.Diff.MatchedOutputs) != 1 ||
		len(body.Replacement.Problems) != 0 {
		t.Fatalf("replacement mapping summary = %#v", body.Replacement)
	}
	if !body.Replacement.OriginalComponentRetained {
		t.Fatalf("original component should be retained: %#v", body.Replacement)
	}
	loaded, err = project.Load(createBody.Project.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, found := findComponent(loaded.Graph, "scalar"); !found {
		t.Fatal("original component should remain in graph")
	}
	if _, found := findComponent(loaded.Graph, "scalar_replacement"); !found {
		t.Fatal("replacement component was not written to graph")
	}
	system := loaded.Graph.Systems[0]
	if containsString(system.Components, "scalar") || !containsString(system.Components, "scalar_replacement") {
		t.Fatalf("system components = %v", system.Components)
	}
	if !hasPublicInputFor(system, "scalar_replacement", "value") || !hasPublicOutputFor(system, "scalar_replacement", "result") {
		t.Fatalf("system public IO = inputs %#v outputs %#v", system.PublicInputs, system.PublicOutputs)
	}
	replacement, found := findComponent(loaded.Graph, "scalar_replacement")
	if !found {
		t.Fatal("replacement missing from loaded graph")
	}
	if got := replacement.Parameters["gain"]; got != 4.25 {
		t.Fatalf("replacement gain = %v, want copied source gain", got)
	}
	if _, err := os.Stat(filepath.Join(root, "projects", "replacement-component-project", "components", "scalar_replacement", "user_step.py")); err != nil {
		t.Fatal(err)
	}
}

func TestReplaceComponentEndpointReportsBrokenMapping(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Broken Replacement Project"}`)))
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
		"name":         "Scalar Replacement",
		"template":     "data_sink",
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/components/replace", bytes.NewReader(payload))
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("replace status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Problems []Problem `json:"problems"`
		Message  string    `json:"message"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Problems) == 0 || !strings.Contains(body.Problems[0].Message, "replacement missing output node") {
		t.Fatalf("replacement problems = %#v message=%s", body.Problems, body.Message)
	}
	loaded, err := project.Load(createBody.Project.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, found := findComponent(loaded.Graph, "scalar_replacement"); found {
		t.Fatal("incompatible replacement should have been rolled back")
	}
	if _, err := os.Stat(filepath.Join(root, "projects", "broken-replacement-project", "components", "scalar_replacement")); !os.IsNotExist(err) {
		t.Fatalf("replacement source rollback err = %v", err)
	}
}

func TestReplaceZoneLoadRCWithANNTemplateRunsExample(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	projectRoot := filepath.Join(root, "projects", "zone-load-ann-replacement")
	if err := copyProjectTree(filepath.Join(repoRoot, "examples", "015_rc_ahu_ann_composition"), projectRoot); err != nil {
		t.Fatal(err)
	}
	projectPath := filepath.Join(projectRoot, "project.bcsproj")

	payload, err := json.Marshal(map[string]any{
		"project_path":   projectPath,
		"component_id":   "zone_rc",
		"name":           "Zone Load ANN",
		"template":       "zone_load_ann",
		"map_parameters": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/components/replace", bytes.NewReader(payload))
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("replace status = %d body=%s", response.Code, response.Body.String())
	}
	var replaceBody struct {
		Component   model.Component             `json:"component"`
		Replacement ComponentReplacementSummary `json:"replacement"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &replaceBody); err != nil {
		t.Fatal(err)
	}
	if replaceBody.Component.ID != "zone_load_ann" ||
		!replaceBody.Replacement.SystemReplaced ||
		len(replaceBody.Replacement.Diff.MissingInputs) != 0 ||
		len(replaceBody.Replacement.Diff.MissingOutputs) != 0 ||
		len(replaceBody.Replacement.Problems) != 0 {
		t.Fatalf("zone replacement summary = %#v component=%#v", replaceBody.Replacement, replaceBody.Component)
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
		Result runtimecore.RunResult `json:"result"`
	}
	if err := json.Unmarshal(runResponse.Body.Bytes(), &runBody); err != nil {
		t.Fatal(err)
	}
	if _, ok := runBody.Result.Outputs["total_power_kw"]; !ok {
		t.Fatalf("replacement run outputs = %#v", runBody.Result.Outputs)
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
	sourcePath := filepath.Join(root, "projects", "delete-component-project", "components", "scratch_gain")
	if info, err := os.Stat(sourcePath); err != nil || !info.IsDir() {
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
		"preset":       "control_signal_input",
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
	inputNode, foundInputNode := findInputNode(component, "bias")
	if !foundInputNode || inputNode.Preset != "control_signal_input" {
		t.Fatalf("input node preset = %#v", inputNode)
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

func TestUpdateNodeEndpointUpdatesPublicIOAndDefaultInput(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	projectSummary := createWorkspaceProject(t, server, "Update Node Project")
	required := false
	payload, err := json.Marshal(map[string]any{
		"project_path": projectSummary.ProjectPath,
		"component_id": "scalar",
		"node_id":      "value",
		"name":         "Room temperature",
		"medium":       "air",
		"value_type":   "int",
		"unit":         "C",
		"required":     required,
		"default":      21,
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/nodes/update", bytes.NewReader(payload))
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}

	loaded, err := project.Load(projectSummary.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	node, found := findInputNode(loaded.Graph.Components[0], "value")
	if !found {
		t.Fatal("value node not found")
	}
	if node.Name != "Room temperature" || node.Medium != "air" || node.ValueType != "int" || node.Unit != "C" {
		t.Fatalf("node metadata = %#v", node)
	}
	if node.Required == nil || *node.Required {
		t.Fatalf("node required = %#v, want false", node.Required)
	}
	if got := node.Default; got != float64(21) {
		t.Fatalf("node default = %#v, want 21", got)
	}
	publicInput := loaded.Graph.Systems[0].PublicInputs[0]
	if publicInput.Name != node.Name || publicInput.Medium != node.Medium || publicInput.ValueType != node.ValueType || publicInput.Unit != node.Unit {
		t.Fatalf("public input metadata = %#v, want node metadata", publicInput)
	}
	if publicInput.Required == nil || *publicInput.Required {
		t.Fatalf("public input required = %#v, want false", publicInput.Required)
	}
	input, err := runtimecore.LoadInput(filepath.Join(loaded.Root, loaded.Project.DefaultInput))
	if err != nil {
		t.Fatal(err)
	}
	if got := input.Inputs["value"]; got != float64(21) {
		t.Fatalf("value default input = %#v, want 21", got)
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

func TestUpdateNodeEndpointRejectsExamples(t *testing.T) {
	server := newTestServer(t)
	payload := []byte(`{
		"project_path": "examples/001_scalar_component/project.bcsproj",
		"component_id": "scalar",
		"node_id": "value",
		"name": "Edited"
	}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/nodes/update", bytes.NewReader(payload))

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
	if runBody.Result.Outputs["second_gain_result"] != 16 {
		t.Fatalf("second_gain_result = %v, want 16", runBody.Result.Outputs["second_gain_result"])
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

func TestUpdateConnectionUnitConversionEndpointWritesGraph(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	summary := createWorkspaceProject(t, server, "Connection Conversion Project")

	componentPayload, err := json.Marshal(map[string]any{
		"project_path": summary.ProjectPath,
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
		"project_path": summary.ProjectPath,
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

	loaded, err := project.Load(summary.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	for componentIndex := range loaded.Graph.Components {
		switch loaded.Graph.Components[componentIndex].ID {
		case "scalar":
			loaded.Graph.Components[componentIndex].Nodes.Outputs[0].Unit = "W"
		case "second_gain":
			loaded.Graph.Components[componentIndex].Nodes.Inputs[0].Unit = "kW"
		}
	}
	if err := writeJSONFile(loaded.GraphPath, loaded.Graph); err != nil {
		t.Fatal(err)
	}

	connectionPayload, err := json.Marshal(map[string]any{
		"project_path":   summary.ProjectPath,
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

	updatePayload, err := json.Marshal(map[string]any{
		"project_path":  summary.ProjectPath,
		"connection_id": connectionBody.Connection.ID,
		"unit_conversion": map[string]any{
			"factor":      0.001,
			"offset":      2,
			"description": "W to kW with bias",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	updateResponse := httptest.NewRecorder()
	updateRequest := httptest.NewRequest(http.MethodPost, "/api/project/connections/update", bytes.NewReader(updatePayload))
	server.Handler().ServeHTTP(updateResponse, updateRequest)
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("update status = %d body=%s", updateResponse.Code, updateResponse.Body.String())
	}
	loaded, err = project.Load(summary.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	conversion := loaded.Graph.Connections[0].UnitConversion
	if conversion == nil || conversion.Mode != "linear" || conversion.Factor == nil || *conversion.Factor != 0.001 || conversion.Offset == nil || *conversion.Offset != 2 {
		t.Fatalf("conversion = %#v", conversion)
	}

	invalidPayload, err := json.Marshal(map[string]any{
		"project_path":    summary.ProjectPath,
		"connection_id":   connectionBody.Connection.ID,
		"unit_conversion": map[string]any{"mode": "table", "factor": 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	invalidResponse := httptest.NewRecorder()
	invalidRequest := httptest.NewRequest(http.MethodPost, "/api/project/connections/update", bytes.NewReader(invalidPayload))
	server.Handler().ServeHTTP(invalidResponse, invalidRequest)
	if invalidResponse.Code != http.StatusBadRequest {
		t.Fatalf("invalid status = %d body=%s", invalidResponse.Code, invalidResponse.Body.String())
	}

	clearPayload, err := json.Marshal(map[string]any{
		"project_path":    summary.ProjectPath,
		"connection_id":   connectionBody.Connection.ID,
		"unit_conversion": nil,
	})
	if err != nil {
		t.Fatal(err)
	}
	clearResponse := httptest.NewRecorder()
	clearRequest := httptest.NewRequest(http.MethodPost, "/api/project/connections/update", bytes.NewReader(clearPayload))
	server.Handler().ServeHTTP(clearResponse, clearRequest)
	if clearResponse.Code != http.StatusOK {
		t.Fatalf("clear status = %d body=%s", clearResponse.Code, clearResponse.Body.String())
	}
	loaded, err = project.Load(summary.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Graph.Connections[0].UnitConversion != nil {
		t.Fatalf("conversion after clear = %#v", loaded.Graph.Connections[0].UnitConversion)
	}
}

func TestValidateEndpointReportsConnectionMediumWarnings(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	summary := createWorkspaceProject(t, server, "Medium Warning Project")

	componentPayload, err := json.Marshal(map[string]any{
		"project_path": summary.ProjectPath,
		"name":         "Water Sink",
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
		"project_path": summary.ProjectPath,
		"component_id": "water_sink",
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

	loaded, err := project.Load(summary.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	for index := range loaded.Graph.Components {
		if loaded.Graph.Components[index].ID == "water_sink" {
			loaded.Graph.Components[index].Nodes.Inputs[0].Medium = "water"
		}
	}
	if err := writeJSONFile(loaded.GraphPath, loaded.Graph); err != nil {
		t.Fatal(err)
	}

	connectionPayload, err := json.Marshal(map[string]any{
		"project_path":   summary.ProjectPath,
		"from_component": "scalar",
		"from_node":      "result",
		"to_component":   "water_sink",
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

	validatePayload, err := json.Marshal(map[string]any{"project_path": summary.ProjectPath})
	if err != nil {
		t.Fatal(err)
	}
	validateResponse := httptest.NewRecorder()
	validateRequest := httptest.NewRequest(http.MethodPost, "/api/validate", bytes.NewReader(validatePayload))
	server.Handler().ServeHTTP(validateResponse, validateRequest)
	if validateResponse.Code != http.StatusOK {
		t.Fatalf("validate status = %d body=%s", validateResponse.Code, validateResponse.Body.String())
	}
	var body struct {
		Validation struct {
			Problems []Problem `json:"problems"`
		} `json:"validation"`
	}
	if err := json.Unmarshal(validateResponse.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Validation.Problems) != 1 {
		t.Fatalf("problems = %#v", body.Validation.Problems)
	}
	problem := body.Validation.Problems[0]
	if problem.Severity != "warning" || problem.ComponentID != "water_sink" || problem.NodeID != "value" {
		t.Fatalf("problem = %#v", problem)
	}
	if !strings.Contains(problem.Message, "connection scalar_result_to_water_sink_value medium mismatch") {
		t.Fatalf("problem message = %s", problem.Message)
	}
}

func TestWorkspaceWorkflowEditsSourceConnectsAndRuns(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	project := createWorkspaceProject(t, server, "Workflow Project")
	source := strings.TrimLeft(`
class ScalarComponent:
    def initialize(self, params, context):
        return {}

    def evaluate(self, inputs, state, params, context):
        value = float(inputs["value"])
        return {"result": value * 3.0}, state
`, "\n")

	sourcePayload, err := json.Marshal(map[string]any{
		"project_path": project.ProjectPath,
		"component_id": "scalar",
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
	if hasErrorProblems(sourceBody.Check.Problems) {
		t.Fatalf("source check errors = %#v", sourceBody.Check.Problems)
	}

	componentPayload, err := json.Marshal(map[string]any{
		"project_path": project.ProjectPath,
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
		"project_path": project.ProjectPath,
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
		"project_path":   project.ProjectPath,
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

	runPayload, err := json.Marshal(map[string]any{
		"project_path": project.ProjectPath,
		"inputs":       map[string]any{"value": 4},
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
		Result runtimecore.RunResult `json:"result"`
	}
	if err := json.Unmarshal(runResponse.Body.Bytes(), &runBody); err != nil {
		t.Fatal(err)
	}
	if got := runBody.Result.Outputs["second_gain_result"]; got != 24.0 {
		t.Fatalf("second_gain_result = %v, want 24", got)
	}
	if got := runBody.Result.ComponentOutputs["scalar"]["result"]; got != 12.0 {
		t.Fatalf("scalar result = %v, want 12", got)
	}
	if got := runBody.Result.ComponentInputs["second_gain"]["value"]; got != 12.0 {
		t.Fatalf("second_gain value input = %v, want 12", got)
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

func TestUpdateComponentContractEndpointWritesDefinitionsAndMetadata(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Contract Project"}`)))
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

	visible := false
	payload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"component_id": "second_gain",
		"parameters": map[string]any{
			"gain": 4.5,
		},
		"parameter_defs": map[string]model.ParameterDefinition{
			"gain": {
				DisplayName: "Test Gain",
				Unit:        "ratio",
				Default:     2.0,
				Current:     4.5,
				Bounds:      &model.ValueBounds{Min: 0.5, Max: 10.0},
				Role:        "optimization_variable",
				Group:       "Tuning",
				Description: "Edited from Studio",
				Visible:     &visible,
			},
		},
		"state_defs": map[string]model.StateDefinition{
			"accumulator": {
				DisplayName: "Accumulator",
				Unit:        "count",
				Initial:     1.0,
				Description: "Editable state",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/component-contract", bytes.NewReader(payload))
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("contract status = %d body=%s", response.Code, response.Body.String())
	}

	loaded, err := project.Load(createBody.Project.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	component, found := findComponent(loaded.Graph, "second_gain")
	if !found {
		t.Fatal("second_gain missing")
	}
	if got := component.Parameters["gain"]; got != 4.5 {
		t.Fatalf("gain = %v, want 4.5", got)
	}
	gainDefinition := component.ParameterDefinitions["gain"]
	if gainDefinition.DisplayName != "Test Gain" || gainDefinition.Role != "optimization_variable" || gainDefinition.Group != "Tuning" {
		t.Fatalf("gain definition = %#v", gainDefinition)
	}
	if gainDefinition.Visible == nil || *gainDefinition.Visible {
		t.Fatalf("gain visible = %#v, want false", gainDefinition.Visible)
	}
	if gainDefinition.Bounds == nil || gainDefinition.Bounds.Min != 0.5 || gainDefinition.Bounds.Max != 10.0 {
		t.Fatalf("gain bounds = %#v", gainDefinition.Bounds)
	}
	if component.StateDefinitions["accumulator"].Initial != 1.0 {
		t.Fatalf("state definitions = %#v", component.StateDefinitions)
	}

	metadataPath := filepath.Join(root, "projects", "contract-project", "components", "second_gain", "component.json")
	metadataBytes, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatal(err)
	}
	var metadata struct {
		Parameters           map[string]any                       `json:"parameters"`
		ParameterDefinitions map[string]model.ParameterDefinition `json:"parameter_defs"`
		StateDefinitions     map[string]model.StateDefinition     `json:"state_defs"`
	}
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata.Parameters["gain"] != 4.5 {
		t.Fatalf("metadata gain = %v, want 4.5", metadata.Parameters["gain"])
	}
	if metadata.ParameterDefinitions["gain"].Visible == nil || *metadata.ParameterDefinitions["gain"].Visible {
		t.Fatalf("metadata visible = %#v", metadata.ParameterDefinitions["gain"].Visible)
	}
	if metadata.StateDefinitions["accumulator"].Unit != "count" {
		t.Fatalf("metadata state definitions = %#v", metadata.StateDefinitions)
	}
	wrapperBytes, err := os.ReadFile(filepath.Join(root, "projects", "contract-project", "components", "second_gain", "wrapper.py"))
	if err != nil {
		t.Fatal(err)
	}
	wrapperContent := string(wrapperBytes)
	for _, want := range []string{`\"Test Gain\"`, `\"optimization_variable\"`, `\"accumulator\"`, `\"count\"`, "Inputs: value", "Outputs: result", "Parameters: gain", "State: accumulator"} {
		if !strings.Contains(wrapperContent, want) {
			t.Fatalf("wrapper contract did not reflect component contract update %q:\n%s", want, wrapperContent)
		}
	}

	badPayload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"component_id": "second_gain",
		"parameter_defs": map[string]model.ParameterDefinition{
			"gain": {
				Current: 4.5,
				Bounds:  &model.ValueBounds{Min: 10.0, Max: 1.0},
				Role:    "optimization_variable",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	badResponse := httptest.NewRecorder()
	badRequest := httptest.NewRequest(http.MethodPost, "/api/project/component-contract", bytes.NewReader(badPayload))
	server.Handler().ServeHTTP(badResponse, badRequest)
	if badResponse.Code != http.StatusBadRequest {
		t.Fatalf("bad bounds status = %d body=%s", badResponse.Code, badResponse.Body.String())
	}
	var badBody apiError
	if err := json.Unmarshal(badResponse.Body.Bytes(), &badBody); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(badBody.Message, "parameter bounds min must be <= max") {
		t.Fatalf("bad bounds message = %#v", badBody)
	}

	deletePayload, err := json.Marshal(map[string]any{
		"project_path":          createBody.Project.ProjectPath,
		"component_id":          "second_gain",
		"delete_state_defs":     []string{"accumulator"},
		"delete_parameter_defs": []string{"gain"},
	})
	if err != nil {
		t.Fatal(err)
	}
	deleteResponse := httptest.NewRecorder()
	deleteRequest := httptest.NewRequest(http.MethodPost, "/api/project/component-contract", bytes.NewReader(deletePayload))
	server.Handler().ServeHTTP(deleteResponse, deleteRequest)
	if deleteResponse.Code != http.StatusOK {
		t.Fatalf("delete contract status = %d body=%s", deleteResponse.Code, deleteResponse.Body.String())
	}
	loaded, err = project.Load(createBody.Project.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	component, _ = findComponent(loaded.Graph, "second_gain")
	if _, exists := component.StateDefinitions["accumulator"]; exists {
		t.Fatalf("state definition should be deleted: %#v", component.StateDefinitions)
	}
	if _, exists := component.ParameterDefinitions["gain"]; exists {
		t.Fatalf("parameter definition should be cleared: %#v", component.ParameterDefinitions)
	}
	wrapperBytes, err = os.ReadFile(filepath.Join(root, "projects", "contract-project", "components", "second_gain", "wrapper.py"))
	if err != nil {
		t.Fatal(err)
	}
	wrapperContent = string(wrapperBytes)
	for _, removed := range []string{`\"Test Gain\"`, `\"accumulator\"`} {
		if strings.Contains(wrapperContent, removed) {
			t.Fatalf("wrapper contract retained deleted metadata %q:\n%s", removed, wrapperContent)
		}
	}
	if !strings.Contains(wrapperContent, "Parameters: gain") || !strings.Contains(wrapperContent, "State: none") {
		t.Fatalf("wrapper docstring did not reflect deleted metadata:\n%s", wrapperContent)
	}
}

func TestUpdateParametersEndpointSyncsExternalExecutableMetadataWithoutOverwritingExecutable(t *testing.T) {
	root, server := newIsolatedTestServer(t)
	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	projectRoot := filepath.Join(root, "projects", "external-executable-project")
	if err := copyProjectTree(filepath.Join(repoRoot, "examples", "010_external_executable_component"), projectRoot); err != nil {
		t.Fatal(err)
	}
	projectPath := filepath.Join(projectRoot, "project.bcsproj")
	executablePath := filepath.Join(projectRoot, "components", "external_gain", "external_gain.py")
	originalExecutable, err := os.ReadFile(executablePath)
	if err != nil {
		t.Fatal(err)
	}

	payload, err := json.Marshal(map[string]any{
		"project_path": projectPath,
		"parameters": map[string]any{
			"external_gain": map[string]any{"gain": 3.25},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/project/parameters", bytes.NewReader(payload))
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}

	currentExecutable, err := os.ReadFile(executablePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(currentExecutable, originalExecutable) {
		t.Fatalf("external executable was overwritten by metadata sync:\n%s", string(currentExecutable))
	}
	metadataBytes, err := os.ReadFile(filepath.Join(projectRoot, "components", "external_gain", "component.json"))
	if err != nil {
		t.Fatal(err)
	}
	var metadata struct {
		Parameters           map[string]any                       `json:"parameters"`
		ParameterDefinitions map[string]model.ParameterDefinition `json:"parameter_defs"`
	}
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata.Parameters["gain"] != 3.25 || metadata.ParameterDefinitions["gain"].Current != 3.25 {
		t.Fatalf("external metadata was not synced:\n%s", string(metadataBytes))
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
