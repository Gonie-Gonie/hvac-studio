package compiler

import (
	"fmt"
	"math"
	"os"
	"strings"

	graphindex "github.com/goniegonie/hvac-studio/tools/go/internal/graph"
	"github.com/goniegonie/hvac-studio/tools/go/internal/model"
	"github.com/goniegonie/hvac-studio/tools/go/internal/project"
	"github.com/goniegonie/hvac-studio/tools/go/internal/projectpath"
)

type Plan struct {
	System         model.System
	Index          *graphindex.Index
	Order          []string
	Incoming       map[string]model.Connection
	PublicInputs   map[string]model.PublicNodeRef
	PublicOutputs  map[string]model.PublicNodeRef
	SystemContains map[string]bool
	Diagnostics    []Diagnostic
}

type Diagnostic struct {
	Severity     string         `json:"severity"`
	Message      string         `json:"message"`
	ConnectionID string         `json:"connection_id,omitempty"`
	From         model.Endpoint `json:"from,omitempty"`
	To           model.Endpoint `json:"to,omitempty"`
}

func Compile(loaded *project.LoadedProject) (*Plan, error) {
	idx, err := graphindex.NewIndex(loaded.Graph)
	if err != nil {
		return nil, err
	}
	if err := validateMLAssets(loaded); err != nil {
		return nil, err
	}

	system, ok := idx.Systems[loaded.Project.EntrySystem]
	if !ok {
		return nil, fmt.Errorf("entry system not found: %s", loaded.Project.EntrySystem)
	}

	plan := &Plan{
		System:         system,
		Index:          idx,
		Incoming:       map[string]model.Connection{},
		PublicInputs:   map[string]model.PublicNodeRef{},
		PublicOutputs:  map[string]model.PublicNodeRef{},
		SystemContains: map[string]bool{},
	}

	for _, componentID := range system.Components {
		if _, ok := idx.Components[componentID]; !ok {
			return nil, fmt.Errorf("system %s references unknown component: %s", system.ID, componentID)
		}
		plan.SystemContains[componentID] = true
	}

	selectedConnections := make([]model.Connection, 0, len(system.Connections))
	for _, connectionID := range system.Connections {
		connection, ok := idx.Connections[connectionID]
		if !ok {
			return nil, fmt.Errorf("system %s references unknown connection: %s", system.ID, connectionID)
		}
		diagnostics, err := validateConnection(idx, plan.SystemContains, connection)
		if err != nil {
			return nil, err
		}
		plan.Diagnostics = append(plan.Diagnostics, diagnostics...)
		incomingKey := endpointKey(connection.To.Component, connection.To.Node)
		if existing, exists := plan.Incoming[incomingKey]; exists {
			return nil, fmt.Errorf(
				"input node has multiple incoming connections: %s.%s from %s and %s",
				connection.To.Component,
				connection.To.Node,
				existing.ID,
				connection.ID,
			)
		}
		plan.Incoming[incomingKey] = connection
		selectedConnections = append(selectedConnections, connection)
	}

	if err := validatePublicIO(idx, plan); err != nil {
		return nil, err
	}
	if err := validateCompositeBoundaries(idx, system); err != nil {
		return nil, err
	}

	order, err := topologicalOrder(system.Components, selectedConnections)
	if err != nil {
		return nil, err
	}
	plan.Order = order

	return plan, nil
}

func validateMLAssets(loaded *project.LoadedProject) error {
	if loaded == nil || loaded.Graph == nil {
		return nil
	}
	for _, component := range loaded.Graph.Components {
		if component.MLMetadata == nil {
			continue
		}
		for _, asset := range component.MLMetadata.AssetPaths() {
			assetPath := strings.TrimSpace(asset.Path)
			if assetPath == "" {
				continue
			}
			absAsset, err := projectpath.ResolveInside(loaded.Root, assetPath)
			if err != nil {
				return fmt.Errorf("component %s ml_metadata.%s must be project-relative and stay inside project root: %s", component.ID, asset.Field, asset.Path)
			}
			info, err := os.Stat(absAsset)
			if os.IsNotExist(err) {
				return fmt.Errorf("component %s ML asset not found: %s", component.ID, asset.Path)
			}
			if err != nil {
				return err
			}
			if info.IsDir() {
				return fmt.Errorf("component %s ML asset is a directory: %s", component.ID, asset.Path)
			}
		}
	}
	return nil
}

