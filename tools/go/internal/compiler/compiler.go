package compiler

import (
	"fmt"
	"strings"

	graphindex "github.com/goniegonie/hvac-studio/tools/go/internal/graph"
	"github.com/goniegonie/hvac-studio/tools/go/internal/model"
	"github.com/goniegonie/hvac-studio/tools/go/internal/project"
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

	order, err := topologicalOrder(system.Components, selectedConnections)
	if err != nil {
		return nil, err
	}
	plan.Order = order

	return plan, nil
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

	diagnostic, err := connectionMediumDiagnostic(connection, sourceNode, targetNode)
	if err != nil {
		return nil, err
	}
	if diagnostic == nil {
		return nil, nil
	}
	return []Diagnostic{*diagnostic}, nil
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
			"algebraic loop detected among components: %s. Add a Delay component, define a solver boundary, or mark the system as iterative",
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

func endpointKey(componentID string, nodeID string) string {
	return componentID + "\x00" + nodeID
}

func EndpointKey(componentID string, nodeID string) string {
	return endpointKey(componentID, nodeID)
}
