package studio

import (
	"bytes"
	"context"
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/goniegonie/hvac-studio/tools/go/internal/apperror"
	"github.com/goniegonie/hvac-studio/tools/go/internal/calibration"
	"github.com/goniegonie/hvac-studio/tools/go/internal/compiler"
	"github.com/goniegonie/hvac-studio/tools/go/internal/model"
	"github.com/goniegonie/hvac-studio/tools/go/internal/modelvalidation"
	"github.com/goniegonie/hvac-studio/tools/go/internal/optimization"
	"github.com/goniegonie/hvac-studio/tools/go/internal/parameterset"
	"github.com/goniegonie/hvac-studio/tools/go/internal/platform"
	"github.com/goniegonie/hvac-studio/tools/go/internal/project"
	runtimecore "github.com/goniegonie/hvac-studio/tools/go/internal/runtime"
	"github.com/goniegonie/hvac-studio/tools/go/internal/schemaexport"
)

//go:embed static
var staticFS embed.FS

func StaticAssets() (fs.FS, error) {
	return fs.Sub(staticFS, "static")
}

type Server struct {
	repoRoot string
	mux      *http.ServeMux
}

type ProjectSummary struct {
	Name         string `json:"name"`
	ProjectPath  string `json:"project_path"`
	RelativePath string `json:"relative_path"`
	Source       string `json:"source"`
}

type ProjectDetail struct {
	Project             *model.Project                  `json:"project"`
	Graph               *model.Graph                    `json:"graph"`
	ProjectPath         string                          `json:"project_path"`
	GraphPath           string                          `json:"graph_path"`
	DefaultInputPath    string                          `json:"default_input_path"`
	DefaultRunInput     *runtimecore.RunInput           `json:"default_run_input"`
	Layout              StudioLayout                    `json:"layout"`
	Root                string                          `json:"root"`
	Runs                []RunSummary                    `json:"runs"`
	Batches             []BatchSummary                  `json:"batches"`
	Exports             []ExportSummary                 `json:"exports"`
	Scenarios           []ScenarioSummary               `json:"scenarios"`
	Datasets            []DatasetSummary                `json:"datasets"`
	SeriesInputs        []SeriesInputSummary            `json:"series_inputs"`
	ParameterSets       []ParameterSetSummary           `json:"parameter_sets"`
	ValidationMappings  []ValidationMappingSummary      `json:"validation_mappings"`
	CalibrationSetups   []CalibrationSetupSummary       `json:"calibration_setups"`
	OptimizationSetups  []OptimizationSetupSummary      `json:"optimization_setups"`
	ValidationRuns      []modelvalidation.RecordSummary `json:"validation_runs"`
	CalibrationResults  []calibration.RecordSummary     `json:"calibration_results"`
	OptimizationResults []optimization.RecordSummary    `json:"optimization_results"`
	MLValidationReports map[string]MLValidationSummary  `json:"ml_validation_reports,omitempty"`
}

type SeriesInputSummary struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	RelativePath    string   `json:"relative_path"`
	StepCount       int      `json:"step_count"`
	TimeKey         string   `json:"time_key"`
	BaseContextKeys []string `json:"base_context_keys,omitempty"`
	StepContextKeys []string `json:"step_context_keys,omitempty"`
}

type MLValidationSummary struct {
	ComponentID          string                    `json:"component_id"`
	ReportPath           string                    `json:"report_path"`
	Dataset              string                    `json:"dataset,omitempty"`
	Metrics              map[string]map[string]any `json:"metrics,omitempty"`
	FeatureSchemaVersion string                    `json:"feature_schema_version,omitempty"`
	ModelAssetChecksum   string                    `json:"model_asset_checksum,omitempty"`
	TrainingPeriod       string                    `json:"training_period,omitempty"`
	ValidationPeriod     string                    `json:"validation_period,omitempty"`
	TimeResolution       string                    `json:"time_resolution,omitempty"`
}

type StudioLayout struct {
	Components map[string]CanvasPosition `json:"components"`
}

type CanvasPosition struct {
	X int `json:"x"`
	Y int `json:"y"`
}

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

type componentTemplateManifest struct {
	ID                   string                               `json:"id"`
	Name                 string                               `json:"name"`
	Kind                 string                               `json:"kind"`
	Category             string                               `json:"category"`
	ExecutionMode        string                               `json:"execution_mode"`
	ClassName            string                               `json:"class_name"`
	Source               componentTemplateSource              `json:"source"`
	Assets               []string                             `json:"assets"`
	Inputs               []model.Node                         `json:"inputs"`
	Outputs              []model.Node                         `json:"outputs"`
	Parameters           map[string]any                       `json:"parameters"`
	ParameterDefinitions map[string]model.ParameterDefinition `json:"parameter_defs"`
	StateDefinitions     map[string]model.StateDefinition     `json:"state_defs"`
	SolverBoundary       *model.SolverBoundary                `json:"solver_boundary,omitempty"`
	MLMetadata           *model.MLMetadata                    `json:"ml_metadata,omitempty"`
}

type componentTemplateSource struct {
	Layout   string `json:"layout,omitempty"`
	Metadata string `json:"metadata,omitempty"`
	Init     string `json:"init,omitempty"`
	Step     string `json:"step,omitempty"`
	Helpers  string `json:"helpers,omitempty"`
	Wrapper  string `json:"wrapper,omitempty"`
}

func (s *componentTemplateSource) UnmarshalJSON(data []byte) error {
	var singleFile string
	if err := json.Unmarshal(data, &singleFile); err == nil {
		*s = componentTemplateSource{
			Layout: "single_file_class",
			Step:   singleFile,
		}
		return nil
	}
	type source componentTemplateSource
	var decoded source
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*s = componentTemplateSource(decoded)
	if strings.TrimSpace(s.Layout) == "" {
		if strings.TrimSpace(s.Wrapper) != "" {
			s.Layout = "generated_wrapper"
		} else {
			s.Layout = "single_file_class"
		}
	}
	return nil
}

type componentTemplateFile struct {
	Role        string
	TemplateRel string
	Content     string
}

type ComponentTemplateSummary struct {
	ID                   string                               `json:"id"`
	Name                 string                               `json:"name"`
	Kind                 string                               `json:"kind"`
	Category             string                               `json:"category"`
	ExecutionMode        string                               `json:"execution_mode"`
	SourceLayout         string                               `json:"source_layout"`
	Inputs               []model.Node                         `json:"inputs,omitempty"`
	Outputs              []model.Node                         `json:"outputs,omitempty"`
	Parameters           map[string]any                       `json:"parameters,omitempty"`
	ParameterDefinitions map[string]model.ParameterDefinition `json:"parameter_defs,omitempty"`
	InputCount           int                                  `json:"input_count"`
	OutputCount          int                                  `json:"output_count"`
	ParameterCount       int                                  `json:"parameter_count"`
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

type updateComponentMLAssetsRequest struct {
	ProjectPath         string                       `json:"project_path"`
	ComponentID         string                       `json:"component_id"`
	ModelFormat         string                       `json:"model_format"`
	RequiredPackages    []string                     `json:"required_packages"`
	ValidTimeResolution string                       `json:"valid_time_resolution"`
	ValidInputRanges    map[string]model.ValueBounds `json:"valid_input_ranges,omitempty"`
	Assets              []componentMLAssetUpload     `json:"assets"`
}

type componentMLAssetUpload struct {
	Field         string `json:"field"`
	FileName      string `json:"file_name"`
	Content       string `json:"content,omitempty"`
	ContentBase64 string `json:"content_base64,omitempty"`
}

type applyComponentMLSchemaNodesRequest struct {
	ProjectPath string `json:"project_path"`
	ComponentID string `json:"component_id"`
}

type MLSchemaNodeApplySummary struct {
	FeatureCount    int      `json:"feature_count"`
	TargetCount     int      `json:"target_count"`
	AddedInputs     []string `json:"added_inputs"`
	AddedOutputs    []string `json:"added_outputs"`
	ExistingInputs  []string `json:"existing_inputs"`
	ExistingOutputs []string `json:"existing_outputs"`
}

type mlSchemaNode struct {
	ID        string
	Name      string
	Medium    string
	ValueType string
	Unit      string
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

type RunSummary struct {
	ID           string         `json:"id"`
	RelativePath string         `json:"relative_path"`
	CreatedAtUTC string         `json:"created_at_utc"`
	ParameterSet string         `json:"parameter_set,omitempty"`
	Outputs      map[string]any `json:"outputs"`
}

type RunRecord struct {
	ID           string                 `json:"id"`
	ProjectName  string                 `json:"project_name"`
	CreatedAtUTC string                 `json:"created_at_utc"`
	ParameterSet string                 `json:"parameter_set,omitempty"`
	Inputs       map[string]any         `json:"inputs"`
	Context      map[string]any         `json:"context"`
	Result       *runtimecore.RunResult `json:"result"`
}

type BatchSummary struct {
	ID           string `json:"id"`
	RelativePath string `json:"relative_path"`
	CreatedAtUTC string `json:"created_at_utc"`
	ParameterSet string `json:"parameter_set,omitempty"`
	CaseCount    int    `json:"case_count"`
	OKCount      int    `json:"ok_count"`
}

type BatchCaseRecord struct {
	ScenarioID   string                 `json:"scenario_id"`
	ScenarioName string                 `json:"scenario_name"`
	OK           bool                   `json:"ok"`
	Inputs       map[string]any         `json:"inputs"`
	Context      map[string]any         `json:"context"`
	Result       *runtimecore.RunResult `json:"result,omitempty"`
	Error        string                 `json:"error,omitempty"`
	Problems     []Problem              `json:"problems,omitempty"`
}

type BatchRecord struct {
	ID           string            `json:"id"`
	ProjectName  string            `json:"project_name"`
	CreatedAtUTC string            `json:"created_at_utc"`
	ParameterSet string            `json:"parameter_set,omitempty"`
	Cases        []BatchCaseRecord `json:"cases"`
}

type ScenarioSummary struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	RelativePath string `json:"relative_path"`
	CreatedAtUTC string `json:"created_at_utc"`
}

type DatasetSummary struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	RelativePath string `json:"relative_path"`
	Format       string `json:"format"`
	RowCount     int    `json:"row_count"`
	ColumnCount  int    `json:"column_count"`
	SHA256       string `json:"sha256,omitempty"`
}

type ParameterSetSummary struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	RelativePath   string `json:"relative_path"`
	CreatedAtUTC   string `json:"created_at_utc"`
	ComponentCount int    `json:"component_count"`
	ParameterCount int    `json:"parameter_count"`
}

type ValidationMappingSummary struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	RelativePath       string `json:"relative_path"`
	Dataset            string `json:"dataset"`
	DatasetChecksum    string `json:"dataset_checksum,omitempty"`
	InputCount         int    `json:"input_count"`
	OutputCount        int    `json:"output_count"`
	MissingValuePolicy string `json:"missing_value_policy,omitempty"`
}

type CalibrationSetupSummary struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	RelativePath   string `json:"relative_path"`
	Algorithm      string `json:"algorithm"`
	Mapping        string `json:"mapping"`
	ParameterCount int    `json:"parameter_count"`
}

type OptimizationSetupSummary struct {
	ID               string                 `json:"id"`
	Name             string                 `json:"name"`
	RelativePath     string                 `json:"relative_path"`
	Algorithm        string                 `json:"algorithm"`
	BaseParameterSet string                 `json:"base_parameter_set,omitempty"`
	Objective        optimization.Objective `json:"objective"`
	VariableCount    int                    `json:"variable_count"`
}

type DatasetPreview struct {
	Summary          DatasetSummary      `json:"summary"`
	Columns          []string            `json:"columns"`
	ColumnProfiles   []ColumnProfile     `json:"column_profiles,omitempty"`
	PreviewRows      []map[string]string `json:"preview_rows"`
	SuggestedInputs  []ColumnSuggestion  `json:"suggested_inputs"`
	SuggestedOutputs []ColumnSuggestion  `json:"suggested_outputs"`
}

type ColumnProfile struct {
	Column       string   `json:"column"`
	ValueType    string   `json:"value_type"`
	MissingCount int      `json:"missing_count"`
	Samples      []string `json:"samples,omitempty"`
}

type ColumnSuggestion struct {
	PublicID  string `json:"public_id"`
	Name      string `json:"name"`
	Column    string `json:"column,omitempty"`
	Unit      string `json:"unit,omitempty"`
	ValueType string `json:"value_type,omitempty"`
	Required  bool   `json:"required"`
}

type ParameterSetDetail struct {
	Summary     ParameterSetSummary `json:"summary"`
	Set         parameterset.Set    `json:"set"`
	Differences []ParameterDiff     `json:"differences"`
}

type ParameterDiff struct {
	Component string `json:"component"`
	Parameter string `json:"parameter"`
	Baseline  any    `json:"baseline,omitempty"`
	Value     any    `json:"value"`
	Exists    bool   `json:"exists"`
}

type ScenarioRecord struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	ProjectName  string         `json:"project_name"`
	CreatedAtUTC string         `json:"created_at_utc"`
	Inputs       map[string]any `json:"inputs"`
	Context      map[string]any `json:"context"`
}

type ExportSummary struct {
	Profile      string `json:"profile"`
	RelativePath string `json:"relative_path"`
	CreatedAtUTC string `json:"created_at_utc"`
}

type ExportManifest struct {
	Profile             string                `json:"profile"`
	CreatedAtUTC        string                `json:"created_at_utc"`
	ProjectName         string                `json:"project_name"`
	ProjectRoot         string                `json:"project_root"`
	ProjectPath         string                `json:"project_path"`
	GraphPath           string                `json:"graph_path"`
	DefaultInput        string                `json:"default_input"`
	EnvironmentLockfile string                `json:"environment_lockfile"`
	InterfaceSchema     string                `json:"interface_schema"`
	Runner              string                `json:"runner"`
	RuntimePython       string                `json:"runtime_python"`
	Files               []string              `json:"files"`
	Components          []string              `json:"components"`
	ModelAssets         []string              `json:"model_assets,omitempty"`
	MLValidationReports []MLValidationSummary `json:"ml_validation_reports,omitempty"`
	Checksums           map[string]string     `json:"checksums,omitempty"`
	PublicInputs        []model.PublicNodeRef `json:"public_inputs"`
	PublicOutputs       []model.PublicNodeRef `json:"public_outputs"`
	ExecutionOrder      []string              `json:"execution_order"`
	ParameterSets       []string              `json:"parameter_sets,omitempty"`
	Datasets            []string              `json:"datasets,omitempty"`
	ValidationMappings  []string              `json:"validation_mappings,omitempty"`
	CalibrationSetups   []string              `json:"calibration_setups,omitempty"`
	OptimizationSetups  []string              `json:"optimization_setups,omitempty"`
	RunRecords          []string              `json:"run_records,omitempty"`
	BatchRecords        []string              `json:"batch_records,omitempty"`
	ValidationRecords   []string              `json:"validation_records,omitempty"`
	CalibrationRecords  []string              `json:"calibration_records,omitempty"`
	OptimizationRecords []string              `json:"optimization_records,omitempty"`
	Commands            []string              `json:"commands,omitempty"`
	IncludeDatasets     bool                  `json:"include_datasets"`
	IncludeCalibration  bool                  `json:"include_calibration_setups"`
	IncludeOptimization bool                  `json:"include_optimization_setups"`
	IncludeMLAssets     bool                  `json:"include_ml_assets"`
	IncludeSDKExamples  bool                  `json:"include_sdk_examples"`
	IncludeRecords      bool                  `json:"include_records"`
}

type SourceDetail struct {
	ComponentID  string `json:"component_id"`
	RelativePath string `json:"relative_path"`
	Content      string `json:"content"`
	ReadOnly     bool   `json:"read_only"`
}

type SourceCheck struct {
	OK               bool      `json:"ok"`
	ComponentID      string    `json:"component_id"`
	RelativePath     string    `json:"relative_path"`
	ExpectedClass    string    `json:"expected_class"`
	ExpectedFunction string    `json:"expected_function,omitempty"`
	LineCount        int       `json:"line_count"`
	Problems         []Problem `json:"problems"`
}

type Problem struct {
	Severity    string `json:"severity"`
	Message     string `json:"message"`
	ComponentID string `json:"component_id,omitempty"`
	NodeID      string `json:"node_id,omitempty"`
	Source      string `json:"source,omitempty"`
	Line        int    `json:"line,omitempty"`
	Column      int    `json:"column,omitempty"`
}

type apiError struct {
	OK       bool             `json:"ok"`
	Error    apperror.Payload `json:"error"`
	Code     int              `json:"code"`
	Kind     string           `json:"kind"`
	Message  string           `json:"message"`
	Problems []Problem        `json:"problems,omitempty"`
}

func New(repoRoot string) (*Server, error) {
	absRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, err
	}
	assets, err := StaticAssets()
	if err != nil {
		return nil, err
	}

	server := &Server{
		repoRoot: absRoot,
		mux:      http.NewServeMux(),
	}
	server.routes(http.FileServer(http.FS(assets)))
	return server, nil
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) routes(staticHandler http.Handler) {
	s.mux.HandleFunc("GET /api/projects", s.handleProjects)
	s.mux.HandleFunc("POST /api/projects", s.handleCreateProject)
	s.mux.HandleFunc("POST /api/projects/copy", s.handleCopyProject)
	s.mux.HandleFunc("GET /api/project", s.handleProject)
	s.mux.HandleFunc("GET /api/project/run", s.handleRunRecord)
	s.mux.HandleFunc("GET /api/project/batch", s.handleBatchRecord)
	s.mux.HandleFunc("GET /api/project/scenario", s.handleScenarioRecord)
	s.mux.HandleFunc("GET /api/project/export", s.handleExportRecord)
	s.mux.HandleFunc("GET /api/project/dataset", s.handleDatasetPreview)
	s.mux.HandleFunc("GET /api/project/parameter-set", s.handleParameterSetDetail)
	s.mux.HandleFunc("GET /api/project/validation-record", s.handleValidationRecord)
	s.mux.HandleFunc("GET /api/project/calibration-record", s.handleCalibrationRecord)
	s.mux.HandleFunc("GET /api/project/optimization-record", s.handleOptimizationRecord)
	s.mux.HandleFunc("GET /api/project/source", s.handleSource)
	s.mux.HandleFunc("GET /api/component-templates", s.handleComponentTemplates)
	s.mux.HandleFunc("POST /api/project/source/check", s.handleCheckSource)
	s.mux.HandleFunc("POST /api/project/datasets/import", s.handleImportDataset)
	s.mux.HandleFunc("POST /api/project/components", s.handleCreateComponent)
	s.mux.HandleFunc("POST /api/project/components/duplicate", s.handleDuplicateComponent)
	s.mux.HandleFunc("POST /api/project/components/replace", s.handleReplaceComponent)
	s.mux.HandleFunc("POST /api/project/components/update", s.handleUpdateComponent)
	s.mux.HandleFunc("POST /api/project/components/ml-assets", s.handleUpdateComponentMLAssets)
	s.mux.HandleFunc("POST /api/project/components/ml-schema-nodes", s.handleApplyComponentMLSchemaNodes)
	s.mux.HandleFunc("POST /api/project/components/delete", s.handleDeleteComponent)
	s.mux.HandleFunc("POST /api/project/system/components", s.handleIncludeComponent)
	s.mux.HandleFunc("POST /api/project/system/components/remove", s.handleRemoveComponentFromSystem)
	s.mux.HandleFunc("POST /api/project/nodes", s.handleCreateNode)
	s.mux.HandleFunc("POST /api/project/nodes/update", s.handleUpdateNode)
	s.mux.HandleFunc("POST /api/project/nodes/delete", s.handleDeleteNode)
	s.mux.HandleFunc("POST /api/project/connections", s.handleCreateConnection)
	s.mux.HandleFunc("POST /api/project/connections/update", s.handleUpdateConnection)
	s.mux.HandleFunc("POST /api/project/connections/delete", s.handleDeleteConnection)
	s.mux.HandleFunc("POST /api/project/layout", s.handleUpdateLayout)
	s.mux.HandleFunc("POST /api/project/input", s.handleUpdateInput)
	s.mux.HandleFunc("POST /api/project/parameters", s.handleUpdateParameters)
	s.mux.HandleFunc("POST /api/project/component-contract", s.handleUpdateComponentContract)
	s.mux.HandleFunc("POST /api/project/parameter-set/apply", s.handleApplyParameterSet)
	s.mux.HandleFunc("POST /api/project/parameters/delete", s.handleDeleteParameter)
	s.mux.HandleFunc("POST /api/project/source", s.handleUpdateSource)
	s.mux.HandleFunc("GET /api/project/validation-mapping", s.handleValidationMappingDetail)
	s.mux.HandleFunc("POST /api/project/validation-mapping", s.handleCreateValidationMapping)
	s.mux.HandleFunc("POST /api/project/validation-mapping/update", s.handleUpdateValidationMapping)
	s.mux.HandleFunc("POST /api/project/validation-mapping/copy", s.handleCopyValidationMapping)
	s.mux.HandleFunc("POST /api/project/validation-mapping/delete", s.handleDeleteValidationMapping)
	s.mux.HandleFunc("POST /api/project/calibration-setup", s.handleCreateCalibrationSetup)
	s.mux.HandleFunc("POST /api/project/optimization-setup", s.handleCreateOptimizationSetup)
	s.mux.HandleFunc("POST /api/project/scenarios", s.handleCreateScenario)
	s.mux.HandleFunc("POST /api/validate", s.handleValidate)
	s.mux.HandleFunc("POST /api/run", s.handleRun)
	s.mux.HandleFunc("POST /api/run-series", s.handleRunSeries)
	s.mux.HandleFunc("POST /api/batch", s.handleBatch)
	s.mux.HandleFunc("POST /api/validation/run", s.handleDataValidation)
	s.mux.HandleFunc("POST /api/calibration/run", s.handleCalibrationRun)
	s.mux.HandleFunc("POST /api/optimization/run", s.handleOptimizationRun)
	s.mux.HandleFunc("POST /api/schema", s.handleSchema)
	s.mux.HandleFunc("POST /api/export", s.handleExport)
	s.mux.Handle("/docs/", http.StripPrefix("/docs/", http.FileServer(http.Dir(filepath.Join(s.repoRoot, "docs")))))
	s.mux.Handle("/", staticHandler)
}

func (s *Server) handleProjects(w http.ResponseWriter, r *http.Request) {
	projects := []ProjectSummary{}
	projects = append(projects, s.findProjectSummaries(filepath.Join(s.repoRoot, "projects"), "workspace")...)
	projects = append(projects, s.findProjectSummaries(filepath.Join(s.repoRoot, "examples"), "example")...)
	sort.Slice(projects, func(i, j int) bool {
		if projects[i].Source != projects[j].Source {
			return projects[i].Source == "workspace"
		}
		return projects[i].RelativePath < projects[j].RelativePath
	})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "projects": projects})
}

func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	req, err := decodeCreateProjectRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	summary, err := s.createProject(req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "project": summary})
}

func (s *Server) handleCopyProject(w http.ResponseWriter, r *http.Request) {
	req, err := decodeCopyProjectRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	summary, err := s.copyProject(req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "project": summary})
}

func (s *Server) handleRunRecord(w http.ResponseWriter, r *http.Request) {
	projectPath, err := s.resolveProjectPath(r.URL.Query().Get("project_path"))
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := project.Load(projectPath)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	record, err := loadRunRecord(loaded.Root, r.URL.Query().Get("run_id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "run_record": record})
}

func (s *Server) handleBatchRecord(w http.ResponseWriter, r *http.Request) {
	projectPath, err := s.resolveProjectPath(r.URL.Query().Get("project_path"))
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := project.Load(projectPath)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	record, err := loadBatchRecord(loaded.Root, r.URL.Query().Get("batch_id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "batch_record": record})
}

func (s *Server) handleScenarioRecord(w http.ResponseWriter, r *http.Request) {
	projectPath, err := s.resolveProjectPath(r.URL.Query().Get("project_path"))
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := project.Load(projectPath)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	record, err := loadScenarioRecord(loaded.Root, r.URL.Query().Get("scenario_id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "scenario": record})
}

func (s *Server) handleExportRecord(w http.ResponseWriter, r *http.Request) {
	projectPath, err := s.resolveProjectPath(r.URL.Query().Get("project_path"))
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := project.Load(projectPath)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	summary, manifest, err := loadExportManifest(loaded.Root, r.URL.Query().Get("profile"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "summary": summary, "export": manifest})
}

func (s *Server) handleDatasetPreview(w http.ResponseWriter, r *http.Request) {
	loaded, err := s.loadProject(r.URL.Query().Get("project_path"))
	if err != nil {
		writeError(w, err)
		return
	}
	preview, err := datasetPreview(loaded, r.URL.Query().Get("path"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "dataset": preview})
}

func (s *Server) handleImportDataset(w http.ResponseWriter, r *http.Request) {
	req, err := decodeImportDatasetRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	preview, err := importDataset(loaded, req)
	if err != nil {
		writeError(w, err)
		return
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "dataset": preview, "summary": preview.Summary, "project": projectDetail(reloaded)})
}

func (s *Server) handleParameterSetDetail(w http.ResponseWriter, r *http.Request) {
	loaded, err := s.loadProject(r.URL.Query().Get("project_path"))
	if err != nil {
		writeError(w, err)
		return
	}
	detail, err := parameterSetDetail(loaded, r.URL.Query().Get("path"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "parameter_set": detail})
}

func (s *Server) handleValidationMappingDetail(w http.ResponseWriter, r *http.Request) {
	loaded, err := s.loadProject(r.URL.Query().Get("project_path"))
	if err != nil {
		writeError(w, err)
		return
	}
	mapping, err := modelvalidation.LoadMapping(loaded.Root, r.URL.Query().Get("path"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "mapping": mapping})
}

func (s *Server) handleCreateValidationMapping(w http.ResponseWriter, r *http.Request) {
	req, err := decodeCreateValidationMappingRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	summary, mapping, err := createValidationMapping(loaded, req)
	if err != nil {
		writeError(w, err)
		return
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "mapping": mapping, "summary": summary, "project": projectDetail(reloaded)})
}

func (s *Server) handleUpdateValidationMapping(w http.ResponseWriter, r *http.Request) {
	req, err := decodeUpdateValidationMappingRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	summary, mapping, err := updateValidationMapping(loaded, req)
	if err != nil {
		writeError(w, err)
		return
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "mapping": mapping, "summary": summary, "project": projectDetail(reloaded)})
}

func (s *Server) handleCopyValidationMapping(w http.ResponseWriter, r *http.Request) {
	req, err := decodeCopyValidationMappingRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	summary, mapping, err := copyValidationMapping(loaded, req)
	if err != nil {
		writeError(w, err)
		return
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "mapping": mapping, "summary": summary, "project": projectDetail(reloaded)})
}

func (s *Server) handleDeleteValidationMapping(w http.ResponseWriter, r *http.Request) {
	req, err := decodeDeleteValidationMappingRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	deletedPath, err := deleteValidationMapping(loaded, req)
	if err != nil {
		writeError(w, err)
		return
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "mapping_path": deletedPath, "project": projectDetail(reloaded)})
}

func (s *Server) handleCreateCalibrationSetup(w http.ResponseWriter, r *http.Request) {
	var req createCalibrationSetupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apperror.Wrap(apperror.CodeInput, err))
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	summary, setup, err := createCalibrationSetup(loaded, req)
	if err != nil {
		writeError(w, err)
		return
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "setup": setup, "summary": summary, "project": projectDetail(reloaded)})
}

func (s *Server) handleCreateOptimizationSetup(w http.ResponseWriter, r *http.Request) {
	var req createOptimizationSetupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apperror.Wrap(apperror.CodeInput, err))
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	summary, setup, err := createOptimizationSetup(loaded, req)
	if err != nil {
		writeError(w, err)
		return
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "setup": setup, "summary": summary, "project": projectDetail(reloaded)})
}

func (s *Server) handleValidationRecord(w http.ResponseWriter, r *http.Request) {
	projectPath, err := s.resolveProjectPath(r.URL.Query().Get("project_path"))
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := project.Load(projectPath)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	record, err := modelvalidation.LoadRecord(loaded.Root, r.URL.Query().Get("record_id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "validation_record": record})
}

func (s *Server) handleCalibrationRecord(w http.ResponseWriter, r *http.Request) {
	projectPath, err := s.resolveProjectPath(r.URL.Query().Get("project_path"))
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := project.Load(projectPath)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	record, err := calibration.LoadRecord(loaded.Root, r.URL.Query().Get("record_id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "calibration_record": record})
}

func (s *Server) handleOptimizationRecord(w http.ResponseWriter, r *http.Request) {
	projectPath, err := s.resolveProjectPath(r.URL.Query().Get("project_path"))
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := project.Load(projectPath)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	record, err := optimization.LoadRecord(loaded.Root, r.URL.Query().Get("record_id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "optimization_record": record})
}

func (s *Server) handleSource(w http.ResponseWriter, r *http.Request) {
	projectPath, err := s.resolveProjectPath(r.URL.Query().Get("project_path"))
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := project.Load(projectPath)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	source, err := loadComponentSource(loaded, r.URL.Query().Get("component_id"), s.ensureWorkspaceProject(loaded.Root) != nil)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "source": source})
}

func (s *Server) handleComponentTemplates(w http.ResponseWriter, r *http.Request) {
	templates, err := listComponentTemplates(s.repoRoot)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "templates": templates})
}

func (s *Server) handleCheckSource(w http.ResponseWriter, r *http.Request) {
	req, err := decodeSourceCheckRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	check, err := checkComponentSource(r.Context(), loaded, req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "check": check})
}

func (s *Server) handleCreateComponent(w http.ResponseWriter, r *http.Request) {
	req, err := decodeCreateComponentRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	component, err := createComponent(loaded, req, s.repoRoot)
	if err != nil {
		writeError(w, err)
		return
	}
	if req.IncludeInSystem {
		if err := includeComponentInSystem(loaded, includeComponentRequest{
			ProjectPath: req.ProjectPath,
			SystemID:    loaded.Project.EntrySystem,
			ComponentID: component.ID,
		}); err != nil {
			writeError(w, err)
			return
		}
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "component": component, "project": projectDetail(reloaded)})
}

func (s *Server) handleDuplicateComponent(w http.ResponseWriter, r *http.Request) {
	req, err := decodeDuplicateComponentRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	component, err := duplicateComponent(loaded, req)
	if err != nil {
		writeError(w, err)
		return
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "component": component, "project": projectDetail(reloaded)})
}

func (s *Server) handleReplaceComponent(w http.ResponseWriter, r *http.Request) {
	req, err := decodeReplaceComponentRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	component, summary, problems, err := replaceComponent(loaded, req, s.repoRoot)
	if err != nil {
		writeErrorWithProblems(w, err, problems)
		return
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "component": component, "replacement": summary, "project": projectDetail(reloaded)})
}

func (s *Server) handleUpdateComponent(w http.ResponseWriter, r *http.Request) {
	req, err := decodeUpdateComponentRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	component, err := updateComponent(loaded, req)
	if err != nil {
		writeError(w, err)
		return
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "component": component, "project": projectDetail(reloaded)})
}

func (s *Server) handleUpdateComponentMLAssets(w http.ResponseWriter, r *http.Request) {
	req, err := decodeUpdateComponentMLAssetsRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	component, importedFiles, err := updateComponentMLAssets(loaded, req)
	if err != nil {
		writeError(w, err)
		return
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "component": component, "imported_files": importedFiles, "project": projectDetail(reloaded)})
}

func (s *Server) handleApplyComponentMLSchemaNodes(w http.ResponseWriter, r *http.Request) {
	req, err := decodeApplyComponentMLSchemaNodesRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	component, summary, err := applyComponentMLSchemaNodes(loaded, req)
	if err != nil {
		writeError(w, err)
		return
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "component": component, "summary": summary, "project": projectDetail(reloaded)})
}

func (s *Server) handleDeleteComponent(w http.ResponseWriter, r *http.Request) {
	req, err := decodeDeleteComponentRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	if err := deleteComponent(loaded, req); err != nil {
		writeError(w, err)
		return
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "project": projectDetail(reloaded)})
}

func (s *Server) handleIncludeComponent(w http.ResponseWriter, r *http.Request) {
	req, err := decodeIncludeComponentRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	if err := includeComponentInSystem(loaded, req); err != nil {
		writeError(w, err)
		return
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "project": projectDetail(reloaded)})
}

func (s *Server) handleRemoveComponentFromSystem(w http.ResponseWriter, r *http.Request) {
	req, err := decodeIncludeComponentRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	if err := removeComponentFromSystem(loaded, req); err != nil {
		writeError(w, err)
		return
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "project": projectDetail(reloaded)})
}

func (s *Server) handleCreateNode(w http.ResponseWriter, r *http.Request) {
	req, err := decodeCreateNodeRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	node, err := createNode(loaded, req)
	if err != nil {
		writeError(w, err)
		return
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "node": node, "project": projectDetail(reloaded)})
}

func (s *Server) handleUpdateNode(w http.ResponseWriter, r *http.Request) {
	req, err := decodeUpdateNodeRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	node, err := updateNode(loaded, req)
	if err != nil {
		writeError(w, err)
		return
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "node": node, "project": projectDetail(reloaded)})
}

func (s *Server) handleDeleteNode(w http.ResponseWriter, r *http.Request) {
	req, err := decodeDeleteNodeRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	node, err := deleteNode(loaded, req)
	if err != nil {
		writeError(w, err)
		return
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "node": node, "project": projectDetail(reloaded)})
}

func (s *Server) handleCreateConnection(w http.ResponseWriter, r *http.Request) {
	req, err := decodeCreateConnectionRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	connection, err := createConnection(loaded, req)
	if err != nil {
		writeError(w, err)
		return
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "connection": connection, "project": projectDetail(reloaded)})
}

func (s *Server) handleUpdateConnection(w http.ResponseWriter, r *http.Request) {
	req, err := decodeUpdateConnectionRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	connection, err := updateConnection(loaded, req)
	if err != nil {
		writeError(w, err)
		return
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "connection": connection, "project": projectDetail(reloaded)})
}

func (s *Server) handleDeleteConnection(w http.ResponseWriter, r *http.Request) {
	req, err := decodeDeleteConnectionRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	connection, err := deleteConnection(loaded, req)
	if err != nil {
		writeError(w, err)
		return
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "connection": connection, "project": projectDetail(reloaded)})
}

func (s *Server) handleUpdateLayout(w http.ResponseWriter, r *http.Request) {
	req, err := decodeUpdateLayoutRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	if err := writeStudioLayout(loaded, req.Components); err != nil {
		writeError(w, err)
		return
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "project": projectDetail(reloaded)})
}

func (s *Server) handleProject(w http.ResponseWriter, r *http.Request) {
	projectPath, err := s.resolveProjectPath(r.URL.Query().Get("project_path"))
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := project.Load(projectPath)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"project": projectDetail(loaded),
	})
}

