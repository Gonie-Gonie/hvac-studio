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

func TestPayloadForUsesStableSchemaAndProblemShape(t *testing.T) {
	err := Wrap(CodePythonWorker, errors.New("component scalar failed"))
	payload := PayloadFor(err, []Problem{{
		Severity:    "error",
		Message:     "bad output",
		ComponentID: "scalar",
		NodeID:      "result",
		Source:      "components/scalar.py",
		Line:        12,
	}})
	if payload.Schema != "hvac-studio.error.v1" {
		t.Fatalf("schema = %s", payload.Schema)
	}
	if payload.Code != int(CodePythonWorker) || payload.Kind != "python_worker" {
		t.Fatalf("payload code/kind = %#v", payload)
	}
	if payload.Message != "component scalar failed" {
		t.Fatalf("message = %s", payload.Message)
	}
	if len(payload.Problems) != 1 || payload.Problems[0].ComponentID != "scalar" || payload.Problems[0].Line != 12 {
		t.Fatalf("problems = %#v", payload.Problems)
	}
}
