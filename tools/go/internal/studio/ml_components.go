package studio

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/goniegonie/hvac-studio/tools/go/internal/apperror"
	"github.com/goniegonie/hvac-studio/tools/go/internal/model"
	"github.com/goniegonie/hvac-studio/tools/go/internal/project"
)

type updateComponentMLAssetsRequest struct {
	ProjectPath         string                       `json:"project_path"`
	ComponentID         string                       `json:"component_id"`
	ModelFormat         string                       `json:"model_format"`
	RequiredPackages    []string                     `json:"required_packages"`
	ValidTimeResolution string                       `json:"valid_time_resolution"`
	ValidInputRanges    map[string]model.ValueBounds `json:"valid_input_ranges,omitempty"`
	Assets              []componentMLAssetUpload     `json:"assets"`
}

type componentMLAssetUpload struct {
	Field         string `json:"field"`
	FileName      string `json:"file_name"`
	Content       string `json:"content,omitempty"`
	ContentBase64 string `json:"content_base64,omitempty"`
}

type applyComponentMLSchemaNodesRequest struct {
	ProjectPath string `json:"project_path"`
	ComponentID string `json:"component_id"`
}

type MLSchemaNodeApplySummary struct {
	FeatureCount    int      `json:"feature_count"`
	TargetCount     int      `json:"target_count"`
	AddedInputs     []string `json:"added_inputs"`
	AddedOutputs    []string `json:"added_outputs"`
	ExistingInputs  []string `json:"existing_inputs"`
	ExistingOutputs []string `json:"existing_outputs"`
}

type mlSchemaNode struct {
	ID        string
	Name      string
	Medium    string
	ValueType string
	Unit      string
}

