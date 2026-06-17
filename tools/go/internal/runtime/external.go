package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/goniegonie/hvac-studio/tools/go/internal/apperror"
	"github.com/goniegonie/hvac-studio/tools/go/internal/model"
	"github.com/goniegonie/hvac-studio/tools/go/internal/platform"
)

const defaultExternalTimeoutMS = 30000

type externalComponentRequest struct {
	ComponentID string         `json:"component_id"`
	Inputs      map[string]any `json:"inputs"`
	State       map[string]any `json:"state"`
	Params      map[string]any `json:"params"`
	Context     map[string]any `json:"context"`
}

type externalComponentResponse struct {
	OK      *bool              `json:"ok,omitempty"`
	Outputs map[string]any     `json:"outputs"`
	State   map[string]any     `json:"state,omitempty"`
	Logs    []ComponentLog     `json:"logs,omitempty"`
	Error   externalErrorShape `json:"error,omitempty"`
}

type externalErrorShape struct {
	Type    string `json:"type,omitempty"`
	Message string `json:"message,omitempty"`
}

func validateExternalComponentConfig(component model.Component) error {
	command, _, err := externalCommandSpec(component)
	if err != nil {
		return err
	}
	if strings.TrimSpace(command) == "" {
		return apperror.Errorf(apperror.CodeValidation, "component %s external executable parameter command is required", component.ID)
	}
	if _, err := externalTimeout(component); err != nil {
		return err
	}
	return nil
}

func (s *Session) evaluateExternalComponent(
	component model.Component,
	inputs map[string]any,
	contextValues map[string]any,
) (map[string]any, map[string]any, []ComponentLog, error) {
	command, args, err := externalCommandSpec(component)
	if err != nil {
		return nil, nil, nil, err
	}
	timeout, err := externalTimeout(component)
	if err != nil {
		return nil, nil, nil, err
	}
	commandPath := resolveExternalCommand(s.loaded.Root, command)
	request := externalComponentRequest{
		ComponentID: component.ID,
		Inputs:      inputs,
		State:       s.states[component.ID],
		Params:      component.Parameters,
		Context:     contextValues,
	}
	requestBytes, err := json.Marshal(request)
	if err != nil {
		return nil, nil, nil, err
	}
	requestBytes = append(requestBytes, '\n')

	ctx := s.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := platform.CommandContext(ctx, commandPath, args...)
	cmd.Dir = s.loaded.Root
	cmd.Stdin = bytes.NewReader(requestBytes)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	logs := externalLogs(component.ID, contextValues, stderr.String())
	if ctx.Err() == context.DeadlineExceeded {
		return nil, nil, logs, fmt.Errorf("external executable timed out after %s: %s", timeout, displayExternalCommand(command, args))
	}
	if runErr != nil {
		return nil, nil, logs, fmt.Errorf("external executable failed: %w; command=%s stderr=%s", runErr, displayExternalCommand(command, args), strings.TrimSpace(stderr.String()))
	}
	if strings.TrimSpace(stdout.String()) == "" {
		return nil, nil, logs, fmt.Errorf("external executable returned empty stdout: %s", displayExternalCommand(command, args))
	}

	var response externalComponentResponse
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		return nil, nil, logs, fmt.Errorf("decode external executable stdout as JSON: %w", err)
	}
	logs = append(logs, normalizeExternalLogs(component.ID, contextValues, response.Logs)...)
	if response.OK != nil && !*response.OK {
		message := response.Error.Message
		if message == "" {
			message = "external executable returned ok=false"
		}
		if response.Error.Type != "" {
			message = response.Error.Type + ": " + message
		}
		return nil, nil, logs, fmt.Errorf("%s", message)
	}
	if response.Outputs == nil {
		return nil, nil, logs, fmt.Errorf("external executable response outputs must be an object")
	}
	if response.State == nil {
		response.State = map[string]any{}
	}
	return response.Outputs, response.State, logs, nil
}

func externalCommandSpec(component model.Component) (string, []string, error) {
	params := component.Parameters
	if params == nil {
		params = map[string]any{}
	}
	command, ok := params["command"].(string)
	if !ok || strings.TrimSpace(command) == "" {
		return "", nil, nil
	}
	args, err := externalArgs(params["args"])
	if err != nil {
		return "", nil, apperror.Errorf(apperror.CodeValidation, "component %s external executable args must be an array of strings", component.ID)
	}
	return strings.TrimSpace(command), args, nil
}

func externalArgs(value any) ([]string, error) {
	if value == nil {
		return nil, nil
	}
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...), nil
	case []any:
		args := make([]string, 0, len(typed))
		for _, item := range typed {
			itemString, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("external args item is %T", item)
			}
			args = append(args, itemString)
		}
		return args, nil
	default:
		return nil, fmt.Errorf("external args is %T", value)
	}
}

func externalTimeout(component model.Component) (time.Duration, error) {
	value := defaultExternalTimeoutMS
	if component.Parameters != nil {
		if raw, ok := component.Parameters["timeout_ms"]; ok {
			parsed, err := numericParameter(raw)
			if err != nil {
				return 0, apperror.Errorf(apperror.CodeValidation, "component %s external executable timeout_ms must be numeric", component.ID)
			}
			value = parsed
		}
	}
	if value <= 0 {
		return 0, apperror.Errorf(apperror.CodeValidation, "component %s external executable timeout_ms must be greater than zero", component.ID)
	}
	return time.Duration(value) * time.Millisecond, nil
}

func numericParameter(value any) (int, error) {
	switch typed := value.(type) {
	case int:
		return typed, nil
	case int64:
		return int(typed), nil
	case float64:
		return int(typed), nil
	case json.Number:
		parsed, err := typed.Int64()
		return int(parsed), err
	case string:
		return strconv.Atoi(strings.TrimSpace(typed))
	default:
		return 0, fmt.Errorf("unsupported numeric parameter %T", value)
	}
}

func resolveExternalCommand(projectRoot string, command string) string {
	if filepath.IsAbs(command) {
		return command
	}
	if strings.ContainsAny(command, `/\`) {
		return filepath.Join(projectRoot, filepath.FromSlash(command))
	}
	candidate := filepath.Join(projectRoot, command)
	if _, err := exec.LookPath(candidate); err == nil {
		return candidate
	}
	return command
}

func displayExternalCommand(command string, args []string) string {
	if len(args) == 0 {
		return command
	}
	return strings.TrimSpace(command + " " + strings.Join(args, " "))
}

func externalLogs(componentID string, context map[string]any, stderr string) []ComponentLog {
	logs := []ComponentLog{}
	for _, line := range strings.Split(stderr, "\n") {
		message := strings.TrimRight(line, "\r")
		if strings.TrimSpace(message) == "" {
			continue
		}
		logs = append(logs, ComponentLog{
			Component: componentID,
			Stage:     "external_executable",
			Stream:    "stderr",
			Severity:  "error",
			Message:   message,
			Time:      componentLogTime(nil, context),
		})
	}
	return logs
}

func normalizeExternalLogs(componentID string, context map[string]any, entries []ComponentLog) []ComponentLog {
	logs := make([]ComponentLog, 0, len(entries))
	for _, entry := range entries {
		if strings.TrimSpace(entry.Message) == "" {
			continue
		}
		if entry.Component == "" {
			entry.Component = componentID
		}
		if entry.Stage == "" {
			entry.Stage = "external_executable"
		}
		if entry.Severity == "" {
			entry.Severity = "info"
		}
		entry.Time = componentLogTime(entry.Time, context)
		logs = append(logs, entry)
	}
	return logs
}