func validateConnection(idx *graphindex.Index, systemContains map[string]bool, connection model.Connection) ([]Diagnostic, error) {
	if !systemContains[connection.From.Component] {
		return nil, fmt.Errorf("connection %s source component is not in system: %s", connection.ID, connection.From.Component)
	}
	if !systemContains[connection.To.Component] {
		return nil, fmt.Errorf("connection %s target component is not in system: %s", connection.ID, connection.To.Component)
	}

	sourceNode, ok := idx.OutputNode(connection.From.Component, connection.From.Node)
	if !ok {
		return nil, fmt.Errorf("connection %s source output node not found: %s.%s", connection.ID, connection.From.Component, connection.From.Node)
	}
	targetNode, ok := idx.InputNode(connection.To.Component, connection.To.Node)
	if !ok {
		return nil, fmt.Errorf("connection %s target input node not found: %s.%s", connection.ID, connection.To.Component, connection.To.Node)
	}
	if err := validateConnectionUnitConversion(connection); err != nil {
		return nil, err
	}

	diagnostics := []Diagnostic{}
	diagnostic, err := connectionMediumDiagnostic(connection, sourceNode, targetNode)
	if err != nil {
		return nil, err
	}
	if diagnostic != nil {
		diagnostics = append(diagnostics, *diagnostic)
	}
	if diagnostic := connectionUnitDiagnostic(connection, sourceNode, targetNode); diagnostic != nil {
		diagnostics = append(diagnostics, *diagnostic)
	}
	return diagnostics, nil
}

func validateConnectionUnitConversion(connection model.Connection) error {
	conversion := connection.UnitConversion
	if conversion == nil {
		return nil
	}
	mode := strings.TrimSpace(conversion.Mode)
	if mode == "" {
		mode = "linear"
	}
	if mode != "linear" {
		return fmt.Errorf("connection %s unit_conversion mode is unsupported: %s", connection.ID, mode)
	}
	if conversion.Factor != nil {
		factor := *conversion.Factor
		if math.IsNaN(factor) || math.IsInf(factor, 0) {
			return fmt.Errorf("connection %s unit_conversion factor must be finite", connection.ID)
		}
		if factor == 0 {
			return fmt.Errorf("connection %s unit_conversion factor must not be 0", connection.ID)
		}
	}
	if conversion.Offset != nil {
		offset := *conversion.Offset
		if math.IsNaN(offset) || math.IsInf(offset, 0) {
			return fmt.Errorf("connection %s unit_conversion offset must be finite", connection.ID)
		}
	}
	return nil
}

func connectionMediumDiagnostic(connection model.Connection, sourceNode model.Node, targetNode model.Node) (*Diagnostic, error) {
	if compatibleMedium(sourceNode.Medium, targetNode.Medium) {
		return nil, nil
	}
	message := fmt.Sprintf(
		"connection %s medium mismatch: %s.%s=%s -> %s.%s=%s",
		connection.ID,
		connection.From.Component,
		connection.From.Node,
		sourceNode.Medium,
		connection.To.Component,
		connection.To.Node,
		targetNode.Medium,
	)
	if connection.AllowMediumMismatch {
		return &Diagnostic{
			Severity:     "warning",
			Message:      message + " allowed by explicit medium override",
			ConnectionID: connection.ID,
			From:         connection.From,
			To:           connection.To,
		}, nil
	}
	if normalizedMedium(sourceNode.Medium) == "signal" && normalizedMedium(targetNode.Medium) != "signal" {
		return &Diagnostic{
			Severity:     "warning",
			Message:      message,
			ConnectionID: connection.ID,
			From:         connection.From,
			To:           connection.To,
		}, nil
	}
	return nil, fmt.Errorf(
		"connection %s medium mismatch: %s.%s=%s -> %s.%s=%s",
		connection.ID,
		connection.From.Component,
		connection.From.Node,
		sourceNode.Medium,
		connection.To.Component,
		connection.To.Node,
		targetNode.Medium,
	)
}

