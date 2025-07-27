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
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/coze-dev/coze-studio/backend/infra/contract/chatmodel"
	"github.com/coze-dev/coze-studio/backend/infra/contract/modelmgr"
	"github.com/coze-dev/coze-studio/backend/pkg/ctxcache"
	"github.com/coze-dev/coze-studio/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-studio/backend/pkg/logs"
)

// ClinxModel represents the model structure returned by CLINX API
// Based on actual API response format
type ClinxModel struct {
	ID                     string   `json:"id"`
	Object                 string   `json:"object"`
	Created                int64    `json:"created"`
	OwnedBy                string   `json:"owned_by"`
	SupportedEndpointTypes []string `json:"supported_endpoint_types"`
}

// ClinxModelsResponse represents the response from CLINX /v1/models API
type ClinxModelsResponse struct {
	Data    []ClinxModel `json:"data"`
	Success bool         `json:"success"`
}

func NewOnlineModelMgr() (modelmgr.Manager, error) {
	return &onlineModelManager{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

type onlineModelManager struct {
	httpClient *http.Client
}

func (o *onlineModelManager) ListModel(ctx context.Context, req *modelmgr.ListModelRequest) (*modelmgr.ListModelResponse, error) {
	models, err := o.fetchModelsFromClinx(ctx)
	if err != nil {
		return nil, fmt.Errorf("[onlineModelManager.ListModel] failed to fetch models from CLINX: %w", err)
	}

	// Convert CLINX models to internal model format
	convertedModels := make([]*modelmgr.Model, 0, len(models))
	for i, clinxModel := range models {
		model, err := o.convertClinxModel(&clinxModel, int64(i+1))
		if err != nil {
			logs.CtxWarnf(ctx, "[onlineModelManager.ListModel] failed to convert model %s: %v", clinxModel.ID, err)
			continue
		}
		convertedModels = append(convertedModels, model)
	}

	// Apply filters
	filteredModels := o.applyFilters(convertedModels, req)

	// Apply pagination
	startIdx := 0
	if req.Cursor != nil {
		start, err := strconv.ParseInt(*req.Cursor, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("[onlineModelManager.ListModel] invalid cursor: %w", err)
		}
		startIdx = int(start)
	}

	limit := req.Limit
	if limit == 0 {
		limit = 100
	}

	var result []*modelmgr.Model
	endIdx := startIdx + limit
	if startIdx < len(filteredModels) {
		if endIdx > len(filteredModels) {
			endIdx = len(filteredModels)
		}
		result = filteredModels[startIdx:endIdx]
	}

	resp := &modelmgr.ListModelResponse{
		ModelList: result,
		HasMore:   endIdx < len(filteredModels),
	}

	if resp.HasMore {
		resp.NextCursor = ptr.Of(strconv.FormatInt(int64(endIdx), 10))
	}

	return resp, nil
}

func (o *onlineModelManager) ListInUseModel(ctx context.Context, limit int, cursor *string) (*modelmgr.ListModelResponse, error) {
	return o.ListModel(ctx, &modelmgr.ListModelRequest{
		Status: []modelmgr.ModelStatus{modelmgr.StatusInUse},
		Limit:  limit,
		Cursor: cursor,
	})
}

func (o *onlineModelManager) MGetModelByID(ctx context.Context, req *modelmgr.MGetModelRequest) ([]*modelmgr.Model, error) {
	models, err := o.fetchModelsFromClinx(ctx)
	if err != nil {
		return nil, fmt.Errorf("[onlineModelManager.MGetModelByID] failed to fetch models from CLINX: %w", err)
	}

	result := make([]*modelmgr.Model, 0, len(req.IDs))
	for _, id := range req.IDs {
		for i, clinxModel := range models {
			if int64(i+1) == id {
				model, err := o.convertClinxModel(&clinxModel, id)
				if err != nil {
					logs.CtxWarnf(ctx, "[onlineModelManager.MGetModelByID] failed to convert model %s: %v", clinxModel.ID, err)
					continue
				}
				result = append(result, model)
				break
			}
		}
	}

	return result, nil
}

func (o *onlineModelManager) fetchModelsFromClinx(ctx context.Context) ([]ClinxModel, error) {
	clinxBaseURL := os.Getenv("CLINX_API_BASE_URL")
	if clinxBaseURL == "" {
		return nil, fmt.Errorf("CLINX_API_BASE_URL environment variable is not set")
	}

	url := clinxBaseURL + "/v1/models"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Add Authorization header with clinx_access_token from context
	if accessToken, ok := ctxcache.Get[string](ctx, "clinx_access_token"); ok && accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
		logs.CtxDebugf(ctx, "[fetchModelsFromClinx] Added Authorization header with clinx_access_token (length: %d)", len(accessToken))
	} else {
		logs.CtxWarnf(ctx, "[fetchModelsFromClinx] No clinx_access_token found in context")
	}

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Read response body for error details
		bodyBytes := make([]byte, 1024)
		if n, _ := resp.Body.Read(bodyBytes); n > 0 {
			logs.CtxErrorf(ctx, "[fetchModelsFromClinx] API error response: %s", string(bodyBytes[:n]))
		}
		return nil, fmt.Errorf("API returned status code %d", resp.StatusCode)
	}

	var clinxResp ClinxModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&clinxResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	logs.CtxInfof(ctx, "[fetchModelsFromClinx] fetched %d models from CLINX API", len(clinxResp.Data))
	return clinxResp.Data, nil
}

