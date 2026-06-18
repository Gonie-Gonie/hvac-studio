package studio

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/goniegonie/hvac-studio/tools/go/internal/apperror"
	"github.com/goniegonie/hvac-studio/tools/go/internal/compiler"
	"github.com/goniegonie/hvac-studio/tools/go/internal/model"
	"github.com/goniegonie/hvac-studio/tools/go/internal/project"
)

func createComponent(loaded *project.LoadedProject, req createComponentRequest, repoRoot string) (model.Component, error) {
	componentName := strings.TrimSpace(req.Name)
	if componentName == "" {
		return model.Component{}, apperror.Errorf(apperror.CodeValidation, "component name is required")
	}
	template := req.Template
	if template == "" {
		template = "scalar"
	}
	templateManifest, templateFiles, err := loadComponentTemplate(repoRoot, template)
	if err != nil {
		return model.Component{}, err
	}

	componentID := uniqueComponentID(loaded.Graph, strings.ReplaceAll(slugify(componentName), "-", "_"))
	if componentID == "" {
		return model.Component{}, apperror.Errorf(apperror.CodeValidation, "component name must contain letters or numbers")
	}
	className := pythonClassName(componentID)
	component := componentFromTemplateManifest(componentID, componentName, templateManifest)

	componentsRoot := filepath.Join(loaded.Root, "components")
	if err := os.MkdirAll(componentsRoot, 0o755); err != nil {
		return model.Component{}, err
	}
	initPath := filepath.Join(componentsRoot, "__init__.py")
	if _, err := os.Stat(initPath); os.IsNotExist(err) {
		if err := os.WriteFile(initPath, []byte(""), 0o644); err != nil {
			return model.Component{}, err
		}
	}
	if err := writeComponentTemplateFiles(loaded.Root, component, templateFiles, templateManifest.ClassName, className); err != nil {
		return model.Component{}, err
	}
	if component.Source.Metadata != "" {
		metadataPath, err := resolveProjectOwnedFile(loaded.Root, component.Source.Metadata)
		if err != nil {
			return model.Component{}, err
		}
		if err := writeComponentMetadataFile(metadataPath, component, className); err != nil {
			return model.Component{}, err
		}
	}
	loaded.Graph.Components = append(loaded.Graph.Components, component)
	if err := writeJSONFile(loaded.GraphPath, loaded.Graph); err != nil {
		return model.Component{}, err
	}
	return component, nil
}

func componentFromTemplateManifest(componentID string, componentName string, manifest componentTemplateManifest) model.Component {
	className := pythonClassName(componentID)
	componentSource := componentSourceForTemplate(manifest.Source, componentID)
	return model.Component{
		ID:            componentID,
		Name:          componentName,
		Kind:          defaultString(manifest.Kind, "user_python"),
		Category:      defaultString(manifest.Category, "utility"),
		ExecutionMode: defaultString(manifest.ExecutionMode, "step"),
		Class:         classPathForComponentSource(componentID, componentSource, className),
		Source:        componentSource,
		Nodes: model.NodeSet{
			Inputs:  componentTemplateNodes(manifest.Inputs, "inlet"),
			Outputs: componentTemplateNodes(manifest.Outputs, "outlet"),
		},
		Parameters:           cloneMap(manifest.Parameters),
		ParameterDefinitions: cloneParameterDefinitions(manifest.ParameterDefinitions),
		StateDefinitions:     cloneStateDefinitions(manifest.StateDefinitions),
		SolverBoundary:       cloneSolverBoundary(manifest.SolverBoundary),
		MLMetadata:           componentMLMetadataForTemplate(manifest.MLMetadata, componentID),
	}
}