func (s *Server) handleUpdateParameters(w http.ResponseWriter, r *http.Request) {
	req, err := decodeUpdateParametersRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	if len(req.Parameters) == 0 {
		writeError(w, apperror.Errorf(apperror.CodeValidation, "parameters are required"))
		return
	}

	for componentID, parameters := range req.Parameters {
		if strings.TrimSpace(componentID) == "" {
			writeError(w, apperror.Errorf(apperror.CodeValidation, "component id is required"))
			return
		}
		found := false
		for componentIndex := range loaded.Graph.Components {
			component := &loaded.Graph.Components[componentIndex]
			if component.ID != componentID {
				continue
			}
			found = true
			if component.Parameters == nil {
				component.Parameters = map[string]any{}
			}
			for name, value := range parameters {
				if strings.TrimSpace(name) == "" {
					writeError(w, apperror.Errorf(apperror.CodeValidation, "parameter name is required"))
					return
				}
				component.Parameters[name] = value
				if component.ParameterDefinitions != nil {
					if definition, exists := component.ParameterDefinitions[name]; exists {
						definition.Current = value
						component.ParameterDefinitions[name] = definition
					}
				}
			}
			if err := syncComponentMetadataFile(loaded, *component); err != nil {
				writeError(w, err)
				return
			}
			break
		}
		if !found {
			writeError(w, apperror.Errorf(apperror.CodeValidation, "component not found: %s", componentID))
			return
		}
	}

	if err := writeJSONFile(loaded.GraphPath, loaded.Graph); err != nil {
		writeError(w, apperror.Wrap(apperror.CodeRuntime, err))
		return
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "project": projectDetail(reloaded)})
}

func (s *Server) handleUpdateComponentContract(w http.ResponseWriter, r *http.Request) {
	req, err := decodeUpdateComponentContractRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	component, err := updateComponentContract(loaded, req)
	if err != nil {
		writeError(w, err)
		return
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "project": projectDetail(reloaded), "component": component})
}

func (s *Server) handleApplyParameterSet(w http.ResponseWriter, r *http.Request) {
	req, err := decodeApplyParameterSetRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	set, err := parameterset.ApplyFile(loaded, req.Path)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := writeJSONFile(loaded.GraphPath, loaded.Graph); err != nil {
		writeError(w, apperror.Wrap(apperror.CodeRuntime, err))
		return
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "project": projectDetail(reloaded), "parameter_set": set})
}

func (s *Server) handleDeleteParameter(w http.ResponseWriter, r *http.Request) {
	req, err := decodeDeleteParameterRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	if err := deleteParameter(loaded, req); err != nil {
		writeError(w, err)
		return
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "project": projectDetail(reloaded)})
}

func (s *Server) handleUpdateInput(w http.ResponseWriter, r *http.Request) {
	req, err := decodeUpdateInputRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	if req.Inputs == nil {
		writeError(w, apperror.Errorf(apperror.CodeValidation, "inputs are required"))
		return
	}
	inputPath, err := resolveProjectOwnedFile(loaded.Root, loaded.Project.DefaultInput)
	if err != nil {
		writeError(w, err)
		return
	}
	input := runtimecore.RunInput{Inputs: req.Inputs, Context: req.Context}
	if input.Context == nil {
		input.Context = map[string]any{}
	}
	if err := writeJSONFile(inputPath, input); err != nil {
		writeError(w, apperror.Wrap(apperror.CodeRuntime, err))
		return
	}
	reloaded, err := project.Load(loaded.Path)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "project": projectDetail(reloaded)})
}

func (s *Server) handleUpdateSource(w http.ResponseWriter, r *http.Request) {
	req, err := decodeSourceRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	sourcePath, err := componentSourcePath(loaded, req.ComponentID)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := os.WriteFile(sourcePath, []byte(req.Content), 0o644); err != nil {
		writeError(w, apperror.Wrap(apperror.CodeRuntime, err))
		return
	}
	source, err := loadComponentSource(loaded, req.ComponentID, false)
	if err != nil {
		writeError(w, err)
		return
	}
	check, err := checkComponentSource(r.Context(), loaded, sourceCheckRequest{
		ProjectPath: req.ProjectPath,
		ComponentID: req.ComponentID,
		Content:     req.Content,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "source": source, "check": check})
}

func (s *Server) handleCreateScenario(w http.ResponseWriter, r *http.Request) {
	req, err := decodeCreateScenarioRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	summary, record, err := writeScenarioRecord(loaded, req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "summary": summary, "scenario": record})
}

func (s *Server) handleValidate(w http.ResponseWriter, r *http.Request) {
	req, err := decodeRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	plan, err := compiler.Compile(loaded)
	if err != nil {
		writeErrorWithProblems(w, apperror.Wrap(apperror.CodeValidation, err), inferProblems(loaded, err))
		return
	}
	problems := compilerDiagnosticsProblems(plan.Diagnostics)
	sourceCheckCount, sourceProblems := checkProjectSources(r.Context(), loaded)
	if hasErrorProblems(sourceProblems) {
		writeErrorWithProblems(w, apperror.Errorf(apperror.CodeValidation, "project source validation failed"), sourceProblems)
		return
	}
	problems = append(problems, sourceProblems...)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true,
		"validation": map[string]any{
			"project_name":     loaded.Project.ProjectName,
			"entry_system":     loaded.Project.EntrySystem,
			"component_count":  len(plan.System.Components),
			"connection_count": len(plan.System.Connections),
			"execution_order":  plan.Order,
			"source_checks":    sourceCheckCount,
			"problems":         problems,
		},
	})
}

func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	req, err := decodeRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}

	input := runtimecore.RunInput{Inputs: req.Inputs, Context: req.Context}
	if input.Inputs == nil {
		input, err = runtimecore.LoadInput(resolveProjectFile(loaded.Root, loaded.Project.DefaultInput))
		if err != nil {
			writeError(w, apperror.Wrap(apperror.CodeInput, err))
			return
		}
	}
	if input.Context == nil {
		input.Context = map[string]any{}
	}
	if problems := projectSourceErrorProblems(r.Context(), loaded); len(problems) > 0 {
		writeErrorWithProblems(w, apperror.Errorf(apperror.CodeValidation, "project source validation failed"), problems)
		return
	}
	if req.ParameterSetPath != "" {
		if _, err := parameterset.ApplyFile(loaded, req.ParameterSetPath); err != nil {
			writeError(w, err)
			return
		}
	}

	timeout, err := requestTimeout(req, 30*time.Second)
	if err != nil {
		writeError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()
	result, err := runtimecore.Run(ctx, loaded, input)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			writeTimeoutError(w, "run", timeout)
			return
		}
		writeErrorWithProblems(w, err, inferProblems(loaded, err))
		return
	}
	if req.ParameterSetPath != "" {
		result.ParameterSet = filepath.ToSlash(req.ParameterSetPath)
	}
	response := map[string]any{"ok": true, "result": result}
	if req.Save {
		runRecord, err := writeRunRecord(loaded, input, result, filepath.ToSlash(req.ParameterSetPath))
		if err != nil {
			writeError(w, apperror.Wrap(apperror.CodeRuntime, err))
			return
		}
		response["run_record"] = runRecord
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleRunSeries(w http.ResponseWriter, r *http.Request) {
	req, err := decodeSeriesRunRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if problems := projectSourceErrorProblems(r.Context(), loaded); len(problems) > 0 {
		writeErrorWithProblems(w, apperror.Errorf(apperror.CodeValidation, "project source validation failed"), problems)
		return
	}
	if req.ParameterSetPath != "" {
		if _, err := parameterset.ApplyFile(loaded, req.ParameterSetPath); err != nil {
			writeError(w, err)
			return
		}
	}

	input := runtimecore.SeriesInput{
		SchemaVersion: req.SchemaVersion,
		Context:       req.Context,
		Steps:         req.Steps,
	}
	if req.InputPath != "" {
		input, err = runtimecore.LoadSeriesInput(resolveProjectFile(loaded.Root, req.InputPath))
		if err != nil {
			writeError(w, err)
			return
		}
	}
	if len(input.Steps) == 0 {
		writeError(w, apperror.Errorf(apperror.CodeInput, "series input requires steps"))
		return
	}

	timeout, err := requestTimeout(apiRequest{TimeoutMS: req.TimeoutMS}, time.Duration(len(input.Steps))*30*time.Second)
	if err != nil {
		writeError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()
	result, err := runtimecore.RunSeries(ctx, loaded, input)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			writeTimeoutError(w, "series", timeout)
			return
		}
		writeErrorWithProblems(w, err, inferProblems(loaded, err))
		return
	}
	if req.ParameterSetPath != "" {
		result.ParameterSet = filepath.ToSlash(req.ParameterSetPath)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "result": result})
}

func (s *Server) handleBatch(w http.ResponseWriter, r *http.Request) {
	req, err := decodeRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	scenarios := loadScenarioSummaries(loaded.Root)
	if len(scenarios) == 0 {
		writeError(w, apperror.Errorf(apperror.CodeValidation, "batch requires at least one saved scenario"))
		return
	}
	if problems := projectSourceErrorProblems(r.Context(), loaded); len(problems) > 0 {
		writeErrorWithProblems(w, apperror.Errorf(apperror.CodeValidation, "project source validation failed"), problems)
		return
	}
	if req.ParameterSetPath != "" {
		if _, err := parameterset.ApplyFile(loaded, req.ParameterSetPath); err != nil {
			writeError(w, err)
			return
		}
	}

	timeout, err := requestTimeout(req, time.Duration(len(scenarios))*30*time.Second)
	if err != nil {
		writeError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()
	cases := make([]BatchCaseRecord, 0, len(scenarios))
	for index := len(scenarios) - 1; index >= 0; index-- {
		scenario, err := loadScenarioRecord(loaded.Root, scenarios[index].ID)
		if err != nil {
			writeError(w, err)
			return
		}
		input := runtimecore.RunInput{Inputs: scenario.Inputs, Context: scenario.Context}
		if input.Context == nil {
			input.Context = map[string]any{}
		}
		caseRecord := BatchCaseRecord{
			ScenarioID:   scenario.ID,
			ScenarioName: scenario.Name,
			Inputs:       input.Inputs,
			Context:      input.Context,
		}
		result, err := runtimecore.Run(ctx, loaded, input)
		if err != nil {
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				writeTimeoutError(w, "batch", timeout)
				return
			}
			caseRecord.Error = err.Error()
			caseRecord.Problems = inferProblems(loaded, err)
		} else {
			if req.ParameterSetPath != "" {
				result.ParameterSet = filepath.ToSlash(req.ParameterSetPath)
			}
			caseRecord.OK = true
			caseRecord.Result = result
		}
		cases = append(cases, caseRecord)
	}
	summary, record, err := writeBatchRecord(loaded, cases, filepath.ToSlash(req.ParameterSetPath))
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeRuntime, err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "summary": summary, "batch": record})
}

func (s *Server) handleDataValidation(w http.ResponseWriter, r *http.Request) {
	var req validationRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apperror.Wrap(apperror.CodeInput, err))
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if req.Save {
		if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
			writeError(w, err)
			return
		}
	}
	if problems := projectSourceErrorProblems(r.Context(), loaded); len(problems) > 0 {
		writeErrorWithProblems(w, apperror.Errorf(apperror.CodeValidation, "project source validation failed"), problems)
		return
	}
	if req.ParameterSetPath != "" {
		if _, err := parameterset.ApplyFile(loaded, req.ParameterSetPath); err != nil {
			writeError(w, err)
			return
		}
	}
	if req.MappingPath == "" {
		mappings := loadValidationMappingSummaries(loaded.Root)
		if len(mappings) == 0 {
			writeError(w, apperror.Errorf(apperror.CodeValidation, "data validation requires a saved mapping"))
			return
		}
		req.MappingPath = mappings[0].RelativePath
	}
	mapping, err := modelvalidation.LoadMapping(loaded.Root, req.MappingPath)
	if err != nil {
		writeError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	result, err := modelvalidation.Run(ctx, loaded, mapping, modelvalidation.Options{HighErrorRows: req.HighErrorRows})
	if err != nil {
		writeErrorWithProblems(w, err, inferProblems(loaded, err))
		return
	}
	if req.ParameterSetPath != "" {
		result.ParameterSet = filepath.ToSlash(req.ParameterSetPath)
	}
	response := map[string]any{"ok": true, "validation_result": result}
	if req.Save {
		summary, err := modelvalidation.WriteRecord(loaded, result)
		if err != nil {
			writeError(w, err)
			return
		}
		response["validation_record"] = summary
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleCalibrationRun(w http.ResponseWriter, r *http.Request) {
	var req calibrationRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apperror.Wrap(apperror.CodeInput, err))
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if req.Save {
		if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
			writeError(w, err)
			return
		}
	}
	if problems := projectSourceErrorProblems(r.Context(), loaded); len(problems) > 0 {
		writeErrorWithProblems(w, apperror.Errorf(apperror.CodeValidation, "project source validation failed"), problems)
		return
	}
	if req.SetupPath == "" {
		setups := loadCalibrationSetupSummaries(loaded.Root)
		if len(setups) == 0 {
			writeError(w, apperror.Errorf(apperror.CodeValidation, "calibration requires a saved setup"))
			return
		}
		req.SetupPath = setups[0].RelativePath
	}
	setup, err := calibration.LoadSetup(loaded.Root, req.SetupPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if req.Save && req.SaveParameterSet == "" {
		req.SaveParameterSet = filepath.ToSlash(filepath.Join("parameter_sets", setup.ID+"_calibrated.json"))
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	result, err := calibration.Run(ctx, loaded.Path, setup, calibration.Options{SaveParameterSet: req.SaveParameterSet})
	if err != nil {
		writeErrorWithProblems(w, err, inferProblems(loaded, err))
		return
	}
	response := map[string]any{"ok": true, "calibration_result": result}
	if req.Save {
		summary, err := calibration.WriteRecord(loaded, result)
		if err != nil {
			writeError(w, err)
			return
		}
		response["calibration_record"] = summary
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleOptimizationRun(w http.ResponseWriter, r *http.Request) {
	var req optimizationRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apperror.Wrap(apperror.CodeInput, err))
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if req.Save {
		if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
			writeError(w, err)
			return
		}
	}
	if problems := projectSourceErrorProblems(r.Context(), loaded); len(problems) > 0 {
		writeErrorWithProblems(w, apperror.Errorf(apperror.CodeValidation, "project source validation failed"), problems)
		return
	}
	if req.SetupPath == "" {
		setups := loadOptimizationSetupSummaries(loaded.Root)
		if len(setups) == 0 {
			writeError(w, apperror.Errorf(apperror.CodeValidation, "optimization requires a saved setup"))
			return
		}
		req.SetupPath = setups[0].RelativePath
	}
	setup, err := optimization.LoadSetup(loaded.Root, req.SetupPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if req.Save && req.SaveScenario == "" {
		req.SaveScenario = filepath.ToSlash(filepath.Join("scenarios", setup.ID+"_optimized.json"))
	}
	if req.Save && req.SaveParameterSet == "" && optimizationSetupHasParameterVariables(setup) {
		req.SaveParameterSet = filepath.ToSlash(filepath.Join("parameter_sets", setup.ID+"_optimized.json"))
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	result, err := optimization.Run(ctx, loaded.Path, setup, optimization.Options{
		SaveScenario:     req.SaveScenario,
		SaveParameterSet: req.SaveParameterSet,
	})
	if err != nil {
		writeErrorWithProblems(w, err, inferProblems(loaded, err))
		return
	}
	response := map[string]any{"ok": true, "optimization_result": result}
	if req.Save {
		summary, err := optimization.WriteRecord(loaded, result)
		if err != nil {
			writeError(w, err)
			return
		}
		response["optimization_record"] = summary
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleSchema(w http.ResponseWriter, r *http.Request) {
	req, err := decodeRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	schema, err := schemaexport.Export(loaded)
	if err != nil {
		writeError(w, apperror.Wrap(apperror.CodeValidation, err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "schema": schema})
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	req, err := decodeExportRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.ensureWorkspaceProject(loaded.Root); err != nil {
		writeError(w, err)
		return
	}
	if problems := projectSourceErrorProblems(r.Context(), loaded); len(problems) > 0 {
		writeErrorWithProblems(w, apperror.Errorf(apperror.CodeValidation, "project source validation failed"), problems)
		return
	}
	summary, manifest, err := writeExportManifest(loaded, req.Profile, exportOptionsFromRequest(req))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "summary": summary, "export": manifest})
}

func (s *Server) loadProject(projectPath string) (*project.LoadedProject, error) {
	resolved, err := s.resolveProjectPath(projectPath)
	if err != nil {
		return nil, err
	}
	loaded, err := project.Load(resolved)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeValidation, err)
	}
	return loaded, nil
}

func projectDetail(loaded *project.LoadedProject) ProjectDetail {
	detail := ProjectDetail{
		Project:             loaded.Project,
		Graph:               loaded.Graph,
		ProjectPath:         loaded.Path,
		GraphPath:           loaded.GraphPath,
		Layout:              loadStudioLayout(loaded.Root),
		Root:                loaded.Root,
		Runs:                loadRunSummaries(loaded.Root),
		Batches:             loadBatchSummaries(loaded.Root),
		Exports:             loadExportSummaries(loaded.Root),
		Scenarios:           loadScenarioSummaries(loaded.Root),
		Datasets:            loadDatasetSummaries(loaded.Root),
		SeriesInputs:        loadSeriesInputSummaries(loaded.Root),
		ParameterSets:       loadParameterSetSummaries(loaded.Root),
		ValidationMappings:  loadValidationMappingSummaries(loaded.Root),
		CalibrationSetups:   loadCalibrationSetupSummaries(loaded.Root),
		OptimizationSetups:  loadOptimizationSetupSummaries(loaded.Root),
		ValidationRuns:      modelvalidation.LoadRecordSummaries(loaded.Root),
		CalibrationResults:  calibration.LoadRecordSummaries(loaded.Root),
		OptimizationResults: optimization.LoadRecordSummaries(loaded.Root),
		MLValidationReports: mlValidationSummaryMap(mlValidationSummaries(loaded, true, false)),
	}
	if inputPath, err := resolveProjectOwnedFile(loaded.Root, loaded.Project.DefaultInput); err == nil {
		detail.DefaultInputPath = inputPath
		if input, err := runtimecore.LoadInput(inputPath); err == nil {
			detail.DefaultRunInput = &input
		}
	}
	return detail
}

func loadStudioLayout(projectRoot string) StudioLayout {
	layout := StudioLayout{Components: map[string]CanvasPosition{}}
	path := filepath.Join(projectRoot, "studio", "layout.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return layout
	}
	if err := json.Unmarshal(data, &layout); err != nil {
		return StudioLayout{Components: map[string]CanvasPosition{}}
	}
	if layout.Components == nil {
		layout.Components = map[string]CanvasPosition{}
	}
	return layout
}

func writeStudioLayout(loaded *project.LoadedProject, positions map[string]CanvasPosition) error {
	componentIDs := map[string]bool{}
	for _, component := range loaded.Graph.Components {
		componentIDs[component.ID] = true
	}
	layout := StudioLayout{Components: map[string]CanvasPosition{}}
	for componentID, position := range positions {
		if !componentIDs[componentID] {
			continue
		}
		if position.X < 0 {
			position.X = 0
		}
		if position.Y < 0 {
			position.Y = 0
		}
		layout.Components[componentID] = position
	}
	path := filepath.Join(loaded.Root, "studio", "layout.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return apperror.Wrap(apperror.CodeRuntime, err)
	}
	if err := writeJSONFile(path, layout); err != nil {
		return err
	}
	return nil
}

func (s *Server) findProjectSummaries(root string, source string) []ProjectSummary {
	projects := []ProjectSummary{}
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != "project.bcsproj" {
			return nil
		}
		loaded, err := project.Load(path)
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(s.repoRoot, path)
		projects = append(projects, ProjectSummary{
			Name:         loaded.Project.ProjectName,
			ProjectPath:  path,
			RelativePath: filepath.ToSlash(rel),
			Source:       source,
		})
		return nil
	})
	return projects
}

func (s *Server) createProject(req createProjectRequest) (ProjectSummary, error) {
	projectName := strings.TrimSpace(req.Name)
	if projectName == "" {
		return ProjectSummary{}, apperror.Errorf(apperror.CodeValidation, "project name is required")
	}
	template := req.Template
	if template == "" {
		template = "scalar"
	}
	if template != "scalar" {
		return ProjectSummary{}, apperror.Errorf(apperror.CodeValidation, "unsupported project template: %s", template)
	}

	slug := slugify(projectName)
	if slug == "" {
		return ProjectSummary{}, apperror.Errorf(apperror.CodeValidation, "project name must contain letters or numbers")
	}
	projectRoot := filepath.Join(s.repoRoot, "projects", slug)
	if _, err := os.Stat(projectRoot); err == nil {
		return ProjectSummary{}, apperror.Errorf(apperror.CodeValidation, "project already exists: projects/%s", slug)
	}

	templateRoot := filepath.Join(s.repoRoot, "templates", "projects", template)
	if _, err := os.Stat(templateRoot); err != nil {
		return ProjectSummary{}, apperror.Errorf(apperror.CodeValidation, "project template is missing: templates/projects/%s", template)
	}
	if err := copyProjectTree(templateRoot, projectRoot); err != nil {
		return ProjectSummary{}, err
	}
	if err := os.MkdirAll(filepath.Join(projectRoot, "runs"), 0o755); err != nil {
		return ProjectSummary{}, err
	}
	if err := os.MkdirAll(filepath.Join(projectRoot, "scenarios"), 0o755); err != nil {
		return ProjectSummary{}, err
	}
	if err := os.MkdirAll(filepath.Join(projectRoot, "exports"), 0o755); err != nil {
		return ProjectSummary{}, err
	}
	projectPath := filepath.Join(projectRoot, "project.bcsproj")
	projBytes, err := os.ReadFile(projectPath)
	if err != nil {
		return ProjectSummary{}, err
	}
	var proj model.Project
	if err := json.Unmarshal(projBytes, &proj); err != nil {
		return ProjectSummary{}, apperror.Wrap(apperror.CodeValidation, err)
	}
	proj.ProjectName = projectName
	if err := writeJSONFile(projectPath, proj); err != nil {
		return ProjectSummary{}, err
	}

	rel, _ := filepath.Rel(s.repoRoot, projectPath)
	return ProjectSummary{
		Name:         projectName,
		ProjectPath:  projectPath,
		RelativePath: filepath.ToSlash(rel),
		Source:       "workspace",
	}, nil
}

func (s *Server) copyProject(req copyProjectRequest) (ProjectSummary, error) {
	loaded, err := s.loadProject(req.ProjectPath)
	if err != nil {
		return ProjectSummary{}, err
	}
	projectName := strings.TrimSpace(req.Name)
	if projectName == "" {
		projectName = loaded.Project.ProjectName + " Copy"
	}
	slugBase := slugify(projectName)
	if slugBase == "" {
		return ProjectSummary{}, apperror.Errorf(apperror.CodeValidation, "project name must contain letters or numbers")
	}

	workspaceRoot := filepath.Join(s.repoRoot, "projects")
	if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
		return ProjectSummary{}, err
	}
	slug := slugBase
	targetName := projectName
	targetRoot := filepath.Join(workspaceRoot, slug)
	for index := 2; ; index++ {
		if _, err := os.Stat(targetRoot); os.IsNotExist(err) {
			break
		} else if err != nil {
			return ProjectSummary{}, err
		}
		slug = fmt.Sprintf("%s-%d", slugBase, index)
		targetName = fmt.Sprintf("%s %d", projectName, index)
		targetRoot = filepath.Join(workspaceRoot, slug)
	}

	if err := copyProjectTree(loaded.Root, targetRoot); err != nil {
		return ProjectSummary{}, err
	}
	projectPath := filepath.Join(targetRoot, "project.bcsproj")
	copied, err := project.Load(projectPath)
	if err != nil {
		return ProjectSummary{}, apperror.Wrap(apperror.CodeValidation, err)
	}
	copied.Project.ProjectName = targetName
	if err := writeJSONFile(projectPath, copied.Project); err != nil {
		return ProjectSummary{}, err
	}
	rel, _ := filepath.Rel(s.repoRoot, projectPath)
	return ProjectSummary{
		Name:         targetName,
		ProjectPath:  projectPath,
		RelativePath: filepath.ToSlash(rel),
		Source:       "workspace",
	}, nil
}

func createComponent(loaded *project.LoadedProject, req createComponentRequest, repoRoot string) (model.Component, error) {
	componentName := strings.TrimSpace(req.Name)
	if componentName == "" {
		return model.Component{}, apperror.Errorf(apperror.CodeValidation, "component name is required")
	}
	template := req.Template
	if template == "" {
		template = "scalar"
	}
	templateManifest, templateFiles, err := loadComponentTemplate(repoRoot, template)
	if err != nil {
		return model.Component{}, err
	}

	componentID := uniqueComponentID(loaded.Graph, strings.ReplaceAll(slugify(componentName), "-", "_"))
	if componentID == "" {
		return model.Component{}, apperror.Errorf(apperror.CodeValidation, "component name must contain letters or numbers")
	}
	className := pythonClassName(componentID)
	component := componentFromTemplateManifest(componentID, componentName, templateManifest)

	componentsRoot := filepath.Join(loaded.Root, "components")
	if err := os.MkdirAll(componentsRoot, 0o755); err != nil {
		return model.Component{}, err
	}
	initPath := filepath.Join(componentsRoot, "__init__.py")
	if _, err := os.Stat(initPath); os.IsNotExist(err) {
		if err := os.WriteFile(initPath, []byte(""), 0o644); err != nil {
			return model.Component{}, err
		}
	}
	if err := writeComponentTemplateFiles(loaded.Root, component, templateFiles, templateManifest.ClassName, className); err != nil {
		return model.Component{}, err
	}
	if component.Source.Metadata != "" {
		metadataPath, err := resolveProjectOwnedFile(loaded.Root, component.Source.Metadata)
		if err != nil {
			return model.Component{}, err
		}
		if err := writeComponentMetadataFile(metadataPath, component, className); err != nil {
			return model.Component{}, err
		}
	}
	loaded.Graph.Components = append(loaded.Graph.Components, component)
	if err := writeJSONFile(loaded.GraphPath, loaded.Graph); err != nil {
		return model.Component{}, err
	}
	return component, nil
}

func componentFromTemplateManifest(componentID string, componentName string, manifest componentTemplateManifest) model.Component {
	className := pythonClassName(componentID)
	componentSource := componentSourceForTemplate(manifest.Source, componentID)
	return model.Component{
		ID:            componentID,
		Name:          componentName,
		Kind:          defaultString(manifest.Kind, "user_python"),
		Category:      defaultString(manifest.Category, "utility"),
		ExecutionMode: defaultString(manifest.ExecutionMode, "step"),
		Class:         classPathForComponentSource(componentID, componentSource, className),
		Source:        componentSource,
		Nodes: model.NodeSet{
			Inputs:  componentTemplateNodes(manifest.Inputs, "inlet"),
			Outputs: componentTemplateNodes(manifest.Outputs, "outlet"),
		},
		Parameters:           cloneMap(manifest.Parameters),
		ParameterDefinitions: cloneParameterDefinitions(manifest.ParameterDefinitions),
		StateDefinitions:     cloneStateDefinitions(manifest.StateDefinitions),
		SolverBoundary:       cloneSolverBoundary(manifest.SolverBoundary),
		MLMetadata:           componentMLMetadataForTemplate(manifest.MLMetadata, componentID),
	}
}

func duplicateComponent(loaded *project.LoadedProject, req duplicateComponentRequest) (model.Component, error) {
	sourceID := strings.TrimSpace(req.SourceComponentID)
	if sourceID == "" {
		return model.Component{}, apperror.Errorf(apperror.CodeValidation, "source_component_id is required")
	}
	source, found := findComponent(loaded.Graph, sourceID)
	if !found {
		return model.Component{}, apperror.Errorf(apperror.CodeValidation, "component not found: %s", sourceID)
	}
	componentName := strings.TrimSpace(req.Name)
	if componentName == "" {
		componentName = strings.TrimSpace(source.Name) + " Copy"
	}
	if strings.TrimSpace(componentName) == "" {
		componentName = sourceID + " Copy"
	}

	componentID := uniqueComponentID(loaded.Graph, strings.ReplaceAll(slugify(componentName), "-", "_"))
	classParts := strings.Split(source.Class, ".")
	if len(classParts) < 1 || classParts[len(classParts)-1] == "" {
		return model.Component{}, apperror.Errorf(apperror.CodeValidation, "component %s class is invalid: %s", sourceID, source.Class)
	}
	className := classParts[len(classParts)-1]
	component := source
	component.ID = componentID
	component.Name = componentName
	component.Class = "components." + componentID + "." + className
	component.Nodes.Inputs = append([]model.Node(nil), source.Nodes.Inputs...)
	component.Nodes.Outputs = append([]model.Node(nil), source.Nodes.Outputs...)
	component.Parameters = map[string]any{}
	for name, value := range source.Parameters {
		component.Parameters[name] = value
	}
	component.ParameterDefinitions = cloneParameterDefinitions(source.ParameterDefinitions)
	component.StateDefinitions = cloneStateDefinitions(source.StateDefinitions)
	component.SolverBoundary = cloneSolverBoundary(source.SolverBoundary)
	component.MLMetadata = cloneMLMetadata(source.MLMetadata)
	component.Source = cloneComponentSource(source.Source)
	if component.Source.Layout == "" || component.Source.Layout == "single_file_class" {
		component.Source.Layout = "single_file_class"
		component.Source.Step = filepath.ToSlash(filepath.Join("components", componentID+".py"))
		component.Class = "components." + componentID + "." + className
	} else if component.Source.Layout == "generated_wrapper" {
		sourceDirRel, err := generatedComponentSourceDir(component.Source)
		if err != nil {
			return model.Component{}, err
		}
		targetDirRel := filepath.ToSlash(filepath.Join("components", componentID))
		component.Source = rebaseComponentSource(component.Source, sourceDirRel, targetDirRel)
		component.Class = classPathForComponentSource(componentID, component.Source, className)
	}

	sourceRoot := filepath.Join(loaded.Root, "components")
	if err := os.MkdirAll(sourceRoot, 0o755); err != nil {
		return model.Component{}, err
	}
	copiedPath, err := copyComponentSourceArtifact(loaded, source, component)
	if err != nil {
		return model.Component{}, err
	}
	if component.Source.Metadata != "" {
		metadataPath, err := resolveProjectOwnedFile(loaded.Root, component.Source.Metadata)
		if err != nil {
			return model.Component{}, err
		}
		if err := writeComponentMetadataFile(metadataPath, component, className); err != nil {
			return model.Component{}, err
		}
	}
	loaded.Graph.Components = append(loaded.Graph.Components, component)
	if err := writeJSONFile(loaded.GraphPath, loaded.Graph); err != nil {
		_ = removeComponentSourceArtifact(copiedPath, component.Source.Layout)
		return model.Component{}, err
	}
	return component, nil
}

func replaceComponent(loaded *project.LoadedProject, req replaceComponentRequest, repoRoot string) (model.Component, ComponentReplacementSummary, []Problem, error) {
	componentID := strings.TrimSpace(req.ComponentID)
	if componentID == "" {
		return model.Component{}, ComponentReplacementSummary{}, nil, apperror.Errorf(apperror.CodeValidation, "component_id is required")
	}
	source, found := findComponent(loaded.Graph, componentID)
	if !found {
		return model.Component{}, ComponentReplacementSummary{}, nil, apperror.Errorf(apperror.CodeValidation, "component not found: %s", componentID)
	}
	replacementName := strings.TrimSpace(req.Name)
	if replacementName == "" {
		replacementName = strings.TrimSpace(source.Name) + " Replacement"
	}
	if strings.TrimSpace(replacementName) == "" {
		replacementName = componentID + " Replacement"
	}
	template := strings.TrimSpace(req.Template)
	if template == "" {
		template = "scalar"
	}
	mapParameters := boolOption(req.MapParameters, true)
	replacement, err := createComponent(loaded, createComponentRequest{
		ProjectPath: loaded.Path,
		Name:        replacementName,
		Template:    template,
	}, repoRoot)
	if err != nil {
		return model.Component{}, ComponentReplacementSummary{}, nil, err
	}
	summary := replacementSummary(loaded, source, replacement, template, mapParameters)
	if len(summary.Problems) > 0 {
		_ = rollbackReplacementComponent(loaded, replacement)
		return model.Component{}, summary, summary.Problems, apperror.Errorf(apperror.CodeValidation, "replacement component is not contract-compatible: %s", strings.Join(problemMessages(summary.Problems), "; "))
	}
	updatedReplacement, parameterMappings, mappedParameters := applyReplacementParameterMapping(loaded, source, replacement, mapParameters)
	replacement = updatedReplacement
	summary.ParameterMappings = parameterMappings
	summary.MappedParameters = mappedParameters
	if err := syncReplacementComponent(loaded, replacement); err != nil {
		_ = rollbackReplacementComponent(loaded, replacement)
		return model.Component{}, summary, nil, err
	}
	if err := rewireReplacementComponent(loaded, source, replacement, &summary); err != nil {
		_ = rollbackReplacementComponent(loaded, replacement)
		return model.Component{}, summary, nil, err
	}
	return replacement, summary, nil, nil
}

func rewireReplacementComponent(loaded *project.LoadedProject, source model.Component, replacement model.Component, summary *ComponentReplacementSummary) error {
	systemIndex := entrySystemIndex(loaded)
	if systemIndex < 0 {
		return apperror.Errorf(apperror.CodeValidation, "entry system not found: %s", loaded.Project.EntrySystem)
	}
	system := &loaded.Graph.Systems[systemIndex]
	if !containsString(system.Components, source.ID) {
		return nil
	}
	for index, componentID := range system.Components {
		if componentID == source.ID {
			system.Components[index] = replacement.ID
			summary.SystemReplaced = true
		}
	}
	for index := range system.PublicInputs {
		if system.PublicInputs[index].Component == source.ID {
			system.PublicInputs[index].Component = replacement.ID
			summary.RewiredPublicInputs++
		}
	}
	for index := range system.PublicOutputs {
		if system.PublicOutputs[index].Component == source.ID {
			system.PublicOutputs[index].Component = replacement.ID
			summary.RewiredPublicOutputs++
		}
	}
	for index := range loaded.Graph.Connections {
		connection := &loaded.Graph.Connections[index]
		if !containsString(system.Connections, connection.ID) {
			continue
		}
		rewired := false
		if connection.From.Component == source.ID {
			connection.From.Component = replacement.ID
			rewired = true
		}
		if connection.To.Component == source.ID {
			connection.To.Component = replacement.ID
			rewired = true
		}
		if rewired {
			summary.RewiredConnections++
		}
	}
	if err := writeJSONFile(loaded.GraphPath, loaded.Graph); err != nil {
		return err
	}
	return nil
}

func replacementSummary(loaded *project.LoadedProject, source model.Component, replacement model.Component, template string, mapParameters bool) ComponentReplacementSummary {
	summary := ComponentReplacementSummary{
		OriginalComponent:         source.ID,
		ReplacementComponent:      replacement.ID,
		Template:                  template,
		OriginalComponentRetained: true,
		MapParameters:             mapParameters,
		Diff:                      componentReplacementDiff(source, replacement),
	}
	systemIndex := entrySystemIndex(loaded)
	if systemIndex < 0 {
		summary.Problems = []Problem{{
			Severity: "error",
			Message:  fmt.Sprintf("entry system not found: %s", loaded.Project.EntrySystem),
		}}
		return summary
	}
	system := loaded.Graph.Systems[systemIndex]
	summary.NodeMappings = replacementNodeMappings(system, loaded.Graph, source, replacement)
	if containsString(system.Components, source.ID) {
		summary.Problems = replacementCompatibilityProblems(system, loaded.Graph, source, replacement)
	}
	return summary
}