func connectionUnitDiagnostic(connection model.Connection, sourceNode model.Node, targetNode model.Node) *Diagnostic {
	sourceUnit := normalizedUnit(sourceNode.Unit)
	targetUnit := normalizedUnit(targetNode.Unit)
	if sourceUnit == "" || targetUnit == "" || sourceUnit == targetUnit || connection.UnitConversion != nil {
		return nil
	}
	return &Diagnostic{
		Severity: "warning",
		Message: fmt.Sprintf(
			"connection %s unit mismatch without conversion: %s.%s=%s -> %s.%s=%s",
			connection.ID,
			connection.From.Component,
			connection.From.Node,
			sourceNode.Unit,
			connection.To.Component,
			connection.To.Node,
			targetNode.Unit,
		),
		ConnectionID: connection.ID,
		From:         connection.From,
		To:           connection.To,
	}
}

func validatePublicIO(idx *graphindex.Index, plan *Plan) error {
	for _, input := range plan.System.PublicInputs {
		if input.ID == "" {
			return fmt.Errorf("public input id is required")
		}
		if _, exists := plan.PublicInputs[input.ID]; exists {
			return fmt.Errorf("duplicate public input id: %s", input.ID)
		}
		if !plan.SystemContains[input.Component] {
			return fmt.Errorf("public input %s references component outside system: %s", input.ID, input.Component)
		}
		if _, ok := idx.InputNode(input.Component, input.Node); !ok {
			return fmt.Errorf("public input %s references unknown input node: %s.%s", input.ID, input.Component, input.Node)
		}
		if _, hasIncoming := plan.Incoming[endpointKey(input.Component, input.Node)]; hasIncoming {
			return fmt.Errorf("public input %s targets an input node that already has an incoming connection: %s.%s", input.ID, input.Component, input.Node)
		}
		plan.PublicInputs[input.ID] = input
	}

	for _, output := range plan.System.PublicOutputs {
		if output.ID == "" {
			return fmt.Errorf("public output id is required")
		}
		if _, exists := plan.PublicOutputs[output.ID]; exists {
			return fmt.Errorf("duplicate public output id: %s", output.ID)
		}
		if !plan.SystemContains[output.Component] {
			return fmt.Errorf("public output %s references component outside system: %s", output.ID, output.Component)
		}
		if _, ok := idx.OutputNode(output.Component, output.Node); !ok {
			return fmt.Errorf("public output %s references unknown output node: %s.%s", output.ID, output.Component, output.Node)
		}
		plan.PublicOutputs[output.ID] = output
	}

	return nil
}

func validateCompositeBoundaries(idx *graphindex.Index, system model.System) error {
	return validateCompositeBoundariesRecursive(idx, system, []string{system.ID}, map[string]bool{})
}

func validateCompositeBoundariesRecursive(idx *graphindex.Index, system model.System, path []string, visited map[string]bool) error {
	if visited[system.ID] {
		return nil
	}
	visited[system.ID] = true
	for _, componentID := range system.Components {
		component := idx.Components[componentID]
		if component.Kind != "composite" {
			continue
		}
		if component.Composite == nil || strings.TrimSpace(component.Composite.System) == "" {
			return fmt.Errorf("component %s kind composite requires composite.system", component.ID)
		}
		childSystemID := strings.TrimSpace(component.Composite.System)
		if systemPathContains(path, childSystemID) {
			return fmt.Errorf("component %s composite system recursion detected: %s -> %s", component.ID, strings.Join(path, " -> "), childSystemID)
		}
		child, ok := idx.Systems[childSystemID]
		if !ok {
			return fmt.Errorf("component %s references unknown composite system: %s", component.ID, childSystemID)
		}
		if err := validateCompositePublicInputs(component, child); err != nil {
			return err
		}
		if err := validateCompositePublicOutputs(component, child); err != nil {
			return err
		}
		if err := validateCompositeBoundariesRecursive(idx, child, appendSystemPath(path, childSystemID), visited); err != nil {
			return err
		}
	}
	return nil
}