func duplicateComponent(loaded *project.LoadedProject, req duplicateComponentRequest) (model.Component, error) {
	sourceID := strings.TrimSpace(req.SourceComponentID)
	if sourceID == "" {
		return model.Component{}, apperror.Errorf(apperror.CodeValidation, "source_component_id is required")
	}
	source, found := findComponent(loaded.Graph, sourceID)
	if !found {
		return model.Component{}, apperror.Errorf(apperror.CodeValidation, "component not found: %s", sourceID)
	}
	componentName := strings.TrimSpace(req.Name)
	if componentName == "" {
		componentName = strings.TrimSpace(source.Name) + " Copy"
	}
	if strings.TrimSpace(componentName) == "" {
		componentName = sourceID + " Copy"
	}

	componentID := uniqueComponentID(loaded.Graph, strings.ReplaceAll(slugify(componentName), "-", "_"))
	classParts := strings.Split(source.Class, ".")
	if len(classParts) < 1 || classParts[len(classParts)-1] == "" {
		return model.Component{}, apperror.Errorf(apperror.CodeValidation, "component %s class is invalid: %s", sourceID, source.Class)
	}
	className := classParts[len(classParts)-1]
	component := source
	component.ID = componentID
	component.Name = componentName
	component.Class = "components." + componentID + "." + className
	component.Nodes.Inputs = append([]model.Node(nil), source.Nodes.Inputs...)
	component.Nodes.Outputs = append([]model.Node(nil), source.Nodes.Outputs...)
	component.Parameters = map[string]any{}
	for name, value := range source.Parameters {
		component.Parameters[name] = value
	}
	component.ParameterDefinitions = cloneParameterDefinitions(source.ParameterDefinitions)
	component.StateDefinitions = cloneStateDefinitions(source.StateDefinitions)
	component.SolverBoundary = cloneSolverBoundary(source.SolverBoundary)
	component.MLMetadata = cloneMLMetadata(source.MLMetadata)
	component.Source = cloneComponentSource(source.Source)
	if component.Source.Layout == "" || component.Source.Layout == "single_file_class" {
		component.Source.Layout = "single_file_class"
		component.Source.Step = filepath.ToSlash(filepath.Join("components", componentID+".py"))
		component.Class = "components." + componentID + "." + className
	} else if component.Source.Layout == "generated_wrapper" {
		sourceDirRel, err := generatedComponentSourceDir(component.Source)
		if err != nil {
			return model.Component{}, err
		}
		targetDirRel := filepath.ToSlash(filepath.Join("components", componentID))
		component.Source = rebaseComponentSource(component.Source, sourceDirRel, targetDirRel)
		component.Class = classPathForComponentSource(componentID, component.Source, className)
	}

	sourceRoot := filepath.Join(loaded.Root, "components")
	if err := os.MkdirAll(sourceRoot, 0o755); err != nil {
		return model.Component{}, err
	}
	copiedPath, err := copyComponentSourceArtifact(loaded, source, component)
	if err != nil {
		return model.Component{}, err
	}
	if component.Source.Metadata != "" {
		metadataPath, err := resolveProjectOwnedFile(loaded.Root, component.Source.Metadata)
		if err != nil {
			return model.Component{}, err
		}
		if err := writeComponentMetadataFile(metadataPath, component, className); err != nil {
			return model.Component{}, err
		}
	}
	loaded.Graph.Components = append(loaded.Graph.Components, component)
	if err := writeJSONFile(loaded.GraphPath, loaded.Graph); err != nil {
		_ = removeComponentSourceArtifact(copiedPath, component.Source.Layout)
		return model.Component{}, err
	}
	return component, nil
}

func updateComponent(loaded *project.LoadedProject, req updateComponentRequest) (model.Component, error) {
	componentID := strings.TrimSpace(req.ComponentID)
	if componentID == "" {
		return model.Component{}, apperror.Errorf(apperror.CodeValidation, "component_id is required")
	}
	componentName := strings.TrimSpace(req.Name)
	if componentName == "" {
		return model.Component{}, apperror.Errorf(apperror.CodeValidation, "component name is required")
	}
	for index := range loaded.Graph.Components {
		if loaded.Graph.Components[index].ID != componentID {
			continue
		}
		loaded.Graph.Components[index].Name = componentName
		if err := syncComponentMetadataFile(loaded, loaded.Graph.Components[index]); err != nil {
			return model.Component{}, err
		}
		if err := writeJSONFile(loaded.GraphPath, loaded.Graph); err != nil {
			return model.Component{}, err
		}
		return loaded.Graph.Components[index], nil
	}
	return model.Component{}, apperror.Errorf(apperror.CodeValidation, "component not found: %s", componentID)
}