func replacementCompatibilityProblems(system model.System, graph *model.Graph, source model.Component, replacement model.Component) []Problem {
	problems := []Problem{}
	for _, input := range system.PublicInputs {
		if input.Component == source.ID && !componentHasInputNode(replacement, input.Node) {
			problems = append(problems, Problem{Severity: "error", ComponentID: source.ID, NodeID: input.Node, Message: fmt.Sprintf("replacement missing input node for public input %s: %s", input.ID, input.Node)})
		}
	}
	for _, output := range system.PublicOutputs {
		if output.Component == source.ID && !componentHasOutputNode(replacement, output.Node) {
			problems = append(problems, Problem{Severity: "error", ComponentID: source.ID, NodeID: output.Node, Message: fmt.Sprintf("replacement missing output node for public output %s: %s", output.ID, output.Node)})
		}
	}
	for _, connectionID := range system.Connections {
		connection, found := findConnection(graph, connectionID)
		if !found {
			continue
		}
		if connection.From.Component == source.ID && !componentHasOutputNode(replacement, connection.From.Node) {
			problems = append(problems, Problem{Severity: "error", ComponentID: source.ID, NodeID: connection.From.Node, Message: fmt.Sprintf("replacement missing output node for connection %s: %s", connection.ID, connection.From.Node)})
		}
		if connection.To.Component == source.ID && !componentHasInputNode(replacement, connection.To.Node) {
			problems = append(problems, Problem{Severity: "error", ComponentID: source.ID, NodeID: connection.To.Node, Message: fmt.Sprintf("replacement missing input node for connection %s: %s", connection.ID, connection.To.Node)})
		}
	}
	return problems
}

func replacementNodeMappings(system model.System, graph *model.Graph, source model.Component, replacement model.Component) []ComponentReplacementMapping {
	mappings := []ComponentReplacementMapping{}
	for _, input := range system.PublicInputs {
		if input.Component != source.ID {
			continue
		}
		status := "preserved"
		detail := "public input"
		if !componentHasInputNode(replacement, input.Node) {
			status = "missing"
			detail = "replacement input node is missing"
		}
		mappings = append(mappings, ComponentReplacementMapping{
			Scope:  "public_input",
			ID:     input.ID,
			From:   endpointLabel(source.ID, input.Node),
			To:     endpointLabel(replacement.ID, input.Node),
			Status: status,
			Detail: detail,
		})
	}
	for _, output := range system.PublicOutputs {
		if output.Component != source.ID {
			continue
		}
		status := "preserved"
		detail := "public output"
		if !componentHasOutputNode(replacement, output.Node) {
			status = "missing"
			detail = "replacement output node is missing"
		}
		mappings = append(mappings, ComponentReplacementMapping{
			Scope:  "public_output",
			ID:     output.ID,
			From:   endpointLabel(source.ID, output.Node),
			To:     endpointLabel(replacement.ID, output.Node),
			Status: status,
			Detail: detail,
		})
	}
	for _, connectionID := range system.Connections {
		connection, found := findConnection(graph, connectionID)
		if !found {
			continue
		}
		if connection.From.Component == source.ID {
			status := "preserved"
			detail := "connection source"
			if !componentHasOutputNode(replacement, connection.From.Node) {
				status = "missing"
				detail = "replacement output node is missing"
			}
			mappings = append(mappings, ComponentReplacementMapping{
				Scope:  "connection_output",
				ID:     connection.ID,
				From:   endpointLabel(source.ID, connection.From.Node),
				To:     endpointLabel(replacement.ID, connection.From.Node),
				Status: status,
				Detail: detail,
			})
		}
		if connection.To.Component == source.ID {
			status := "preserved"
			detail := "connection target"
			if !componentHasInputNode(replacement, connection.To.Node) {
				status = "missing"
				detail = "replacement input node is missing"
			}
			mappings = append(mappings, ComponentReplacementMapping{
				Scope:  "connection_input",
				ID:     connection.ID,
				From:   endpointLabel(source.ID, connection.To.Node),
				To:     endpointLabel(replacement.ID, connection.To.Node),
				Status: status,
				Detail: detail,
			})
		}
	}
	return mappings
}

func applyReplacementParameterMapping(loaded *project.LoadedProject, source model.Component, replacement model.Component, mapParameters bool) (model.Component, []ComponentReplacementMapping, int) {
	mappings := []ComponentReplacementMapping{}
	mapped := 0
	for _, name := range componentParameterIDs(replacement) {
		sourceValue, hasSourceValue := source.Parameters[name]
		status := "missing"
		detail := "source parameter is not present"
		if !mapParameters {
			status = "skipped"
			detail = "parameter mapping disabled"
		} else if hasSourceValue {
			if replacement.Parameters == nil {
				replacement.Parameters = map[string]any{}
			}
			replacement.Parameters[name] = sourceValue
			if replacement.ParameterDefinitions != nil {
				definition := replacement.ParameterDefinitions[name]
				definition.Current = sourceValue
				replacement.ParameterDefinitions[name] = definition
			}
			status = "copied"
			detail = "same-name parameter value copied"
			mapped++
		}
		mappings = append(mappings, ComponentReplacementMapping{
			Scope:  "parameter",
			ID:     name,
			From:   source.ID + "." + name,
			To:     replacement.ID + "." + name,
			Status: status,
			Detail: detail,
		})
	}
	return replacement, mappings, mapped
}

func syncReplacementComponent(loaded *project.LoadedProject, replacement model.Component) error {
	for index := range loaded.Graph.Components {
		if loaded.Graph.Components[index].ID == replacement.ID {
			loaded.Graph.Components[index] = replacement
			if err := syncComponentMetadataFile(loaded, replacement); err != nil {
				return err
			}
			return writeJSONFile(loaded.GraphPath, loaded.Graph)
		}
	}
	return apperror.Errorf(apperror.CodeValidation, "replacement component not found after creation: %s", replacement.ID)
}

func componentReplacementDiff(source model.Component, replacement model.Component) ComponentReplacementDiff {
	sourceInputs := inputNodeIDs(source)
	replacementInputs := inputNodeIDs(replacement)
	sourceOutputs := outputNodeIDs(source)
	replacementOutputs := outputNodeIDs(replacement)
	sourceParameters := componentParameterIDs(source)
	replacementParameters := componentParameterIDs(replacement)
	return ComponentReplacementDiff{
		OriginalInputs:        sourceInputs,
		ReplacementInputs:     replacementInputs,
		MatchedInputs:         intersectStrings(sourceInputs, replacementInputs),
		MissingInputs:         differenceStrings(sourceInputs, replacementInputs),
		AddedInputs:           differenceStrings(replacementInputs, sourceInputs),
		OriginalOutputs:       sourceOutputs,
		ReplacementOutputs:    replacementOutputs,
		MatchedOutputs:        intersectStrings(sourceOutputs, replacementOutputs),
		MissingOutputs:        differenceStrings(sourceOutputs, replacementOutputs),
		AddedOutputs:          differenceStrings(replacementOutputs, sourceOutputs),
		OriginalParameters:    sourceParameters,
		ReplacementParameters: replacementParameters,
		MatchedParameters:     intersectStrings(sourceParameters, replacementParameters),
		MissingParameters:     differenceStrings(sourceParameters, replacementParameters),
		AddedParameters:       differenceStrings(replacementParameters, sourceParameters),
	}
}

func inputNodeIDs(component model.Component) []string {
	ids := make([]string, 0, len(component.Nodes.Inputs))
	for _, node := range component.Nodes.Inputs {
		ids = append(ids, node.ID)
	}
	sort.Strings(ids)
	return ids
}

func outputNodeIDs(component model.Component) []string {
	ids := make([]string, 0, len(component.Nodes.Outputs))
	for _, node := range component.Nodes.Outputs {
		ids = append(ids, node.ID)
	}
	sort.Strings(ids)
	return ids
}

func componentParameterIDs(component model.Component) []string {
	seen := map[string]bool{}
	for name := range component.Parameters {
		if strings.TrimSpace(name) != "" {
			seen[name] = true
		}
	}
	for name := range component.ParameterDefinitions {
		if strings.TrimSpace(name) != "" {
			seen[name] = true
		}
	}
	ids := make([]string, 0, len(seen))
	for name := range seen {
		ids = append(ids, name)
	}
	sort.Strings(ids)
	return ids
}

func intersectStrings(left []string, right []string) []string {
	rightSet := map[string]bool{}
	for _, value := range right {
		rightSet[value] = true
	}
	out := []string{}
	for _, value := range left {
		if rightSet[value] {
			out = append(out, value)
		}
	}
	return out
}

func differenceStrings(left []string, right []string) []string {
	rightSet := map[string]bool{}
	for _, value := range right {
		rightSet[value] = true
	}
	out := []string{}
	for _, value := range left {
		if !rightSet[value] {
			out = append(out, value)
		}
	}
	return out
}

func endpointLabel(componentID string, nodeID string) string {
	return componentID + "." + nodeID
}

func problemMessages(problems []Problem) []string {
	messages := make([]string, 0, len(problems))
	for _, problem := range problems {
		if strings.TrimSpace(problem.Message) != "" {
			messages = append(messages, problem.Message)
		}
	}
	return messages
}

func rollbackReplacementComponent(loaded *project.LoadedProject, replacement model.Component) error {
	copiedPath, pathErr := componentSourceArtifactPath(loaded, replacement)
	kept := loaded.Graph.Components[:0]
	for _, component := range loaded.Graph.Components {
		if component.ID != replacement.ID {
			kept = append(kept, component)
		}
	}
	loaded.Graph.Components = kept
	for index := range loaded.Graph.Systems {
		loaded.Graph.Systems[index].Components = removeString(loaded.Graph.Systems[index].Components, replacement.ID)
	}
	if err := writeJSONFile(loaded.GraphPath, loaded.Graph); err != nil {
		return err
	}
	if pathErr == nil {
		_ = removeComponentSourceArtifact(copiedPath, replacement.Source.Layout)
	}
	return nil
}

func updateComponent(loaded *project.LoadedProject, req updateComponentRequest) (model.Component, error) {
	componentID := strings.TrimSpace(req.ComponentID)
	if componentID == "" {
		return model.Component{}, apperror.Errorf(apperror.CodeValidation, "component_id is required")
	}
	componentName := strings.TrimSpace(req.Name)
	if componentName == "" {
		return model.Component{}, apperror.Errorf(apperror.CodeValidation, "component name is required")
	}
	for index := range loaded.Graph.Components {
		if loaded.Graph.Components[index].ID != componentID {
			continue
		}
		loaded.Graph.Components[index].Name = componentName
		if err := syncComponentMetadataFile(loaded, loaded.Graph.Components[index]); err != nil {
			return model.Component{}, err
		}
		if err := writeJSONFile(loaded.GraphPath, loaded.Graph); err != nil {
			return model.Component{}, err
		}
		return loaded.Graph.Components[index], nil
	}
	return model.Component{}, apperror.Errorf(apperror.CodeValidation, "component not found: %s", componentID)
}

func updateComponentMLAssets(loaded *project.LoadedProject, req updateComponentMLAssetsRequest) (model.Component, []string, error) {
	componentID := strings.TrimSpace(req.ComponentID)
	if componentID == "" {
		return model.Component{}, nil, apperror.Errorf(apperror.CodeValidation, "component_id is required")
	}
	componentIndex := -1
	for index := range loaded.Graph.Components {
		if loaded.Graph.Components[index].ID == componentID {
			componentIndex = index
			break
		}
	}
	if componentIndex < 0 {
		return model.Component{}, nil, apperror.Errorf(apperror.CodeValidation, "component not found: %s", componentID)
	}

	metadata := cloneMLMetadata(loaded.Graph.Components[componentIndex].MLMetadata)
	if metadata == nil {
		metadata = &model.MLMetadata{}
	}
	metadata.ModelFormat = strings.TrimSpace(req.ModelFormat)
	metadata.RequiredPackages = cleanRequiredPackages(req.RequiredPackages)
	metadata.ValidTimeResolution = strings.TrimSpace(req.ValidTimeResolution)
	if req.ValidInputRanges != nil {
		metadata.ValidInputRanges = cloneValueBoundsMap(req.ValidInputRanges)
	}

	importedFiles := []string{}
	for _, asset := range req.Assets {
		target, err := mlMetadataAssetField(metadata, asset.Field)
		if err != nil {
			return model.Component{}, nil, err
		}
		content, err := decodeMLAssetContent(asset)
		if err != nil {
			return model.Component{}, nil, err
		}
		name, err := cleanMLAssetFileName(asset.Field, asset.FileName)
		if err != nil {
			return model.Component{}, nil, err
		}
		rel := projectComponentSourceRel(componentID, name)
		path, err := resolveProjectOwnedFile(loaded.Root, rel)
		if err != nil {
			return model.Component{}, nil, err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return model.Component{}, nil, err
		}
		if err := os.WriteFile(path, content, 0o644); err != nil {
			return model.Component{}, nil, err
		}
		*target = rel
		importedFiles = append(importedFiles, rel)
	}

	loaded.Graph.Components[componentIndex].MLMetadata = metadata
	if err := syncComponentMetadataFile(loaded, loaded.Graph.Components[componentIndex]); err != nil {
		return model.Component{}, nil, err
	}
	if err := writeJSONFile(loaded.GraphPath, loaded.Graph); err != nil {
		return model.Component{}, nil, err
	}
	return loaded.Graph.Components[componentIndex], importedFiles, nil
}

func mlMetadataAssetField(metadata *model.MLMetadata, field string) (*string, error) {
	switch strings.TrimSpace(field) {
	case "model_file":
		return &metadata.ModelFile, nil
	case "input_scaler_file":
		return &metadata.InputScalerFile, nil
	case "output_scaler_file":
		return &metadata.OutputScalerFile, nil
	case "feature_schema_file":
		return &metadata.FeatureSchemaFile, nil
	case "target_schema_file":
		return &metadata.TargetSchemaFile, nil
	case "training_metadata_file":
		return &metadata.TrainingMetadataFile, nil
	case "validation_report_file":
		return &metadata.ValidationReportFile, nil
	default:
		return nil, apperror.Errorf(apperror.CodeValidation, "unsupported ML asset field: %s", field)
	}
}

func decodeMLAssetContent(asset componentMLAssetUpload) ([]byte, error) {
	if strings.TrimSpace(asset.ContentBase64) != "" {
		content, err := base64.StdEncoding.DecodeString(asset.ContentBase64)
		if err != nil {
			return nil, apperror.Errorf(apperror.CodeInput, "ML asset %s is not valid base64", asset.Field)
		}
		return content, nil
	}
	if asset.Content != "" {
		return []byte(asset.Content), nil
	}
	return nil, apperror.Errorf(apperror.CodeValidation, "ML asset content is required for %s", asset.Field)
}

