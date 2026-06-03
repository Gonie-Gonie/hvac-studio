package compiler

import (
	"strings"
	"testing"

	"github.com/goniegonie/hvac-studio/tools/go/internal/model"
	"github.com/goniegonie/hvac-studio/tools/go/internal/project"
)

func TestCompileDetectsAlgebraicLoop(t *testing.T) {
	loaded := &project.LoadedProject{
		Project: &model.Project{EntrySystem: "MainSystem"},
		Graph: &model.Graph{
			SchemaVersion: "0.1.0",
			Systems: []model.System{
				{
					ID:          "MainSystem",
					Components:  []string{"a", "b"},
					Connections: []string{"a_to_b", "b_to_a"},
				},
			},
			Components: []model.Component{
				component("a"),
				component("b"),
			},
			Connections: []model.Connection{
				{ID: "a_to_b", From: model.Endpoint{Component: "a", Node: "out"}, To: model.Endpoint{Component: "b", Node: "in"}},
				{ID: "b_to_a", From: model.Endpoint{Component: "b", Node: "out"}, To: model.Endpoint{Component: "a", Node: "in"}},
			},
		},
	}

	_, err := Compile(loaded)
	if err == nil {
		t.Fatal("expected loop error")
	}
	if !strings.Contains(err.Error(), "algebraic loop detected") {
		t.Fatalf("expected algebraic loop message, got %v", err)
	}
}

func TestCompileOrdersFeedForwardGraph(t *testing.T) {
	loaded := &project.LoadedProject{
		Project: &model.Project{EntrySystem: "MainSystem"},
		Graph: &model.Graph{
			SchemaVersion: "0.1.0",
			Systems: []model.System{
				{
					ID:          "MainSystem",
					Components:  []string{"a", "b"},
					Connections: []string{"a_to_b"},
					PublicInputs: []model.PublicNodeRef{
						{ID: "value", Component: "a", Node: "in"},
					},
					PublicOutputs: []model.PublicNodeRef{
						{ID: "result", Component: "b", Node: "out"},
					},
				},
			},
			Components: []model.Component{
				component("a"),
				component("b"),
			},
			Connections: []model.Connection{
				{ID: "a_to_b", From: model.Endpoint{Component: "a", Node: "out"}, To: model.Endpoint{Component: "b", Node: "in"}},
			},
		},
	}

	plan, err := Compile(loaded)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(plan.Order, ","); got != "a,b" {
		t.Fatalf("unexpected order: %s", got)
	}
}

func component(id string) model.Component {
	return model.Component{
		ID:   id,
		Kind: "user_python",
		Nodes: model.NodeSet{
			Inputs: []model.Node{
				{ID: "in", Medium: "signal", ValueType: "float"},
			},
			Outputs: []model.Node{
				{ID: "out", Medium: "signal", ValueType: "float"},
			},
		},
	}
}
