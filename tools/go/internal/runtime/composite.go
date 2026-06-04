package runtime

import (
	"fmt"
	"strings"

	"github.com/goniegonie/hvac-studio/tools/go/internal/apperror"
	"github.com/goniegonie/hvac-studio/tools/go/internal/model"
)

func validateCompositeComponentConfig(component model.Component) error {
	if component.Composite == nil || strings.TrimSpace(component.Composite.System) == "" {
		return apperror.Errorf(apperror.CodeValidation, "component %s kind composite requires composite.system", component.ID)
	}
	return nil
}

func (s *Session) evaluateCompositeComponent(
	component model.Component,
	inputs map[string]any,
	contextValues map[string]any,
) (map[string]any, map[string]any, []ComponentLog, error) {
	if err := validateCompositeComponentConfig(component); err != nil {
		return nil, nil, nil, err
	}
	childSystem := strings.TrimSpace(component.Composite.System)
	if containsString(s.stack, childSystem) {
		return nil, nil, nil, fmt.Errorf("composite system recursion detected: %s -> %s", strings.Join(s.stack, " -> "), childSystem)
	}

	projectCopy := *s.loaded.Project
	projectCopy.EntrySystem = childSystem
	loadedCopy := *s.loaded
	loadedCopy.Project = &projectCopy

	childSession, err := newSessionWithStack(s.ctx, &loadedCopy, cloneAnyMap(contextValues), appendString(s.stack, childSystem))
	if err != nil {
		return nil, nil, nil, err
	}
	defer childSession.Close()
	childSession.applyStates(nestedCompositeStates(s.states[component.ID]))

	childResult, err := childSession.Evaluate(RunInput{
		Inputs:  cloneAnyMap(inputs),
		Context: cloneAnyMap(contextValues),
	})
	if err != nil {
		return nil, nil, childSessionLogs(childResult), err
	}

	nextState := map[string]any{
		"system": childSystem,
		"states": cloneStates(childResult.States),
	}
	return childResult.Outputs, nextState, childResult.ComponentLogs, nil
}

func (s *Session) applyStates(states map[string]map[string]any) {
	for componentID, state := range states {
		if _, exists := s.states[componentID]; exists {
			s.states[componentID] = cloneAnyMap(state)
		}
	}
}

func nestedCompositeStates(state map[string]any) map[string]map[string]any {
	raw, ok := state["states"]
	if !ok || raw == nil {
		return nil
	}
	if typed, ok := raw.(map[string]map[string]any); ok {
		return cloneStates(typed)
	}
	if generic, ok := raw.(map[string]any); ok {
		states := map[string]map[string]any{}
		for componentID, rawState := range generic {
			if componentState, ok := rawState.(map[string]any); ok {
				states[componentID] = cloneAnyMap(componentState)
			}
		}
		return states
	}
	return nil
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func appendString(values []string, value string) []string {
	next := append([]string(nil), values...)
	return append(next, value)
}

func childSessionLogs(result *RunResult) []ComponentLog {
	if result == nil {
		return nil
	}
	return result.ComponentLogs
}
