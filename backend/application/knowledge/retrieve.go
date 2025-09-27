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

package knowledge

import (
	"context"
	"strconv"
	"time"

	knowledgeModel "github.com/coze-dev/coze-studio/backend/api/model/crossdomain/knowledge"
	dataset "github.com/coze-dev/coze-studio/backend/api/model/data/knowledge"
	"github.com/coze-dev/coze-studio/backend/domain/knowledge/service"
	"github.com/coze-dev/coze-studio/backend/pkg/errorx"
	"github.com/coze-dev/coze-studio/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-studio/backend/pkg/lang/slices"
	"github.com/coze-dev/coze-studio/backend/pkg/logs"
	"github.com/coze-dev/coze-studio/backend/types/errno"
)

// Retrieve 知识库检索接口
func (k *KnowledgeApplicationService) Retrieve(ctx context.Context, req *dataset.KnowledgeRetrieveRequest) (*dataset.KnowledgeRetrieveResponse, error) {
	startTime := time.Now()

	// 参数验证
	if req == nil {
		return nil, errorx.New(errno.ErrKnowledgeInvalidParamCode, errorx.KV("msg", "request is nil"))
	}

	// 转换知识库ID
	knowledgeIDs, err := convertKnowledgeIDs(req.KnowledgeIDs)
	if err != nil {
		logs.CtxErrorf(ctx, "convert knowledge ids failed, err: %v", err)
		return nil, errorx.New(errno.ErrKnowledgeInvalidParamCode, errorx.KV("msg", "invalid knowledge_ids"))
	}

	// 构建检索请求
	domainReq := &service.RetrieveRequest{
		Query:        req.Query,
		KnowledgeIDs: knowledgeIDs,
		Strategy:     convertRetrievalStrategy(req),
		ChatHistory:  nil, // 公开API暂不支持聊天历史
	}

	// 调用领域服务
	domainResp, err := k.DomainSVC.Retrieve(ctx, domainReq)
	if err != nil {
		logs.CtxErrorf(ctx, "domain retrieve failed, err: %v", err)
		return nil, err
	}

	// 转换响应
	resp := convertRetrieveResponse(domainResp)
	resp.Duration = time.Since(startTime).Milliseconds()

	// 查询重写功能暂不在响应中返回重写后的查询

	return resp, nil
}

// convertKnowledgeIDs 转换知识库ID列表
func convertKnowledgeIDs(knowledgeIDStrs []string) ([]int64, error) {
	knowledgeIDs := make([]int64, 0, len(knowledgeIDStrs))
	for _, idStr := range knowledgeIDStrs {
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return nil, err
		}
		knowledgeIDs = append(knowledgeIDs, id)
	}
	return knowledgeIDs, nil
}

// convertRetrievalStrategy 转换检索策略
func convertRetrievalStrategy(req *dataset.KnowledgeRetrieveRequest) *knowledgeModel.RetrievalStrategy {
	strategy := &knowledgeModel.RetrievalStrategy{
		SearchType:         req.GetSearchType(),
		TopK:               ptr.Of(req.GetTopK()),
		MinScore:           ptr.Of(req.GetMinScore()),
		EnableQueryRewrite: req.GetEnableQueryRewrite(),
		EnableRerank:       req.GetEnableRerank(),
		EnableNL2SQL:       req.GetEnableNL2SQL(),
		IsPersonalOnly:     false, // 公开API默认不限制个人内容
	}

	// 如果有自定义策略配置，优先使用
	if req.Strategy != nil {
		strategy.SearchType = req.Strategy.SearchType
		if req.Strategy.TopK != nil {
			strategy.TopK = req.Strategy.TopK
		}
		if req.Strategy.MinScore != nil {
			strategy.MinScore = req.Strategy.MinScore
		}
		strategy.EnableQueryRewrite = req.Strategy.EnableQueryRewrite
		strategy.EnableRerank = req.Strategy.EnableRerank
		strategy.EnableNL2SQL = req.Strategy.EnableNL2SQL
		strategy.IsPersonalOnly = req.Strategy.IsPersonalOnly
	}

	return strategy
}

// convertRetrieveResponse 转换检索响应
func convertRetrieveResponse(domainResp *service.RetrieveResponse) *dataset.KnowledgeRetrieveResponse {
	resp := dataset.NewKnowledgeRetrieveResponse()

	if domainResp == nil || len(domainResp.RetrieveSlices) == 0 {
		return resp
	}

	resp.Results = slices.Transform(domainResp.RetrieveSlices, func(slice *knowledgeModel.RetrieveSlice) *dataset.RetrieveResult {
		return convertRetrieveSlice(slice)
	})

	resp.Total = len(resp.Results)

	return resp
}

// convertRetrieveSlice 转换检索切片
func convertRetrieveSlice(slice *knowledgeModel.RetrieveSlice) *dataset.RetrieveResult {
	if slice == nil || slice.Slice == nil {
		return nil
	}

	result := &dataset.RetrieveResult{
		SliceID:      slice.Slice.ID,
		DocumentID:   slice.Slice.DocumentID,
		DocumentName: slice.Slice.DocumentName,
		KnowledgeID:  slice.Slice.KnowledgeID,
		Score:        slice.Score,
		Content:      slice.Slice.GetSliceContent(),
		ContentType:  getContentType(slice.Slice),
		CreatedAt:    slice.Slice.CreatedAtMs,
		UpdatedAt:    slice.Slice.UpdatedAtMs,
		Extra:        slice.Slice.Extra,
	}

	// 设置知识库名称
	if slice.Slice.Extra != nil {
		if knowledgeName, ok := slice.Slice.Extra["knowledge_name"]; ok {
			result.KnowledgeName = knowledgeName
		}
		if documentURL, ok := slice.Slice.Extra["document_url"]; ok && documentURL != "" {
			result.DocumentURL = ptr.Of(documentURL)
		}
	}

	return result
}

// getContentType 获取内容类型
func getContentType(slice *knowledgeModel.Slice) string {
	if slice.RawContent == nil || len(slice.RawContent) == 0 {
		return "text"
	}

	switch slice.RawContent[0].Type {
	case knowledgeModel.SliceContentTypeText:
		return "text"
	case knowledgeModel.SliceContentTypeTable:
		return "table"
	// case knowledgeModel.SliceContentTypeImage:
	//	return "image"
	default:
		return "text"
	}
}
