package apperror

import (
	"errors"
	"testing"
)

func TestExitCode(t *testing.T) {
	if got := ExitCode(nil); got != 0 {
		t.Fatalf("nil error code = %d", got)
	}
	if got := ExitCode(Wrap(CodeInput, errors.New("bad input"))); got != int(CodeInput) {
		t.Fatalf("input error code = %d", got)
	}
	if got := ExitCode(errors.New("plain")); got != int(CodeRuntime) {
		t.Fatalf("plain error code = %d", got)
	}
}

func TestWrapKeepsExistingCode(t *testing.T) {
	err := Wrap(CodeInput, errors.New("bad input"))
	wrapped := Wrap(CodeRuntime, err)
	if got := ErrorCode(wrapped); got != CodeInput {
		t.Fatalf("wrapped code = %v", got)
	}
}

func TestDocumentedCodeTaxonomy(t *testing.T) {
	cases := []struct {
		code Code
		exit int
		name string
	}{
		{CodeSuccess, 0, "success"},
		{CodeValidation, 1, "validation"},
		{CodeRuntime, 2, "runtime"},
		{CodeInput, 3, "input"},
		{CodePythonWorker, 4, "python_worker"},
		{CodeLicenseRuntime, 5, "license_runtime"},
	}
	for _, tc := range cases {
		if int(tc.code) != tc.exit {
			t.Fatalf("%s exit code = %d, want %d", tc.name, tc.code, tc.exit)
		}
		if got := CodeName(tc.code); got != tc.name {
			t.Fatalf("code name = %s, want %s", got, tc.name)
		}
	}
}