func validateCompositePublicInputs(component model.Component, child model.System) error {
	nodes := inputNodeSet(component)
	for _, publicInput := range child.PublicInputs {
		if !nodes[publicInput.ID] {
			return fmt.Errorf("component %s composite input node missing child public input: %s", component.ID, publicInput.ID)
		}
	}
	public := publicInputSet(child)
	for _, node := range component.Nodes.Inputs {
		if !public[node.ID] {
			return fmt.Errorf("component %s composite input node has no child public input: %s", component.ID, node.ID)
		}
	}
	return nil
}

func validateCompositePublicOutputs(component model.Component, child model.System) error {
	nodes := outputNodeSet(component)
	for _, publicOutput := range child.PublicOutputs {
		if !nodes[publicOutput.ID] {
			return fmt.Errorf("component %s composite output node missing child public output: %s", component.ID, publicOutput.ID)
		}
	}
	public := publicOutputSet(child)
	for _, node := range component.Nodes.Outputs {
		if !public[node.ID] {
			return fmt.Errorf("component %s composite output node has no child public output: %s", component.ID, node.ID)
		}
	}
	return nil
}

func topologicalOrder(componentIDs []string, connections []model.Connection) ([]string, error) {
	componentSet := map[string]bool{}
	inDegree := map[string]int{}
	edges := map[string][]string{}

	for _, id := range componentIDs {
		componentSet[id] = true
		inDegree[id] = 0
	}

	for _, connection := range connections {
		from := connection.From.Component
		to := connection.To.Component
		if from == to {
			return nil, fmt.Errorf("algebraic loop detected: component connects to itself: %s", from)
		}
		if !componentSet[from] || !componentSet[to] {
			continue
		}
		edges[from] = append(edges[from], to)
		inDegree[to]++
	}

	queue := make([]string, 0, len(componentIDs))
	for _, id := range componentIDs {
		if inDegree[id] == 0 {
			queue = append(queue, id)
		}
	}

	order := make([]string, 0, len(componentIDs))
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		order = append(order, current)
		for _, next := range edges[current] {
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}

	if len(order) != len(componentIDs) {
		blocked := make([]string, 0)
		for _, id := range componentIDs {
			if inDegree[id] > 0 {
				blocked = append(blocked, id)
			}
		}
		return nil, fmt.Errorf(
			"algebraic loop detected among components: %s. Add a Delay component or wrap the feedback behavior in an explicit solver boundary component",
			strings.Join(blocked, ", "),
		)
	}

	return order, nil
}

func compatibleMedium(source string, target string) bool {
	source = normalizedMedium(source)
	target = normalizedMedium(target)
	if source == "" || target == "" {
		return true
	}
	if source == "generic" || target == "generic" {
		return true
	}
	return source == target
}

func normalizedMedium(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizedUnit(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func inputNodeSet(component model.Component) map[string]bool {
	nodes := map[string]bool{}
	for _, node := range component.Nodes.Inputs {
		nodes[node.ID] = true
	}
	return nodes
}

func outputNodeSet(component model.Component) map[string]bool {
	nodes := map[string]bool{}
	for _, node := range component.Nodes.Outputs {
		nodes[node.ID] = true
	}
	return nodes
}

func publicInputSet(system model.System) map[string]bool {
	inputs := map[string]bool{}
	for _, input := range system.PublicInputs {
		inputs[input.ID] = true
	}
	return inputs
}

func publicOutputSet(system model.System) map[string]bool {
	outputs := map[string]bool{}
	for _, output := range system.PublicOutputs {
		outputs[output.ID] = true
	}
	return outputs
}

func systemPathContains(path []string, systemID string) bool {
	for _, value := range path {
		if value == systemID {
			return true
		}
	}
	return false
}

func appendSystemPath(path []string, systemID string) []string {
	next := append([]string(nil), path...)
	return append(next, systemID)
}

func endpointKey(componentID string, nodeID string) string {
	return componentID + "\x00" + nodeID
}

func EndpointKey(componentID string, nodeID string) string {
	return endpointKey(componentID, nodeID)
}
