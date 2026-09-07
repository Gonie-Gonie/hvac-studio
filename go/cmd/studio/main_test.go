package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/goniegonie/hvac-studio/go/internal/platform"
)

func TestFindRootFromRepositoryRoot(t *testing.T) {
	root := t.TempDir()
	mkdirAll(t, filepath.Join(root, "examples"))
	mkdirAll(t, filepath.Join(root, "go"))
	writeFile(t, filepath.Join(root, "go", "go.mod"), []byte("module test\n"))

	found, err := findRootFrom(filepath.Join(root, "go"))
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
	writeFile(t, platform.BinExecutable(root, "bcs-runner"), []byte("runner"))

	found, err := findRootFrom(filepath.Join(root, "bin"))
	if err != nil {
		t.Fatal(err)
	}
	if found != root {
		t.Fatalf("root = %s, want %s", found, root)
	}
}

func TestFindRepoRootPrefersPackagedExecutableOverCheckoutWorkingDirectory(t *testing.T) {
	root := t.TempDir()
	mkdirAll(t, filepath.Join(root, "examples"))
	mkdirAll(t, filepath.Join(root, "go"))
	writeFile(t, filepath.Join(root, "go", "go.mod"), []byte("module test\n"))
	packageRoot := filepath.Join(root, "dist", "latest")
	mkdirAll(t, filepath.Join(packageRoot, "examples"))
	mkdirAll(t, filepath.Join(packageRoot, "bin"))
	writeFile(t, filepath.Join(packageRoot, "release-manifest.json"), []byte("{}"))
	writeFile(t, platform.BinExecutable(packageRoot, "bcs-runner"), []byte("runner"))
	for _, executable := range []string{
		filepath.Join(packageRoot, "HVAC Studio.exe"),
		platform.BinExecutable(packageRoot, "studio"),
	} {
		found, err := findRepoRootFromPaths(root, executable)
		if err != nil {
			t.Fatal(err)
		}
		if found != packageRoot {
			t.Fatalf("root for %s = %s, want %s", executable, found, packageRoot)
		}
	}

	found, err := findRepoRootFromPaths(root, filepath.Join(root, ".tmp", "studio.exe"))
	if err != nil {
		t.Fatal(err)
	}
	if found != root {
		t.Fatalf("development root = %s, want %s", found, root)
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
