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
	if !strings.Contains(err.Error(), "explicit solver boundary component") {
		t.Fatalf("expected solver boundary guidance, got %v", err)
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

func TestCompileAllowsExplicitSolverBoundaryComponentInAcyclicGraph(t *testing.T) {
	loaded := &project.LoadedProject{
		Project: &model.Project{EntrySystem: "MainSystem"},
		Graph: &model.Graph{
			SchemaVersion: "0.1.0",
			Systems: []model.System{
				{
					ID:          "MainSystem",
					Components:  []string{"solver"},
					Connections: []string{},
					PublicInputs: []model.PublicNodeRef{
						{ID: "target", Component: "solver", Node: "in"},
					},
					PublicOutputs: []model.PublicNodeRef{
						{ID: "solution", Component: "solver", Node: "out"},
					},
				},
			},
			Components: []model.Component{
				{
					ID:       "solver",
					Kind:     "user_python",
					Category: "solver",
					Nodes: model.NodeSet{
						Inputs:  []model.Node{{ID: "in", Medium: "signal", ValueType: "float"}},
						Outputs: []model.Node{{ID: "out", Medium: "signal", ValueType: "float"}},
					},
					SolverBoundary: &model.SolverBoundary{
						Method:        "fixed_point",
						MaxIterations: 10,
						Tolerance:     0.001,
					},
				},
			},
			Connections: []model.Connection{},
		},
	}

	plan, err := Compile(loaded)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(plan.Order, ","); got != "solver" {
		t.Fatalf("unexpected order: %s", got)
	}
}

func TestCompileWarnsForSignalToPhysicalMediumConnection(t *testing.T) {
	loaded := compileProjectWithConnection(
		componentWithMedia("controller", "signal", "signal"),
		componentWithMedia("coil", "water", "water"),
		model.Connection{
			ID:   "controller_to_coil",
			From: model.Endpoint{Component: "controller", Node: "out"},
			To:   model.Endpoint{Component: "coil", Node: "in"},
		},
	)

	plan, err := Compile(loaded)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Diagnostics) != 1 {
		t.Fatalf("diagnostics = %#v", plan.Diagnostics)
	}
	diagnostic := plan.Diagnostics[0]
	if diagnostic.Severity != "warning" || diagnostic.ConnectionID != "controller_to_coil" {
		t.Fatalf("diagnostic = %#v", diagnostic)
	}
	if !strings.Contains(diagnostic.Message, "medium mismatch") {
		t.Fatalf("diagnostic message = %s", diagnostic.Message)
	}
}

func TestCompileRejectsPhysicalMediumMismatchByDefault(t *testing.T) {
	loaded := compileProjectWithConnection(
		componentWithMedia("fan", "signal", "air"),
		componentWithMedia("coil", "water", "water"),
		model.Connection{
			ID:   "fan_to_coil",
			From: model.Endpoint{Component: "fan", Node: "out"},
			To:   model.Endpoint{Component: "coil", Node: "in"},
		},
	)

	_, err := Compile(loaded)

	if err == nil || !strings.Contains(err.Error(), "connection fan_to_coil medium mismatch: fan.out=air -> coil.in=water") {
		t.Fatalf("error = %v", err)
	}
}

func TestCompileWarnsForExplicitMediumOverride(t *testing.T) {
	loaded := compileProjectWithConnection(
		componentWithMedia("fan", "signal", "air"),
		componentWithMedia("coil", "water", "water"),
		model.Connection{
			ID:                  "fan_to_coil",
			From:                model.Endpoint{Component: "fan", Node: "out"},
			To:                  model.Endpoint{Component: "coil", Node: "in"},
			AllowMediumMismatch: true,
		},
	)

	plan, err := Compile(loaded)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Diagnostics) != 1 {
		t.Fatalf("diagnostics = %#v", plan.Diagnostics)
	}
	if !strings.Contains(plan.Diagnostics[0].Message, "allowed by explicit medium override") {
		t.Fatalf("diagnostic message = %s", plan.Diagnostics[0].Message)
	}
}

func TestCompileWarnsForUnitMismatchWithoutConversion(t *testing.T) {
	source := componentWithMedia("meter", "signal", "signal")
	source.Nodes.Outputs[0].Unit = "W"
	target := componentWithMedia("load", "signal", "signal")
	target.Nodes.Inputs[0].Unit = "kW"
	loaded := compileProjectWithConnection(
		source,
		target,
		model.Connection{
			ID:   "meter_to_load",
			From: model.Endpoint{Component: "meter", Node: "out"},
			To:   model.Endpoint{Component: "load", Node: "in"},
		},
	)

	plan, err := Compile(loaded)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Diagnostics) != 1 {
		t.Fatalf("diagnostics = %#v", plan.Diagnostics)
	}
	if !strings.Contains(plan.Diagnostics[0].Message, "unit mismatch without conversion") {
		t.Fatalf("diagnostic message = %s", plan.Diagnostics[0].Message)
	}
}

func TestCompileAcceptsExplicitUnitConversion(t *testing.T) {
	source := componentWithMedia("meter", "signal", "signal")
	source.Nodes.Outputs[0].Unit = "W"
	target := componentWithMedia("load", "signal", "signal")
	target.Nodes.Inputs[0].Unit = "kW"
	factor := 0.001
	loaded := compileProjectWithConnection(
		source,
		target,
		model.Connection{
			ID:             "meter_to_load",
			From:           model.Endpoint{Component: "meter", Node: "out"},
			To:             model.Endpoint{Component: "load", Node: "in"},
			UnitConversion: &model.UnitConversion{Mode: "linear", Factor: &factor},
		},
	)

	plan, err := Compile(loaded)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", plan.Diagnostics)
	}
}

func compileProjectWithConnection(source model.Component, target model.Component, connection model.Connection) *project.LoadedProject {
	return &project.LoadedProject{
		Project: &model.Project{EntrySystem: "MainSystem"},
		Graph: &model.Graph{
			SchemaVersion: "0.1.0",
			Systems: []model.System{
				{
					ID:          "MainSystem",
					Components:  []string{source.ID, target.ID},
					Connections: []string{connection.ID},
					PublicInputs: []model.PublicNodeRef{
						{ID: "value", Component: source.ID, Node: "in"},
					},
					PublicOutputs: []model.PublicNodeRef{
						{ID: "result", Component: target.ID, Node: "out"},
					},
				},
			},
			Components:  []model.Component{source, target},
			Connections: []model.Connection{connection},
		},
	}
}

func component(id string) model.Component {
	return componentWithMedia(id, "signal", "signal")
}

func componentWithMedia(id string, inputMedium string, outputMedium string) model.Component {
	return model.Component{
		ID:   id,
		Kind: "user_python",
		Nodes: model.NodeSet{
			Inputs: []model.Node{
				{ID: "in", Medium: inputMedium, ValueType: "float"},
			},
			Outputs: []model.Node{
				{ID: "out", Medium: outputMedium, ValueType: "float"},
			},
		},
	}
}
