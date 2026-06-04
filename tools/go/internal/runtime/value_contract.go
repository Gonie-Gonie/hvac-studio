package runtime

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"

	"github.com/goniegonie/hvac-studio/tools/go/internal/apperror"
	"github.com/goniegonie/hvac-studio/tools/go/internal/model"
)

func applyConnectionUnitConversion(connection model.Connection, value any) (any, error) {
	conversion := connection.UnitConversion
	if conversion == nil {
		return value, nil
	}
	mode := strings.TrimSpace(conversion.Mode)
	if mode == "" {
		mode = "linear"
	}
	if mode != "linear" {
		return nil, apperror.Errorf(apperror.CodeValidation, "connection %s unit_conversion mode is unsupported: %s", connection.ID, mode)
	}
	number, ok := numericValue(value)
	if !ok {
		return nil, apperror.Errorf(apperror.CodeRuntime, "connection %s unit conversion requires a numeric value, got %T", connection.ID, value)
	}
	factor := 1.0
	if conversion.Factor != nil {
		factor = *conversion.Factor
	}
	offset := 0.0
	if conversion.Offset != nil {
		offset = *conversion.Offset
	}
	return number*factor + offset, nil
}

func validateInputValue(componentID string, node model.Node, value any, code apperror.Code) error {
	if err := validateValueType(value, node.ValueType); err != nil {
		return apperror.Errorf(code, "component %s input %s expects %s, got %T", componentID, node.ID, node.ValueType, value)
	}
	return nil
}

func validateOutputValue(component model.Component, node model.Node, value any) error {
	if err := validateValueType(value, node.ValueType); err != nil {
		code := apperror.CodePythonWorker
		if component.Kind == "external_exe" {
			code = apperror.CodeRuntime
		}
		return apperror.Errorf(code, "component %s output %s expects %s, got %T", component.ID, node.ID, node.ValueType, value)
	}
	return nil
}

func validateValueType(value any, valueType string) error {
	valueType = strings.ToLower(strings.TrimSpace(valueType))
	if valueType == "" {
		return nil
	}
	switch valueType {
	case "float", "number", "scalar":
		if _, ok := numericValue(value); ok {
			return nil
		}
	case "integer", "int":
		if number, ok := numericValue(value); ok && math.Trunc(number) == number {
			return nil
		}
	case "boolean", "bool":
		if _, ok := value.(bool); ok {
			return nil
		}
	case "string":
		if _, ok := value.(string); ok {
			return nil
		}
	case "array", "list":
		if isArrayValue(value) {
			return nil
		}
	case "object", "map":
		if isMapValue(value) {
			return nil
		}
	default:
		return nil
	}
	return fmt.Errorf("value does not match %s", valueType)
}

func numericValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case int:
		return float64(typed), true
	case int8:
		return float64(typed), true
	case int16:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case uint:
		return float64(typed), true
	case uint8:
		return float64(typed), true
	case uint16:
		return float64(typed), true
	case uint32:
		return float64(typed), true
	case uint64:
		return float64(typed), true
	case float32:
		return float64(typed), true
	case float64:
		return typed, true
	case json.Number:
		number, err := typed.Float64()
		return number, err == nil
	default:
		return 0, false
	}
}

func isArrayValue(value any) bool {
	if value == nil {
		return false
	}
	kind := reflect.TypeOf(value).Kind()
	return kind == reflect.Array || kind == reflect.Slice
}

func isMapValue(value any) bool {
	if value == nil {
		return false
	}
	return reflect.TypeOf(value).Kind() == reflect.Map
}