func cleanMLAssetFileName(field string, fileName string) (string, error) {
	fileName = strings.TrimSpace(fileName)
	if fileName == "" {
		fileName = defaultMLAssetFileName(field)
	}
	if fileName == "" || strings.ContainsAny(fileName, `/\`) {
		return "", apperror.Errorf(apperror.CodeValidation, "ML asset file name must be a file name: %s", fileName)
	}
	clean := filepath.Clean(fileName)
	if clean == "." || clean == ".." || clean != filepath.Base(clean) {
		return "", apperror.Errorf(apperror.CodeValidation, "ML asset file name must be a file name: %s", fileName)
	}
	return clean, nil
}

func defaultMLAssetFileName(field string) string {
	switch strings.TrimSpace(field) {
	case "model_file":
		return "model.bin"
	case "input_scaler_file":
		return "input_scaler.json"
	case "output_scaler_file":
		return "output_scaler.json"
	case "feature_schema_file":
		return "feature_schema.json"
	case "target_schema_file":
		return "target_schema.json"
	case "training_metadata_file":
		return "training_metadata.json"
	case "validation_report_file":
		return "validation_report.json"
	default:
		return ""
	}
}

func cleanRequiredPackages(values []string) []string {
	seen := map[string]bool{}
	cleaned := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		cleaned = append(cleaned, value)
	}
	return cleaned
}

func cloneValueBoundsMap(values map[string]model.ValueBounds) map[string]model.ValueBounds {
	if values == nil {
		return nil
	}
	out := map[string]model.ValueBounds{}
	for key, value := range values {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		out[key] = value
	}
	return out
}

func applyComponentMLSchemaNodes(loaded *project.LoadedProject, req applyComponentMLSchemaNodesRequest) (model.Component, MLSchemaNodeApplySummary, error) {
	componentID := strings.TrimSpace(req.ComponentID)
	if componentID == "" {
		return model.Component{}, MLSchemaNodeApplySummary{}, apperror.Errorf(apperror.CodeValidation, "component_id is required")
	}
	component, found := findComponent(loaded.Graph, componentID)
	if !found {
		return model.Component{}, MLSchemaNodeApplySummary{}, apperror.Errorf(apperror.CodeValidation, "component not found: %s", componentID)
	}
	if component.MLMetadata == nil {
		return model.Component{}, MLSchemaNodeApplySummary{}, apperror.Errorf(apperror.CodeValidation, "component has no ML metadata: %s", componentID)
	}
	features, err := readMLSchemaNodes(loaded.Root, component.MLMetadata.FeatureSchemaFile, []string{"features"}, "float")
	if err != nil {
		return model.Component{}, MLSchemaNodeApplySummary{}, err
	}
	targets, err := readMLSchemaNodes(loaded.Root, component.MLMetadata.TargetSchemaFile, []string{"targets", "outputs"}, "float")
	if err != nil {
		return model.Component{}, MLSchemaNodeApplySummary{}, err
	}
	if len(features) == 0 && len(targets) == 0 {
		return model.Component{}, MLSchemaNodeApplySummary{}, apperror.Errorf(apperror.CodeValidation, "ML feature or target schema is required")
	}

	summary := MLSchemaNodeApplySummary{
		FeatureCount: len(features),
		TargetCount:  len(targets),
	}
	if len(features) > 0 {
		required := true
		current, _ := findComponent(loaded.Graph, componentID)
		if componentHasInputNode(current, "features") {
			summary.ExistingInputs = append(summary.ExistingInputs, "features")
		} else {
			if _, err := createNode(loaded, createNodeRequest{
				ComponentID: componentID,
				Direction:   "input",
				ID:          "features",
				Name:        "Features",
				Medium:      "signal",
				ValueType:   "object",
				Required:    &required,
				Default:     map[string]any{},
			}); err != nil {
				return model.Component{}, MLSchemaNodeApplySummary{}, err
			}
			summary.AddedInputs = append(summary.AddedInputs, "features")
		}
	}
	for _, target := range targets {
		current, _ := findComponent(loaded.Graph, componentID)
		if componentHasOutputNode(current, target.ID) {
			summary.ExistingOutputs = append(summary.ExistingOutputs, target.ID)
			continue
		}
		if componentHasNode(current, target.ID) {
			return model.Component{}, MLSchemaNodeApplySummary{}, apperror.Errorf(apperror.CodeValidation, "schema target conflicts with existing non-output node: %s.%s", componentID, target.ID)
		}
		if _, err := createNode(loaded, createNodeRequest{
			ComponentID: componentID,
			Direction:   "output",
			ID:          target.ID,
			Name:        defaultString(target.Name, displayNameFromID(target.ID)),
			Medium:      defaultString(target.Medium, "signal"),
			ValueType:   defaultString(target.ValueType, "float"),
			Unit:        target.Unit,
		}); err != nil {
			return model.Component{}, MLSchemaNodeApplySummary{}, err
		}
		summary.AddedOutputs = append(summary.AddedOutputs, target.ID)
	}
	updated, found := findComponent(loaded.Graph, componentID)
	if !found {
		return model.Component{}, MLSchemaNodeApplySummary{}, apperror.Errorf(apperror.CodeValidation, "component not found after schema node apply: %s", componentID)
	}
	return updated, summary, nil
}

func readMLSchemaNodes(projectRoot string, rel string, keys []string, fallbackValueType string) ([]mlSchemaNode, error) {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return nil, nil
	}
	path, err := resolveProjectOwnedFile(projectRoot, rel)
	if err != nil {
		return nil, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, apperror.Errorf(apperror.CodeValidation, "ML schema file is missing: %s", filepath.ToSlash(rel))
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(content, &document); err != nil {
		return nil, apperror.Wrap(apperror.CodeValidation, err)
	}
	for _, key := range keys {
		raw, ok := document[key]
		if !ok {
			continue
		}
		nodes, err := decodeMLSchemaNodes(raw, fallbackValueType)
		if err != nil {
			return nil, err
		}
		return nodes, nil
	}
	return nil, nil
}

func decodeMLSchemaNodes(raw json.RawMessage, fallbackValueType string) ([]mlSchemaNode, error) {
	var names []string
	if err := json.Unmarshal(raw, &names); err == nil {
		nodes := []mlSchemaNode{}
		for _, name := range names {
			node, err := mlSchemaNodeFromParts("", name, "", fallbackValueType, "")
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, node)
		}
		return nodes, nil
	}
	var objects []struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Medium    string `json:"medium"`
		ValueType string `json:"value_type"`
		Unit      string `json:"unit"`
	}
	if err := json.Unmarshal(raw, &objects); err != nil {
		return nil, apperror.Wrap(apperror.CodeValidation, err)
	}
	nodes := []mlSchemaNode{}
	for _, item := range objects {
		node, err := mlSchemaNodeFromParts(item.ID, item.Name, item.Medium, defaultString(item.ValueType, fallbackValueType), item.Unit)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

func mlSchemaNodeFromParts(id string, name string, medium string, valueType string, unit string) (mlSchemaNode, error) {
	id = strings.TrimSpace(id)
	name = strings.TrimSpace(name)
	if id == "" {
		id = name
	}
	id = strings.ReplaceAll(slugify(id), "-", "_")
	if id == "" {
		return mlSchemaNode{}, apperror.Errorf(apperror.CodeValidation, "ML schema node id is required")
	}
	if id[0] >= '0' && id[0] <= '9' {
		id = "n_" + id
	}
	if !isIdentifierLike(id) {
		return mlSchemaNode{}, apperror.Errorf(apperror.CodeValidation, "ML schema node id is invalid: %s", id)
	}
	if name == "" {
		name = displayNameFromID(id)
	}
	return mlSchemaNode{
		ID:        id,
		Name:      name,
		Medium:    strings.TrimSpace(medium),
		ValueType: strings.TrimSpace(valueType),
		Unit:      strings.TrimSpace(unit),
	}, nil
}

func deleteComponent(loaded *project.LoadedProject, req deleteComponentRequest) error {
	componentID := strings.TrimSpace(req.ComponentID)
	if componentID == "" {
		return apperror.Errorf(apperror.CodeValidation, "component_id is required")
	}
	componentIndex := -1
	for index := range loaded.Graph.Components {
		if loaded.Graph.Components[index].ID == componentID {
			componentIndex = index
			break
		}
	}
	if componentIndex < 0 {
		return apperror.Errorf(apperror.CodeValidation, "component not found: %s", componentID)
	}
	deletedComponent := loaded.Graph.Components[componentIndex]
	for _, system := range loaded.Graph.Systems {
		if containsString(system.Components, componentID) {
			return apperror.Errorf(apperror.CodeValidation, "component is still used by system %s; remove it from the system first", system.ID)
		}
	}
	for _, connection := range loaded.Graph.Connections {
		if connection.From.Component == componentID || connection.To.Component == componentID {
			return apperror.Errorf(apperror.CodeValidation, "component still has connection reference: %s", connection.ID)
		}
	}

	sourceArtifactPath, err := componentSourceArtifactPath(loaded, deletedComponent)
	if err != nil {
		return err
	}
	sourceShared := false
	for index := range loaded.Graph.Components {
		if index == componentIndex {
			continue
		}
		otherPath, err := componentSourceArtifactPath(loaded, loaded.Graph.Components[index])
		if err == nil && sameFilesystemPath(otherPath, sourceArtifactPath) {
			sourceShared = true
			break
		}
	}

	loaded.Graph.Components = append(loaded.Graph.Components[:componentIndex], loaded.Graph.Components[componentIndex+1:]...)
	if _, err := compiler.Compile(loaded); err != nil {
		return apperror.Wrap(apperror.CodeValidation, err)
	}
	if err := writeJSONFile(loaded.GraphPath, loaded.Graph); err != nil {
		return err
	}
	if !sourceShared {
		if err := removeComponentSourceArtifact(sourceArtifactPath, deletedComponent.Source.Layout); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func includeComponentInSystem(loaded *project.LoadedProject, req includeComponentRequest) error {
	componentID := strings.TrimSpace(req.ComponentID)
	if componentID == "" {
		return apperror.Errorf(apperror.CodeValidation, "component_id is required")
	}
	component, foundComponent := findComponent(loaded.Graph, componentID)
	if !foundComponent {
		return apperror.Errorf(apperror.CodeValidation, "component not found: %s", componentID)
	}

	systemID := req.SystemID
	if systemID == "" {
		systemID = loaded.Project.EntrySystem
	}
	systemIndex := -1
	for index := range loaded.Graph.Systems {
		if loaded.Graph.Systems[index].ID == systemID {
			systemIndex = index
			break
		}
	}
	if systemIndex < 0 {
		return apperror.Errorf(apperror.CodeValidation, "system not found: %s", systemID)
	}

	system := &loaded.Graph.Systems[systemIndex]
	if containsString(system.Components, componentID) {
		return nil
	}

	inputPath, input, err := loadEditableDefaultInput(loaded)
	if err != nil {
		return err
	}
	system.Components = append(system.Components, componentID)
	for _, node := range component.Nodes.Inputs {
		if hasPublicInputFor(*system, componentID, node.ID) {
			continue
		}
		publicID := uniquePublicNodeID(system.PublicInputs, componentID+"_"+node.ID)
		system.PublicInputs = append(system.PublicInputs, model.PublicNodeRef{
			ID:        publicID,
			Name:      node.Name,
			Component: componentID,
			Node:      node.ID,
			Medium:    node.Medium,
			ValueType: node.ValueType,
			Unit:      node.Unit,
			Required:  node.Required,
			Default:   node.Default,
		})
		if _, exists := input.Inputs[publicID]; !exists {
			input.Inputs[publicID] = defaultValueForNode(node)
		}
	}
	for _, node := range component.Nodes.Outputs {
		if hasPublicOutputFor(*system, componentID, node.ID) {
			continue
		}
		publicID := uniquePublicNodeID(system.PublicOutputs, componentID+"_"+node.ID)
		system.PublicOutputs = append(system.PublicOutputs, model.PublicNodeRef{
			ID:        publicID,
			Name:      node.Name,
			Component: componentID,
			Node:      node.ID,
			Medium:    node.Medium,
			ValueType: node.ValueType,
			Unit:      node.Unit,
			Default:   node.Default,
		})
	}

	if err := writeJSONFile(inputPath, input); err != nil {
		return err
	}
	return writeJSONFile(loaded.GraphPath, loaded.Graph)
}

func removeComponentFromSystem(loaded *project.LoadedProject, req includeComponentRequest) error {
	componentID := strings.TrimSpace(req.ComponentID)
	if componentID == "" {
		return apperror.Errorf(apperror.CodeValidation, "component_id is required")
	}
	if _, foundComponent := findComponent(loaded.Graph, componentID); !foundComponent {
		return apperror.Errorf(apperror.CodeValidation, "component not found: %s", componentID)
	}

	systemID := req.SystemID
	if systemID == "" {
		systemID = loaded.Project.EntrySystem
	}
	systemIndex := -1
	for index := range loaded.Graph.Systems {
		if loaded.Graph.Systems[index].ID == systemID {
			systemIndex = index
			break
		}
	}
	if systemIndex < 0 {
		return apperror.Errorf(apperror.CodeValidation, "system not found: %s", systemID)
	}

	system := &loaded.Graph.Systems[systemIndex]
	if !containsString(system.Components, componentID) {
		return nil
	}
	inputPath, input, err := loadEditableDefaultInput(loaded)
	if err != nil {
		return err
	}

	system.Components = removeString(system.Components, componentID)
	removedPublicInputs := removePublicInputsForComponent(system, componentID)
	for _, inputID := range removedPublicInputs {
		delete(input.Inputs, inputID)
	}
	removePublicOutputsForComponent(system, componentID)

	removedConnections := []model.Connection{}
	keptConnectionIDs := system.Connections[:0]
	for _, connectionID := range system.Connections {
		connection, found := findConnection(loaded.Graph, connectionID)
		if !found {
			keptConnectionIDs = append(keptConnectionIDs, connectionID)
			continue
		}
		if connection.From.Component == componentID || connection.To.Component == componentID {
			removedConnections = append(removedConnections, connection)
			continue
		}
		keptConnectionIDs = append(keptConnectionIDs, connectionID)
	}
	system.Connections = keptConnectionIDs

	for _, connection := range removedConnections {
		if connection.To.Component == componentID || !containsString(system.Components, connection.To.Component) {
			continue
		}
		if systemHasIncomingConnection(*system, loaded.Graph, connection.To.Component, connection.To.Node) || hasPublicInputFor(*system, connection.To.Component, connection.To.Node) {
			continue
		}
		component, foundComponent := findComponent(loaded.Graph, connection.To.Component)
		if !foundComponent {
			return apperror.Errorf(apperror.CodeValidation, "connection target component not found: %s", connection.To.Component)
		}
		node, foundNode := findInputNode(component, connection.To.Node)
		if !foundNode {
			return apperror.Errorf(apperror.CodeValidation, "connection target input node not found: %s.%s", connection.To.Component, connection.To.Node)
		}
		publicID := uniquePublicNodeID(system.PublicInputs, connection.To.Component+"_"+connection.To.Node)
		system.PublicInputs = append(system.PublicInputs, model.PublicNodeRef{
			ID:        publicID,
			Name:      node.Name,
			Component: connection.To.Component,
			Node:      node.ID,
			Medium:    node.Medium,
			ValueType: node.ValueType,
			Unit:      node.Unit,
			Required:  node.Required,
			Default:   node.Default,
		})
		if _, exists := input.Inputs[publicID]; !exists {
			input.Inputs[publicID] = defaultValueForNode(node)
		}
	}

	loaded.Graph.Connections = removeUnreferencedConnections(loaded.Graph.Connections, loaded.Graph.Systems)
	if _, err := compiler.Compile(loaded); err != nil {
		return apperror.Wrap(apperror.CodeValidation, err)
	}
	if err := writeJSONFile(inputPath, input); err != nil {
		return err
	}
	return writeJSONFile(loaded.GraphPath, loaded.Graph)
}

func createNode(loaded *project.LoadedProject, req createNodeRequest) (model.Node, error) {
	componentID := strings.TrimSpace(req.ComponentID)
	if componentID == "" {
		return model.Node{}, apperror.Errorf(apperror.CodeValidation, "component_id is required")
	}
	nodeID := strings.TrimSpace(req.ID)
	if !isIdentifierLike(nodeID) {
		return model.Node{}, apperror.Errorf(apperror.CodeValidation, "node id must start with a letter or underscore and contain only letters, numbers, and underscores")
	}
	direction := strings.ToLower(strings.TrimSpace(req.Direction))
	isInput := false
	nodeDirection := ""
	switch direction {
	case "input", "in", "inlet":
		isInput = true
		nodeDirection = "inlet"
	case "output", "out", "outlet":
		nodeDirection = "outlet"
	default:
		return model.Node{}, apperror.Errorf(apperror.CodeValidation, "node direction must be input or output")
	}

	componentIndex := -1
	for index := range loaded.Graph.Components {
		if loaded.Graph.Components[index].ID == componentID {
			componentIndex = index
			break
		}
	}
	if componentIndex < 0 {
		return model.Node{}, apperror.Errorf(apperror.CodeValidation, "component not found: %s", componentID)
	}
	component := &loaded.Graph.Components[componentIndex]
	if componentHasNode(*component, nodeID) {
		return model.Node{}, apperror.Errorf(apperror.CodeValidation, "component already has node: %s.%s", componentID, nodeID)
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = nodeID
	}
	medium := strings.TrimSpace(req.Medium)
	if medium == "" {
		medium = "signal"
	}
	valueType := strings.TrimSpace(req.ValueType)
	if valueType == "" {
		valueType = "float"
	}
	node := model.Node{
		ID:        nodeID,
		Name:      name,
		Direction: nodeDirection,
		Medium:    medium,
		ValueType: valueType,
		Unit:      strings.TrimSpace(req.Unit),
		Required:  req.Required,
		Default:   req.Default,
	}
	if isInput {
		component.Nodes.Inputs = append(component.Nodes.Inputs, node)
	} else {
		component.Nodes.Outputs = append(component.Nodes.Outputs, node)
	}

	inputPath, input, err := loadEditableDefaultInput(loaded)
	if err != nil {
		return model.Node{}, err
	}
	for index := range loaded.Graph.Systems {
		system := &loaded.Graph.Systems[index]
		if !containsString(system.Components, componentID) {
			continue
		}
		if isInput {
			if hasPublicInputFor(*system, componentID, nodeID) {
				continue
			}
			publicID := uniquePublicNodeID(system.PublicInputs, componentID+"_"+nodeID)
			system.PublicInputs = append(system.PublicInputs, model.PublicNodeRef{
				ID:        publicID,
				Name:      node.Name,
				Component: componentID,
				Node:      node.ID,
				Medium:    node.Medium,
				ValueType: node.ValueType,
				Unit:      node.Unit,
				Required:  node.Required,
				Default:   node.Default,
			})
			if _, exists := input.Inputs[publicID]; !exists {
				input.Inputs[publicID] = defaultValueForNode(node)
			}
			continue
		}
		if hasPublicOutputFor(*system, componentID, nodeID) {
			continue
		}
		publicID := uniquePublicNodeID(system.PublicOutputs, componentID+"_"+nodeID)
		system.PublicOutputs = append(system.PublicOutputs, model.PublicNodeRef{
			ID:        publicID,
			Name:      node.Name,
			Component: componentID,
			Node:      node.ID,
			Medium:    node.Medium,
			ValueType: node.ValueType,
			Unit:      node.Unit,
			Default:   node.Default,
		})
	}
	if _, err := compiler.Compile(loaded); err != nil {
		return model.Node{}, apperror.Wrap(apperror.CodeValidation, err)
	}
	if err := syncComponentMetadataFile(loaded, *component); err != nil {
		return model.Node{}, err
	}
	if err := writeJSONFile(inputPath, input); err != nil {
		return model.Node{}, err
	}
	if err := writeJSONFile(loaded.GraphPath, loaded.Graph); err != nil {
		return model.Node{}, err
	}
	return node, nil
}

func updateNode(loaded *project.LoadedProject, req updateNodeRequest) (model.Node, error) {
	componentID := strings.TrimSpace(req.ComponentID)
	nodeID := strings.TrimSpace(req.NodeID)
	if componentID == "" || nodeID == "" {
		return model.Node{}, apperror.Errorf(apperror.CodeValidation, "component_id and node_id are required")
	}

	componentIndex := -1
	for index := range loaded.Graph.Components {
		if loaded.Graph.Components[index].ID == componentID {
			componentIndex = index
			break
		}
	}
	if componentIndex < 0 {
		return model.Node{}, apperror.Errorf(apperror.CodeValidation, "component not found: %s", componentID)
	}

	component := &loaded.Graph.Components[componentIndex]
	nodeIndex := -1
	isInput := true
	for index := range component.Nodes.Inputs {
		if component.Nodes.Inputs[index].ID == nodeID {
			nodeIndex = index
			break
		}
	}
	if nodeIndex < 0 {
		isInput = false
		for index := range component.Nodes.Outputs {
			if component.Nodes.Outputs[index].ID == nodeID {
				nodeIndex = index
				break
			}
		}
	}
	if nodeIndex < 0 {
		return model.Node{}, apperror.Errorf(apperror.CodeValidation, "node not found: %s.%s", componentID, nodeID)
	}

	var node *model.Node
	if isInput {
		node = &component.Nodes.Inputs[nodeIndex]
	} else {
		node = &component.Nodes.Outputs[nodeIndex]
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = nodeID
	}
	medium := strings.TrimSpace(req.Medium)
	if medium == "" {
		medium = "signal"
	}
	valueType := strings.TrimSpace(req.ValueType)
	if valueType == "" {
		valueType = "float"
	}
	node.Name = name
	node.Medium = medium
	node.ValueType = valueType
	node.Unit = strings.TrimSpace(req.Unit)
	node.Required = req.Required
	if req.DefaultProvided {
		node.Default = req.Default
	}
	updatedNode := *node

	inputPath := ""
	var input runtimecore.RunInput
	inputDirty := false
	if isInput {
		var err error
		inputPath, input, err = loadEditableDefaultInput(loaded)
		if err != nil {
			return model.Node{}, err
		}
	}

	for systemIndex := range loaded.Graph.Systems {
		system := &loaded.Graph.Systems[systemIndex]
		if !containsString(system.Components, componentID) {
			continue
		}
		if isInput {
			for refIndex := range system.PublicInputs {
				ref := &system.PublicInputs[refIndex]
				if ref.Component != componentID || ref.Node != nodeID {
					continue
				}
				updatePublicNodeRef(ref, updatedNode)
				if req.DefaultProvided {
					input.Inputs[ref.ID] = defaultValueForNode(updatedNode)
					inputDirty = true
					continue
				}
				if _, exists := input.Inputs[ref.ID]; !exists {
					input.Inputs[ref.ID] = defaultValueForNode(updatedNode)
					inputDirty = true
				}
			}
			continue
		}
		for refIndex := range system.PublicOutputs {
			ref := &system.PublicOutputs[refIndex]
			if ref.Component != componentID || ref.Node != nodeID {
				continue
			}
			updatePublicNodeRef(ref, updatedNode)
		}
	}

	if _, err := compiler.Compile(loaded); err != nil {
		return model.Node{}, apperror.Wrap(apperror.CodeValidation, err)
	}
	if err := syncComponentMetadataFile(loaded, *component); err != nil {
		return model.Node{}, err
	}
	if inputDirty {
		if err := writeJSONFile(inputPath, input); err != nil {
			return model.Node{}, err
		}
	}
	if err := writeJSONFile(loaded.GraphPath, loaded.Graph); err != nil {
		return model.Node{}, err
	}
	return updatedNode, nil
}

func deleteNode(loaded *project.LoadedProject, req deleteNodeRequest) (model.Node, error) {
	componentID := strings.TrimSpace(req.ComponentID)
	nodeID := strings.TrimSpace(req.NodeID)
	if componentID == "" || nodeID == "" {
		return model.Node{}, apperror.Errorf(apperror.CodeValidation, "component_id and node_id are required")
	}

	componentIndex := -1
	for index := range loaded.Graph.Components {
		if loaded.Graph.Components[index].ID == componentID {
			componentIndex = index
			break
		}
	}
	if componentIndex < 0 {
		return model.Node{}, apperror.Errorf(apperror.CodeValidation, "component not found: %s", componentID)
	}
	component := &loaded.Graph.Components[componentIndex]
	node, isInput, foundNode := removeNodeFromComponent(component, nodeID)
	if !foundNode {
		return model.Node{}, apperror.Errorf(apperror.CodeValidation, "node not found: %s.%s", componentID, nodeID)
	}

	inputPath, input, err := loadEditableDefaultInput(loaded)
	if err != nil {
		return model.Node{}, err
	}
	for systemIndex := range loaded.Graph.Systems {
		system := &loaded.Graph.Systems[systemIndex]
		if !containsString(system.Components, componentID) {
			continue
		}
		if isInput {
			for _, inputID := range removePublicInputsFor(system, componentID, nodeID) {
				delete(input.Inputs, inputID)
			}
		} else {
			removePublicOutputsFor(system, componentID, nodeID)
		}

		removedConnections := []model.Connection{}
		keptConnectionIDs := system.Connections[:0]
		for _, connectionID := range system.Connections {
			connection, found := findConnection(loaded.Graph, connectionID)
			if !found {
				keptConnectionIDs = append(keptConnectionIDs, connectionID)
				continue
			}
			if endpointMatches(connection.From, componentID, nodeID) || endpointMatches(connection.To, componentID, nodeID) {
				removedConnections = append(removedConnections, connection)
				continue
			}
			keptConnectionIDs = append(keptConnectionIDs, connectionID)
		}
		system.Connections = keptConnectionIDs

		for _, connection := range removedConnections {
			if endpointMatches(connection.To, componentID, nodeID) || !containsString(system.Components, connection.To.Component) {
				continue
			}
			if systemHasIncomingConnection(*system, loaded.Graph, connection.To.Component, connection.To.Node) || hasPublicInputFor(*system, connection.To.Component, connection.To.Node) {
				continue
			}
			targetComponent, foundComponent := findComponent(loaded.Graph, connection.To.Component)
			if !foundComponent {
				return model.Node{}, apperror.Errorf(apperror.CodeValidation, "connection target component not found: %s", connection.To.Component)
			}
			targetNode, foundTargetNode := findInputNode(targetComponent, connection.To.Node)
			if !foundTargetNode {
				return model.Node{}, apperror.Errorf(apperror.CodeValidation, "connection target input node not found: %s.%s", connection.To.Component, connection.To.Node)
			}
			publicID := uniquePublicNodeID(system.PublicInputs, connection.To.Component+"_"+connection.To.Node)
			system.PublicInputs = append(system.PublicInputs, model.PublicNodeRef{
				ID:        publicID,
				Name:      targetNode.Name,
				Component: connection.To.Component,
				Node:      targetNode.ID,
				Medium:    targetNode.Medium,
				ValueType: targetNode.ValueType,
				Unit:      targetNode.Unit,
				Required:  targetNode.Required,
				Default:   targetNode.Default,
			})
			if _, exists := input.Inputs[publicID]; !exists {
				input.Inputs[publicID] = defaultValueForNode(targetNode)
			}
		}
	}

	loaded.Graph.Connections = removeUnreferencedConnections(loaded.Graph.Connections, loaded.Graph.Systems)
	if _, err := compiler.Compile(loaded); err != nil {
		return model.Node{}, apperror.Wrap(apperror.CodeValidation, err)
	}
	if err := syncComponentMetadataFile(loaded, *component); err != nil {
		return model.Node{}, err
	}
	if err := writeJSONFile(inputPath, input); err != nil {
		return model.Node{}, err
	}
	if err := writeJSONFile(loaded.GraphPath, loaded.Graph); err != nil {
		return model.Node{}, err
	}
	return node, nil
}

func deleteParameter(loaded *project.LoadedProject, req deleteParameterRequest) error {
	componentID := strings.TrimSpace(req.ComponentID)
	name := strings.TrimSpace(req.Name)
	if componentID == "" || name == "" {
		return apperror.Errorf(apperror.CodeValidation, "component_id and name are required")
	}
	for index := range loaded.Graph.Components {
		component := &loaded.Graph.Components[index]
		if component.ID != componentID {
			continue
		}
		if component.Parameters == nil {
			return apperror.Errorf(apperror.CodeValidation, "parameter not found: %s.%s", componentID, name)
		}
		if _, found := component.Parameters[name]; !found {
			return apperror.Errorf(apperror.CodeValidation, "parameter not found: %s.%s", componentID, name)
		}
		delete(component.Parameters, name)
		if component.ParameterDefinitions != nil {
			delete(component.ParameterDefinitions, name)
		}
		if err := syncComponentMetadataFile(loaded, *component); err != nil {
			return err
		}
		return writeJSONFile(loaded.GraphPath, loaded.Graph)
	}
	return apperror.Errorf(apperror.CodeValidation, "component not found: %s", componentID)
}

func updateComponentContract(loaded *project.LoadedProject, req updateComponentContractRequest) (model.Component, error) {
	componentID := strings.TrimSpace(req.ComponentID)
	if componentID == "" {
		return model.Component{}, apperror.Errorf(apperror.CodeValidation, "component_id is required")
	}
	componentIndex := -1
	for index := range loaded.Graph.Components {
		if loaded.Graph.Components[index].ID == componentID {
			componentIndex = index
			break
		}
	}
	if componentIndex < 0 {
		return model.Component{}, apperror.Errorf(apperror.CodeValidation, "component not found: %s", componentID)
	}
	component := &loaded.Graph.Components[componentIndex]

	if req.Parameters != nil && component.Parameters == nil {
		component.Parameters = map[string]any{}
	}
	for name, value := range req.Parameters {
		name = strings.TrimSpace(name)
		if !isIdentifierLike(name) {
			return model.Component{}, apperror.Errorf(apperror.CodeValidation, "parameter name must start with a letter or underscore and contain only letters, numbers, and underscores")
		}
		component.Parameters[name] = value
	}

	if len(req.ParameterDefinitions) > 0 && component.ParameterDefinitions == nil {
		component.ParameterDefinitions = map[string]model.ParameterDefinition{}
	}
	for name, definition := range req.ParameterDefinitions {
		name = strings.TrimSpace(name)
		if !isIdentifierLike(name) {
			return model.Component{}, apperror.Errorf(apperror.CodeValidation, "parameter definition name must start with a letter or underscore and contain only letters, numbers, and underscores")
		}
		current, hasCurrent := component.Parameters[name]
		definition = normalizeParameterDefinition(name, definition, current, hasCurrent)
		if err := validateParameterDefinition(component.ID, name, definition); err != nil {
			return model.Component{}, err
		}
		component.ParameterDefinitions[name] = definition
		if component.Parameters == nil {
			component.Parameters = map[string]any{}
		}
		if _, exists := component.Parameters[name]; !exists {
			if definition.Current != nil {
				component.Parameters[name] = definition.Current
			} else if definition.Default != nil {
				component.Parameters[name] = definition.Default
			}
		}
	}
	for _, name := range req.DeleteParameterDefinitions {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if component.ParameterDefinitions != nil {
			delete(component.ParameterDefinitions, name)
		}
	}

	if len(req.StateDefinitions) > 0 && component.StateDefinitions == nil {
		component.StateDefinitions = map[string]model.StateDefinition{}
	}
	for name, definition := range req.StateDefinitions {
		name = strings.TrimSpace(name)
		if !isIdentifierLike(name) {
			return model.Component{}, apperror.Errorf(apperror.CodeValidation, "state definition name must start with a letter or underscore and contain only letters, numbers, and underscores")
		}
		component.StateDefinitions[name] = normalizeStateDefinition(name, definition)
	}
	for _, name := range req.DeleteStateDefinitions {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if component.StateDefinitions != nil {
			delete(component.StateDefinitions, name)
		}
	}

	if err := syncComponentMetadataFile(loaded, *component); err != nil {
		return model.Component{}, err
	}
	if err := writeJSONFile(loaded.GraphPath, loaded.Graph); err != nil {
		return model.Component{}, err
	}
	return *component, nil
}

func normalizeParameterDefinition(name string, definition model.ParameterDefinition, current any, hasCurrent bool) model.ParameterDefinition {
	if strings.TrimSpace(definition.DisplayName) == "" {
		definition.DisplayName = displayNameFromID(name)
	}
	if definition.Current == nil && hasCurrent {
		definition.Current = current
	}
	if definition.Default == nil && hasCurrent {
		definition.Default = current
	}
	return definition
}

func validateParameterDefinition(componentID string, name string, definition model.ParameterDefinition) error {
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

func normalizeStateDefinition(name string, definition model.StateDefinition) model.StateDefinition {
	if strings.TrimSpace(definition.DisplayName) == "" {
		definition.DisplayName = displayNameFromID(name)
	}
	return definition
}

func createConnection(loaded *project.LoadedProject, req createConnectionRequest) (model.Connection, error) {
	systemID := req.SystemID
	if systemID == "" {
		systemID = loaded.Project.EntrySystem
	}
	systemIndex := -1
	for index := range loaded.Graph.Systems {
		if loaded.Graph.Systems[index].ID == systemID {
			systemIndex = index
			break
		}
	}
	if systemIndex < 0 {
		return model.Connection{}, apperror.Errorf(apperror.CodeValidation, "system not found: %s", systemID)
	}
	fromComponent := strings.TrimSpace(req.FromComponent)
	fromNode := strings.TrimSpace(req.FromNode)
	toComponent := strings.TrimSpace(req.ToComponent)
	toNode := strings.TrimSpace(req.ToNode)
	if fromComponent == "" || fromNode == "" || toComponent == "" || toNode == "" {
		return model.Connection{}, apperror.Errorf(apperror.CodeValidation, "connection endpoints are required")
	}

	system := &loaded.Graph.Systems[systemIndex]
	if !containsString(system.Components, fromComponent) {
		return model.Connection{}, apperror.Errorf(apperror.CodeValidation, "connection source component is not in system: %s", fromComponent)
	}
	if !containsString(system.Components, toComponent) {
		return model.Connection{}, apperror.Errorf(apperror.CodeValidation, "connection target component is not in system: %s", toComponent)
	}

	inputPath, input, err := loadEditableDefaultInput(loaded)
	if err != nil {
		return model.Connection{}, err
	}
	connection := model.Connection{
		ID: uniqueConnectionID(
			loaded.Graph,
			fmt.Sprintf("%s_%s_to_%s_%s", fromComponent, fromNode, toComponent, toNode),
		),
		From: model.Endpoint{Component: fromComponent, Node: fromNode},
		To:   model.Endpoint{Component: toComponent, Node: toNode},
	}
	removedPublicInputs := removePublicInputsFor(system, toComponent, toNode)
	for _, inputID := range removedPublicInputs {
		delete(input.Inputs, inputID)
	}
	loaded.Graph.Connections = append(loaded.Graph.Connections, connection)
	system.Connections = append(system.Connections, connection.ID)
	if _, err := compiler.Compile(loaded); err != nil {
		return model.Connection{}, apperror.Wrap(apperror.CodeValidation, err)
	}
	if err := writeJSONFile(inputPath, input); err != nil {
		return model.Connection{}, err
	}
	if err := writeJSONFile(loaded.GraphPath, loaded.Graph); err != nil {
		return model.Connection{}, err
	}
	return connection, nil
}

func updateConnection(loaded *project.LoadedProject, req updateConnectionRequest) (model.Connection, error) {
	connectionID := strings.TrimSpace(req.ConnectionID)
	if connectionID == "" {
		return model.Connection{}, apperror.Errorf(apperror.CodeValidation, "connection_id is required")
	}
	if !req.UnitConversionWasPresent {
		return model.Connection{}, apperror.Errorf(apperror.CodeValidation, "unit_conversion is required")
	}
	systemID := req.SystemID
	if systemID == "" {
		systemID = loaded.Project.EntrySystem
	}
	systemIndex := -1
	for index := range loaded.Graph.Systems {
		if loaded.Graph.Systems[index].ID == systemID {
			systemIndex = index
			break
		}
	}
	if systemIndex < 0 {
		return model.Connection{}, apperror.Errorf(apperror.CodeValidation, "system not found: %s", systemID)
	}

	connectionIndex := -1
	for index, item := range loaded.Graph.Connections {
		if item.ID == connectionID {
			connectionIndex = index
			break
		}
	}
	if connectionIndex < 0 {
		return model.Connection{}, apperror.Errorf(apperror.CodeValidation, "connection not found: %s", connectionID)
	}
	system := &loaded.Graph.Systems[systemIndex]
	if !containsString(system.Connections, connectionID) {
		return model.Connection{}, apperror.Errorf(apperror.CodeValidation, "connection is not in system %s: %s", systemID, connectionID)
	}

	loaded.Graph.Connections[connectionIndex].UnitConversion = normalizeUnitConversion(req.UnitConversion)
	if _, err := compiler.Compile(loaded); err != nil {
		return model.Connection{}, apperror.Wrap(apperror.CodeValidation, err)
	}
	if err := writeJSONFile(loaded.GraphPath, loaded.Graph); err != nil {
		return model.Connection{}, err
	}
	return loaded.Graph.Connections[connectionIndex], nil
}

func normalizeUnitConversion(conversion *model.UnitConversion) *model.UnitConversion {
	if conversion == nil {
		return nil
	}
	normalized := *conversion
	normalized.Mode = strings.TrimSpace(normalized.Mode)
	if normalized.Mode == "" {
		normalized.Mode = "linear"
	}
	normalized.Description = strings.TrimSpace(normalized.Description)
	return &normalized
}

func deleteConnection(loaded *project.LoadedProject, req deleteConnectionRequest) (model.Connection, error) {
	connectionID := strings.TrimSpace(req.ConnectionID)
	if connectionID == "" {
		return model.Connection{}, apperror.Errorf(apperror.CodeValidation, "connection_id is required")
	}
	systemID := req.SystemID
	if systemID == "" {
		systemID = loaded.Project.EntrySystem
	}
	systemIndex := -1
	for index := range loaded.Graph.Systems {
		if loaded.Graph.Systems[index].ID == systemID {
			systemIndex = index
			break
		}
	}
	if systemIndex < 0 {
		return model.Connection{}, apperror.Errorf(apperror.CodeValidation, "system not found: %s", systemID)
	}

	connectionIndex := -1
	var connection model.Connection
	for index, item := range loaded.Graph.Connections {
		if item.ID == connectionID {
			connectionIndex = index
			connection = item
			break
		}
	}
	if connectionIndex < 0 {
		return model.Connection{}, apperror.Errorf(apperror.CodeValidation, "connection not found: %s", connectionID)
	}

	system := &loaded.Graph.Systems[systemIndex]
	if !containsString(system.Connections, connectionID) {
		return model.Connection{}, apperror.Errorf(apperror.CodeValidation, "system %s does not contain connection: %s", systemID, connectionID)
	}

	inputPath, input, err := loadEditableDefaultInput(loaded)
	if err != nil {
		return model.Connection{}, err
	}
	system.Connections = removeString(system.Connections, connectionID)
	if !graphReferencesConnection(loaded.Graph.Systems, connectionID) {
		loaded.Graph.Connections = append(loaded.Graph.Connections[:connectionIndex], loaded.Graph.Connections[connectionIndex+1:]...)
	}
	if !systemHasIncomingConnection(*system, loaded.Graph, connection.To.Component, connection.To.Node) && !hasPublicInputFor(*system, connection.To.Component, connection.To.Node) {
		component, foundComponent := findComponent(loaded.Graph, connection.To.Component)
		if !foundComponent {
			return model.Connection{}, apperror.Errorf(apperror.CodeValidation, "connection target component not found: %s", connection.To.Component)
		}
		node, foundNode := findInputNode(component, connection.To.Node)
		if !foundNode {
			return model.Connection{}, apperror.Errorf(apperror.CodeValidation, "connection target input node not found: %s.%s", connection.To.Component, connection.To.Node)
		}
		publicID := uniquePublicNodeID(system.PublicInputs, connection.To.Component+"_"+connection.To.Node)
		system.PublicInputs = append(system.PublicInputs, model.PublicNodeRef{
			ID:        publicID,
			Name:      node.Name,
			Component: connection.To.Component,
			Node:      node.ID,
			Medium:    node.Medium,
			ValueType: node.ValueType,
			Unit:      node.Unit,
			Required:  node.Required,
			Default:   node.Default,
		})
		if _, exists := input.Inputs[publicID]; !exists {
			input.Inputs[publicID] = defaultValueForNode(node)
		}
	}
	if _, err := compiler.Compile(loaded); err != nil {
		return model.Connection{}, apperror.Wrap(apperror.CodeValidation, err)
	}
	if err := writeJSONFile(inputPath, input); err != nil {
		return model.Connection{}, err
	}
	if err := writeJSONFile(loaded.GraphPath, loaded.Graph); err != nil {
		return model.Connection{}, err
	}
	return connection, nil
}

func loadComponentSource(loaded *project.LoadedProject, componentID string, readOnly bool) (SourceDetail, error) {
	sourcePath, err := componentSourcePath(loaded, componentID)
	if err != nil {
		return SourceDetail{}, err
	}
	sourceBytes, err := os.ReadFile(sourcePath)
	if err != nil {
		return SourceDetail{}, apperror.Wrap(apperror.CodeValidation, err)
	}
	rel, _ := filepath.Rel(loaded.Root, sourcePath)
	return SourceDetail{
		ComponentID:  componentID,
		RelativePath: filepath.ToSlash(rel),
		Content:      string(sourceBytes),
		ReadOnly:     readOnly,
	}, nil
}

func checkComponentSource(ctx context.Context, loaded *project.LoadedProject, req sourceCheckRequest) (SourceCheck, error) {
	componentID := strings.TrimSpace(req.ComponentID)
	component, found := findComponent(loaded.Graph, componentID)
	if !found {
		return SourceCheck{}, apperror.Errorf(apperror.CodeValidation, "component not found: %s", componentID)
	}
	sourcePath, err := componentSourcePath(loaded, componentID)
	if err != nil {
		return SourceCheck{}, err
	}
	rel, _ := filepath.Rel(loaded.Root, sourcePath)
	expectedClass := classNameFromPath(component.Class)
	expectedFunction := ""
	if component.Source.Layout == "generated_wrapper" {
		expectedFunction = "step"
	}
	check := SourceCheck{
		OK:               true,
		ComponentID:      componentID,
		RelativePath:     filepath.ToSlash(rel),
		ExpectedClass:    expectedClass,
		ExpectedFunction: expectedFunction,
		LineCount:        countLines(req.Content),
		Problems:         []Problem{},
	}
	if strings.TrimSpace(req.Content) == "" {
		check.Problems = append(check.Problems, Problem{Severity: "error", Message: "source is empty", ComponentID: componentID})
	}
	if component.Source.Layout == "generated_wrapper" {
		check.Problems = append(check.Problems, generatedWrapperStepProblems(componentID, req.Content)...)
	} else {
		check.Problems = append(check.Problems, singleFileClassProblems(componentID, req.Content, expectedClass)...)
	}
	if !strings.Contains(req.Content, "return") {
		check.Problems = append(check.Problems, Problem{Severity: "warning", Message: "source has no return statement", ComponentID: componentID})
	}
	check.Problems = append(check.Problems, sourceContractReferenceProblems(component, req.Content)...)
	syntaxProblems := pythonSyntaxProblems(ctx, loaded, componentID, filepath.ToSlash(rel), req.Content)
	check.Problems = append(check.Problems, syntaxProblems...)
	if !hasErrorProblems(syntaxProblems) {
		check.Problems = append(check.Problems, pythonUndefinedNameProblems(ctx, loaded, componentID, filepath.ToSlash(rel), req.Content)...)
	}
	if component.Source.Layout != "generated_wrapper" && !hasErrorProblems(syntaxProblems) && expectedClass != "" {
		check.Problems = append(check.Problems, pythonLoadProblems(ctx, loaded, componentID, filepath.ToSlash(rel), expectedClass, req.Content)...)
	}
	check.OK = !hasErrorProblems(check.Problems)
	return check, nil
}

func checkProjectSources(ctx context.Context, loaded *project.LoadedProject) (int, []Problem) {
	problems := []Problem{}
	count := 0
	for _, component := range loaded.Graph.Components {
		if component.Kind != "user_python" {
			continue
		}
		count++
		source, err := loadComponentSource(loaded, component.ID, false)
		if err != nil {
			problems = append(problems, Problem{
				Severity:    "error",
				Message:     fmt.Sprintf("source check failed: %s", err),
				ComponentID: component.ID,
			})
			continue
		}
		check, err := checkComponentSource(ctx, loaded, sourceCheckRequest{
			ComponentID: component.ID,
			Content:     source.Content,
		})
		if err != nil {
			problems = append(problems, Problem{
				Severity:    "error",
				Message:     fmt.Sprintf("source check failed: %s", err),
				ComponentID: component.ID,
			})
			continue
		}
		problems = append(problems, check.Problems...)
	}
	return count, problems
}

func projectSourceErrorProblems(ctx context.Context, loaded *project.LoadedProject) []Problem {
	_, problems := checkProjectSources(ctx, loaded)
	if hasErrorProblems(problems) {
		return problems
	}
	return []Problem{}
}

func componentSourcePath(loaded *project.LoadedProject, componentID string) (string, error) {
	component, found := findComponent(loaded.Graph, componentID)
	if !found {
		return "", apperror.Errorf(apperror.CodeValidation, "component not found: %s", componentID)
	}
	if sourceRel := editableComponentSource(component); sourceRel != "" {
		return resolveProjectOwnedFile(loaded.Root, sourceRel)
	}
	parts := strings.Split(component.Class, ".")
	if len(parts) < 3 || parts[0] != "components" {
		return "", apperror.Errorf(apperror.CodeValidation, "component %s class does not map to a project source file: %s", componentID, component.Class)
	}
	modulePath := filepath.Join(parts[:len(parts)-1]...) + ".py"
	return resolveProjectOwnedFile(loaded.Root, modulePath)
}

func editableComponentSource(component model.Component) string {
	source := component.Source
	switch source.Layout {
	case "generated_wrapper":
		return strings.TrimSpace(source.Step)
	case "single_file_class", "":
		if strings.TrimSpace(source.Step) != "" {
			return strings.TrimSpace(source.Step)
		}
	}
	return ""
}

func componentSourceArtifactPath(loaded *project.LoadedProject, component model.Component) (string, error) {
	if component.Source.Layout == "generated_wrapper" {
		sourceDir, err := generatedComponentSourceDir(component.Source)
		if err != nil {
			return "", err
		}
		return resolveProjectOwnedFile(loaded.Root, sourceDir)
	}
	if sourceRel := editableComponentSource(component); sourceRel != "" {
		return resolveProjectOwnedFile(loaded.Root, sourceRel)
	}
	parts := strings.Split(component.Class, ".")
	if len(parts) < 3 || parts[0] != "components" {
		return "", apperror.Errorf(apperror.CodeValidation, "component %s class does not map to a project source file: %s", component.ID, component.Class)
	}
	modulePath := filepath.Join(parts[:len(parts)-1]...) + ".py"
	return resolveProjectOwnedFile(loaded.Root, modulePath)
}

func generatedComponentSourceDir(source model.ComponentSource) (string, error) {
	paths := []string{source.Metadata, source.Init, source.Step, source.Helpers, source.Wrapper}
	sourceDir := ""
	for _, sourcePath := range paths {
		sourcePath = strings.TrimSpace(sourcePath)
		if sourcePath == "" {
			continue
		}
		clean, err := cleanRelativePath(sourcePath)
		if err != nil {
			return "", err
		}
		dir := filepath.ToSlash(filepath.Dir(clean))
		if dir == "." || dir == "" {
			return "", apperror.Errorf(apperror.CodeValidation, "generated_wrapper source files must live in a component directory")
		}
		if sourceDir == "" {
			sourceDir = dir
			continue
		}
		if dir != sourceDir {
			return "", apperror.Errorf(apperror.CodeValidation, "generated_wrapper source files must share one directory")
		}
	}
	if sourceDir == "" {
		return "", apperror.Errorf(apperror.CodeValidation, "generated_wrapper source directory is missing")
	}
	return sourceDir, nil
}

func rebaseComponentSource(source model.ComponentSource, oldDir string, newDir string) model.ComponentSource {
	return model.ComponentSource{
		Layout:   source.Layout,
		Metadata: rebaseComponentSourcePath(source.Metadata, oldDir, newDir),
		Init:     rebaseComponentSourcePath(source.Init, oldDir, newDir),
		Step:     rebaseComponentSourcePath(source.Step, oldDir, newDir),
		Helpers:  rebaseComponentSourcePath(source.Helpers, oldDir, newDir),
		Wrapper:  rebaseComponentSourcePath(source.Wrapper, oldDir, newDir),
	}
}

func rebaseComponentSourcePath(sourcePath string, oldDir string, newDir string) string {
	sourcePath = strings.TrimSpace(filepath.ToSlash(sourcePath))
	if sourcePath == "" {
		return ""
	}
	oldDir = strings.TrimSuffix(strings.TrimSpace(filepath.ToSlash(oldDir)), "/")
	newDir = strings.TrimSuffix(strings.TrimSpace(filepath.ToSlash(newDir)), "/")
	if sourcePath == oldDir {
		return newDir
	}
	prefix := oldDir + "/"
	if strings.HasPrefix(sourcePath, prefix) {
		return newDir + "/" + strings.TrimPrefix(sourcePath, prefix)
	}
	return filepath.ToSlash(filepath.Join(newDir, filepath.Base(sourcePath)))
}

func copyComponentSourceArtifact(loaded *project.LoadedProject, source model.Component, target model.Component) (string, error) {
	sourcePath, err := componentSourceArtifactPath(loaded, source)
	if err != nil {
		return "", err
	}
	targetPath, err := componentSourceArtifactPath(loaded, target)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(targetPath); err == nil {
		rel, _ := filepath.Rel(loaded.Root, targetPath)
		return "", apperror.Errorf(apperror.CodeValidation, "component source already exists: %s", filepath.ToSlash(rel))
	} else if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	if target.Source.Layout == "generated_wrapper" {
		if err := copyProjectTree(sourcePath, targetPath); err != nil {
			return "", err
		}
		return targetPath, nil
	}
	sourceBytes, err := os.ReadFile(sourcePath)
	if err != nil {
		return "", apperror.Wrap(apperror.CodeValidation, err)
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(targetPath, sourceBytes, 0o644); err != nil {
		return "", err
	}
	return targetPath, nil
}

func removeComponentSourceArtifact(path string, layout string) error {
	if layout == "generated_wrapper" {
		return os.RemoveAll(path)
	}
	return os.Remove(path)
}

func sameFilesystemPath(left string, right string) bool {
	leftAbs, leftErr := filepath.Abs(left)
	rightAbs, rightErr := filepath.Abs(right)
	if leftErr == nil {
		left = leftAbs
	}
	if rightErr == nil {
		right = rightAbs
	}
	return filepath.Clean(left) == filepath.Clean(right)
}

func cleanRelativePath(rel string) (string, error) {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return "", apperror.Errorf(apperror.CodeValidation, "relative path is required")
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", apperror.Errorf(apperror.CodeValidation, "relative path escapes project: %s", rel)
	}
	return clean, nil
}

func classNameFromPath(classPath string) string {
	classPath = strings.TrimSpace(classPath)
	if classPath == "" {
		return ""
	}
	parts := strings.Split(classPath, ".")
	return strings.TrimSpace(parts[len(parts)-1])
}

func countLines(content string) int {
	if content == "" {
		return 0
	}
	return strings.Count(content, "\n") + 1
}

func findPythonClassLine(content string, className string) int {
	pattern := regexp.MustCompile(`(?m)^class\s+` + regexp.QuoteMeta(className) + `\b`)
	return regexpLine(content, pattern.FindStringIndex(content))
}

func singleFileClassProblems(componentID string, content string, expectedClass string) []Problem {
	problems := []Problem{}
	if expectedClass == "" {
		problems = append(problems, Problem{Severity: "error", Message: "component class path is invalid", ComponentID: componentID})
	} else if line := findPythonClassLine(content, expectedClass); line == 0 {
		problems = append(problems, Problem{Severity: "error", Message: fmt.Sprintf("expected class is missing: %s", expectedClass), ComponentID: componentID})
	}
	if line, params := findPythonMethodSignature(content, "evaluate"); line == 0 {
		problems = append(problems, Problem{Severity: "error", Message: "evaluate method is missing", ComponentID: componentID})
	} else if !pythonMethodSignatureMatches(params, []string{"self", "inputs", "state", "params", "context"}) {
		problems = append(problems, Problem{
			Severity:    "error",
			Message:     "evaluate signature must be (self, inputs, state, params, context)",
			ComponentID: componentID,
			Line:        line,
		})
	}
	if line, params := findPythonMethodSignature(content, "initialize"); line != 0 && !pythonMethodSignatureMatches(params, []string{"self", "params", "context"}) {
		problems = append(problems, Problem{
			Severity:    "error",
			Message:     "initialize signature must be (self, params, context)",
			ComponentID: componentID,
			Line:        line,
		})
	}
	return problems
}

func generatedWrapperStepProblems(componentID string, content string) []Problem {
	line, params := findPythonFunctionSignature(content, "step")
	if line == 0 {
		return []Problem{{Severity: "error", Message: "step function is missing", ComponentID: componentID}}
	}
	if pythonMethodSignatureMatches(params, []string{"inputs", "state", "params", "context"}) {
		return []Problem{}
	}
	return []Problem{{
		Severity:    "error",
		Message:     "step signature must be (inputs, state, params, context)",
		ComponentID: componentID,
		Line:        line,
	}}
}

func findPythonFunctionSignature(content string, functionName string) (int, []string) {
	pattern := regexp.MustCompile(`(?m)^def\s+` + regexp.QuoteMeta(functionName) + `\s*\(([^)]*)\)`)
	match := pattern.FindStringSubmatchIndex(content)
	if len(match) < 4 {
		return 0, nil
	}
	return regexpLine(content, match[:2]), pythonParameterNames(content[match[2]:match[3]])
}

func findPythonMethodSignature(content string, methodName string) (int, []string) {
	pattern := regexp.MustCompile(`(?m)^\s+def\s+` + regexp.QuoteMeta(methodName) + `\s*\(([^)]*)\)`)
	match := pattern.FindStringSubmatchIndex(content)
	if len(match) < 4 {
		return 0, nil
	}
	return regexpLine(content, match[:2]), pythonParameterNames(content[match[2]:match[3]])
}

func pythonParameterNames(signature string) []string {
	parts := strings.Split(signature, ",")
	names := []string{}
	for _, part := range parts {
		name := strings.TrimSpace(part)
		if name == "" {
			continue
		}
		name = strings.TrimLeft(name, "*")
		if index := strings.Index(name, "="); index >= 0 {
			name = name[:index]
		}
		if index := strings.Index(name, ":"); index >= 0 {
			name = name[:index]
		}
		name = strings.TrimSpace(name)
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}

func pythonMethodSignatureMatches(actual []string, expected []string) bool {
	if len(actual) != len(expected) {
		return false
	}
	for i := range expected {
		if actual[i] != expected[i] {
			return false
		}
	}
	return true
}

func sourceContractReferenceProblems(component model.Component, content string) []Problem {
	problems := []Problem{}
	for _, node := range component.Nodes.Inputs {
		if node.IsRequired() && !sourceReferencesInput(content, node.ID) {
			problems = append(problems, Problem{
				Severity:    "warning",
				Message:     fmt.Sprintf("required input node is not referenced in source: %s", node.ID),
				ComponentID: component.ID,
			})
		}
	}
	for _, node := range component.Nodes.Outputs {
		if !sourceReferencesQuotedName(content, node.ID) {
			problems = append(problems, Problem{
				Severity:    "warning",
				Message:     fmt.Sprintf("output node is not obviously returned by source: %s", node.ID),
				ComponentID: component.ID,
			})
		}
	}
	return problems
}

func sourceReferencesInput(content string, id string) bool {
	doubleQuoted := fmt.Sprintf(`"%s"`, id)
	singleQuoted := fmt.Sprintf(`'%s'`, id)
	return strings.Contains(content, "inputs["+doubleQuoted+"]") ||
		strings.Contains(content, "inputs["+singleQuoted+"]") ||
		strings.Contains(content, "inputs.get("+doubleQuoted) ||
		strings.Contains(content, "inputs.get("+singleQuoted)
}

func sourceReferencesQuotedName(content string, id string) bool {
	return strings.Contains(content, fmt.Sprintf(`"%s"`, id)) || strings.Contains(content, fmt.Sprintf(`'%s'`, id))
}

func regexpLine(content string, match []int) int {
	if len(match) != 2 {
		return 0
	}
	return strings.Count(content[:match[0]], "\n") + 1
}

func pythonSyntaxProblems(ctx context.Context, loaded *project.LoadedProject, componentID string, relativePath string, content string) []Problem {
	pythonExe := resolveStudioPython(loaded.Root, loaded.Project.Environment)
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := platform.CommandContext(checkCtx, pythonExe, "-c", "import sys\ncompile(sys.stdin.read(), sys.argv[1], 'exec')", relativePath)
	cmd.Dir = loaded.Root
	cmd.Stdin = strings.NewReader(content)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if checkCtx.Err() != nil {
			return []Problem{{Severity: "warning", Message: "python syntax check timed out", ComponentID: componentID}}
		}
		stderrText := strings.TrimSpace(stderr.String())
		if stderrText == "" {
			return []Problem{{Severity: "warning", Message: "python syntax check unavailable: " + err.Error(), ComponentID: componentID}}
		}
		return []Problem{syntaxProblemFromStderr(componentID, stderrText)}
	}
	return []Problem{}
}

func pythonLoadProblems(ctx context.Context, loaded *project.LoadedProject, componentID string, relativePath string, expectedClass string, content string) []Problem {
	pythonExe := resolveStudioPython(loaded.Root, loaded.Project.Environment)
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	script := strings.Join([]string{
		"import sys",
		"namespace = {}",
		"source = sys.stdin.read()",
		"exec(compile(source, sys.argv[1], 'exec'), namespace)",
		"cls = namespace.get(sys.argv[2])",
		"if cls is None:",
		"    raise AttributeError('expected class is missing: ' + sys.argv[2])",
		"if not callable(cls):",
		"    raise TypeError('expected class is not callable: ' + sys.argv[2])",
	}, "\n")
	cmd := platform.CommandContext(checkCtx, pythonExe, "-c", script, relativePath, expectedClass)
	cmd.Dir = loaded.Root
	cmd.Stdin = strings.NewReader(content)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if checkCtx.Err() != nil {
			return []Problem{{Severity: "warning", Message: "python load check timed out", ComponentID: componentID}}
		}
		stderrText := strings.TrimSpace(stderr.String())
		if stderrText == "" {
			return []Problem{{Severity: "warning", Message: "python load check unavailable: " + err.Error(), ComponentID: componentID}}
		}
		problem := syntaxProblemFromStderr(componentID, stderrText)
		problem.Message = "source load failed: " + problem.Message
		return []Problem{problem}
	}
	return []Problem{}
}

func pythonUndefinedNameProblems(ctx context.Context, loaded *project.LoadedProject, componentID string, relativePath string, content string) []Problem {
	pythonExe := resolveStudioPython(loaded.Root, loaded.Project.Environment)
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	script := strings.Join([]string{
		"import ast, builtins, json, sys",
		"source = sys.stdin.read()",
		"tree = ast.parse(source, filename=sys.argv[1])",
		"allowed = set(dir(builtins)) | {'self', 'inputs', 'state', 'params', 'context'}",
		"assigned = set()",
		"loads = []",
		"class Visitor(ast.NodeVisitor):",
		"    def visit_FunctionDef(self, node):",
		"        assigned.add(node.name)",
		"        for arg in list(node.args.posonlyargs) + list(node.args.args) + list(node.args.kwonlyargs):",
		"            assigned.add(arg.arg)",
		"        if node.args.vararg:",
		"            assigned.add(node.args.vararg.arg)",
		"        if node.args.kwarg:",
		"            assigned.add(node.args.kwarg.arg)",
		"        self.generic_visit(node)",
		"    def visit_ClassDef(self, node):",
		"        assigned.add(node.name)",
		"        self.generic_visit(node)",
		"    def visit_Import(self, node):",
		"        for alias in node.names:",
		"            assigned.add(alias.asname or alias.name.split('.')[0])",
		"    def visit_ImportFrom(self, node):",
		"        for alias in node.names:",
		"            assigned.add(alias.asname or alias.name)",
		"    def visit_Name(self, node):",
		"        if isinstance(node.ctx, (ast.Store, ast.Param)):",
		"            assigned.add(node.id)",
		"        elif isinstance(node.ctx, ast.Load):",
		"            loads.append((node.id, node.lineno, node.col_offset + 1))",
		"        self.generic_visit(node)",
		"Visitor().visit(tree)",
		"seen = set()",
		"problems = []",
		"for name, line, column in loads:",
		"    if name in assigned or name in allowed or name in seen:",
		"        continue",
		"    seen.add(name)",
		"    problems.append({'name': name, 'line': line, 'column': column})",
		"print(json.dumps(problems))",
	}, "\n")
	cmd := platform.CommandContext(checkCtx, pythonExe, "-c", script, relativePath)
	cmd.Dir = loaded.Root
	cmd.Stdin = strings.NewReader(content)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return []Problem{}
	}
	var reported []struct {
		Name   string `json:"name"`
		Line   int    `json:"line"`
		Column int    `json:"column"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &reported); err != nil {
		return []Problem{}
	}
	problems := []Problem{}
	for _, item := range reported {
		problems = append(problems, Problem{
			Severity:    "warning",
			Message:     fmt.Sprintf("undefined name may fail at runtime: %s", item.Name),
			ComponentID: componentID,
			Line:        item.Line,
			Column:      item.Column,
		})
	}
	return problems
}