func deleteComponent(loaded *project.LoadedProject, req deleteComponentRequest) error {
	componentID := strings.TrimSpace(req.ComponentID)
	if componentID == "" {
		return apperror.Errorf(apperror.CodeValidation, "component_id is required")
	}
	componentIndex := -1
	for index := range loaded.Graph.Components {
		if loaded.Graph.Components[index].ID == componentID {
			componentIndex = index
			break
		}
	}
	if componentIndex < 0 {
		return apperror.Errorf(apperror.CodeValidation, "component not found: %s", componentID)
	}
	deletedComponent := loaded.Graph.Components[componentIndex]
	for _, system := range loaded.Graph.Systems {
		if containsString(system.Components, componentID) {
			return apperror.Errorf(apperror.CodeValidation, "component is still used by system %s; remove it from the system first", system.ID)
		}
	}
	for _, connection := range loaded.Graph.Connections {
		if connection.From.Component == componentID || connection.To.Component == componentID {
			return apperror.Errorf(apperror.CodeValidation, "component still has connection reference: %s", connection.ID)
		}
	}

	sourceArtifactPath, err := componentSourceArtifactPath(loaded, deletedComponent)
	if err != nil {
		return err
	}
	sourceShared := false
	for index := range loaded.Graph.Components {
		if index == componentIndex {
			continue
		}
		otherPath, err := componentSourceArtifactPath(loaded, loaded.Graph.Components[index])
		if err == nil && sameFilesystemPath(otherPath, sourceArtifactPath) {
			sourceShared = true
			break
		}
	}

	loaded.Graph.Components = append(loaded.Graph.Components[:componentIndex], loaded.Graph.Components[componentIndex+1:]...)
	if _, err := compiler.Compile(loaded); err != nil {
		return apperror.Wrap(apperror.CodeValidation, err)
	}
	if err := writeJSONFile(loaded.GraphPath, loaded.Graph); err != nil {
		return err
	}
	if !sourceShared {
		if err := removeComponentSourceArtifact(sourceArtifactPath, deletedComponent.Source.Layout); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func includeComponentInSystem(loaded *project.LoadedProject, req includeComponentRequest) error {
	componentID := strings.TrimSpace(req.ComponentID)
	if componentID == "" {
		return apperror.Errorf(apperror.CodeValidation, "component_id is required")
	}
	component, foundComponent := findComponent(loaded.Graph, componentID)
	if !foundComponent {
		return apperror.Errorf(apperror.CodeValidation, "component not found: %s", componentID)
	}

	systemID := req.SystemID
	if systemID == "" {
		systemID = loaded.Project.EntrySystem
	}
	systemIndex := -1
	for index := range loaded.Graph.Systems {
		if loaded.Graph.Systems[index].ID == systemID {
			systemIndex = index
			break
		}
	}
	if systemIndex < 0 {
		return apperror.Errorf(apperror.CodeValidation, "system not found: %s", systemID)
	}

	system := &loaded.Graph.Systems[systemIndex]
	if containsString(system.Components, componentID) {
		return nil
	}

	inputPath, input, err := loadEditableDefaultInput(loaded)
	if err != nil {
		return err
	}
	system.Components = append(system.Components, componentID)
	for _, node := range component.Nodes.Inputs {
		if hasPublicInputFor(*system, componentID, node.ID) {
			continue
		}
		publicID := uniquePublicNodeID(system.PublicInputs, componentID+"_"+node.ID)
		system.PublicInputs = append(system.PublicInputs, model.PublicNodeRef{
			ID:        publicID,
			Name:      node.Name,
			Component: componentID,
			Node:      node.ID,
			Medium:    node.Medium,
			ValueType: node.ValueType,
			Unit:      node.Unit,
			Required:  node.Required,
			Default:   node.Default,
		})
		if _, exists := input.Inputs[publicID]; !exists {
			input.Inputs[publicID] = defaultValueForNode(node)
		}
	}
	for _, node := range component.Nodes.Outputs {
		if hasPublicOutputFor(*system, componentID, node.ID) {
			continue
		}
		publicID := uniquePublicNodeID(system.PublicOutputs, componentID+"_"+node.ID)
		system.PublicOutputs = append(system.PublicOutputs, model.PublicNodeRef{
			ID:        publicID,
			Name:      node.Name,
			Component: componentID,
			Node:      node.ID,
			Medium:    node.Medium,
			ValueType: node.ValueType,
			Unit:      node.Unit,
			Default:   node.Default,
		})
	}

	if err := writeJSONFile(inputPath, input); err != nil {
		return err
	}
	return writeJSONFile(loaded.GraphPath, loaded.Graph)
}

func removeComponentFromSystem(loaded *project.LoadedProject, req includeComponentRequest) error {
	componentID := strings.TrimSpace(req.ComponentID)
	if componentID == "" {
		return apperror.Errorf(apperror.CodeValidation, "component_id is required")
	}
	if _, foundComponent := findComponent(loaded.Graph, componentID); !foundComponent {
		return apperror.Errorf(apperror.CodeValidation, "component not found: %s", componentID)
	}

	systemID := req.SystemID
	if systemID == "" {
		systemID = loaded.Project.EntrySystem
	}
	systemIndex := -1
	for index := range loaded.Graph.Systems {
		if loaded.Graph.Systems[index].ID == systemID {
			systemIndex = index
			break
		}
	}
	if systemIndex < 0 {
		return apperror.Errorf(apperror.CodeValidation, "system not found: %s", systemID)
	}

	system := &loaded.Graph.Systems[systemIndex]
	if !containsString(system.Components, componentID) {
		return nil
	}
	inputPath, input, err := loadEditableDefaultInput(loaded)
	if err != nil {
		return err
	}

	system.Components = removeString(system.Components, componentID)
	removedPublicInputs := removePublicInputsForComponent(system, componentID)
	for _, inputID := range removedPublicInputs {
		delete(input.Inputs, inputID)
	}
	removePublicOutputsForComponent(system, componentID)

	removedConnections := []model.Connection{}
	keptConnectionIDs := system.Connections[:0]
	for _, connectionID := range system.Connections {
		connection, found := findConnection(loaded.Graph, connectionID)
		if !found {
			keptConnectionIDs = append(keptConnectionIDs, connectionID)
			continue
		}
		if connection.From.Component == componentID || connection.To.Component == componentID {
			removedConnections = append(removedConnections, connection)
			continue
		}
		keptConnectionIDs = append(keptConnectionIDs, connectionID)
	}
	system.Connections = keptConnectionIDs

	for _, connection := range removedConnections {
		if connection.To.Component == componentID || !containsString(system.Components, connection.To.Component) {
			continue
		}
		if systemHasIncomingConnection(*system, loaded.Graph, connection.To.Component, connection.To.Node) || hasPublicInputFor(*system, connection.To.Component, connection.To.Node) {
			continue
		}
		component, foundComponent := findComponent(loaded.Graph, connection.To.Component)
		if !foundComponent {
			return apperror.Errorf(apperror.CodeValidation, "connection target component not found: %s", connection.To.Component)
		}
		node, foundNode := findInputNode(component, connection.To.Node)
		if !foundNode {
			return apperror.Errorf(apperror.CodeValidation, "connection target input node not found: %s.%s", connection.To.Component, connection.To.Node)
		}
		publicID := uniquePublicNodeID(system.PublicInputs, connection.To.Component+"_"+connection.To.Node)
		system.PublicInputs = append(system.PublicInputs, model.PublicNodeRef{
			ID:        publicID,
			Name:      node.Name,
			Component: connection.To.Component,
			Node:      node.ID,
			Medium:    node.Medium,
			ValueType: node.ValueType,
			Unit:      node.Unit,
			Required:  node.Required,
			Default:   node.Default,
		})
		if _, exists := input.Inputs[publicID]; !exists {
			input.Inputs[publicID] = defaultValueForNode(node)
		}
	}

	loaded.Graph.Connections = removeUnreferencedConnections(loaded.Graph.Connections, loaded.Graph.Systems)
	if _, err := compiler.Compile(loaded); err != nil {
		return apperror.Wrap(apperror.CodeValidation, err)
	}
	if err := writeJSONFile(inputPath, input); err != nil {
		return err
	}
	return writeJSONFile(loaded.GraphPath, loaded.Graph)
}
