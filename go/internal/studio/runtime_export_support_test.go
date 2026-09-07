package studio

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/goniegonie/hvac-studio/go/internal/platform"
)

func TestRuntimeExportSupportMapsRepositoryPathsToPortableLayout(t *testing.T) {
	root := t.TempDir()
	pythonRoot := ".toolchain/python/cpython-3.12-windows-x86_64-none"
	sources := map[string]string{
		"go/go.mod": "module test\n",
		filepath.Join(".tmp/bin", platform.ExecutableName("bcs-runner")): "runner",
		filepath.Join(".tmp/bin", platform.ExecutableName("bcs-env")):    "env",
		"scripts/release/runtime-manifest.json":                          `{"runtime":"repository"}`,
		filepath.Join(pythonRoot, platform.ExecutableName("python")):     "python",
		filepath.Join(pythonRoot, "Lib/runtime_module.py"):               "runtime module",
		filepath.Join(pythonRoot, "Lib/__pycache__/runtime_module.pyc"):  "cache",
		"python/bcs_worker/bcs_worker/worker.py":                         "worker module",
		"python/bcs_sdk/bcs_sdk/client.py":                               "sdk module",
	}
	for rel, content := range sources {
		writeTestFile(t, filepath.Join(root, filepath.FromSlash(rel)), content)
	}
	exportRoot := filepath.Join(t.TempDir(), "runtime-export")
	files, err := writeRuntimeExportSupportFiles(filepath.Join(root, "projects", "example"), exportRoot, exportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string]string{
		filepath.Join("bin", platform.ExecutableName("bcs-runner")):        "runner",
		filepath.Join("bin", platform.ExecutableName("bcs-env")):           "env",
		"runtime/manifest.json":                                            `{"runtime":"repository"}`,
		filepath.Join("runtime/python", platform.ExecutableName("python")): "python",
		"runtime/python/Lib/runtime_module.py":                             "runtime module",
		"python/bcs_worker/bcs_worker/worker.py":                           "worker module",
	}
	if len(files) != len(expected) {
		t.Fatalf("support files = %v, want only portable support files", files)
	}
	for rel, want := range expected {
		if !containsString(files, filepath.ToSlash(rel)) {
			t.Fatalf("support manifest is missing %s", rel)
		}
		content, err := os.ReadFile(filepath.Join(exportRoot, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		if string(content) != want {
			t.Fatalf("support file %s = %q, want %q", rel, content, want)
		}
	}
}