func syntaxProblemFromStderr(componentID string, stderrText string) Problem {
	line := 0
	linePattern := regexp.MustCompile(`(?m)File ".*", line ([0-9]+)`)
	if match := linePattern.FindStringSubmatch(stderrText); len(match) == 2 {
		fmt.Sscanf(match[1], "%d", &line)
	}
	lines := strings.Split(stderrText, "\n")
	message := strings.TrimSpace(lines[len(lines)-1])
	if message == "" {
		message = "python syntax error"
	}
	return Problem{Severity: "error", Message: message, ComponentID: componentID, Line: line}
}

func resolveStudioPython(projectRoot string, env model.EnvironmentConfig) string {
	if env.Python == "" {
		env.Python = "python"
	}
	if filepath.IsAbs(env.Python) {
		return env.Python
	}
	projectPython := filepath.Join(projectRoot, env.Python)
	if _, err := os.Stat(projectPython); err == nil {
		return projectPython
	}
	if platform.IsDefaultPythonName(env.Python) {
		if packagedPython := platform.FindNearestRuntimePython(projectRoot); packagedPython != "" {
			return packagedPython
		}
	}
	return env.Python
}

func hasErrorProblems(problems []Problem) bool {
	for _, problem := range problems {
		if problem.Severity == "error" {
			return true
		}
	}
	return false
}

func (s *Server) resolveProjectPath(projectPath string) (string, error) {
	if projectPath == "" {
		return "", apperror.Errorf(apperror.CodeValidation, "project_path is required")
	}
	if !filepath.IsAbs(projectPath) {
		projectPath = filepath.Join(s.repoRoot, projectPath)
	}
	absProjectPath, err := filepath.Abs(projectPath)
	if err != nil {
		return "", apperror.Wrap(apperror.CodeValidation, err)
	}
	rel, err := filepath.Rel(s.repoRoot, absProjectPath)
	if err != nil {
		return "", apperror.Wrap(apperror.CodeValidation, err)
	}
	if strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return "", apperror.Errorf(apperror.CodeValidation, "project_path must stay inside repository: %s", projectPath)
	}
	if _, err := os.Stat(absProjectPath); err != nil {
		return "", apperror.Wrap(apperror.CodeValidation, err)
	}
	return absProjectPath, nil
}

func (s *Server) ensureWorkspaceProject(projectRoot string) error {
	workspaceRoot, err := filepath.Abs(filepath.Join(s.repoRoot, "projects"))
	if err != nil {
		return apperror.Wrap(apperror.CodeValidation, err)
	}
	absProjectRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return apperror.Wrap(apperror.CodeValidation, err)
	}
	rel, err := filepath.Rel(workspaceRoot, absProjectRoot)
	if err != nil {
		return apperror.Wrap(apperror.CodeValidation, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return apperror.Errorf(apperror.CodeValidation, "only workspace projects under projects/ can be edited")
	}
	return nil
}

