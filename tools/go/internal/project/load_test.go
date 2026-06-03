package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadAcceptsEnvironmentLockfile(t *testing.T) {
	root := t.TempDir()
	writeProjectFixture(t, root, `"lockfile": "requirements.lock.txt"`)
	writeFile(t, filepath.Join(root, "requirements.lock.txt"), "# no third-party dependencies\n")

	loaded, err := Load(filepath.Join(root, "project.bcsproj"))
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Project.Environment.Lockfile != "requirements.lock.txt" {
		t.Fatalf("lockfile = %q", loaded.Project.Environment.Lockfile)
	}
}

func TestLoadRejectsMissingEnvironmentLockfile(t *testing.T) {
	root := t.TempDir()
	writeProjectFixture(t, root, `"lockfile": "missing.lock"`)

	_, err := Load(filepath.Join(root, "project.bcsproj"))
	if err == nil {
		t.Fatal("expected missing lockfile error")
	}
	if !strings.Contains(err.Error(), "project environment lockfile not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func writeProjectFixture(t *testing.T, root string, environmentLine string) {
	t.Helper()
	writeFile(t, filepath.Join(root, "project.bcsproj"), `{
  "project_name": "fixture",
  "schema_version": "0.1.0",
  "entry_system": "MainSystem",
  "graph": "graph.json",
  "environment": {
    "mode": "project",
    "python": "python",
    `+environmentLine+`
  }
}
`)
	writeFile(t, filepath.Join(root, "graph.json"), `{
  "schema_version": "0.1.0",
  "systems": [],
  "components": [],
  "connections": []
}
`)
}

func writeFile(t *testing.T, path string, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}
