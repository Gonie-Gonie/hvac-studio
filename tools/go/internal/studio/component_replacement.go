package studio

import (
	"fmt"
	"sort"
	"strings"

	"github.com/goniegonie/hvac-studio/tools/go/internal/apperror"
	"github.com/goniegonie/hvac-studio/tools/go/internal/model"
	"github.com/goniegonie/hvac-studio/tools/go/internal/project"
)

func replaceComponent(loaded *project.LoadedProject, req replaceComponentRequest, repoRoot string) (model.Component, ComponentReplacementSummary, []Problem, error) {
	componentID := strings.TrimSpace(req.ComponentID)
	if componentID == "" {
		return model.Component{}, ComponentReplacementSummary{}, nil, apperror.Errorf(apperror.CodeValidation, "component_id is required")
	}
	source, found := findComponent(loaded.Graph, componentID)
	if !found {
		return model.Component{}, ComponentReplacementSummary{}, nil, apperror.Errorf(apperror.CodeValidation, "component not found: %s", componentID)
	}
	replacementName := strings.TrimSpace(req.Name)
	if replacementName == "" {
		replacementName = strings.TrimSpace(source.Name) + " Replacement"
	}
	if strings.TrimSpace(replacementName) == "" {
		replacementName = componentID + " Replacement"
	}
	template := strings.TrimSpace(req.Template)
	if template == "" {
		template = "scalar"
	}
	mapParameters := boolOption(req.MapParameters, true)
	replacement, err := createComponent(loaded, createComponentRequest{
		ProjectPath: loaded.Path,
		Name:        replacementName,
		Template:    template,
	}, repoRoot)
	if err != nil {
		return model.Component{}, ComponentReplacementSummary{}, nil, err
	}
	summary := replacementSummary(loaded, source, replacement, template, mapParameters)
	if len(summary.Problems) > 0 {
		_ = rollbackReplacementComponent(loaded, replacement)
		return model.Component{}, summary, summary.Problems, apperror.Errorf(apperror.CodeValidation, "replacement component is not contract-compatible: %s", strings.Join(problemMessages(summary.Problems), "; "))
	}
	updatedReplacement, parameterMappings, mappedParameters := applyReplacementParameterMapping(loaded, source, replacement, mapParameters)
	replacement = updatedReplacement
	summary.ParameterMappings = parameterMappings
	summary.MappedParameters = mappedParameters
	if err := syncReplacementComponent(loaded, replacement); err != nil {
		_ = rollbackReplacementComponent(loaded, replacement)
		return model.Component{}, summary, nil, err
	}
	if err := rewireReplacementComponent(loaded, source, replacement, &summary); err != nil {
		_ = rollbackReplacementComponent(loaded, replacement)
		return model.Component{}, summary, nil, err
	}
	return replacement, summary, nil, nil
}

func rewireReplacementComponent(loaded *project.LoadedProject, source model.Component, replacement model.Component, summary *ComponentReplacementSummary) error {
	systemIndex := entrySystemIndex(loaded)
	if systemIndex < 0 {
		return apperror.Errorf(apperror.CodeValidation, "entry system not found: %s", loaded.Project.EntrySystem)
	}
	system := &loaded.Graph.Systems[systemIndex]
	if !containsString(system.Components, source.ID) {
		return nil
	}
	for index, componentID := range system.Components {
		if componentID == source.ID {
			system.Components[index] = replacement.ID
			summary.SystemReplaced = true
		}
	}
	for index := range system.PublicInputs {
		if system.PublicInputs[index].Component == source.ID {
			system.PublicInputs[index].Component = replacement.ID
			summary.RewiredPublicInputs++
		}
	}
	for index := range system.PublicOutputs {
		if system.PublicOutputs[index].Component == source.ID {
			system.PublicOutputs[index].Component = replacement.ID
			summary.RewiredPublicOutputs++
		}
	}
	for index := range loaded.Graph.Connections {
		connection := &loaded.Graph.Connections[index]
		if !containsString(system.Connections, connection.ID) {
			continue
		}
		rewired := false
		if connection.From.Component == source.ID {
			connection.From.Component = replacement.ID
			rewired = true
		}
		if connection.To.Component == source.ID {
			connection.To.Component = replacement.ID
			rewired = true
		}
		if rewired {
			summary.RewiredConnections++
		}
	}
	if err := writeJSONFile(loaded.GraphPath, loaded.Graph); err != nil {
		return err
	}
	return nil
}

