package graph

import (
	"fmt"

	"github.com/goniegonie/hvac-studio/tools/go/internal/model"
)

type Index struct {
	Systems     map[string]model.System
	Components  map[string]model.Component
	Connections map[string]model.Connection
}

func NewIndex(g *model.Graph) (*Index, error) {
	idx := &Index{
		Systems:     map[string]model.System{},
		Components:  map[string]model.Component{},
		Connections: map[string]model.Connection{},
	}

	for _, system := range g.Systems {
		if system.ID == "" {
			return nil, fmt.Errorf("system id is required")
		}
		if _, exists := idx.Systems[system.ID]; exists {
			return nil, fmt.Errorf("duplicate system id: %s", system.ID)
		}
		idx.Systems[system.ID] = system
	}

	for _, component := range g.Components {
		if component.ID == "" {
			return nil, fmt.Errorf("component id is required")
		}
		if _, exists := idx.Components[component.ID]; exists {
			return nil, fmt.Errorf("duplicate component id: %s", component.ID)
		}
		idx.Components[component.ID] = component
	}

	for _, connection := range g.Connections {
		if connection.ID == "" {
			return nil, fmt.Errorf("connection id is required")
		}
		if _, exists := idx.Connections[connection.ID]; exists {
			return nil, fmt.Errorf("duplicate connection id: %s", connection.ID)
		}
		idx.Connections[connection.ID] = connection
	}

	return idx, nil
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
