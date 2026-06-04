package runtime

import (
	"context"
	"fmt"
	"time"

	"github.com/goniegonie/hvac-studio/tools/go/internal/apperror"
	"github.com/goniegonie/hvac-studio/tools/go/internal/compiler"
	"github.com/goniegonie/hvac-studio/tools/go/internal/model"
	"github.com/goniegonie/hvac-studio/tools/go/internal/project"
	"github.com/goniegonie/hvac-studio/tools/go/internal/pythonworker"
)

type Session struct {
	ctx    context.Context
	loaded *project.LoadedProject
	plan   *compiler.Plan
	client *pythonworker.Client
	states map[string]map[string]any
	logs   []ComponentLog
}

func NewSession(ctx context.Context, loaded *project.LoadedProject) (*Session, error) {
	return newSession(ctx, loaded, map[string]any{})
}

func newSession(ctx context.Context, loaded *project.LoadedProject, initContext map[string]any) (*Session, error) {
	plan, err := compiler.Compile(loaded)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeValidation, err)
	}

	session := &Session{
		ctx:    ctx,
		loaded: loaded,
		plan:   plan,
		states: map[string]map[string]any{},
	}
	if session.requiresPythonWorker() {
		pythonExe := resolvePython(loaded.Root, loaded.Project.Environment)
		client, err := pythonworker.Start(ctx, pythonExe, loaded.Root)
		if err != nil {
			return nil, apperror.Wrap(apperror.CodePythonWorker, err)
		}
		session.client = client
	}
	if err := session.loadComponents(initContext); err != nil {
		_ = session.Close()
		return nil, err
	}
	return session, nil
}

func (s *Session) requiresPythonWorker() bool {
	for _, componentID := range s.plan.Order {
		component := s.plan.Index.Components[componentID]
		if component.Kind == "user_python" {
			return true
		}
	}
	return false
}

func (s *Session) loadComponents(initContext map[string]any) error {
	if initContext == nil {
		initContext = map[string]any{}
	}
	for _, componentID := range s.plan.Order {
		component := s.plan.Index.Components[componentID]
		if component.Parameters == nil {
			component.Parameters = map[string]any{}
		}
		switch component.Kind {
		case "user_python":
			if component.ExecutionMode == "external_executable" {
				return apperror.Errorf(apperror.CodeValidation, "component %s kind user_python cannot use external_executable mode", component.ID)
			}
			if component.Class == "" {
				return apperror.Errorf(apperror.CodeValidation, "component %s kind user_python requires class", component.ID)
			}
			loadLogs, err := s.client.LoadComponent(component.ID, component.Class, s.loaded.Root)
			s.logs = append(s.logs, componentLogsFromWorker(loadLogs)...)
			if err != nil {
				return apperror.Wrap(apperror.CodePythonWorker, fmt.Errorf("load component %s: %w", component.ID, err))
			}
			state, initLogs, err := s.client.InitializeComponent(component.ID, component.Parameters, initContext)
			s.logs = append(s.logs, componentLogsFromWorker(initLogs)...)
			if err != nil {
				return apperror.Wrap(apperror.CodePythonWorker, fmt.Errorf("initialize component %s: %w", component.ID, err))
			}
			if state == nil {
				state = map[string]any{}
			}
			s.states[component.ID] = state
		case "external_exe":
			if component.ExecutionMode != "" && component.ExecutionMode != "external_executable" {
				return apperror.Errorf(apperror.CodeValidation, "component %s kind external_exe requires external_executable mode", component.ID)
			}
			if err := validateExternalComponentConfig(component); err != nil {
				return err
			}
			s.states[component.ID] = map[string]any{}
		default:
			return apperror.Errorf(apperror.CodeValidation, "component %s kind %q is not supported by the runner", component.ID, component.Kind)
		}
	}
	return nil
}