func replacementSummary(loaded *project.LoadedProject, source model.Component, replacement model.Component, template string, mapParameters bool) ComponentReplacementSummary {
	summary := ComponentReplacementSummary{
		OriginalComponent:         source.ID,
		ReplacementComponent:      replacement.ID,
		Template:                  template,
		OriginalComponentRetained: true,
		MapParameters:             mapParameters,
		Diff:                      componentReplacementDiff(source, replacement),
	}
	systemIndex := entrySystemIndex(loaded)
	if systemIndex < 0 {
		summary.Problems = []Problem{{
			Severity: "error",
			Message:  fmt.Sprintf("entry system not found: %s", loaded.Project.EntrySystem),
		}}
		return summary
	}
	system := loaded.Graph.Systems[systemIndex]
	summary.NodeMappings = replacementNodeMappings(system, loaded.Graph, source, replacement)
	if containsString(system.Components, source.ID) {
		summary.Problems = replacementCompatibilityProblems(system, loaded.Graph, source, replacement)
	}
	return summary
}

func replacementCompatibilityProblems(system model.System, graph *model.Graph, source model.Component, replacement model.Component) []Problem {
	problems := []Problem{}
	for _, input := range system.PublicInputs {
		if input.Component == source.ID && !componentHasInputNode(replacement, input.Node) {
			problems = append(problems, Problem{Severity: "error", ComponentID: source.ID, NodeID: input.Node, Message: fmt.Sprintf("replacement missing input node for public input %s: %s", input.ID, input.Node)})
		}
	}
	for _, output := range system.PublicOutputs {
		if output.Component == source.ID && !componentHasOutputNode(replacement, output.Node) {
			problems = append(problems, Problem{Severity: "error", ComponentID: source.ID, NodeID: output.Node, Message: fmt.Sprintf("replacement missing output node for public output %s: %s", output.ID, output.Node)})
		}
	}
	for _, connectionID := range system.Connections {
		connection, found := findConnection(graph, connectionID)
		if !found {
			continue
		}
		if connection.From.Component == source.ID && !componentHasOutputNode(replacement, connection.From.Node) {
			problems = append(problems, Problem{Severity: "error", ComponentID: source.ID, NodeID: connection.From.Node, Message: fmt.Sprintf("replacement missing output node for connection %s: %s", connection.ID, connection.From.Node)})
		}
		if connection.To.Component == source.ID && !componentHasInputNode(replacement, connection.To.Node) {
			problems = append(problems, Problem{Severity: "error", ComponentID: source.ID, NodeID: connection.To.Node, Message: fmt.Sprintf("replacement missing input node for connection %s: %s", connection.ID, connection.To.Node)})
		}
	}
	return problems
}

