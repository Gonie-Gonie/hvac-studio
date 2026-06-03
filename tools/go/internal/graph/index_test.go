package graph

import (
	"strings"
	"testing"

	"github.com/goniegonie/hvac-studio/tools/go/internal/model"
)

func TestNewIndexRejectsMissingGraphSchemaVersion(t *testing.T) {
	_, err := NewIndex(&model.Graph{})
	if err == nil || !strings.Contains(err.Error(), "graph schema_version is required") {
		t.Fatalf("error = %v", err)
	}
}

func TestNewIndexRejectsDuplicateSystemComponentReference(t *testing.T) {
	graph := validGraph()
	graph.Systems[0].Components = []string{"gain", "gain"}

	_, err := NewIndex(graph)

	if err == nil || !strings.Contains(err.Error(), "duplicate system MainSystem component reference id: gain") {
		t.Fatalf("error = %v", err)
	}
}

func TestNewIndexRejectsDuplicateComponentNode(t *testing.T) {
	graph := validGraph()
	graph.Components[0].Nodes.Inputs = append(graph.Components[0].Nodes.Inputs, graph.Components[0].Nodes.Inputs[0])

	_, err := NewIndex(graph)

	if err == nil || !strings.Contains(err.Error(), "component gain duplicate input node id: value") {
		t.Fatalf("error = %v", err)
	}
}

func TestNewIndexRejectsMissingConnectionEndpoint(t *testing.T) {
	graph := validGraph()
	graph.Connections = []model.Connection{
		{ID: "broken", From: model.Endpoint{Component: "gain"}, To: model.Endpoint{Component: "gain", Node: "value"}},
	}
	graph.Systems[0].Connections = []string{"broken"}

	_, err := NewIndex(graph)

	if err == nil || !strings.Contains(err.Error(), "connection broken source node is required") {
		t.Fatalf("error = %v", err)
	}
}

func TestNewIndexRejectsDuplicatePublicInput(t *testing.T) {
	graph := validGraph()
	graph.Systems[0].PublicInputs = append(graph.Systems[0].PublicInputs, graph.Systems[0].PublicInputs[0])

	_, err := NewIndex(graph)

	if err == nil || !strings.Contains(err.Error(), "duplicate system MainSystem public input id: value") {
		t.Fatalf("error = %v", err)
	}
}

func validGraph() *model.Graph {
	return &model.Graph{
		SchemaVersion: "0.1.0",
		Systems: []model.System{
			{
				ID:          "MainSystem",
				Components:  []string{"gain"},
				Connections: []string{},
				PublicInputs: []model.PublicNodeRef{
					{ID: "value", Component: "gain", Node: "value"},
				},
				PublicOutputs: []model.PublicNodeRef{
					{ID: "result", Component: "gain", Node: "result"},
				},
			},
		},
		Components: []model.Component{
			{
				ID:   "gain",
				Kind: "user_python",
				Nodes: model.NodeSet{
					Inputs: []model.Node{
						{ID: "value", Medium: "signal", ValueType: "float"},
					},
					Outputs: []model.Node{
						{ID: "result", Medium: "signal", ValueType: "float"},
					},
				},
			},
		},
		Connections: []model.Connection{},
	}
}