func (o *onlineModelManager) convertClinxModel(clinxModel *ClinxModel, id int64) (*modelmgr.Model, error) {
	// All models from API are considered in use
	status := modelmgr.StatusInUse

	// Infer model info from ID and owned_by
	modelInfo := o.inferModelInfo(clinxModel)

	// Create default parameters
	defaultParams := o.createDefaultParameters()

	// Get CLINX base URL for chat completion calls
	clinxBaseURL := os.Getenv("CLINX_API_BASE_URL")
	if clinxBaseURL == "" {
		return nil, fmt.Errorf("CLINX_API_BASE_URL environment variable is not set")
	}
	if !strings.HasSuffix(clinxBaseURL, "/v1") {
		clinxBaseURL += "/v1" // Ensure base URL ends with /v1
	}

	model := &modelmgr.Model{
		ID:      id,
		Name:    modelInfo.DisplayName,
		IconURL: modelInfo.IconURL,
		IconURI: "",
		Description: &modelmgr.MultilingualText{
			ZH: modelInfo.Description,
			EN: modelInfo.Description,
		},
		DefaultParameters: defaultParams,
		Meta: modelmgr.ModelMeta{
			//Name:     modelInfo.DisplayName,
			Protocol: modelInfo.Protocol,
			Capability: &modelmgr.Capability{
				FunctionCall:    modelInfo.Capabilities.FunctionCall,
				InputModal:      modelInfo.Capabilities.InputModal,
				InputTokens:     modelInfo.MaxTokens,
				JSONMode:        modelInfo.Capabilities.JSONMode,
				MaxTokens:       modelInfo.MaxTokens,
				OutputModal:     modelInfo.Capabilities.OutputModal,
				OutputTokens:    modelInfo.MaxTokens / 4, // Rough estimate
				PrefixCaching:   false,
				Reasoning:       modelInfo.Capabilities.Reasoning,
				PrefillResponse: false,
			},
			ConnConfig: &chatmodel.Config{
				BaseURL:          clinxBaseURL, // Use CLINX API base URL, client will append appropriate endpoint
				APIKey:           "",           // Will be populated with user's clinx_access_token at runtime
				Timeout:          30 * time.Second,
				Model:            clinxModel.ID,
				Temperature:      ptr.Of(float32(0.7)),
				FrequencyPenalty: ptr.Of(float32(0)),
				PresencePenalty:  ptr.Of(float32(0)),
				MaxTokens:        ptr.Of(4096),
				TopP:             ptr.Of(float32(1)),
				TopK:             ptr.Of(0),
				Stop:             []string{},
				OpenAI:           &chatmodel.OpenAIConfig{ByAzure: false},
			},
			Status: status,
		},
	}

	return model, nil
}

// ModelInfo represents inferred model information
type ModelInfo struct {
	DisplayName  string
	Description  string
	Protocol     chatmodel.Protocol
	IconURL      string
	MaxTokens    int
	Capabilities ParsedCapabilities
}

