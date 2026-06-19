package studio

import (
	"fmt"
	"strings"

	"github.com/goniegonie/hvac-studio/tools/go/internal/apperror"
	"github.com/goniegonie/hvac-studio/tools/go/internal/compiler"
	"github.com/goniegonie/hvac-studio/tools/go/internal/model"
	"github.com/goniegonie/hvac-studio/tools/go/internal/project"
	runtimecore "github.com/goniegonie/hvac-studio/tools/go/internal/runtime"
)

func createNode(loaded *project.LoadedProject, req createNodeRequest) (model.Node, error) {
	componentID := strings.TrimSpace(req.ComponentID)
	if componentID == "" {
		return model.Node{}, apperror.Errorf(apperror.CodeValidation, "component_id is required")
	}
	nodeID := strings.TrimSpace(req.ID)
	if !isIdentifierLike(nodeID) {
		return model.Node{}, apperror.Errorf(apperror.CodeValidation, "node id must start with a letter or underscore and contain only letters, numbers, and underscores")
	}
	direction := strings.ToLower(strings.TrimSpace(req.Direction))
	isInput := false
	nodeDirection := ""
	switch direction {
	case "input", "in", "inlet":
		isInput = true
		nodeDirection = "inlet"
	case "output", "out", "outlet":
		nodeDirection = "outlet"
	default:
		return model.Node{}, apperror.Errorf(apperror.CodeValidation, "node direction must be input or output")
	}

	componentIndex := -1
	for index := range loaded.Graph.Components {
		if loaded.Graph.Components[index].ID == componentID {
			componentIndex = index
			break
		}
	}
	if componentIndex < 0 {
		return model.Node{}, apperror.Errorf(apperror.CodeValidation, "component not found: %s", componentID)
	}
	component := &loaded.Graph.Components[componentIndex]
	if componentHasNode(*component, nodeID) {
		return model.Node{}, apperror.Errorf(apperror.CodeValidation, "component already has node: %s.%s", componentID, nodeID)
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = nodeID
	}
	medium := strings.TrimSpace(req.Medium)
	if medium == "" {
		medium = "signal"
	}
	valueType := strings.TrimSpace(req.ValueType)
	if valueType == "" {
		valueType = "float"
	}
	node := model.Node{
		ID:        nodeID,
		Name:      name,
		Preset:    strings.TrimSpace(req.Preset),
		Direction: nodeDirection,
		Medium:    medium,
		ValueType: valueType,
		Unit:      strings.TrimSpace(req.Unit),
		Required:  req.Required,
		Default:   req.Default,
	}
	if isInput {
		component.Nodes.Inputs = append(component.Nodes.Inputs, node)
	} else {
		component.Nodes.Outputs = append(component.Nodes.Outputs, node)
	}

	inputPath, input, err := loadEditableDefaultInput(loaded)
	if err != nil {
		return model.Node{}, err
	}
	for index := range loaded.Graph.Systems {
		system := &loaded.Graph.Systems[index]
		if !containsString(system.Components, componentID) {
			continue
		}
		if isInput {
			if hasPublicInputFor(*system, componentID, nodeID) {
				continue
			}
			publicID := uniquePublicNodeID(system.PublicInputs, componentID+"_"+nodeID)
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
			continue
		}
		if hasPublicOutputFor(*system, componentID, nodeID) {
			continue
		}
		publicID := uniquePublicNodeID(system.PublicOutputs, componentID+"_"+nodeID)
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
	if _, err := compiler.Compile(loaded); err != nil {
		return model.Node{}, apperror.Wrap(apperror.CodeValidation, err)
	}
	if err := syncComponentMetadataFile(loaded, *component); err != nil {
		return model.Node{}, err
	}
	if err := writeJSONFile(inputPath, input); err != nil {
		return model.Node{}, err
	}
	if err := writeJSONFile(loaded.GraphPath, loaded.Graph); err != nil {
		return model.Node{}, err
	}
	return node, nil
}

func updateNode(loaded *project.LoadedProject, req updateNodeRequest) (model.Node, error) {
	componentID := strings.TrimSpace(req.ComponentID)
	nodeID := strings.TrimSpace(req.NodeID)
	if componentID == "" || nodeID == "" {
		return model.Node{}, apperror.Errorf(apperror.CodeValidation, "component_id and node_id are required")
	}

	componentIndex := -1
	for index := range loaded.Graph.Components {
		if loaded.Graph.Components[index].ID == componentID {
			componentIndex = index
			break
		}
	}
	if componentIndex < 0 {
		return model.Node{}, apperror.Errorf(apperror.CodeValidation, "component not found: %s", componentID)
	}

	component := &loaded.Graph.Components[componentIndex]
	nodeIndex := -1
	isInput := true
	for index := range component.Nodes.Inputs {
		if component.Nodes.Inputs[index].ID == nodeID {
			nodeIndex = index
			break
		}
	}
	if nodeIndex < 0 {
		isInput = false
		for index := range component.Nodes.Outputs {
			if component.Nodes.Outputs[index].ID == nodeID {
				nodeIndex = index
				break
			}
		}
	}
	if nodeIndex < 0 {
		return model.Node{}, apperror.Errorf(apperror.CodeValidation, "node not found: %s.%s", componentID, nodeID)
	}

	newID := strings.TrimSpace(req.NewID)
	if newID == "" {
		newID = nodeID
	}
	if !isIdentifierLike(newID) {
		return model.Node{}, apperror.Errorf(apperror.CodeValidation, "node id must start with a letter or underscore and contain only letters, numbers, and underscores")
	}
	if newID != nodeID && componentHasNode(*component, newID) {
		return model.Node{}, apperror.Errorf(apperror.CodeValidation, "component already has node: %s.%s", componentID, newID)
	}

	var node *model.Node
	if isInput {
		node = &component.Nodes.Inputs[nodeIndex]
	} else {
		node = &component.Nodes.Outputs[nodeIndex]
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = nodeID
	}
	medium := strings.TrimSpace(req.Medium)
	if medium == "" {
		medium = "signal"
	}
	valueType := strings.TrimSpace(req.ValueType)
	if valueType == "" {
		valueType = "float"
	}
	node.ID = newID
	node.Name = name
	node.Medium = medium
	node.ValueType = valueType
	node.Unit = strings.TrimSpace(req.Unit)
	node.Required = req.Required
	if req.DefaultProvided {
		node.Default = req.Default
	}
	updatedNode := *node

	inputPath := ""
	var input runtimecore.RunInput
	inputDirty := false
	if isInput {
		var err error
		inputPath, input, err = loadEditableDefaultInput(loaded)
		if err != nil {
			return model.Node{}, err
		}
	}

	for systemIndex := range loaded.Graph.Systems {
		system := &loaded.Graph.Systems[systemIndex]
		if !containsString(system.Components, componentID) {
			continue
		}
		if isInput {
			for refIndex := range system.PublicInputs {
				ref := &system.PublicInputs[refIndex]
				if ref.Component != componentID || ref.Node != nodeID {
					continue
				}
				updatePublicNodeRef(ref, updatedNode)
				if req.DefaultProvided {
					input.Inputs[ref.ID] = defaultValueForNode(updatedNode)
					inputDirty = true
					continue
				}
				if _, exists := input.Inputs[ref.ID]; !exists {
					input.Inputs[ref.ID] = defaultValueForNode(updatedNode)
					inputDirty = true
				}
			}
			continue
		}
		for refIndex := range system.PublicOutputs {
			ref := &system.PublicOutputs[refIndex]
			if ref.Component != componentID || ref.Node != nodeID {
				continue
			}
			updatePublicNodeRef(ref, updatedNode)
		}
	}
	if newID != nodeID {
		for connectionIndex := range loaded.Graph.Connections {
			connection := &loaded.Graph.Connections[connectionIndex]
			if endpointMatches(connection.From, componentID, nodeID) {
				connection.From.Node = newID
			}
			if endpointMatches(connection.To, componentID, nodeID) {
				connection.To.Node = newID
			}
		}
	}

	if _, err := compiler.Compile(loaded); err != nil {
		return model.Node{}, apperror.Wrap(apperror.CodeValidation, err)
	}
	if err := syncComponentMetadataFile(loaded, *component); err != nil {
		return model.Node{}, err
	}
	if inputDirty {
		if err := writeJSONFile(inputPath, input); err != nil {
			return model.Node{}, err
		}
	}
	if err := writeJSONFile(loaded.GraphPath, loaded.Graph); err != nil {
		return model.Node{}, err
	}
	return updatedNode, nil
}

func deleteNode(loaded *project.LoadedProject, req deleteNodeRequest) (model.Node, error) {
	componentID := strings.TrimSpace(req.ComponentID)
	nodeID := strings.TrimSpace(req.NodeID)
	if componentID == "" || nodeID == "" {
		return model.Node{}, apperror.Errorf(apperror.CodeValidation, "component_id and node_id are required")
	}

	componentIndex := -1
	for index := range loaded.Graph.Components {
		if loaded.Graph.Components[index].ID == componentID {
			componentIndex = index
			break
		}
	}
	if componentIndex < 0 {
		return model.Node{}, apperror.Errorf(apperror.CodeValidation, "component not found: %s", componentID)
	}
	component := &loaded.Graph.Components[componentIndex]
	node, isInput, foundNode := removeNodeFromComponent(component, nodeID)
	if !foundNode {
		return model.Node{}, apperror.Errorf(apperror.CodeValidation, "node not found: %s.%s", componentID, nodeID)
	}

	inputPath, input, err := loadEditableDefaultInput(loaded)
	if err != nil {
		return model.Node{}, err
	}
	for systemIndex := range loaded.Graph.Systems {
		system := &loaded.Graph.Systems[systemIndex]
		if !containsString(system.Components, componentID) {
			continue
		}
		if isInput {
			for _, inputID := range removePublicInputsFor(system, componentID, nodeID) {
				delete(input.Inputs, inputID)
			}
		} else {
			removePublicOutputsFor(system, componentID, nodeID)
		}

		removedConnections := []model.Connection{}
		keptConnectionIDs := system.Connections[:0]
		for _, connectionID := range system.Connections {
			connection, found := findConnection(loaded.Graph, connectionID)
			if !found {
				keptConnectionIDs = append(keptConnectionIDs, connectionID)
				continue
			}
			if endpointMatches(connection.From, componentID, nodeID) || endpointMatches(connection.To, componentID, nodeID) {
				removedConnections = append(removedConnections, connection)
				continue
			}
			keptConnectionIDs = append(keptConnectionIDs, connectionID)
		}
		system.Connections = keptConnectionIDs

		for _, connection := range removedConnections {
			if endpointMatches(connection.To, componentID, nodeID) || !containsString(system.Components, connection.To.Component) {
				continue
			}
			if systemHasIncomingConnection(*system, loaded.Graph, connection.To.Component, connection.To.Node) || hasPublicInputFor(*system, connection.To.Component, connection.To.Node) {
				continue
			}
			targetComponent, foundComponent := findComponent(loaded.Graph, connection.To.Component)
			if !foundComponent {
				return model.Node{}, apperror.Errorf(apperror.CodeValidation, "connection target component not found: %s", connection.To.Component)
			}
			targetNode, foundTargetNode := findInputNode(targetComponent, connection.To.Node)
			if !foundTargetNode {
				return model.Node{}, apperror.Errorf(apperror.CodeValidation, "connection target input node not found: %s.%s", connection.To.Component, connection.To.Node)
			}
			publicID := uniquePublicNodeID(system.PublicInputs, connection.To.Component+"_"+connection.To.Node)
			system.PublicInputs = append(system.PublicInputs, model.PublicNodeRef{
				ID:        publicID,
				Name:      targetNode.Name,
				Component: connection.To.Component,
				Node:      targetNode.ID,
				Medium:    targetNode.Medium,
				ValueType: targetNode.ValueType,
				Unit:      targetNode.Unit,
				Required:  targetNode.Required,
				Default:   targetNode.Default,
			})
			if _, exists := input.Inputs[publicID]; !exists {
				input.Inputs[publicID] = defaultValueForNode(targetNode)
			}
		}
	}

	loaded.Graph.Connections = removeUnreferencedConnections(loaded.Graph.Connections, loaded.Graph.Systems)
	if _, err := compiler.Compile(loaded); err != nil {
		return model.Node{}, apperror.Wrap(apperror.CodeValidation, err)
	}
	if err := syncComponentMetadataFile(loaded, *component); err != nil {
		return model.Node{}, err
	}
	if err := writeJSONFile(inputPath, input); err != nil {
		return model.Node{}, err
	}
	if err := writeJSONFile(loaded.GraphPath, loaded.Graph); err != nil {
		return model.Node{}, err
	}
	return node, nil
}

func deleteParameter(loaded *project.LoadedProject, req deleteParameterRequest) error {
	componentID := strings.TrimSpace(req.ComponentID)
	name := strings.TrimSpace(req.Name)
	if componentID == "" || name == "" {
		return apperror.Errorf(apperror.CodeValidation, "component_id and name are required")
	}
	for index := range loaded.Graph.Components {
		component := &loaded.Graph.Components[index]
		if component.ID != componentID {
			continue
		}
		if component.Parameters == nil {
			return apperror.Errorf(apperror.CodeValidation, "parameter not found: %s.%s", componentID, name)
		}
		if _, found := component.Parameters[name]; !found {
			return apperror.Errorf(apperror.CodeValidation, "parameter not found: %s.%s", componentID, name)
		}
		delete(component.Parameters, name)
		if component.ParameterDefinitions != nil {
			delete(component.ParameterDefinitions, name)
		}
		if err := syncComponentMetadataFile(loaded, *component); err != nil {
			return err
		}
		return writeJSONFile(loaded.GraphPath, loaded.Graph)
	}
	return apperror.Errorf(apperror.CodeValidation, "component not found: %s", componentID)
}

func updateComponentContract(loaded *project.LoadedProject, req updateComponentContractRequest) (model.Component, error) {
	componentID := strings.TrimSpace(req.ComponentID)
	if componentID == "" {
		return model.Component{}, apperror.Errorf(apperror.CodeValidation, "component_id is required")
	}
	componentIndex := -1
	for index := range loaded.Graph.Components {
		if loaded.Graph.Components[index].ID == componentID {
			componentIndex = index
			break
		}
	}
	if componentIndex < 0 {
		return model.Component{}, apperror.Errorf(apperror.CodeValidation, "component not found: %s", componentID)
	}
	component := &loaded.Graph.Components[componentIndex]

	if req.Parameters != nil && component.Parameters == nil {
		component.Parameters = map[string]any{}
	}
	for name, value := range req.Parameters {
		name = strings.TrimSpace(name)
		if !isIdentifierLike(name) {
			return model.Component{}, apperror.Errorf(apperror.CodeValidation, "parameter name must start with a letter or underscore and contain only letters, numbers, and underscores")
		}
		component.Parameters[name] = value
	}

	if len(req.ParameterDefinitions) > 0 && component.ParameterDefinitions == nil {
		component.ParameterDefinitions = map[string]model.ParameterDefinition{}
	}
	for name, definition := range req.ParameterDefinitions {
		name = strings.TrimSpace(name)
		if !isIdentifierLike(name) {
			return model.Component{}, apperror.Errorf(apperror.CodeValidation, "parameter definition name must start with a letter or underscore and contain only letters, numbers, and underscores")
		}
		current, hasCurrent := component.Parameters[name]
		definition = normalizeParameterDefinition(name, definition, current, hasCurrent)
		if err := validateParameterDefinition(component.ID, name, definition); err != nil {
			return model.Component{}, err
		}
		component.ParameterDefinitions[name] = definition
		if component.Parameters == nil {
			component.Parameters = map[string]any{}
		}
		if _, exists := component.Parameters[name]; !exists {
			if definition.Current != nil {
				component.Parameters[name] = definition.Current
			} else if definition.Default != nil {
				component.Parameters[name] = definition.Default
			}
		}
	}
	for _, name := range req.DeleteParameterDefinitions {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if component.ParameterDefinitions != nil {
			delete(component.ParameterDefinitions, name)
		}
	}

	if len(req.StateDefinitions) > 0 && component.StateDefinitions == nil {
		component.StateDefinitions = map[string]model.StateDefinition{}
	}
	for name, definition := range req.StateDefinitions {
		name = strings.TrimSpace(name)
		if !isIdentifierLike(name) {
			return model.Component{}, apperror.Errorf(apperror.CodeValidation, "state definition name must start with a letter or underscore and contain only letters, numbers, and underscores")
		}
		component.StateDefinitions[name] = normalizeStateDefinition(name, definition)
	}
	for _, name := range req.DeleteStateDefinitions {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if component.StateDefinitions != nil {
			delete(component.StateDefinitions, name)
		}
	}

	if err := syncComponentMetadataFile(loaded, *component); err != nil {
		return model.Component{}, err
	}
	if err := writeJSONFile(loaded.GraphPath, loaded.Graph); err != nil {
		return model.Component{}, err
	}
	return *component, nil
}

func normalizeParameterDefinition(name string, definition model.ParameterDefinition, current any, hasCurrent bool) model.ParameterDefinition {
	if strings.TrimSpace(definition.DisplayName) == "" {
		definition.DisplayName = displayNameFromID(name)
	}
	if definition.Current == nil && hasCurrent {
		definition.Current = current
	}
	if definition.Default == nil && hasCurrent {
		definition.Default = current
	}
	return definition
}

func validateParameterDefinition(componentID string, name string, definition model.ParameterDefinition) error {
	if definition.Bounds == nil {
		return nil
	}
	hasMin := definition.Bounds.Min != nil
	hasMax := definition.Bounds.Max != nil
	var minValue, maxValue float64
	if hasMin {
		var ok bool
		minValue, ok = studioNumberValue(definition.Bounds.Min)
		if !ok {
			return apperror.Errorf(apperror.CodeValidation, "parameter bounds min must be numeric: %s.%s", componentID, name)
		}
	}
	if hasMax {
		var ok bool
		maxValue, ok = studioNumberValue(definition.Bounds.Max)
		if !ok {
			return apperror.Errorf(apperror.CodeValidation, "parameter bounds max must be numeric: %s.%s", componentID, name)
		}
	}
	if hasMin && hasMax && minValue > maxValue {
		return apperror.Errorf(apperror.CodeValidation, "parameter bounds min must be <= max: %s.%s", componentID, name)
	}
	return nil
}

func normalizeStateDefinition(name string, definition model.StateDefinition) model.StateDefinition {
	if strings.TrimSpace(definition.DisplayName) == "" {
		definition.DisplayName = displayNameFromID(name)
	}
	return definition
}

func createConnection(loaded *project.LoadedProject, req createConnectionRequest) (model.Connection, error) {
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
		return model.Connection{}, apperror.Errorf(apperror.CodeValidation, "system not found: %s", systemID)
	}
	fromComponent := strings.TrimSpace(req.FromComponent)
	fromNode := strings.TrimSpace(req.FromNode)
	toComponent := strings.TrimSpace(req.ToComponent)
	toNode := strings.TrimSpace(req.ToNode)
	if fromComponent == "" || fromNode == "" || toComponent == "" || toNode == "" {
		return model.Connection{}, apperror.Errorf(apperror.CodeValidation, "connection endpoints are required")
	}

	system := &loaded.Graph.Systems[systemIndex]
	if !containsString(system.Components, fromComponent) {
		return model.Connection{}, apperror.Errorf(apperror.CodeValidation, "connection source component is not in system: %s", fromComponent)
	}
	if !containsString(system.Components, toComponent) {
		return model.Connection{}, apperror.Errorf(apperror.CodeValidation, "connection target component is not in system: %s", toComponent)
	}

	inputPath, input, err := loadEditableDefaultInput(loaded)
	if err != nil {
		return model.Connection{}, err
	}
	connection := model.Connection{
		ID: uniqueConnectionID(
			loaded.Graph,
			fmt.Sprintf("%s_%s_to_%s_%s", fromComponent, fromNode, toComponent, toNode),
		),
		From: model.Endpoint{Component: fromComponent, Node: fromNode},
		To:   model.Endpoint{Component: toComponent, Node: toNode},
	}
	removedPublicInputs := removePublicInputsFor(system, toComponent, toNode)
	for _, inputID := range removedPublicInputs {
		delete(input.Inputs, inputID)
	}
	loaded.Graph.Connections = append(loaded.Graph.Connections, connection)
	system.Connections = append(system.Connections, connection.ID)
	if _, err := compiler.Compile(loaded); err != nil {
		return model.Connection{}, apperror.Wrap(apperror.CodeValidation, err)
	}
	if err := writeJSONFile(inputPath, input); err != nil {
		return model.Connection{}, err
	}
	if err := writeJSONFile(loaded.GraphPath, loaded.Graph); err != nil {
		return model.Connection{}, err
	}
	return connection, nil
}

