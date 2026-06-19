package studio

import (
	"encoding/json"

	"github.com/goniegonie/hvac-studio/tools/go/internal/calibration"
	"github.com/goniegonie/hvac-studio/tools/go/internal/model"
	"github.com/goniegonie/hvac-studio/tools/go/internal/optimization"
	runtimecore "github.com/goniegonie/hvac-studio/tools/go/internal/runtime"
)

type apiRequest struct {
	ProjectPath      string         `json:"project_path"`
	Inputs           map[string]any `json:"inputs"`
	Context          map[string]any `json:"context"`
	Save             bool           `json:"save"`
	ParameterSetPath string         `json:"parameter_set_path"`
	TimeoutMS        int            `json:"timeout_ms"`
}

type seriesRunRequest struct {
	ProjectPath      string                   `json:"project_path"`
	InputPath        string                   `json:"input_path"`
	SchemaVersion    string                   `json:"schema_version,omitempty"`
	Context          map[string]any           `json:"context"`
	Steps            []runtimecore.SeriesStep `json:"steps"`
	ParameterSetPath string                   `json:"parameter_set_path"`
	TimeoutMS        int                      `json:"timeout_ms"`
}

type createProjectRequest struct {
	Name     string `json:"name"`
	Template string `json:"template"`
}

type copyProjectRequest struct {
	ProjectPath string `json:"project_path"`
	Name        string `json:"name"`
}

type createComponentRequest struct {
	ProjectPath     string `json:"project_path"`
	Name            string `json:"name"`
	Template        string `json:"template"`
	IncludeInSystem bool   `json:"include_in_system"`
}

type duplicateComponentRequest struct {
	ProjectPath       string `json:"project_path"`
	SourceComponentID string `json:"source_component_id"`
	Name              string `json:"name"`
}

type replaceComponentRequest struct {
	ProjectPath   string `json:"project_path"`
	ComponentID   string `json:"component_id"`
	Name          string `json:"name"`
	Template      string `json:"template"`
	MapParameters *bool  `json:"map_parameters,omitempty"`
}

type ComponentReplacementSummary struct {
	OriginalComponent         string                        `json:"original_component"`
	ReplacementComponent      string                        `json:"replacement_component"`
	Template                  string                        `json:"template,omitempty"`
	SystemReplaced            bool                          `json:"system_replaced"`
	RewiredConnections        int                           `json:"rewired_connections"`
	RewiredPublicInputs       int                           `json:"rewired_public_inputs"`
	RewiredPublicOutputs      int                           `json:"rewired_public_outputs"`
	MappedParameters          int                           `json:"mapped_parameters"`
	OriginalComponentRetained bool                          `json:"original_component_retained"`
	MapParameters             bool                          `json:"map_parameters"`
	NodeMappings              []ComponentReplacementMapping `json:"node_mappings,omitempty"`
	ParameterMappings         []ComponentReplacementMapping `json:"parameter_mappings,omitempty"`
	Diff                      ComponentReplacementDiff      `json:"diff"`
	Problems                  []Problem                     `json:"problems,omitempty"`
}

type ComponentReplacementMapping struct {
	Scope  string `json:"scope"`
	ID     string `json:"id"`
	From   string `json:"from"`
	To     string `json:"to"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

type ComponentReplacementDiff struct {
	OriginalInputs        []string `json:"original_inputs,omitempty"`
	ReplacementInputs     []string `json:"replacement_inputs,omitempty"`
	MatchedInputs         []string `json:"matched_inputs,omitempty"`
	MissingInputs         []string `json:"missing_inputs,omitempty"`
	AddedInputs           []string `json:"added_inputs,omitempty"`
	OriginalOutputs       []string `json:"original_outputs,omitempty"`
	ReplacementOutputs    []string `json:"replacement_outputs,omitempty"`
	MatchedOutputs        []string `json:"matched_outputs,omitempty"`
	MissingOutputs        []string `json:"missing_outputs,omitempty"`
	AddedOutputs          []string `json:"added_outputs,omitempty"`
	OriginalParameters    []string `json:"original_parameters,omitempty"`
	ReplacementParameters []string `json:"replacement_parameters,omitempty"`
	MatchedParameters     []string `json:"matched_parameters,omitempty"`
	MissingParameters     []string `json:"missing_parameters,omitempty"`
	AddedParameters       []string `json:"added_parameters,omitempty"`
}

type updateComponentRequest struct {
	ProjectPath string `json:"project_path"`
	ComponentID string `json:"component_id"`
	Name        string `json:"name"`
}

type deleteComponentRequest struct {
	ProjectPath string `json:"project_path"`
	ComponentID string `json:"component_id"`
}

type includeComponentRequest struct {
	ProjectPath string `json:"project_path"`
	SystemID    string `json:"system_id"`
	ComponentID string `json:"component_id"`
}

type createNodeRequest struct {
	ProjectPath string `json:"project_path"`
	ComponentID string `json:"component_id"`
	Direction   string `json:"direction"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	Preset      string `json:"preset"`
	Medium      string `json:"medium"`
	ValueType   string `json:"value_type"`
	Unit        string `json:"unit"`
	Required    *bool  `json:"required"`
	Default     any    `json:"default"`
}

