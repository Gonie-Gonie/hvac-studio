package studio

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/goniegonie/hvac-studio/tools/go/internal/apperror"
)

func decodeJSONRequest[T any](r *http.Request) (T, error) {
	defer r.Body.Close()
	var req T
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, apperror.Wrap(apperror.CodeInput, err)
	}
	return req, nil
}

func decodeCreateProjectRequest(r *http.Request) (createProjectRequest, error) {
	return decodeJSONRequest[createProjectRequest](r)
}

func decodeCopyProjectRequest(r *http.Request) (copyProjectRequest, error) {
	return decodeJSONRequest[copyProjectRequest](r)
}

func decodeCreateComponentRequest(r *http.Request) (createComponentRequest, error) {
	return decodeJSONRequest[createComponentRequest](r)
}

func decodeDuplicateComponentRequest(r *http.Request) (duplicateComponentRequest, error) {
	return decodeJSONRequest[duplicateComponentRequest](r)
}

func decodeReplaceComponentRequest(r *http.Request) (replaceComponentRequest, error) {
	return decodeJSONRequest[replaceComponentRequest](r)
}

func decodeUpdateComponentRequest(r *http.Request) (updateComponentRequest, error) {
	return decodeJSONRequest[updateComponentRequest](r)
}

func decodeUpdateComponentMLAssetsRequest(r *http.Request) (updateComponentMLAssetsRequest, error) {
	return decodeJSONRequest[updateComponentMLAssetsRequest](r)
}

func decodeApplyComponentMLSchemaNodesRequest(r *http.Request) (applyComponentMLSchemaNodesRequest, error) {
	return decodeJSONRequest[applyComponentMLSchemaNodesRequest](r)
}

func decodeDeleteComponentRequest(r *http.Request) (deleteComponentRequest, error) {
	return decodeJSONRequest[deleteComponentRequest](r)
}

func decodeIncludeComponentRequest(r *http.Request) (includeComponentRequest, error) {
	return decodeJSONRequest[includeComponentRequest](r)
}

func decodeCreateNodeRequest(r *http.Request) (createNodeRequest, error) {
	return decodeJSONRequest[createNodeRequest](r)
}

func decodeUpdateNodeRequest(r *http.Request) (updateNodeRequest, error) {
	return decodeJSONRequest[updateNodeRequest](r)
}

func decodeDeleteNodeRequest(r *http.Request) (deleteNodeRequest, error) {
	return decodeJSONRequest[deleteNodeRequest](r)
}

func decodeCreateConnectionRequest(r *http.Request) (createConnectionRequest, error) {
	return decodeJSONRequest[createConnectionRequest](r)
}

func decodeUpdateConnectionRequest(r *http.Request) (updateConnectionRequest, error) {
	return decodeJSONRequest[updateConnectionRequest](r)
}

func decodeDeleteConnectionRequest(r *http.Request) (deleteConnectionRequest, error) {
	return decodeJSONRequest[deleteConnectionRequest](r)
}

func decodeUpdateLayoutRequest(r *http.Request) (updateLayoutRequest, error) {
	return decodeJSONRequest[updateLayoutRequest](r)
}

func decodeExportRequest(r *http.Request) (exportRequest, error) {
	return decodeJSONRequest[exportRequest](r)
}

func decodeSourceRequest(r *http.Request) (sourceRequest, error) {
	return decodeJSONRequest[sourceRequest](r)
}

func decodeSourceCheckRequest(r *http.Request) (sourceCheckRequest, error) {
	return decodeJSONRequest[sourceCheckRequest](r)
}

func decodeCreateScenarioRequest(r *http.Request) (createScenarioRequest, error) {
	return decodeJSONRequest[createScenarioRequest](r)
}

func decodeUpdateParametersRequest(r *http.Request) (updateParametersRequest, error) {
	return decodeJSONRequest[updateParametersRequest](r)
}

func decodeUpdateComponentContractRequest(r *http.Request) (updateComponentContractRequest, error) {
	return decodeJSONRequest[updateComponentContractRequest](r)
}

func decodeApplyParameterSetRequest(r *http.Request) (applyParameterSetRequest, error) {
	return decodeJSONRequest[applyParameterSetRequest](r)
}

func decodeDeleteParameterRequest(r *http.Request) (deleteParameterRequest, error) {
	return decodeJSONRequest[deleteParameterRequest](r)
}

func decodeImportDatasetRequest(r *http.Request) (importDatasetRequest, error) {
	return decodeJSONRequest[importDatasetRequest](r)
}

func decodeCreateValidationMappingRequest(r *http.Request) (createValidationMappingRequest, error) {
	return decodeJSONRequest[createValidationMappingRequest](r)
}

func decodeUpdateValidationMappingRequest(r *http.Request) (updateValidationMappingRequest, error) {
	return decodeJSONRequest[updateValidationMappingRequest](r)
}

func decodeCopyValidationMappingRequest(r *http.Request) (copyValidationMappingRequest, error) {
	return decodeJSONRequest[copyValidationMappingRequest](r)
}

func decodeDeleteValidationMappingRequest(r *http.Request) (deleteValidationMappingRequest, error) {
	return decodeJSONRequest[deleteValidationMappingRequest](r)
}

func decodeCreateCalibrationSetupRequest(r *http.Request) (createCalibrationSetupRequest, error) {
	return decodeJSONRequest[createCalibrationSetupRequest](r)
}

func decodeCreateOptimizationSetupRequest(r *http.Request) (createOptimizationSetupRequest, error) {
	return decodeJSONRequest[createOptimizationSetupRequest](r)
}

func decodeUpdateInputRequest(r *http.Request) (updateInputRequest, error) {
	return decodeJSONRequest[updateInputRequest](r)
}

func decodeRequest(r *http.Request) (apiRequest, error) {
	return decodeJSONRequest[apiRequest](r)
}

func decodeSeriesRunRequest(r *http.Request) (seriesRunRequest, error) {
	return decodeJSONRequest[seriesRunRequest](r)
}

func decodeValidationRunRequest(r *http.Request) (validationRunRequest, error) {
	return decodeJSONRequest[validationRunRequest](r)
}

func decodeCalibrationRunRequest(r *http.Request) (calibrationRunRequest, error) {
	return decodeJSONRequest[calibrationRunRequest](r)
}

func decodeOptimizationRunRequest(r *http.Request) (optimizationRunRequest, error) {
	return decodeJSONRequest[optimizationRunRequest](r)
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
