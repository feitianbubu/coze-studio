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
	"github.com/coze-dev/coze-studio/backend/api/model/crossdomain/knowledge"
)

// KnowledgeRetrieveRequest 知识库检索请求
type KnowledgeRetrieveRequest struct {
	// 查询文本
	Query string `json:"query" vd:"len($)>0; msg:'query cannot be empty'"`

	// 知识库ID列表
	KnowledgeIDs []string `json:"knowledge_ids" vd:"len($)>0; msg:'knowledge_ids cannot be empty'"`

	// 检索策略配置
	Strategy *RetrievalStrategyConfig `json:"strategy,omitempty"`

	// 返回结果数量，默认10
	TopK *int64 `json:"top_k,omitempty"`

	// 最小相似度分数，默认0.0
	MinScore *float64 `json:"min_score,omitempty"`

	// 检索类型：0=语义检索，1=全文检索，2=混合检索，默认混合检索
	SearchType *int32 `json:"search_type,omitempty"`

	// 是否启用查询重写，默认true
	EnableQueryRewrite *bool `json:"enable_query_rewrite,omitempty"`

	// 是否启用重排序，默认true
	EnableRerank *bool `json:"enable_rerank,omitempty"`

	// 是否启用NL2SQL，默认false
	EnableNL2SQL *bool `json:"enable_nl2sql,omitempty"`
}

// RetrievalStrategyConfig 检索策略配置
type RetrievalStrategyConfig struct {
	// 检索类型：0=语义检索，1=全文检索，2=混合检索
	SearchType knowledge.SearchType `json:"search_type"`

	// 返回结果数量
	TopK *int64 `json:"top_k,omitempty"`

	// 最小相似度分数
	MinScore *float64 `json:"min_score,omitempty"`

	// 是否启用查询重写
	EnableQueryRewrite bool `json:"enable_query_rewrite"`

	// 是否启用重排序
	EnableRerank bool `json:"enable_rerank"`

	// 是否启用NL2SQL
	EnableNL2SQL bool `json:"enable_nl2sql"`

	// 是否仅返回个人创建的内容
	IsPersonalOnly bool `json:"is_personal_only"`
}

// KnowledgeRetrieveResponse 知识库检索响应
type KnowledgeRetrieveResponse struct {
	// 检索结果列表
	Results []*RetrieveResult `json:"results"`

	// 总数量
	Total int `json:"total"`

	// 检索用时（毫秒）
	Duration int64 `json:"duration"`

	// 实际使用的查询文本（如果启用了查询重写）
	ActualQuery *string `json:"actual_query,omitempty"`
}

// RetrieveResult 检索结果项
type RetrieveResult struct {
	// 切片ID
	SliceID int64 `json:"slice_id"`

	// 文档ID
	DocumentID int64 `json:"document_id"`

	// 文档名称
	DocumentName string `json:"document_name"`

	// 知识库ID
	KnowledgeID int64 `json:"knowledge_id"`

	// 知识库名称
	KnowledgeName string `json:"knowledge_name"`

	// 相似度分数
	Score float64 `json:"score"`

	// 内容
	Content string `json:"content"`

	// 内容类型：text, table, image
	ContentType string `json:"content_type"`

	// 文档URL（如果有）
	DocumentURL *string `json:"document_url,omitempty"`

	// 额外信息
	Extra map[string]string `json:"extra,omitempty"`

	// 创建时间（毫秒时间戳）
	CreatedAt int64 `json:"created_at"`

	// 更新时间（毫秒时间戳）
	UpdatedAt int64 `json:"updated_at"`
}

// NewKnowledgeRetrieveRequest 创建检索请求
func NewKnowledgeRetrieveRequest() *KnowledgeRetrieveRequest {
	return &KnowledgeRetrieveRequest{}
}

// NewKnowledgeRetrieveResponse 创建检索响应
func NewKnowledgeRetrieveResponse() *KnowledgeRetrieveResponse {
	return &KnowledgeRetrieveResponse{
		Results: make([]*RetrieveResult, 0),
	}
}

// GetTopK 获取TopK值，有默认值
func (r *KnowledgeRetrieveRequest) GetTopK() int64 {
	if r.TopK != nil {
		return *r.TopK
	}
	return 10
}

// GetMinScore 获取最小分数，有默认值
func (r *KnowledgeRetrieveRequest) GetMinScore() float64 {
	if r.MinScore != nil {
		return *r.MinScore
	}
	return 0.0
}

// GetSearchType 获取检索类型，有默认值
func (r *KnowledgeRetrieveRequest) GetSearchType() knowledge.SearchType {
	if r.SearchType != nil {
		return knowledge.SearchType(*r.SearchType)
	}
	return knowledge.SearchTypeHybrid
}

// GetEnableQueryRewrite 获取是否启用查询重写，有默认值
func (r *KnowledgeRetrieveRequest) GetEnableQueryRewrite() bool {
	if r.EnableQueryRewrite != nil {
		return *r.EnableQueryRewrite
	}
	return true
}

// GetEnableRerank 获取是否启用重排序，有默认值
func (r *KnowledgeRetrieveRequest) GetEnableRerank() bool {
	if r.EnableRerank != nil {
		return *r.EnableRerank
	}
	return true
}

// GetEnableNL2SQL 获取是否启用NL2SQL，有默认值
func (r *KnowledgeRetrieveRequest) GetEnableNL2SQL() bool {
	if r.EnableNL2SQL != nil {
		return *r.EnableNL2SQL
	}
	return false
}