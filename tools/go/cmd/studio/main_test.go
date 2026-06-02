package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindRootFromRepositoryRoot(t *testing.T) {
	root := t.TempDir()
	mkdirAll(t, filepath.Join(root, "examples"))
	mkdirAll(t, filepath.Join(root, "tools", "go"))
	writeFile(t, filepath.Join(root, "tools", "go", "go.mod"), []byte("module test\n"))

	found, err := findRootFrom(filepath.Join(root, "tools", "go"))
	if err != nil {
		t.Fatal(err)
	}
	if found != root {
		t.Fatalf("root = %s, want %s", found, root)
	}
}

func TestFindRootFromPortablePackageBin(t *testing.T) {
	root := t.TempDir()
	mkdirAll(t, filepath.Join(root, "examples"))
	mkdirAll(t, filepath.Join(root, "bin"))
	writeFile(t, filepath.Join(root, "bin", "bcs-runner.exe"), []byte("runner"))

	found, err := findRootFrom(filepath.Join(root, "bin"))
	if err != nil {
		t.Fatal(err)
	}
	if found != root {
		t.Fatalf("root = %s, want %s", found, root)
	}
}

func TestShouldRunServerSupportsNoWindowAlias(t *testing.T) {
	if !shouldRunServer(false, true) {
		t.Fatal("--no-window should keep working as a server-mode alias")
	}
	if !shouldRunServer(true, false) {
		t.Fatal("--server should enable server mode")
	}
	if shouldRunServer(false, false) {
		t.Fatal("default launch should use the Wails desktop app")
	}
}

func mkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, path string, contents []byte) {
	t.Helper()
	if err := os.WriteFile(path, contents, 0o644); err != nil {
		t.Fatal(err)
	}
}