func (s *Session) Evaluate(input RunInput) (*RunResult, error) {
	started := time.Now()
	if input.Inputs == nil {
		input.Inputs = map[string]any{}
	}
	if input.Context == nil {
		input.Context = map[string]any{}
	}

	componentInputsByID := map[string]map[string]any{}
	componentOutputs := map[string]map[string]any{}
	nodeValues := []NodeValueTrace{}
	timings := []ComponentTiming{}
	logs := append([]ComponentLog(nil), s.logs...)

	for _, componentID := range s.plan.Order {
		component := s.plan.Index.Components[componentID]
		componentInputs, err := collectInputs(component, s.plan, input.Inputs, componentOutputs)
		if err != nil {
			return nil, err
		}
		componentInputsByID[component.ID] = componentInputs
		nodeValues = append(nodeValues, nodeValueTraces(component.ID, "input", component.Nodes.Inputs, componentInputs)...)

		componentStarted := time.Now()
		outputs, nextState, evalLogs, stage, err := s.evaluateComponent(component, componentInputs, input.Context)
		logs = append(logs, evalLogs...)
		timings = append(timings, ComponentTiming{
			Component:  component.ID,
			Stage:      stage,
			DurationMS: durationMilliseconds(time.Since(componentStarted)),
		})
		if err != nil {
			return nil, err
		}
		if outputs == nil {
			outputs = map[string]any{}
		}
		if nextState == nil {
			nextState = map[string]any{}
		}
		if err := validateOutputs(component, outputs); err != nil {
			return nil, err
		}
		componentOutputs[component.ID] = outputs
		nodeValues = append(nodeValues, nodeValueTraces(component.ID, "output", component.Nodes.Outputs, outputs)...)
		s.states[component.ID] = nextState
	}

	publicOutputs := map[string]any{}
	for _, output := range s.plan.System.PublicOutputs {
		componentValues := componentOutputs[output.Component]
		value, ok := componentValues[output.Node]
		if !ok {
			return nil, apperror.Errorf(apperror.CodeRuntime, "public output %s could not read %s.%s", output.ID, output.Component, output.Node)
		}
		publicOutputs[output.ID] = value
	}

	return &RunResult{
		OK:               true,
		Outputs:          publicOutputs,
		ComponentInputs:  componentInputsByID,
		ComponentOutputs: componentOutputs,
		NodeValues:       nodeValues,
		ConnectionValues: connectionValueTraces(s.plan, componentOutputs),
		States:           cloneStates(s.states),
		Context:          input.Context,
		ExecutionOrder:   s.plan.Order,
		ComponentTimings: timings,
		ComponentLogs:    logs,
		DurationMS:       durationMilliseconds(time.Since(started)),
	}, nil
}

func (s *Session) evaluateComponent(
	component model.Component,
	inputs map[string]any,
	context map[string]any,
) (map[string]any, map[string]any, []ComponentLog, string, error) {
	switch component.Kind {
	case "user_python":
		evaluate := s.client.EvaluateComponent
		stage := "evaluate"
		if component.ExecutionMode == "vectorized" {
			evaluate = s.client.EvaluateComponentBatch
			stage = "evaluate_batch"
		}
		outputs, nextState, evalLogs, err := evaluate(component.ID, inputs, s.states[component.ID], component.Parameters, context)
		logs := componentLogsFromWorker(evalLogs)
		if err != nil {
			return nil, nil, logs, stage, apperror.Wrap(apperror.CodePythonWorker, fmt.Errorf("evaluate component %s: %w", component.ID, err))
		}
		return outputs, nextState, logs, stage, nil
	case "external_exe":
		stage := "external_executable"
		outputs, nextState, logs, err := s.evaluateExternalComponent(component, inputs, context)
		if err != nil {
			return nil, nil, logs, stage, apperror.Wrap(apperror.CodeRuntime, fmt.Errorf("evaluate component %s: %w", component.ID, err))
		}
		return outputs, nextState, logs, stage, nil
	default:
		stage := "evaluate"
		return nil, nil, nil, stage, apperror.Errorf(apperror.CodeValidation, "component %s kind %q is not supported by the runner", component.ID, component.Kind)
	}
}

func (s *Session) Close() error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Close()
}

func cloneStates(states map[string]map[string]any) map[string]map[string]any {
	cloned := map[string]map[string]any{}
	for componentID, state := range states {
		cloned[componentID] = map[string]any{}
		for name, value := range state {
			cloned[componentID][name] = value
		}
	}
	return cloned
}

func durationMilliseconds(duration time.Duration) float64 {
	return float64(duration.Microseconds()) / 1000
}

func componentLogsFromWorker(entries []pythonworker.LogEntry) []ComponentLog {
	logs := make([]ComponentLog, 0, len(entries))
	for _, entry := range entries {
		if entry.Message == "" {
			continue
		}
		component := entry.ComponentID
		if component == "" {
			component = "component"
		}
		stage := entry.Stage
		if stage == "" {
			stage = "evaluate"
		}
		severity := entry.Severity
		if severity == "" {
			severity = "info"
			if entry.Stream == "stderr" {
				severity = "error"
			}
		}
		logs = append(logs, ComponentLog{
			Component: component,
			Stage:     stage,
			Stream:    entry.Stream,
			Severity:  severity,
			Message:   entry.Message,
		})
	}
	return logs
}
