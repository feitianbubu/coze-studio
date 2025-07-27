/*
 * Copyright 2025 coze-dev Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package online

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/coze-dev/coze-studio/backend/infra/contract/modelmgr"
	"github.com/coze-dev/coze-studio/backend/pkg/ctxcache"
)

func TestOnlineModelManager_ListModel(t *testing.T) {
	// Create a mock server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("Expected path /v1/models, got %s", r.URL.Path)
		}
		
		response := `{
			"data": [
				{
					"id": "gpt-4o-2024-08-06",
					"object": "model",
					"created": 1626777600,
					"owned_by": "openai",
					"supported_endpoint_types": ["openai"]
				},
				{
					"id": "claude-sonnet-4-20250514",
					"object": "model",
					"created": 1626777600,
					"owned_by": "vertex-ai",
					"supported_endpoint_types": ["openai"]
				}
			],
			"success": true
		}`
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer mockServer.Close()

	// Set the mock server URL as the CLINX_API_BASE_URL
	os.Setenv("CLINX_API_BASE_URL", mockServer.URL)
	defer os.Unsetenv("CLINX_API_BASE_URL")

	// Create the online model manager
	mgr, err := NewOnlineModelMgr()
	if err != nil {
		t.Fatalf("Failed to create online model manager: %v", err)
	}

	// Test ListModel
	ctx := context.Background()
	req := &modelmgr.ListModelRequest{
		Limit: 10,
	}

	resp, err := mgr.ListModel(ctx, req)
	if err != nil {
		t.Fatalf("ListModel failed: %v", err)
	}

	if len(resp.ModelList) != 2 {
		t.Errorf("Expected 2 models, got %d", len(resp.ModelList))
	}

	// Check the first model
	model1 := resp.ModelList[0]
	if model1.Name != "GPT-4o" {
		t.Errorf("Expected model name 'GPT-4o', got '%s'", model1.Name)
	}

	if string(model1.Meta.Protocol) != "openai" {
		t.Errorf("Expected protocol 'openai', got '%s'", string(model1.Meta.Protocol))
	}

	// Check the second model  
	model2 := resp.ModelList[1]
	if model2.Name != "Claude 4 Sonnet" {
		t.Errorf("Expected model name 'Claude 4 Sonnet', got '%s'", model2.Name)
	}

	if string(model2.Meta.Protocol) != "openai" {
		t.Errorf("Expected protocol 'openai', got '%s'", string(model2.Meta.Protocol))
	}

	t.Logf("Successfully fetched %d models from online API", len(resp.ModelList))
}

func TestOnlineModelManager_ListInUseModel(t *testing.T) {
	// Create a mock server with one active and one offline model
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `{
			"data": [
				{
					"id": "gpt-4o-2024-08-06",
					"object": "model",
					"created": 1626777600,
					"owned_by": "openai",
					"supported_endpoint_types": ["openai"]
				}
			],
			"success": true
		}`
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer mockServer.Close()

	os.Setenv("CLINX_API_BASE_URL", mockServer.URL)
	defer os.Unsetenv("CLINX_API_BASE_URL")

	mgr, err := NewOnlineModelMgr()
	if err != nil {
		t.Fatalf("Failed to create online model manager: %v", err)
	}

	ctx := context.Background()
	resp, err := mgr.ListInUseModel(ctx, 10, nil)
	if err != nil {
		t.Fatalf("ListInUseModel failed: %v", err)
	}

	// Should return the model
	if len(resp.ModelList) != 1 {
		t.Errorf("Expected 1 in-use model, got %d", len(resp.ModelList))
	}

	if resp.ModelList[0].Name != "GPT-4o" {
		t.Errorf("Expected in-use model 'GPT-4o', got '%s'", resp.ModelList[0].Name)
	}

	t.Logf("Successfully filtered to %d in-use models", len(resp.ModelList))
}

func TestOnlineModelManager_APIFailure(t *testing.T) {
	// Create a mock server that returns an error
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer mockServer.Close()

	os.Setenv("CLINX_API_BASE_URL", mockServer.URL)
	defer os.Unsetenv("CLINX_API_BASE_URL")

	mgr, err := NewOnlineModelMgr()
	if err != nil {
		t.Fatalf("Failed to create online model manager: %v", err)
	}

	ctx := context.Background()
	req := &modelmgr.ListModelRequest{Limit: 10}

	_, err = mgr.ListModel(ctx, req)
	if err == nil {
		t.Error("Expected error when API returns 500, but got nil")
	}

	t.Logf("Correctly handled API failure: %v", err)
}

func TestInferProtocolFromModel(t *testing.T) {
	mgr := &onlineModelManager{}

	tests := []struct {
		model    ClinxModel
		expected string
	}{
		{ClinxModel{ID: "gpt-4o", OwnedBy: "openai"}, "openai"},
		{ClinxModel{ID: "claude-sonnet-4", OwnedBy: "vertex-ai"}, "openai"},
		{ClinxModel{ID: "gemini-2.0", OwnedBy: "vertex-ai"}, "openai"},
		{ClinxModel{ID: "deepseek-v3", OwnedBy: "custom"}, "openai"},
		{ClinxModel{ID: "doubao-1-5-pro", OwnedBy: "custom"}, "openai"},
		{ClinxModel{ID: "kimi-latest", OwnedBy: "custom"}, "openai"},
		{ClinxModel{ID: "unknown-model", OwnedBy: "unknown"}, "openai"}, // all online models use openai protocol
	}

	for _, test := range tests {
		result := mgr.inferProtocolFromModel(&test.model)
		if string(result) != test.expected {
			t.Errorf("inferProtocolFromModel(%+v) = %s, expected %s", test.model, string(result), test.expected)
		}
	}
}

func TestGenerateDisplayName(t *testing.T) {
	mgr := &onlineModelManager{}

	tests := []struct {
		modelID  string
		expected string
	}{
		{"claude-sonnet-4-20250514", "Claude 4 Sonnet"},
		{"gpt-4o-2024-08-06", "GPT-4o"},
		{"deepseek-v3-0324", "DeepSeek V3"},
		{"gemini-2.5-pro", "Gemini 2.5 Pro"},
		{"kimi-latest", "Kimi Latest"},
		{"text-embedding-3-large", "Text Embedding"},
		{"unknown-model", "Unknown model"},
	}

	for _, test := range tests {
		result := mgr.generateDisplayName(test.modelID)
		if result != test.expected {
			t.Errorf("generateDisplayName(%s) = %s, expected %s", test.modelID, result, test.expected)
		}
	}
}

func TestInferCapabilitiesFromModelID(t *testing.T) {
	mgr := &onlineModelManager{}

	// Test Claude model capabilities
	claudeCapabilities := mgr.inferCapabilitiesFromModelID("claude-sonnet-4-20250514")
	if !claudeCapabilities.FunctionCall {
		t.Error("Expected Claude to have FunctionCall capability")
	}
	if !claudeCapabilities.Reasoning {
		t.Error("Expected Claude to have Reasoning capability")
	}

	// Test GPT-4 capabilities
	gptCapabilities := mgr.inferCapabilitiesFromModelID("gpt-4o-2024-08-06")
	if !gptCapabilities.FunctionCall {
		t.Error("Expected GPT-4 to have FunctionCall capability")
	}

	// Check if vision models have image modal
	hasImageModal := false
	for _, modal := range gptCapabilities.InputModal {
		if modal == modelmgr.ModalImage {
			hasImageModal = true
			break
		}
	}
	if !hasImageModal {
		t.Error("Expected GPT-4 to have Image input modal")
	}

	// Test audio model capabilities
	audioCapabilities := mgr.inferCapabilitiesFromModelID("gpt-4o-audio-preview")
	hasAudioModal := false
	for _, modal := range audioCapabilities.InputModal {
		if modal == modelmgr.ModalAudio {
			hasAudioModal = true
			break
		}
	}
	if !hasAudioModal {
		t.Error("Expected audio model to have Audio input modal")
	}
}

func TestOnlineModelManager_WithAuthToken(t *testing.T) {
	const expectedToken = "test-access-token-12345"
	var receivedAuthHeader string

	// Create a mock server that captures the Authorization header
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("Expected path /v1/models, got %s", r.URL.Path)
		}

		// Capture the Authorization header
		receivedAuthHeader = r.Header.Get("Authorization")
		
		response := `{
			"data": [
				{
					"id": "gpt-4o-2024-08-06",
					"object": "model",
					"created": 1626777600,
					"owned_by": "openai",
					"supported_endpoint_types": ["openai"]
				}
			],
			"success": true
		}`
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer mockServer.Close()

	// Set the mock server URL
	os.Setenv("CLINX_API_BASE_URL", mockServer.URL)
	defer os.Unsetenv("CLINX_API_BASE_URL")

	// Create the online model manager
	mgr, err := NewOnlineModelMgr()
	if err != nil {
		t.Fatalf("Failed to create online model manager: %v", err)
	}

	// Create context with access token
	ctx := context.Background()
	ctx = ctxcache.Init(ctx)
	ctxcache.Store(ctx, "clinx_access_token", expectedToken)

	// Test ListModel
	req := &modelmgr.ListModelRequest{Limit: 10}
	_, err = mgr.ListModel(ctx, req)
	if err != nil {
		t.Fatalf("ListModel failed: %v", err)
	}

	// Verify the Authorization header was sent correctly
	expectedAuthHeader := "Bearer " + expectedToken
	if receivedAuthHeader != expectedAuthHeader {
		t.Errorf("Expected Authorization header '%s', got '%s'", expectedAuthHeader, receivedAuthHeader)
	}

	t.Logf("Successfully sent Authorization header: %s", receivedAuthHeader)
}

func TestOnlineModelManager_WithoutAuthToken(t *testing.T) {
	var receivedAuthHeader string

	// Create a mock server that captures the Authorization header
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Capture the Authorization header (should be empty)
		receivedAuthHeader = r.Header.Get("Authorization")
		
		response := `{
			"data": [
				{
					"id": "gpt-4o-2024-08-06",
					"object": "model",
					"created": 1626777600,
					"owned_by": "openai",
					"supported_endpoint_types": ["openai"]
				}
			],
			"success": true
		}`
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer mockServer.Close()

	// Set the mock server URL
	os.Setenv("CLINX_API_BASE_URL", mockServer.URL)
	defer os.Unsetenv("CLINX_API_BASE_URL")

	// Create the online model manager
	mgr, err := NewOnlineModelMgr()
	if err != nil {
		t.Fatalf("Failed to create online model manager: %v", err)
	}

	// Create context without access token
	ctx := context.Background()

	// Test ListModel
	req := &modelmgr.ListModelRequest{Limit: 10}
	_, err = mgr.ListModel(ctx, req)
	if err != nil {
		t.Fatalf("ListModel failed: %v", err)
	}

	// Verify no Authorization header was sent
	if receivedAuthHeader != "" {
		t.Errorf("Expected no Authorization header, but got '%s'", receivedAuthHeader)
	}

	t.Log("Correctly handled request without access token")
}

func TestOnlineModelManager_BaseURLConfiguration(t *testing.T) {
	// Create a mock server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `{
			"data": [
				{
					"id": "gpt-4o-2024-08-06",
					"object": "model",
					"created": 1626777600,
					"owned_by": "openai",
					"supported_endpoint_types": ["openai"]
				}
			],
			"success": true
		}`
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer mockServer.Close()

	// Set the mock server URL as the CLINX_API_BASE_URL
	os.Setenv("CLINX_API_BASE_URL", mockServer.URL)
	defer os.Unsetenv("CLINX_API_BASE_URL")

	// Create the online model manager
	mgr, err := NewOnlineModelMgr()
	if err != nil {
		t.Fatalf("Failed to create online model manager: %v", err)
	}

	// Test ListModel
	ctx := context.Background()
	req := &modelmgr.ListModelRequest{Limit: 10}
	
	resp, err := mgr.ListModel(ctx, req)
	if err != nil {
		t.Fatalf("ListModel failed: %v", err)
	}

	if len(resp.ModelList) != 1 {
		t.Errorf("Expected 1 model, got %d", len(resp.ModelList))
	}

	// Verify that the BaseURL is set to the CLINX API base URL
	model := resp.ModelList[0]
	if model.Meta.ConnConfig.BaseURL != mockServer.URL {
		t.Errorf("Expected BaseURL to be '%s', got '%s'", mockServer.URL, model.Meta.ConnConfig.BaseURL)
	}

	t.Logf("Correctly set BaseURL to: %s", model.Meta.ConnConfig.BaseURL)
	t.Logf("Model uses OpenAI protocol which will use standard /v1/chat/completions endpoint")
}