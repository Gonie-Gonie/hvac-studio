package studio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/goniegonie/hvac-studio/tools/go/internal/apperror"
	"github.com/goniegonie/hvac-studio/tools/go/internal/model"
	"github.com/goniegonie/hvac-studio/tools/go/internal/project"
)

type componentTemplateManifest struct {
	ID                   string                               `json:"id"`
	Name                 string                               `json:"name"`
	Kind                 string                               `json:"kind"`
	Category             string                               `json:"category"`
	ExecutionMode        string                               `json:"execution_mode"`
	ClassName            string                               `json:"class_name"`
	Source               componentTemplateSource              `json:"source"`
	Assets               []string                             `json:"assets"`
	Inputs               []model.Node                         `json:"inputs"`
	Outputs              []model.Node                         `json:"outputs"`
	Parameters           map[string]any                       `json:"parameters"`
	ParameterDefinitions map[string]model.ParameterDefinition `json:"parameter_defs"`
	StateDefinitions     map[string]model.StateDefinition     `json:"state_defs"`
	SolverBoundary       *model.SolverBoundary                `json:"solver_boundary,omitempty"`
	MLMetadata           *model.MLMetadata                    `json:"ml_metadata,omitempty"`
}

type componentTemplateSource struct {
	Layout   string `json:"layout,omitempty"`
	Metadata string `json:"metadata,omitempty"`
	Init     string `json:"init,omitempty"`
	Step     string `json:"step,omitempty"`
	Helpers  string `json:"helpers,omitempty"`
	Wrapper  string `json:"wrapper,omitempty"`
}

func (s *componentTemplateSource) UnmarshalJSON(data []byte) error {
	var singleFile string
	if err := json.Unmarshal(data, &singleFile); err == nil {
		*s = componentTemplateSource{
			Layout: "single_file_class",
			Step:   singleFile,
		}
		return nil
	}
	type source componentTemplateSource
	var decoded source
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*s = componentTemplateSource(decoded)
	if strings.TrimSpace(s.Layout) == "" {
		if strings.TrimSpace(s.Wrapper) != "" {
			s.Layout = "generated_wrapper"
		} else {
			s.Layout = "single_file_class"
		}
	}
	return nil
}

type componentTemplateFile struct {
	Role        string
	TemplateRel string
	Content     string
}

type ComponentTemplateSummary struct {
	ID                   string                               `json:"id"`
	Name                 string                               `json:"name"`
	Kind                 string                               `json:"kind"`
	Category             string                               `json:"category"`
	ExecutionMode        string                               `json:"execution_mode"`
	SourceLayout         string                               `json:"source_layout"`
	Inputs               []model.Node                         `json:"inputs,omitempty"`
	Outputs              []model.Node                         `json:"outputs,omitempty"`
	Parameters           map[string]any                       `json:"parameters,omitempty"`
	ParameterDefinitions map[string]model.ParameterDefinition `json:"parameter_defs,omitempty"`
	InputCount           int                                  `json:"input_count"`
	OutputCount          int                                  `json:"output_count"`
	ParameterCount       int                                  `json:"parameter_count"`
}

func listComponentTemplates(repoRoot string) ([]ComponentTemplateSummary, error) {
	componentsRoot := filepath.Join(repoRoot, "templates", "components")
	entries, err := os.ReadDir(componentsRoot)
	if err != nil {
		return nil, apperror.Errorf(apperror.CodeValidation, "component templates directory is missing: templates/components")
	}
	templates := []ComponentTemplateSummary{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		id := entry.Name()
		manifest, _, err := loadComponentTemplate(repoRoot, id)
		if err != nil {
			return nil, err
		}
		displayID := strings.TrimSpace(manifest.ID)
		if displayID == "" {
			displayID = id
		}
		name := strings.TrimSpace(manifest.Name)
		if name == "" {
			name = displayNameFromID(displayID)
		}
		templates = append(templates, ComponentTemplateSummary{
			ID:                   id,
			Name:                 name,
			Kind:                 defaultString(manifest.Kind, "user_python"),
			Category:             defaultString(manifest.Category, "utility"),
			ExecutionMode:        defaultString(manifest.ExecutionMode, "step"),
			SourceLayout:         defaultString(manifest.Source.Layout, "single_file_class"),
			Inputs:               componentTemplateNodes(manifest.Inputs, "inlet"),
			Outputs:              componentTemplateNodes(manifest.Outputs, "outlet"),
			Parameters:           cloneMap(manifest.Parameters),
			ParameterDefinitions: cloneParameterDefinitions(manifest.ParameterDefinitions),
			InputCount:           len(manifest.Inputs),
			OutputCount:          len(manifest.Outputs),
			ParameterCount:       len(manifest.Parameters),
		})
	}
	sort.Slice(templates, func(i, j int) bool {
		if templates[i].ID == "scalar" {
			return true
		}
		if templates[j].ID == "scalar" {
			return false
		}
		return templates[i].ID < templates[j].ID
	})
	return templates, nil
}

