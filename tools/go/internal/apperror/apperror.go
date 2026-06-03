package apperror

import (
	"errors"
	"fmt"
)

type Code int

const (
	CodeSuccess Code = iota
	CodeValidation
	CodeRuntime
	CodeInput
	CodePythonWorker
	CodeLicenseRuntime
)

type Error struct {
	Code Code
	Err  error
}

type Payload struct {
	Schema   string    `json:"schema"`
	Code     int       `json:"code"`
	Kind     string    `json:"kind"`
	Message  string    `json:"message"`
	Problems []Problem `json:"problems,omitempty"`
}

type Problem struct {
	Severity    string `json:"severity,omitempty"`
	Message     string `json:"message"`
	ComponentID string `json:"component_id,omitempty"`
	NodeID      string `json:"node_id,omitempty"`
	Source      string `json:"source,omitempty"`
	Line        int    `json:"line,omitempty"`
	Column      int    `json:"column,omitempty"`
}

func Wrap(code Code, err error) error {
	if err == nil {
		return nil
	}
	var appErr *Error
	if errors.As(err, &appErr) {
		return err
	}
	return &Error{Code: code, Err: err}
}

func Errorf(code Code, format string, args ...any) error {
	return &Error{Code: code, Err: fmt.Errorf(format, args...)}
}

func (e *Error) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func ExitCode(err error) int {
	if err == nil {
		return int(CodeSuccess)
	}
	var appErr *Error
	if errors.As(err, &appErr) {
		return int(appErr.Code)
	}
	return int(CodeRuntime)
}

func CodeName(code Code) string {
	switch code {
	case CodeSuccess:
		return "success"
	case CodeValidation:
		return "validation"
	case CodeRuntime:
		return "runtime"
	case CodeInput:
		return "input"
	case CodePythonWorker:
		return "python_worker"
	case CodeLicenseRuntime:
		return "license_runtime"
	default:
		return fmt.Sprintf("unknown_%d", code)
	}
}

func ErrorCode(err error) Code {
	if err == nil {
		return CodeSuccess
	}
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return CodeRuntime
}

func PayloadFor(err error, problems []Problem) Payload {
	code := ErrorCode(err)
	message := ""
	if err != nil {
		var appErr *Error
		if errors.As(err, &appErr) && appErr.Unwrap() != nil {
			message = appErr.Unwrap().Error()
		} else {
			message = err.Error()
		}
	}
	return Payload{
		Schema:   "hvac-studio.error.v1",
		Code:     int(code),
		Kind:     CodeName(code),
		Message:  message,
		Problems: problems,
	}
}
