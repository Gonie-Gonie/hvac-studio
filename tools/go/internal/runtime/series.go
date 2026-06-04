package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/goniegonie/hvac-studio/tools/go/internal/apperror"
	"github.com/goniegonie/hvac-studio/tools/go/internal/project"
)

type SeriesInput struct {
	SchemaVersion string         `json:"schema_version,omitempty"`
	Context       map[string]any `json:"context,omitempty"`
	Steps         []SeriesStep   `json:"steps"`
}

type SeriesStep struct {
	ID      string         `json:"id,omitempty"`
	Inputs  map[string]any `json:"inputs,omitempty"`
	Context map[string]any `json:"context,omitempty"`
}

type SeriesResult struct {
	OK             bool                      `json:"ok"`
	ParameterSet   string                    `json:"parameter_set,omitempty"`
	StepCount      int                       `json:"step_count"`
	Outputs        map[string][]any          `json:"outputs"`
	Series         []SeriesPoint             `json:"series"`
	FinalStates    map[string]map[string]any `json:"final_states"`
	ExecutionOrder []string                  `json:"execution_order"`
	DurationMS     float64                   `json:"duration_ms,omitempty"`
}

type SeriesPoint struct {
	Index            int                       `json:"index"`
	ID               string                    `json:"id,omitempty"`
	Time             any                       `json:"time,omitempty"`
	Inputs           map[string]any            `json:"inputs"`
	Context          map[string]any            `json:"context"`
	Outputs          map[string]any            `json:"outputs"`
	ComponentInputs  map[string]map[string]any `json:"component_inputs,omitempty"`
	ComponentOutputs map[string]map[string]any `json:"component_outputs,omitempty"`
	NodeValues       []NodeValueTrace          `json:"node_values,omitempty"`
	ConnectionValues []ConnectionValueTrace    `json:"connection_values,omitempty"`
	States           map[string]map[string]any `json:"states"`
	ExecutionOrder   []string                  `json:"execution_order,omitempty"`
	ComponentTimings []ComponentTiming         `json:"component_timings,omitempty"`
	ComponentLogs    []ComponentLog            `json:"component_logs,omitempty"`
	DurationMS       float64                   `json:"duration_ms,omitempty"`
}

func RunSeries(ctx context.Context, loaded *project.LoadedProject, input SeriesInput) (*SeriesResult, error) {
	started := time.Now()
	if len(input.Steps) == 0 {
		return nil, apperror.Errorf(apperror.CodeInput, "series input requires at least one step")
	}
	if input.Context == nil {
		input.Context = map[string]any{}
	}

	session, err := newSession(ctx, loaded, cloneAnyMap(input.Context))
	if err != nil {
		return nil, err
	}
	defer session.Close()

	result := &SeriesResult{
		OK:             true,
		StepCount:      len(input.Steps),
		Outputs:        map[string][]any{},
		Series:         []SeriesPoint{},
		FinalStates:    map[string]map[string]any{},
		ExecutionOrder: append([]string(nil), session.plan.Order...),
	}
	for index, step := range input.Steps {
		stepInputs := step.Inputs
		if stepInputs == nil {
			stepInputs = map[string]any{}
		}
		stepContext := mergeContext(input.Context, step.Context)
		stepResult, err := session.Evaluate(RunInput{
			Inputs:  stepInputs,
			Context: stepContext,
		})
		if err != nil {
			return nil, err
		}

		for name, value := range stepResult.Outputs {
			result.Outputs[name] = append(result.Outputs[name], value)
		}
		result.FinalStates = stepResult.States
		result.Series = append(result.Series, SeriesPoint{
			Index:            index,
			ID:               step.ID,
			Time:             stepResult.Context["time"],
			Inputs:           stepInputs,
			Context:          stepResult.Context,
			Outputs:          stepResult.Outputs,
			ComponentInputs:  stepResult.ComponentInputs,
			ComponentOutputs: stepResult.ComponentOutputs,
			NodeValues:       stepResult.NodeValues,
			ConnectionValues: stepResult.ConnectionValues,
			States:           stepResult.States,
			ExecutionOrder:   stepResult.ExecutionOrder,
			ComponentTimings: stepResult.ComponentTimings,
			ComponentLogs:    stepResult.ComponentLogs,
			DurationMS:       stepResult.DurationMS,
		})
	}
	result.DurationMS = durationMilliseconds(time.Since(started))
	return result, nil
}

func LoadSeriesInput(inputPath string) (SeriesInput, error) {
	inputBytes, err := os.ReadFile(inputPath)
	if err != nil {
		return SeriesInput{}, apperror.Wrap(apperror.CodeInput, err)
	}
	var structured SeriesInput
	if err := json.Unmarshal(inputBytes, &structured); err != nil {
		return SeriesInput{}, apperror.Wrap(apperror.CodeInput, err)
	}
	if len(structured.Steps) == 0 {
		return SeriesInput{}, apperror.Errorf(apperror.CodeInput, "series input requires steps")
	}
	if structured.Context == nil {
		structured.Context = map[string]any{}
	}
	return structured, nil
}

func WriteSeriesResult(outputPath string, result *SeriesResult) error {
	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	if outputPath == "" {
		fmt.Println(string(output))
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(outputPath, append(output, '\n'), 0o644)
}

func mergeContext(base map[string]any, override map[string]any) map[string]any {
	merged := cloneAnyMap(base)
	for name, value := range override {
		merged[name] = value
	}
	return merged
}

func cloneAnyMap(source map[string]any) map[string]any {
	cloned := map[string]any{}
	for name, value := range source {
		cloned[name] = value
	}
	return cloned
}
