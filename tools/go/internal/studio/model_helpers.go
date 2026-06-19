package studio

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/goniegonie/hvac-studio/tools/go/internal/model"
	"github.com/goniegonie/hvac-studio/tools/go/internal/project"
)

func slugify(value string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(value) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash && b.Len() > 0 {
			b.WriteRune('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func uniqueComponentID(graph *model.Graph, base string) string {
	base = strings.Trim(base, "_")
	if base == "" {
		return ""
	}
	exists := map[string]bool{}
	for _, component := range graph.Components {
		exists[component.ID] = true
	}
	candidate := base
	for index := 2; exists[candidate]; index++ {
		candidate = fmt.Sprintf("%s_%d", base, index)
	}
	return candidate
}

func uniqueConnectionID(graph *model.Graph, base string) string {
	base = strings.ReplaceAll(slugify(base), "-", "_")
	if base == "" {
		base = "connection"
	}
	exists := map[string]bool{}
	for _, connection := range graph.Connections {
		exists[connection.ID] = true
	}
	candidate := base
	for index := 2; exists[candidate]; index++ {
		candidate = fmt.Sprintf("%s_%d", base, index)
	}
	return candidate
}

func findComponent(graph *model.Graph, componentID string) (model.Component, bool) {
	if graph == nil {
		return model.Component{}, false
	}
	for _, component := range graph.Components {
		if component.ID == componentID {
			return component, true
		}
	}
	return model.Component{}, false
}

func findSystem(graph *model.Graph, systemID string) (model.System, bool) {
	if graph == nil {
		return model.System{}, false
	}
	for _, system := range graph.Systems {
		if system.ID == systemID {
			return system, true
		}
	}
	return model.System{}, false
}

func findConnection(graph *model.Graph, connectionID string) (model.Connection, bool) {
	for _, connection := range graph.Connections {
		if connection.ID == connectionID {
			return connection, true
		}
	}
	return model.Connection{}, false
}

func findInputNode(component model.Component, nodeID string) (model.Node, bool) {
	for _, node := range component.Nodes.Inputs {
		if node.ID == nodeID {
			return node, true
		}
	}
	return model.Node{}, false
}

func removeNodeFromComponent(component *model.Component, nodeID string) (model.Node, bool, bool) {
	for index, node := range component.Nodes.Inputs {
		if node.ID == nodeID {
			component.Nodes.Inputs = append(component.Nodes.Inputs[:index], component.Nodes.Inputs[index+1:]...)
			return node, true, true
		}
	}
	for index, node := range component.Nodes.Outputs {
		if node.ID == nodeID {
			component.Nodes.Outputs = append(component.Nodes.Outputs[:index], component.Nodes.Outputs[index+1:]...)
			return node, false, true
		}
	}
	return model.Node{}, false, false
}

func componentHasNode(component model.Component, nodeID string) bool {
	for _, node := range component.Nodes.Inputs {
		if node.ID == nodeID {
			return true
		}
	}
	for _, node := range component.Nodes.Outputs {
		if node.ID == nodeID {
			return true
		}
	}
	return false
}

func componentHasInputNode(component model.Component, nodeID string) bool {
	for _, node := range component.Nodes.Inputs {
		if node.ID == nodeID {
			return true
		}
	}
	return false
}

func componentHasOutputNode(component model.Component, nodeID string) bool {
	for _, node := range component.Nodes.Outputs {
		if node.ID == nodeID {
			return true
		}
	}
	return false
}

func isIdentifierLike(value string) bool {
	if value == "" {
		return false
	}
	for index, r := range value {
		if index == 0 {
			if r != '_' && !unicode.IsLetter(r) {
				return false
			}
			continue
		}
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func removeString(values []string, target string) []string {
	kept := values[:0]
	for _, value := range values {
		if value == target {
			continue
		}
		kept = append(kept, value)
	}
	return kept
}

func endpointMatches(endpoint model.Endpoint, componentID string, nodeID string) bool {
	return endpoint.Component == componentID && endpoint.Node == nodeID
}

func graphReferencesConnection(systems []model.System, connectionID string) bool {
	for _, system := range systems {
		if containsString(system.Connections, connectionID) {
			return true
		}
	}
	return false
}

func systemHasIncomingConnection(system model.System, graph *model.Graph, componentID string, nodeID string) bool {
	for _, connectionID := range system.Connections {
		for _, connection := range graph.Connections {
			if connection.ID == connectionID && connection.To.Component == componentID && connection.To.Node == nodeID {
				return true
			}
		}
	}
	return false
}

func entrySystem(loaded *project.LoadedProject) model.System {
	index := entrySystemIndex(loaded)
	if index < 0 {
		return model.System{}
	}
	return loaded.Graph.Systems[index]
}

func entrySystemIndex(loaded *project.LoadedProject) int {
	if loaded == nil || loaded.Graph == nil || loaded.Project == nil {
		return -1
	}
	for index, system := range loaded.Graph.Systems {
		if system.ID == loaded.Project.EntrySystem {
			return index
		}
	}
	return -1
}

func hasPublicInputFor(system model.System, componentID string, nodeID string) bool {
	for _, input := range system.PublicInputs {
		if input.Component == componentID && input.Node == nodeID {
			return true
		}
	}
	return false
}

func hasPublicOutputFor(system model.System, componentID string, nodeID string) bool {
	for _, output := range system.PublicOutputs {
		if output.Component == componentID && output.Node == nodeID {
			return true
		}
	}
	return false
}

func removePublicOutputsFor(system *model.System, componentID string, nodeID string) {
	kept := system.PublicOutputs[:0]
	for _, output := range system.PublicOutputs {
		if output.Component == componentID && output.Node == nodeID {
			continue
		}
		kept = append(kept, output)
	}
	system.PublicOutputs = kept
}

func removePublicInputsFor(system *model.System, componentID string, nodeID string) []string {
	removed := []string{}
	kept := system.PublicInputs[:0]
	for _, input := range system.PublicInputs {
		if input.Component == componentID && input.Node == nodeID {
			removed = append(removed, input.ID)
			continue
		}
		kept = append(kept, input)
	}
	system.PublicInputs = kept
	return removed
}

func removePublicInputsForComponent(system *model.System, componentID string) []string {
	removed := []string{}
	kept := system.PublicInputs[:0]
	for _, input := range system.PublicInputs {
		if input.Component == componentID {
			removed = append(removed, input.ID)
			continue
		}
		kept = append(kept, input)
	}
	system.PublicInputs = kept
	return removed
}

func removePublicOutputsForComponent(system *model.System, componentID string) {
	kept := system.PublicOutputs[:0]
	for _, output := range system.PublicOutputs {
		if output.Component == componentID {
			continue
		}
		kept = append(kept, output)
	}
	system.PublicOutputs = kept
}

func removeUnreferencedConnections(connections []model.Connection, systems []model.System) []model.Connection {
	kept := connections[:0]
	for _, connection := range connections {
		if graphReferencesConnection(systems, connection.ID) {
			kept = append(kept, connection)
		}
	}
	return kept
}

func uniquePublicNodeID(refs []model.PublicNodeRef, base string) string {
	exists := map[string]bool{}
	for _, ref := range refs {
		exists[ref.ID] = true
	}
	candidate := base
	for index := 2; exists[candidate]; index++ {
		candidate = fmt.Sprintf("%s_%d", base, index)
	}
	return candidate
}

func normalizeColumnName(value string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(value) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func updatePublicNodeRef(ref *model.PublicNodeRef, node model.Node) {
	ref.Node = node.ID
	ref.Name = node.Name
	ref.Medium = node.Medium
	ref.ValueType = node.ValueType
	ref.Unit = node.Unit
	ref.Required = node.Required
	ref.Default = node.Default
}

func defaultValueForNode(node model.Node) any {
	if node.Default != nil {
		return node.Default
	}
	switch node.ValueType {
	case "int", "integer":
		return 0
	case "bool", "boolean":
		return false
	case "string":
		return ""
	default:
		return 0.0
	}
}

func pythonClassName(componentID string) string {
	var b strings.Builder
	capitalizeNext := true
	for _, r := range componentID {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if b.Len() == 0 && unicode.IsDigit(r) {
				b.WriteRune('C')
			}
			if capitalizeNext {
				b.WriteRune(unicode.ToUpper(r))
				capitalizeNext = false
			} else {
				b.WriteRune(r)
			}
			continue
		}
		capitalizeNext = true
	}
	if b.Len() == 0 {
		return "UserComponent"
	}
	b.WriteString("Component")
	return b.String()
}

func displayNameFromID(id string) string {
	parts := strings.FieldsFunc(id, func(r rune) bool {
		return r == '_' || r == '-' || r == ' '
	})
	for index, part := range parts {
		if part == "" {
			continue
		}
		runes := []rune(part)
		runes[0] = unicode.ToUpper(runes[0])
		parts[index] = string(runes)
	}
	name := strings.Join(parts, " ")
	if name == "" {
		return id
	}
	return name
}

func cloneMap(values map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range values {
		out[key] = value
	}
	return out
}

func cloneComponentSource(value model.ComponentSource) model.ComponentSource {
	return model.ComponentSource{
		Layout:   value.Layout,
		Metadata: value.Metadata,
		Init:     value.Init,
		Step:     value.Step,
		Helpers:  value.Helpers,
		Wrapper:  value.Wrapper,
	}
}

func cloneParameterDefinitions(values map[string]model.ParameterDefinition) map[string]model.ParameterDefinition {
	if len(values) == 0 {
		return nil
	}
	out := map[string]model.ParameterDefinition{}
	for key, value := range values {
		if value.Bounds != nil {
			bounds := *value.Bounds
			value.Bounds = &bounds
		}
		if value.Visible != nil {
			visible := *value.Visible
			value.Visible = &visible
		}
		out[key] = value
	}
	return out
}

func cloneStateDefinitions(values map[string]model.StateDefinition) map[string]model.StateDefinition {
	if len(values) == 0 {
		return nil
	}
	out := map[string]model.StateDefinition{}
	for key, value := range values {
		out[key] = value
	}
	return out
}

func cloneSolverBoundary(value *model.SolverBoundary) *model.SolverBoundary {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneMLMetadata(value *model.MLMetadata) *model.MLMetadata {
	if value == nil {
		return nil
	}
	cloned := *value
	cloned.RequiredPackages = append([]string{}, value.RequiredPackages...)
	if value.ValidInputRanges != nil {
		cloned.ValidInputRanges = map[string]model.ValueBounds{}
		for key, bounds := range value.ValidInputRanges {
			cloned.ValidInputRanges[key] = bounds
		}
	}
	return &cloned
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
