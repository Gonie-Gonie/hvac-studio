package studio

import (
	"strings"

	"github.com/goniegonie/hvac-studio/tools/go/internal/apperror"
	"github.com/goniegonie/hvac-studio/tools/go/internal/model"
)

func normalizeParameterDefinition(name string, definition model.ParameterDefinition, current any, hasCurrent bool) model.ParameterDefinition {
	if strings.TrimSpace(definition.DisplayName) == "" {
		definition.DisplayName = displayNameFromID(name)
	}
	definition.Role = strings.TrimSpace(definition.Role)
	if definition.Current == nil && hasCurrent {
		definition.Current = current
	}
	if definition.Default == nil && hasCurrent {
		definition.Default = current
	}
	return definition
}

func validateParameterDefinition(componentID string, name string, definition model.ParameterDefinition) error {
	if definition.Role != "" && !isValidParameterRole(definition.Role) {
		return apperror.Errorf(apperror.CodeValidation, "parameter role is invalid: %s.%s", componentID, name)
	}
	if definition.Bounds == nil {
		return nil
	}
	hasMin := definition.Bounds.Min != nil
	hasMax := definition.Bounds.Max != nil
	var minValue, maxValue float64
	if hasMin {
		var ok bool
		minValue, ok = studioNumberValue(definition.Bounds.Min)
		if !ok {
			return apperror.Errorf(apperror.CodeValidation, "parameter bounds min must be numeric: %s.%s", componentID, name)
		}
	}
	if hasMax {
		var ok bool
		maxValue, ok = studioNumberValue(definition.Bounds.Max)
		if !ok {
			return apperror.Errorf(apperror.CodeValidation, "parameter bounds max must be numeric: %s.%s", componentID, name)
		}
	}
	if hasMin && hasMax && minValue > maxValue {
		return apperror.Errorf(apperror.CodeValidation, "parameter bounds min must be <= max: %s.%s", componentID, name)
	}
	return nil
}

func isValidParameterRole(role string) bool {
	switch role {
	case "fixed", "scenario_input", "calibration_target", "optimization_variable", "derived":
		return true
	default:
		return false
	}
}

func normalizeStateDefinition(name string, definition model.StateDefinition) model.StateDefinition {
	if strings.TrimSpace(definition.DisplayName) == "" {
		definition.DisplayName = displayNameFromID(name)
	}
	return definition
}
