// White-box test: package wikiresync so we can register the real handler methods.
package wikiresync

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// newTestRouter builds a minimal Gin engine wiring the real handler methods so
// any change to handleTriggerResync or handleResyncStatus is exercised.
func newTestRouter(job *ResyncJob, trigger func()) *gin.Engine {
	r := gin.New()
	routes := &Routes{
		triggerUC: NewTriggerResyncUseCase(job, trigger, nil),
		statusUC:  NewGetResyncStatusUseCase(job),
	}
	r.POST("/resync", routes.handleTriggerResync)
	r.GET("/resync/status", routes.handleResyncStatus)
	return r
}

func TestHandleTriggerResync_Returns202(t *testing.T) {
	job := NewResyncJob()
	launched := make(chan struct{}, 1)
	router := newTestRouter(job, func() {
		launched <- struct{}{}
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/resync", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("expected 202, got %d", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["ok"] != true {
		t.Errorf("expected {ok:true}, got %v", body)
	}
	select {
	case <-launched:
	default:
		t.Error("trigger was not called")
	}
}

func TestHandleTriggerResync_Returns409WhenAlreadyRunning(t *testing.T) {
	job := NewResyncJob()
	job.Start() // simulate a running job

	called := false
	router := newTestRouter(job, func() { called = true })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/resync", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", w.Code)
	}
	var body ResyncErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.Error.Code != ErrCodeResyncAlreadyRunning {
		t.Errorf("expected code %q, got %q", ErrCodeResyncAlreadyRunning, body.Error.Code)
	}
	if called {
		t.Error("trigger must not be called when job is already running")
	}
}

func TestHandleResyncStatus_IdleState(t *testing.T) {
	job := NewResyncJob()
	router := newTestRouter(job, func() {})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/resync/status", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var s JobStatus
	if err := json.Unmarshal(w.Body.Bytes(), &s); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if s.Running {
		t.Error("idle job should not be running")
	}
	if s.Done {
		t.Error("idle job should not be done")
	}
}

func TestHandleResyncStatus_RunningState(t *testing.T) {
	job := NewResyncJob()
	job.Start()
	job.SetPhase(PhaseLinks)

	router := newTestRouter(job, func() {})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/resync/status", nil)
	router.ServeHTTP(w, req)

	var s JobStatus
	if err := json.Unmarshal(w.Body.Bytes(), &s); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if !s.Running {
		t.Error("job should be running")
	}
	if s.Phase != "links" {
		t.Errorf("expected phase 'links', got %q", s.Phase)
	}
}

func TestHandleResyncStatus_DoneState(t *testing.T) {
	job := NewResyncJob()
	job.Start()
	job.SetPhase(PhaseSearch)
	job.Finish(nil)

	router := newTestRouter(job, func() {})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/resync/status", nil)
	router.ServeHTTP(w, req)

	var s JobStatus
	if err := json.Unmarshal(w.Body.Bytes(), &s); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if s.Running {
		t.Error("finished job should not be running")
	}
	if !s.Done {
		t.Error("finished job should be done")
	}
	if s.Error != "" {
		t.Errorf("expected no error, got %q", s.Error)
	}
}

func TestHandleResyncStatus_DoneWithError(t *testing.T) {
	job := NewResyncJob()
	job.Start()
	job.SetPhase(PhaseTree)
	job.Finish(errors.New("tree failed"))

	router := newTestRouter(job, func() {})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/resync/status", nil)
	router.ServeHTTP(w, req)

	var s JobStatus
	if err := json.Unmarshal(w.Body.Bytes(), &s); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if !s.Done {
		t.Error("expected done=true")
	}
	if s.Error != "tree failed" {
		t.Errorf("expected error message, got %q", s.Error)
	}
}

func TestHandleTriggerResync_PhaseOmittedWhenEmpty(t *testing.T) {
	job := NewResyncJob()
	router := newTestRouter(job, func() {})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/resync/status", nil)
	router.ServeHTTP(w, req)

	// phase must be absent (omitempty) when empty so JS ?? null works correctly.
	var raw map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := raw["phase"]; ok {
		t.Errorf("phase key must be absent when empty, got %v", raw["phase"])
	}
}
