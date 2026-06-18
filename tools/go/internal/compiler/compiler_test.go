package compiler

import (
	"os"
	"path/filepath"
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

func TestCompileValidatesMLAssetPaths(t *testing.T) {
	root := t.TempDir()
	assetPath := filepath.Join(root, "assets", "model.json")
	if err := os.MkdirAll(filepath.Dir(assetPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(assetPath, []byte(`{"ok":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	loaded := &project.LoadedProject{
		Root:    root,
		Project: &model.Project{EntrySystem: "MainSystem"},
		Graph: &model.Graph{
			SchemaVersion: "0.1.0",
			Systems: []model.System{
				{
					ID:          "MainSystem",
					Components:  []string{"ann"},
					Connections: []string{},
					PublicInputs: []model.PublicNodeRef{
						{ID: "features", Component: "ann", Node: "features"},
					},
					PublicOutputs: []model.PublicNodeRef{
						{ID: "prediction", Component: "ann", Node: "prediction"},
					},
				},
			},
			Components: []model.Component{
				{
					ID:   "ann",
					Kind: "user_python",
					Nodes: model.NodeSet{
						Inputs:  []model.Node{{ID: "features", Medium: "signal", ValueType: "object"}},
						Outputs: []model.Node{{ID: "prediction", Medium: "signal", ValueType: "float"}},
					},
					MLMetadata: &model.MLMetadata{
						ModelFormat:       "custom",
						ModelFile:         "assets/model.json",
						FeatureSchemaFile: "assets/features.json",
					},
				},
			},
		},
	}

	loaded.Graph.Components[0].MLMetadata.ModelFormat = "custom-linear"
	_, err := Compile(loaded)
	if err == nil || !strings.Contains(err.Error(), "ml_metadata.model_format is unsupported: custom-linear") {
		t.Fatalf("format error = %v", err)
	}
	loaded.Graph.Components[0].MLMetadata.ModelFormat = "custom"

	_, err = Compile(loaded)
	if err == nil || !strings.Contains(err.Error(), "ML asset not found: assets/features.json") {
		t.Fatalf("error = %v", err)
	}

	if err := os.WriteFile(filepath.Join(root, "assets", "features.json"), []byte(`{"features":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Compile(loaded); err != nil {
		t.Fatal(err)
	}
}

func TestCompileAllowsCompositeBoundaryWithMatchingPublicIO(t *testing.T) {
	loaded := compositeLoadedProject()

	plan, err := Compile(loaded)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(plan.Order, ","); got != "wrapped_gain" {
		t.Fatalf("unexpected order: %s", got)
	}
}

func TestCompileRejectsCompositeBoundaryMismatch(t *testing.T) {
	loaded := compositeLoadedProject()
	loaded.Graph.Systems[1].PublicOutputs[0].ID = "child_result"

	_, err := Compile(loaded)

	if err == nil || !strings.Contains(err.Error(), "composite output node missing child public output: child_result") {
		t.Fatalf("error = %v", err)
	}
}

func TestCompileRejectsCompositeRecursion(t *testing.T) {
	loaded := compositeLoadedProject()
	loaded.Graph.Components[0].Composite.System = "MainSystem"

	_, err := Compile(loaded)

	if err == nil || !strings.Contains(err.Error(), "composite system recursion detected: MainSystem -> MainSystem") {
		t.Fatalf("error = %v", err)
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

func TestCompileRejectsInvalidUnitConversion(t *testing.T) {
	source := componentWithMedia("meter", "signal", "signal")
	source.Nodes.Outputs[0].Unit = "W"
	target := componentWithMedia("load", "signal", "signal")
	target.Nodes.Inputs[0].Unit = "kW"
	loaded := compileProjectWithConnection(
		source,
		target,
		model.Connection{
			ID:             "meter_to_load",
			From:           model.Endpoint{Component: "meter", Node: "out"},
			To:             model.Endpoint{Component: "load", Node: "in"},
			UnitConversion: &model.UnitConversion{Mode: "table"},
		},
	)

	_, err := Compile(loaded)

	if err == nil || !strings.Contains(err.Error(), "unit_conversion mode is unsupported") {
		t.Fatalf("error = %v", err)
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

func compositeLoadedProject() *project.LoadedProject {
	return &project.LoadedProject{
		Project: &model.Project{EntrySystem: "MainSystem"},
		Graph: &model.Graph{
			SchemaVersion: "0.1.0",
			Systems: []model.System{
				{
					ID:          "MainSystem",
					Components:  []string{"wrapped_gain"},
					Connections: []string{},
					PublicInputs: []model.PublicNodeRef{
						{ID: "value", Component: "wrapped_gain", Node: "value"},
					},
					PublicOutputs: []model.PublicNodeRef{
						{ID: "result", Component: "wrapped_gain", Node: "result"},
					},
				},
				{
					ID:          "GainSystem",
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
					ID:   "wrapped_gain",
					Kind: "composite",
					Composite: &model.CompositeReference{
						System: "GainSystem",
					},
					Nodes: model.NodeSet{
						Inputs:  []model.Node{{ID: "value", Medium: "signal", ValueType: "float"}},
						Outputs: []model.Node{{ID: "result", Medium: "signal", ValueType: "float"}},
					},
				},
				{
					ID:   "gain",
					Kind: "user_python",
					Nodes: model.NodeSet{
						Inputs:  []model.Node{{ID: "value", Medium: "signal", ValueType: "float"}},
						Outputs: []model.Node{{ID: "result", Medium: "signal", ValueType: "float"}},
					},
				},
			},
			Connections: []model.Connection{},
		},
	}
}
