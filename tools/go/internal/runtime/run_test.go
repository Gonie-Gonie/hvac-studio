package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goniegonie/hvac-studio/tools/go/internal/apperror"
	"github.com/goniegonie/hvac-studio/tools/go/internal/compiler"
	graphindex "github.com/goniegonie/hvac-studio/tools/go/internal/graph"
	"github.com/goniegonie/hvac-studio/tools/go/internal/model"
)

func TestResolvePythonPrefersProjectRelativePython(t *testing.T) {
	root := t.TempDir()
	projectRoot := filepath.Join(root, "examples", "demo")
	projectPython := filepath.Join(projectRoot, ".venv", "Scripts", "python.exe")
	if err := os.MkdirAll(filepath.Dir(projectPython), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(projectPython, []byte("python"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := resolvePython(projectRoot, model.EnvironmentConfig{Python: ".venv/Scripts/python.exe"})

	if got != projectPython {
		t.Fatalf("python = %s, want %s", got, projectPython)
	}
}

func TestResolvePythonFindsPackagedRuntime(t *testing.T) {
	root := t.TempDir()
	projectRoot := filepath.Join(root, "examples", "demo")
	packagedPython := filepath.Join(root, "runtime", "python", "python.exe")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(packagedPython), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(packagedPython, []byte("python"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := resolvePython(projectRoot, model.EnvironmentConfig{Python: "python"})

	if got != packagedPython {
		t.Fatalf("python = %s, want %s", got, packagedPython)
	}
}

func TestValidateOutputsRejectsMissingDeclaredOutput(t *testing.T) {
	component := contractComponent()

	err := validateOutputs(component, map[string]any{})

	if err == nil || !strings.Contains(err.Error(), "did not return declared output node: result") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateOutputsRejectsUndeclaredOutput(t *testing.T) {
	component := contractComponent()

	err := validateOutputs(component, map[string]any{"result": 1, "debug": 2})

	if err == nil || !strings.Contains(err.Error(), "returned undeclared output node: debug") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateOutputsUsesRuntimeCodeForExternalExecutable(t *testing.T) {
	component := contractComponent()
	component.Kind = "external_exe"

	err := validateOutputs(component, map[string]any{})

	if apperror.ErrorCode(err) != apperror.CodeRuntime {
		t.Fatalf("error code = %v, want runtime", apperror.ErrorCode(err))
	}
}

func TestValidateOutputsUsesRuntimeCodeForComposite(t *testing.T) {
	component := contractComponent()
	component.Kind = "composite"

	err := validateOutputs(component, map[string]any{})

	if apperror.ErrorCode(err) != apperror.CodeRuntime {
		t.Fatalf("error code = %v, want runtime", apperror.ErrorCode(err))
	}
}

func TestValidateOutputsAcceptsDeclaredOutputs(t *testing.T) {
	component := contractComponent()

	if err := validateOutputs(component, map[string]any{"result": 1}); err != nil {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateOutputsRejectsValueTypeMismatch(t *testing.T) {
	component := contractComponent()
	component.Nodes.Outputs[0].ValueType = "float"

	err := validateOutputs(component, map[string]any{"result": "not-a-number"})

	if err == nil || !strings.Contains(err.Error(), "component scalar output result expects float") {
		t.Fatalf("error = %v", err)
	}
}

func TestApplyConnectionUnitConversionUsesLinearFactorAndOffset(t *testing.T) {
	factor := 0.001
	offset := 2.0
	connection := model.Connection{
		ID:             "watts_to_kw_bias",
		UnitConversion: &model.UnitConversion{Mode: "linear", Factor: &factor, Offset: &offset},
	}

	value, err := applyConnectionUnitConversion(connection, 3000.0)

	if err != nil {
		t.Fatal(err)
	}
	if value != 5.0 {
		t.Fatalf("converted value = %v, want 5", value)
	}
}

func TestNodeValueTracesIncludeContractMetadata(t *testing.T) {
	traces := nodeValueTraces("coil", "input", []model.Node{
		{ID: "chw_in", Medium: "water", ValueType: "float", Unit: "degC"},
		{ID: "unused", Medium: "water", ValueType: "float"},
	}, map[string]any{"chw_in": 6.5})

	if len(traces) != 1 {
		t.Fatalf("traces = %#v", traces)
	}
	trace := traces[0]
	if trace.Component != "coil" || trace.Node != "chw_in" || trace.Direction != "input" {
		t.Fatalf("trace endpoint = %#v", trace)
	}
	if trace.Medium != "water" || trace.ValueType != "float" || trace.Unit != "degC" || trace.Value != 6.5 {
		t.Fatalf("trace metadata = %#v", trace)
	}
}

func TestConnectionValueTracesIncludeEndpointMetadata(t *testing.T) {
	graph := &model.Graph{
		SchemaVersion: "0.1.0",
		Components: []model.Component{
			{
				ID:   "load",
				Kind: "user_python",
				Nodes: model.NodeSet{
					Outputs: []model.Node{{ID: "out", Medium: "signal", ValueType: "float", Unit: "kW"}},
				},
			},
			{
				ID:   "controller",
				Kind: "user_python",
				Nodes: model.NodeSet{
					Inputs: []model.Node{{ID: "in", Medium: "signal", ValueType: "float", Unit: "kW"}},
				},
			},
		},
		Connections: []model.Connection{
			{
				ID:   "load_to_controller",
				From: model.Endpoint{Component: "load", Node: "out"},
				To:   model.Endpoint{Component: "controller", Node: "in"},
			},
		},
	}
	index, err := graphindex.NewIndex(graph)
	if err != nil {
		t.Fatal(err)
	}
	plan := &compiler.Plan{
		System: model.System{Connections: []string{"load_to_controller"}},
		Index:  index,
	}

	traces := connectionValueTraces(plan, map[string]map[string]any{
		"load": {"out": 550.0},
	})

	if len(traces) != 1 {
		t.Fatalf("traces = %#v", traces)
	}
	trace := traces[0]
	if trace.ID != "load_to_controller" || trace.From.Component != "load" || trace.To.Component != "controller" {
		t.Fatalf("trace endpoint = %#v", trace)
	}
	if trace.SourceMedium != "signal" || trace.TargetMedium != "signal" || trace.ValueType != "float" || trace.Unit != "kW" {
		t.Fatalf("trace metadata = %#v", trace)
	}
	if trace.Value != 550.0 {
		t.Fatalf("trace value = %v, want 550", trace.Value)
	}
}

func TestNestedCompositeStatesClonesChildState(t *testing.T) {
	state := map[string]any{
		"system": "ChildSystem",
		"states": map[string]map[string]any{
			"gain": {"calls": 2},
		},
	}

	cloned := nestedCompositeStates(state)
	cloned["gain"]["calls"] = 3

	original := state["states"].(map[string]map[string]any)["gain"]["calls"]
	if original != 2 {
		t.Fatalf("original state mutated: %v", original)
	}
}

func contractComponent() model.Component {
	return model.Component{
		ID: "scalar",
		Nodes: model.NodeSet{
			Outputs: []model.Node{{ID: "result"}},
		},
	}
}