func (o *onlineModelManager) inferModelInfo(clinxModel *ClinxModel) *ModelInfo {
	// Infer protocol from owned_by and model ID
	protocol := o.inferProtocolFromModel(clinxModel)

	// Infer capabilities from model ID
	capabilities := o.inferCapabilitiesFromModelID(clinxModel.ID)

	// Generate display name from model ID
	displayName := o.generateDisplayName(clinxModel.ID)

	// Set appropriate max tokens based on model
	maxTokens := o.inferMaxTokens(clinxModel.ID)

	return &ModelInfo{
		DisplayName:  displayName,
		Description:  fmt.Sprintf("%s model powered by %s", displayName, clinxModel.OwnedBy),
		Protocol:     protocol,
		IconURL:      o.getModelIconURL(clinxModel.OwnedBy),
		MaxTokens:    maxTokens,
		Capabilities: capabilities,
	}
}

func (o *onlineModelManager) inferProtocolFromModel(clinxModel *ClinxModel) chatmodel.Protocol {
	// All online models from CLINX API use OpenAI protocol
	// The OpenAI builder will automatically handle CLINX's /v1/chat/completion endpoint
	return chatmodel.ProtocolOpenAI
}

func (o *onlineModelManager) inferCapabilitiesFromModelID(modelID string) ParsedCapabilities {
	capabilities := ParsedCapabilities{
		FunctionCall: false,
		JSONMode:     false,
		Reasoning:    false,
		InputModal:   []modelmgr.Modal{modelmgr.ModalText},
		OutputModal:  []modelmgr.Modal{modelmgr.ModalText},
	}

	// Infer capabilities based on model ID patterns
	if strings.Contains(modelID, "claude") || strings.Contains(modelID, "gpt-4") || strings.Contains(modelID, "deepseek") {
		capabilities.FunctionCall = true
		capabilities.JSONMode = true
	}

	if strings.Contains(modelID, "claude") || strings.Contains(modelID, "deepseek") {
		capabilities.Reasoning = true
	}

	// Audio models
	if strings.Contains(modelID, "audio") {
		capabilities.InputModal = append(capabilities.InputModal, modelmgr.ModalAudio)
		capabilities.OutputModal = append(capabilities.OutputModal, modelmgr.ModalAudio)
	}

	// Vision models (most modern models support vision)
	if strings.Contains(modelID, "vision") || strings.Contains(modelID, "gpt-4") ||
		strings.Contains(modelID, "claude") || strings.Contains(modelID, "gemini") {
		capabilities.InputModal = append(capabilities.InputModal, modelmgr.ModalImage)
	}

	// Video models
	if strings.Contains(modelID, "video") || modelID == "vidu2.0" || modelID == "viduq1" ||
		strings.Contains(modelID, "kling") || strings.Contains(modelID, "jimeng_vgfm") {
		capabilities.InputModal = append(capabilities.InputModal, modelmgr.ModalVideo)
		capabilities.OutputModal = append(capabilities.OutputModal, modelmgr.ModalVideo)
	}

	return capabilities
}

func (o *onlineModelManager) generateDisplayName(modelID string) string {
	// Convert model ID to a more readable display name
	switch {
	case strings.HasPrefix(modelID, "claude-3-5-haiku"):
		return "Claude 3.5 Haiku"
	case strings.HasPrefix(modelID, "claude-3-7-sonnet"):
		return "Claude 3.7 Sonnet"
	case strings.HasPrefix(modelID, "claude-opus-4"):
		return "Claude 4 Opus"
	case strings.HasPrefix(modelID, "claude-sonnet-4"):
		return "Claude 4 Sonnet"
	case strings.HasPrefix(modelID, "gpt-4o"):
		return "GPT-4o"
	case strings.HasPrefix(modelID, "gpt-4.1"):
		return "GPT-4.1"
	case strings.HasPrefix(modelID, "deepseek"):
		return "DeepSeek V3"
	case strings.HasPrefix(modelID, "doubao"):
		return "Doubao Pro"
	case strings.HasPrefix(modelID, "gemini-2.5"):
		return "Gemini 2.5 Pro"
	case strings.HasPrefix(modelID, "gemini-2.0"):
		return "Gemini 2.0 Flash"
	case strings.HasPrefix(modelID, "kimi"):
		return "Kimi Latest"
	case strings.HasPrefix(modelID, "text-embedding"):
		return "Text Embedding"
	default:
		// Capitalize first letter and replace dashes/underscores with spaces
		name := strings.ReplaceAll(modelID, "-", " ")
		name = strings.ReplaceAll(name, "_", " ")
		if len(name) > 0 {
			name = strings.ToUpper(name[:1]) + name[1:]
		}
		return name
	}
}

