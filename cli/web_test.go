package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"daily-tasks/internal"
)

func testWebServer(dataPath string) *webServer {
	return &webServer{
		dataPath: dataPath,
		staticFS: fstest.MapFS{
			"index.html": &fstest.MapFile{Data: []byte("<!doctype html><html></html>")},
		},
	}
}

func TestWebServerStateAndSave(t *testing.T) {
	tempDir := t.TempDir()
	dataPath := tempDir + "/tasks.json"
	server := testWebServer(dataPath)

	stateReq := httptest.NewRequest(http.MethodGet, "/api/state", nil)
	stateRes := httptest.NewRecorder()
	server.routes().ServeHTTP(stateRes, stateReq)

	if stateRes.Code != http.StatusOK {
		t.Fatalf("expected GET /api/state status 200, got %d", stateRes.Code)
	}

	var initial webStateResponse
	if err := json.Unmarshal(stateRes.Body.Bytes(), &initial); err != nil {
		t.Fatalf("failed to decode initial state: %v", err)
	}
	if initial.Data.NextID != 1 {
		t.Fatalf("expected empty state NextID=1, got %d", initial.Data.NextID)
	}
	if initial.DataPath != dataPath {
		t.Fatalf("expected data path %q, got %q", dataPath, initial.DataPath)
	}

	payload := internal.Data{
		LastReset:  "2026-03-27",
		NextID:     2,
		ThemeIndex: 3,
		Tasks: []internal.Task{
			{ID: 1, Title: "Ship local web server", Duration: 30, Status: "todo", Order: 1},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	saveReq := httptest.NewRequest(http.MethodPut, "/api/data", bytes.NewReader(body))
	saveRes := httptest.NewRecorder()
	server.routes().ServeHTTP(saveRes, saveReq)

	if saveRes.Code != http.StatusOK {
		t.Fatalf("expected PUT /api/data status 200, got %d", saveRes.Code)
	}

	var saved webStateResponse
	if err := json.Unmarshal(saveRes.Body.Bytes(), &saved); err != nil {
		t.Fatalf("failed to decode save response: %v", err)
	}
	if saved.Action != "saved" {
		t.Fatalf("expected action saved, got %q", saved.Action)
	}
	if len(saved.Data.Tasks) != 1 || saved.Data.Tasks[0].Title != "Ship local web server" {
		t.Fatalf("expected saved task in response, got %+v", saved.Data.Tasks)
	}
	if saved.Data.LastModified == 0 {
		t.Fatal("expected LastModified to be set after save")
	}

	loaded, err := internal.LoadData(dataPath)
	if err != nil {
		t.Fatalf("failed to load saved data: %v", err)
	}
	if len(loaded.Tasks) != 1 || loaded.Tasks[0].Title != "Ship local web server" {
		t.Fatalf("expected saved task on disk, got %+v", loaded.Tasks)
	}
}

func TestWebServerSyncRequiresConfig(t *testing.T) {
	t.Setenv("DAILY_TASKS_WEBDAV_URL", "")
	t.Setenv("DAILY_TASKS_WEBDAV_USER", "")
	t.Setenv("DAILY_TASKS_WEBDAV_PASS", "")

	tempDir := t.TempDir()
	server := testWebServer(tempDir + "/tasks.json")

	req := httptest.NewRequest(http.MethodPost, "/api/sync", nil)
	res := httptest.NewRecorder()
	server.routes().ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected POST /api/sync status 400, got %d", res.Code)
	}

	var payload webErrorResponse
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if !strings.Contains(payload.Error, "not configured") {
		t.Fatalf("expected not configured error, got %q", payload.Error)
	}
}
