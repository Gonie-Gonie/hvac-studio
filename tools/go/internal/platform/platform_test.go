package platform

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestExecutableNameUsesHostSuffix(t *testing.T) {
	got := ExecutableName("bcs-runner")
	if runtime.GOOS == "windows" {
		if got != "bcs-runner.exe" {
			t.Fatalf("executable = %s", got)
		}
		return
	}
	if got != "bcs-runner" {
		t.Fatalf("executable = %s", got)
	}
}

func TestRuntimePythonCandidatesIncludePackagedRuntime(t *testing.T) {
	root := filepath.Join("C:", "package")
	candidates := RuntimePythonCandidates(root)
	if len(candidates) == 0 {
		t.Fatal("expected runtime Python candidates")
	}
	if candidates[0] != filepath.Join(root, "runtime", "python", ExecutableName("python")) {
		t.Fatalf("first candidate = %s", candidates[0])
	}
}

func TestIsDefaultPythonName(t *testing.T) {
	for _, name := range []string{"python", "python.exe", "python3", "python3.exe"} {
		if !IsDefaultPythonName(name) {
			t.Fatalf("%s should be a default Python name", name)
		}
	}
	if IsDefaultPythonName("custom-python") {
		t.Fatal("custom executable name should not be a default Python name")
	}
}