func (o *onlineModelManager) inferMaxTokens(modelID string) int {
	// Infer max tokens based on model ID
	switch {
	case strings.Contains(modelID, "256k"):
		return 256000
	case strings.Contains(modelID, "claude-opus-4") || strings.Contains(modelID, "claude-sonnet-4"):
		return 200000
	case strings.Contains(modelID, "claude"):
		return 200000
	case strings.Contains(modelID, "gpt-4"):
		return 128000
	case strings.Contains(modelID, "deepseek"):
		return 128000
	case strings.Contains(modelID, "gemini-2.5"):
		return 1000000
	case strings.Contains(modelID, "gemini"):
		return 128000
	case strings.Contains(modelID, "kimi"):
		return 200000
	case strings.Contains(modelID, "embedding"):
		return 8192
	default:
		return 32000 // Default reasonable value
	}
}

func (o *onlineModelManager) getModelIconURL(ownedBy string) string {
	// Return appropriate icon URL based on provider
	switch ownedBy {
	case "openai":
		return "default_icon/openai_v2.png"
	case "vertex-ai":
		return "default_icon/google_v2.png"
	case "jimeng":
		return "default_icon/jimeng_v2.png"
	default:
		return "default_icon/custom_v2.png"
	}
}

func (o *onlineModelManager) createDefaultParameters() []*modelmgr.Parameter {
	return []*modelmgr.Parameter{
		{
			Name: modelmgr.Temperature,
			Label: &modelmgr.MultilingualText{
				ZH: "生成随机性",
				EN: "Temperature",
			},
			Desc: &modelmgr.MultilingualText{
				ZH: "调高温度会使得模型的输出更多样性和创新性",
				EN: "Higher temperature makes model output more creative and diverse",
			},
			Type:      modelmgr.ValueTypeFloat,
			Min:       "0",
			Max:       "1",
			Precision: 1,
			DefaultVal: modelmgr.DefaultValue{
				modelmgr.DefaultTypeDefault:  "1.0",
				modelmgr.DefaultTypeBalance:  "0.8",
				modelmgr.DefaultTypeCreative: "1",
				modelmgr.DefaultTypePrecise:  "0.3",
			},
			Style: modelmgr.DisplayStyle{
				Widget: modelmgr.WidgetSlider,
				Label: &modelmgr.MultilingualText{
					ZH: "生成多样性",
					EN: "Generation diversity",
				},
			},
		},
		{
			Name: modelmgr.MaxTokens,
			Label: &modelmgr.MultilingualText{
				ZH: "最大回复长度",
				EN: "Response max length",
			},
			Desc: &modelmgr.MultilingualText{
				ZH: "控制模型输出的Tokens长度上限",
				EN: "Control the maximum length of model output tokens",
			},
			Type: modelmgr.ValueTypeInt,
			Min:  "1",
			Max:  "4096",
			DefaultVal: modelmgr.DefaultValue{
				modelmgr.DefaultTypeDefault: "4096",
			},
			Style: modelmgr.DisplayStyle{
				Widget: modelmgr.WidgetSlider,
				Label: &modelmgr.MultilingualText{
					ZH: "输入及输出设置",
					EN: "Input and output settings",
				},
			},
		},
	}
}

func (o *onlineModelManager) applyFilters(models []*modelmgr.Model, req *modelmgr.ListModelRequest) []*modelmgr.Model {
	var filtered []*modelmgr.Model

	for _, model := range models {
		// Apply fuzzy name filter
		if req.FuzzyModelName != nil && !strings.Contains(model.Name, *req.FuzzyModelName) {
			continue
		}

		// Apply status filter
		if len(req.Status) > 0 {
			found := false
			for _, status := range req.Status {
				if model.Meta.Status == status {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		filtered = append(filtered, model)
	}

	return filtered
}

type ParsedCapabilities struct {
	FunctionCall bool
	JSONMode     bool
	Reasoning    bool
	InputModal   []modelmgr.Modal
	OutputModal  []modelmgr.Modal
}