type deleteNodeRequest struct {
	ProjectPath string `json:"project_path"`
	ComponentID string `json:"component_id"`
	NodeID      string `json:"node_id"`
}

type updateNodeRequest struct {
	ProjectPath     string `json:"project_path"`
	ComponentID     string `json:"component_id"`
	NodeID          string `json:"node_id"`
	NewID           string `json:"new_id"`
	Name            string `json:"name"`
	Medium          string `json:"medium"`
	ValueType       string `json:"value_type"`
	Unit            string `json:"unit"`
	Required        *bool  `json:"required"`
	Default         any    `json:"default"`
	DefaultProvided bool   `json:"-"`
}

func (r *updateNodeRequest) UnmarshalJSON(data []byte) error {
	type request updateNodeRequest
	var decoded request
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*r = updateNodeRequest(decoded)
	_, r.DefaultProvided = raw["default"]
	return nil
}

type createConnectionRequest struct {
	ProjectPath   string `json:"project_path"`
	SystemID      string `json:"system_id"`
	FromComponent string `json:"from_component"`
	FromNode      string `json:"from_node"`
	ToComponent   string `json:"to_component"`
	ToNode        string `json:"to_node"`
}

type updateConnectionRequest struct {
	ProjectPath              string                `json:"project_path"`
	SystemID                 string                `json:"system_id"`
	ConnectionID             string                `json:"connection_id"`
	UnitConversion           *model.UnitConversion `json:"unit_conversion"`
	UnitConversionWasPresent bool                  `json:"-"`
}

func (r *updateConnectionRequest) UnmarshalJSON(data []byte) error {
	type request updateConnectionRequest
	var decoded request
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*r = updateConnectionRequest(decoded)
	_, r.UnitConversionWasPresent = raw["unit_conversion"]
	return nil
}

type deleteConnectionRequest struct {
	ProjectPath  string `json:"project_path"`
	SystemID     string `json:"system_id"`
	ConnectionID string `json:"connection_id"`
}

type updateLayoutRequest struct {
	ProjectPath string                    `json:"project_path"`
	Components  map[string]CanvasPosition `json:"components"`
}

type exportRequest struct {
	ProjectPath               string `json:"project_path"`
	Profile                   string `json:"profile"`
	IncludeDatasets           *bool  `json:"include_datasets,omitempty"`
	IncludeCalibrationSetups  *bool  `json:"include_calibration_setups,omitempty"`
	IncludeOptimizationSetups *bool  `json:"include_optimization_setups,omitempty"`
	IncludeMLAssets           *bool  `json:"include_ml_assets,omitempty"`
	IncludeSDKExamples        *bool  `json:"include_sdk_examples,omitempty"`
	IncludeRecords            bool   `json:"include_records"`
}

type exportOptions struct {
	IncludeDatasets           bool
	IncludeCalibrationSetups  bool
	IncludeOptimizationSetups bool
	IncludeMLAssets           bool
	IncludeSDKExamples        bool
	IncludeRecords            bool
}

func exportOptionsFromRequest(req exportRequest) exportOptions {
	return exportOptions{
		IncludeDatasets:           boolOption(req.IncludeDatasets, true),
		IncludeCalibrationSetups:  boolOption(req.IncludeCalibrationSetups, true),
		IncludeOptimizationSetups: boolOption(req.IncludeOptimizationSetups, true),
		IncludeMLAssets:           boolOption(req.IncludeMLAssets, true),
		IncludeSDKExamples:        boolOption(req.IncludeSDKExamples, true),
		IncludeRecords:            req.IncludeRecords,
	}
}

