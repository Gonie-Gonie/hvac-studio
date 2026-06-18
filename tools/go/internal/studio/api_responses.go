package studio

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/goniegonie/hvac-studio/tools/go/internal/apperror"
	"github.com/goniegonie/hvac-studio/tools/go/internal/compiler"
	"github.com/goniegonie/hvac-studio/tools/go/internal/project"
)

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, err error) {
	writeErrorWithProblems(w, err, nil)
}

func writeTimeoutError(w http.ResponseWriter, workflow string, timeout time.Duration) {
	err := apperror.Errorf(apperror.CodeRuntime, "%s timed out after %s", workflow, formatTimeoutDuration(timeout))
	payload := apperror.PayloadFor(err, nil)
	writeJSON(w, http.StatusGatewayTimeout, apiError{
		OK:      false,
		Error:   payload,
		Code:    payload.Code,
		Kind:    payload.Kind,
		Message: payload.Message,
	})
}

func writeErrorWithProblems(w http.ResponseWriter, err error, problems []Problem) {
	code := apperror.ErrorCode(err)
	status := http.StatusInternalServerError
	switch code {
	case apperror.CodeValidation:
		status = http.StatusBadRequest
	case apperror.CodeInput:
		status = http.StatusUnprocessableEntity
	case apperror.CodePythonWorker:
		status = http.StatusBadGateway
	}
	payload := apperror.PayloadFor(err, toAppProblems(problems))
	writeJSON(w, status, apiError{
		OK:       false,
		Error:    payload,
		Code:     payload.Code,
		Kind:     payload.Kind,
		Message:  payload.Message,
		Problems: problems,
	})
}

func formatTimeoutDuration(timeout time.Duration) string {
	if timeout%time.Second == 0 {
		return fmt.Sprintf("%.0f seconds", timeout.Seconds())
	}
	return timeout.String()
}

func toAppProblems(problems []Problem) []apperror.Problem {
	if len(problems) == 0 {
		return nil
	}
	out := make([]apperror.Problem, 0, len(problems))
	for _, problem := range problems {
		out = append(out, apperror.Problem{
			Severity:    problem.Severity,
			Message:     problem.Message,
			ComponentID: problem.ComponentID,
			NodeID:      problem.NodeID,
			Source:      problem.Source,
			Line:        problem.Line,
			Column:      problem.Column,
		})
	}
	return out
}

func inferProblems(loaded *project.LoadedProject, err error) []Problem {
	message := fmt.Sprint(err)
	problem := Problem{Severity: "error", Message: message}
	if loaded == nil || loaded.Graph == nil {
		return []Problem{problem}
	}
	for _, component := range loaded.Graph.Components {
		if strings.Contains(message, component.ID) {
			problem.ComponentID = component.ID
			for _, node := range component.Nodes.Inputs {
				if strings.Contains(message, component.ID+"."+node.ID) || strings.Contains(message, " "+node.ID) {
					problem.NodeID = node.ID
					break
				}
			}
			if problem.NodeID == "" {
				for _, node := range component.Nodes.Outputs {
					if strings.Contains(message, component.ID+"."+node.ID) || strings.Contains(message, " "+node.ID) {
						problem.NodeID = node.ID
						break
					}
				}
			}
			break
		}
	}
	if location, ok := tracebackSourceLocation(loaded, message, problem.ComponentID); ok {
		problem.ComponentID = location.ComponentID
		problem.Source = location.Source
		problem.Line = location.Line
	}
	return []Problem{problem}
}

func compilerDiagnosticsProblems(diagnostics []compiler.Diagnostic) []Problem {
	problems := make([]Problem, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		problem := Problem{
			Severity:    defaultString(diagnostic.Severity, "warning"),
			Message:     diagnostic.Message,
			ComponentID: diagnostic.To.Component,
			NodeID:      diagnostic.To.Node,
		}
		if problem.ComponentID == "" {
			problem.ComponentID = diagnostic.From.Component
			problem.NodeID = diagnostic.From.Node
		}
		problems = append(problems, problem)
	}
	return problems
}