func updateConnection(loaded *project.LoadedProject, req updateConnectionRequest) (model.Connection, error) {
	connectionID := strings.TrimSpace(req.ConnectionID)
	if connectionID == "" {
		return model.Connection{}, apperror.Errorf(apperror.CodeValidation, "connection_id is required")
	}
	if !req.UnitConversionWasPresent {
		return model.Connection{}, apperror.Errorf(apperror.CodeValidation, "unit_conversion is required")
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
		return model.Connection{}, apperror.Errorf(apperror.CodeValidation, "system not found: %s", systemID)
	}

	connectionIndex := -1
	for index, item := range loaded.Graph.Connections {
		if item.ID == connectionID {
			connectionIndex = index
			break
		}
	}
	if connectionIndex < 0 {
		return model.Connection{}, apperror.Errorf(apperror.CodeValidation, "connection not found: %s", connectionID)
	}
	system := &loaded.Graph.Systems[systemIndex]
	if !containsString(system.Connections, connectionID) {
		return model.Connection{}, apperror.Errorf(apperror.CodeValidation, "connection is not in system %s: %s", systemID, connectionID)
	}

	loaded.Graph.Connections[connectionIndex].UnitConversion = normalizeUnitConversion(req.UnitConversion)
	if _, err := compiler.Compile(loaded); err != nil {
		return model.Connection{}, apperror.Wrap(apperror.CodeValidation, err)
	}
	if err := writeJSONFile(loaded.GraphPath, loaded.Graph); err != nil {
		return model.Connection{}, err
	}
	return loaded.Graph.Connections[connectionIndex], nil
}