func boolOption(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

type sourceRequest struct {
	ProjectPath string `json:"project_path"`
	ComponentID string `json:"component_id"`
	Content     string `json:"content"`
}

type sourceCheckRequest struct {
	ProjectPath string `json:"project_path"`
	ComponentID string `json:"component_id"`
	Content     string `json:"content"`
}

type createScenarioRequest struct {
	ProjectPath string         `json:"project_path"`
	Name        string         `json:"name"`
	Inputs      map[string]any `json:"inputs"`
	Context     map[string]any `json:"context"`
}

type updateParametersRequest struct {
	ProjectPath string                    `json:"project_path"`
	Parameters  map[string]map[string]any `json:"parameters"`
}

type updateComponentContractRequest struct {
	ProjectPath                string                               `json:"project_path"`
	ComponentID                string                               `json:"component_id"`
	Parameters                 map[string]any                       `json:"parameters"`
	ParameterDefinitions       map[string]model.ParameterDefinition `json:"parameter_defs"`
	DeleteParameterDefinitions []string                             `json:"delete_parameter_defs"`
	StateDefinitions           map[string]model.StateDefinition     `json:"state_defs"`
	DeleteStateDefinitions     []string                             `json:"delete_state_defs"`
}

type applyParameterSetRequest struct {
	ProjectPath string `json:"project_path"`
	Path        string `json:"path"`
}

type deleteParameterRequest struct {
	ProjectPath string `json:"project_path"`
	ComponentID string `json:"component_id"`
	Name        string `json:"name"`
}

type importDatasetRequest struct {
	ProjectPath string `json:"project_path"`
	SourcePath  string `json:"source_path"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	Delimiter   string `json:"delimiter"`
	Encoding    string `json:"encoding"`
}

type createValidationMappingRequest struct {
	ProjectPath           string            `json:"project_path"`
	DatasetPath           string            `json:"dataset_path"`
	ID                    string            `json:"id"`
	Name                  string            `json:"name"`
	TimeColumn            string            `json:"time_column"`
	InputColumns          map[string]string `json:"input_columns"`
	ObservedOutputColumns map[string]string `json:"observed_output_columns"`
	UnitHints             map[string]string `json:"unit_hints"`
	MissingValuePolicy    string            `json:"missing_value_policy"`
}

type updateValidationMappingRequest struct {
	ProjectPath string `json:"project_path"`
	MappingPath string `json:"mapping_path"`
	Name        string `json:"name"`
}

type copyValidationMappingRequest struct {
	ProjectPath string `json:"project_path"`
	MappingPath string `json:"mapping_path"`
	Name        string `json:"name"`
}

type deleteValidationMappingRequest struct {
	ProjectPath string `json:"project_path"`
	MappingPath string `json:"mapping_path"`
}

type createCalibrationSetupRequest struct {
	ProjectPath      string                      `json:"project_path"`
	MappingPath      string                      `json:"mapping_path"`
	ID               string                      `json:"id"`
	Name             string                      `json:"name"`
	Algorithm        string                      `json:"algorithm"`
	BaseParameterSet string                      `json:"base_parameter_set"`
	ObjectiveOutputs map[string]float64          `json:"objective_outputs"`
	Parameters       []calibration.ParameterSpec `json:"parameters"`
	StoppingRules    calibration.StoppingRules   `json:"stopping_rules"`
}

type createOptimizationSetupRequest struct {
	ProjectPath       string                          `json:"project_path"`
	ID                string                          `json:"id"`
	Name              string                          `json:"name"`
	Algorithm         string                          `json:"algorithm"`
	BaseParameterSet  string                          `json:"base_parameter_set"`
	BaseInputs        map[string]any                  `json:"base_inputs"`
	Context           map[string]any                  `json:"context"`
	Objective         optimization.Objective          `json:"objective"`
	DecisionVariables []optimization.DecisionVariable `json:"decision_variables"`
	Constraints       []optimization.Constraint       `json:"constraints"`
}

type updateInputRequest struct {
	ProjectPath string         `json:"project_path"`
	Inputs      map[string]any `json:"inputs"`
	Context     map[string]any `json:"context"`
}

type validationRunRequest struct {
	ProjectPath      string `json:"project_path"`
	MappingPath      string `json:"mapping_path"`
	ParameterSetPath string `json:"parameter_set_path"`
	HighErrorRows    int    `json:"high_error_rows"`
	Save             bool   `json:"save"`
}

type calibrationRunRequest struct {
	ProjectPath      string `json:"project_path"`
	SetupPath        string `json:"setup_path"`
	SaveParameterSet string `json:"save_parameter_set"`
	Save             bool   `json:"save"`
}

type optimizationRunRequest struct {
	ProjectPath      string `json:"project_path"`
	SetupPath        string `json:"setup_path"`
	SaveScenario     string `json:"save_scenario"`
	SaveParameterSet string `json:"save_parameter_set"`
	Save             bool   `json:"save"`
}