func loadComponentTemplate(repoRoot, template string) (componentTemplateManifest, []componentTemplateFile, error) {
	templateRoot := filepath.Join(repoRoot, "templates", "components", template)
	manifestPath := filepath.Join(templateRoot, "manifest.json")
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return componentTemplateManifest{}, nil, apperror.Errorf(apperror.CodeValidation, "component template manifest is missing: templates/components/%s/manifest.json", template)
	}
	var manifest componentTemplateManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return componentTemplateManifest{}, nil, apperror.Wrap(apperror.CodeValidation, err)
	}
	if strings.TrimSpace(manifest.ClassName) == "" {
		return componentTemplateManifest{}, nil, apperror.Errorf(apperror.CodeValidation, "component template %s class_name is required", template)
	}
	files, err := loadComponentTemplateFiles(templateRoot, template, manifest.Source)
	if err != nil {
		return componentTemplateManifest{}, nil, err
	}
	assetFiles, err := loadComponentTemplateAssetFiles(templateRoot, template, manifest.Assets)
	if err != nil {
		return componentTemplateManifest{}, nil, err
	}
	files = append(files, assetFiles...)
	return manifest, files, nil
}

func loadComponentTemplateAssetFiles(templateRoot, template string, assets []string) ([]componentTemplateFile, error) {
	files := []componentTemplateFile{}
	for _, rel := range assets {
		if strings.TrimSpace(rel) == "" {
			continue
		}
		file, err := loadComponentTemplateFile(templateRoot, template, "asset", rel)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, nil
}

func loadComponentTemplateFiles(templateRoot, template string, source componentTemplateSource) ([]componentTemplateFile, error) {
	layout := defaultString(source.Layout, "single_file_class")
	switch layout {
	case "single_file_class":
		if strings.TrimSpace(source.Step) == "" {
			return nil, apperror.Errorf(apperror.CodeValidation, "component template %s source step is required", template)
		}
		file, err := loadComponentTemplateFile(templateRoot, template, "step", source.Step)
		if err != nil {
			return nil, err
		}
		return []componentTemplateFile{file}, nil
	case "generated_wrapper":
		if strings.TrimSpace(source.Step) == "" || strings.TrimSpace(source.Wrapper) == "" {
			return nil, apperror.Errorf(apperror.CodeValidation, "component template %s generated_wrapper source requires step and wrapper", template)
		}
		if strings.TrimSpace(source.Metadata) != "" {
			if _, err := cleanRelativePath(source.Metadata); err != nil {
				return nil, apperror.Errorf(apperror.CodeValidation, "component template %s source path is invalid: %s", template, source.Metadata)
			}
		}
		refs := []struct {
			role string
			rel  string
		}{
			{"init", source.Init},
			{"step", source.Step},
			{"helpers", source.Helpers},
			{"wrapper", source.Wrapper},
		}
		files := []componentTemplateFile{}
		for _, ref := range refs {
			if strings.TrimSpace(ref.rel) == "" {
				continue
			}
			file, err := loadComponentTemplateFile(templateRoot, template, ref.role, ref.rel)
			if err != nil {
				return nil, err
			}
			files = append(files, file)
		}
		return files, nil
	default:
		return nil, apperror.Errorf(apperror.CodeValidation, "component template %s source layout is unsupported: %s", template, layout)
	}
}

func loadComponentTemplateFile(templateRoot, template, role, rel string) (componentTemplateFile, error) {
	cleanSource, err := cleanRelativePath(rel)
	if err != nil {
		return componentTemplateFile{}, apperror.Errorf(apperror.CodeValidation, "component template %s source path is invalid: %s", template, rel)
	}
	sourcePath := filepath.Join(templateRoot, cleanSource)
	sourceBytes, err := os.ReadFile(sourcePath)
	if err != nil {
		return componentTemplateFile{}, apperror.Errorf(apperror.CodeValidation, "component template source is missing: templates/components/%s/%s", template, rel)
	}
	return componentTemplateFile{Role: role, TemplateRel: filepath.ToSlash(cleanSource), Content: string(sourceBytes)}, nil
}

func rewriteTemplateClassName(source, oldClass, newClass string) (string, error) {
	oldDeclaration := "class " + strings.TrimSpace(oldClass) + ":"
	if !strings.Contains(source, oldDeclaration) {
		return "", apperror.Errorf(apperror.CodeValidation, "component template source does not declare %s", oldClass)
	}
	return strings.Replace(source, oldDeclaration, "class "+newClass+":", 1), nil
}

func componentSourceForTemplate(source componentTemplateSource, componentID string) model.ComponentSource {
	layout := defaultString(source.Layout, "single_file_class")
	if layout != "generated_wrapper" {
		return model.ComponentSource{
			Layout: "single_file_class",
			Step:   filepath.ToSlash(filepath.Join("components", componentID+".py")),
		}
	}
	return model.ComponentSource{
		Layout:   "generated_wrapper",
		Metadata: projectComponentSourceRel(componentID, defaultString(source.Metadata, "component.json")),
		Init:     projectComponentSourceRel(componentID, source.Init),
		Step:     projectComponentSourceRel(componentID, source.Step),
		Helpers:  projectComponentSourceRel(componentID, source.Helpers),
		Wrapper:  projectComponentSourceRel(componentID, source.Wrapper),
	}
}

func componentMLMetadataForTemplate(value *model.MLMetadata, componentID string) *model.MLMetadata {
	cloned := cloneMLMetadata(value)
	if cloned == nil {
		return nil
	}
	for _, target := range []struct {
		value *string
	}{
		{&cloned.ModelFile},
		{&cloned.InputScalerFile},
		{&cloned.OutputScalerFile},
		{&cloned.FeatureSchemaFile},
		{&cloned.TargetSchemaFile},
		{&cloned.TrainingMetadataFile},
		{&cloned.ValidationReportFile},
	} {
		if strings.TrimSpace(*target.value) == "" || filepath.IsAbs(*target.value) {
			continue
		}
		*target.value = projectComponentSourceRel(componentID, *target.value)
	}
	return cloned
}

func projectComponentSourceRel(componentID string, rel string) string {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return ""
	}
	return filepath.ToSlash(filepath.Join("components", componentID, filepath.FromSlash(rel)))
}

