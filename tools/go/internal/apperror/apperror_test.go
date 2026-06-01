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
