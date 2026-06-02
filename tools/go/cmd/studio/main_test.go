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

func TestUniqueStringsKeepsFirstNonEmptyValue(t *testing.T) {
	values := uniqueStrings([]string{"", "edge", "chrome", "edge"})
	if len(values) != 2 || values[0] != "edge" || values[1] != "chrome" {
		t.Fatalf("values = %#v", values)
	}
}

func TestJoinEnvPathSkipsMissingRoot(t *testing.T) {
	t.Setenv("HVAC_STUDIO_TEST_ROOT", "")
	if got := joinEnvPath("HVAC_STUDIO_TEST_ROOT", "bin", "studio.exe"); got != "" {
		t.Fatalf("path = %s, want empty", got)
	}
	t.Setenv("HVAC_STUDIO_TEST_ROOT", "C:\\Studio")
	if got := joinEnvPath("HVAC_STUDIO_TEST_ROOT", "bin", "studio.exe"); got == "" {
		t.Fatal("path should be joined when root exists")
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
