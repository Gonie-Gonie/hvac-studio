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

func TestNewIndexRejectsIncompatibleGraphSchemaVersion(t *testing.T) {
	graph := validGraph()
	graph.SchemaVersion = "0.2.0"

	_, err := NewIndex(graph)

	if err == nil || !strings.Contains(err.Error(), "graph schema_version 0.2.0 is not compatible") {
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

func TestNewIndexAcceptsComponentAuthoringMetadata(t *testing.T) {
	graph := validGraph()
	graph.Components[0].Category = "physical_component"
	graph.Components[0].ExecutionMode = "step"
	graph.Components[0].Source = model.ComponentSource{
		Layout:   "generated_wrapper",
		Metadata: "components/gain/component.json",
		Init:     "components/gain/user_init.py",
		Step:     "components/gain/user_step.py",
		Helpers:  "components/gain/helpers.py",
		Wrapper:  "components/gain/wrapper.py",
	}
	graph.Components[0].Nodes.Inputs[0].Preset = "scalar_input"
	graph.Components[0].Nodes.Outputs[0].Preset = "scalar_output"
	graph.Components[0].ParameterDefinitions = map[string]model.ParameterDefinition{
		"gain": {
			DisplayName: "Gain",
			Unit:        "ratio",
			Default:     2.0,
			Current:     2.5,
			Bounds:      &model.ValueBounds{Min: 0.0, Max: 10.0},
			Role:        "calibration_target",
			Group:       "Model",
			Description: "Multiplier applied to the input value.",
		},
	}
	graph.Components[0].StateDefinitions = map[string]model.StateDefinition{
		"calls": {
			DisplayName: "Call Count",
			Unit:        "count",
			Initial:     0,
			Description: "Number of completed evaluations.",
		},
	}

	if _, err := NewIndex(graph); err != nil {
		t.Fatal(err)
	}
}

func TestNewIndexRejectsInvalidComponentCategory(t *testing.T) {
	graph := validGraph()
	graph.Components[0].Category = "pumpish"

	_, err := NewIndex(graph)

	if err == nil || !strings.Contains(err.Error(), "component gain category is invalid: pumpish") {
		t.Fatalf("error = %v", err)
	}
}

func TestNewIndexRejectsInvalidComponentSourceLayout(t *testing.T) {
	graph := validGraph()
	graph.Components[0].Source = model.ComponentSource{Layout: "managed_region"}

	_, err := NewIndex(graph)

	if err == nil || !strings.Contains(err.Error(), "component gain source layout is invalid: managed_region") {
		t.Fatalf("error = %v", err)
	}
}

func TestNewIndexRejectsGeneratedWrapperWithoutStepSource(t *testing.T) {
	graph := validGraph()
	graph.Components[0].Source = model.ComponentSource{
		Layout:  "generated_wrapper",
		Wrapper: "components/gain/wrapper.py",
	}

	_, err := NewIndex(graph)

	if err == nil || !strings.Contains(err.Error(), "component gain generated_wrapper source step is required") {
		t.Fatalf("error = %v", err)
	}
}

func TestNewIndexRejectsInvalidNodePreset(t *testing.T) {
	graph := validGraph()
	graph.Components[0].Nodes.Inputs[0].Preset = "mystery_port"

	_, err := NewIndex(graph)

	if err == nil || !strings.Contains(err.Error(), "component gain input node value preset is invalid: mystery_port") {
		t.Fatalf("error = %v", err)
	}
}

func TestNewIndexRejectsCompositeWithoutSystemReference(t *testing.T) {
	graph := validGraph()
	graph.Components[0].Kind = "composite"
	graph.Components[0].Category = "composite_wrapper"

	_, err := NewIndex(graph)

	if err == nil || !strings.Contains(err.Error(), "component gain kind composite requires composite.system") {
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
