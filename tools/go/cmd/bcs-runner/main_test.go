package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/goniegonie/hvac-studio/tools/go/internal/apperror"
)

func TestRunReturnsValidationExitCodeForUsage(t *testing.T) {
	err := run([]string{"bcs-runner"})
	if got := apperror.ExitCode(err); got != int(apperror.CodeValidation) {
		t.Fatalf("exit code = %d, want %d", got, apperror.CodeValidation)
	}
}

func TestRunReturnsInputExitCodeForMissingPublicInput(t *testing.T) {
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "missing-input.json")
	if err := os.WriteFile(inputPath, []byte(`{"inputs":{},"context":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	projectPath := filepath.Join("..", "..", "..", "..", "examples", "001_scalar_component", "project.bcsproj")
	err := run([]string{
		"bcs-runner",
		"run",
		"--project",
		projectPath,
		"--input",
		inputPath,
		"--output",
		filepath.Join(tmpDir, "output.json"),
	})
	if got := apperror.ExitCode(err); got != int(apperror.CodeInput) {
		t.Fatalf("exit code = %d, want %d; error=%v", got, apperror.CodeInput, err)
	}
}

func TestRunReturnsInputExitCodeForInvalidInputJSON(t *testing.T) {
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "invalid-input.json")
	if err := os.WriteFile(inputPath, []byte(`{"inputs":`), 0o644); err != nil {
		t.Fatal(err)
	}

	projectPath := filepath.Join("..", "..", "..", "..", "examples", "001_scalar_component", "project.bcsproj")
	err := run([]string{
		"bcs-runner",
		"run",
		"--project",
		projectPath,
		"--input",
		inputPath,
		"--output",
		filepath.Join(tmpDir, "output.json"),
	})
	if got := apperror.ExitCode(err); got != int(apperror.CodeInput) {
		t.Fatalf("exit code = %d, want %d; error=%v", got, apperror.CodeInput, err)
	}
}
