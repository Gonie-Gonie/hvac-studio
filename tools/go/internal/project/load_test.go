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

func TestLoadRejectsEnvironmentLockfileOutsideProject(t *testing.T) {
	tests := []struct {
		name     string
		lockfile string
	}{
		{name: "parent traversal", lockfile: "../requirements.lock.txt"},
		{name: "nested parent traversal", lockfile: "env/../../requirements.lock.txt"},
		{name: "absolute path", lockfile: filepath.ToSlash(filepath.Join(t.TempDir(), "requirements.lock.txt"))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writeProjectFixture(t, root, `"lockfile": "`+tt.lockfile+`"`)

			_, err := Load(filepath.Join(root, "project.bcsproj"))

			if err == nil || !strings.Contains(err.Error(), "project environment lockfile must stay inside project root") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestLoadRejectsUnknownProjectFields(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "project.bcsproj"), `{
  "project_name": "fixture",
  "schema_version": "0.1.0",
  "entry_system": "MainSystem",
  "graph": "graph.json",
  "surprise": true
}
`)
	writeFile(t, filepath.Join(root, "graph.json"), `{
  "schema_version": "0.1.0",
  "systems": [],
  "components": [],
  "connections": []
}
`)

	_, err := Load(filepath.Join(root, "project.bcsproj"))
	if err == nil || !strings.Contains(err.Error(), `decode project: json: unknown field "surprise"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadRejectsMissingProjectSchemaVersion(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "project.bcsproj"), `{
  "project_name": "fixture",
  "entry_system": "MainSystem",
  "graph": "graph.json"
}
`)

	_, err := Load(filepath.Join(root, "project.bcsproj"))
	if err == nil || !strings.Contains(err.Error(), "project schema_version is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadAcceptsCompatibleProjectSchemaPatchVersion(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "project.bcsproj"), `{
  "project_name": "fixture",
  "schema_version": "0.1.9",
  "entry_system": "MainSystem",
  "graph": "graph.json"
}
`)
	writeFile(t, filepath.Join(root, "graph.json"), `{
  "schema_version": "0.1.0",
  "systems": [],
  "components": [],
  "connections": []
}
`)

	loaded, err := Load(filepath.Join(root, "project.bcsproj"))
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Project.SchemaVersion != "0.1.9" {
		t.Fatalf("schema_version = %s", loaded.Project.SchemaVersion)
	}
}

func TestLoadRejectsIncompatibleProjectSchemaVersion(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "project.bcsproj"), `{
  "project_name": "fixture",
  "schema_version": "0.2.0",
  "entry_system": "MainSystem",
  "graph": "graph.json"
}
`)
	writeFile(t, filepath.Join(root, "graph.json"), `{
  "schema_version": "0.1.0",
  "systems": [],
  "components": [],
  "connections": []
}
`)

	_, err := Load(filepath.Join(root, "project.bcsproj"))
	if err == nil || !strings.Contains(err.Error(), "project schema_version 0.2.0 is not compatible") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadRejectsUnknownGraphFields(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "project.bcsproj"), `{
  "project_name": "fixture",
  "schema_version": "0.1.0",
  "entry_system": "MainSystem",
  "graph": "graph.json"
}
`)
	writeFile(t, filepath.Join(root, "graph.json"), `{
  "schema_version": "0.1.0",
  "systems": [],
  "components": [],
  "connections": [],
  "surprise": true
}
`)

	_, err := Load(filepath.Join(root, "project.bcsproj"))
	if err == nil || !strings.Contains(err.Error(), `decode graph: json: unknown field "surprise"`) {
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
