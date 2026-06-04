package migration

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInspectProjectReportsCompatiblePatchLine(t *testing.T) {
	root := t.TempDir()
	writeMigrationFixture(t, root, "0.1.9", "0.1.0")

	report, err := InspectProject(filepath.Join(root, "project.bcsproj"), false)
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK {
		t.Fatalf("report should be ok: %#v", report)
	}
	if len(report.Artifacts) != 2 {
		t.Fatalf("artifact count = %d", len(report.Artifacts))
	}
	if report.Artifacts[0].Kind != "project" || report.Artifacts[0].NeedsMigration {
		t.Fatalf("project artifact report = %#v", report.Artifacts[0])
	}
	if report.Artifacts[1].Kind != "graph" || report.Artifacts[1].NeedsMigration {
		t.Fatalf("graph artifact report = %#v", report.Artifacts[1])
	}
	if len(report.Actions) != 1 || report.Actions[0].Kind != "no_migration_needed" {
		t.Fatalf("actions = %#v", report.Actions)
	}
}

func TestInspectProjectReportsIncompatibleVersion(t *testing.T) {
	root := t.TempDir()
	writeMigrationFixture(t, root, "0.2.0", "0.1.0")

	report, err := InspectProject(filepath.Join(root, "project.bcsproj"), true)
	if err != nil {
		t.Fatal(err)
	}
	if report.OK {
		t.Fatalf("report should require migration: %#v", report)
	}
	if len(report.Actions) != 1 || report.Actions[0].Kind != "manual_migration_required" {
		t.Fatalf("actions = %#v", report.Actions)
	}
	if !report.Artifacts[0].NeedsMigration {
		t.Fatalf("project artifact should need migration: %#v", report.Artifacts[0])
	}
}

func writeMigrationFixture(t *testing.T, root string, projectVersion string, graphVersion string) {
	t.Helper()
	writeMigrationFile(t, filepath.Join(root, "project.bcsproj"), `{
  "project_name": "fixture",
  "schema_version": "`+projectVersion+`",
  "entry_system": "MainSystem",
  "graph": "graph.json"
}
`)
	writeMigrationFile(t, filepath.Join(root, "graph.json"), `{
  "schema_version": "`+graphVersion+`",
  "systems": [],
  "components": [],
  "connections": []
}
`)
}

func writeMigrationFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
