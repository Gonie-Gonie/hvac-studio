package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCollectStatusForRepositoryRoot(t *testing.T) {
	restoreVersionCommand := stubVersionCommand()
	defer restoreVersionCommand()

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "tools", "go", "go.mod"), "module test\n")
	writeFile(t, filepath.Join(root, "runtime", "manifest.json"), "{}\n")
	mkdirAll(t, filepath.Join(root, "python", "bcs_worker"))
	mkdirAll(t, filepath.Join(root, "python", "bcs_sdk"))
	mkdirAll(t, filepath.Join(root, "schema"))
	mkdirAll(t, filepath.Join(root, "examples"))
	mkdirAll(t, filepath.Join(root, "templates"))
	writeFile(t, filepath.Join(root, ".venv", "Scripts", "python.exe"), "fake python\n")

	status := collectStatus(root)
	if !status.OK {
		t.Fatalf("status should be ok: %#v", status.Problems)
	}
	if status.Mode != "repository" {
		t.Fatalf("mode = %s, want repository", status.Mode)
	}
	if !status.Python.Present {
		t.Fatal("python should be present")
	}
}

func TestCollectStatusForPortableRoot(t *testing.T) {
	restoreVersionCommand := stubVersionCommand()
	defer restoreVersionCommand()

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "release-manifest.json"), "{}\n")
	writeFile(t, filepath.Join(root, "runtime", "manifest.json"), "{}\n")
	writeFile(t, filepath.Join(root, "runtime", "python", "python.exe"), "fake python\n")
	writeFile(t, filepath.Join(root, "bin", "bcs-runner.exe"), "runner\n")
	writeFile(t, filepath.Join(root, "bin", "studio.exe"), "studio\n")
	writeFile(t, filepath.Join(root, "HVAC Studio.exe"), "desktop\n")
	mkdirAll(t, filepath.Join(root, "python", "bcs_worker"))
	mkdirAll(t, filepath.Join(root, "python", "bcs_sdk"))
	mkdirAll(t, filepath.Join(root, "schema"))
	mkdirAll(t, filepath.Join(root, "examples"))
	mkdirAll(t, filepath.Join(root, "templates"))

	status := collectStatus(root)
	if !status.OK {
		t.Fatalf("status should be ok: %#v", status.Problems)
	}
	if status.Mode != "portable-studio" {
		t.Fatalf("mode = %s, want portable-studio", status.Mode)
	}
}

func TestCollectStatusForRuntimeExportRoot(t *testing.T) {
	restoreVersionCommand := stubVersionCommand()
	defer restoreVersionCommand()

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "manifest.json"), "{}\n")
	writeFile(t, filepath.Join(root, "README.md"), "# Runtime Export\n")
	writeFile(t, filepath.Join(root, "run-default.ps1"), "Write-Host ok\n")
	writeFile(t, filepath.Join(root, "project", "project.bcsproj"), "{}\n")
	writeFile(t, filepath.Join(root, "project", "graph.json"), "{}\n")
	writeFile(t, filepath.Join(root, "schema", "public-io.json"), "{}\n")
	writeFile(t, filepath.Join(root, "bin", "bcs-runner.exe"), "runner\n")
	writeFile(t, filepath.Join(root, "bin", "bcs-env.exe"), "env\n")
	writeFile(t, filepath.Join(root, "runtime", "python", "python.exe"), "fake python\n")

	status := collectStatus(root)
	if !status.OK {
		t.Fatalf("status should be ok: %#v", status.Problems)
	}
	if status.Mode != "runtime-export" {
		t.Fatalf("mode = %s, want runtime-export", status.Mode)
	}
}

func TestCollectStatusReportsMissingPython(t *testing.T) {
	t.Setenv("HVAC_STUDIO_PYTHON", "")
	t.Setenv("PATH", "")

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "runtime", "manifest.json"), "{}\n")
	mkdirAll(t, filepath.Join(root, "python", "bcs_worker"))
	mkdirAll(t, filepath.Join(root, "python", "bcs_sdk"))
	mkdirAll(t, filepath.Join(root, "schema"))
	mkdirAll(t, filepath.Join(root, "examples"))
	mkdirAll(t, filepath.Join(root, "templates"))

	status := collectStatus(root)
	if status.OK {
		t.Fatal("status should fail without a Python runtime")
	}
	if status.Python.Present {
		t.Fatal("python should be missing")
	}
}

func TestRunCheckWritesJSON(t *testing.T) {
	restoreVersionCommand := stubVersionCommand()
	defer restoreVersionCommand()

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "tools", "go", "go.mod"), "module test\n")
	writeFile(t, filepath.Join(root, "runtime", "manifest.json"), "{}\n")
	writeFile(t, filepath.Join(root, ".venv", "Scripts", "python.exe"), "fake python\n")
	mkdirAll(t, filepath.Join(root, "python", "bcs_worker"))
	mkdirAll(t, filepath.Join(root, "python", "bcs_sdk"))
	mkdirAll(t, filepath.Join(root, "schema"))
	mkdirAll(t, filepath.Join(root, "examples"))
	mkdirAll(t, filepath.Join(root, "templates"))

	var output bytes.Buffer
	err := run([]string{"bcs-env", "check", "--root", root, "--json"}, &output)
	if err != nil {
		t.Fatal(err)
	}
	var status envStatus
	if err := json.Unmarshal(output.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if !status.OK || status.Root == "" {
		t.Fatalf("unexpected status: %#v", status)
	}
}

func mkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, path string, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o755); err != nil {
		t.Fatal(err)
	}
}

func stubVersionCommand() func() {
	original := versionCommand
	versionCommand = func(path string, arg string) (string, string) {
		return "Python 3.12.test", ""
	}
	return func() {
		versionCommand = original
	}
}