func decodeCreateProjectRequest(r *http.Request) (createProjectRequest, error) {
	defer r.Body.Close()
	var req createProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeCopyProjectRequest(r *http.Request) (copyProjectRequest, error) {
	defer r.Body.Close()
	var req copyProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeCreateComponentRequest(r *http.Request) (createComponentRequest, error) {
	defer r.Body.Close()
	var req createComponentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeDuplicateComponentRequest(r *http.Request) (duplicateComponentRequest, error) {
	defer r.Body.Close()
	var req duplicateComponentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeReplaceComponentRequest(r *http.Request) (replaceComponentRequest, error) {
	defer r.Body.Close()
	var req replaceComponentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeUpdateComponentRequest(r *http.Request) (updateComponentRequest, error) {
	defer r.Body.Close()
	var req updateComponentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeUpdateComponentMLAssetsRequest(r *http.Request) (updateComponentMLAssetsRequest, error) {
	defer r.Body.Close()
	var req updateComponentMLAssetsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeApplyComponentMLSchemaNodesRequest(r *http.Request) (applyComponentMLSchemaNodesRequest, error) {
	defer r.Body.Close()
	var req applyComponentMLSchemaNodesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeDeleteComponentRequest(r *http.Request) (deleteComponentRequest, error) {
	defer r.Body.Close()
	var req deleteComponentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeIncludeComponentRequest(r *http.Request) (includeComponentRequest, error) {
	defer r.Body.Close()
	var req includeComponentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeCreateNodeRequest(r *http.Request) (createNodeRequest, error) {
	defer r.Body.Close()
	var req createNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeUpdateNodeRequest(r *http.Request) (updateNodeRequest, error) {
	defer r.Body.Close()
	var req updateNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeDeleteNodeRequest(r *http.Request) (deleteNodeRequest, error) {
	defer r.Body.Close()
	var req deleteNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeCreateConnectionRequest(r *http.Request) (createConnectionRequest, error) {
	defer r.Body.Close()
	var req createConnectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeUpdateConnectionRequest(r *http.Request) (updateConnectionRequest, error) {
	defer r.Body.Close()
	var req updateConnectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeDeleteConnectionRequest(r *http.Request) (deleteConnectionRequest, error) {
	defer r.Body.Close()
	var req deleteConnectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeUpdateLayoutRequest(r *http.Request) (updateLayoutRequest, error) {
	defer r.Body.Close()
	var req updateLayoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeExportRequest(r *http.Request) (exportRequest, error) {
	defer r.Body.Close()
	var req exportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeSourceRequest(r *http.Request) (sourceRequest, error) {
	defer r.Body.Close()
	var req sourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeSourceCheckRequest(r *http.Request) (sourceCheckRequest, error) {
	defer r.Body.Close()
	var req sourceCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeCreateScenarioRequest(r *http.Request) (createScenarioRequest, error) {
	defer r.Body.Close()
	var req createScenarioRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeUpdateParametersRequest(r *http.Request) (updateParametersRequest, error) {
	defer r.Body.Close()
	var req updateParametersRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeUpdateComponentContractRequest(r *http.Request) (updateComponentContractRequest, error) {
	defer r.Body.Close()
	var req updateComponentContractRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeApplyParameterSetRequest(r *http.Request) (applyParameterSetRequest, error) {
	defer r.Body.Close()
	var req applyParameterSetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeDeleteParameterRequest(r *http.Request) (deleteParameterRequest, error) {
	defer r.Body.Close()
	var req deleteParameterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeImportDatasetRequest(r *http.Request) (importDatasetRequest, error) {
	defer r.Body.Close()
	var req importDatasetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeCreateValidationMappingRequest(r *http.Request) (createValidationMappingRequest, error) {
	defer r.Body.Close()
	var req createValidationMappingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeUpdateValidationMappingRequest(r *http.Request) (updateValidationMappingRequest, error) {
	defer r.Body.Close()
	var req updateValidationMappingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeCopyValidationMappingRequest(r *http.Request) (copyValidationMappingRequest, error) {
	defer r.Body.Close()
	var req copyValidationMappingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeDeleteValidationMappingRequest(r *http.Request) (deleteValidationMappingRequest, error) {
	defer r.Body.Close()
	var req deleteValidationMappingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeUpdateInputRequest(r *http.Request) (updateInputRequest, error) {
	defer r.Body.Close()
	var req updateInputRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeRequest(r *http.Request) (apiRequest, error) {
	defer r.Body.Close()
	var req apiRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeSeriesRunRequest(r *http.Request) (seriesRunRequest, error) {
	defer r.Body.Close()
	var req seriesRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func requestTimeout(req apiRequest, fallback time.Duration) (time.Duration, error) {
	if req.TimeoutMS == 0 {
		return fallback, nil
	}
	if req.TimeoutMS < 100 {
		return 0, apperror.Errorf(apperror.CodeInput, "timeout_ms must be at least 100")
	}
	timeout := time.Duration(req.TimeoutMS) * time.Millisecond
	if timeout > 30*time.Minute {
		return 0, apperror.Errorf(apperror.CodeInput, "timeout_ms must be at most 1800000")
	}
	return timeout, nil
}

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

type sourceLocation struct {
	ComponentID string
	Source      string
	Line        int
}

type tracebackFrame struct {
	Path string
	Line int
}

func tracebackSourceLocation(loaded *project.LoadedProject, message string, preferredComponentID string) (sourceLocation, bool) {
	frames := tracebackFrames(message)
	if len(frames) == 0 {
		return sourceLocation{}, false
	}
	paths := componentEditableSourcePaths(loaded)
	for index := len(frames) - 1; index >= 0; index-- {
		frame := frames[index]
		if frame.Line <= 0 {
			continue
		}
		for _, candidate := range paths {
			if preferredComponentID != "" && candidate.ComponentID != preferredComponentID {
				continue
			}
			if sameTracebackPath(frame.Path, candidate.AbsPath) {
				return sourceLocation{
					ComponentID: candidate.ComponentID,
					Source:      candidate.Source,
					Line:        frame.Line,
				}, true
			}
		}
	}
	for index := len(frames) - 1; index >= 0; index-- {
		frame := frames[index]
		if frame.Line <= 0 {
			continue
		}
		for _, candidate := range paths {
			if sameTracebackPath(frame.Path, candidate.AbsPath) {
				return sourceLocation{
					ComponentID: candidate.ComponentID,
					Source:      candidate.Source,
					Line:        frame.Line,
				}, true
			}
		}
	}
	return tracebackProjectSourceLocation(loaded, frames, preferredComponentID)
}

func tracebackFrames(message string) []tracebackFrame {
	pattern := regexp.MustCompile(`(?m)^\s*File "([^"]+)", line ([0-9]+)`)
	matches := pattern.FindAllStringSubmatch(message, -1)
	frames := make([]tracebackFrame, 0, len(matches))
	for _, match := range matches {
		if len(match) != 3 {
			continue
		}
		line, err := strconv.Atoi(match[2])
		if err != nil {
			continue
		}
		frames = append(frames, tracebackFrame{Path: filepath.Clean(match[1]), Line: line})
	}
	return frames
}

type componentSourcePathCandidate struct {
	ComponentID string
	Source      string
	AbsPath     string
}

func componentEditableSourcePaths(loaded *project.LoadedProject) []componentSourcePathCandidate {
	paths := []componentSourcePathCandidate{}
	for _, component := range loaded.Graph.Components {
		sourcePath, err := componentSourcePath(loaded, component.ID)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(loaded.Root, sourcePath)
		if err != nil {
			rel = sourcePath
		}
		paths = append(paths, componentSourcePathCandidate{
			ComponentID: component.ID,
			Source:      filepath.ToSlash(rel),
			AbsPath:     filepath.Clean(sourcePath),
		})
	}
	return paths
}

func tracebackProjectSourceLocation(loaded *project.LoadedProject, frames []tracebackFrame, preferredComponentID string) (sourceLocation, bool) {
	absRoot, err := filepath.Abs(loaded.Root)
	if err != nil {
		return sourceLocation{}, false
	}
	absRoot = canonicalExistingPath(absRoot)

	for index := len(frames) - 1; index >= 0; index-- {
		frame := frames[index]
		if frame.Line <= 0 {
			continue
		}
		framePath := filepath.Clean(filepath.FromSlash(strings.TrimSpace(frame.Path)))
		if framePath == "" {
			continue
		}
		if !filepath.IsAbs(framePath) {
			framePath = filepath.Join(absRoot, framePath)
		}
		frameAbs, err := filepath.Abs(framePath)
		if err != nil {
			continue
		}
		frameAbs = canonicalExistingPath(frameAbs)
		rel, err := filepath.Rel(absRoot, frameAbs)
		if err != nil {
			continue
		}
		if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
			continue
		}
		return sourceLocation{
			ComponentID: preferredComponentID,
			Source:      filepath.ToSlash(rel),
			Line:        frame.Line,
		}, true
	}
	return sourceLocation{}, false
}

func sameTracebackPath(tracebackPath string, sourcePath string) bool {
	tracebackPath = cleanPathForComparison(tracebackPath)
	sourcePath = cleanPathForComparison(sourcePath)
	if filepath.IsAbs(tracebackPath) {
		if tracebackAbs, err := filepath.Abs(tracebackPath); err == nil {
			tracebackPath = tracebackAbs
		}
	}
	if sourceAbs, err := filepath.Abs(sourcePath); err == nil {
		sourcePath = sourceAbs
	}
	if sameExistingFile(tracebackPath, sourcePath) {
		return true
	}
	tracebackPath = canonicalExistingPath(tracebackPath)
	sourcePath = canonicalExistingPath(sourcePath)
	if strings.EqualFold(tracebackPath, sourcePath) {
		return true
	}
	tracebackSlash := filepath.ToSlash(tracebackPath)
	sourceSlash := filepath.ToSlash(sourcePath)
	return strings.HasSuffix(sourceSlash, tracebackSlash)
}

func cleanPathForComparison(path string) string {
	return filepath.Clean(filepath.FromSlash(strings.TrimSpace(path)))
}

func canonicalExistingPath(path string) string {
	if evaluated, err := filepath.EvalSymlinks(path); err == nil {
		path = evaluated
	}
	return filepath.Clean(path)
}

func sameExistingFile(left string, right string) bool {
	leftInfo, leftErr := os.Stat(left)
	rightInfo, rightErr := os.Stat(right)
	if leftErr != nil || rightErr != nil {
		return false
	}
	return os.SameFile(leftInfo, rightInfo)
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

func resolveProjectFile(projectRoot string, path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(projectRoot, path)
}

func resolveProjectOwnedFile(projectRoot string, path string) (string, error) {
	if path == "" {
		return "", apperror.Errorf(apperror.CodeValidation, "project file path is required")
	}
	resolved := resolveProjectFile(projectRoot, path)
	absPath, err := filepath.Abs(resolved)
	if err != nil {
		return "", apperror.Wrap(apperror.CodeValidation, err)
	}
	absRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return "", apperror.Wrap(apperror.CodeValidation, err)
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return "", apperror.Wrap(apperror.CodeValidation, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", apperror.Errorf(apperror.CodeValidation, "project file path must stay inside project root: %s", path)
	}
	return absPath, nil
}

func loadEditableDefaultInput(loaded *project.LoadedProject) (string, runtimecore.RunInput, error) {
	inputPath, err := resolveProjectOwnedFile(loaded.Root, loaded.Project.DefaultInput)
	if err != nil {
		return "", runtimecore.RunInput{}, err
	}
	input, err := runtimecore.LoadInput(inputPath)
	if err != nil {
		return "", runtimecore.RunInput{}, err
	}
	if input.Inputs == nil {
		input.Inputs = map[string]any{}
	}
	if input.Context == nil {
		input.Context = map[string]any{}
	}
	return inputPath, input, nil
}

func writeRunRecord(loaded *project.LoadedProject, input runtimecore.RunInput, result *runtimecore.RunResult, parameterSet string) (RunSummary, error) {
	now := time.Now().UTC()
	runID := "run-" + now.Format("20060102-150405.000000000")
	runsRoot := filepath.Join(loaded.Root, "runs")
	if err := os.MkdirAll(runsRoot, 0o755); err != nil {
		return RunSummary{}, err
	}
	runPath := filepath.Join(runsRoot, runID+".json")
	record := RunRecord{
		ID:           runID,
		ProjectName:  loaded.Project.ProjectName,
		CreatedAtUTC: now.Format(time.RFC3339Nano),
		ParameterSet: parameterSet,
		Inputs:       input.Inputs,
		Context:      input.Context,
		Result:       result,
	}
	if err := writeJSONFile(runPath, record); err != nil {
		return RunSummary{}, err
	}
	rel, _ := filepath.Rel(loaded.Root, runPath)
	return RunSummary{
		ID:           runID,
		RelativePath: filepath.ToSlash(rel),
		CreatedAtUTC: record.CreatedAtUTC,
		ParameterSet: record.ParameterSet,
		Outputs:      result.Outputs,
	}, nil
}

func loadRunSummaries(projectRoot string) []RunSummary {
	runFiles, err := filepath.Glob(filepath.Join(projectRoot, "runs", "run-*.json"))
	if err != nil {
		return []RunSummary{}
	}
	summaries := []RunSummary{}
	for _, runPath := range runFiles {
		runBytes, err := os.ReadFile(runPath)
		if err != nil {
			continue
		}
		var record RunRecord
		if err := json.Unmarshal(runBytes, &record); err != nil {
			continue
		}
		rel, _ := filepath.Rel(projectRoot, runPath)
		outputs := map[string]any{}
		if record.Result != nil {
			outputs = record.Result.Outputs
		}
		summaries = append(summaries, RunSummary{
			ID:           record.ID,
			RelativePath: filepath.ToSlash(rel),
			CreatedAtUTC: record.CreatedAtUTC,
			ParameterSet: record.ParameterSet,
			Outputs:      outputs,
		})
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].CreatedAtUTC > summaries[j].CreatedAtUTC
	})
	return summaries
}

func loadRunRecord(projectRoot string, runID string) (RunRecord, error) {
	if runID == "" {
		return RunRecord{}, apperror.Errorf(apperror.CodeValidation, "run_id is required")
	}
	if filepath.Base(runID) != runID || strings.ContainsAny(runID, `/\`) {
		return RunRecord{}, apperror.Errorf(apperror.CodeValidation, "run_id must be a run record id")
	}
	runPath, err := resolveProjectOwnedFile(projectRoot, filepath.Join("runs", runID+".json"))
	if err != nil {
		return RunRecord{}, err
	}
	runBytes, err := os.ReadFile(runPath)
	if err != nil {
		return RunRecord{}, apperror.Wrap(apperror.CodeValidation, err)
	}
	var record RunRecord
	if err := json.Unmarshal(runBytes, &record); err != nil {
		return RunRecord{}, apperror.Wrap(apperror.CodeValidation, err)
	}
	return record, nil
}

func writeBatchRecord(loaded *project.LoadedProject, cases []BatchCaseRecord, parameterSet string) (BatchSummary, BatchRecord, error) {
	now := time.Now().UTC()
	batchID := "batch-" + now.Format("20060102-150405.000000000")
	runsRoot := filepath.Join(loaded.Root, "runs")
	if err := os.MkdirAll(runsRoot, 0o755); err != nil {
		return BatchSummary{}, BatchRecord{}, err
	}
	batchPath := filepath.Join(runsRoot, batchID+".json")
	record := BatchRecord{
		ID:           batchID,
		ProjectName:  loaded.Project.ProjectName,
		CreatedAtUTC: now.Format(time.RFC3339Nano),
		ParameterSet: parameterSet,
		Cases:        cases,
	}
	if err := writeJSONFile(batchPath, record); err != nil {
		return BatchSummary{}, BatchRecord{}, err
	}
	rel, _ := filepath.Rel(loaded.Root, batchPath)
	return BatchSummary{
		ID:           batchID,
		RelativePath: filepath.ToSlash(rel),
		CreatedAtUTC: record.CreatedAtUTC,
		ParameterSet: record.ParameterSet,
		CaseCount:    len(cases),
		OKCount:      batchOKCount(cases),
	}, record, nil
}

func loadBatchSummaries(projectRoot string) []BatchSummary {
	batchFiles, err := filepath.Glob(filepath.Join(projectRoot, "runs", "batch-*.json"))
	if err != nil {
		return []BatchSummary{}
	}
	summaries := []BatchSummary{}
	for _, batchPath := range batchFiles {
		batchBytes, err := os.ReadFile(batchPath)
		if err != nil {
			continue
		}
		var record BatchRecord
		if err := json.Unmarshal(batchBytes, &record); err != nil {
			continue
		}
		rel, _ := filepath.Rel(projectRoot, batchPath)
		summaries = append(summaries, BatchSummary{
			ID:           record.ID,
			RelativePath: filepath.ToSlash(rel),
			CreatedAtUTC: record.CreatedAtUTC,
			ParameterSet: record.ParameterSet,
			CaseCount:    len(record.Cases),
			OKCount:      batchOKCount(record.Cases),
		})
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].CreatedAtUTC > summaries[j].CreatedAtUTC
	})
	return summaries
}

func loadBatchRecord(projectRoot string, batchID string) (BatchRecord, error) {
	if batchID == "" {
		return BatchRecord{}, apperror.Errorf(apperror.CodeValidation, "batch_id is required")
	}
	if filepath.Base(batchID) != batchID || strings.ContainsAny(batchID, `/\`) {
		return BatchRecord{}, apperror.Errorf(apperror.CodeValidation, "batch_id must be a batch record id")
	}
	batchPath, err := resolveProjectOwnedFile(projectRoot, filepath.Join("runs", batchID+".json"))
	if err != nil {
		return BatchRecord{}, err
	}
	batchBytes, err := os.ReadFile(batchPath)
	if err != nil {
		return BatchRecord{}, apperror.Wrap(apperror.CodeValidation, err)
	}
	var record BatchRecord
	if err := json.Unmarshal(batchBytes, &record); err != nil {
		return BatchRecord{}, apperror.Wrap(apperror.CodeValidation, err)
	}
	return record, nil
}

func batchOKCount(cases []BatchCaseRecord) int {
	count := 0
	for _, item := range cases {
		if item.OK {
			count++
		}
	}
	return count
}

func writeScenarioRecord(loaded *project.LoadedProject, req createScenarioRequest) (ScenarioSummary, ScenarioRecord, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return ScenarioSummary{}, ScenarioRecord{}, apperror.Errorf(apperror.CodeValidation, "scenario name is required")
	}
	scenarioID := slugify(name)
	if scenarioID == "" {
		return ScenarioSummary{}, ScenarioRecord{}, apperror.Errorf(apperror.CodeValidation, "scenario name must contain letters or numbers")
	}
	if req.Inputs == nil {
		return ScenarioSummary{}, ScenarioRecord{}, apperror.Errorf(apperror.CodeValidation, "scenario inputs are required")
	}
	context := req.Context
	if context == nil {
		context = map[string]any{}
	}
	now := time.Now().UTC()
	scenariosRoot := filepath.Join(loaded.Root, "scenarios")
	if err := os.MkdirAll(scenariosRoot, 0o755); err != nil {
		return ScenarioSummary{}, ScenarioRecord{}, err
	}
	scenarioPath := filepath.Join(scenariosRoot, scenarioID+".json")
	if _, err := os.Stat(scenarioPath); err == nil {
		scenarioID = scenarioID + "-" + now.Format("20060102-150405")
		scenarioPath = filepath.Join(scenariosRoot, scenarioID+".json")
	}
	record := ScenarioRecord{
		ID:           scenarioID,
		Name:         name,
		ProjectName:  loaded.Project.ProjectName,
		CreatedAtUTC: now.Format(time.RFC3339Nano),
		Inputs:       req.Inputs,
		Context:      context,
	}
	if err := writeJSONFile(scenarioPath, record); err != nil {
		return ScenarioSummary{}, ScenarioRecord{}, err
	}
	rel, _ := filepath.Rel(loaded.Root, scenarioPath)
	return ScenarioSummary{
		ID:           record.ID,
		Name:         record.Name,
		RelativePath: filepath.ToSlash(rel),
		CreatedAtUTC: record.CreatedAtUTC,
	}, record, nil
}

func loadScenarioSummaries(projectRoot string) []ScenarioSummary {
	scenarioFiles, err := filepath.Glob(filepath.Join(projectRoot, "scenarios", "*.json"))
	if err != nil {
		return []ScenarioSummary{}
	}
	summaries := []ScenarioSummary{}
	for _, scenarioPath := range scenarioFiles {
		scenarioBytes, err := os.ReadFile(scenarioPath)
		if err != nil {
			continue
		}
		var record ScenarioRecord
		if err := json.Unmarshal(scenarioBytes, &record); err != nil {
			continue
		}
		rel, _ := filepath.Rel(projectRoot, scenarioPath)
		summaries = append(summaries, ScenarioSummary{
			ID:           record.ID,
			Name:         record.Name,
			RelativePath: filepath.ToSlash(rel),
			CreatedAtUTC: record.CreatedAtUTC,
		})
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].CreatedAtUTC > summaries[j].CreatedAtUTC
	})
	return summaries
}

func loadScenarioRecord(projectRoot string, scenarioID string) (ScenarioRecord, error) {
	if scenarioID == "" {
		return ScenarioRecord{}, apperror.Errorf(apperror.CodeValidation, "scenario_id is required")
	}
	if filepath.Base(scenarioID) != scenarioID || strings.ContainsAny(scenarioID, `/\`) {
		return ScenarioRecord{}, apperror.Errorf(apperror.CodeValidation, "scenario_id must be a scenario id")
	}
	scenarioPath, err := resolveProjectOwnedFile(projectRoot, filepath.Join("scenarios", scenarioID+".json"))
	if err != nil {
		return ScenarioRecord{}, err
	}
	scenarioBytes, err := os.ReadFile(scenarioPath)
	if err != nil {
		return ScenarioRecord{}, apperror.Wrap(apperror.CodeValidation, err)
	}
	var record ScenarioRecord
	if err := json.Unmarshal(scenarioBytes, &record); err != nil {
		return ScenarioRecord{}, apperror.Wrap(apperror.CodeValidation, err)
	}
	return record, nil
}

func loadDatasetSummaries(projectRoot string) []DatasetSummary {
	files := appendMatchingFiles(filepath.Join(projectRoot, "datasets"), []string{"*.csv", "*.json"})
	summaries := []DatasetSummary{}
	for _, path := range files {
		rel, _ := filepath.Rel(projectRoot, path)
		id := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		rowCount, columnCount := datasetShape(path)
		checksum, _ := datasetChecksum(path)
		summaries = append(summaries, DatasetSummary{
			ID:           id,
			Name:         displayNameFromID(id),
			RelativePath: filepath.ToSlash(rel),
			Format:       strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), "."),
			RowCount:     rowCount,
			ColumnCount:  columnCount,
			SHA256:       checksum,
		})
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].RelativePath < summaries[j].RelativePath
	})
	return summaries
}

func importDataset(loaded *project.LoadedProject, req importDatasetRequest) (DatasetPreview, error) {
	sourcePath := strings.TrimSpace(req.SourcePath)
	if sourcePath == "" {
		return DatasetPreview{}, apperror.Errorf(apperror.CodeValidation, "source_path is required")
	}
	if encoding := strings.TrimSpace(strings.ToLower(req.Encoding)); encoding != "" && encoding != "utf-8" && encoding != "utf8" && encoding != "utf-8-bom" {
		return DatasetPreview{}, apperror.Errorf(apperror.CodeValidation, "unsupported dataset encoding: %s", req.Encoding)
	}
	if !filepath.IsAbs(sourcePath) {
		abs, err := filepath.Abs(sourcePath)
		if err != nil {
			return DatasetPreview{}, apperror.Wrap(apperror.CodeValidation, err)
		}
		sourcePath = abs
	}
	info, err := os.Stat(sourcePath)
	if err != nil {
		return DatasetPreview{}, apperror.Wrap(apperror.CodeInput, fmt.Errorf("dataset source not found: %s", sourcePath))
	}
	if info.IsDir() {
		return DatasetPreview{}, apperror.Errorf(apperror.CodeInput, "dataset source must be a CSV file: %s", sourcePath)
	}
	if strings.ToLower(filepath.Ext(sourcePath)) != ".csv" {
		return DatasetPreview{}, apperror.Errorf(apperror.CodeInput, "dataset import supports CSV files: %s", sourcePath)
	}

	delimiter, err := requestedDatasetDelimiter(req.Delimiter)
	if err != nil {
		return DatasetPreview{}, err
	}
	records, _, err := readCSVRecords(sourcePath, delimiter)
	if err != nil {
		return DatasetPreview{}, err
	}
	if len(records) == 0 {
		return DatasetPreview{}, apperror.Errorf(apperror.CodeInput, "dataset source has no header row: %s", sourcePath)
	}
	if !hasNonEmptyHeader(records[0]) {
		return DatasetPreview{}, apperror.Errorf(apperror.CodeInput, "dataset source header row is empty: %s", sourcePath)
	}

	id := strings.ReplaceAll(slugify(req.ID), "-", "_")
	if id == "" {
		id = strings.ReplaceAll(slugify(req.Name), "-", "_")
	}
	if id == "" {
		id = strings.ReplaceAll(slugify(strings.TrimSuffix(filepath.Base(sourcePath), filepath.Ext(sourcePath))), "-", "_")
	}
	id = uniqueDatasetID(loaded.Root, id)
	targetRel := filepath.Join("datasets", id+".csv")
	targetPath := filepath.Join(loaded.Root, targetRel)
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return DatasetPreview{}, apperror.Wrap(apperror.CodeRuntime, err)
	}
	if err := writeNormalizedCSV(targetPath, records); err != nil {
		return DatasetPreview{}, err
	}
	preview, err := datasetPreview(loaded, targetRel)
	if err != nil {
		return DatasetPreview{}, err
	}
	if strings.TrimSpace(req.Name) != "" {
		preview.Summary.Name = strings.TrimSpace(req.Name)
	}
	return preview, nil
}

func datasetPreview(loaded *project.LoadedProject, relativePath string) (DatasetPreview, error) {
	resolved, err := resolveProjectOwnedFile(loaded.Root, relativePath)
	if err != nil {
		return DatasetPreview{}, err
	}
	rel, _ := filepath.Rel(loaded.Root, resolved)
	rows, columns, err := readDatasetPreviewRows(resolved, 8)
	if err != nil {
		return DatasetPreview{}, err
	}
	rowCount, columnCount := datasetShape(resolved)
	checksum, _ := datasetChecksum(resolved)
	summary := DatasetSummary{
		ID:           strings.TrimSuffix(filepath.Base(resolved), filepath.Ext(resolved)),
		Name:         displayNameFromID(strings.TrimSuffix(filepath.Base(resolved), filepath.Ext(resolved))),
		RelativePath: filepath.ToSlash(rel),
		Format:       strings.TrimPrefix(strings.ToLower(filepath.Ext(resolved)), "."),
		RowCount:     rowCount,
		ColumnCount:  columnCount,
		SHA256:       checksum,
	}
	system := entrySystem(loaded)
	return DatasetPreview{
		Summary:          summary,
		Columns:          columns,
		ColumnProfiles:   inferColumnProfiles(columns, rows),
		PreviewRows:      rows,
		SuggestedInputs:  columnSuggestions(system.PublicInputs, columns),
		SuggestedOutputs: columnSuggestions(system.PublicOutputs, columns),
	}, nil
}

func readDatasetPreviewRows(path string, limit int) ([]map[string]string, []string, error) {
	if strings.ToLower(filepath.Ext(path)) != ".csv" {
		return []map[string]string{}, []string{}, nil
	}
	records, _, err := readCSVRecords(path, 0)
	if err != nil {
		return nil, nil, err
	}
	if len(records) == 0 {
		return []map[string]string{}, []string{}, nil
	}
	columns := append([]string(nil), records[0]...)
	rows := []map[string]string{}
	for _, record := range records[1:] {
		if len(rows) >= limit {
			break
		}
		row := map[string]string{}
		for index, column := range columns {
			if index < len(record) {
				row[column] = record[index]
			} else {
				row[column] = ""
			}
		}
		rows = append(rows, row)
	}
	return rows, columns, nil
}

func readCSVRecords(path string, delimiter rune) ([][]string, rune, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, apperror.Wrap(apperror.CodeInput, err)
	}
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	if delimiter == 0 {
		delimiter = detectCSVDelimiter(string(data))
	}
	if delimiter == 0 {
		delimiter = ','
	}
	reader := csv.NewReader(strings.NewReader(string(data)))
	reader.FieldsPerRecord = -1
	reader.Comma = delimiter
	records, err := reader.ReadAll()
	if err != nil {
		return nil, 0, apperror.Wrap(apperror.CodeInput, err)
	}
	return records, delimiter, nil
}

func requestedDatasetDelimiter(value string) (rune, error) {
	raw := strings.ToLower(value)
	if raw == "\t" {
		return '\t', nil
	}
	value = strings.TrimSpace(raw)
	switch value {
	case "", "auto":
		return 0, nil
	case ",", "comma":
		return ',', nil
	case ";", "semicolon", "semi-colon":
		return ';', nil
	case "\\t", "tab":
		return '\t', nil
	case "|", "pipe":
		return '|', nil
	}
	runes := []rune(value)
	if len(runes) == 1 && runes[0] != '"' && runes[0] != '\r' && runes[0] != '\n' {
		return runes[0], nil
	}
	return 0, apperror.Errorf(apperror.CodeValidation, "unsupported dataset delimiter: %s", value)
}

func detectCSVDelimiter(data string) rune {
	firstLine := ""
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) != "" {
			firstLine = line
			break
		}
	}
	if firstLine == "" {
		return ','
	}
	type candidate struct {
		delimiter rune
		count     int
	}
	candidates := []candidate{
		{delimiter: ',', count: strings.Count(firstLine, ",")},
		{delimiter: ';', count: strings.Count(firstLine, ";")},
		{delimiter: '\t', count: strings.Count(firstLine, "\t")},
		{delimiter: '|', count: strings.Count(firstLine, "|")},
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].count > candidates[j].count
	})
	if candidates[0].count == 0 {
		return ','
	}
	return candidates[0].delimiter
}

func writeNormalizedCSV(path string, records [][]string) error {
	file, err := os.Create(path)
	if err != nil {
		return apperror.Wrap(apperror.CodeRuntime, err)
	}
	defer file.Close()
	writer := csv.NewWriter(file)
	if err := writer.WriteAll(records); err != nil {
		return apperror.Wrap(apperror.CodeRuntime, err)
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return apperror.Wrap(apperror.CodeRuntime, err)
	}
	return nil
}

func hasNonEmptyHeader(columns []string) bool {
	for _, column := range columns {
		if strings.TrimSpace(column) != "" {
			return true
		}
	}
	return false
}

func inferColumnProfiles(columns []string, rows []map[string]string) []ColumnProfile {
	profiles := []ColumnProfile{}
	for _, column := range columns {
		profile := ColumnProfile{Column: column, ValueType: "number"}
		samples := []string{}
		seen := map[string]bool{}
		for _, row := range rows {
			raw := strings.TrimSpace(row[column])
			if raw == "" {
				profile.MissingCount++
				continue
			}
			if _, err := strconv.ParseFloat(raw, 64); err != nil {
				profile.ValueType = "string"
			}
			if !seen[raw] && len(samples) < 3 {
				seen[raw] = true
				samples = append(samples, raw)
			}
		}
		if len(rows) == 0 {
			profile.ValueType = "unknown"
		}
		profile.Samples = samples
		profiles = append(profiles, profile)
	}
	return profiles
}

func datasetChecksum(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum), nil
}

func columnSuggestions(refs []model.PublicNodeRef, columns []string) []ColumnSuggestion {
	suggestions := []ColumnSuggestion{}
	for _, ref := range refs {
		suggestions = append(suggestions, ColumnSuggestion{
			PublicID:  ref.ID,
			Name:      ref.Name,
			Column:    matchColumn(ref, columns),
			Unit:      ref.Unit,
			ValueType: ref.ValueType,
			Required:  ref.IsRequired(),
		})
	}
	return suggestions
}

func matchColumn(ref model.PublicNodeRef, columns []string) string {
	targets := []string{ref.ID, ref.Name}
	for _, target := range targets {
		normalizedTarget := normalizeColumnName(target)
		if normalizedTarget == "" {
			continue
		}
		for _, column := range columns {
			if normalizeColumnName(column) == normalizedTarget {
				return column
			}
		}
	}
	for _, target := range targets {
		normalizedTarget := normalizeColumnName(target)
		if normalizedTarget == "" {
			continue
		}
		for _, column := range columns {
			normalizedColumn := normalizeColumnName(column)
			if strings.HasSuffix(normalizedColumn, normalizedTarget) || strings.Contains(normalizedColumn, normalizedTarget) {
				return column
			}
		}
	}
	return ""
}

func createValidationMapping(loaded *project.LoadedProject, req createValidationMappingRequest) (ValidationMappingSummary, modelvalidation.Mapping, error) {
	if strings.TrimSpace(req.DatasetPath) == "" {
		return ValidationMappingSummary{}, modelvalidation.Mapping{}, apperror.Errorf(apperror.CodeValidation, "dataset_path is required")
	}
	preview, err := datasetPreview(loaded, req.DatasetPath)
	if err != nil {
		return ValidationMappingSummary{}, modelvalidation.Mapping{}, err
	}
	inputColumns := nonEmptyColumns(req.InputColumns)
	if len(inputColumns) == 0 {
		inputColumns = suggestionsToColumns(preview.SuggestedInputs)
	}
	outputColumns := nonEmptyColumns(req.ObservedOutputColumns)
	if len(outputColumns) == 0 {
		outputColumns = suggestionsToColumns(preview.SuggestedOutputs)
	}
	unitHints := datasetUnitHints(req.UnitHints, preview.Columns)
	if len(inputColumns) == 0 {
		return ValidationMappingSummary{}, modelvalidation.Mapping{}, apperror.Errorf(apperror.CodeValidation, "validation mapping requires at least one input column")
	}
	if len(outputColumns) == 0 {
		return ValidationMappingSummary{}, modelvalidation.Mapping{}, apperror.Errorf(apperror.CodeValidation, "validation mapping requires at least one observed output column")
	}

	id := strings.ReplaceAll(slugify(req.ID), "-", "_")
	if id == "" {
		id = uniqueValidationMappingID(loaded.Root, strings.ReplaceAll(slugify(preview.Summary.ID+"_mapping"), "-", "_"))
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = displayNameFromID(id)
	}
	policy := strings.TrimSpace(req.MissingValuePolicy)
	policy, err = modelvalidation.NormalizeMissingValuePolicy(policy)
	if err != nil {
		return ValidationMappingSummary{}, modelvalidation.Mapping{}, err
	}

	mapping := modelvalidation.Mapping{
		ID:                    id,
		Name:                  name,
		Dataset:               preview.Summary.RelativePath,
		DatasetChecksum:       preview.Summary.SHA256,
		TimeColumn:            firstMatchingColumn(preview.Columns, req.TimeColumn, "time", "timestamp"),
		InputColumns:          inputColumns,
		ObservedOutputColumns: outputColumns,
		UnitHints:             unitHints,
		MissingValuePolicy:    policy,
	}
	mappingPath := filepath.Join(loaded.Root, "validation", "mappings", id+".json")
	if _, err := os.Stat(mappingPath); err == nil {
		return ValidationMappingSummary{}, modelvalidation.Mapping{}, apperror.Errorf(apperror.CodeValidation, "validation mapping already exists: %s", id)
	} else if !os.IsNotExist(err) {
		return ValidationMappingSummary{}, modelvalidation.Mapping{}, apperror.Wrap(apperror.CodeRuntime, err)
	}
	if err := os.MkdirAll(filepath.Dir(mappingPath), 0o755); err != nil {
		return ValidationMappingSummary{}, modelvalidation.Mapping{}, apperror.Wrap(apperror.CodeRuntime, err)
	}
	if err := writeJSONFile(mappingPath, mapping); err != nil {
		return ValidationMappingSummary{}, modelvalidation.Mapping{}, apperror.Wrap(apperror.CodeRuntime, err)
	}
	rel, _ := filepath.Rel(loaded.Root, mappingPath)
	summary := ValidationMappingSummary{
		ID:                 mapping.ID,
		Name:               mapping.Name,
		RelativePath:       filepath.ToSlash(rel),
		Dataset:            mapping.Dataset,
		DatasetChecksum:    mapping.DatasetChecksum,
		InputCount:         len(mapping.InputColumns),
		OutputCount:        len(mapping.ObservedOutputColumns),
		MissingValuePolicy: mapping.MissingValuePolicy,
	}
	mapping.Path = summary.RelativePath
	return summary, mapping, nil
}

func updateValidationMapping(loaded *project.LoadedProject, req updateValidationMappingRequest) (ValidationMappingSummary, modelvalidation.Mapping, error) {
	mappingPath, relativePath, err := resolveValidationMappingFile(loaded.Root, req.MappingPath)
	if err != nil {
		return ValidationMappingSummary{}, modelvalidation.Mapping{}, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return ValidationMappingSummary{}, modelvalidation.Mapping{}, apperror.Errorf(apperror.CodeValidation, "validation mapping name is required")
	}
	mapping, err := modelvalidation.LoadMapping(loaded.Root, relativePath)
	if err != nil {
		return ValidationMappingSummary{}, modelvalidation.Mapping{}, err
	}
	mapping.Name = name
	if err := writeJSONFile(mappingPath, mapping); err != nil {
		return ValidationMappingSummary{}, modelvalidation.Mapping{}, apperror.Wrap(apperror.CodeRuntime, err)
	}
	mapping.Path = relativePath
	return validationMappingSummaryFromMapping(mapping), mapping, nil
}

func copyValidationMapping(loaded *project.LoadedProject, req copyValidationMappingRequest) (ValidationMappingSummary, modelvalidation.Mapping, error) {
	_, relativePath, err := resolveValidationMappingFile(loaded.Root, req.MappingPath)
	if err != nil {
		return ValidationMappingSummary{}, modelvalidation.Mapping{}, err
	}
	mapping, err := modelvalidation.LoadMapping(loaded.Root, relativePath)
	if err != nil {
		return ValidationMappingSummary{}, modelvalidation.Mapping{}, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = strings.TrimSpace(mapping.Name)
		if name == "" {
			name = displayNameFromID(mapping.ID)
		}
		name += " Copy"
	}
	id := uniqueValidationMappingID(loaded.Root, strings.ReplaceAll(slugify(name), "-", "_"))
	mapping.ID = id
	mapping.Name = name
	mapping.Path = ""
	mappingPath := filepath.Join(loaded.Root, "validation", "mappings", id+".json")
	if _, err := os.Stat(mappingPath); err == nil {
		return ValidationMappingSummary{}, modelvalidation.Mapping{}, apperror.Errorf(apperror.CodeValidation, "validation mapping already exists: %s", id)
	} else if !os.IsNotExist(err) {
		return ValidationMappingSummary{}, modelvalidation.Mapping{}, apperror.Wrap(apperror.CodeRuntime, err)
	}
	if err := os.MkdirAll(filepath.Dir(mappingPath), 0o755); err != nil {
		return ValidationMappingSummary{}, modelvalidation.Mapping{}, apperror.Wrap(apperror.CodeRuntime, err)
	}
	if err := writeJSONFile(mappingPath, mapping); err != nil {
		return ValidationMappingSummary{}, modelvalidation.Mapping{}, apperror.Wrap(apperror.CodeRuntime, err)
	}
	rel, _ := filepath.Rel(loaded.Root, mappingPath)
	mapping.Path = filepath.ToSlash(rel)
	return validationMappingSummaryFromMapping(mapping), mapping, nil
}

func deleteValidationMapping(loaded *project.LoadedProject, req deleteValidationMappingRequest) (string, error) {
	mappingPath, relativePath, err := resolveValidationMappingFile(loaded.Root, req.MappingPath)
	if err != nil {
		return "", err
	}
	if reference := calibrationSetupReferencingValidationMapping(loaded.Root, relativePath); reference != "" {
		return "", apperror.Errorf(apperror.CodeValidation, "validation mapping is used by calibration setup: %s", reference)
	}
	if err := os.Remove(mappingPath); err != nil {
		return "", apperror.Wrap(apperror.CodeRuntime, err)
	}
	return relativePath, nil
}

func resolveValidationMappingFile(projectRoot string, mappingPath string) (string, string, error) {
	absPath, err := resolveProjectOwnedFile(projectRoot, mappingPath)
	if err != nil {
		return "", "", err
	}
	rel, err := filepath.Rel(projectRoot, absPath)
	if err != nil {
		return "", "", apperror.Wrap(apperror.CodeValidation, err)
	}
	rel = filepath.ToSlash(rel)
	if !strings.HasPrefix(rel, "validation/mappings/") || strings.ToLower(filepath.Ext(rel)) != ".json" {
		return "", "", apperror.Errorf(apperror.CodeValidation, "validation mapping path must be under validation/mappings: %s", mappingPath)
	}
	return absPath, rel, nil
}

func calibrationSetupReferencingValidationMapping(projectRoot string, mappingPath string) string {
	target := filepath.ToSlash(mappingPath)
	for _, summary := range loadCalibrationSetupSummaries(projectRoot) {
		if filepath.ToSlash(summary.Mapping) == target {
			return summary.RelativePath
		}
	}
	return ""
}

func validationMappingSummaryFromMapping(mapping modelvalidation.Mapping) ValidationMappingSummary {
	name := strings.TrimSpace(mapping.Name)
	if name == "" {
		name = displayNameFromID(mapping.ID)
	}
	policy, err := modelvalidation.NormalizeMissingValuePolicy(mapping.MissingValuePolicy)
	if err != nil {
		policy = modelvalidation.MissingPolicyError
	}
	return ValidationMappingSummary{
		ID:                 mapping.ID,
		Name:               name,
		RelativePath:       filepath.ToSlash(mapping.Path),
		Dataset:            filepath.ToSlash(mapping.Dataset),
		DatasetChecksum:    mapping.DatasetChecksum,
		InputCount:         len(mapping.InputColumns),
		OutputCount:        len(mapping.ObservedOutputColumns),
		MissingValuePolicy: policy,
	}
}

func createCalibrationSetup(loaded *project.LoadedProject, req createCalibrationSetupRequest) (CalibrationSetupSummary, calibration.Setup, error) {
	mappingPath := strings.TrimSpace(req.MappingPath)
	if mappingPath == "" {
		mappings := loadValidationMappingSummaries(loaded.Root)
		if len(mappings) == 0 {
			return CalibrationSetupSummary{}, calibration.Setup{}, apperror.Errorf(apperror.CodeValidation, "calibration setup requires a validation mapping")
		}
		mappingPath = mappings[0].RelativePath
	}
	mapping, err := modelvalidation.LoadMapping(loaded.Root, mappingPath)
	if err != nil {
		return CalibrationSetupSummary{}, calibration.Setup{}, err
	}
	if strings.TrimSpace(req.BaseParameterSet) != "" {
		if _, err := resolveProjectOwnedFile(loaded.Root, req.BaseParameterSet); err != nil {
			return CalibrationSetupSummary{}, calibration.Setup{}, err
		}
	}
	objectiveOutputs := req.ObjectiveOutputs
	if len(objectiveOutputs) == 0 {
		objectiveOutputs = map[string]float64{}
		for outputID := range mapping.ObservedOutputColumns {
			objectiveOutputs[outputID] = 1.0
		}
	}
	parameters := req.Parameters
	if len(parameters) == 0 {
		parameters = defaultCalibrationParameters(loaded.Graph)
	}
	if len(parameters) == 0 {
		return CalibrationSetupSummary{}, calibration.Setup{}, apperror.Errorf(apperror.CodeValidation, "calibration setup requires at least one calibration_target parameter with numeric bounds")
	}
	algorithm := strings.TrimSpace(req.Algorithm)
	if algorithm == "" {
		algorithm = "grid"
	}
	if algorithm != "grid" && algorithm != "least_squares" && algorithm != "differential_evolution" {
		return CalibrationSetupSummary{}, calibration.Setup{}, apperror.Errorf(apperror.CodeValidation, "unsupported calibration algorithm: %s", algorithm)
	}
	id := strings.ReplaceAll(slugify(req.ID), "-", "_")
	if id == "" {
		id = uniqueCalibrationSetupID(loaded.Root, strings.ReplaceAll(slugify(mapping.ID+"_calibration"), "-", "_"))
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = displayNameFromID(id)
	}
	setup := calibration.Setup{
		ID:               id,
		Name:             name,
		Algorithm:        algorithm,
		Mapping:          filepath.ToSlash(mapping.Path),
		BaseParameterSet: filepath.ToSlash(strings.TrimSpace(req.BaseParameterSet)),
		Objective: calibration.Objective{
			Metric:  "rmse",
			Outputs: objectiveOutputs,
		},
		Parameters:    parameters,
		StoppingRules: req.StoppingRules,
	}
	setupPath := filepath.Join(loaded.Root, "calibration", "setups", id+".json")
	if _, err := os.Stat(setupPath); err == nil {
		return CalibrationSetupSummary{}, calibration.Setup{}, apperror.Errorf(apperror.CodeValidation, "calibration setup already exists: %s", id)
	} else if !os.IsNotExist(err) {
		return CalibrationSetupSummary{}, calibration.Setup{}, apperror.Wrap(apperror.CodeRuntime, err)
	}
	if err := writeJSONFile(setupPath, setup); err != nil {
		return CalibrationSetupSummary{}, calibration.Setup{}, apperror.Wrap(apperror.CodeRuntime, err)
	}
	rel, _ := filepath.Rel(loaded.Root, setupPath)
	setup.Path = filepath.ToSlash(rel)
	summary := CalibrationSetupSummary{
		ID:             setup.ID,
		Name:           setup.Name,
		RelativePath:   setup.Path,
		Algorithm:      setup.Algorithm,
		Mapping:        filepath.ToSlash(setup.Mapping),
		ParameterCount: len(setup.Parameters),
	}
	return summary, setup, nil
}

func createOptimizationSetup(loaded *project.LoadedProject, req createOptimizationSetupRequest) (OptimizationSetupSummary, optimization.Setup, error) {
	baseInputs := req.BaseInputs
	if baseInputs == nil {
		baseInputs = map[string]any{}
	}
	contextValues := req.Context
	if contextValues == nil {
		contextValues = map[string]any{"time": 0.0, "dt": 60.0}
	}
	objective := req.Objective
	if strings.TrimSpace(objective.Output) == "" {
		outputID := defaultOptimizationObjectiveOutput(loaded.Graph, loaded.Project.EntrySystem)
		if outputID == "" {
			return OptimizationSetupSummary{}, optimization.Setup{}, apperror.Errorf(apperror.CodeValidation, "optimization setup requires a numeric public output objective")
		}
		objective = optimization.Objective{Output: outputID, Sense: "min"}
	}
	if objective.Sense == "" {
		objective.Sense = "min"
	}
	algorithm := strings.TrimSpace(req.Algorithm)
	if algorithm == "" {
		algorithm = "grid"
	}
	if algorithm != "grid" && algorithm != "differential_evolution" {
		return OptimizationSetupSummary{}, optimization.Setup{}, apperror.Errorf(apperror.CodeValidation, "unsupported optimization algorithm: %s", algorithm)
	}
	baseParameterSet := strings.TrimSpace(req.BaseParameterSet)
	if baseParameterSet != "" {
		if _, err := resolveProjectOwnedFile(loaded.Root, baseParameterSet); err != nil {
			return OptimizationSetupSummary{}, optimization.Setup{}, err
		}
	}
	variables := req.DecisionVariables
	if len(variables) == 0 {
		variable, ok := defaultOptimizationDecisionVariable(loaded.Graph, loaded.Project.EntrySystem, baseInputs)
		if !ok {
			return OptimizationSetupSummary{}, optimization.Setup{}, apperror.Errorf(apperror.CodeValidation, "optimization setup requires a numeric public input or optimization_variable parameter")
		}
		variables = []optimization.DecisionVariable{variable}
	}
	id := strings.ReplaceAll(slugify(req.ID), "-", "_")
	if id == "" {
		id = uniqueOptimizationSetupID(loaded.Root, strings.ReplaceAll(slugify(objective.Output+"_optimization"), "-", "_"))
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = displayNameFromID(id)
	}
	setup := optimization.Setup{
		ID:                id,
		Name:              name,
		Algorithm:         algorithm,
		BaseInputs:        baseInputs,
		Context:           contextValues,
		BaseParameterSet:  filepath.ToSlash(baseParameterSet),
		Objective:         objective,
		DecisionVariables: variables,
		Constraints:       req.Constraints,
	}
	setupPath := filepath.Join(loaded.Root, "optimization", "setups", id+".json")
	if _, err := os.Stat(setupPath); err == nil {
		return OptimizationSetupSummary{}, optimization.Setup{}, apperror.Errorf(apperror.CodeValidation, "optimization setup already exists: %s", id)
	} else if !os.IsNotExist(err) {
		return OptimizationSetupSummary{}, optimization.Setup{}, apperror.Wrap(apperror.CodeRuntime, err)
	}
	if err := writeJSONFile(setupPath, setup); err != nil {
		return OptimizationSetupSummary{}, optimization.Setup{}, apperror.Wrap(apperror.CodeRuntime, err)
	}
	rel, _ := filepath.Rel(loaded.Root, setupPath)
	setup.Path = filepath.ToSlash(rel)
	summary := OptimizationSetupSummary{
		ID:               setup.ID,
		Name:             setup.Name,
		RelativePath:     setup.Path,
		Algorithm:        setup.Algorithm,
		BaseParameterSet: filepath.ToSlash(setup.BaseParameterSet),
		Objective:        setup.Objective,
		VariableCount:    len(setup.DecisionVariables),
	}
	return summary, setup, nil
}

func optimizationSetupHasParameterVariables(setup optimization.Setup) bool {
	for _, variable := range setup.DecisionVariables {
		if variable.Kind == "component_parameter" {
			return true
		}
	}
	return false
}

func defaultCalibrationParameters(graph *model.Graph) []calibration.ParameterSpec {
	if graph == nil {
		return nil
	}
	parameters := []calibration.ParameterSpec{}
	for _, component := range graph.Components {
		names := sortedMapKeys(component.ParameterDefinitions)
		for _, name := range names {
			definition := component.ParameterDefinitions[name]
			if definition.Role != "calibration_target" || definition.Bounds == nil {
				continue
			}
			if _, ok := studioNumberValue(component.Parameters[name]); !ok {
				continue
			}
			minValue, minOK := studioNumberValue(definition.Bounds.Min)
			maxValue, maxOK := studioNumberValue(definition.Bounds.Max)
			if !minOK || !maxOK || maxValue < minValue {
				continue
			}
			parameters = append(parameters, calibration.ParameterSpec{
				Component: component.ID,
				Name:      name,
				Min:       minValue,
				Max:       maxValue,
				Step:      defaultGridStep(minValue, maxValue),
			})
		}
	}
	return parameters
}

func defaultOptimizationObjectiveOutput(graph *model.Graph, systemID string) string {
	system, ok := findSystem(graph, systemID)
	if !ok {
		return ""
	}
	for _, output := range system.PublicOutputs {
		if isNumericValueType(output.ValueType) {
			return output.ID
		}
	}
	return ""
}

func defaultOptimizationDecisionVariable(graph *model.Graph, systemID string, baseInputs map[string]any) (optimization.DecisionVariable, bool) {
	system, ok := findSystem(graph, systemID)
	if !ok {
		return optimization.DecisionVariable{}, false
	}
	candidates := []model.PublicNodeRef{}
	for _, input := range system.PublicInputs {
		if !isNumericValueType(input.ValueType) {
			continue
		}
		if _, ok := studioNumberValue(baseInputs[input.ID]); !ok {
			continue
		}
		candidates = append(candidates, input)
	}
	if len(candidates) == 0 {
		return defaultOptimizationParameterDecisionVariable(graph)
	}
	chosen := candidates[0]
	for _, candidate := range candidates {
		id := strings.ToLower(candidate.ID)
		if strings.Contains(id, "setpoint") || strings.Contains(id, "speed") || strings.Contains(id, "fraction") {
			chosen = candidate
			break
		}
	}
	value, _ := studioNumberValue(baseInputs[chosen.ID])
	minValue, maxValue := defaultDecisionBounds(value)
	return optimization.DecisionVariable{
		Kind: "public_input",
		Name: chosen.ID,
		Min:  minValue,
		Max:  maxValue,
		Step: defaultGridStep(minValue, maxValue),
	}, true
}

func defaultOptimizationParameterDecisionVariable(graph *model.Graph) (optimization.DecisionVariable, bool) {
	if graph == nil {
		return optimization.DecisionVariable{}, false
	}
	for _, component := range graph.Components {
		names := sortedMapKeys(component.ParameterDefinitions)
		for _, name := range names {
			definition := component.ParameterDefinitions[name]
			if definition.Role != "optimization_variable" {
				continue
			}
			value, ok := studioNumberValue(component.Parameters[name])
			if !ok {
				continue
			}
			minValue, maxValue := defaultDecisionBounds(value)
			if definition.Bounds != nil {
				if minBound, ok := studioNumberValue(definition.Bounds.Min); ok {
					minValue = minBound
				}
				if maxBound, ok := studioNumberValue(definition.Bounds.Max); ok {
					maxValue = maxBound
				}
			}
			return optimization.DecisionVariable{
				Kind:      "component_parameter",
				Component: component.ID,
				Name:      name,
				Min:       minValue,
				Max:       maxValue,
				Step:      defaultGridStep(minValue, maxValue),
			}, true
		}
	}
	return optimization.DecisionVariable{}, false
}

func defaultDecisionBounds(value float64) (float64, float64) {
	if value == 0 {
		return 0, 1
	}
	delta := math.Abs(value) * 0.2
	if delta == 0 {
		delta = 1
	}
	return value - delta, value + delta
}

func defaultGridStep(minValue float64, maxValue float64) float64 {
	step := (maxValue - minValue) / 4.0
	if step <= 0 {
		return 1
	}
	return math.Round(step*1e9) / 1e9
}

func isNumericValueType(valueType string) bool {
	switch strings.ToLower(strings.TrimSpace(valueType)) {
	case "float", "int", "integer", "number":
		return true
	default:
		return false
	}
}

func studioNumberValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		number, err := typed.Float64()
		return number, err == nil
	default:
		return 0, false
	}
}

func sortedMapKeys[V any](values map[string]V) []string {
	keys := []string{}
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func nonEmptyColumns(values map[string]string) map[string]string {
	result := map[string]string{}
	for id, column := range values {
		id = strings.TrimSpace(id)
		column = strings.TrimSpace(column)
		if id != "" && column != "" {
			result[id] = column
		}
	}
	return result
}

func datasetUnitHints(values map[string]string, columns []string) map[string]string {
	allowed := map[string]bool{}
	for _, column := range columns {
		allowed[column] = true
	}
	result := map[string]string{}
	for column, unit := range values {
		column = strings.TrimSpace(column)
		unit = strings.TrimSpace(unit)
		if column != "" && unit != "" && allowed[column] {
			result[column] = unit
		}
	}
	return result
}

func suggestionsToColumns(suggestions []ColumnSuggestion) map[string]string {
	values := map[string]string{}
	for _, suggestion := range suggestions {
		if suggestion.PublicID != "" && suggestion.Column != "" {
			values[suggestion.PublicID] = suggestion.Column
		}
	}
	return values
}

func firstMatchingColumn(columns []string, preferred string, fallbacks ...string) string {
	candidates := append([]string{preferred}, fallbacks...)
	for _, candidate := range candidates {
		normalized := normalizeColumnName(candidate)
		if normalized == "" {
			continue
		}
		for _, column := range columns {
			if normalizeColumnName(column) == normalized {
				return column
			}
		}
	}
	return ""
}

func uniqueValidationMappingID(projectRoot string, base string) string {
	base = strings.Trim(base, "_")
	if base == "" {
		base = "mapping"
	}
	exists := map[string]bool{}
	for _, summary := range loadValidationMappingSummaries(projectRoot) {
		exists[summary.ID] = true
	}
	candidate := base
	for index := 2; exists[candidate]; index++ {
		candidate = fmt.Sprintf("%s_%d", base, index)
	}
	return candidate
}

func uniqueDatasetID(projectRoot string, base string) string {
	base = strings.Trim(base, "_")
	if base == "" {
		base = "dataset"
	}
	exists := map[string]bool{}
	for _, summary := range loadDatasetSummaries(projectRoot) {
		exists[summary.ID] = true
	}
	candidate := base
	for index := 2; exists[candidate]; index++ {
		candidate = fmt.Sprintf("%s_%d", base, index)
	}
	return candidate
}

func uniqueCalibrationSetupID(projectRoot string, base string) string {
	base = strings.Trim(base, "_")
	if base == "" {
		base = "calibration_setup"
	}
	exists := map[string]bool{}
	for _, summary := range loadCalibrationSetupSummaries(projectRoot) {
		exists[summary.ID] = true
	}
	candidate := base
	for index := 2; exists[candidate]; index++ {
		candidate = fmt.Sprintf("%s_%d", base, index)
	}
	return candidate
}

func uniqueOptimizationSetupID(projectRoot string, base string) string {
	base = strings.Trim(base, "_")
	if base == "" {
		base = "optimization_setup"
	}
	exists := map[string]bool{}
	for _, summary := range loadOptimizationSetupSummaries(projectRoot) {
		exists[summary.ID] = true
	}
	candidate := base
	for index := 2; exists[candidate]; index++ {
		candidate = fmt.Sprintf("%s_%d", base, index)
	}
	return candidate
}

func loadParameterSetSummaries(projectRoot string) []ParameterSetSummary {
	files := appendMatchingFiles(filepath.Join(projectRoot, "parameter_sets"), []string{"*.json"})
	summaries := []ParameterSetSummary{}
	for _, path := range files {
		summary, ok := parameterSetSummary(projectRoot, path)
		if ok {
			summaries = append(summaries, summary)
		}
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].RelativePath < summaries[j].RelativePath
	})
	return summaries
}

func loadSeriesInputSummaries(projectRoot string) []SeriesInputSummary {
	files := appendMatchingFiles(filepath.Join(projectRoot, "inputs"), []string{"*.json"})
	summaries := []SeriesInputSummary{}
	for _, path := range files {
		input, err := runtimecore.LoadSeriesInput(path)
		if err != nil {
			continue
		}
		rel, _ := filepath.Rel(projectRoot, path)
		id := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		summaries = append(summaries, SeriesInputSummary{
			ID:              id,
			Name:            displayNameFromID(id),
			RelativePath:    filepath.ToSlash(rel),
			StepCount:       len(input.Steps),
			TimeKey:         seriesInputTimeKey(input),
			BaseContextKeys: sortedMapKeys(input.Context),
			StepContextKeys: seriesStepContextKeys(input.Steps),
		})
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].RelativePath < summaries[j].RelativePath
	})
	return summaries
}

func seriesInputTimeKey(input runtimecore.SeriesInput) string {
	if _, ok := input.Context["time"]; ok {
		return "context.time"
	}
	for _, step := range input.Steps {
		if _, ok := step.Context["time"]; ok {
			return "context.time"
		}
	}
	return "step index"
}

func seriesStepContextKeys(steps []runtimecore.SeriesStep) []string {
	keys := map[string]bool{}
	for _, step := range steps {
		for key := range step.Context {
			keys[key] = true
		}
	}
	return sortedMapKeys(keys)
}

func loadValidationMappingSummaries(projectRoot string) []ValidationMappingSummary {
	files := appendMatchingFiles(filepath.Join(projectRoot, "validation", "mappings"), []string{"*.json"})
	summaries := []ValidationMappingSummary{}
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var mapping modelvalidation.Mapping
		if err := json.Unmarshal(data, &mapping); err != nil {
			continue
		}
		id := mapping.ID
		if id == "" {
			id = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		}
		name := mapping.Name
		if name == "" {
			name = displayNameFromID(id)
		}
		policy, err := modelvalidation.NormalizeMissingValuePolicy(mapping.MissingValuePolicy)
		if err != nil {
			policy = modelvalidation.MissingPolicyError
		}
		rel, _ := filepath.Rel(projectRoot, path)
		summaries = append(summaries, ValidationMappingSummary{
			ID:                 id,
			Name:               name,
			RelativePath:       filepath.ToSlash(rel),
			Dataset:            filepath.ToSlash(mapping.Dataset),
			DatasetChecksum:    mapping.DatasetChecksum,
			InputCount:         len(mapping.InputColumns),
			OutputCount:        len(mapping.ObservedOutputColumns),
			MissingValuePolicy: policy,
		})
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].RelativePath < summaries[j].RelativePath
	})
	return summaries
}

func loadCalibrationSetupSummaries(projectRoot string) []CalibrationSetupSummary {
	files := appendMatchingFiles(filepath.Join(projectRoot, "calibration", "setups"), []string{"*.json"})
	summaries := []CalibrationSetupSummary{}
	for _, path := range files {
		rel, _ := filepath.Rel(projectRoot, path)
		setup, err := calibration.LoadSetup(projectRoot, rel)
		if err != nil {
			continue
		}
		name := setup.Name
		if name == "" {
			name = displayNameFromID(setup.ID)
		}
		summaries = append(summaries, CalibrationSetupSummary{
			ID:             setup.ID,
			Name:           name,
			RelativePath:   filepath.ToSlash(rel),
			Algorithm:      setup.Algorithm,
			Mapping:        filepath.ToSlash(setup.Mapping),
			ParameterCount: len(setup.Parameters),
		})
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].RelativePath < summaries[j].RelativePath
	})
	return summaries
}

func loadOptimizationSetupSummaries(projectRoot string) []OptimizationSetupSummary {
	files := appendMatchingFiles(filepath.Join(projectRoot, "optimization", "setups"), []string{"*.json"})
	summaries := []OptimizationSetupSummary{}
	for _, path := range files {
		rel, _ := filepath.Rel(projectRoot, path)
		setup, err := optimization.LoadSetup(projectRoot, rel)
		if err != nil {
			continue
		}
		name := setup.Name
		if name == "" {
			name = displayNameFromID(setup.ID)
		}
		summaries = append(summaries, OptimizationSetupSummary{
			ID:               setup.ID,
			Name:             name,
			RelativePath:     filepath.ToSlash(rel),
			Algorithm:        setup.Algorithm,
			BaseParameterSet: filepath.ToSlash(setup.BaseParameterSet),
			Objective:        setup.Objective,
			VariableCount:    len(setup.DecisionVariables),
		})
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].RelativePath < summaries[j].RelativePath
	})
	return summaries
}

func appendMatchingFiles(root string, patterns []string) []string {
	files := []string{}
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(root, pattern))
		if err != nil {
			continue
		}
		files = append(files, matches...)
	}
	sort.Strings(files)
	return files
}

func datasetShape(path string) (int, int) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".csv":
		records, _, err := readCSVRecords(path, 0)
		if err != nil || len(records) == 0 {
			return 0, 0
		}
		return len(records) - 1, len(records[0])
	case ".json":
		data, err := os.ReadFile(path)
		if err != nil {
			return 0, 0
		}
		var rows []map[string]any
		if err := json.Unmarshal(data, &rows); err == nil {
			if len(rows) == 0 {
				return 0, 0
			}
			return len(rows), len(rows[0])
		}
		var object map[string]any
		if err := json.Unmarshal(data, &object); err == nil {
			return 1, len(object)
		}
	}
	return 0, 0
}

func parameterSetSummary(projectRoot string, path string) (ParameterSetSummary, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ParameterSetSummary{}, false
	}
	var record struct {
		ID           string                    `json:"id"`
		Name         string                    `json:"name"`
		CreatedAtUTC string                    `json:"created_at_utc"`
		Components   map[string]map[string]any `json:"components"`
		Parameters   map[string]any            `json:"parameters"`
	}
	if err := json.Unmarshal(data, &record); err != nil {
		return ParameterSetSummary{}, false
	}
	id := record.ID
	if id == "" {
		id = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	name := record.Name
	if name == "" {
		name = displayNameFromID(id)
	}
	parameterCount := 0
	for _, params := range record.Components {
		parameterCount += len(params)
	}
	if parameterCount == 0 && record.Parameters != nil {
		parameterCount = len(record.Parameters)
	}
	rel, _ := filepath.Rel(projectRoot, path)
	return ParameterSetSummary{
		ID:             id,
		Name:           name,
		RelativePath:   filepath.ToSlash(rel),
		CreatedAtUTC:   record.CreatedAtUTC,
		ComponentCount: len(record.Components),
		ParameterCount: parameterCount,
	}, true
}

func parameterSetDetail(loaded *project.LoadedProject, relativePath string) (ParameterSetDetail, error) {
	set, err := parameterset.Load(loaded.Root, relativePath)
	if err != nil {
		return ParameterSetDetail{}, err
	}
	resolved, err := resolveProjectOwnedFile(loaded.Root, relativePath)
	if err != nil {
		return ParameterSetDetail{}, err
	}
	summary, ok := parameterSetSummary(loaded.Root, resolved)
	if !ok {
		return ParameterSetDetail{}, apperror.Errorf(apperror.CodeInput, "parameter set could not be summarized: %s", relativePath)
	}
	return ParameterSetDetail{
		Summary:     summary,
		Set:         set,
		Differences: parameterSetDifferences(loaded.Graph, set),
	}, nil
}

func parameterSetDifferences(graph *model.Graph, set parameterset.Set) []ParameterDiff {
	components := map[string]model.Component{}
	for _, component := range graph.Components {
		components[component.ID] = component
	}
	diffs := []ParameterDiff{}
	for componentID, values := range set.Components {
		component, componentExists := components[componentID]
		for name, value := range values {
			baseline, exists := component.Parameters[name]
			diffs = append(diffs, ParameterDiff{
				Component: componentID,
				Parameter: name,
				Baseline:  baseline,
				Value:     value,
				Exists:    componentExists && exists,
			})
		}
	}
	sort.Slice(diffs, func(i, j int) bool {
		if diffs[i].Component != diffs[j].Component {
			return diffs[i].Component < diffs[j].Component
		}
		return diffs[i].Parameter < diffs[j].Parameter
	})
	return diffs
}

func writeExportManifest(loaded *project.LoadedProject, profile string, options exportOptions) (ExportSummary, ExportManifest, error) {
	if profile == "" {
		profile = "runtime_package"
	}
	if profile != "runtime_package" && profile != "research_project" {
		return ExportSummary{}, ExportManifest{}, apperror.Errorf(apperror.CodeValidation, "unsupported export profile: %s", profile)
	}
	plan, err := compiler.Compile(loaded)
	if err != nil {
		return ExportSummary{}, ExportManifest{}, apperror.Wrap(apperror.CodeValidation, err)
	}
	now := time.Now().UTC()
	projectPath, _, err := projectOwnedRelativePath(loaded.Root, loaded.Path)
	if err != nil {
		return ExportSummary{}, ExportManifest{}, err
	}
	graphPath, _, err := projectOwnedRelativePath(loaded.Root, loaded.GraphPath)
	if err != nil {
		return ExportSummary{}, ExportManifest{}, err
	}
	defaultInputPath := ""
	if loaded.Project.DefaultInput != "" {
		defaultInputPath, _, err = projectOwnedRelativePath(loaded.Root, loaded.Project.DefaultInput)
		if err != nil {
			return ExportSummary{}, ExportManifest{}, err
		}
	}
	environmentLockfilePath := ""
	if loaded.Project.Environment.Lockfile != "" {
		environmentLockfilePath, _, err = projectOwnedRelativePath(loaded.Root, loaded.Project.Environment.Lockfile)
		if err != nil {
			return ExportSummary{}, ExportManifest{}, err
		}
	}
	exportRoot := filepath.Join(loaded.Root, "exports", profile)
	if err := resetGeneratedDir(filepath.Join(loaded.Root, "exports"), exportRoot); err != nil {
		return ExportSummary{}, ExportManifest{}, err
	}
	files, err := writeRuntimeExportProject(loaded, filepath.Join(exportRoot, "project"), options)
	if err != nil {
		return ExportSummary{}, ExportManifest{}, err
	}
	supportFiles, err := writeRuntimeExportSupportFiles(loaded.Root, exportRoot, options)
	if err != nil {
		return ExportSummary{}, ExportManifest{}, err
	}
	files = append(files, supportFiles...)
	interfaceSchemaPath := "schema/public-io.json"
	schema, err := schemaexport.Export(loaded)
	if err != nil {
		return ExportSummary{}, ExportManifest{}, apperror.Wrap(apperror.CodeValidation, err)
	}
	if err := schemaexport.Write(filepath.Join(exportRoot, filepath.FromSlash(interfaceSchemaPath)), schema); err != nil {
		return ExportSummary{}, ExportManifest{}, err
	}
	files = append(files, interfaceSchemaPath)
	entrypoints := runtimeExportEntrypoints(files, plan, exportArtifactPath(projectPath), exportArtifactPath(defaultInputPath), exportArtifactPath(environmentLockfilePath), options)
	entrypointFiles, err := writeRuntimeExportEntrypoints(exportRoot, entrypoints)
	if err != nil {
		return ExportSummary{}, ExportManifest{}, err
	}
	files = append(files, entrypointFiles...)
	sort.Strings(files)
	checksums, err := exportFileChecksums(exportRoot, files)
	if err != nil {
		return ExportSummary{}, ExportManifest{}, err
	}
	commands := []string{}
	for _, entrypoint := range entrypoints {
		if strings.HasSuffix(entrypoint.Rel, ".ps1") {
			commands = append(commands, entrypoint.Rel)
		}
	}
	manifest := ExportManifest{
		Profile:             profile,
		CreatedAtUTC:        now.Format(time.RFC3339Nano),
		ProjectName:         loaded.Project.ProjectName,
		ProjectRoot:         "project",
		ProjectPath:         exportArtifactPath(projectPath),
		GraphPath:           exportArtifactPath(graphPath),
		DefaultInput:        exportArtifactPath(defaultInputPath),
		EnvironmentLockfile: exportArtifactPath(environmentLockfilePath),
		InterfaceSchema:     interfaceSchemaPath,
		Runner:              "bin/bcs-runner.exe",
		RuntimePython:       "runtime/python/python.exe",
		Files:               files,
		Components:          append([]string{}, plan.System.Components...),
		ModelAssets:         modelAssetExportPaths(loaded.Graph, options.IncludeMLAssets),
		MLValidationReports: mlValidationSummaries(loaded, options.IncludeMLAssets, true),
		Checksums:           checksums,
		PublicInputs:        append([]model.PublicNodeRef{}, plan.System.PublicInputs...),
		PublicOutputs:       append([]model.PublicNodeRef{}, plan.System.PublicOutputs...),
		ExecutionOrder:      append([]string{}, plan.Order...),
		ParameterSets:       exportFilesWithPrefix(files, "project/parameter_sets/"),
		Datasets:            exportFilesWithPrefix(files, "project/datasets/"),
		ValidationMappings:  exportFilesWithPrefix(files, "project/validation/mappings/"),
		CalibrationSetups:   exportFilesWithPrefix(files, "project/calibration/setups/"),
		OptimizationSetups:  exportFilesWithPrefix(files, "project/optimization/setups/"),
		RunRecords:          exportFilesWithPrefix(files, "project/runs/"),
		BatchRecords:        exportFilesWithPrefix(files, "project/batches/"),
		ValidationRecords:   exportFilesWithPrefix(files, "project/validation/runs/"),
		CalibrationRecords:  exportFilesWithPrefix(files, "project/calibration/results/"),
		OptimizationRecords: exportFilesWithPrefix(files, "project/optimization/results/"),
		Commands:            commands,
		IncludeDatasets:     options.IncludeDatasets,
		IncludeCalibration:  options.IncludeCalibrationSetups,
		IncludeOptimization: options.IncludeOptimizationSetups,
		IncludeMLAssets:     options.IncludeMLAssets,
		IncludeSDKExamples:  options.IncludeSDKExamples,
		IncludeRecords:      options.IncludeRecords,
	}
	exportPath := filepath.Join(exportRoot, "manifest.json")
	if err := os.MkdirAll(filepath.Dir(exportPath), 0o755); err != nil {
		return ExportSummary{}, ExportManifest{}, err
	}
	if err := writeJSONFile(exportPath, manifest); err != nil {
		return ExportSummary{}, ExportManifest{}, err
	}
	rel, _ := filepath.Rel(loaded.Root, exportPath)
	return ExportSummary{
		Profile:      profile,
		RelativePath: filepath.ToSlash(rel),
		CreatedAtUTC: manifest.CreatedAtUTC,
	}, manifest, nil
}

func exportArtifactPath(path string) string {
	if path == "" {
		return ""
	}
	return filepath.ToSlash(filepath.Join("project", path))
}

func modelAssetExportPaths(graph *model.Graph, includeMLAssets bool) []string {
	if graph == nil || !includeMLAssets {
		return nil
	}
	paths := []string{}
	seen := map[string]bool{}
	for _, component := range graph.Components {
		if component.MLMetadata == nil {
			continue
		}
		for _, asset := range component.MLMetadata.AssetPaths() {
			assetPath := strings.TrimSpace(asset.Path)
			if assetPath == "" {
				continue
			}
			exportPath := exportArtifactPath(assetPath)
			if seen[exportPath] {
				continue
			}
			seen[exportPath] = true
			paths = append(paths, exportPath)
		}
	}
	sort.Strings(paths)
	return paths
}

func mlValidationSummaries(loaded *project.LoadedProject, includeMLAssets bool, exportPaths bool) []MLValidationSummary {
	if loaded == nil || loaded.Graph == nil || !includeMLAssets {
		return nil
	}
	summaries := []MLValidationSummary{}
	for _, component := range loaded.Graph.Components {
		if component.MLMetadata == nil || strings.TrimSpace(component.MLMetadata.ValidationReportFile) == "" {
			continue
		}
		reportRel := strings.TrimSpace(component.MLMetadata.ValidationReportFile)
		reportPath, err := resolveProjectOwnedFile(loaded.Root, reportRel)
		if err != nil {
			continue
		}
		reportBytes, err := os.ReadFile(reportPath)
		if err != nil {
			continue
		}
		var report struct {
			Dataset              string                    `json:"dataset"`
			Metrics              map[string]map[string]any `json:"metrics"`
			FeatureSchemaVersion string                    `json:"feature_schema_version"`
			ModelAssetChecksum   string                    `json:"model_asset_checksum"`
			TrainingPeriod       string                    `json:"training_period"`
			ValidationPeriod     string                    `json:"validation_period"`
			TimeResolution       string                    `json:"time_resolution"`
		}
		if err := json.Unmarshal(reportBytes, &report); err != nil {
			continue
		}
		modelChecksum := strings.TrimSpace(report.ModelAssetChecksum)
		if modelChecksum == "" && strings.TrimSpace(component.MLMetadata.ModelFile) != "" {
			if modelPath, err := resolveProjectOwnedFile(loaded.Root, component.MLMetadata.ModelFile); err == nil {
				modelChecksum, _ = datasetChecksum(modelPath)
			}
		}
		path := filepath.ToSlash(reportRel)
		if exportPaths {
			path = exportArtifactPath(path)
		}
		summaries = append(summaries, MLValidationSummary{
			ComponentID:          component.ID,
			ReportPath:           path,
			Dataset:              report.Dataset,
			Metrics:              report.Metrics,
			FeatureSchemaVersion: report.FeatureSchemaVersion,
			ModelAssetChecksum:   modelChecksum,
			TrainingPeriod:       report.TrainingPeriod,
			ValidationPeriod:     report.ValidationPeriod,
			TimeResolution:       report.TimeResolution,
		})
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].ComponentID < summaries[j].ComponentID
	})
	return summaries
}

func mlValidationSummaryMap(values []MLValidationSummary) map[string]MLValidationSummary {
	if len(values) == 0 {
		return nil
	}
	out := map[string]MLValidationSummary{}
	for _, value := range values {
		out[value.ComponentID] = value
	}
	return out
}

func exportFileChecksums(exportRoot string, files []string) (map[string]string, error) {
	checksums := map[string]string{}
	for _, rel := range files {
		path := filepath.Join(exportRoot, filepath.FromSlash(rel))
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(data)
		checksums[rel] = fmt.Sprintf("%x", sum)
	}
	return checksums, nil
}

func writeRuntimeExportProject(loaded *project.LoadedProject, targetRoot string, options exportOptions) ([]string, error) {
	if err := resetGeneratedDir(filepath.Dir(targetRoot), targetRoot); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(targetRoot, 0o755); err != nil {
		return nil, err
	}

	files := []string{}
	seen := map[string]bool{}
	projectPath, _, err := projectOwnedRelativePath(loaded.Root, loaded.Path)
	if err != nil {
		return nil, err
	}
	graphPath, _, err := projectOwnedRelativePath(loaded.Root, loaded.GraphPath)
	if err != nil {
		return nil, err
	}
	for _, rel := range []string{projectPath, graphPath, loaded.Project.DefaultInput, loaded.Project.Environment.Lockfile} {
		if err := copyRuntimeExportFile(loaded.Root, targetRoot, rel, &files, seen); err != nil {
			return nil, err
		}
	}
	for _, rel := range []string{
		"components",
		"inputs",
		"scenarios",
		"parameter_sets",
	} {
		if err := copyRuntimeExportDir(loaded.Root, targetRoot, rel, &files, seen); err != nil {
			return nil, err
		}
	}
	if options.IncludeMLAssets {
		if err := copyRuntimeExportDir(loaded.Root, targetRoot, "assets", &files, seen); err != nil {
			return nil, err
		}
	}
	if options.IncludeDatasets {
		for _, rel := range []string{"datasets", "validation/mappings"} {
			if err := copyRuntimeExportDir(loaded.Root, targetRoot, rel, &files, seen); err != nil {
				return nil, err
			}
		}
	}
	if options.IncludeCalibrationSetups {
		if err := copyRuntimeExportDir(loaded.Root, targetRoot, "calibration/setups", &files, seen); err != nil {
			return nil, err
		}
	}
	if options.IncludeOptimizationSetups {
		if err := copyRuntimeExportDir(loaded.Root, targetRoot, "optimization/setups", &files, seen); err != nil {
			return nil, err
		}
	}
	if options.IncludeRecords {
		for _, rel := range []string{"runs", "batches", "validation/runs", "calibration/results", "optimization/results"} {
			if err := copyRuntimeExportDir(loaded.Root, targetRoot, rel, &files, seen); err != nil {
				return nil, err
			}
		}
	}
	sort.Strings(files)
	return files, nil
}

func writeRuntimeExportSupportFiles(projectRoot string, exportRoot string, options exportOptions) ([]string, error) {
	supportRoot := findRuntimeSupportRoot(projectRoot)
	if supportRoot == "" {
		return []string{}, nil
	}
	files := []string{}
	seen := map[string]bool{}
	for _, rel := range []string{"bin/bcs-runner.exe", "bin/bcs-env.exe", "runtime/manifest.json"} {
		if err := copyExternalExportFile(supportRoot, exportRoot, rel, &files, seen); err != nil {
			return nil, err
		}
	}
	for _, rel := range []string{"schema/serve-request.schema.json", "schema/serve-response.schema.json"} {
		if err := copyExternalExportFile(supportRoot, exportRoot, rel, &files, seen); err != nil {
			return nil, err
		}
	}
	if err := copyExternalExportDir(supportRoot, exportRoot, "runtime/python", &files, seen); err != nil {
		return nil, err
	}
	if options.IncludeSDKExamples {
		if err := copyExternalExportDir(supportRoot, exportRoot, "python/bcs_sdk", &files, seen); err != nil {
			return nil, err
		}
	}
	sort.Strings(files)
	return files, nil
}

type runtimeExportEntrypoint struct {
	Rel     string
	Content string
}

func runtimeExportEntrypoints(files []string, plan *compiler.Plan, projectPath string, defaultInput string, lockfile string, options exportOptions) []runtimeExportEntrypoint {
	mapping := firstProjectRelativeExport(files, "project/validation/mappings/")
	calibrationSetup := firstProjectRelativeExport(files, "project/calibration/setups/")
	optimizationSetup := firstProjectRelativeExport(files, "project/optimization/setups/")
	entrypoints := []runtimeExportEntrypoint{
		{Rel: "check-env.ps1", Content: runtimeExportCheckEnvScript()},
		{Rel: "run-default.ps1", Content: runtimeExportRunScript(projectPath, defaultInput)},
		{Rel: "run-scenario.ps1", Content: runtimeExportScenarioScript(projectPath, defaultInput)},
		{Rel: "serve.ps1", Content: runtimeExportServeScript(projectPath)},
		{Rel: "docs/CLI_Guide.md", Content: runtimeExportCLIGuide(files, plan, projectPath, defaultInput, options.IncludeSDKExamples)},
	}
	if options.IncludeSDKExamples {
		entrypoints = append(entrypoints, runtimeExportEntrypoint{Rel: "sdk-example.py", Content: runtimeExportSDKExample(projectPath, defaultInput)})
	}
	if firstProjectRelativeExport(files, "project/scenarios/") != "" {
		entrypoints = append(entrypoints, runtimeExportEntrypoint{Rel: "run-batch.ps1", Content: runtimeExportBatchScript(projectPath)})
	}
	if mapping != "" {
		entrypoints = append(entrypoints, runtimeExportEntrypoint{Rel: "validate-data.ps1", Content: runtimeExportValidationScript(projectPath, mapping)})
	}
	if calibrationSetup != "" {
		entrypoints = append(entrypoints, runtimeExportEntrypoint{Rel: "calibrate.ps1", Content: runtimeExportCalibrationScript(projectPath, calibrationSetup)})
	}
	if optimizationSetup != "" {
		entrypoints = append(entrypoints, runtimeExportEntrypoint{Rel: "optimize.ps1", Content: runtimeExportOptimizationScript(projectPath, optimizationSetup)})
		if options.IncludeSDKExamples {
			entrypoints = append(entrypoints, runtimeExportEntrypoint{Rel: "optimize-sdk.py", Content: runtimeExportOptimizationSDKExample(projectPath, optimizationSetup)})
		}
	}
	entrypoints = append([]runtimeExportEntrypoint{{Rel: "README.md", Content: runtimeExportReadme(projectPath, defaultInput, lockfile, entrypoints)}}, entrypoints...)
	return entrypoints
}

func writeRuntimeExportEntrypoints(exportRoot string, files []runtimeExportEntrypoint) ([]string, error) {
	written := []string{}
	for _, file := range files {
		path := filepath.Join(exportRoot, filepath.FromSlash(file.Rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(path, []byte(file.Content), 0o644); err != nil {
			return nil, err
		}
		written = append(written, file.Rel)
	}
	return written, nil
}

func runtimeExportRunScript(projectPath string, defaultInput string) string {
	projectLiteral := powerShellSingleQuotedPath(projectPath)
	inputLiteral := powerShellSingleQuotedPath(defaultInput)
	return strings.TrimLeft(fmt.Sprintf(`
param(
  [string]$Output = ""
)

$ErrorActionPreference = 'Stop'
$Root = Split-Path -Parent $MyInvocation.MyCommand.Path
$Runner = Join-Path $Root 'bin\bcs-runner.exe'
if (-not (Test-Path -LiteralPath $Runner)) {
  $Runner = 'bcs-runner.exe'
}
$PythonRoot = Join-Path $Root 'runtime\python'
if (Test-Path -LiteralPath $PythonRoot) {
  $env:PATH = (@($PythonRoot, (Join-Path $Root 'bin'), $env:PATH) | Where-Object { $_ }) -join [IO.Path]::PathSeparator
}
$Project = Join-Path $Root '%s'
$DefaultInput = '%s'
$RunArgs = @('run', '--project', $Project)
if ($DefaultInput) {
  $RunArgs += @('--input', (Join-Path $Root $DefaultInput))
}
if (-not $Output) {
  $Output = Join-Path $Root 'outputs\latest.json'
}
$OutputDir = Split-Path -Parent $Output
if ($OutputDir) {
  New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null
}
& $Runner validate --project $Project
& $Runner @RunArgs --output $Output
Write-Host "wrote $Output"
`, projectLiteral, inputLiteral), "\r\n")
}

func runtimeExportCheckEnvScript() string {
	return strings.TrimLeft(`
param(
  [switch]$Json
)

$ErrorActionPreference = 'Stop'
$Root = Split-Path -Parent $MyInvocation.MyCommand.Path
$EnvTool = Join-Path $Root 'bin\bcs-env.exe'
if (-not (Test-Path -LiteralPath $EnvTool)) {
  $EnvTool = 'bcs-env.exe'
}
$PythonRoot = Join-Path $Root 'runtime\python'
if (Test-Path -LiteralPath $PythonRoot) {
  $env:PATH = (@($PythonRoot, (Join-Path $Root 'bin'), $env:PATH) | Where-Object { $_ }) -join [IO.Path]::PathSeparator
}
$Args = @('check', '--root', $Root)
if ($Json) {
  $Args += '--json'
}
& $EnvTool @Args
`, "\r\n")
}

func runtimeExportScenarioScript(projectPath string, defaultInput string) string {
	inputLiteral := powerShellSingleQuotedPath(defaultInput)
	return strings.TrimLeft(fmt.Sprintf(`
param(
  [string]$Input = '%s',
  [string]$Output = "",
  [string]$ParameterSet = ""
)

%s
if (-not $Input) {
  throw 'Input is required because this project has no default input.'
} elseif (-not [IO.Path]::IsPathRooted($Input)) {
  $Input = Join-Path $Root $Input
}
if (-not $Output) {
  $Output = Join-Path $Root 'outputs\scenario-result.json'
} elseif (-not [IO.Path]::IsPathRooted($Output)) {
  $Output = Join-Path $Root $Output
}
$OutputDir = Split-Path -Parent $Output
if ($OutputDir) {
  New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null
}
$RunArgs = @('run', '--project', $Project, '--input', $Input, '--output', $Output)
if ($ParameterSet) {
  $RunArgs += @('--parameter-set', $ParameterSet)
}
& $Runner validate --project $Project
& $Runner @RunArgs
Write-Host "wrote $Output"
`, inputLiteral, runtimeExportScriptPreamble(projectPath)), "\r\n")
}

func runtimeExportBatchScript(projectPath string) string {
	return strings.TrimLeft(fmt.Sprintf(`
param(
  [string]$ScenarioDir = "",
  [string]$OutputDir = "",
  [string]$ParameterSet = ""
)

%s
if (-not $ScenarioDir) {
  $ScenarioDir = Join-Path $Root 'project\scenarios'
} elseif (-not [IO.Path]::IsPathRooted($ScenarioDir)) {
  $ScenarioDir = Join-Path $Root $ScenarioDir
}
if (-not $OutputDir) {
  $OutputDir = Join-Path $Root 'outputs\batch'
} elseif (-not [IO.Path]::IsPathRooted($OutputDir)) {
  $OutputDir = Join-Path $Root $OutputDir
}
New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null
$RunArgs = @('run', '--project', $Project)
if ($ParameterSet) {
  $RunArgs += @('--parameter-set', $ParameterSet)
}
Get-ChildItem -LiteralPath $ScenarioDir -Filter '*.json' | Sort-Object Name | ForEach-Object {
  $Output = Join-Path $OutputDir ($_.BaseName + '.json')
  & $Runner @RunArgs --input $_.FullName --output $Output
  Write-Host "wrote $Output"
}
`, runtimeExportScriptPreamble(projectPath)), "\r\n")
}

func runtimeExportValidationScript(projectPath string, mapping string) string {
	return strings.TrimLeft(fmt.Sprintf(`
param(
  [string]$Mapping = '%s',
  [string]$Output = "",
  [string]$ParameterSet = "",
  [int]$HighErrorRows = 3,
  [switch]$SaveRecord
)

%s
if (-not $Output) {
  $Output = Join-Path $Root 'outputs\validation-result.json'
} elseif (-not [IO.Path]::IsPathRooted($Output)) {
  $Output = Join-Path $Root $Output
}
$OutputDir = Split-Path -Parent $Output
if ($OutputDir) {
  New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null
}
$WorkflowArgs = @('validate-data', '--project', $Project, '--mapping', $Mapping, '--high-error-rows', [string]$HighErrorRows, '--output', $Output)
if ($ParameterSet) {
  $WorkflowArgs += @('--parameter-set', $ParameterSet)
}
if ($SaveRecord) {
  $WorkflowArgs += '--save-record'
}
& $Runner @WorkflowArgs
Write-Host "wrote $Output"
`, powerShellSingleQuotedPath(mapping), runtimeExportScriptPreamble(projectPath)), "\r\n")
}

func runtimeExportCalibrationScript(projectPath string, setup string) string {
	return strings.TrimLeft(fmt.Sprintf(`
param(
  [string]$Setup = '%s',
  [string]$Output = "",
  [string]$SaveParameterSet = "",
  [switch]$SaveRecord
)

%s
if (-not $Output) {
  $Output = Join-Path $Root 'outputs\calibration-result.json'
} elseif (-not [IO.Path]::IsPathRooted($Output)) {
  $Output = Join-Path $Root $Output
}
$OutputDir = Split-Path -Parent $Output
if ($OutputDir) {
  New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null
}
$WorkflowArgs = @('calibrate', '--project', $Project, '--setup', $Setup, '--output', $Output)
if ($SaveParameterSet) {
  $WorkflowArgs += @('--save-parameter-set', $SaveParameterSet)
}
if ($SaveRecord) {
  $WorkflowArgs += '--save-record'
}
& $Runner @WorkflowArgs
Write-Host "wrote $Output"
`, powerShellSingleQuotedPath(setup), runtimeExportScriptPreamble(projectPath)), "\r\n")
}

func runtimeExportOptimizationScript(projectPath string, setup string) string {
	return strings.TrimLeft(fmt.Sprintf(`
param(
  [string]$Setup = '%s',
  [string]$Output = "",
  [string]$SaveScenario = "",
  [string]$SaveParameterSet = "",
  [switch]$SaveRecord
)

%s
if (-not $Output) {
  $Output = Join-Path $Root 'outputs\optimization-result.json'
} elseif (-not [IO.Path]::IsPathRooted($Output)) {
  $Output = Join-Path $Root $Output
}
$OutputDir = Split-Path -Parent $Output
if ($OutputDir) {
  New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null
}
$WorkflowArgs = @('optimize', '--project', $Project, '--setup', $Setup, '--output', $Output)
if ($SaveScenario) {
  $WorkflowArgs += @('--save-scenario', $SaveScenario)
}
if ($SaveParameterSet) {
  $WorkflowArgs += @('--save-parameter-set', $SaveParameterSet)
}
if ($SaveRecord) {
  $WorkflowArgs += '--save-record'
}
& $Runner @WorkflowArgs
Write-Host "wrote $Output"
`, powerShellSingleQuotedPath(setup), runtimeExportScriptPreamble(projectPath)), "\r\n")
}

func runtimeExportServeScript(projectPath string) string {
	return strings.TrimLeft(fmt.Sprintf(`
param(
  [string]$RequestFile = "",
  [string]$Output = ""
)

%s
$ServeArgs = @('serve', '--project', $Project)
if ($RequestFile) {
  if (-not [IO.Path]::IsPathRooted($RequestFile)) {
    $RequestFile = Join-Path $Root $RequestFile
  }
  if ($Output -and -not [IO.Path]::IsPathRooted($Output)) {
    $Output = Join-Path $Root $Output
  }
  if ($Output) {
    $OutputDir = Split-Path -Parent $Output
    if ($OutputDir) {
      New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null
    }
    Get-Content -LiteralPath $RequestFile | & $Runner @ServeArgs | Tee-Object -FilePath $Output
  } else {
    Get-Content -LiteralPath $RequestFile | & $Runner @ServeArgs
  }
} else {
  & $Runner @ServeArgs
}
`, runtimeExportScriptPreamble(projectPath)), "\r\n")
}

func runtimeExportSDKExample(projectPath string, defaultInput string) string {
	return fmt.Sprintf(`from pathlib import Path
import json
import sys


ROOT = Path(__file__).resolve().parent
SDK_ROOT = ROOT / "python" / "bcs_sdk"
if SDK_ROOT.exists():
    sys.path.insert(0, str(SDK_ROOT))

from bcs_sdk import RunnerClient


RUNNER = ROOT / "bin" / "bcs-runner.exe"
PROJECT = ROOT / %q
INPUT_REL = %q
if not INPUT_REL:
    raise SystemExit("This export has no default input. Pass an input file to bcs-runner directly.")
INPUT = ROOT / INPUT_REL
OUTPUT = ROOT / "outputs" / "sdk-example-output.json"

with INPUT.open("r", encoding="utf-8") as handle:
    payload = json.load(handle)

client = RunnerClient(project=PROJECT, runner=RUNNER, persistent=False)
client.validate_project()
result = client.run_once(
    dict(payload.get("inputs") or {}),
    dict(payload.get("context") or {}),
    output=OUTPUT,
)
print(json.dumps(result["outputs"], indent=2, sort_keys=True))
`, filepath.FromSlash(projectPath), filepath.FromSlash(defaultInput))
}

func runtimeExportOptimizationSDKExample(projectPath string, setup string) string {
	return fmt.Sprintf(`from pathlib import Path
import argparse
import json
import sys


ROOT = Path(__file__).resolve().parent
SDK_ROOT = ROOT / "python" / "bcs_sdk"
if SDK_ROOT.exists():
    sys.path.insert(0, str(SDK_ROOT))

from bcs_sdk import RunnerClient


RUNNER = ROOT / "bin" / "bcs-runner.exe"
PROJECT = ROOT / %q
DEFAULT_SETUP = %q


parser = argparse.ArgumentParser(description="Run an exported HVAC Studio optimization setup through bcs_sdk.")
parser.add_argument("--setup", default=DEFAULT_SETUP, help="Project-relative optimization setup path.")
parser.add_argument("--output", default=str(ROOT / "outputs" / "optimization-sdk-result.json"), help="Output JSON path.")
parser.add_argument("--save-scenario", default="", help="Project-relative scenario path for the optimized public inputs.")
parser.add_argument("--save-parameter-set", default="", help="Project-relative parameter set path for optimized component parameters.")
parser.add_argument("--save-record", action="store_true", help="Save an optimization result record under the exported project.")
args = parser.parse_args()

output = Path(args.output)
if not output.is_absolute():
    output = ROOT / output
output.parent.mkdir(parents=True, exist_ok=True)

client = RunnerClient(project=PROJECT, runner=RUNNER, persistent=False)
client.validate_project()
result = client.run_optimization(
    setup=args.setup,
    save_scenario=args.save_scenario or None,
    save_parameter_set=args.save_parameter_set or None,
    save_record=args.save_record,
    output=output,
)
print(json.dumps({
    "ok": result.get("ok"),
    "best_objective": result.get("best_objective"),
    "saved_scenario": result.get("saved_scenario", ""),
    "saved_parameter_set": result.get("saved_parameter_set", ""),
    "output": str(output),
}, indent=2, sort_keys=True))
`, filepath.FromSlash(projectPath), setup)
}

func runtimeExportScriptPreamble(projectPath string) string {
	projectLiteral := powerShellSingleQuotedPath(projectPath)
	return strings.TrimLeft(fmt.Sprintf(`
$ErrorActionPreference = 'Stop'
$Root = Split-Path -Parent $MyInvocation.MyCommand.Path
$Runner = Join-Path $Root 'bin\bcs-runner.exe'
if (-not (Test-Path -LiteralPath $Runner)) {
  $Runner = 'bcs-runner.exe'
}
$PythonRoot = Join-Path $Root 'runtime\python'
if (Test-Path -LiteralPath $PythonRoot) {
  $env:PATH = (@($PythonRoot, (Join-Path $Root 'bin'), $env:PATH) | Where-Object { $_ }) -join [IO.Path]::PathSeparator
}
$Project = Join-Path $Root '%s'
`, projectLiteral), "\r\n")
}

func runtimeExportReadme(projectPath string, defaultInput string, lockfile string, entrypoints []runtimeExportEntrypoint) string {
	inputLine := ""
	if defaultInput != "" {
		inputLine = fmt.Sprintf("- Default input: `%s`\n", defaultInput)
	}
	lockfileLine := ""
	if lockfile != "" {
		lockfileLine = fmt.Sprintf("- Python lockfile: `%s`\n", lockfile)
	}
	commandLines := []string{}
	pythonExamples := []string{}
	for _, entrypoint := range entrypoints {
		if strings.HasSuffix(entrypoint.Rel, ".ps1") {
			commandLines = append(commandLines, fmt.Sprintf("- `powershell -ExecutionPolicy Bypass -File .\\%s`", entrypoint.Rel))
		}
		if strings.HasSuffix(entrypoint.Rel, ".py") {
			pythonExamples = append(pythonExamples, fmt.Sprintf("`%s`", entrypoint.Rel))
		}
	}
	pythonLine := ""
	if len(pythonExamples) > 0 {
		pythonLine = fmt.Sprintf("- Python SDK examples: %s\n", strings.Join(pythonExamples, ", "))
	}
	return "# Runtime Export\n\n" +
		"This folder contains a runnable Studio runtime export.\n\n" +
		fmt.Sprintf("- Project: `%s`\n", projectPath) +
		inputLine +
		lockfileLine +
		"- Public IO schema: `schema/public-io.json`\n" +
		"- CLI guide: `docs/CLI_Guide.md`\n" +
		pythonLine +
		"- Runner: `bin/bcs-runner.exe`\n\n" +
		"Available Windows commands:\n\n" +
		strings.Join(commandLines, "\n") +
		"\n"
}

func runtimeExportCLIGuide(files []string, plan *compiler.Plan, projectPath string, defaultInput string, includeSDKExamples bool) string {
	inputs := []model.PublicNodeRef{}
	outputs := []model.PublicNodeRef{}
	components := []string{}
	if plan != nil {
		inputs = plan.System.PublicInputs
		outputs = plan.System.PublicOutputs
		components = plan.System.Components
	}
	scenarioInput := strings.ReplaceAll(defaultInput, "/", `\`)
	if scenarioInput == "" {
		scenarioInput = "project\\inputs\\input.json"
	}
	sections := []string{
		"# Runtime CLI Guide",
		"",
		fmt.Sprintf("- Project: `%s`", projectPath),
		fmt.Sprintf("- Default input: `%s`", defaultInput),
		"- Public schema: `schema/public-io.json`",
		"",
		"## Commands",
		"",
		"- `powershell -ExecutionPolicy Bypass -File .\\check-env.ps1 -Json`",
		"- `powershell -ExecutionPolicy Bypass -File .\\run-default.ps1`",
		fmt.Sprintf("- `powershell -ExecutionPolicy Bypass -File .\\run-scenario.ps1 -Input %s`", scenarioInput),
		"- `powershell -ExecutionPolicy Bypass -File .\\serve.ps1 -RequestFile requests.jsonl -Output outputs\\serve-responses.jsonl`",
	}
	if includeSDKExamples {
		sections = append(sections, "- `runtime\\python\\python.exe sdk-example.py`")
	}
	if len(exportFilesWithPrefix(files, "project/scenarios/")) > 0 {
		sections = append(sections, "- `powershell -ExecutionPolicy Bypass -File .\\run-batch.ps1`")
	}
	if len(exportFilesWithPrefix(files, "project/validation/mappings/")) > 0 {
		sections = append(sections, "- `powershell -ExecutionPolicy Bypass -File .\\validate-data.ps1`")
	}
	if len(exportFilesWithPrefix(files, "project/calibration/setups/")) > 0 {
		sections = append(sections, "- `powershell -ExecutionPolicy Bypass -File .\\calibrate.ps1`")
	}
	if len(exportFilesWithPrefix(files, "project/optimization/setups/")) > 0 {
		sections = append(sections, "- `powershell -ExecutionPolicy Bypass -File .\\optimize.ps1`")
		if includeSDKExamples {
			sections = append(sections, "- `runtime\\python\\python.exe optimize-sdk.py`")
		}
	}
	sections = append(sections,
		"",
		"## Public Inputs",
		"",
		runtimeExportPublicNodeTable(inputs),
		"",
		"## Public Outputs",
		"",
		runtimeExportPublicNodeTable(outputs),
		"",
		"## Components",
		"",
		runtimeExportBulletList(components),
		"",
		"## Included Artifacts",
		"",
		"### Parameter Sets",
		"",
		runtimeExportBulletList(exportFilesWithPrefix(files, "project/parameter_sets/")),
		"",
		"### Validation Mappings",
		"",
		runtimeExportBulletList(exportFilesWithPrefix(files, "project/validation/mappings/")),
		"",
		"### Calibration Setups",
		"",
		runtimeExportBulletList(exportFilesWithPrefix(files, "project/calibration/setups/")),
		"",
		"### Optimization Setups",
		"",
		runtimeExportBulletList(exportFilesWithPrefix(files, "project/optimization/setups/")),
		"",
		"## Troubleshooting",
		"",
		"- Run `check-env.ps1 -Json` first and inspect any reported problem.",
		"- Keep input paths relative to the export root unless you intentionally pass an absolute path.",
		"- Runner errors use stable exit codes and structured JSON when called with `--error-format json`.",
		"",
	)
	return strings.Join(sections, "\n")
}

func runtimeExportPublicNodeTable(nodes []model.PublicNodeRef) string {
	if len(nodes) == 0 {
		return "_None._"
	}
	var builder strings.Builder
	builder.WriteString("| ID | Name | Type | Unit | Required |\n")
	builder.WriteString("|---|---|---|---|---|\n")
	for _, node := range nodes {
		required := "no"
		if node.IsRequired() {
			required = "yes"
		}
		builder.WriteString(fmt.Sprintf("| `%s` | %s | `%s` | `%s` | %s |\n",
			node.ID,
			markdownTableCell(node.Name),
			node.ValueType,
			node.Unit,
			required,
		))
	}
	return strings.TrimRight(builder.String(), "\n")
}

func runtimeExportBulletList(values []string) string {
	if len(values) == 0 {
		return "_None._"
	}
	lines := []string{}
	for _, value := range values {
		lines = append(lines, fmt.Sprintf("- `%s`", value))
	}
	return strings.Join(lines, "\n")
}

func markdownTableCell(value string) string {
	value = strings.ReplaceAll(value, "|", `\|`)
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return value
}

func powerShellSingleQuotedPath(path string) string {
	path = strings.ReplaceAll(filepath.ToSlash(path), "/", `\`)
	return strings.ReplaceAll(path, `'`, `''`)
}

func findRuntimeSupportRoot(start string) string {
	absStart, err := filepath.Abs(start)
	if err != nil {
		return ""
	}
	for {
		runner := platform.BinExecutable(absStart, "bcs-runner")
		python := platform.RuntimePythonPath(absStart)
		if _, runnerErr := os.Stat(runner); runnerErr == nil {
			if _, pythonErr := os.Stat(python); pythonErr == nil {
				return absStart
			}
		}
		parent := filepath.Dir(absStart)
		if parent == absStart {
			return ""
		}
		absStart = parent
	}
}

func copyRuntimeExportDir(projectRoot string, targetRoot string, rel string, files *[]string, seen map[string]bool) error {
	sourceRoot, err := resolveProjectOwnedFile(projectRoot, rel)
	if err != nil {
		return err
	}
	info, err := os.Stat(sourceRoot)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return apperror.Errorf(apperror.CodeValidation, "export source is not a directory: %s", rel)
	}
	return filepath.WalkDir(sourceRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		sourceRel, err := filepath.Rel(sourceRoot, path)
		if err != nil {
			return err
		}
		if sourceRel == "." {
			return nil
		}
		if entry.IsDir() && entry.Name() == "__pycache__" {
			return filepath.SkipDir
		}
		if entry.IsDir() {
			return nil
		}
		if strings.HasSuffix(entry.Name(), ".pyc") || strings.HasSuffix(entry.Name(), ".pyo") {
			return nil
		}
		return copyRuntimeExportFile(projectRoot, targetRoot, filepath.Join(rel, sourceRel), files, seen)
	})
}

func copyExternalExportDir(sourceRoot string, targetRoot string, rel string, files *[]string, seen map[string]bool) error {
	sourcePath := filepath.Join(sourceRoot, filepath.FromSlash(rel))
	info, err := os.Stat(sourcePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return apperror.Errorf(apperror.CodeValidation, "export support source is not a directory: %s", rel)
	}
	return filepath.WalkDir(sourcePath, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		sourceRel, err := filepath.Rel(sourcePath, path)
		if err != nil {
			return err
		}
		if sourceRel == "." {
			return nil
		}
		if entry.IsDir() && entry.Name() == "__pycache__" {
			return filepath.SkipDir
		}
		if entry.IsDir() {
			return nil
		}
		if strings.HasSuffix(entry.Name(), ".pyc") || strings.HasSuffix(entry.Name(), ".pyo") {
			return nil
		}
		return copyExternalExportFile(sourceRoot, targetRoot, filepath.Join(rel, sourceRel), files, seen)
	})
}

func copyRuntimeExportFile(projectRoot string, targetRoot string, rel string, files *[]string, seen map[string]bool) error {
	if rel == "" || rel == "." {
		return nil
	}
	ownedRel, sourcePath, err := projectOwnedRelativePath(projectRoot, rel)
	if err != nil {
		return err
	}
	info, err := os.Stat(sourcePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.IsDir() {
		return nil
	}
	artifactPath := exportArtifactPath(ownedRel)
	if seen[artifactPath] {
		return nil
	}
	bytes, err := os.ReadFile(sourcePath)
	if err != nil {
		return err
	}
	targetPath := filepath.Join(targetRoot, ownedRel)
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(targetPath, bytes, info.Mode().Perm()); err != nil {
		return err
	}
	seen[artifactPath] = true
	*files = append(*files, artifactPath)
	return nil
}

func copyExternalExportFile(sourceRoot string, targetRoot string, rel string, files *[]string, seen map[string]bool) error {
	if rel == "" || rel == "." {
		return nil
	}
	artifactPath := filepath.ToSlash(rel)
	if seen[artifactPath] {
		return nil
	}
	sourcePath := filepath.Join(sourceRoot, filepath.FromSlash(rel))
	info, err := os.Stat(sourcePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.IsDir() {
		return nil
	}
	bytes, err := os.ReadFile(sourcePath)
	if err != nil {
		return err
	}
	targetPath := filepath.Join(targetRoot, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(targetPath, bytes, info.Mode().Perm()); err != nil {
		return err
	}
	seen[artifactPath] = true
	*files = append(*files, artifactPath)
	return nil
}

func projectOwnedRelativePath(projectRoot string, path string) (string, string, error) {
	resolved, err := resolveProjectOwnedFile(projectRoot, path)
	if err != nil {
		return "", "", err
	}
	absRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return "", "", apperror.Wrap(apperror.CodeValidation, err)
	}
	rel, err := filepath.Rel(absRoot, resolved)
	if err != nil {
		return "", "", apperror.Wrap(apperror.CodeValidation, err)
	}
	return rel, resolved, nil
}

func resetGeneratedDir(ownerRoot string, targetPath string) error {
	ownerRoot, err := filepath.Abs(ownerRoot)
	if err != nil {
		return err
	}
	targetPath, err = filepath.Abs(targetPath)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(ownerRoot, targetPath)
	if err != nil {
		return err
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return apperror.Errorf(apperror.CodeValidation, "generated export path must stay inside export root: %s", targetPath)
	}
	if err := os.RemoveAll(targetPath); err != nil {
		return err
	}
	return os.MkdirAll(targetPath, 0o755)
}

func loadExportSummaries(projectRoot string) []ExportSummary {
	manifestFiles, err := filepath.Glob(filepath.Join(projectRoot, "exports", "*", "manifest.json"))
	if err != nil {
		return []ExportSummary{}
	}
	summaries := []ExportSummary{}
	for _, manifestPath := range manifestFiles {
		manifestBytes, err := os.ReadFile(manifestPath)
		if err != nil {
			continue
		}
		var manifest ExportManifest
		if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
			continue
		}
		rel, _ := filepath.Rel(projectRoot, manifestPath)
		profile := manifest.Profile
		if profile == "" {
			profile = filepath.Base(filepath.Dir(manifestPath))
		}
		summaries = append(summaries, ExportSummary{
			Profile:      profile,
			RelativePath: filepath.ToSlash(rel),
			CreatedAtUTC: manifest.CreatedAtUTC,
		})
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].CreatedAtUTC > summaries[j].CreatedAtUTC
	})
	return summaries
}

func loadExportManifest(projectRoot string, profile string) (ExportSummary, ExportManifest, error) {
	if profile == "" {
		profile = "runtime_package"
	}
	if filepath.Base(profile) != profile || strings.ContainsAny(profile, `/\`) {
		return ExportSummary{}, ExportManifest{}, apperror.Errorf(apperror.CodeValidation, "profile must be an export profile id")
	}
	manifestPath, err := resolveProjectOwnedFile(projectRoot, filepath.Join("exports", profile, "manifest.json"))
	if err != nil {
		return ExportSummary{}, ExportManifest{}, err
	}
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return ExportSummary{}, ExportManifest{}, apperror.Wrap(apperror.CodeValidation, err)
	}
	var manifest ExportManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return ExportSummary{}, ExportManifest{}, apperror.Wrap(apperror.CodeValidation, err)
	}
	if manifest.Profile == "" {
		manifest.Profile = profile
	}
	rel, _ := filepath.Rel(projectRoot, manifestPath)
	return ExportSummary{
		Profile:      manifest.Profile,
		RelativePath: filepath.ToSlash(rel),
		CreatedAtUTC: manifest.CreatedAtUTC,
	}, manifest, nil
}

func writeJSONFile(path string, value any) error {
	output, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(output, '\n'), 0o644)
}

func copyProjectTree(sourceRoot string, targetRoot string) error {
	sourceRoot, err := filepath.Abs(sourceRoot)
	if err != nil {
		return err
	}
	targetRoot, err = filepath.Abs(targetRoot)
	if err != nil {
		return err
	}
	if _, err := os.Stat(targetRoot); err == nil {
		return apperror.Errorf(apperror.CodeValidation, "target project already exists: %s", targetRoot)
	}
	return filepath.WalkDir(sourceRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(sourceRoot, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if entry.IsDir() && entry.Name() == "__pycache__" {
			return filepath.SkipDir
		}
		if !entry.IsDir() && (strings.HasSuffix(entry.Name(), ".pyc") || strings.HasSuffix(entry.Name(), ".pyo")) {
			return nil
		}
		targetPath := filepath.Join(targetRoot, rel)
		if entry.IsDir() {
			return os.MkdirAll(targetPath, 0o755)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		bytes, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		return os.WriteFile(targetPath, bytes, info.Mode().Perm())
	})
}

func slugify(value string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(value) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash && b.Len() > 0 {
			b.WriteRune('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func uniqueComponentID(graph *model.Graph, base string) string {
	base = strings.Trim(base, "_")
	if base == "" {
		return ""
	}
	exists := map[string]bool{}
	for _, component := range graph.Components {
		exists[component.ID] = true
	}
	candidate := base
	for index := 2; exists[candidate]; index++ {
		candidate = fmt.Sprintf("%s_%d", base, index)
	}
	return candidate
}

func uniqueConnectionID(graph *model.Graph, base string) string {
	base = strings.ReplaceAll(slugify(base), "-", "_")
	if base == "" {
		base = "connection"
	}
	exists := map[string]bool{}
	for _, connection := range graph.Connections {
		exists[connection.ID] = true
	}
	candidate := base
	for index := 2; exists[candidate]; index++ {
		candidate = fmt.Sprintf("%s_%d", base, index)
	}
	return candidate
}

func findComponent(graph *model.Graph, componentID string) (model.Component, bool) {
	if graph == nil {
		return model.Component{}, false
	}
	for _, component := range graph.Components {
		if component.ID == componentID {
			return component, true
		}
	}
	return model.Component{}, false
}

func findSystem(graph *model.Graph, systemID string) (model.System, bool) {
	if graph == nil {
		return model.System{}, false
	}
	for _, system := range graph.Systems {
		if system.ID == systemID {
			return system, true
		}
	}
	return model.System{}, false
}

func findConnection(graph *model.Graph, connectionID string) (model.Connection, bool) {
	for _, connection := range graph.Connections {
		if connection.ID == connectionID {
			return connection, true
		}
	}
	return model.Connection{}, false
}

func findInputNode(component model.Component, nodeID string) (model.Node, bool) {
	for _, node := range component.Nodes.Inputs {
		if node.ID == nodeID {
			return node, true
		}
	}
	return model.Node{}, false
}

func removeNodeFromComponent(component *model.Component, nodeID string) (model.Node, bool, bool) {
	for index, node := range component.Nodes.Inputs {
		if node.ID == nodeID {
			component.Nodes.Inputs = append(component.Nodes.Inputs[:index], component.Nodes.Inputs[index+1:]...)
			return node, true, true
		}
	}
	for index, node := range component.Nodes.Outputs {
		if node.ID == nodeID {
			component.Nodes.Outputs = append(component.Nodes.Outputs[:index], component.Nodes.Outputs[index+1:]...)
			return node, false, true
		}
	}
	return model.Node{}, false, false
}

func componentHasNode(component model.Component, nodeID string) bool {
	for _, node := range component.Nodes.Inputs {
		if node.ID == nodeID {
			return true
		}
	}
	for _, node := range component.Nodes.Outputs {
		if node.ID == nodeID {
			return true
		}
	}
	return false
}

func componentHasInputNode(component model.Component, nodeID string) bool {
	for _, node := range component.Nodes.Inputs {
		if node.ID == nodeID {
			return true
		}
	}
	return false
}

func componentHasOutputNode(component model.Component, nodeID string) bool {
	for _, node := range component.Nodes.Outputs {
		if node.ID == nodeID {
			return true
		}
	}
	return false
}

func isIdentifierLike(value string) bool {
	if value == "" {
		return false
	}
	for index, r := range value {
		if index == 0 {
			if r != '_' && !unicode.IsLetter(r) {
				return false
			}
			continue
		}
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func exportFilesWithPrefix(files []string, prefix string) []string {
	matches := []string{}
	for _, file := range files {
		if strings.HasPrefix(file, prefix) {
			matches = append(matches, file)
		}
	}
	sort.Strings(matches)
	return matches
}

func firstProjectRelativeExport(files []string, prefix string) string {
	matches := exportFilesWithPrefix(files, prefix)
	if len(matches) == 0 {
		return ""
	}
	return strings.TrimPrefix(matches[0], "project/")
}

func removeString(values []string, target string) []string {
	kept := values[:0]
	for _, value := range values {
		if value == target {
			continue
		}
		kept = append(kept, value)
	}
	return kept
}

func endpointMatches(endpoint model.Endpoint, componentID string, nodeID string) bool {
	return endpoint.Component == componentID && endpoint.Node == nodeID
}

func graphReferencesConnection(systems []model.System, connectionID string) bool {
	for _, system := range systems {
		if containsString(system.Connections, connectionID) {
			return true
		}
	}
	return false
}

func systemHasIncomingConnection(system model.System, graph *model.Graph, componentID string, nodeID string) bool {
	for _, connectionID := range system.Connections {
		for _, connection := range graph.Connections {
			if connection.ID == connectionID && connection.To.Component == componentID && connection.To.Node == nodeID {
				return true
			}
		}
	}
	return false
}

func entrySystem(loaded *project.LoadedProject) model.System {
	index := entrySystemIndex(loaded)
	if index < 0 {
		return model.System{}
	}
	return loaded.Graph.Systems[index]
}

func entrySystemIndex(loaded *project.LoadedProject) int {
	if loaded == nil || loaded.Graph == nil || loaded.Project == nil {
		return -1
	}
	for index, system := range loaded.Graph.Systems {
		if system.ID == loaded.Project.EntrySystem {
			return index
		}
	}
	return -1
}

func hasPublicInputFor(system model.System, componentID string, nodeID string) bool {
	for _, input := range system.PublicInputs {
		if input.Component == componentID && input.Node == nodeID {
			return true
		}
	}
	return false
}

func hasPublicOutputFor(system model.System, componentID string, nodeID string) bool {
	for _, output := range system.PublicOutputs {
		if output.Component == componentID && output.Node == nodeID {
			return true
		}
	}
	return false
}

func removePublicOutputsFor(system *model.System, componentID string, nodeID string) {
	kept := system.PublicOutputs[:0]
	for _, output := range system.PublicOutputs {
		if output.Component == componentID && output.Node == nodeID {
			continue
		}
		kept = append(kept, output)
	}
	system.PublicOutputs = kept
}

func removePublicInputsFor(system *model.System, componentID string, nodeID string) []string {
	removed := []string{}
	kept := system.PublicInputs[:0]
	for _, input := range system.PublicInputs {
		if input.Component == componentID && input.Node == nodeID {
			removed = append(removed, input.ID)
			continue
		}
		kept = append(kept, input)
	}
	system.PublicInputs = kept
	return removed
}

func removePublicInputsForComponent(system *model.System, componentID string) []string {
	removed := []string{}
	kept := system.PublicInputs[:0]
	for _, input := range system.PublicInputs {
		if input.Component == componentID {
			removed = append(removed, input.ID)
			continue
		}
		kept = append(kept, input)
	}
	system.PublicInputs = kept
	return removed
}

func removePublicOutputsForComponent(system *model.System, componentID string) {
	kept := system.PublicOutputs[:0]
	for _, output := range system.PublicOutputs {
		if output.Component == componentID {
			continue
		}
		kept = append(kept, output)
	}
	system.PublicOutputs = kept
}

func removeUnreferencedConnections(connections []model.Connection, systems []model.System) []model.Connection {
	kept := connections[:0]
	for _, connection := range connections {
		if graphReferencesConnection(systems, connection.ID) {
			kept = append(kept, connection)
		}
	}
	return kept
}

func uniquePublicNodeID(refs []model.PublicNodeRef, base string) string {
	exists := map[string]bool{}
	for _, ref := range refs {
		exists[ref.ID] = true
	}
	candidate := base
	for index := 2; exists[candidate]; index++ {
		candidate = fmt.Sprintf("%s_%d", base, index)
	}
	return candidate
}

func normalizeColumnName(value string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(value) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func updatePublicNodeRef(ref *model.PublicNodeRef, node model.Node) {
	ref.Name = node.Name
	ref.Medium = node.Medium
	ref.ValueType = node.ValueType
	ref.Unit = node.Unit
	ref.Required = node.Required
	ref.Default = node.Default
}

func defaultValueForNode(node model.Node) any {
	if node.Default != nil {
		return node.Default
	}
	switch node.ValueType {
	case "int", "integer":
		return 0
	case "bool", "boolean":
		return false
	case "string":
		return ""
	default:
		return 0.0
	}
}

func pythonClassName(componentID string) string {
	var b strings.Builder
	capitalizeNext := true
	for _, r := range componentID {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if b.Len() == 0 && unicode.IsDigit(r) {
				b.WriteRune('C')
			}
			if capitalizeNext {
				b.WriteRune(unicode.ToUpper(r))
				capitalizeNext = false
			} else {
				b.WriteRune(r)
			}
			continue
		}
		capitalizeNext = true
	}
	if b.Len() == 0 {
		return "UserComponent"
	}
	b.WriteString("Component")
	return b.String()
}

func listComponentTemplates(repoRoot string) ([]ComponentTemplateSummary, error) {
	componentsRoot := filepath.Join(repoRoot, "templates", "components")
	entries, err := os.ReadDir(componentsRoot)
	if err != nil {
		return nil, apperror.Errorf(apperror.CodeValidation, "component templates directory is missing: templates/components")
	}
	templates := []ComponentTemplateSummary{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		id := entry.Name()
		manifest, _, err := loadComponentTemplate(repoRoot, id)
		if err != nil {
			return nil, err
		}
		displayID := strings.TrimSpace(manifest.ID)
		if displayID == "" {
			displayID = id
		}
		name := strings.TrimSpace(manifest.Name)
		if name == "" {
			name = displayNameFromID(displayID)
		}
		templates = append(templates, ComponentTemplateSummary{
			ID:                   id,
			Name:                 name,
			Kind:                 defaultString(manifest.Kind, "user_python"),
			Category:             defaultString(manifest.Category, "utility"),
			ExecutionMode:        defaultString(manifest.ExecutionMode, "step"),
			SourceLayout:         defaultString(manifest.Source.Layout, "single_file_class"),
			Inputs:               componentTemplateNodes(manifest.Inputs, "inlet"),
			Outputs:              componentTemplateNodes(manifest.Outputs, "outlet"),
			Parameters:           cloneMap(manifest.Parameters),
			ParameterDefinitions: cloneParameterDefinitions(manifest.ParameterDefinitions),
			InputCount:           len(manifest.Inputs),
			OutputCount:          len(manifest.Outputs),
			ParameterCount:       len(manifest.Parameters),
		})
	}
	sort.Slice(templates, func(i, j int) bool {
		if templates[i].ID == "scalar" {
			return true
		}
		if templates[j].ID == "scalar" {
			return false
		}
		return templates[i].ID < templates[j].ID
	})
	return templates, nil
}

func loadComponentTemplate(repoRoot, template string) (componentTemplateManifest, []componentTemplateFile, error) {
	templateRoot := filepath.Join(repoRoot, "templates", "components", template)
	manifestPath := filepath.Join(templateRoot, "manifest.json")
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return componentTemplateManifest{}, nil, apperror.Errorf(apperror.CodeValidation, "component template manifest is missing: templates/components/%s/manifest.json", template)
	}
	var manifest componentTemplateManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return componentTemplateManifest{}, nil, apperror.Wrap(apperror.CodeValidation, err)
	}
	if strings.TrimSpace(manifest.ClassName) == "" {
		return componentTemplateManifest{}, nil, apperror.Errorf(apperror.CodeValidation, "component template %s class_name is required", template)
	}
	files, err := loadComponentTemplateFiles(templateRoot, template, manifest.Source)
	if err != nil {
		return componentTemplateManifest{}, nil, err
	}
	assetFiles, err := loadComponentTemplateAssetFiles(templateRoot, template, manifest.Assets)
	if err != nil {
		return componentTemplateManifest{}, nil, err
	}
	files = append(files, assetFiles...)
	return manifest, files, nil
}

func loadComponentTemplateAssetFiles(templateRoot, template string, assets []string) ([]componentTemplateFile, error) {
	files := []componentTemplateFile{}
	for _, rel := range assets {
		if strings.TrimSpace(rel) == "" {
			continue
		}
		file, err := loadComponentTemplateFile(templateRoot, template, "asset", rel)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, nil
}

func loadComponentTemplateFiles(templateRoot, template string, source componentTemplateSource) ([]componentTemplateFile, error) {
	layout := defaultString(source.Layout, "single_file_class")
	switch layout {
	case "single_file_class":
		if strings.TrimSpace(source.Step) == "" {
			return nil, apperror.Errorf(apperror.CodeValidation, "component template %s source step is required", template)
		}
		file, err := loadComponentTemplateFile(templateRoot, template, "step", source.Step)
		if err != nil {
			return nil, err
		}
		return []componentTemplateFile{file}, nil
	case "generated_wrapper":
		if strings.TrimSpace(source.Step) == "" || strings.TrimSpace(source.Wrapper) == "" {
			return nil, apperror.Errorf(apperror.CodeValidation, "component template %s generated_wrapper source requires step and wrapper", template)
		}
		if strings.TrimSpace(source.Metadata) != "" {
			if _, err := cleanRelativePath(source.Metadata); err != nil {
				return nil, apperror.Errorf(apperror.CodeValidation, "component template %s source path is invalid: %s", template, source.Metadata)
			}
		}
		refs := []struct {
			role string
			rel  string
		}{
			{"init", source.Init},
			{"step", source.Step},
			{"helpers", source.Helpers},
			{"wrapper", source.Wrapper},
		}
		files := []componentTemplateFile{}
		for _, ref := range refs {
			if strings.TrimSpace(ref.rel) == "" {
				continue
			}
			file, err := loadComponentTemplateFile(templateRoot, template, ref.role, ref.rel)
			if err != nil {
				return nil, err
			}
			files = append(files, file)
		}
		return files, nil
	default:
		return nil, apperror.Errorf(apperror.CodeValidation, "component template %s source layout is unsupported: %s", template, layout)
	}
}

func loadComponentTemplateFile(templateRoot, template, role, rel string) (componentTemplateFile, error) {
	cleanSource, err := cleanRelativePath(rel)
	if err != nil {
		return componentTemplateFile{}, apperror.Errorf(apperror.CodeValidation, "component template %s source path is invalid: %s", template, rel)
	}
	sourcePath := filepath.Join(templateRoot, cleanSource)
	sourceBytes, err := os.ReadFile(sourcePath)
	if err != nil {
		return componentTemplateFile{}, apperror.Errorf(apperror.CodeValidation, "component template source is missing: templates/components/%s/%s", template, rel)
	}
	return componentTemplateFile{Role: role, TemplateRel: filepath.ToSlash(cleanSource), Content: string(sourceBytes)}, nil
}

func rewriteTemplateClassName(source, oldClass, newClass string) (string, error) {
	oldDeclaration := "class " + strings.TrimSpace(oldClass) + ":"
	if !strings.Contains(source, oldDeclaration) {
		return "", apperror.Errorf(apperror.CodeValidation, "component template source does not declare %s", oldClass)
	}
	return strings.Replace(source, oldDeclaration, "class "+newClass+":", 1), nil
}

func componentSourceForTemplate(source componentTemplateSource, componentID string) model.ComponentSource {
	layout := defaultString(source.Layout, "single_file_class")
	if layout != "generated_wrapper" {
		return model.ComponentSource{
			Layout: "single_file_class",
			Step:   filepath.ToSlash(filepath.Join("components", componentID+".py")),
		}
	}
	return model.ComponentSource{
		Layout:   "generated_wrapper",
		Metadata: projectComponentSourceRel(componentID, defaultString(source.Metadata, "component.json")),
		Init:     projectComponentSourceRel(componentID, source.Init),
		Step:     projectComponentSourceRel(componentID, source.Step),
		Helpers:  projectComponentSourceRel(componentID, source.Helpers),
		Wrapper:  projectComponentSourceRel(componentID, source.Wrapper),
	}
}

func projectComponentSourceRel(componentID string, rel string) string {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return ""
	}
	return filepath.ToSlash(filepath.Join("components", componentID, filepath.FromSlash(rel)))
}

func classPathForComponentSource(componentID string, source model.ComponentSource, className string) string {
	if source.Layout == "generated_wrapper" && strings.TrimSpace(source.Wrapper) != "" {
		module := strings.TrimSuffix(filepath.ToSlash(source.Wrapper), ".py")
		module = strings.ReplaceAll(module, "/", ".")
		return module + "." + className
	}
	return "components." + componentID + "." + className
}

func writeComponentTemplateFiles(projectRoot string, component model.Component, files []componentTemplateFile, templateClassName string, className string) error {
	for _, file := range files {
		targetRel := componentTemplateTargetRel(component, file)
		if targetRel == "" {
			continue
		}
		targetPath, err := resolveProjectOwnedFile(projectRoot, targetRel)
		if err != nil {
			return err
		}
		if _, err := os.Stat(targetPath); err == nil {
			return apperror.Errorf(apperror.CodeValidation, "component source already exists: %s", filepath.ToSlash(targetRel))
		} else if err != nil && !os.IsNotExist(err) {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		content := file.Content
		if component.Source.Layout != "generated_wrapper" || file.Role == "wrapper" {
			rewritten, err := rewriteTemplateClassName(content, templateClassName, className)
			if err != nil {
				return err
			}
			content = rewritten
		}
		if err := os.WriteFile(targetPath, []byte(content), 0o644); err != nil {
			return err
		}
	}
	if component.Source.Layout == "generated_wrapper" {
		initPath, err := resolveProjectOwnedFile(projectRoot, filepath.ToSlash(filepath.Join("components", component.ID, "__init__.py")))
		if err != nil {
			return err
		}
		if _, err := os.Stat(initPath); os.IsNotExist(err) {
			if err := os.WriteFile(initPath, []byte(""), 0o644); err != nil {
				return err
			}
		}
	}
	return nil
}

func componentTemplateTargetRel(component model.Component, file componentTemplateFile) string {
	if file.Role == "asset" {
		return projectComponentSourceRel(component.ID, file.TemplateRel)
	}
	if component.Source.Layout != "generated_wrapper" {
		return component.Source.Step
	}
	switch file.Role {
	case "init":
		return component.Source.Init
	case "step":
		return component.Source.Step
	case "helpers":
		return component.Source.Helpers
	case "wrapper":
		return component.Source.Wrapper
	default:
		return ""
	}
}

func writeComponentMetadataFile(path string, component model.Component, className string) error {
	metadata := struct {
		ID                   string                               `json:"id"`
		Name                 string                               `json:"name"`
		Kind                 string                               `json:"kind"`
		Category             string                               `json:"category"`
		ExecutionMode        string                               `json:"execution_mode"`
		ClassName            string                               `json:"class_name"`
		Source               model.ComponentSource                `json:"source"`
		Nodes                model.NodeSet                        `json:"nodes"`
		Parameters           map[string]any                       `json:"parameters"`
		ParameterDefinitions map[string]model.ParameterDefinition `json:"parameter_defs,omitempty"`
		StateDefinitions     map[string]model.StateDefinition     `json:"state_defs,omitempty"`
		SolverBoundary       *model.SolverBoundary                `json:"solver_boundary,omitempty"`
		MLMetadata           *model.MLMetadata                    `json:"ml_metadata,omitempty"`
	}{
		ID:                   component.ID,
		Name:                 component.Name,
		Kind:                 component.Kind,
		Category:             component.Category,
		ExecutionMode:        component.ExecutionMode,
		ClassName:            className,
		Source:               component.Source,
		Nodes:                component.Nodes,
		Parameters:           component.Parameters,
		ParameterDefinitions: component.ParameterDefinitions,
		StateDefinitions:     component.StateDefinitions,
		SolverBoundary:       component.SolverBoundary,
		MLMetadata:           component.MLMetadata,
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return writeJSONFile(path, metadata)
}

func syncComponentMetadataFile(loaded *project.LoadedProject, component model.Component) error {
	if strings.TrimSpace(component.Source.Metadata) == "" {
		return nil
	}
	metadataPath, err := resolveProjectOwnedFile(loaded.Root, component.Source.Metadata)
	if err != nil {
		return err
	}
	return writeComponentMetadataFile(metadataPath, component, classNameFromPath(component.Class))
}

func componentTemplateNodes(nodes []model.Node, direction string) []model.Node {
	out := make([]model.Node, 0, len(nodes))
	for _, node := range nodes {
		if node.Name == "" {
			node.Name = displayNameFromID(node.ID)
		}
		if node.Direction == "" {
			node.Direction = direction
		}
		if node.Medium == "" {
			node.Medium = "signal"
		}
		if node.ValueType == "" {
			node.ValueType = "float"
		}
		out = append(out, node)
	}
	return out
}

func displayNameFromID(id string) string {
	parts := strings.FieldsFunc(id, func(r rune) bool {
		return r == '_' || r == '-' || r == ' '
	})
	for index, part := range parts {
		if part == "" {
			continue
		}
		runes := []rune(part)
		runes[0] = unicode.ToUpper(runes[0])
		parts[index] = string(runes)
	}
	name := strings.Join(parts, " ")
	if name == "" {
		return id
	}
	return name
}

func cloneMap(values map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range values {
		out[key] = value
	}
	return out
}

func cloneComponentSource(value model.ComponentSource) model.ComponentSource {
	return model.ComponentSource{
		Layout:   value.Layout,
		Metadata: value.Metadata,
		Init:     value.Init,
		Step:     value.Step,
		Helpers:  value.Helpers,
		Wrapper:  value.Wrapper,
	}
}

func cloneParameterDefinitions(values map[string]model.ParameterDefinition) map[string]model.ParameterDefinition {
	if len(values) == 0 {
		return nil
	}
	out := map[string]model.ParameterDefinition{}
	for key, value := range values {
		if value.Bounds != nil {
			bounds := *value.Bounds
			value.Bounds = &bounds
		}
		if value.Visible != nil {
			visible := *value.Visible
			value.Visible = &visible
		}
		out[key] = value
	}
	return out
}

func cloneStateDefinitions(values map[string]model.StateDefinition) map[string]model.StateDefinition {
	if len(values) == 0 {
		return nil
	}
	out := map[string]model.StateDefinition{}
	for key, value := range values {
		out[key] = value
	}
	return out
}

func cloneSolverBoundary(value *model.SolverBoundary) *model.SolverBoundary {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneMLMetadata(value *model.MLMetadata) *model.MLMetadata {
	if value == nil {
		return nil
	}
	cloned := *value
	cloned.RequiredPackages = append([]string{}, value.RequiredPackages...)
	if value.ValidInputRanges != nil {
		cloned.ValidInputRanges = map[string]model.ValueBounds{}
		for key, bounds := range value.ValidInputRanges {
			cloned.ValidInputRanges[key] = bounds
		}
	}
	return &cloned
}

func componentMLMetadataForTemplate(value *model.MLMetadata, componentID string) *model.MLMetadata {
	cloned := cloneMLMetadata(value)
	if cloned == nil {
		return nil
	}
	for _, target := range []struct {
		value *string
	}{
		{&cloned.ModelFile},
		{&cloned.InputScalerFile},
		{&cloned.OutputScalerFile},
		{&cloned.FeatureSchemaFile},
		{&cloned.TargetSchemaFile},
		{&cloned.TrainingMetadataFile},
		{&cloned.ValidationReportFile},
	} {
		if strings.TrimSpace(*target.value) == "" || filepath.IsAbs(*target.value) {
			continue
		}
		*target.value = projectComponentSourceRel(componentID, *target.value)
	}
	return cloned
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
