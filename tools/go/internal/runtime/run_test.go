package runtime

import (
	"os"
	"path/filepath"
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