func replacementNodeMappings(system model.System, graph *model.Graph, source model.Component, replacement model.Component) []ComponentReplacementMapping {
	mappings := []ComponentReplacementMapping{}
	for _, input := range system.PublicInputs {
		if input.Component != source.ID {
			continue
		}
		status := "preserved"
		detail := "public input"
		if !componentHasInputNode(replacement, input.Node) {
			status = "missing"
			detail = "replacement input node is missing"
		}
		mappings = append(mappings, ComponentReplacementMapping{
			Scope:  "public_input",
			ID:     input.ID,
			From:   endpointLabel(source.ID, input.Node),
			To:     endpointLabel(replacement.ID, input.Node),
			Status: status,
			Detail: detail,
		})
	}
	for _, output := range system.PublicOutputs {
		if output.Component != source.ID {
			continue
		}
		status := "preserved"
		detail := "public output"
		if !componentHasOutputNode(replacement, output.Node) {
			status = "missing"
			detail = "replacement output node is missing"
		}
		mappings = append(mappings, ComponentReplacementMapping{
			Scope:  "public_output",
			ID:     output.ID,
			From:   endpointLabel(source.ID, output.Node),
			To:     endpointLabel(replacement.ID, output.Node),
			Status: status,
			Detail: detail,
		})
	}
	for _, connectionID := range system.Connections {
		connection, found := findConnection(graph, connectionID)
		if !found {
			continue
		}
		if connection.From.Component == source.ID {
			status := "preserved"
			detail := "connection source"
			if !componentHasOutputNode(replacement, connection.From.Node) {
				status = "missing"
				detail = "replacement output node is missing"
			}
			mappings = append(mappings, ComponentReplacementMapping{
				Scope:  "connection_output",
				ID:     connection.ID,
				From:   endpointLabel(source.ID, connection.From.Node),
				To:     endpointLabel(replacement.ID, connection.From.Node),
				Status: status,
				Detail: detail,
			})
		}
		if connection.To.Component == source.ID {
			status := "preserved"
			detail := "connection target"
			if !componentHasInputNode(replacement, connection.To.Node) {
				status = "missing"
				detail = "replacement input node is missing"
			}
			mappings = append(mappings, ComponentReplacementMapping{
				Scope:  "connection_input",
				ID:     connection.ID,
				From:   endpointLabel(source.ID, connection.To.Node),
				To:     endpointLabel(replacement.ID, connection.To.Node),
				Status: status,
				Detail: detail,
			})
		}
	}
	return mappings
}

func applyReplacementParameterMapping(loaded *project.LoadedProject, source model.Component, replacement model.Component, mapParameters bool) (model.Component, []ComponentReplacementMapping, int) {
	mappings := []ComponentReplacementMapping{}
	mapped := 0
	for _, name := range componentParameterIDs(replacement) {
		sourceValue, hasSourceValue := source.Parameters[name]
		status := "missing"
		detail := "source parameter is not present"
		if !mapParameters {
			status = "skipped"
			detail = "parameter mapping disabled"
		} else if hasSourceValue {
			if replacement.Parameters == nil {
				replacement.Parameters = map[string]any{}
			}
			replacement.Parameters[name] = sourceValue
			if replacement.ParameterDefinitions != nil {
				definition := replacement.ParameterDefinitions[name]
				definition.Current = sourceValue
				replacement.ParameterDefinitions[name] = definition
			}
			status = "copied"
			detail = "same-name parameter value copied"
			mapped++
		}
		mappings = append(mappings, ComponentReplacementMapping{
			Scope:  "parameter",
			ID:     name,
			From:   source.ID + "." + name,
			To:     replacement.ID + "." + name,
			Status: status,
			Detail: detail,
		})
	}
	return replacement, mappings, mapped
}

func syncReplacementComponent(loaded *project.LoadedProject, replacement model.Component) error {
	for index := range loaded.Graph.Components {
		if loaded.Graph.Components[index].ID == replacement.ID {
			loaded.Graph.Components[index] = replacement
			if err := syncComponentMetadataFile(loaded, replacement); err != nil {
				return err
			}
			return writeJSONFile(loaded.GraphPath, loaded.Graph)
		}
	}
	return apperror.Errorf(apperror.CodeValidation, "replacement component not found after creation: %s", replacement.ID)
}