func classPathForComponentSource(componentID string, source model.ComponentSource, className string) string {
	if source.Layout == "generated_wrapper" && strings.TrimSpace(source.Wrapper) != "" {
		module := strings.TrimSuffix(filepath.ToSlash(source.Wrapper), ".py")
		module = strings.ReplaceAll(module, "/", ".")
		return module + "." + className
	}
	return "components." + componentID + "." + className
}

func writeComponentTemplateFiles(projectRoot string, component model.Component, files []componentTemplateFile, templateClassName string, className string) error {
	for _, file := range files {
		targetRel := componentTemplateTargetRel(component, file)
		if targetRel == "" {
			continue
		}
		targetPath, err := resolveProjectOwnedFile(projectRoot, targetRel)
		if err != nil {
			return err
		}
		if _, err := os.Stat(targetPath); err == nil {
			return apperror.Errorf(apperror.CodeValidation, "component source already exists: %s", filepath.ToSlash(targetRel))
		} else if err != nil && !os.IsNotExist(err) {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		content := file.Content
		if component.Source.Layout != "generated_wrapper" || file.Role == "wrapper" {
			rewritten, err := rewriteTemplateClassName(content, templateClassName, className)
			if err != nil {
				return err
			}
			content = rewritten
		}
		if err := os.WriteFile(targetPath, []byte(content), 0o644); err != nil {
			return err
		}
	}
	if component.Source.Layout == "generated_wrapper" {
		initPath, err := resolveProjectOwnedFile(projectRoot, filepath.ToSlash(filepath.Join("components", component.ID, "__init__.py")))
		if err != nil {
			return err
		}
		if _, err := os.Stat(initPath); os.IsNotExist(err) {
			if err := os.WriteFile(initPath, []byte(""), 0o644); err != nil {
				return err
			}
		}
		if err := writeGeneratedWrapperFile(projectRoot, component); err != nil {
			return err
		}
	}
	return nil
}

func componentTemplateTargetRel(component model.Component, file componentTemplateFile) string {
	if file.Role == "asset" {
		return projectComponentSourceRel(component.ID, file.TemplateRel)
	}
	if component.Source.Layout != "generated_wrapper" {
		return component.Source.Step
	}
	switch file.Role {
	case "init":
		return component.Source.Init
	case "step":
		return component.Source.Step
	case "helpers":
		return component.Source.Helpers
	case "wrapper":
		return component.Source.Wrapper
	default:
		return ""
	}
}

func writeComponentMetadataFile(path string, component model.Component, className string) error {
	metadata := struct {
		ID                   string                               `json:"id"`
		Name                 string                               `json:"name"`
		Kind                 string                               `json:"kind"`
		Category             string                               `json:"category"`
		ExecutionMode        string                               `json:"execution_mode"`
		ClassName            string                               `json:"class_name"`
		Source               model.ComponentSource                `json:"source"`
		Nodes                model.NodeSet                        `json:"nodes"`
		Parameters           map[string]any                       `json:"parameters"`
		ParameterDefinitions map[string]model.ParameterDefinition `json:"parameter_defs,omitempty"`
		StateDefinitions     map[string]model.StateDefinition     `json:"state_defs,omitempty"`
		SolverBoundary       *model.SolverBoundary                `json:"solver_boundary,omitempty"`
		MLMetadata           *model.MLMetadata                    `json:"ml_metadata,omitempty"`
	}{
		ID:                   component.ID,
		Name:                 component.Name,
		Kind:                 component.Kind,
		Category:             component.Category,
		ExecutionMode:        component.ExecutionMode,
		ClassName:            className,
		Source:               component.Source,
		Nodes:                component.Nodes,
		Parameters:           component.Parameters,
		ParameterDefinitions: component.ParameterDefinitions,
		StateDefinitions:     component.StateDefinitions,
		SolverBoundary:       component.SolverBoundary,
		MLMetadata:           component.MLMetadata,
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return writeJSONFile(path, metadata)
}

func syncComponentMetadataFile(loaded *project.LoadedProject, component model.Component) error {
	if strings.TrimSpace(component.Source.Metadata) == "" {
		return nil
	}
	metadataPath, err := resolveProjectOwnedFile(loaded.Root, component.Source.Metadata)
	if err != nil {
		return err
	}
	if err := writeComponentMetadataFile(metadataPath, component, classNameFromPath(component.Class)); err != nil {
		return err
	}
	if component.Source.Layout == "generated_wrapper" {
		return writeGeneratedWrapperFile(loaded.Root, component)
	}
	return nil
}

func writeGeneratedWrapperFile(projectRoot string, component model.Component) error {
	wrapperRel := strings.TrimSpace(component.Source.Wrapper)
	if component.Source.Layout != "generated_wrapper" || wrapperRel == "" {
		return nil
	}
	wrapperPath, err := resolveProjectOwnedFile(projectRoot, wrapperRel)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(wrapperPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(wrapperPath, []byte(generatedWrapperContent(component)), 0o644)
}

func generatedWrapperContent(component model.Component) string {
	className := classNameFromPath(component.Class)
	if strings.TrimSpace(className) == "" {
		className = pythonClassName(component.ID)
	}
	return "import json\n" +
		"from . import user_init, user_step\n\n\n" +
		"class " + className + ":\n" +
		"    \"\"\"Studio-owned runtime wrapper.\n\n" +
		"    Component contract metadata is regenerated from graph.json/component.json.\n" +
		"    Edit user_step.py for model logic.\n" +
		"    \"\"\"\n\n" +
		"    input_nodes = json.loads(" + pythonStringLiteralForWrapper(componentNodeContractMap(component.Nodes.Inputs)) + ")\n" +
		"    output_nodes = json.loads(" + pythonStringLiteralForWrapper(componentNodeContractMap(component.Nodes.Outputs)) + ")\n" +
		"    parameter_schema = json.loads(" + pythonStringLiteralForWrapper(component.ParameterDefinitions) + ")\n" +
		"    state_schema = json.loads(" + pythonStringLiteralForWrapper(component.StateDefinitions) + ")\n\n" +
		"    def initialize(self, params, context):\n" +
		"        state = user_init.initialize(params, context)\n" +
		"        if state is None:\n" +
		"            return {}\n" +
		"        return state\n\n" +
		"    def evaluate(self, inputs, state, params, context):\n" +
		"        return user_step.step(inputs, state, params, context)\n"
}

func componentNodeContractMap(nodes []model.Node) map[string]model.Node {
	out := map[string]model.Node{}
	for _, node := range nodes {
		out[node.ID] = node
	}
	return out
}

func pythonStringLiteralForWrapper(value any) string {
	if value == nil {
		return strconv.Quote("{}")
	}
	data, err := json.Marshal(value)
	if err != nil || string(data) == "null" {
		return strconv.Quote("{}")
	}
	return strconv.Quote(string(data))
}

func componentTemplateNodes(nodes []model.Node, direction string) []model.Node {
	out := make([]model.Node, 0, len(nodes))
	for _, node := range nodes {
		if node.Name == "" {
			node.Name = displayNameFromID(node.ID)
		}
		if node.Direction == "" {
			node.Direction = direction
		}
		if node.Medium == "" {
			node.Medium = "signal"
		}
		if node.ValueType == "" {
			node.ValueType = "float"
		}
		out = append(out, node)
	}
	return out
}
