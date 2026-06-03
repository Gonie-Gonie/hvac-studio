package runtime

import (
	"context"
	"fmt"
	"time"

	"github.com/goniegonie/hvac-studio/tools/go/internal/apperror"
	"github.com/goniegonie/hvac-studio/tools/go/internal/compiler"
	"github.com/goniegonie/hvac-studio/tools/go/internal/project"
	"github.com/goniegonie/hvac-studio/tools/go/internal/pythonworker"
)

type Session struct {
	loaded *project.LoadedProject
	plan   *compiler.Plan
	client *pythonworker.Client
	states map[string]map[string]any
}

func NewSession(ctx context.Context, loaded *project.LoadedProject) (*Session, error) {
	return newSession(ctx, loaded, map[string]any{})
}

func newSession(ctx context.Context, loaded *project.LoadedProject, initContext map[string]any) (*Session, error) {
	plan, err := compiler.Compile(loaded)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeValidation, err)
	}

	pythonExe := resolvePython(loaded.Root, loaded.Project.Environment)
	client, err := pythonworker.Start(ctx, pythonExe, loaded.Root)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodePythonWorker, err)
	}

	session := &Session{
		loaded: loaded,
		plan:   plan,
		client: client,
		states: map[string]map[string]any{},
	}
	if err := session.loadComponents(initContext); err != nil {
		_ = client.Close()
		return nil, err
	}
	return session, nil
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
		if component.Kind != "user_python" {
			return apperror.Errorf(apperror.CodeValidation, "component %s kind %q is not supported by the MVP runner", component.ID, component.Kind)
		}
		if component.Class == "" {
			return apperror.Errorf(apperror.CodeValidation, "component %s kind user_python requires class", component.ID)
		}
		if err := s.client.LoadComponent(component.ID, component.Class, s.loaded.Root); err != nil {
			return apperror.Wrap(apperror.CodePythonWorker, fmt.Errorf("load component %s: %w", component.ID, err))
		}
		state, err := s.client.InitializeComponent(component.ID, component.Parameters, initContext)
		if err != nil {
			return apperror.Wrap(apperror.CodePythonWorker, fmt.Errorf("initialize component %s: %w", component.ID, err))
		}
		if state == nil {
			state = map[string]any{}
		}
		s.states[component.ID] = state
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

	for _, componentID := range s.plan.Order {
		component := s.plan.Index.Components[componentID]
		componentInputs, err := collectInputs(component, s.plan, input.Inputs, componentOutputs)
		if err != nil {
			return nil, err
		}
		componentInputsByID[component.ID] = componentInputs
		nodeValues = append(nodeValues, nodeValueTraces(component.ID, "input", component.Nodes.Inputs, componentInputs)...)

		componentStarted := time.Now()
		outputs, nextState, err := s.client.EvaluateComponent(
			component.ID,
			componentInputs,
			s.states[component.ID],
			component.Parameters,
			input.Context,
		)
		timings = append(timings, ComponentTiming{
			Component:  component.ID,
			Stage:      "evaluate",
			DurationMS: durationMilliseconds(time.Since(componentStarted)),
		})
		if err != nil {
			return nil, apperror.Wrap(apperror.CodePythonWorker, fmt.Errorf("evaluate component %s: %w", component.ID, err))
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
		DurationMS:       durationMilliseconds(time.Since(started)),
	}, nil
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