func normalizeUnitConversion(conversion *model.UnitConversion) *model.UnitConversion {
	if conversion == nil {
		return nil
	}
	normalized := *conversion
	normalized.Mode = strings.TrimSpace(normalized.Mode)
	if normalized.Mode == "" {
		normalized.Mode = "linear"
	}
	normalized.Description = strings.TrimSpace(normalized.Description)
	return &normalized
}

func deleteConnection(loaded *project.LoadedProject, req deleteConnectionRequest) (model.Connection, error) {
	connectionID := strings.TrimSpace(req.ConnectionID)
	if connectionID == "" {
		return model.Connection{}, apperror.Errorf(apperror.CodeValidation, "connection_id is required")
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
		return model.Connection{}, apperror.Errorf(apperror.CodeValidation, "system not found: %s", systemID)
	}

	connectionIndex := -1
	var connection model.Connection
	for index, item := range loaded.Graph.Connections {
		if item.ID == connectionID {
			connectionIndex = index
			connection = item
			break
		}
	}
	if connectionIndex < 0 {
		return model.Connection{}, apperror.Errorf(apperror.CodeValidation, "connection not found: %s", connectionID)
	}

	system := &loaded.Graph.Systems[systemIndex]
	if !containsString(system.Connections, connectionID) {
		return model.Connection{}, apperror.Errorf(apperror.CodeValidation, "system %s does not contain connection: %s", systemID, connectionID)
	}

	inputPath, input, err := loadEditableDefaultInput(loaded)
	if err != nil {
		return model.Connection{}, err
	}
	system.Connections = removeString(system.Connections, connectionID)
	if !graphReferencesConnection(loaded.Graph.Systems, connectionID) {
		loaded.Graph.Connections = append(loaded.Graph.Connections[:connectionIndex], loaded.Graph.Connections[connectionIndex+1:]...)
	}
	if !systemHasIncomingConnection(*system, loaded.Graph, connection.To.Component, connection.To.Node) && !hasPublicInputFor(*system, connection.To.Component, connection.To.Node) {
		component, foundComponent := findComponent(loaded.Graph, connection.To.Component)
		if !foundComponent {
			return model.Connection{}, apperror.Errorf(apperror.CodeValidation, "connection target component not found: %s", connection.To.Component)
		}
		node, foundNode := findInputNode(component, connection.To.Node)
		if !foundNode {
			return model.Connection{}, apperror.Errorf(apperror.CodeValidation, "connection target input node not found: %s.%s", connection.To.Component, connection.To.Node)
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
	if _, err := compiler.Compile(loaded); err != nil {
		return model.Connection{}, apperror.Wrap(apperror.CodeValidation, err)
	}
	if err := writeJSONFile(inputPath, input); err != nil {
		return model.Connection{}, err
	}
	if err := writeJSONFile(loaded.GraphPath, loaded.Graph); err != nil {
		return model.Connection{}, err
	}
	return connection, nil
}
