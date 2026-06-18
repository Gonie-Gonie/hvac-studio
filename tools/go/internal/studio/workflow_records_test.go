package studio

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/goniegonie/hvac-studio/tools/go/internal/model"
	"github.com/goniegonie/hvac-studio/tools/go/internal/project"
	runtimecore "github.com/goniegonie/hvac-studio/tools/go/internal/runtime"
)

func TestRunRecordEndpointReturnsSavedRecord(t *testing.T) {
	_, server := newIsolatedTestServer(t)
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader([]byte(`{"name":"Run Record Project"}`)))
	server.Handler().ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", createResponse.Code, createResponse.Body.String())
	}
	var createBody struct {
		Project ProjectSummary `json:"project"`
	}
	if err := json.Unmarshal(createResponse.Body.Bytes(), &createBody); err != nil {
		t.Fatal(err)
	}

	runPayload, err := json.Marshal(map[string]any{
		"project_path": createBody.Project.ProjectPath,
		"save":         true,
	})
	if err != nil {
		t.Fatal(err)
	}
	runResponse := httptest.NewRecorder()
	runRequest := httptest.NewRequest(http.MethodPost, "/api/run", bytes.NewReader(runPayload))
	server.Handler().ServeHTTP(runResponse, runRequest)
	if runResponse.Code != http.StatusOK {
		t.Fatalf("run status = %d body=%s", runResponse.Code, runResponse.Body.String())
	}
	var runBody struct {
		RunRecord RunSummary `json:"run_record"`
	}
	if err := json.Unmarshal(runResponse.Body.Bytes(), &runBody); err != nil {
		t.Fatal(err)
	}

	recordResponse := httptest.NewRecorder()
	recordRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/project/run?project_path="+url.QueryEscape(createBody.Project.ProjectPath)+"&run_id="+url.QueryEscape(runBody.RunRecord.ID),
		nil,
	)
	server.Handler().ServeHTTP(recordResponse, recordRequest)
	if recordResponse.Code != http.StatusOK {
		t.Fatalf("record status = %d body=%s", recordResponse.Code, recordResponse.Body.String())
	}
	var recordBody struct {
		RunRecord RunRecord `json:"run_record"`
	}
	if err := json.Unmarshal(recordResponse.Body.Bytes(), &recordBody); err != nil {
		t.Fatal(err)
	}
	if recordBody.RunRecord.ID != runBody.RunRecord.ID {
		t.Fatalf("record id = %s, want %s", recordBody.RunRecord.ID, runBody.RunRecord.ID)
	}
	if recordBody.RunRecord.Result.Outputs["result"] != 8.0 {
		t.Fatalf("record result = %v, want 8", recordBody.RunRecord.Result.Outputs["result"])
	}
}

func TestRunRecordsRoundTrip(t *testing.T) {
	projectRoot := t.TempDir()
	loaded := &project.LoadedProject{
		Project: &model.Project{ProjectName: "recorded"},
		Root:    projectRoot,
	}
	input := runtimecore.RunInput{
		Inputs:  map[string]any{"value": 4.0},
		Context: map[string]any{"time": 0.0, "dt": 60.0},
	}
	result := &runtimecore.RunResult{
		OK:      true,
		Outputs: map[string]any{"result": 8.0},
	}

	summary, err := writeRunRecord(loaded, input, result, "")
	if err != nil {
		t.Fatal(err)
	}
	if summary.RelativePath == "" {
		t.Fatal("run summary did not include relative path")
	}
	summaries := loadRunSummaries(projectRoot)
	if len(summaries) != 1 {
		t.Fatalf("run summary count = %d", len(summaries))
	}
	if summaries[0].ID != summary.ID {
		t.Fatalf("run id = %s, want %s", summaries[0].ID, summary.ID)
	}
}
