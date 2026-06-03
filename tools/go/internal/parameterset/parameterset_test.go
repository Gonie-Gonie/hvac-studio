package parameterset

import (
	"testing"

	"github.com/goniegonie/hvac-studio/tools/go/internal/model"
)

func TestApplyUpdatesOnlyDeclaredGraphParameters(t *testing.T) {
	graph := &model.Graph{
		Components: []model.Component{
			{ID: "gain", Parameters: map[string]any{"gain": 2.5}},
		},
	}

	err := Apply(graph, Set{Components: map[string]map[string]any{"gain": {"gain": 3.0}}})
	if err != nil {
		t.Fatal(err)
	}

	if graph.Components[0].Parameters["gain"] != 3.0 {
		t.Fatalf("gain = %v, want 3", graph.Components[0].Parameters["gain"])
	}
}

func TestApplyRejectsUnknownParameter(t *testing.T) {
	graph := &model.Graph{
		Components: []model.Component{
			{ID: "gain", Parameters: map[string]any{"gain": 2.5}},
		},
	}

	err := Apply(graph, Set{Components: map[string]map[string]any{"gain": {"offset": 1.0}}})
	if err == nil {
		t.Fatal("expected error")
	}
}