func componentReplacementDiff(source model.Component, replacement model.Component) ComponentReplacementDiff {
	sourceInputs := inputNodeIDs(source)
	replacementInputs := inputNodeIDs(replacement)
	sourceOutputs := outputNodeIDs(source)
	replacementOutputs := outputNodeIDs(replacement)
	sourceParameters := componentParameterIDs(source)
	replacementParameters := componentParameterIDs(replacement)
	return ComponentReplacementDiff{
		OriginalInputs:        sourceInputs,
		ReplacementInputs:     replacementInputs,
		MatchedInputs:         intersectStrings(sourceInputs, replacementInputs),
		MissingInputs:         differenceStrings(sourceInputs, replacementInputs),
		AddedInputs:           differenceStrings(replacementInputs, sourceInputs),
		OriginalOutputs:       sourceOutputs,
		ReplacementOutputs:    replacementOutputs,
		MatchedOutputs:        intersectStrings(sourceOutputs, replacementOutputs),
		MissingOutputs:        differenceStrings(sourceOutputs, replacementOutputs),
		AddedOutputs:          differenceStrings(replacementOutputs, sourceOutputs),
		OriginalParameters:    sourceParameters,
		ReplacementParameters: replacementParameters,
		MatchedParameters:     intersectStrings(sourceParameters, replacementParameters),
		MissingParameters:     differenceStrings(sourceParameters, replacementParameters),
		AddedParameters:       differenceStrings(replacementParameters, sourceParameters),
	}
}

func inputNodeIDs(component model.Component) []string {
	ids := make([]string, 0, len(component.Nodes.Inputs))
	for _, node := range component.Nodes.Inputs {
		ids = append(ids, node.ID)
	}
	sort.Strings(ids)
	return ids
}

func outputNodeIDs(component model.Component) []string {
	ids := make([]string, 0, len(component.Nodes.Outputs))
	for _, node := range component.Nodes.Outputs {
		ids = append(ids, node.ID)
	}
	sort.Strings(ids)
	return ids
}

func componentParameterIDs(component model.Component) []string {
	seen := map[string]bool{}
	for name := range component.Parameters {
		if strings.TrimSpace(name) != "" {
			seen[name] = true
		}
	}
	for name := range component.ParameterDefinitions {
		if strings.TrimSpace(name) != "" {
			seen[name] = true
		}
	}
	ids := make([]string, 0, len(seen))
	for name := range seen {
		ids = append(ids, name)
	}
	sort.Strings(ids)
	return ids
}

func intersectStrings(left []string, right []string) []string {
	rightSet := map[string]bool{}
	for _, value := range right {
		rightSet[value] = true
	}
	out := []string{}
	for _, value := range left {
		if rightSet[value] {
			out = append(out, value)
		}
	}
	return out
}

func differenceStrings(left []string, right []string) []string {
	rightSet := map[string]bool{}
	for _, value := range right {
		rightSet[value] = true
	}
	out := []string{}
	for _, value := range left {
		if !rightSet[value] {
			out = append(out, value)
		}
	}
	return out
}

func endpointLabel(componentID string, nodeID string) string {
	return componentID + "." + nodeID
}

func problemMessages(problems []Problem) []string {
	messages := make([]string, 0, len(problems))
	for _, problem := range problems {
		if strings.TrimSpace(problem.Message) != "" {
			messages = append(messages, problem.Message)
		}
	}
	return messages
}

func rollbackReplacementComponent(loaded *project.LoadedProject, replacement model.Component) error {
	copiedPath, pathErr := componentSourceArtifactPath(loaded, replacement)
	kept := loaded.Graph.Components[:0]
	for _, component := range loaded.Graph.Components {
		if component.ID != replacement.ID {
			kept = append(kept, component)
		}
	}
	loaded.Graph.Components = kept
	for index := range loaded.Graph.Systems {
		loaded.Graph.Systems[index].Components = removeString(loaded.Graph.Systems[index].Components, replacement.ID)
	}
	if err := writeJSONFile(loaded.GraphPath, loaded.Graph); err != nil {
		return err
	}
	if pathErr == nil {
		_ = removeComponentSourceArtifact(copiedPath, replacement.Source.Layout)
	}
	return nil
}