func decodeUpdateComponentMLAssetsRequest(r *http.Request) (updateComponentMLAssetsRequest, error) {
	defer r.Body.Close()
	var req updateComponentMLAssetsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeApplyComponentMLSchemaNodesRequest(r *http.Request) (applyComponentMLSchemaNodesRequest, error) {
	defer r.Body.Close()
	var req applyComponentMLSchemaNodesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func updateComponentMLAssets(loaded *project.LoadedProject, req updateComponentMLAssetsRequest) (model.Component, []string, error) {
	componentID := strings.TrimSpace(req.ComponentID)
	if componentID == "" {
		return model.Component{}, nil, apperror.Errorf(apperror.CodeValidation, "component_id is required")
	}
	componentIndex := -1
	for index := range loaded.Graph.Components {
		if loaded.Graph.Components[index].ID == componentID {
			componentIndex = index
			break
		}
	}
	if componentIndex < 0 {
		return model.Component{}, nil, apperror.Errorf(apperror.CodeValidation, "component not found: %s", componentID)
	}

	metadata := cloneMLMetadata(loaded.Graph.Components[componentIndex].MLMetadata)
	if metadata == nil {
		metadata = &model.MLMetadata{}
	}
	metadata.ModelFormat = strings.TrimSpace(req.ModelFormat)
	metadata.RequiredPackages = cleanRequiredPackages(req.RequiredPackages)
	metadata.ValidTimeResolution = strings.TrimSpace(req.ValidTimeResolution)
	if req.ValidInputRanges != nil {
		metadata.ValidInputRanges = cloneValueBoundsMap(req.ValidInputRanges)
	}

	importedFiles := []string{}
	for _, asset := range req.Assets {
		target, err := mlMetadataAssetField(metadata, asset.Field)
		if err != nil {
			return model.Component{}, nil, err
		}
		content, err := decodeMLAssetContent(asset)
		if err != nil {
			return model.Component{}, nil, err
		}
		name, err := cleanMLAssetFileName(asset.Field, asset.FileName)
		if err != nil {
			return model.Component{}, nil, err
		}
		rel := projectComponentSourceRel(componentID, name)
		path, err := resolveProjectOwnedFile(loaded.Root, rel)
		if err != nil {
			return model.Component{}, nil, err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return model.Component{}, nil, err
		}
		if err := os.WriteFile(path, content, 0o644); err != nil {
			return model.Component{}, nil, err
		}
		*target = rel
		importedFiles = append(importedFiles, rel)
	}

	loaded.Graph.Components[componentIndex].MLMetadata = metadata
	if err := syncComponentMetadataFile(loaded, loaded.Graph.Components[componentIndex]); err != nil {
		return model.Component{}, nil, err
	}
	if err := writeJSONFile(loaded.GraphPath, loaded.Graph); err != nil {
		return model.Component{}, nil, err
	}
	return loaded.Graph.Components[componentIndex], importedFiles, nil
}

func mlMetadataAssetField(metadata *model.MLMetadata, field string) (*string, error) {
	switch strings.TrimSpace(field) {
	case "model_file":
		return &metadata.ModelFile, nil
	case "input_scaler_file":
		return &metadata.InputScalerFile, nil
	case "output_scaler_file":
		return &metadata.OutputScalerFile, nil
	case "feature_schema_file":
		return &metadata.FeatureSchemaFile, nil
	case "target_schema_file":
		return &metadata.TargetSchemaFile, nil
	case "training_metadata_file":
		return &metadata.TrainingMetadataFile, nil
	case "validation_report_file":
		return &metadata.ValidationReportFile, nil
	default:
		return nil, apperror.Errorf(apperror.CodeValidation, "unsupported ML asset field: %s", field)
	}
}

func decodeMLAssetContent(asset componentMLAssetUpload) ([]byte, error) {
	if strings.TrimSpace(asset.ContentBase64) != "" {
		content, err := base64.StdEncoding.DecodeString(asset.ContentBase64)
		if err != nil {
			return nil, apperror.Errorf(apperror.CodeInput, "ML asset %s is not valid base64", asset.Field)
		}
		return content, nil
	}
	if asset.Content != "" {
		return []byte(asset.Content), nil
	}
	return nil, apperror.Errorf(apperror.CodeValidation, "ML asset content is required for %s", asset.Field)
}

func cleanMLAssetFileName(field string, fileName string) (string, error) {
	fileName = strings.TrimSpace(fileName)
	if fileName == "" {
		fileName = defaultMLAssetFileName(field)
	}
	if fileName == "" || strings.ContainsAny(fileName, `/\`) {
		return "", apperror.Errorf(apperror.CodeValidation, "ML asset file name must be a file name: %s", fileName)
	}
	clean := filepath.Clean(fileName)
	if clean == "." || clean == ".." || clean != filepath.Base(clean) {
		return "", apperror.Errorf(apperror.CodeValidation, "ML asset file name must be a file name: %s", fileName)
	}
	return clean, nil
}

func defaultMLAssetFileName(field string) string {
	switch strings.TrimSpace(field) {
	case "model_file":
		return "model.bin"
	case "input_scaler_file":
		return "input_scaler.json"
	case "output_scaler_file":
		return "output_scaler.json"
	case "feature_schema_file":
		return "feature_schema.json"
	case "target_schema_file":
		return "target_schema.json"
	case "training_metadata_file":
		return "training_metadata.json"
	case "validation_report_file":
		return "validation_report.json"
	default:
		return ""
	}
}

func cleanRequiredPackages(values []string) []string {
	seen := map[string]bool{}
	cleaned := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		cleaned = append(cleaned, value)
	}
	return cleaned
}

func cloneValueBoundsMap(values map[string]model.ValueBounds) map[string]model.ValueBounds {
	if values == nil {
		return nil
	}
	out := map[string]model.ValueBounds{}
	for key, value := range values {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		out[key] = value
	}
	return out
}

func applyComponentMLSchemaNodes(loaded *project.LoadedProject, req applyComponentMLSchemaNodesRequest) (model.Component, MLSchemaNodeApplySummary, error) {
	componentID := strings.TrimSpace(req.ComponentID)
	if componentID == "" {
		return model.Component{}, MLSchemaNodeApplySummary{}, apperror.Errorf(apperror.CodeValidation, "component_id is required")
	}
	component, found := findComponent(loaded.Graph, componentID)
	if !found {
		return model.Component{}, MLSchemaNodeApplySummary{}, apperror.Errorf(apperror.CodeValidation, "component not found: %s", componentID)
	}
	if component.MLMetadata == nil {
		return model.Component{}, MLSchemaNodeApplySummary{}, apperror.Errorf(apperror.CodeValidation, "component has no ML metadata: %s", componentID)
	}
	features, err := readMLSchemaNodes(loaded.Root, component.MLMetadata.FeatureSchemaFile, []string{"features"}, "float")
	if err != nil {
		return model.Component{}, MLSchemaNodeApplySummary{}, err
	}
	targets, err := readMLSchemaNodes(loaded.Root, component.MLMetadata.TargetSchemaFile, []string{"targets", "outputs"}, "float")
	if err != nil {
		return model.Component{}, MLSchemaNodeApplySummary{}, err
	}
	if len(features) == 0 && len(targets) == 0 {
		return model.Component{}, MLSchemaNodeApplySummary{}, apperror.Errorf(apperror.CodeValidation, "ML feature or target schema is required")
	}

	summary := MLSchemaNodeApplySummary{
		FeatureCount: len(features),
		TargetCount:  len(targets),
	}
	if len(features) > 0 {
		required := true
		current, _ := findComponent(loaded.Graph, componentID)
		if componentHasInputNode(current, "features") {
			summary.ExistingInputs = append(summary.ExistingInputs, "features")
		} else {
			if _, err := createNode(loaded, createNodeRequest{
				ComponentID: componentID,
				Direction:   "input",
				ID:          "features",
				Name:        "Features",
				Medium:      "signal",
				ValueType:   "object",
				Required:    &required,
				Default:     map[string]any{},
			}); err != nil {
				return model.Component{}, MLSchemaNodeApplySummary{}, err
			}
			summary.AddedInputs = append(summary.AddedInputs, "features")
		}
	}
	for _, target := range targets {
		current, _ := findComponent(loaded.Graph, componentID)
		if componentHasOutputNode(current, target.ID) {
			summary.ExistingOutputs = append(summary.ExistingOutputs, target.ID)
			continue
		}
		if componentHasNode(current, target.ID) {
			return model.Component{}, MLSchemaNodeApplySummary{}, apperror.Errorf(apperror.CodeValidation, "schema target conflicts with existing non-output node: %s.%s", componentID, target.ID)
		}
		if _, err := createNode(loaded, createNodeRequest{
			ComponentID: componentID,
			Direction:   "output",
			ID:          target.ID,
			Name:        defaultString(target.Name, displayNameFromID(target.ID)),
			Medium:      defaultString(target.Medium, "signal"),
			ValueType:   defaultString(target.ValueType, "float"),
			Unit:        target.Unit,
		}); err != nil {
			return model.Component{}, MLSchemaNodeApplySummary{}, err
		}
		summary.AddedOutputs = append(summary.AddedOutputs, target.ID)
	}
	updated, found := findComponent(loaded.Graph, componentID)
	if !found {
		return model.Component{}, MLSchemaNodeApplySummary{}, apperror.Errorf(apperror.CodeValidation, "component not found after schema node apply: %s", componentID)
	}
	return updated, summary, nil
}

func readMLSchemaNodes(projectRoot string, rel string, keys []string, fallbackValueType string) ([]mlSchemaNode, error) {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return nil, nil
	}
	path, err := resolveProjectOwnedFile(projectRoot, rel)
	if err != nil {
		return nil, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, apperror.Errorf(apperror.CodeValidation, "ML schema file is missing: %s", filepath.ToSlash(rel))
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(content, &document); err != nil {
		return nil, apperror.Wrap(apperror.CodeValidation, err)
	}
	for _, key := range keys {
		raw, ok := document[key]
		if !ok {
			continue
		}
		nodes, err := decodeMLSchemaNodes(raw, fallbackValueType)
		if err != nil {
			return nil, err
		}
		return nodes, nil
	}
	return nil, nil
}

func decodeMLSchemaNodes(raw json.RawMessage, fallbackValueType string) ([]mlSchemaNode, error) {
	var names []string
	if err := json.Unmarshal(raw, &names); err == nil {
		nodes := []mlSchemaNode{}
		for _, name := range names {
			node, err := mlSchemaNodeFromParts("", name, "", fallbackValueType, "")
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, node)
		}
		return nodes, nil
	}
	var objects []struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Medium    string `json:"medium"`
		ValueType string `json:"value_type"`
		Unit      string `json:"unit"`
	}
	if err := json.Unmarshal(raw, &objects); err != nil {
		return nil, apperror.Wrap(apperror.CodeValidation, err)
	}
	nodes := []mlSchemaNode{}
	for _, item := range objects {
		node, err := mlSchemaNodeFromParts(item.ID, item.Name, item.Medium, defaultString(item.ValueType, fallbackValueType), item.Unit)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

func mlSchemaNodeFromParts(id string, name string, medium string, valueType string, unit string) (mlSchemaNode, error) {
	id = strings.TrimSpace(id)
	name = strings.TrimSpace(name)
	if id == "" {
		id = name
	}
	id = strings.ReplaceAll(slugify(id), "-", "_")
	if id == "" {
		return mlSchemaNode{}, apperror.Errorf(apperror.CodeValidation, "ML schema node id is required")
	}
	if id[0] >= '0' && id[0] <= '9' {
		id = "n_" + id
	}
	if !isIdentifierLike(id) {
		return mlSchemaNode{}, apperror.Errorf(apperror.CodeValidation, "ML schema node id is invalid: %s", id)
	}
	if name == "" {
		name = displayNameFromID(id)
	}
	return mlSchemaNode{
		ID:        id,
		Name:      name,
		Medium:    strings.TrimSpace(medium),
		ValueType: strings.TrimSpace(valueType),
		Unit:      strings.TrimSpace(unit),
	}, nil
}
