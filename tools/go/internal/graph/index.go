package graph

import (
	"fmt"
	"strings"

	"github.com/goniegonie/hvac-studio/tools/go/internal/model"
)

type Index struct {
	Systems     map[string]model.System
	Components  map[string]model.Component
	Connections map[string]model.Connection
}

func NewIndex(g *model.Graph) (*Index, error) {
	if strings.TrimSpace(g.SchemaVersion) == "" {
		return nil, fmt.Errorf("graph schema_version is required")
	}

	idx := &Index{
		Systems:     map[string]model.System{},
		Components:  map[string]model.Component{},
		Connections: map[string]model.Connection{},
	}

	for _, system := range g.Systems {
		if strings.TrimSpace(system.ID) == "" {
			return nil, fmt.Errorf("system id is required")
		}
		if _, exists := idx.Systems[system.ID]; exists {
			return nil, fmt.Errorf("duplicate system id: %s", system.ID)
		}
		if err := validateSystem(system); err != nil {
			return nil, err
		}
		idx.Systems[system.ID] = system
	}

	for _, component := range g.Components {
		if strings.TrimSpace(component.ID) == "" {
			return nil, fmt.Errorf("component id is required")
		}
		if _, exists := idx.Components[component.ID]; exists {
			return nil, fmt.Errorf("duplicate component id: %s", component.ID)
		}
		if err := validateComponent(component); err != nil {
			return nil, err
		}
		idx.Components[component.ID] = component
	}

	for _, connection := range g.Connections {
		if strings.TrimSpace(connection.ID) == "" {
			return nil, fmt.Errorf("connection id is required")
		}
		if _, exists := idx.Connections[connection.ID]; exists {
			return nil, fmt.Errorf("duplicate connection id: %s", connection.ID)
		}
		if err := validateConnectionShape(connection); err != nil {
			return nil, err
		}
		idx.Connections[connection.ID] = connection
	}

	return idx, nil
}

func validateSystem(system model.System) error {
	if err := validateUniqueIDs("system "+system.ID+" component reference", system.Components); err != nil {
		return err
	}
	if err := validateUniqueIDs("system "+system.ID+" connection reference", system.Connections); err != nil {
		return err
	}
	if err := validatePublicNodeRefs("system "+system.ID+" public input", system.PublicInputs); err != nil {
		return err
	}
	if err := validatePublicNodeRefs("system "+system.ID+" public output", system.PublicOutputs); err != nil {
		return err
	}
	return nil
}

func validateComponent(component model.Component) error {
	switch component.Kind {
	case "user_python", "builtin_go", "external_exe", "future_compiled":
	default:
		if strings.TrimSpace(component.Kind) == "" {
			return fmt.Errorf("component %s kind is required", component.ID)
		}
		return fmt.Errorf("component %s kind is invalid: %s", component.ID, component.Kind)
	}
	if err := validateNodes(component.ID, "input", component.Nodes.Inputs); err != nil {
		return err
	}
	if err := validateNodes(component.ID, "output", component.Nodes.Outputs); err != nil {
		return err
	}
	return nil
}

func validateNodes(componentID string, direction string, nodes []model.Node) error {
	seen := map[string]bool{}
	for _, node := range nodes {
		if strings.TrimSpace(node.ID) == "" {
			return fmt.Errorf("component %s %s node id is required", componentID, direction)
		}
		if seen[node.ID] {
			return fmt.Errorf("component %s duplicate %s node id: %s", componentID, direction, node.ID)
		}
		seen[node.ID] = true
		if strings.TrimSpace(node.Medium) == "" {
			return fmt.Errorf("component %s %s node %s medium is required", componentID, direction, node.ID)
		}
		if strings.TrimSpace(node.ValueType) == "" {
			return fmt.Errorf("component %s %s node %s value_type is required", componentID, direction, node.ID)
		}
	}
	return nil
}

func validateConnectionShape(connection model.Connection) error {
	if strings.TrimSpace(connection.From.Component) == "" {
		return fmt.Errorf("connection %s source component is required", connection.ID)
	}
	if strings.TrimSpace(connection.From.Node) == "" {
		return fmt.Errorf("connection %s source node is required", connection.ID)
	}
	if strings.TrimSpace(connection.To.Component) == "" {
		return fmt.Errorf("connection %s target component is required", connection.ID)
	}
	if strings.TrimSpace(connection.To.Node) == "" {
		return fmt.Errorf("connection %s target node is required", connection.ID)
	}
	return nil
}

func validatePublicNodeRefs(label string, refs []model.PublicNodeRef) error {
	seen := map[string]bool{}
	for _, ref := range refs {
		if strings.TrimSpace(ref.ID) == "" {
			return fmt.Errorf("%s id is required", label)
		}
		if seen[ref.ID] {
			return fmt.Errorf("duplicate %s id: %s", label, ref.ID)
		}
		seen[ref.ID] = true
		if strings.TrimSpace(ref.Component) == "" {
			return fmt.Errorf("%s %s component is required", label, ref.ID)
		}
		if strings.TrimSpace(ref.Node) == "" {
			return fmt.Errorf("%s %s node is required", label, ref.ID)
		}
	}
	return nil
}

func validateUniqueIDs(label string, ids []string) error {
	seen := map[string]bool{}
	for _, id := range ids {
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("%s id is required", label)
		}
		if seen[id] {
			return fmt.Errorf("duplicate %s id: %s", label, id)
		}
		seen[id] = true
	}
	return nil
}

func (i *Index) InputNode(componentID string, nodeID string) (model.Node, bool) {
	component, ok := i.Components[componentID]
	if !ok {
		return model.Node{}, false
	}
	for _, node := range component.Nodes.Inputs {
		if node.ID == nodeID {
			return node, true
		}
	}
	return model.Node{}, false
}

func (i *Index) OutputNode(componentID string, nodeID string) (model.Node, bool) {
	component, ok := i.Components[componentID]
	if !ok {
		return model.Node{}, false
	}
	for _, node := range component.Nodes.Outputs {
		if node.ID == nodeID {
			return node, true
		}
	}
	return model.Node{}, false
}
