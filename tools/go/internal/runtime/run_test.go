package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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

func TestValidateOutputsAcceptsDeclaredOutputs(t *testing.T) {
	component := contractComponent()

	if err := validateOutputs(component, map[string]any{"result": 1}); err != nil {
		t.Fatalf("error = %v", err)
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
